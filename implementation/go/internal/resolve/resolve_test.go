package resolve

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func TestSelectVersion(t *testing.T) {
	t.Run("filters out non-semver tags", func(t *testing.T) {
		got, err := SelectVersion([]string{"latest", "1.2.0", "dev", "1.1.9"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.0")
		}
	})

	t.Run("ignores invalid tag forms such as leading v and partial bare versions", func(t *testing.T) {
		got, err := SelectVersion([]string{"v1.2.3", "1.2", "1.2.3"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.3" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.3")
		}
	})

	t.Run("chooses the highest satisfying stable version", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.4.0", "1.2.3", "1.3.9", "2.0.0"}, ">=1.2.0 <2.0.0")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.4.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.4.0")
		}
	})

	t.Run("accepts bare exact version constraints", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.2.4", "1.2.3", "1.2.2"}, "1.2.3")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.3" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.3")
		}
	})

	t.Run("refuses prereleases unless explicitly admitted", func(t *testing.T) {
		_, err := SelectVersion([]string{"1.3.0-beta.1", "1.3.0-rc.1"}, ">=1.3.0 <2.0.0")
		if err == nil {
			t.Fatal("SelectVersion() error = nil, want prerelease rejection")
		}
	})

	t.Run("prefers a stable release over a prerelease with the same base version", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.3.0-rc.1", "1.3.0", "1.2.9"}, ">=1.3.0-0 <2.0.0")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.3.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.3.0")
		}
	})

	t.Run("allows unconstrained prerelease-only selection", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.3.0-beta.1", "1.3.0-alpha.2"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.3.0-beta.1" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.3.0-beta.1")
		}
	})
}

