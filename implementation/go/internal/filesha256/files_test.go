package filesha256

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateReturnsFalseWhenNoChecksumFile(t *testing.T) {
	root := t.TempDir()
	found, err := Validate(root)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if found {
		t.Fatal("Validate() = true, want false when files.sha256 absent")
	}
}

func TestValidateAcceptsValidChecksumFile(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	content := []byte("hello\n")
	writeFile(t, filepath.Join(root, "SKILL.md"), content)
	writeChecksumFile(t, root, hashEntry(content, "SKILL.md")+"\n")

	found, err := Validate(root)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !found {
		t.Fatal("Validate() = false, want true for valid checksum")
	}
}

func TestValidateRejectsChecksumMismatch(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeFile(t, filepath.Join(root, "SKILL.md"), []byte("hello\n"))
	writeChecksumFile(t, root, hashEntry([]byte("wrong\n"), "SKILL.md")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want checksum mismatch")
	}
}

func TestValidateRejectsNonUTF8(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeFile(t, filepath.Join(root, ".skill", "files.sha256"), []byte{0xff, 0xfe, 0x0a})

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want UTF-8 rejection")
	}
}

func TestValidateRejectsEmptyLine(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	content := []byte("hello\n")
	writeFile(t, filepath.Join(root, "SKILL.md"), content)
	writeChecksumFile(t, root, hashEntry(content, "SKILL.md")+"\n\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want empty line rejection")
	}
}

func TestValidateRejectsMalformedLine(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeChecksumFile(t, root, "not-a-valid-checksum-line\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want malformed line rejection")
	}
}

func TestValidateRejectsSelfReference(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeChecksumFile(t, root, hashEntry([]byte("x"), ".skill/files.sha256")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want self-reference rejection")
	}
}

func TestValidateRejectsUnsortedPaths(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	contentA := []byte("a\n")
	contentB := []byte("b\n")
	writeFile(t, filepath.Join(root, "a.md"), contentA)
	writeFile(t, filepath.Join(root, "b.md"), contentB)
	writeChecksumFile(t, root, hashEntry(contentB, "b.md")+"\n"+hashEntry(contentA, "a.md")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want unsorted rejection")
	}
}

func TestValidateRejectsDuplicatePaths(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	content := []byte("a\n")
	writeFile(t, filepath.Join(root, "a.md"), content)
	entry := hashEntry(content, "a.md")
	writeChecksumFile(t, root, entry+"\n"+entry+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want duplicate rejection")
	}
}

func TestValidateRejectsMissingFile(t *testing.T) {
	root := t.TempDir()
	setupSkillDir(t, root)
	writeChecksumFile(t, root, hashEntry([]byte("x"), "missing.md")+"\n")

	_, err := Validate(root)
	if err == nil {
		t.Fatal("Validate() error = nil, want missing file rejection")
	}
}

func TestValidatePathRejectsAbsolutePath(t *testing.T) {
	if err := validatePath("/etc/passwd"); err == nil {
		t.Fatal("validatePath() error = nil, want absolute path rejection")
	}
}

func TestValidatePathRejectsBackslash(t *testing.T) {
	if err := validatePath("scripts\\list.sh"); err == nil {
		t.Fatal("validatePath() error = nil, want backslash rejection")
	}
}

func TestValidatePathRejectsTraversal(t *testing.T) {
	if err := validatePath("../secret"); err == nil {
		t.Fatal("validatePath() error = nil, want traversal rejection")
	}
}

func TestValidatePathRejectsEmptyComponents(t *testing.T) {
	for _, path := range []string{"", ".", "a//b", "./a"} {
		if err := validatePath(path); err == nil {
			t.Fatalf("validatePath(%q) error = nil, want rejection", path)
		}
	}
}

func TestValidatePathRejectsNullAndControlChars(t *testing.T) {
	if err := validatePath("a\x00b"); err == nil {
		t.Fatal("validatePath() error = nil, want null byte rejection")
	}
	if err := validatePath("a\nb"); err == nil {
		t.Fatal("validatePath() error = nil, want newline rejection")
	}
	if err := validatePath("a\rb"); err == nil {
		t.Fatal("validatePath() error = nil, want CR rejection")
	}
}

func TestValidatePathAcceptsValidPaths(t *testing.T) {
	for _, path := range []string{"SKILL.md", "scripts/list.sh", ".skill/manifest.json", "a/b/c/d.txt"} {
		if err := validatePath(path); err != nil {
			t.Fatalf("validatePath(%q) error = %v, want nil", path, err)
		}
	}
}

func setupSkillDir(t *testing.T, root string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.skill) error = %v", err)
	}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeChecksumFile(t *testing.T, root, content string) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".skill", "files.sha256"), []byte(content))
}

func hashEntry(data []byte, relPath string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + relPath
}
