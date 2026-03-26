package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

type Options struct {
	Registry    oci.Registry
	Reference   string
	TargetDir   string
	Now         func() time.Time
	StateWriter func(string, State) error
}

var commitActivation = func(activation *Activation) error {
	return activation.Commit()
}

func Install(ctx context.Context, options Options) (State, error) {
	if options.Registry == nil {
		return State{}, fmt.Errorf("registry is required")
	}
	if options.Reference == "" {
		return State{}, fmt.Errorf("artifact reference is required")
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

	fetched, err := oci.Fetch(ctx, options.Registry, options.Reference)
	if err != nil {
		return State{}, err
	}

	mirroredManifestBytes, mirroredManifest, err := archivepkg.ReadMirroredManifestTGZ(bytes.NewReader(fetched.Archive))
	if err != nil {
		return State{}, fmt.Errorf("read mirrored manifest from fetched archive: %w", err)
	}
	if !bytes.Equal(fetched.Config, mirroredManifestBytes) {
		return State{}, fmt.Errorf("OCI config blob does not match mirrored manifest")
	}

	extractedRoot, extractedManifest, err := archivepkg.ExtractTGZTemp(bytes.NewReader(fetched.Archive), filepath.Dir(options.TargetDir))
	if err != nil {
		return State{}, fmt.Errorf("extract fetched archive: %w", err)
	}
	defer os.RemoveAll(filepath.Dir(extractedRoot))

	if extractedManifest.Name != mirroredManifest.Name || extractedManifest.Version != mirroredManifest.Version {
		return State{}, fmt.Errorf("extracted manifest does not match mirrored manifest")
	}

	activation, err := Activate(extractedRoot, options.TargetDir, mirroredManifest.Name)
	if err != nil {
		return State{}, err
	}

	resolved := ResolvedSkill{
		Name:      mirroredManifest.Name,
		Version:   mirroredManifest.Version,
		Digest:    mustParseReference(fetched.Reference).Digest,
		Reference: fetched.Reference,
	}

	state, err := nextState(options.TargetDir, resolved, now().UTC())
	if err != nil {
		if rollbackErr := activation.Rollback(); rollbackErr != nil {
			return State{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return State{}, err
	}
	if err := stateWriter(options.TargetDir, state); err != nil {
		if rollbackErr := activation.Rollback(); rollbackErr != nil {
			return State{}, fmt.Errorf("%w; rollback failed: %v", err, rollbackErr)
		}
		return State{}, err
	}
	if err := commitActivation(activation); err != nil {
		return State{}, fmt.Errorf("install succeeded but cleanup failed: %w", err)
	}

	return state, nil
}

func nextState(targetDir string, resolved ResolvedSkill, timestamp time.Time) (State, error) {
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

	active := []ResolvedSkill{resolved}
	history = append(history, HistoryEntry{
		Timestamp: timestamp.Format(time.RFC3339),
		Action:    action,
		Resolved:  []ResolvedSkill{resolved},
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

func mustParseReference(raw string) oci.Reference {
	ref, err := oci.ParseReference(raw)
	if err != nil {
		panic(err)
	}
	return ref
}
