package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	lockfilepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/lockfile"
	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
)

func TestInstallSingleRootWritesActiveDirectoryAndState(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	ref := pushSkill(t, registry, fixtureSkillDir(t), "registry.example.com/agentskills/list-directory")

	targetDir := t.TempDir()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	state, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: ref,
		TargetDir: targetDir,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "list-directory", "SKILL.md")); err != nil {
		t.Fatalf("Stat(active SKILL.md) error = %v", err)
	}

	loaded, err := LoadState(targetDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	if len(loaded.Active) != 1 || loaded.Active[0].Name != "list-directory" {
		t.Fatalf("loaded active = %#v, want one list-directory entry", loaded.Active)
	}
	if len(loaded.History) != 1 || loaded.History[0].Action != "install" {
		t.Fatalf("loaded history = %#v, want single install entry", loaded.History)
	}
	if got := loaded.History[0].Timestamp; got != now.Format(time.RFC3339) {
		t.Fatalf("history timestamp = %q, want %q", got, now.Format(time.RFC3339))
	}
	if !equalResolvedSets(loaded.Active, loaded.History[0].Resolved) {
		t.Fatalf("history snapshot = %#v, want active snapshot %#v", loaded.History[0].Resolved, loaded.Active)
	}
	if state.Active[0].Reference != ref {
		t.Fatalf("state active reference = %q, want %q", state.Active[0].Reference, ref)
	}
}