func TestParseConstraint(t *testing.T) {
	t.Run("invalid empty constraint strings", func(t *testing.T) {
		if _, err := ParseConstraint(" \t "); err == nil {
			t.Fatal("ParseConstraint() error = nil, want invalid empty constraint")
		}
	})

	t.Run("invalid leading v constraint versions", func(t *testing.T) {
		if _, err := ParseConstraint("v1.2.3"); err == nil {
			t.Fatal("ParseConstraint() error = nil, want invalid leading v")
		}
	})

	t.Run("invalid unsupported shorthand and coercions", func(t *testing.T) {
		for _, raw := range []string{"=>1.2.3", "=<1.2.3", "~>1.2.3", ">=1.0.0, <2.0.0"} {
			if _, err := ParseConstraint(raw); err == nil {
				t.Fatalf("ParseConstraint(%q) error = nil, want invalid shorthand/coercion", raw)
			}
		}
	})

	t.Run("valid comparator sets", func(t *testing.T) {
		if _, err := ParseConstraint(">=1.0.0 <2.0.0"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})

	t.Run("valid caret ranges", func(t *testing.T) {
		if _, err := ParseConstraint("^1.2.3"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})

	t.Run("valid tilde ranges", func(t *testing.T) {
		if _, err := ParseConstraint("~1.4.2"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})

	t.Run("valid OR expressions", func(t *testing.T) {
		if _, err := ParseConstraint(">=1.0.0 <2.0.0 || ^3.0.0"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})
}

func TestMergeConstraints(t *testing.T) {
	t.Run("merged compatible constraints", func(t *testing.T) {
		merged, err := MergeConstraints(">=1.0.0 <2.0.0", "^1.2.3")
		if err != nil {
			t.Fatalf("MergeConstraints() error = %v", err)
		}

		if !merged.Check("1.5.0") {
			t.Fatal("merged constraint should accept 1.5.0")
		}
		if merged.Check("2.0.0") {
			t.Fatal("merged constraint should reject 2.0.0")
		}
	})

	t.Run("merged incompatible constraints", func(t *testing.T) {
		merged, err := MergeConstraints(">=1.0.0 <2.0.0", "^2.1.0")
		if err != nil {
			t.Fatalf("MergeConstraints() error = %v", err)
		}

		if merged.Check("1.5.0") {
			t.Fatal("merged constraint should reject 1.5.0")
		}
		if merged.Check("2.1.0") {
			t.Fatal("merged constraint should reject 2.1.0")
		}
	})
}

func TestRootForms(t *testing.T) {
	t.Run("exact root-form invariants", func(t *testing.T) {
		root, err := NewExactRoot("oci://registry.example.com/agentskills/python-development@sha256:abc")
		if err != nil {
			t.Fatalf("NewExactRoot() error = %v", err)
		}

		if got := root.Reference(); got != "oci://registry.example.com/agentskills/python-development@sha256:abc" {
			t.Fatalf("Reference() = %q, want exact reference", got)
		}
		if root.Name() != "" {
			t.Fatalf("Name() = %q, want empty", root.Name())
		}
		if root.OCIBase() != "" {
			t.Fatalf("OCIBase() = %q, want empty", root.OCIBase())
		}
		if !root.IsExact() {
			t.Fatal("IsExact() = false, want true")
		}
	})

	t.Run("named root-form invariants", func(t *testing.T) {
		root, err := NewNamedRoot("python-development", "registry.example.com/agentskills")
		if err != nil {
			t.Fatalf("NewNamedRoot() error = %v", err)
		}

		if got := root.Name(); got != "python-development" {
			t.Fatalf("Name() = %q, want %q", got, "python-development")
		}
		if got := root.OCIBase(); got != "registry.example.com/agentskills" {
			t.Fatalf("OCIBase() = %q, want %q", got, "registry.example.com/agentskills")
		}
		if root.Reference() != "" {
			t.Fatalf("Reference() = %q, want empty", root.Reference())
		}
		if root.IsExact() {
			t.Fatal("IsExact() = true, want false")
		}
	})
}

func TestResolveGraph(t *testing.T) {
	t.Run("root with one required dependency", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/child", manifestpkg.Manifest{
			SchemaVersion: "v1",
			PackageType:   "skill",
			Name:          "child",
			Version:       "1.0.0",
			Skill:         manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1",
			PackageType:   "skill",
			Name:          "root",
			Version:       "1.0.0",
			Skill:         manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{
				{Name: "child", Version: "^1.0.0"},
			},
		})

		graph, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err != nil {
			t.Fatalf("ResolveGraph() error = %v", err)
		}

		assertGraphPackages(t, graph, "root", "child")
		assertGraphEdges(t, graph, "root", "child")
		if got := graph.Packages["root"].Reference; got != rootRef.Canonical() {
			t.Fatalf("root reference = %q, want exact root ref %q", got, rootRef.Canonical())
		}
	})

	t.Run("transitive dependency chain", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/grandchild", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "grandchild", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/child", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "child", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "grandchild", Version: "^1.0.0"}},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}},
		})

		graph, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err != nil {
			t.Fatalf("ResolveGraph() error = %v", err)
		}

		assertGraphPackages(t, graph, "root", "child", "grandchild")
		assertGraphEdges(t, graph, "root", "child")
		assertGraphEdges(t, graph, "child", "grandchild")
	})

	t.Run("named root resolves highest available version from base", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.3.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/child", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "child", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})

		root, err := NewNamedRoot("root", "registry.example.com/agentskills")
		if err != nil {
			t.Fatalf("NewNamedRoot() error = %v", err)
		}

		graph, err := ResolveGraph(context.Background(), registry, root)
		if err != nil {
			t.Fatalf("ResolveGraph() error = %v", err)
		}

		if got := graph.Packages["root"].Version; got != "1.3.0" {
			t.Fatalf("root version = %q, want %q", got, "1.3.0")
		}
		assertGraphEdges(t, graph, "root", "child")
	})

	t.Run("shared dependency with compatible constraints", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/shared", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "shared", Version: "1.2.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/shared", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "shared", Version: "1.4.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/left", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "left", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "shared", Version: ">=1.0.0 <2.0.0"}},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/right", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "right", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "shared", Version: "^1.2.0"}},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{
				{Name: "left", Version: "^1.0.0"},
				{Name: "right", Version: "^1.0.0"},
			},
		})

		graph, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err != nil {
			t.Fatalf("ResolveGraph() error = %v", err)
		}

		if got := graph.Packages["shared"].Version; got != "1.4.0" {
			t.Fatalf("shared version = %q, want %q", got, "1.4.0")
		}
		assertGraphEdges(t, graph, "left", "shared")
		assertGraphEdges(t, graph, "right", "shared")
	})

	t.Run("shared dependency merges OR constraints by logical AND", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/shared", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "shared", Version: "1.5.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/shared", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "shared", Version: "3.2.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/left", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "left", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "shared", Version: ">=1.0.0 <2.0.0 || ^3.0.0"}},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/right", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "right", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "shared", Version: "^3.1.0"}},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{
				{Name: "left", Version: "^1.0.0"},
				{Name: "right", Version: "^1.0.0"},
			},
		})

		graph, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err != nil {
			t.Fatalf("ResolveGraph() error = %v", err)
		}

		if got := graph.Packages["shared"].Version; got != "3.2.0" {
			t.Fatalf("shared version = %q, want %q", got, "3.2.0")
		}
	})

	t.Run("shared dependency with incompatible constraints", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/shared", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "shared", Version: "1.5.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/shared", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "shared", Version: "2.1.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/left", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "left", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "shared", Version: ">=1.0.0 <2.0.0"}},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/right", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "right", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "shared", Version: "^2.1.0"}},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{
				{Name: "left", Version: "^1.0.0"},
				{Name: "right", Version: "^1.0.0"},
			},
		})

		_, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err == nil || !strings.Contains(err.Error(), "shared") {
			t.Fatalf("ResolveGraph() error = %v, want incompatible shared constraint failure", err)
		}
	})

	t.Run("dependency repo with only non semver tags fails resolution", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkillWithTag(t, registry, "registry.example.com/agentskills/child", "latest", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "child", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}},
		})

		_, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err == nil || !strings.Contains(err.Error(), "child") {
			t.Fatalf("ResolveGraph() error = %v, want non-semver tag resolution failure", err)
		}
	})

	t.Run("self dependency failure", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "root", Version: "^1.0.0"}},
		})

		_, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err == nil || !strings.Contains(err.Error(), "self-dependency") {
			t.Fatalf("ResolveGraph() error = %v, want self-dependency failure", err)
		}
	})

	t.Run("cycle failure", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		pushGraphSkill(t, registry, "registry.example.com/agentskills/a", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "a", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "b", Version: "^1.0.0"}},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/b", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "b", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "c", Version: "^1.0.0"}},
		})
		rootRef := pushGraphSkill(t, registry, "registry.example.com/agentskills/root", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "root", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "a", Version: "^1.0.0"}},
		})
		pushGraphSkill(t, registry, "registry.example.com/agentskills/c", manifestpkg.Manifest{
			SchemaVersion: "v1", PackageType: "skill", Name: "c", Version: "1.0.0", Skill: manifestpkg.Skill{Entrypoint: "SKILL.md"},
			Dependencies: []manifestpkg.Dependency{{Name: "a", Version: "^1.0.0"}},
		})

		_, err := ResolveGraph(context.Background(), registry, mustExactRoot(t, rootRef.Canonical()))
		if err == nil || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("ResolveGraph() error = %v, want cycle failure", err)
		}
	})
}

