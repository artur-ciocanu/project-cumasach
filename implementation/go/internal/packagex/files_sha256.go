package packagex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GenerateFilesSHA256(sourceDir string) ([]byte, error) {
	entries, err := collectChecksumFiles(sourceDir)
	if err != nil {
		return nil, err
	}

	var builder strings.Builder
	for _, entry := range entries {
		sum, err := sha256File(entry.path)
		if err != nil {
			return nil, err
		}

		builder.WriteString(hex.EncodeToString(sum[:]))
		builder.WriteString("  ")
		builder.WriteString(entry.relPath)
		builder.WriteByte('\n')
	}

	return []byte(builder.String()), nil
}

type checksumFile struct {
	path    string
	relPath string
}

func collectChecksumFiles(sourceDir string) ([]checksumFile, error) {
	var entries []checksumFile

	if err := filepath.WalkDir(sourceDir, func(currentPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %q: %w", currentPath, err)
		}
		if currentPath == sourceDir {
			return nil
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported symlink entry %q", currentPath)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported filesystem entry %q", currentPath)
		}

		relPath, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return fmt.Errorf("relative path for %q: %w", currentPath, err)
		}

		normalized := filepath.ToSlash(relPath)
		if err := validateChecksumPath(normalized); err != nil {
			return err
		}
		if normalized == ".skill/files.sha256" {
			return nil
		}

		entries = append(entries, checksumFile{
			path:    currentPath,
			relPath: normalized,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	return entries, nil
}

func validateChecksumPath(relPath string) error {
	if relPath == "" || relPath == "." {
		return fmt.Errorf("invalid checksum path %q: empty path", relPath)
	}
	if strings.HasPrefix(relPath, "/") {
		return fmt.Errorf("invalid checksum path %q: absolute paths are not allowed", relPath)
	}
	if strings.Contains(relPath, "\\") {
		return fmt.Errorf("invalid checksum path %q: backslash separators are not allowed", relPath)
	}

	for _, component := range strings.Split(relPath, "/") {
		if component == "" || component == "." {
			return fmt.Errorf("invalid checksum path %q: ambiguous path encoding is not allowed", relPath)
		}
		if component == ".." {
			return fmt.Errorf("invalid checksum path %q: path traversal is not allowed", relPath)
		}
		if strings.ContainsAny(component, "\x00\r\n") {
			return fmt.Errorf("invalid checksum path %q: invalid path characters", relPath)
		}
	}

	return nil
}

func sha256File(path string) ([sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("open %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("hash %q: %w", path, err)
	}

	var sum [sha256.Size]byte
	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}
