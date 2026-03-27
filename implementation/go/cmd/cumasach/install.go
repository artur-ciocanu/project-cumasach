package main

import (
	"fmt"

	installpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/install"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
	"github.com/spf13/cobra"
)

var newInstallRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newInstallCmd() *cobra.Command {
	var targetDir string
	var from string
	var lockfile string

	cmd := &cobra.Command{
		Use:   "install artifact-ref",
		Short: "Install a single skill artifact into a flat target directory",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return cobra.MaximumNArgs(1)(cmd, args)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetDir == "" {
				return fmt.Errorf("--target is required")
			}
			if len(args) == 0 {
				if lockfile != "" {
					return fmt.Errorf("--lockfile is not implemented in this slice")
				}
				return fmt.Errorf("artifact reference is required")
			}
			if lockfile != "" {
				return fmt.Errorf("--lockfile is not implemented in this slice")
			}

			registry := newInstallRegistry()
			root, err := parseInstallRoot(args[0], from)
			if err != nil {
				return err
			}

			graph, err := resolve.ResolveGraph(cmd.Context(), registry, root)
			if err != nil {
				return err
			}
			state, err := installpkg.Install(cmd.Context(), installpkg.Options{
				Registry:  registry,
				Graph:     &graph,
				TargetDir: targetDir,
			})
			if err != nil {
				return err
			}
			installed, ok := graph.Packages[graph.Root]
			if !ok {
				return fmt.Errorf("install completed without an active skill")
			}

			_ = state
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "installed %s %s\n", installed.Name, installed.Version); err != nil {
				return fmt.Errorf("write install result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&targetDir, "target", "", "Target flat skills directory")
	cmd.Flags().StringVar(&from, "from", "", "Repository base for package-name resolution")
	cmd.Flags().StringVar(&lockfile, "lockfile", "", "Install from a lockfile")

	return cmd
}

func parseInstallRoot(value, from string) (resolve.Root, error) {
	if _, err := oci.ParseReference(value); err == nil {
		return resolve.NewExactRoot(value)
	} else if isLikelyArtifactReference(value) {
		return resolve.Root{}, err
	}

	if from == "" {
		return resolve.Root{}, fmt.Errorf("--from is required when installing by package name")
	}
	return resolve.NewNamedRoot(value, from)
}

func isLikelyArtifactReference(value string) bool {
	for _, r := range value {
		if r == '/' || r == '@' || r == ':' {
			return true
		}
	}
	return false
}
