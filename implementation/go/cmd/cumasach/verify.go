package main

import (
	"fmt"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	verifypkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/verify"
	"github.com/spf13/cobra"
)

var newVerifyRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newVerifyCmd() *cobra.Command {
	var noVerify bool
	var certificateIdentity string
	var certificateOIDCIssuer string
	var builderID string
	var sourceRepository string

	cmd := &cobra.Command{
		Use:   "verify <package.tgz|artifact-ref>",
		Short: "Verify a skill package or OCI-published artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]
			policy := verifypkg.TrustPolicy{
				NoVerify:              noVerify,
				CertificateIdentity:   certificateIdentity,
				CertificateOIDCIssuer: certificateOIDCIssuer,
				BuilderID:             builderID,
				SourceRepository:      sourceRepository,
			}

			var (
				result verifypkg.Result
				err    error
			)
			if _, parseErr := oci.ParseReference(input); parseErr == nil {
				result, err = verifypkg.VerifyReference(cmd.Context(), newVerifyRegistry(), input, policy)
			} else if oci.LooksLikeReference(input) {
				return parseErr
			} else {
				result, err = verifypkg.VerifyPackage(input)
			}
			if err != nil {
				return err
			}

			switch result.Mode {
			case "oci":
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "verified OCI artifact %s %s\n", result.Name, result.Version); err != nil {
					return fmt.Errorf("write verify result: %w", err)
				}
			default:
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "verified package %s %s\n", result.Name, result.Version); err != nil {
					return fmt.Errorf("write verify result: %w", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "Bypass OCI signature and provenance verification")
	cmd.Flags().StringVar(&certificateIdentity, "certificate-identity", "", "Expected Sigstore certificate identity for OCI artifact verification")
	cmd.Flags().StringVar(&certificateOIDCIssuer, "certificate-oidc-issuer", "", "Expected Sigstore OIDC issuer for OCI artifact verification")
	cmd.Flags().StringVar(&builderID, "builder-id", "", "Expected SLSA builder identity for OCI artifact verification")
	cmd.Flags().StringVar(&sourceRepository, "source-repo", "", "Expected source repository URI recorded in provenance")

	return cmd
}
