package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

func TestWriteTGZDeterministic(t *testing.T) {
	sourceDir, expectedManifest := createSkillTree(t, "list-directory")

	var first bytes.Buffer
	if err := WriteTGZ(&first, sourceDir, expectedManifest); err != nil {
		t.Fatalf("WriteTGZ() first error = %v", err)
	}

	var second bytes.Buffer
	if err := WriteTGZ(&second, sourceDir, expectedManifest); err != nil {
		t.Fatalf("WriteTGZ() second error = %v", err)
	}

	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("WriteTGZ() bytes differ for identical input")
	}
}

func TestWriteTGZRejectsInvalidControlCharactersInSourcePaths(t *testing.T) {
	sourceDir, expectedManifest := createSkillTree(t, "list-directory")

	badPath := filepath.Join(sourceDir, "references", "bad\nname.txt")
	if err := os.WriteFile(badPath, []byte("bad\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(badPath) error = %v", err)
	}

	var archive bytes.Buffer
	err := WriteTGZ(&archive, sourceDir, expectedManifest)
	if err == nil {
		t.Fatal("WriteTGZ() error = nil, want invalid path character failure")
	}

	if !strings.Contains(err.Error(), "invalid path characters") {
		t.Fatalf("WriteTGZ() error = %q, want invalid path characters context", err)
	}
}

func TestReadManifestTGZReturnsManifest(t *testing.T) {
	sourceDir, expectedManifest := createSkillTree(t, "list-directory")
	archiveBytes := writeArchiveBytes(t, sourceDir, expectedManifest)

	loaded, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err != nil {
		t.Fatalf("ReadManifestTGZ() error = %v", err)
	}

	if loaded.Name != expectedManifest.Name {
		t.Fatalf("Name = %q, want %q", loaded.Name, expectedManifest.Name)
	}

	if loaded.Version != expectedManifest.Version {
		t.Fatalf("Version = %q, want %q", loaded.Version, expectedManifest.Version)
	}
}

func TestReadManifestTGZRejectsMultipleTopLevelDirectories(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "alpha/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "alpha/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# alpha\n"),
		},
		tarEntry{
			Name:     "alpha/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "alpha/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "alpha"),
		},
		tarEntry{
			Name:     "beta/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want multiple top-level directory failure")
	}

	if !strings.Contains(err.Error(), "top-level") {
		t.Fatalf("ReadManifestTGZ() error = %q, want top-level context", err)
	}
}

func TestReadManifestTGZRejectsManifestNameMismatch(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "wrong-name/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "wrong-name/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# wrong-name\n"),
		},
		tarEntry{
			Name:     "wrong-name/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "wrong-name/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want manifest name mismatch failure")
	}

	if !strings.Contains(err.Error(), "manifest") || !strings.Contains(err.Error(), "name") {
		t.Fatalf("ReadManifestTGZ() error = %q, want manifest name mismatch context", err)
	}
}

func TestReadManifestTGZRejectsPathTraversalEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "list-directory/../escape.txt",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("nope"),
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want traversal failure")
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("ReadManifestTGZ() error = %q, want path traversal context", err)
	}
}

func TestReadManifestTGZRejectsWindowsAbsolutePaths(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "C:\\tmp\\pwn",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("nope"),
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want Windows absolute path failure")
	}

	if !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("ReadManifestTGZ() error = %q, want absolute path context", err)
	}
}

func TestReadManifestTGZRejectsDuplicateEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "list-directory/references/dup.txt",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("first"),
		},
		tarEntry{
			Name:     "list-directory/references/dup.txt",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("second"),
		},
	)

	_, err := ReadManifestTGZ(bytes.NewReader(archiveBytes))
	if err == nil {
		t.Fatal("ReadManifestTGZ() error = nil, want duplicate entry failure")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("ReadManifestTGZ() error = %q, want duplicate context", err)
	}
}

func TestExtractTGZTempSafelyExtractsArchive(t *testing.T) {
	sourceDir, expectedManifest := createSkillTree(t, "list-directory")
	archiveBytes := writeArchiveBytes(t, sourceDir, expectedManifest)

	extractedRoot, extractedManifest, err := ExtractTGZTemp(bytes.NewReader(archiveBytes), t.TempDir())
	if err != nil {
		t.Fatalf("ExtractTGZTemp() error = %v", err)
	}

	if filepath.Base(extractedRoot) != expectedManifest.Name {
		t.Fatalf("ExtractTGZTemp() root base = %q, want %q", filepath.Base(extractedRoot), expectedManifest.Name)
	}

	if extractedManifest.Name != expectedManifest.Name {
		t.Fatalf("ExtractTGZTemp() manifest name = %q, want %q", extractedManifest.Name, expectedManifest.Name)
	}

	skillBytes, err := os.ReadFile(filepath.Join(extractedRoot, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md) error = %v", err)
	}

	if string(skillBytes) != "# list-directory\n" {
		t.Fatalf("SKILL.md = %q, want %q", string(skillBytes), "# list-directory\n")
	}
}

