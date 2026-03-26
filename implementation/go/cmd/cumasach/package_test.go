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
	cmd := newRootCmd()
	var stdout bytes.Buffer

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) = !ok")
	}

	skillDir := filepath.Join(filepath.Dir(filename), "../../../testdata/skills/list-directory")
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

		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
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
