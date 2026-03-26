package packagex

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTGZWritesArchiveWithGeneratedFilesSHA256(t *testing.T) {
	sourceDir := copySkillFixture(t, "list-directory")

	var archive bytes.Buffer
	if err := BuildTGZ(&archive, sourceDir, BuildOptions{IncludeFilesSHA256: true}); err != nil {
		t.Fatalf("BuildTGZ() error = %v", err)
	}

	entries := readArchiveEntries(t, archive.Bytes())
	checksumBody, ok := entries["list-directory/.skill/files.sha256"]
	if !ok {
		t.Fatal("BuildTGZ() archive missing .skill/files.sha256")
	}

	want, err := GenerateFilesSHA256(sourceDir)
	if err != nil {
		t.Fatalf("GenerateFilesSHA256() error = %v", err)
	}

	if !bytes.Equal(checksumBody, want) {
		t.Fatalf(".skill/files.sha256 = %q, want %q", string(checksumBody), string(want))
	}
}

func TestBuildTGZOmitsFilesSHA256WhenDisabled(t *testing.T) {
	sourceDir := copySkillFixture(t, "list-directory")
	if err := os.WriteFile(filepath.Join(sourceDir, ".skill", "files.sha256"), []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(files.sha256) error = %v", err)
	}

	var archive bytes.Buffer
	if err := BuildTGZ(&archive, sourceDir, BuildOptions{}); err != nil {
		t.Fatalf("BuildTGZ() error = %v", err)
	}

	entries := readArchiveEntries(t, archive.Bytes())
	if _, ok := entries["list-directory/.skill/files.sha256"]; ok {
		t.Fatal("BuildTGZ() archive unexpectedly contains .skill/files.sha256")
	}
}

func TestBuildTGZRejectsInvalidSkillDirectory(t *testing.T) {
	sourceDir := testdataPath(t, "../../../testdata/invalid/missing-skill-md")

	var archive bytes.Buffer
	err := BuildTGZ(&archive, sourceDir, BuildOptions{IncludeFilesSHA256: true})
	if err == nil {
		t.Fatal("BuildTGZ() error = nil, want invalid skill directory failure")
	}

	if !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("BuildTGZ() error = %q, want missing SKILL.md context", err)
	}
}

func TestBuildTGZRejectsDirectoryManifestNameMismatch(t *testing.T) {
	sourceDir := copySkillFixture(t, "list-directory")
	renamedDir := filepath.Join(filepath.Dir(sourceDir), "wrong-name")
	if err := os.Rename(sourceDir, renamedDir); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	var archive bytes.Buffer
	err := BuildTGZ(&archive, renamedDir, BuildOptions{IncludeFilesSHA256: true})
	if err == nil {
		t.Fatal("BuildTGZ() error = nil, want manifest name mismatch failure")
	}

	if !strings.Contains(err.Error(), "manifest name") {
		t.Fatalf("BuildTGZ() error = %q, want manifest name context", err)
	}
}

func readArchiveEntries(t *testing.T, archiveBytes []byte) map[string][]byte {
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

		body, err := ioReadAll(tarReader)
		if err != nil {
			t.Fatalf("read archive body %q: %v", header.Name, err)
		}
		entries[header.Name] = body
	}

	return entries
}

func ioReadAll(r *tar.Reader) ([]byte, error) {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
