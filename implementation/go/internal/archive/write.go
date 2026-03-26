package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

func WriteTGZ(w io.Writer, sourceDir string, expected manifest.Manifest) error {
	if expected.Name == "" {
		return fmt.Errorf("manifest name is required")
	}

	loaded, err := manifest.LoadFile(filepath.Join(sourceDir, ".skill", "manifest.json"))
	if err != nil {
		return fmt.Errorf("load source manifest: %w", err)
	}

	if loaded.Name != expected.Name {
		return fmt.Errorf("source manifest name %q does not match expected manifest name %q", loaded.Name, expected.Name)
	}

	if filepath.Base(filepath.Clean(sourceDir)) != expected.Name {
		return fmt.Errorf("source directory %q does not match manifest name %q", filepath.Base(filepath.Clean(sourceDir)), expected.Name)
	}

	entries, err := collectEntries(sourceDir)
	if err != nil {
		return err
	}

	gzipWriter, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	gzipWriter.Header.ModTime = unixEpoch
	gzipWriter.Header.OS = 255

	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		if err := writeEntry(tarWriter, sourceDir, entry); err != nil {
			tarWriter.Close()
			gzipWriter.Close()
			return err
		}
	}

	if err := tarWriter.Close(); err != nil {
		gzipWriter.Close()
		return fmt.Errorf("close tar writer: %w", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("close gzip writer: %w", err)
	}

	return nil
}

func collectEntries(sourceDir string) ([]string, error) {
	entries := []string{sourceDir}

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

		if !entry.Type().IsRegular() && !entry.IsDir() {
			return fmt.Errorf("unsupported filesystem entry %q", currentPath)
		}

		entries = append(entries, currentPath)
		return nil
	}); err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		left := archiveName(sourceDir, entries[i])
		right := archiveName(sourceDir, entries[j])
		return left < right
	})

	return entries, nil
}

func writeEntry(tarWriter *tar.Writer, sourceDir, currentPath string) error {
	info, err := os.Lstat(currentPath)
	if err != nil {
		return fmt.Errorf("stat %q: %w", currentPath, err)
	}

	name := archiveName(sourceDir, currentPath)
	if _, err := validateArchivePath(name); err != nil {
		return err
	}
	header := &tar.Header{
		Name:    name,
		ModTime: unixEpoch,
		Mode:    int64(info.Mode().Perm()),
		Uid:     0,
		Gid:     0,
		Format:  tar.FormatUSTAR,
	}

	switch {
	case info.IsDir():
		header.Typeflag = tar.TypeDir
		header.Name += "/"
	case info.Mode().IsRegular():
		header.Typeflag = tar.TypeReg
		header.Size = info.Size()
	default:
		return fmt.Errorf("unsupported filesystem entry %q", currentPath)
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header %q: %w", currentPath, err)
	}

	if !info.Mode().IsRegular() {
		return nil
	}

	file, err := os.Open(currentPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", currentPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(tarWriter, file); err != nil {
		return fmt.Errorf("write tar body %q: %w", currentPath, err)
	}

	return nil
}

func archiveName(sourceDir, currentPath string) string {
	rootName := filepath.Base(filepath.Clean(sourceDir))
	if currentPath == sourceDir {
		return rootName
	}

	relPath, _ := filepath.Rel(sourceDir, currentPath)
	return rootName + "/" + filepath.ToSlash(strings.TrimPrefix(relPath, string(filepath.Separator)))
}
