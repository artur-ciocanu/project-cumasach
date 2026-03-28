package lockfile

import (
	"encoding/json"
	"strings"
	"testing"
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
