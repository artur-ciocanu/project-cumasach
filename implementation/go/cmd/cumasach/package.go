package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
	"github.com/spf13/cobra"
)

func newPackageCmd() *cobra.Command {
	var outputPath string
	var includeFilesSHA256 bool

	cmd := &cobra.Command{
		Use:   "package skill-dir",
		Short: "Package a skill directory as a .tgz artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (runErr error) {
			resolvedOutputPath, err := resolvePackageOutputPath(args[0], outputPath)
			if err != nil {
				return err
			}

			return writePackageArchive(resolvedOutputPath, args[0], packagex.BuildOptions{
				IncludeFilesSHA256: includeFilesSHA256,
			})
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Write the package archive to this path")
	cmd.Flags().BoolVar(&includeFilesSHA256, "files-sha256", false, "Include generated .skill/files.sha256 in the archive")

	return cmd
}

func resolvePackageOutputPath(skillDir, outputPath string) (string, error) {
	if outputPath != "" {
		return outputPath, nil
	}

	loaded, err := manifest.LoadFile(filepath.Join(skillDir, ".skill", "manifest.json"))
	if err != nil {
		return "", fmt.Errorf("load source manifest: %w", err)
	}

	return filepath.Join("dist", fmt.Sprintf("%s-%s.tgz", loaded.Name, loaded.Version)), nil
}

func writePackageArchive(outputPath, skillDir string, options packagex.BuildOptions) (runErr error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), ".cumasach-package-*.tgz")
	if err != nil {
		return fmt.Errorf("create temporary output file: %w", err)
	}

	tempPath := tempFile.Name()
	defer func() {
		if tempFile != nil {
			if closeErr := tempFile.Close(); runErr == nil && closeErr != nil {
				runErr = fmt.Errorf("close temporary output file: %w", closeErr)
			}
		}
		if runErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := packagex.BuildTGZ(tempFile, skillDir, options); err != nil {
		return err
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary output file: %w", err)
	}
	tempFile = nil

	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("move package archive into place: %w", err)
	}

	return nil
}
