package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPackageCommandBuildsArchiveWithFilesSHA256(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer

	skillDir := fixtureSkillDir(t)
	outputPath := filepath.Join(t.TempDir(), "list-directory.tgz")

	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"package",
		skillDir,
		"--output", outputPath,
		"--files-sha256",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	archiveBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}

	entries := readTGZEntries(t, archiveBytes)
	if _, ok := entries["list-directory/.skill/files.sha256"]; !ok {
		t.Fatal("archive missing list-directory/.skill/files.sha256")
	}
}

func TestPackageCommandUsesDefaultOutputPath(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer

	cwd := t.TempDir()
	skillDir := fixtureSkillDir(t)
	outputPath := filepath.Join(cwd, "dist", "list-directory-1.2.3.tgz")

	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"package",
		skillDir,
		"--files-sha256",
	})
	cmd.SetContext(t.Context())
	cmd.SetErr(&stdout)
	cmd.SetOut(&stdout)

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() { _ = os.Chdir(originalWD) }()

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("Stat(default output) error = %v", err)
	}
}

func TestPackageCommandFailurePreservesExistingOutputFile(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer

	outputPath := filepath.Join(t.TempDir(), "list-directory.tgz")
	originalContents := []byte("existing archive contents")
	if err := os.WriteFile(outputPath, originalContents, 0o644); err != nil {
		t.Fatalf("WriteFile(output) error = %v", err)
	}

	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"package",
		filepath.Join(t.TempDir(), "missing-skill"),
		"--output", outputPath,
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}

	gotContents, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("ReadFile(output) error = %v", readErr)
	}

	if !bytes.Equal(gotContents, originalContents) {
		t.Fatalf("output contents = %q, want %q", string(gotContents), string(originalContents))
	}
}

func fixtureSkillDir(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) = !ok")
	}

	return filepath.Join(filepath.Dir(filename), "../../../testdata/skills/list-directory")
}

func readTGZEntries(t *testing.T, archiveBytes []byte) map[string][]byte {
	t.Helper()

	gzipReader, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	entries := map[string][]byte{}
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tarReader.Next() error = %v", err)
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		body, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("ReadAll(%q) error = %v", header.Name, err)
		}
		entries[header.Name] = body
	}

	return entries
}