func TestInstallRejectsConfigManifestMismatch(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	repository := "registry.example.com/agentskills/list-directory"
	archiveBytes := buildPackageBytes(t, fixtureSkillDir(t))
	badConfig := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"list-directory","version":"9.9.9","skill":{"entrypoint":"SKILL.md"}}`)

	ref, err := oci.Push(context.Background(), registry, repository, badConfig, archiveBytes, oci.PushOptions{Tag: "1.2.3"})
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	_, err = Install(context.Background(), Options{
		Registry:  registry,
		Reference: ref.Canonical(),
		TargetDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("Install() error = nil, want mismatch failure")
	}
	if !strings.Contains(err.Error(), "does not match mirrored manifest") {
		t.Fatalf("Install() error = %q, want mismatch context", err)
	}
}

func TestInstallUpgradeReplacesActiveDirectoryAndAppendsHistory(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	targetDir := t.TempDir()

	firstRef := pushSkill(t, registry, fixtureSkillDir(t), "registry.example.com/agentskills/list-directory")
	firstNow := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	if _, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: firstRef,
		TargetDir: targetDir,
		Now: func() time.Time {
			return firstNow
		},
	}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	secondDir := mutatedSkillDir(t, "2.0.0", "# list-directory\n\nversion two\n")
	secondRef := pushSkill(t, registry, secondDir, "registry.example.com/agentskills/list-directory")
	secondNow := time.Date(2026, 3, 26, 13, 0, 0, 0, time.UTC)
	state, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: secondRef,
		TargetDir: targetDir,
		Now: func() time.Time {
			return secondNow
		},
	})
	if err != nil {
		t.Fatalf("second Install() error = %v", err)
	}

	skillBytes, err := os.ReadFile(filepath.Join(targetDir, "list-directory", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(active SKILL.md) error = %v", err)
	}
	if !strings.Contains(string(skillBytes), "version two") {
		t.Fatalf("SKILL.md = %q, want upgraded contents", string(skillBytes))
	}

	if len(state.History) != 2 {
		t.Fatalf("history length = %d, want 2", len(state.History))
	}
	if state.History[0].Action != "install" || state.History[1].Action != "upgrade" {
		t.Fatalf("history actions = %#v, want install then upgrade", state.History)
	}
	if !equalResolvedSets(state.Active, state.History[len(state.History)-1].Resolved) {
		t.Fatalf("newest history = %#v, want active %#v", state.History[len(state.History)-1].Resolved, state.Active)
	}
	if state.Active[0].Version != "2.0.0" {
		t.Fatalf("active version = %q, want %q", state.Active[0].Version, "2.0.0")
	}
}

func TestInstallRejectsMalformedExistingInstallState(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	targetDir := t.TempDir()
	ref := pushSkill(t, registry, fixtureSkillDir(t), "registry.example.com/agentskills/list-directory")

	badState := State{
		SchemaVersion: SchemaVersion,
		Target:        Target{Path: targetDir},
		Active: []ResolvedSkill{
			{
				Name:      "list-directory",
				Version:   "1.2.3",
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Reference: "oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
		History: []HistoryEntry{
			{
				Timestamp: "2026-03-26T11:00:00Z",
				Action:    "install",
				Resolved: []ResolvedSkill{
					{
						Name:      "list-directory",
						Version:   "1.2.2",
						Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
						Reference: "oci://registry.example.com/agentskills/list-directory@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					},
				},
			},
		},
	}
	if err := WriteState(targetDir, badState); err == nil {
		t.Fatal("WriteState() error = nil, want semantic validation failure")
	}

	statePath := StatePath(targetDir)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(state dir) error = %v", err)
	}
	raw := `{
  "schemaVersion": "v1",
  "target": {"path": "` + targetDir + `"},
  "active": [
    {
      "name": "list-directory",
      "version": "1.2.3",
      "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "reference": "oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  ],
  "history": [
    {
      "timestamp": "2026-03-26T11:00:00Z",
      "action": "install",
      "resolved": [
        {
          "name": "list-directory",
          "version": "1.2.2",
          "digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "reference": "oci://registry.example.com/agentskills/list-directory@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(statePath, []byte(raw), 0o644); err != nil {
		t.Fatalf("WriteFile(bad state) error = %v", err)
	}

	_, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: ref,
		TargetDir: targetDir,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want malformed existing state failure")
	}
	if !strings.Contains(err.Error(), "active does not match newest history snapshot") {
		t.Fatalf("Install() error = %q, want semantic validation context", err)
	}
}

func TestWriteStateAcceptsEquivalentActiveAndHistorySetsInDifferentOrders(t *testing.T) {
	targetDir := t.TempDir()
	state := State{
		SchemaVersion: SchemaVersion,
		Target:        Target{Path: targetDir},
		Active: []ResolvedSkill{
			{
				Name:      "root",
				Version:   "1.0.0",
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Reference: "oci://registry.example.com/agentskills/root@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				Name:      "child",
				Version:   "1.0.0",
				Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Reference: "oci://registry.example.com/agentskills/child@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		History: []HistoryEntry{
			{
				Timestamp: "2026-03-26T11:00:00Z",
				Action:    "install",
				Resolved: []ResolvedSkill{
					{
						Name:      "child",
						Version:   "1.0.0",
						Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
						Reference: "oci://registry.example.com/agentskills/child@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					},
					{
						Name:      "root",
						Version:   "1.0.0",
						Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
						Reference: "oci://registry.example.com/agentskills/root@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					},
				},
			},
		},
	}

	if err := WriteState(targetDir, state); err != nil {
		t.Fatalf("WriteState() error = %v", err)
	}

	loaded, err := LoadState(targetDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if !equalResolvedSets(loaded.Active, state.Active) {
		t.Fatalf("loaded active = %#v, want %#v", loaded.Active, state.Active)
	}
}

func TestInstallRollsBackActivationWhenStateWriteFails(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	targetDir := t.TempDir()

	firstRef := pushSkill(t, registry, fixtureSkillDir(t), "registry.example.com/agentskills/list-directory")
	if _, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: firstRef,
		TargetDir: targetDir,
	}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	secondDir := mutatedSkillDir(t, "2.0.0", "# list-directory\n\nversion two\n")
	secondRef := pushSkill(t, registry, secondDir, "registry.example.com/agentskills/list-directory")

	_, err := Install(context.Background(), Options{
		Registry:    registry,
		Reference:   secondRef,
		TargetDir:   targetDir,
		StateWriter: func(string, State) error { return errors.New("boom") },
	})
	if err == nil {
		t.Fatal("Install() error = nil, want state write failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Install() error = %q, want state write failure context", err)
	}

	skillBytes, err := os.ReadFile(filepath.Join(targetDir, "list-directory", "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(active SKILL.md) error = %v", err)
	}
	if strings.Contains(string(skillBytes), "version two") {
		t.Fatalf("SKILL.md = %q, want rollback to preserve previous contents", string(skillBytes))
	}
}

func TestInstallFailsWhenBackupCleanupFails(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	targetDir := t.TempDir()

	firstRef := pushSkill(t, registry, fixtureSkillDir(t), "registry.example.com/agentskills/list-directory")
	if _, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: firstRef,
		TargetDir: targetDir,
	}); err != nil {
		t.Fatalf("first Install() error = %v", err)
	}

	secondDir := mutatedSkillDir(t, "2.0.0", "# list-directory\n\nversion two\n")
	secondRef := pushSkill(t, registry, secondDir, "registry.example.com/agentskills/list-directory")

	previousCommit := commitActivations
	commitActivations = func([]*Activation) error {
		return errors.New("cleanup failed")
	}
	defer func() {
		commitActivations = previousCommit
	}()

	_, err := Install(context.Background(), Options{
		Registry:  registry,
		Reference: secondRef,
		TargetDir: targetDir,
	})
	if err == nil {
		t.Fatal("Install() error = nil, want cleanup failure")
	}
	if !strings.Contains(err.Error(), "install succeeded but cleanup failed") {
		t.Fatalf("Install() error = %q, want cleanup failure context", err)
	}

	skillBytes, readErr := os.ReadFile(filepath.Join(targetDir, "list-directory", "SKILL.md"))
	if readErr != nil {
		t.Fatalf("ReadFile(active SKILL.md) error = %v", readErr)
	}
	if !strings.Contains(string(skillBytes), "version two") {
		t.Fatalf("SKILL.md = %q, want upgraded contents despite cleanup failure", string(skillBytes))
	}

	loaded, loadErr := LoadState(targetDir)
	if loadErr != nil {
		t.Fatalf("LoadState() error = %v", loadErr)
	}
	if len(loaded.Active) != 1 || loaded.Active[0].Version != "2.0.0" {
		t.Fatalf("loaded active = %#v, want upgraded active state", loaded.Active)
	}
}

func TestInstallGraph(t *testing.T) {
	t.Run("activates resolved graph and preserves unrelated active skills", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		unrelatedRef := pushSkill(t, registry, namedSkillDir(t, "notes", "0.1.0", "# notes\n", nil), "registry.example.com/agentskills/notes")
		if _, err := Install(context.Background(), Options{
			Registry:  registry,
			Reference: unrelatedRef,
			TargetDir: targetDir,
		}); err != nil {
			t.Fatalf("Install(unrelated) error = %v", err)
		}

		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		graph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")

		state, err := Install(context.Background(), Options{
			Registry:  registry,
			Graph:     &graph,
			TargetDir: targetDir,
		})
		if err != nil {
			t.Fatalf("Install(graph) error = %v", err)
		}

		for _, name := range []string{"notes", "root", "child"} {
			if _, err := os.Stat(filepath.Join(targetDir, name, "SKILL.md")); err != nil {
				t.Fatalf("Stat(%s/SKILL.md) error = %v", name, err)
			}
		}

		if len(state.Active) != 3 {
			t.Fatalf("len(state.Active) = %d, want 3", len(state.Active))
		}
		if !equalResolvedSets(state.Active, state.History[len(state.History)-1].Resolved) {
			t.Fatalf("newest history = %#v, want active %#v", state.History[len(state.History)-1].Resolved, state.Active)
		}
	})

	t.Run("preserves preexisting unmanaged skill directories", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		unmanagedDir := filepath.Join(targetDir, "manual-skill")
		if err := os.MkdirAll(unmanagedDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(manual-skill) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(unmanagedDir, "SKILL.md"), []byte("# manual\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(manual SKILL.md) error = %v", err)
		}

		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		graph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")

		state, err := Install(context.Background(), Options{
			Registry:  registry,
			Graph:     &graph,
			TargetDir: targetDir,
		})
		if err != nil {
			t.Fatalf("Install(graph) error = %v", err)
		}

		for _, name := range []string{"manual-skill", "root", "child"} {
			if _, err := os.Stat(filepath.Join(targetDir, name, "SKILL.md")); err != nil {
				t.Fatalf("Stat(%s/SKILL.md) error = %v", name, err)
			}
		}
		if len(state.Active) != 2 {
			t.Fatalf("len(state.Active) = %d, want 2 managed skills", len(state.Active))
		}
	})

	t.Run("replaces selected dependency and keeps full active snapshot in state", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		firstGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		if _, err := Install(context.Background(), Options{Registry: registry, Graph: &firstGraph, TargetDir: targetDir}); err != nil {
			t.Fatalf("Install(first graph) error = %v", err)
		}

		pushResolvedSkill(t, registry, "child", "2.0.0", nil)
		pushResolvedSkill(t, registry, "root", "2.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^2.0.0"}})
		secondGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		state, err := Install(context.Background(), Options{Registry: registry, Graph: &secondGraph, TargetDir: targetDir})
		if err != nil {
			t.Fatalf("Install(second graph) error = %v", err)
		}

		if got := activeSkillVersion(t, state.Active, "child"); got != "2.0.0" {
			t.Fatalf("active child version = %q, want %q", got, "2.0.0")
		}
		if len(state.History) != 2 {
			t.Fatalf("len(state.History) = %d, want 2", len(state.History))
		}
		for _, skill := range state.History[len(state.History)-1].Resolved {
			if skill.Reference == "" || skill.Digest == "" {
				t.Fatalf("history entry = %#v, want intact reference and digest", skill)
			}
		}
	})

	t.Run("config mismatch in any fetched artifact fails before activation", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		rootRef := pushSkill(t, registry, namedSkillDir(t, "root", "1.0.0", "# root\n", nil), "registry.example.com/agentskills/root")
		badArchive := buildPackageBytes(t, namedSkillDir(t, "child", "1.0.0", "# child\n", nil))
		badConfig := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"child","version":"9.9.9","skill":{"entrypoint":"SKILL.md"}}`)
		badRef, err := oci.Push(context.Background(), registry, "registry.example.com/agentskills/child", badConfig, badArchive, oci.PushOptions{Tag: "1.0.0"})
		if err != nil {
			t.Fatalf("Push(bad child) error = %v", err)
		}

		graph := resolve.Graph{
			Root: "root",
			Packages: map[string]resolve.SelectedPackage{
				"root": {
					Name:      "root",
					Version:   "1.0.0",
					Reference: rootRef,
					Digest:    mustParseReference(rootRef).Digest,
				},
				"child": {
					Name:      "child",
					Version:   "1.0.0",
					Reference: badRef.Canonical(),
					Digest:    badRef.Digest,
				},
			},
			Edges: map[string][]string{"root": []string{"child"}},
		}

		_, err = Install(context.Background(), Options{Registry: registry, Graph: &graph, TargetDir: targetDir})
		if err == nil || !strings.Contains(err.Error(), "does not match mirrored manifest") {
			t.Fatalf("Install(graph) error = %v, want config mismatch failure", err)
		}
		if _, statErr := os.Stat(filepath.Join(targetDir, "root")); !os.IsNotExist(statErr) {
			t.Fatalf("root activation should not exist, stat error = %v", statErr)
		}
	})
}

func TestRestoreOnStateWriteFailure(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	targetDir := t.TempDir()

	pushResolvedSkill(t, registry, "child", "1.0.0", nil)
	pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
	firstGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
	if _, err := Install(context.Background(), Options{Registry: registry, Graph: &firstGraph, TargetDir: targetDir}); err != nil {
		t.Fatalf("Install(first graph) error = %v", err)
	}
	beforeState, err := LoadState(targetDir)
	if err != nil {
		t.Fatalf("LoadState(before) error = %v", err)
	}

	pushResolvedSkill(t, registry, "child", "2.0.0", nil)
	pushResolvedSkill(t, registry, "root", "2.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^2.0.0"}})
	secondGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")

	_, err = Install(context.Background(), Options{
		Registry:    registry,
		Graph:       &secondGraph,
		TargetDir:   targetDir,
		StateWriter: func(string, State) error { return errors.New("boom") },
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Install(graph) error = %v, want state write failure", err)
	}

	afterState, err := LoadState(targetDir)
	if err != nil {
		t.Fatalf("LoadState(after) error = %v", err)
	}
	if !equalResolvedSets(beforeState.Active, afterState.Active) {
		t.Fatalf("active after rollback = %#v, want %#v", afterState.Active, beforeState.Active)
	}
	if got := activeSkillVersion(t, afterState.Active, "child"); got != "1.0.0" {
		t.Fatalf("active child version after rollback = %q, want %q", got, "1.0.0")
	}
}

func TestInstallFromLockfile(t *testing.T) {
	t.Run("activates lockfile graph and preserves unrelated active skills", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		unrelatedRef := pushSkill(t, registry, namedSkillDir(t, "notes", "0.1.0", "# notes\n", nil), "registry.example.com/agentskills/notes")
		if _, err := Install(context.Background(), Options{
			Registry:  registry,
			Reference: unrelatedRef,
			TargetDir: targetDir,
		}); err != nil {
			t.Fatalf("Install(unrelated) error = %v", err)
		}

		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		liveGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		lock, err := lockfilepkg.FromGraph(liveGraph)
		if err != nil {
			t.Fatalf("FromGraph() error = %v", err)
		}
		graph, err := lockfilepkg.ToGraph(lock)
		if err != nil {
			t.Fatalf("ToGraph() error = %v", err)
		}

		state, err := Install(context.Background(), Options{
			Registry:  registry,
			Graph:     &graph,
			TargetDir: targetDir,
		})
		if err != nil {
			t.Fatalf("Install(lockfile graph) error = %v", err)
		}

		for _, name := range []string{"notes", "root", "child"} {
			if _, err := os.Stat(filepath.Join(targetDir, name, "SKILL.md")); err != nil {
				t.Fatalf("Stat(%s/SKILL.md) error = %v", name, err)
			}
		}
		if len(state.Active) != 3 {
			t.Fatalf("len(state.Active) = %d, want 3", len(state.Active))
		}
		if !equalResolvedSets(state.Active, state.History[len(state.History)-1].Resolved) {
			t.Fatalf("newest history = %#v, want active %#v", state.History[len(state.History)-1].Resolved, state.Active)
		}
	})

	t.Run("preserves preexisting unmanaged skill directories during lockfile install", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		unmanagedDir := filepath.Join(targetDir, "manual-skill")
		if err := os.MkdirAll(unmanagedDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(manual-skill) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(unmanagedDir, "SKILL.md"), []byte("# manual\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(manual SKILL.md) error = %v", err)
		}

		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		liveGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		lock, err := lockfilepkg.FromGraph(liveGraph)
		if err != nil {
			t.Fatalf("FromGraph() error = %v", err)
		}
		graph, err := lockfilepkg.ToGraph(lock)
		if err != nil {
			t.Fatalf("ToGraph() error = %v", err)
		}

		state, err := Install(context.Background(), Options{
			Registry:  registry,
			Graph:     &graph,
			TargetDir: targetDir,
		})
		if err != nil {
			t.Fatalf("Install(lockfile graph) error = %v", err)
		}

		for _, name := range []string{"manual-skill", "root", "child"} {
			if _, err := os.Stat(filepath.Join(targetDir, name, "SKILL.md")); err != nil {
				t.Fatalf("Stat(%s/SKILL.md) error = %v", name, err)
			}
		}
		if len(state.Active) != 2 {
			t.Fatalf("len(state.Active) = %d, want 2 managed skills", len(state.Active))
		}
	})

	t.Run("fetched artifact version mismatch against lockfile metadata fails", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		liveGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		lock, err := lockfilepkg.FromGraph(liveGraph)
		if err != nil {
			t.Fatalf("FromGraph() error = %v", err)
		}
		for i := range lock.Packages {
			if lock.Packages[i].Name == "child" {
				lock.Packages[i].Version = "9.9.9"
			}
		}
		graph, err := lockfilepkg.ToGraph(lock)
		if err != nil {
			t.Fatalf("ToGraph() error = %v", err)
		}

		_, err = Install(context.Background(), Options{Registry: registry, Graph: &graph, TargetDir: targetDir})
		if err == nil {
			t.Fatal("Install(lockfile graph) error = nil, want version mismatch failure")
		}
		if !strings.Contains(err.Error(), "does not match expected selected package") {
			t.Fatalf("Install(lockfile graph) error = %q, want selected package mismatch context", err)
		}
	})

	t.Run("fetched artifact digest mismatch against lockfile metadata fails", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		liveGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		lock, err := lockfilepkg.FromGraph(liveGraph)
		if err != nil {
			t.Fatalf("FromGraph() error = %v", err)
		}
		graph, err := lockfilepkg.ToGraph(lock)
		if err != nil {
			t.Fatalf("ToGraph() error = %v", err)
		}
		selected := graph.Packages["child"]
		selected.Digest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
		graph.Packages["child"] = selected

		_, err = Install(context.Background(), Options{Registry: registry, Graph: &graph, TargetDir: targetDir})
		if err == nil {
			t.Fatal("Install(lockfile graph) error = nil, want digest mismatch failure")
		}
		if !strings.Contains(err.Error(), "does not match expected selected package") && !strings.Contains(err.Error(), "digest") {
			t.Fatalf("Install(lockfile graph) error = %q, want digest mismatch context", err)
		}
	})
}

func TestRollback(t *testing.T) {
	t.Run("restores the immediately previous snapshot and appends rollback history", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		firstGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		firstTime := time.Date(2026, 3, 30, 15, 0, 0, 0, time.UTC)
		if _, err := Install(context.Background(), Options{
			Registry:  registry,
			Graph:     &firstGraph,
			TargetDir: targetDir,
			Now: func() time.Time {
				return firstTime
			},
		}); err != nil {
			t.Fatalf("Install(first graph) error = %v", err)
		}

		pushResolvedSkill(t, registry, "child", "2.0.0", nil)
		pushResolvedSkill(t, registry, "root", "2.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^2.0.0"}})
		secondGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		secondTime := time.Date(2026, 3, 30, 16, 0, 0, 0, time.UTC)
		if _, err := Install(context.Background(), Options{
			Registry:  registry,
			Graph:     &secondGraph,
			TargetDir: targetDir,
			Now: func() time.Time {
				return secondTime
			},
		}); err != nil {
			t.Fatalf("Install(second graph) error = %v", err)
		}

		rollbackTime := time.Date(2026, 3, 30, 17, 0, 0, 0, time.UTC)
		state, err := Rollback(context.Background(), Options{
			Registry:  registry,
			TargetDir: targetDir,
			Now: func() time.Time {
				return rollbackTime
			},
		})
		if err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}

		if got := activeSkillVersion(t, state.Active, "child"); got != "1.0.0" {
			t.Fatalf("active child version after rollback = %q, want %q", got, "1.0.0")
		}
		if got := activeSkillVersion(t, state.Active, "root"); got != "1.0.0" {
			t.Fatalf("active root version after rollback = %q, want %q", got, "1.0.0")
		}
		if len(state.History) != 3 {
			t.Fatalf("len(state.History) = %d, want 3", len(state.History))
		}
		if got := state.History[2].Action; got != "rollback" {
			t.Fatalf("state.History[2].Action = %q, want %q", got, "rollback")
		}
		if got := state.History[2].Timestamp; got != rollbackTime.Format(time.RFC3339) {
			t.Fatalf("state.History[2].Timestamp = %q, want %q", got, rollbackTime.Format(time.RFC3339))
		}
		if !equalResolvedSets(state.Active, state.History[2].Resolved) {
			t.Fatalf("newest history = %#v, want active %#v", state.History[2].Resolved, state.Active)
		}
	})

	t.Run("re-fetches the previous snapshot artifacts by canonical reference", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		firstGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		if _, err := Install(context.Background(), Options{Registry: registry, Graph: &firstGraph, TargetDir: targetDir}); err != nil {
			t.Fatalf("Install(first graph) error = %v", err)
		}

		pushResolvedSkill(t, registry, "child", "2.0.0", nil)
		pushResolvedSkill(t, registry, "root", "2.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^2.0.0"}})
		secondGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		if _, err := Install(context.Background(), Options{Registry: registry, Graph: &secondGraph, TargetDir: targetDir}); err != nil {
			t.Fatalf("Install(second graph) error = %v", err)
		}

		stateBeforeRollback, err := LoadState(targetDir)
		if err != nil {
			t.Fatalf("LoadState(before rollback) error = %v", err)
		}
		firstSnapshot := stateBeforeRollback.History[0].Resolved
		for _, skill := range firstSnapshot {
			ref, err := oci.ParseReference(skill.Reference)
			if err != nil {
				t.Fatalf("ParseReference(%q) error = %v", skill.Reference, err)
			}
			registry.Delete(ref.Repository, ref.Digest)
		}

		_, err = Rollback(context.Background(), Options{Registry: registry, TargetDir: targetDir})
		if err == nil {
			t.Fatal("Rollback() error = nil, want fetch failure after old artifacts are removed")
		}
		if !strings.Contains(err.Error(), "artifact") && !strings.Contains(err.Error(), "not found") {
			t.Fatalf("Rollback() error = %q, want fetch failure context", err)
		}
	})

	t.Run("fails when there is no earlier snapshot", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		ref := pushSkill(t, registry, fixtureSkillDir(t), "registry.example.com/agentskills/list-directory")
		if _, err := Install(context.Background(), Options{
			Registry:  registry,
			Reference: ref,
			TargetDir: targetDir,
		}); err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		_, err := Rollback(context.Background(), Options{Registry: registry, TargetDir: targetDir})
		if err == nil {
			t.Fatal("Rollback() error = nil, want missing previous snapshot failure")
		}
		if !strings.Contains(err.Error(), "no earlier history snapshot") {
			t.Fatalf("Rollback() error = %q, want missing previous snapshot context", err)
		}
	})

	t.Run("fails when install state is malformed", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		statePath := StatePath(targetDir)
		if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
			t.Fatalf("MkdirAll(state dir) error = %v", err)
		}
		raw := `{
  "schemaVersion": "v1",
  "target": {"path": "` + targetDir + `"},
  "active": [],
  "history": [
    {
      "timestamp": "2026-03-30T15:00:00Z",
      "action": "install",
      "resolved": [
        {
          "name": "root",
          "version": "1.0.0",
          "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "reference": "oci://registry.example.com/agentskills/root@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        }
      ]
    }
  ]
}`
		if err := os.WriteFile(statePath, []byte(raw), 0o644); err != nil {
			t.Fatalf("WriteFile(bad state) error = %v", err)
		}

		_, err := Rollback(context.Background(), Options{Registry: registry, TargetDir: targetDir})
		if err == nil {
			t.Fatal("Rollback() error = nil, want malformed state failure")
		}
		if !strings.Contains(err.Error(), "active does not match newest history snapshot") {
			t.Fatalf("Rollback() error = %q, want semantic validation context", err)
		}
	})

	t.Run("accepts install-state history even when timestamps are not monotonic", func(t *testing.T) {
		targetDir := t.TempDir()

		statePath := StatePath(targetDir)
		if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
			t.Fatalf("MkdirAll(state dir) error = %v", err)
		}
		raw := `{
  "schemaVersion": "v1",
  "target": {"path": "` + targetDir + `"},
  "active": [
    {
      "name": "root",
      "version": "1.0.0",
      "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "reference": "oci://registry.example.com/agentskills/root@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    }
  ],
  "history": [
    {
      "timestamp": "2026-03-30T16:00:00Z",
      "action": "install",
      "resolved": [
        {
          "name": "root",
          "version": "2.0.0",
          "digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
          "reference": "oci://registry.example.com/agentskills/root@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
        }
      ]
    },
    {
      "timestamp": "2026-03-30T15:00:00Z",
      "action": "rollback",
      "resolved": [
        {
          "name": "root",
          "version": "1.0.0",
          "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
          "reference": "oci://registry.example.com/agentskills/root@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
        }
      ]
    }
  ]
}`
		if err := os.WriteFile(statePath, []byte(raw), 0o644); err != nil {
			t.Fatalf("WriteFile(bad state) error = %v", err)
		}

		state, err := LoadState(targetDir)
		if err != nil {
			t.Fatalf("LoadState() error = %v", err)
		}
		if len(state.History) != 2 {
			t.Fatalf("len(state.History) = %d, want 2", len(state.History))
		}
	})

	t.Run("restores the pre-rollback active view when state writing fails", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		targetDir := t.TempDir()

		pushResolvedSkill(t, registry, "child", "1.0.0", nil)
		pushResolvedSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		firstGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		if _, err := Install(context.Background(), Options{Registry: registry, Graph: &firstGraph, TargetDir: targetDir}); err != nil {
			t.Fatalf("Install(first graph) error = %v", err)
		}

		pushResolvedSkill(t, registry, "child", "2.0.0", nil)
		pushResolvedSkill(t, registry, "root", "2.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^2.0.0"}})
		secondGraph := resolveGraphForInstall(t, registry, "root", "registry.example.com/agentskills")
		if _, err := Install(context.Background(), Options{Registry: registry, Graph: &secondGraph, TargetDir: targetDir}); err != nil {
			t.Fatalf("Install(second graph) error = %v", err)
		}

		beforeRollback, err := LoadState(targetDir)
		if err != nil {
			t.Fatalf("LoadState(before rollback) error = %v", err)
		}

		_, err = Rollback(context.Background(), Options{
			Registry:    registry,
			TargetDir:   targetDir,
			StateWriter: func(string, State) error { return errors.New("boom") },
		})
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("Rollback() error = %v, want state write failure", err)
		}

		afterFailure, err := LoadState(targetDir)
		if err != nil {
			t.Fatalf("LoadState(after failure) error = %v", err)
		}
		if !equalResolvedSets(beforeRollback.Active, afterFailure.Active) {
			t.Fatalf("active after failed rollback = %#v, want %#v", afterFailure.Active, beforeRollback.Active)
		}
		if got := activeSkillVersion(t, afterFailure.Active, "child"); got != "2.0.0" {
			t.Fatalf("active child version after failed rollback = %q, want %q", got, "2.0.0")
		}
	})
}

func pushSkill(t *testing.T, registry *oci.MemoryRegistry, skillDir, repository string) string {
	t.Helper()

	archiveBytes := buildPackageBytes(t, skillDir)
	mirroredManifestBytes, _, err := archivepkg.ReadMirroredManifestTGZ(bytes.NewReader(archiveBytes))
	if err != nil {
		t.Fatalf("ReadMirroredManifestTGZ() error = %v", err)
	}
	var mirroredManifest manifestpkg.Manifest
	if err := json.Unmarshal(mirroredManifestBytes, &mirroredManifest); err != nil {
		t.Fatalf("json.Unmarshal(mirrored manifest) error = %v", err)
	}

	ref, err := oci.Push(context.Background(), registry, repository, mirroredManifestBytes, archiveBytes, oci.PushOptions{Tag: mirroredManifest.Version})
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	return ref.Canonical()
}

func pushResolvedSkill(t *testing.T, registry *oci.MemoryRegistry, name, version string, dependencies []manifestpkg.Dependency) string {
	t.Helper()
	return pushSkill(t, registry, namedSkillDir(t, name, version, "# "+name+"\n", dependencies), "registry.example.com/agentskills/"+name)
}

func resolveGraphForInstall(t *testing.T, registry *oci.MemoryRegistry, rootName, base string) resolve.Graph {
	t.Helper()

	root, err := resolve.NewNamedRoot(rootName, base)
	if err != nil {
		t.Fatalf("NewNamedRoot() error = %v", err)
	}
	graph, err := resolve.ResolveGraph(context.Background(), registry, root)
	if err != nil {
		t.Fatalf("ResolveGraph() error = %v", err)
	}
	return graph
}

func buildPackageBytes(t *testing.T, skillDir string) []byte {
	t.Helper()

	var archive bytes.Buffer
	if err := packagex.BuildTGZ(&archive, skillDir, packagex.BuildOptions{
		IncludeFilesSHA256: true,
	}); err != nil {
		t.Fatalf("BuildTGZ() error = %v", err)
	}

	return archive.Bytes()
}

func fixtureSkillDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) = !ok")
	}

	return filepath.Join(filepath.Dir(filename), "../../../testdata/skills/list-directory")
}

func mutatedSkillDir(t *testing.T, version, skillBody string) string {
	t.Helper()

	parent := t.TempDir()
	root := filepath.Join(parent, "list-directory")
	if err := copyDir(fixtureSkillDir(t), root); err != nil {
		t.Fatalf("copyDir() error = %v", err)
	}

	manifestPath := filepath.Join(root, ".skill", "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile(manifest) error = %v", err)
	}
	manifestBytes = bytes.Replace(manifestBytes, []byte(`"version": "1.2.3"`), []byte(`"version": "`+version+`"`), 1)
	if err := os.WriteFile(manifestPath, manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(skillBody), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	return root
}

func namedSkillDir(t *testing.T, name, version, skillBody string, dependencies []manifestpkg.Dependency) string {
	t.Helper()

	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, ".skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.skill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(skillBody), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	manifestBytes, err := json.MarshalIndent(manifestpkg.Manifest{
		SchemaVersion: "v1",
		PackageType:   "skill",
		Name:          name,
		Version:       version,
		Skill:         manifestpkg.Skill{Entrypoint: "SKILL.md"},
		Dependencies:  dependencies,
	}, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(manifest) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".skill", "manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	return root
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode().Perm())
	})
}

func equalResolvedSets(left, right []ResolvedSkill) bool {
	if len(left) != len(right) {
		return false
	}
	byName := make(map[string]ResolvedSkill, len(left))
	for _, entry := range left {
		byName[entry.Name] = entry
	}
	for _, entry := range right {
		if got, ok := byName[entry.Name]; !ok || got != entry {
			return false
		}
	}
	return true
}

func activeSkillVersion(t *testing.T, active []ResolvedSkill, name string) string {
	t.Helper()
	for _, skill := range active {
		if skill.Name == name {
			return skill.Version
		}
	}
	t.Fatalf("active skill %q not found in %#v", name, active)
	return ""
}
