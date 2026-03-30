package main

import (
	"fmt"

	installpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/install"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/spf13/cobra"
)

var newRollbackRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newRollbackCmd() *cobra.Command {
	var targetDir string

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back to the previous install-state snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetDir == "" {
				return fmt.Errorf("--target is required")
			}

			if _, err := installpkg.Rollback(cmd.Context(), installpkg.Options{
				Registry:  newRollbackRegistry(),
				TargetDir: targetDir,
			}); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "rolled back target %s\n", targetDir); err != nil {
				return fmt.Errorf("write rollback result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&targetDir, "target", "", "Target flat skills directory")

	return cmd
}
