package packagex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerateFilesSHA256DeterministicAndSorted(t *testing.T) {
	sourceDir := copySkillFixture(t, "list-directory")
	manifestBytes, err := os.ReadFile(filepath.Join(sourceDir, ".skill", "manifest.json"))
	if err != nil {
		t.Fatalf("ReadFile(manifest.json) error = %v", err)
	}

	got, err := GenerateFilesSHA256(sourceDir)
	if err != nil {
		t.Fatalf("GenerateFilesSHA256() error = %v", err)
	}

	wantLines := []string{
		hashLine(t, manifestBytes, ".skill/manifest.json"),
		hashLine(t, []byte("# list-directory\n"), "SKILL.md"),
		hashLine(t, []byte("# Usage\n\nRun the helper script.\n"), "references/usage.md"),
		hashLine(t, []byte("#!/bin/sh\nls \"$@\"\n"), "scripts/list.sh"),
	}

	want := strings.Join(wantLines, "\n") + "\n"
	if string(got) != want {
		t.Fatalf("GenerateFilesSHA256() = %q, want %q", string(got), want)
	}

	again, err := GenerateFilesSHA256(sourceDir)
	if err != nil {
		t.Fatalf("GenerateFilesSHA256() second error = %v", err)
	}

	if !bytes.Equal(got, again) {
		t.Fatal("GenerateFilesSHA256() bytes differ for identical input")
	}
}

func TestGenerateFilesSHA256ExcludesItself(t *testing.T) {
	sourceDir := copySkillFixture(t, "list-directory")
	checksumPath := filepath.Join(sourceDir, ".skill", "files.sha256")
	if err := os.WriteFile(checksumPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(files.sha256) error = %v", err)
	}

	got, err := GenerateFilesSHA256(sourceDir)
	if err != nil {
		t.Fatalf("GenerateFilesSHA256() error = %v", err)
	}

	if strings.Contains(string(got), ".skill/files.sha256") {
		t.Fatalf("GenerateFilesSHA256() = %q, want self-exclusion", string(got))
	}
}

func TestGenerateFilesSHA256RejectsAmbiguousPaths(t *testing.T) {
	sourceDir := copySkillFixture(t, "list-directory")
	badPath := filepath.Join(sourceDir, "references", "bad\nname.txt")
	if err := os.WriteFile(badPath, []byte("bad\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(badPath) error = %v", err)
	}

	_, err := GenerateFilesSHA256(sourceDir)
	if err == nil {
		t.Fatal("GenerateFilesSHA256() error = nil, want ambiguous path failure")
	}

	if !strings.Contains(err.Error(), "invalid path characters") {
		t.Fatalf("GenerateFilesSHA256() error = %q, want invalid path characters context", err)
	}
}

func hashLine(t *testing.T, body []byte, relPath string) string {
	t.Helper()

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]) + "  " + relPath
}

func copySkillFixture(t *testing.T, name string) string {
	t.Helper()

	sourceRoot := testdataPath(t, "../../../testdata/skills/"+name)
	destRoot := filepath.Join(t.TempDir(), name)

	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(destRoot) error = %v", err)
	}

	if err := filepath.Walk(sourceRoot, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceRoot, currentPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(destRoot, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode().Perm())
		}

		data, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, info.Mode().Perm())
	}); err != nil {
		t.Fatalf("copy fixture %q: %v", name, err)
	}

	return destRoot
}

func testdataPath(t *testing.T, relative string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), relative))
}