func TestExtractTGZTempRejectsPathTraversalEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "../escape.txt",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("nope"),
		},
	)

	_, _, err := ExtractTGZTemp(bytes.NewReader(archiveBytes), t.TempDir())
	if err == nil {
		t.Fatal("ExtractTGZTemp() error = nil, want traversal failure")
	}

	if !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("ExtractTGZTemp() error = %q, want path traversal context", err)
	}
}

func TestExtractTGZTempRejectsDuplicateEntries(t *testing.T) {
	archiveBytes := buildArchive(t,
		tarEntry{
			Name:     "list-directory/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/SKILL.md",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("# list-directory\n"),
		},
		tarEntry{
			Name:     "list-directory/.skill/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/.skill/manifest.json",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     mustManifestJSON(t, "list-directory"),
		},
		tarEntry{
			Name:     "list-directory/references/",
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		},
		tarEntry{
			Name:     "list-directory/references/dup.txt",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("first"),
		},
		tarEntry{
			Name:     "list-directory/references/dup.txt",
			Typeflag: tar.TypeReg,
			Mode:     0o644,
			Body:     []byte("second"),
		},
	)

	_, _, err := ExtractTGZTemp(bytes.NewReader(archiveBytes), t.TempDir())
	if err == nil {
		t.Fatal("ExtractTGZTemp() error = nil, want duplicate entry failure")
	}

	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("ExtractTGZTemp() error = %q, want duplicate context", err)
	}
}

func TestWriteTGZRejectsSymlinksInSourceDirectory(t *testing.T) {
	sourceDir, expectedManifest := createSkillTree(t, "list-directory")

	symlinkPath := filepath.Join(sourceDir, "references", "linked.txt")
	targetPath := filepath.Join(sourceDir, "SKILL.md")
	if err := os.Symlink(targetPath, symlinkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	var buf bytes.Buffer
	err := WriteTGZ(&buf, sourceDir, expectedManifest)
	if err == nil {
		t.Fatal("WriteTGZ() error = nil, want symlink rejection failure")
	}

	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("WriteTGZ() error = %q, want symlink context", err)
	}
}

type tarEntry struct {
	Name     string
	Typeflag byte
	Mode     int64
	Body     []byte
}

func createSkillTree(t *testing.T, name string) (string, manifest.Manifest) {
	t.Helper()

	root := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(root, ".skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.skill) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "references"), 0o755); err != nil {
		t.Fatalf("MkdirAll(references) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "references", "notes.txt"), []byte("notes\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(notes.txt) error = %v", err)
	}

	manifestValue := manifest.Manifest{
		SchemaVersion: "v1",
		PackageType:   "skill",
		Name:          name,
		Version:       "1.2.3",
		Description:   "Test skill",
		Skill: manifest.Skill{
			Entrypoint: "SKILL.md",
		},
	}

	manifestBytes, err := json.MarshalIndent(manifestValue, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(manifest) error = %v", err)
	}
	manifestBytes = append(manifestBytes, '\n')

	if err := os.WriteFile(filepath.Join(root, ".skill", "manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest.json) error = %v", err)
	}

	return root, manifestValue
}

func writeArchiveBytes(t *testing.T, sourceDir string, manifestValue manifest.Manifest) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := WriteTGZ(&buf, sourceDir, manifestValue); err != nil {
		t.Fatalf("WriteTGZ() error = %v", err)
	}

	return buf.Bytes()
}

func buildArchive(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzipWriter, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		t.Fatalf("NewWriterLevel() error = %v", err)
	}
	gzipWriter.Header.ModTime = unixEpoch
	gzipWriter.Header.OS = 255

	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{
			Name:     entry.Name,
			Typeflag: entry.Typeflag,
			Mode:     entry.Mode,
			ModTime:  unixEpoch,
			Format:   tar.FormatPAX,
		}
		if entry.Typeflag == tar.TypeReg {
			header.Size = int64(len(entry.Body))
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("WriteHeader(%q) error = %v", entry.Name, err)
		}

		if entry.Typeflag == tar.TypeReg {
			if _, err := tarWriter.Write(entry.Body); err != nil {
				t.Fatalf("Write(%q) error = %v", entry.Name, err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}

	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}

	return buf.Bytes()
}

func mustManifestJSON(t *testing.T, name string) []byte {
	t.Helper()

	manifestBytes, err := json.MarshalIndent(manifest.Manifest{
		SchemaVersion: "v1",
		PackageType:   "skill",
		Name:          name,
		Version:       "1.2.3",
		Skill: manifest.Skill{
			Entrypoint: "SKILL.md",
		},
	}, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent(manifest) error = %v", err)
	}

	return append(manifestBytes, '\n')
}
