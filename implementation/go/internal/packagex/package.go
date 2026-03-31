package packagex

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/filesha256"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

type BuildOptions struct {
	IncludeFilesSHA256 bool
}

func BuildTGZ(w io.Writer, sourceDir string, options BuildOptions) error {
	loaded, err := validateSkillDir(sourceDir)
	if err != nil {
		return err
	}

	stagingDir, err := stageSkillDir(sourceDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(stagingDir))

	if !options.IncludeFilesSHA256 {
		if _, err := filesha256.Validate(stagingDir); err != nil {
			return fmt.Errorf("validate existing .skill/files.sha256: %w", err)
		}
		return archive.WriteTGZ(w, stagingDir, loaded)
	}

	checksumBytes, err := GenerateFilesSHA256(stagingDir)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(stagingDir, ".skill", "files.sha256"), checksumBytes, 0o644); err != nil {
		return fmt.Errorf("write .skill/files.sha256: %w", err)
	}

	return archive.WriteTGZ(w, stagingDir, loaded)
}

func validateSkillDir(sourceDir string) (manifest.Manifest, error) {
	manifestPath := filepath.Join(sourceDir, ".skill", "manifest.json")
	loaded, err := manifest.LoadFile(manifestPath)
	if err != nil {
		return manifest.Manifest{}, fmt.Errorf("load source manifest: %w", err)
	}

	if filepath.Base(filepath.Clean(sourceDir)) != loaded.Name {
		return manifest.Manifest{}, fmt.Errorf("source directory %q does not match manifest name %q", filepath.Base(filepath.Clean(sourceDir)), loaded.Name)
	}

	if info, err := os.Stat(filepath.Join(sourceDir, "SKILL.md")); err != nil {
		if os.IsNotExist(err) {
			return manifest.Manifest{}, fmt.Errorf("skill directory missing %q", filepath.Join(sourceDir, "SKILL.md"))
		}
		return manifest.Manifest{}, fmt.Errorf("stat %q: %w", filepath.Join(sourceDir, "SKILL.md"), err)
	} else if info.IsDir() {
		return manifest.Manifest{}, fmt.Errorf("skill path %q must be a file", filepath.Join(sourceDir, "SKILL.md"))
	}

	return loaded, nil
}

func stageSkillDir(sourceDir string) (string, error) {
	parentDir, err := os.MkdirTemp("", "skill-package-")
	if err != nil {
		return "", fmt.Errorf("create staging directory: %w", err)
	}

	destRoot := filepath.Join(parentDir, filepath.Base(filepath.Clean(sourceDir)))
	if err := copyTree(sourceDir, destRoot); err != nil {
		os.RemoveAll(parentDir)
		return "", err
	}

	return destRoot, nil
}

func copyTree(sourceDir, destDir string) error {
	return filepath.WalkDir(sourceDir, func(currentPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %q: %w", currentPath, err)
		}

		relPath, err := filepath.Rel(sourceDir, currentPath)
		if err != nil {
			return fmt.Errorf("relative path for %q: %w", currentPath, err)
		}

		targetPath := destDir
		if relPath != "." {
			targetPath = filepath.Join(destDir, relPath)
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %q: %w", currentPath, err)
		}

		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported symlink entry %q", currentPath)
		}

		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported filesystem entry %q", currentPath)
		}

		data, err := os.ReadFile(currentPath)
		if err != nil {
			return fmt.Errorf("read %q: %w", currentPath, err)
		}

		return os.WriteFile(targetPath, data, info.Mode().Perm())
	})
}
