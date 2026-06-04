package main

import (
	"context"
	"fmt"

	installpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/install"
	lockfilepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/lockfile"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
	verifypkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/verify"
	"github.com/spf13/cobra"
)

var newInstallRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newInstallCmd() *cobra.Command {
	var targetDir string
	var from string
	var lockfile string
	var noVerify bool
	var certificateIdentity string
	var certificateOIDCIssuer string
	var builderID string
	var sourceRepository string

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
				if lockfile == "" {
					return fmt.Errorf("artifact reference is required")
				}
			}

			registry := newInstallRegistry()
			policy := verifypkg.TrustPolicy{
				NoVerify:              noVerify,
				CertificateIdentity:   certificateIdentity,
				CertificateOIDCIssuer: certificateOIDCIssuer,
				BuilderID:             builderID,
				SourceRepository:      sourceRepository,
			}
			var graph resolve.Graph
			if lockfile != "" {
				file, err := lockfilepkg.Load(lockfile)
				if err != nil {
					return err
				}
				if len(args) > 0 {
					if err := lockfilepkg.MatchRootInput(file, args[0], from); err != nil {
						return err
					}
				}
				graph, err = lockfilepkg.ToGraph(file)
				if err != nil {
					return err
				}
			} else {
				root, err := parseInstallRoot(args[0], from)
				if err != nil {
					return err
				}

				graph, err = resolve.ResolveGraph(cmd.Context(), registry, root)
				if err != nil {
					return err
				}
			}
			if err := preverifyGraph(cmd.Context(), registry, graph, policy); err != nil {
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
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Bypass OCI signature and provenance verification")
	cmd.Flags().StringVar(&certificateIdentity, "certificate-identity", "", "Expected Sigstore certificate identity for OCI artifact verification")
	cmd.Flags().StringVar(&certificateOIDCIssuer, "certificate-oidc-issuer", "", "Expected Sigstore OIDC issuer for OCI artifact verification")
	cmd.Flags().StringVar(&builderID, "builder-id", "", "Expected SLSA builder identity for OCI artifact verification")
	cmd.Flags().StringVar(&sourceRepository, "source-repo", "", "Expected source repository URI recorded in provenance")

	return cmd
}

func preverifyGraph(ctx context.Context, registry oci.Registry, graph resolve.Graph, policy verifypkg.TrustPolicy) error {
	if policy.NoVerify {
		return nil
	}
	if err := policy.ValidateForOCI(); err != nil {
		return err
	}

	for _, selected := range graph.Packages {
		if _, err := verifypkg.VerifyReference(ctx, registry, selected.Reference, policy); err != nil {
			return err
		}
	}
	return nil
}

func parseInstallRoot(value, from string) (resolve.Root, error) {
	return parseResolveRoot(value, from, "installing")
}

func parseResolveRoot(value, from, verb string) (resolve.Root, error) {
	if _, err := oci.ParseReference(value); err == nil {
		return resolve.NewExactRootWithBase(value, from)
	} else if isLikelyArtifactReference(value) {
		return resolve.Root{}, err
	}

	if from == "" {
		return resolve.Root{}, fmt.Errorf("--from is required when %s by package name", verb)
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