func pushGraphSkill(t *testing.T, registry *oci.MemoryRegistry, repository string, manifest manifestpkg.Manifest) oci.Reference {
	t.Helper()
	return pushGraphSkillWithTag(t, registry, repository, manifest.Version, manifest)
}

func pushGraphSkillWithTag(t *testing.T, registry *oci.MemoryRegistry, repository, tag string, manifest manifestpkg.Manifest) oci.Reference {
	t.Helper()

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	ref, err := oci.Push(context.Background(), registry, repository, manifestJSON, []byte(repository+":"+tag+":"+manifest.Version), oci.PushOptions{Tag: tag})
	if err != nil {
		t.Fatalf("oci.Push() error = %v", err)
	}
	return ref
}

func mustExactRoot(t *testing.T, reference string) Root {
	t.Helper()

	root, err := NewExactRoot(reference)
	if err != nil {
		t.Fatalf("NewExactRoot() error = %v", err)
	}
	return root
}

func assertGraphPackages(t *testing.T, graph Graph, names ...string) {
	t.Helper()

	if len(graph.Packages) != len(names) {
		t.Fatalf("len(graph.Packages) = %d, want %d", len(graph.Packages), len(names))
	}
	for _, name := range names {
		if _, ok := graph.Packages[name]; !ok {
			t.Fatalf("graph.Packages missing %q", name)
		}
	}
}

func assertGraphEdges(t *testing.T, graph Graph, from string, tos ...string) {
	t.Helper()

	got := graph.Edges[from]
	if len(got) != len(tos) {
		t.Fatalf("graph.Edges[%q] len = %d, want %d (%v)", from, len(got), len(tos), got)
	}
	for _, want := range tos {
		found := false
		for _, candidate := range got {
			if candidate == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("graph.Edges[%q] = %v, want to contain %q", from, got, want)
		}
	}
}
