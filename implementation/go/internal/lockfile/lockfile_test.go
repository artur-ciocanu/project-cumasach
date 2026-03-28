package lockfile

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
)

func TestLoadReaderValidMinimalLockfile(t *testing.T) {
	lockfileJSON := marshalLockfileJSON(t, validMinimalLockfile())

	loaded, err := LoadReader(strings.NewReader(lockfileJSON))
	if err != nil {
		t.Fatalf("LoadReader() error = %v", err)
	}

	if loaded.SchemaVersion != "v1" {
		t.Fatalf("SchemaVersion = %q, want %q", loaded.SchemaVersion, "v1")
	}
	if loaded.Root.Name != "root-skill" {
		t.Fatalf("Root.Name = %q, want %q", loaded.Root.Name, "root-skill")
	}
	if len(loaded.Packages) != 1 {
		t.Fatalf("len(Packages) = %d, want 1", len(loaded.Packages))
	}
}

func TestLoadReaderRejectsDuplicatePackageNames(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Packages = append(lock.Packages, lock.Packages[0])

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want duplicate package name failure")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("LoadReader() error = %q, want duplicate package name context", err)
	}
}

func TestLoadReaderRejectsMissingRootPackage(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Root.Name = "missing-root"

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want missing root package failure")
	}
	if !strings.Contains(err.Error(), "root") {
		t.Fatalf("LoadReader() error = %q, want root package context", err)
	}
}

func TestLoadReaderRejectsUnknownEdgeEndpoints(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Edges = []Edge{{From: "root-skill", To: "missing-dependency"}}

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want unknown edge endpoint failure")
	}
	if !strings.Contains(err.Error(), "edge") {
		t.Fatalf("LoadReader() error = %q, want edge endpoint context", err)
	}
}

func TestLoadReaderRejectsRootMismatch(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*File)
	}{
		{
			name: "name",
			mut: func(lock *File) {
				lock.Root.Name = "other-skill"
			},
		},
		{
			name: "version",
			mut: func(lock *File) {
				lock.Root.Version = "9.9.9"
			},
		},
		{
			name: "reference",
			mut: func(lock *File) {
				lock.Root.Reference = "oci://registry.example.com/agentskills/root-skill@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			lock := validMinimalLockfile()
			tc.mut(&lock)

			_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
			if err == nil {
				t.Fatal("LoadReader() error = nil, want root mismatch failure")
			}
			if !strings.Contains(err.Error(), "root") {
				t.Fatalf("LoadReader() error = %q, want root mismatch context", err)
			}
		})
	}
}

func TestLoadReaderRejectsInvalidCanonicalReference(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Packages[0].Reference = "registry.example.com/agentskills/root-skill:1.2.3"

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want invalid canonical reference failure")
	}
	if !strings.Contains(err.Error(), "reference") {
		t.Fatalf("LoadReader() error = %q, want reference context", err)
	}
}

func TestLoadReaderRejectsMalformedOCILocatorShape(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Root.Reference = "oci:///agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lock.Packages[0].Reference = lock.Root.Reference

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want malformed OCI locator failure")
	}
	if !strings.Contains(err.Error(), "reference") {
		t.Fatalf("LoadReader() error = %q, want reference context", err)
	}
}

func TestLoadReaderRejectsTagQualifiedDigestReference(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Root.Reference = "oci://registry.example.com/agentskills/root-skill:latest@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	lock.Packages[0].Reference = lock.Root.Reference

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want tag-qualified digest reference failure")
	}
	if !strings.Contains(err.Error(), "reference") {
		t.Fatalf("LoadReader() error = %q, want reference context", err)
	}
}

func TestLoadReaderRejectsPackageDigestReferenceMismatch(t *testing.T) {
	lock := validMinimalLockfile()
	lock.Packages[0].Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want digest/reference mismatch failure")
	}
	if !strings.Contains(err.Error(), "digest") {
		t.Fatalf("LoadReader() error = %q, want digest mismatch context", err)
	}
}

