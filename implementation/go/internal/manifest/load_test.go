package manifest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadFileValidManifest(t *testing.T) {
	t.Setenv("PWD", "")

	manifestPath := testdataPath(t, "../../../testdata/skills/list-directory/.skill/manifest.json")
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalWD); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})

	loaded, err := LoadFile(manifestPath)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}

	if loaded.SchemaVersion != "v1" {
		t.Fatalf("SchemaVersion = %q, want %q", loaded.SchemaVersion, "v1")
	}

	if loaded.PackageType != "skill" {
		t.Fatalf("PackageType = %q, want %q", loaded.PackageType, "skill")
	}

	if loaded.Name != "list-directory" {
		t.Fatalf("Name = %q, want %q", loaded.Name, "list-directory")
	}

	if loaded.Version != "1.2.3" {
		t.Fatalf("Version = %q, want %q", loaded.Version, "1.2.3")
	}

	if loaded.Skill.Entrypoint != "SKILL.md" {
		t.Fatalf("Skill.Entrypoint = %q, want %q", loaded.Skill.Entrypoint, "SKILL.md")
	}
}

func TestLoadReaderRejectsSchemaInvalidManifest(t *testing.T) {
	manifestBytes, err := os.ReadFile(testdataPath(t, "../../../testdata/invalid/bad-manifest/.skill/manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	_, err = LoadReader(strings.NewReader(string(manifestBytes)))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want schema validation failure")
	}

	if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("LoadReader() error = %q, want schema validation context", err)
	}
}

func TestLoadReaderRejectsMalformedJSON(t *testing.T) {
	_, err := LoadReader(strings.NewReader("{"))
	if err == nil {
		t.Fatal("LoadReader() error = nil, want JSON parse failure")
	}
}

func testdataPath(t *testing.T, relative string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), relative))
}
