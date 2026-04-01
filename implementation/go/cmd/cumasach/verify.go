package main

import (
	"fmt"

	verifypkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/verify"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/spf13/cobra"
)

var newVerifyRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify <package.tgz|artifact-ref>",
		Short: "Verify a skill package or OCI-published artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := args[0]

			var (
				result verifypkg.Result
				err    error
			)
			if _, parseErr := oci.ParseReference(input); parseErr == nil {
				result, err = verifypkg.VerifyReference(cmd.Context(), newVerifyRegistry(), input)
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

	return cmd
}
