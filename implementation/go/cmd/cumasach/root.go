package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cumasach",
		Short: "Reference CLI for the Cumasach packaging specification",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newPackageCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newLockCmd())

	return cmd
}
