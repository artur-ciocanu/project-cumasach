package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
)

type Options struct {
	Registry    oci.Registry
	Reference   string
	Graph       *resolve.Graph
	TargetDir   string
	Now         func() time.Time
	StateWriter func(string, State) error
}

var commitActivations = func(activations []*Activation) error {
	return CommitAll(activations)
}

func Install(ctx context.Context, options Options) (State, error) {
	if options.Registry == nil {
		return State{}, fmt.Errorf("registry is required")
	}
	if options.Reference == "" && options.Graph == nil {
		return State{}, fmt.Errorf("artifact reference or resolved graph is required")
	}
	if options.TargetDir == "" {
		return State{}, fmt.Errorf("target directory is required")
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}
	stateWriter := options.StateWriter
	if stateWriter == nil {
		stateWriter = WriteState
	}
	if err := os.MkdirAll(filepath.Dir(options.TargetDir), 0o755); err != nil {
		return State{}, fmt.Errorf("create target parent directory: %w", err)
	}

	prepared, resolved, err := prepareInstall(ctx, options)
	if err != nil {
		return State{}, err
	}
	defer cleanupPrepared(prepared)

	activations, err := ActivateAll(options.TargetDir, prepared)
	if err != nil {
		return State{}, err
	}

	state, err := nextState(options.TargetDir, resolved, now().UTC())
	if err != nil {
		if rollbackErr := RollbackAll(activations); rollbackErr != nil {
			return State{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return State{}, err
	}
	if err := stateWriter(options.TargetDir, state); err != nil {
		if rollbackErr := RollbackAll(activations); rollbackErr != nil {
			return State{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return State{}, err
	}
	if err := commitActivations(activations); err != nil {
		return State{}, fmt.Errorf("install succeeded but cleanup failed: %w", err)
	}

	return state, nil
}

func Rollback(ctx context.Context, options Options) (State, error) {
	if options.Registry == nil {
		return State{}, fmt.Errorf("registry is required")
	}
	if options.TargetDir == "" {
		return State{}, fmt.Errorf("target directory is required")
	}

	now := options.Now
	if now == nil {
		now = time.Now
	}
	stateWriter := options.StateWriter
	if stateWriter == nil {
		stateWriter = WriteState
	}

	current, err := LoadState(options.TargetDir)
	if err != nil {
		return State{}, err
	}
	if len(current.History) < 2 {
		return State{}, fmt.Errorf("rollback failed: no earlier history snapshot exists")
	}

	targetSnapshot := append([]ResolvedSkill(nil), current.History[len(current.History)-2].Resolved...)
	graph, err := graphFromResolvedSnapshot(targetSnapshot)
	if err != nil {
		return State{}, err
	}

	prepared, resolved, err := prepareGraphInstall(ctx, options.Registry, options.TargetDir, graph)
	if err != nil {
		return State{}, err
	}
	defer cleanupPrepared(prepared)

	activations, err := restoreSnapshot(options.TargetDir, current.Active, prepared)
	if err != nil {
		return State{}, err
	}

	state, err := nextRollbackState(current, targetSnapshot, now().UTC())
	if err != nil {
		if rollbackErr := RollbackAll(activations); rollbackErr != nil {
			return State{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return State{}, err
	}
	if err := stateWriter(options.TargetDir, state); err != nil {
		if rollbackErr := RollbackAll(activations); rollbackErr != nil {
			return State{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return State{}, err
	}
	if err := commitActivations(activations); err != nil {
		return State{}, fmt.Errorf("rollback succeeded but cleanup failed: %w", err)
	}

	_ = resolved
	return state, nil
}

func prepareInstall(ctx context.Context, options Options) ([]PreparedSkill, []ResolvedSkill, error) {
	if options.Graph != nil {
		return prepareGraphInstall(ctx, options.Registry, options.TargetDir, *options.Graph)
	}

	fetched, err := oci.Fetch(ctx, options.Registry, options.Reference)
	if err != nil {
		return nil, nil, err
	}

	prepared, err := prepareFetchedArtifact(fetched, options.TargetDir)
	if err != nil {
		return nil, nil, err
	}
	return []PreparedSkill{prepared.PreparedSkill}, []ResolvedSkill{prepared.Resolved}, nil
}

type preparedArtifact struct {
	PreparedSkill
	Resolved ResolvedSkill
}

func prepareGraphInstall(ctx context.Context, registry oci.Registry, targetDir string, graph resolve.Graph) ([]PreparedSkill, []ResolvedSkill, error) {
	names := make([]string, 0, len(graph.Packages))
	for name := range graph.Packages {
		names = append(names, name)
	}
	slices.Sort(names)

	prepared := make([]PreparedSkill, 0, len(names))
	resolved := make([]ResolvedSkill, 0, len(names))
	for _, name := range names {
		selected := graph.Packages[name]
		fetched, err := oci.Fetch(ctx, registry, selected.Reference)
		if err != nil {
			return nil, nil, err
		}

		artifact, err := prepareFetchedArtifactForSelected(fetched, targetDir, selected)
		if err != nil {
			return nil, nil, err
		}
		prepared = append(prepared, artifact.PreparedSkill)
		resolved = append(resolved, artifact.Resolved)
	}
	return prepared, resolved, nil
}

func prepareFetchedArtifact(fetched oci.FetchedArtifact, targetDir string) (preparedArtifact, error) {
	parsedRef, err := oci.ParseReference(fetched.Reference)
	if err != nil {
		return preparedArtifact{}, fmt.Errorf("parse fetched artifact reference: %w", err)
	}

	mirroredManifestBytes, mirroredManifest, err := archivepkg.ReadMirroredManifestTGZ(bytes.NewReader(fetched.Archive))
	if err != nil {
		return preparedArtifact{}, fmt.Errorf("read mirrored manifest from fetched archive: %w", err)
	}
	if !bytes.Equal(fetched.Config, mirroredManifestBytes) {
		return preparedArtifact{}, fmt.Errorf("OCI config blob does not match mirrored manifest")
	}

	extractedRoot, extractedManifest, err := archivepkg.ExtractTGZTemp(bytes.NewReader(fetched.Archive), filepath.Dir(targetDir))
	if err != nil {
		return preparedArtifact{}, fmt.Errorf("extract fetched archive: %w", err)
	}
	if extractedManifest.Name != mirroredManifest.Name || extractedManifest.Version != mirroredManifest.Version {
		return preparedArtifact{}, fmt.Errorf("extracted manifest does not match mirrored manifest")
	}

	return preparedArtifact{
		PreparedSkill: PreparedSkill{
			ExtractedRoot: extractedRoot,
			SkillName:     mirroredManifest.Name,
		},
		Resolved: ResolvedSkill{
			Name:      mirroredManifest.Name,
			Version:   mirroredManifest.Version,
			Digest:    parsedRef.Digest,
			Reference: parsedRef.Canonical(),
		},
	}, nil
}

func prepareFetchedArtifactForSelected(fetched oci.FetchedArtifact, targetDir string, selected resolve.SelectedPackage) (preparedArtifact, error) {
	artifact, err := prepareFetchedArtifact(fetched, targetDir)
	if err != nil {
		return preparedArtifact{}, err
	}

	expectedRef, err := oci.ParseReference(selected.Reference)
	if err != nil {
		return preparedArtifact{}, fmt.Errorf("parse selected package reference for %q: %w", selected.Name, err)
	}
	expectedReference := expectedRef.Canonical()
	if artifact.Resolved.Reference != expectedReference {
		return preparedArtifact{}, fmt.Errorf("fetched artifact %q does not match expected selected package %q reference %q", artifact.Resolved.Reference, selected.Name, expectedReference)
	}
	if artifact.Resolved.Digest != selected.Digest {
		return preparedArtifact{}, fmt.Errorf("fetched artifact digest %q does not match expected selected package %q digest %q", artifact.Resolved.Digest, selected.Name, selected.Digest)
	}
	if artifact.Resolved.Name != selected.Name {
		return preparedArtifact{}, fmt.Errorf("fetched artifact name %q does not match expected selected package %q", artifact.Resolved.Name, selected.Name)
	}
	if artifact.Resolved.Version != selected.Version {
		return preparedArtifact{}, fmt.Errorf("fetched artifact version %q does not match expected selected package %q version %q", artifact.Resolved.Version, selected.Name, selected.Version)
	}

	return artifact, nil
}

func cleanupPrepared(prepared []PreparedSkill) {
	for _, skill := range prepared {
		if skill.ExtractedRoot != "" {
			_ = os.RemoveAll(filepath.Dir(skill.ExtractedRoot))
		}
	}
}

func restoreSnapshot(targetDir string, current []ResolvedSkill, prepared []PreparedSkill) ([]*Activation, error) {
	targetNames := make(map[string]struct{}, len(prepared))
	for _, skill := range prepared {
		targetNames[skill.SkillName] = struct{}{}
	}

	activations := make([]*Activation, 0, len(current)+len(prepared))
	for _, active := range current {
		if _, keep := targetNames[active.Name]; keep {
			continue
		}
		activation, err := Deactivate(targetDir, active.Name)
		if err != nil {
			_ = RollbackAll(activations)
			return nil, err
		}
		activations = append(activations, activation)
	}

	for _, skill := range prepared {
		activation, err := Activate(skill.ExtractedRoot, targetDir, skill.SkillName)
		if err != nil {
			_ = RollbackAll(activations)
			return nil, err
		}
		activations = append(activations, activation)
	}

	return activations, nil
}

func nextState(targetDir string, selected []ResolvedSkill, timestamp time.Time) (State, error) {
	previous, exists, err := LoadStateIfExists(targetDir)
	if err != nil {
		return State{}, err
	}

	action := "install"
	history := make([]HistoryEntry, 0, len(previous.History)+1)
	if exists {
		action = "upgrade"
		history = append(history, previous.History...)
	}

	active := mergeActive(previous.Active, selected)
	history = append(history, HistoryEntry{
		Timestamp: timestamp.Format(time.RFC3339),
		Action:    action,
		Resolved:  append([]ResolvedSkill(nil), active...),
	})

	return State{
		SchemaVersion: SchemaVersion,
		Target: Target{
			Path: targetDir,
		},
		Active:  active,
		History: history,
	}, nil
}

func mergeActive(previous, selected []ResolvedSkill) []ResolvedSkill {
	merged := make(map[string]ResolvedSkill, len(previous)+len(selected))
	for _, skill := range previous {
		merged[skill.Name] = skill
	}
	for _, skill := range selected {
		merged[skill.Name] = skill
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	slices.Sort(names)

	active := make([]ResolvedSkill, 0, len(names))
	for _, name := range names {
		active = append(active, merged[name])
	}
	return active
}

func nextRollbackState(previous State, selected []ResolvedSkill, timestamp time.Time) (State, error) {
	history := make([]HistoryEntry, 0, len(previous.History)+1)
	history = append(history, previous.History...)
	history = append(history, HistoryEntry{
		Timestamp: timestamp.Format(time.RFC3339),
		Action:    "rollback",
		Resolved:  append([]ResolvedSkill(nil), selected...),
	})

	return State{
		SchemaVersion: SchemaVersion,
		Target:        previous.Target,
		Active:        append([]ResolvedSkill(nil), selected...),
		History:       history,
	}, nil
}

func graphFromResolvedSnapshot(snapshot []ResolvedSkill) (resolve.Graph, error) {
	if len(snapshot) == 0 {
		return resolve.Graph{}, fmt.Errorf("rollback snapshot must not be empty")
	}

	names := make([]string, 0, len(snapshot))
	packages := make(map[string]resolve.SelectedPackage, len(snapshot))
	edges := make(map[string][]string, len(snapshot))
	for _, skill := range snapshot {
		if _, exists := packages[skill.Name]; exists {
			return resolve.Graph{}, fmt.Errorf("rollback snapshot contains duplicate skill %q", skill.Name)
		}
		names = append(names, skill.Name)
		packages[skill.Name] = resolve.SelectedPackage{
			Name:      skill.Name,
			Version:   skill.Version,
			Reference: skill.Reference,
			Digest:    skill.Digest,
		}
		edges[skill.Name] = nil
	}
	slices.Sort(names)

	// The rollback graph is used solely for fetch-and-activate; the root identity
	// is not meaningful. Pick the alphabetically first name for determinism.
	return resolve.Graph{
		Root:     names[0],
		Packages: packages,
		Edges:    edges,
	}, nil
}
