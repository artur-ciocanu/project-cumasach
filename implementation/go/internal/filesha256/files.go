package filesha256

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

var checksumLinePattern = regexp.MustCompile(`^([0-9a-f]{64})  (.+)$`)

func Validate(root string) (bool, error) {
	checksumPath := filepath.Join(root, ".skill", "files.sha256")
	checksumBytes, err := os.ReadFile(checksumPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %q: %w", checksumPath, err)
	}
	if !utf8.Valid(checksumBytes) {
		return false, fmt.Errorf("%q is not valid UTF-8", filepath.ToSlash(filepath.Join(".skill", "files.sha256")))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(checksumBytes)))
	var previousPath string
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		if line == "" {
			return false, fmt.Errorf(".skill/files.sha256 line %d is malformed", lineNumber)
		}

		matches := checksumLinePattern.FindStringSubmatch(line)
		if matches == nil {
			return false, fmt.Errorf(".skill/files.sha256 line %d is malformed", lineNumber)
		}

		relPath := matches[2]
		if err := validatePath(relPath); err != nil {
			return false, err
		}
		if relPath == ".skill/files.sha256" {
			return false, fmt.Errorf(".skill/files.sha256 must not list itself")
		}
		if previousPath != "" {
			if relPath == previousPath {
				return false, fmt.Errorf(".skill/files.sha256 contains duplicate path %q", relPath)
			}
			if relPath < previousPath {
				return false, fmt.Errorf(".skill/files.sha256 contains unsorted path %q after %q", relPath, previousPath)
			}
		}
		previousPath = relPath

		targetPath := filepath.Join(root, filepath.FromSlash(relPath))
		info, err := os.Stat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return false, fmt.Errorf(".skill/files.sha256 path %q not found in package", relPath)
			}
			return false, fmt.Errorf("stat checksum path %q: %w", relPath, err)
		}
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf(".skill/files.sha256 path %q is not a regular file", relPath)
		}

		body, err := os.ReadFile(targetPath)
		if err != nil {
			return false, fmt.Errorf("read checksum path %q: %w", relPath, err)
		}
		sum := sha256.Sum256(body)
		if hex.EncodeToString(sum[:]) != matches[1] {
			return false, fmt.Errorf(".skill/files.sha256 checksum mismatch for %q", relPath)
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("read .skill/files.sha256: %w", err)
	}

	return true, nil
}

func validatePath(relPath string) error {
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