func TestLoadReaderRejectsCyclicGraph(t *testing.T) {
	lock := File{
		SchemaVersion: "v1",
		Root: Root{
			Name:      "root-skill",
			Version:   "1.2.3",
			Reference: "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Packages: []Package{
			{
				Name:      "root-skill",
				Version:   "1.2.3",
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Reference: "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			{
				Name:      "dep-skill",
				Version:   "2.0.0",
				Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Reference: "oci://registry.example.com/agentskills/dep-skill@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
		},
		Edges: []Edge{
			{From: "root-skill", To: "dep-skill"},
			{From: "dep-skill", To: "root-skill"},
		},
	}

	_, err := LoadReader(strings.NewReader(marshalLockfileJSON(t, lock)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want cycle failure")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("LoadReader() error = %q, want cycle context", err)
	}
}

func TestFromGraph(t *testing.T) {
	graph := resolve.Graph{
		Root: "root-skill",
		Packages: map[string]resolve.SelectedPackage{
			"z-dependency": {
				Name:       "z-dependency",
				Version:    "2.0.0",
				Reference:  "registry.example.com/agentskills/z-dependency@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Digest:     "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Repository: "registry.example.com/agentskills/z-dependency",
				Manifest: manifest.Manifest{
					SchemaVersion: "v1",
					PackageType:   "skill",
					Name:          "z-dependency",
					Version:       "2.0.0",
					Skill:         manifest.Skill{Entrypoint: "SKILL.md"},
				},
			},
			"root-skill": {
				Name:       "root-skill",
				Version:    "1.2.3",
				Reference:  "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Digest:     "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Repository: "registry.example.com/agentskills/root-skill",
				Manifest: manifest.Manifest{
					SchemaVersion: "v1",
					PackageType:   "skill",
					Name:          "root-skill",
					Version:       "1.2.3",
					Skill:         manifest.Skill{Entrypoint: "SKILL.md"},
				},
			},
			"a-dependency": {
				Name:       "a-dependency",
				Version:    "1.0.0",
				Reference:  "oci://registry.example.com/agentskills/a-dependency@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				Digest:     "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				Repository: "registry.example.com/agentskills/a-dependency",
				Manifest: manifest.Manifest{
					SchemaVersion: "v1",
					PackageType:   "skill",
					Name:          "a-dependency",
					Version:       "1.0.0",
					Skill:         manifest.Skill{Entrypoint: "SKILL.md"},
				},
			},
		},
		Edges: map[string][]string{
			"z-dependency": nil,
			"root-skill":   {"z-dependency", "a-dependency"},
			"a-dependency": {"z-dependency"},
		},
	}

	lock, err := FromGraph(graph)
	if err != nil {
		t.Fatalf("FromGraph() error = %v", err)
	}

	if lock.Root.Name != "root-skill" {
		t.Fatalf("Root.Name = %q, want %q", lock.Root.Name, "root-skill")
	}
	if lock.Root.Version != "1.2.3" {
		t.Fatalf("Root.Version = %q, want %q", lock.Root.Version, "1.2.3")
	}
	if lock.Root.Reference != "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("Root.Reference = %q, want canonical root reference", lock.Root.Reference)
	}

	wantPackages := []Package{
		{
			Name:      "a-dependency",
			Version:   "1.0.0",
			Digest:    "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			Reference: "oci://registry.example.com/agentskills/a-dependency@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		},
		{
			Name:      "root-skill",
			Version:   "1.2.3",
			Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Reference: "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			Name:      "z-dependency",
			Version:   "2.0.0",
			Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Reference: "oci://registry.example.com/agentskills/z-dependency@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
	if len(lock.Packages) != len(wantPackages) {
		t.Fatalf("len(Packages) = %d, want %d", len(lock.Packages), len(wantPackages))
	}
	for i, want := range wantPackages {
		if got := lock.Packages[i]; got != want {
			t.Fatalf("Packages[%d] = %+v, want %+v", i, got, want)
		}
	}

	wantEdges := []Edge{
		{From: "a-dependency", To: "z-dependency"},
		{From: "root-skill", To: "a-dependency"},
		{From: "root-skill", To: "z-dependency"},
	}
	if len(lock.Edges) != len(wantEdges) {
		t.Fatalf("len(Edges) = %d, want %d", len(lock.Edges), len(wantEdges))
	}
	for i, want := range wantEdges {
		if got := lock.Edges[i]; got != want {
			t.Fatalf("Edges[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func validMinimalLockfile() File {
	return File{
		SchemaVersion: "v1",
		Root: Root{
			Name:      "root-skill",
			Version:   "1.2.3",
			Reference: "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		Packages: []Package{
			{
				Name:      "root-skill",
				Version:   "1.2.3",
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Reference: "oci://registry.example.com/agentskills/root-skill@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
		},
		Edges: []Edge{},
	}
}

func marshalLockfileJSON(t *testing.T, lock File) string {
	t.Helper()

	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return string(data)
}
