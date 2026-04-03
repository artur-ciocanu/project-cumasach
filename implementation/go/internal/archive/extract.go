package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

func ExtractTGZTemp(r io.Reader, parentDir string) (string, manifest.Manifest, error) {
	tempDir, err := os.MkdirTemp(parentDir, "skill-archive-")
	if err != nil {
		return "", manifest.Manifest{}, fmt.Errorf("create temp extraction directory: %w", err)
	}

	state, err := inspectArchive(r, func(header *tar.Header, reader io.Reader, cleanName string) error {
		targetPath := filepath.Join(tempDir, filepath.FromSlash(cleanName))

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create directory %q: %w", targetPath, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create parent directory for %q: %w", targetPath, err)
			}

			file, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return fmt.Errorf("create file %q: %w", targetPath, err)
			}

			if _, err := io.Copy(file, reader); err != nil {
				file.Close()
				return fmt.Errorf("write file %q: %w", targetPath, err)
			}

			if err := file.Close(); err != nil {
				return fmt.Errorf("close file %q: %w", targetPath, err)
			}
		}

		return nil
	})
	if err != nil {
		os.RemoveAll(tempDir)
		return "", manifest.Manifest{}, err
	}

	extractedRoot := filepath.Join(tempDir, state.topLevel)
	return extractedRoot, state.manifest, nil
}
