package verify

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	packagex "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
)

func TestVerifyPackage(t *testing.T) {
	t.Run("valid package archive succeeds", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		archivePath := buildPackageArchive(t, sourceDir, true)

		result, err := VerifyPackage(archivePath)
		if err != nil {
			t.Fatalf("VerifyPackage() error = %v", err)
		}
		if got := result.Mode; got != "package" {
			t.Fatalf("Mode = %q, want %q", got, "package")
		}
		if got := result.Name; got != "list-directory" {
			t.Fatalf("Name = %q, want %q", got, "list-directory")
		}
		if got := result.Version; got != "1.2.3" {
			t.Fatalf("Version = %q, want %q", got, "1.2.3")
		}
		if !result.VerifiedFilesSHA256 {
			t.Fatal("VerifiedFilesSHA256 = false, want true")
		}
	})

	t.Run("schema invalid manifest fails", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		if err := os.WriteFile(filepath.Join(sourceDir, ".skill", "manifest.json"), []byte(`{"schemaVersion":"v1"}`), 0o644); err != nil {
			t.Fatalf("WriteFile(manifest.json) error = %v", err)
		}
		archivePath := writeArchiveFromDir(t, sourceDir)

		_, err := VerifyPackage(archivePath)
		if err == nil {
			t.Fatal("VerifyPackage() error = nil, want schema validation failure")
		}
		if !strings.Contains(err.Error(), "schema") {
			t.Fatalf("VerifyPackage() error = %q, want schema failure", err)
		}
	})

	t.Run("semantically invalid manifest fails", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		if err := os.WriteFile(filepath.Join(sourceDir, ".skill", "manifest.json"), []byte(`{
  "schemaVersion": "v1",
  "packageType": "skill",
  "name": "list-directory",
  "version": "1.2.3",
  "skill": {"entrypoint": "SKILL.md"},
  "dependencies": [{"name": "child", "version": "1.2"}]
}`), 0o644); err != nil {
			t.Fatalf("WriteFile(manifest.json) error = %v", err)
		}
		archivePath := writeArchiveFromDir(t, sourceDir)

		_, err := VerifyPackage(archivePath)
		if err == nil {
			t.Fatal("VerifyPackage() error = nil, want semantic validation failure")
		}
		if !strings.Contains(err.Error(), "constraint") {
			t.Fatalf("VerifyPackage() error = %q, want dependency constraint failure", err)
		}
	})

	t.Run("checksum mismatch fails", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		writeChecksumFile(t, sourceDir, []string{
			hashLine(t, []byte("wrong manifest"), ".skill/manifest.json"),
			hashLine(t, []byte("# list-directory\n"), "SKILL.md"),
			hashLine(t, []byte("# Usage\n\nRun the helper script.\n"), "references/usage.md"),
			hashLine(t, []byte("#!/bin/sh\nls \"$@\"\n"), "scripts/list.sh"),
		})
		archivePath := writeArchiveFromDir(t, sourceDir)

		_, err := VerifyPackage(archivePath)
		if err == nil {
			t.Fatal("VerifyPackage() error = nil, want checksum mismatch failure")
		}
		if !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("VerifyPackage() error = %q, want checksum mismatch context", err)
		}
	})

	t.Run("unsorted checksum entries fail", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		writeChecksumFile(t, sourceDir, []string{
			hashLine(t, []byte("# list-directory\n"), "SKILL.md"),
			hashLine(t, mustReadFile(t, filepath.Join(sourceDir, ".skill", "manifest.json")), ".skill/manifest.json"),
		})
		archivePath := writeArchiveFromDir(t, sourceDir)

		_, err := VerifyPackage(archivePath)
		if err == nil {
			t.Fatal("VerifyPackage() error = nil, want unsorted checksum failure")
		}
		if !strings.Contains(err.Error(), "unsorted") {
			t.Fatalf("VerifyPackage() error = %q, want unsorted failure", err)
		}
	})

	t.Run("duplicate checksum paths fail", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		manifestBytes := mustReadFile(t, filepath.Join(sourceDir, ".skill", "manifest.json"))
		writeChecksumFile(t, sourceDir, []string{
			hashLine(t, manifestBytes, ".skill/manifest.json"),
			hashLine(t, manifestBytes, ".skill/manifest.json"),
		})
		archivePath := writeArchiveFromDir(t, sourceDir)

		_, err := VerifyPackage(archivePath)
		if err == nil {
			t.Fatal("VerifyPackage() error = nil, want duplicate checksum failure")
		}
		if !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("VerifyPackage() error = %q, want duplicate failure", err)
		}
	})

	t.Run("ambiguous checksum paths fail", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		manifestBytes := mustReadFile(t, filepath.Join(sourceDir, ".skill", "manifest.json"))
		writeChecksumFile(t, sourceDir, []string{
			hashLine(t, manifestBytes, "./.skill/manifest.json"),
		})
		archivePath := writeArchiveFromDir(t, sourceDir)

		_, err := VerifyPackage(archivePath)
		if err == nil {
			t.Fatal("VerifyPackage() error = nil, want ambiguous checksum path failure")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("VerifyPackage() error = %q, want ambiguous checksum path failure", err)
		}
	})

	t.Run("package without files sha256 still succeeds", func(t *testing.T) {
		sourceDir := copySkillFixture(t, "list-directory")
		archivePath := buildPackageArchive(t, sourceDir, false)

		result, err := VerifyPackage(archivePath)
		if err != nil {
			t.Fatalf("VerifyPackage() error = %v", err)
		}
		if result.VerifiedFilesSHA256 {
			t.Fatal("VerifiedFilesSHA256 = true, want false")
		}
	})
}

func buildPackageArchive(t *testing.T, sourceDir string, includeFilesSHA256 bool) string {
	t.Helper()

	var archive bytes.Buffer
	if err := packagex.BuildTGZ(&archive, sourceDir, packagex.BuildOptions{IncludeFilesSHA256: includeFilesSHA256}); err != nil {
		t.Fatalf("BuildTGZ() error = %v", err)
	}

	archivePath := filepath.Join(t.TempDir(), filepath.Base(sourceDir)+".tgz")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(archivePath) error = %v", err)
	}
	return archivePath
}

func writeArchiveFromDir(t *testing.T, sourceDir string) string {
	t.Helper()

	loaded, err := manifestpkg.LoadFile(filepath.Join(sourceDir, ".skill", "manifest.json"))
	if err == nil {
		var archive bytes.Buffer
		if err := archivepkg.WriteTGZ(&archive, sourceDir, loaded); err == nil {
			archivePath := filepath.Join(t.TempDir(), filepath.Base(sourceDir)+".tgz")
			if err := os.WriteFile(archivePath, archive.Bytes(), 0o644); err != nil {
				t.Fatalf("WriteFile(archivePath) error = %v", err)
			}
			return archivePath
		}
	}

	archivePath := filepath.Join(t.TempDir(), filepath.Base(sourceDir)+".tgz")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Create(archivePath) error = %v", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	if err := filepath.Walk(sourceDir, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(filepath.Dir(sourceDir), currentPath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relPath)
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		body, err := os.ReadFile(currentPath)
		if err != nil {
			return err
		}
		_, err = tarWriter.Write(body)
		return err
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}

	return archivePath
}

func writeChecksumFile(t *testing.T, sourceDir string, lines []string) {
	t.Helper()

	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sourceDir, ".skill", "files.sha256"), []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(files.sha256) error = %v", err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
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
