package main

import (
	"fmt"
	"os"

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
			outputFile, err := os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}
			defer func() {
				if closeErr := outputFile.Close(); runErr == nil && closeErr != nil {
					runErr = fmt.Errorf("close output file: %w", closeErr)
				}
			}()

			return packagex.BuildTGZ(outputFile, args[0], packagex.BuildOptions{
				IncludeFilesSHA256: includeFilesSHA256,
			})
		},
	}

	cmd.Flags().StringVar(&outputPath, "output", "", "Write the package archive to this path")
	cmd.Flags().BoolVar(&includeFilesSHA256, "files-sha256", false, "Include generated .skill/files.sha256 in the archive")
	if err := cmd.MarkFlagRequired("output"); err != nil {
		panic(err)
	}

	return cmd
}
