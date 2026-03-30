package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/lockfile"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
	"github.com/spf13/cobra"
)

var newLockRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newLockCmd() *cobra.Command {
	var from string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "lock artifact-ref",
		Short: "Resolve a skill graph and write a lockfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := parseResolveRoot(args[0], from, "locking")
			if err != nil {
				return err
			}

			if outputPath == "" {
				outputPath = "./skill.lock.json"
			}

			graph, err := resolve.ResolveGraph(cmd.Context(), newLockRegistry(), root)
			if err != nil {
				return err
			}

			if err := writeLockfile(outputPath, graph); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "locked %s to %s\n", graph.Root, outputPath); err != nil {
				return fmt.Errorf("write lock result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Repository base for package-name resolution")
	cmd.Flags().StringVar(&outputPath, "output", "", "Write the lockfile to this path")

	return cmd
}

func writeLockfile(outputPath string, graph resolve.Graph) (runErr error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create lockfile output directory: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(outputPath), ".cumasach-lock-*.json")
	if err != nil {
		return fmt.Errorf("create temporary lockfile: %w", err)
	}

	tempPath := tempFile.Name()
	defer func() {
		if tempFile != nil {
			if closeErr := tempFile.Close(); runErr == nil && closeErr != nil {
				runErr = fmt.Errorf("close temporary lockfile: %w", closeErr)
			}
		}
		if runErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := lockfile.Write(tempFile, graph); err != nil {
		return err
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary lockfile: %w", err)
	}
	tempFile = nil

	if err := os.Rename(tempPath, outputPath); err != nil {
		return fmt.Errorf("move lockfile into place: %w", err)
	}

	return nil
}
