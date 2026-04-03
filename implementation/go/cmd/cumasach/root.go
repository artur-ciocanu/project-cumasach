package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRootCmd(version, commit, date string) *cobra.Command {
	var jsonOutput bool
	var verbose bool
	var noColor bool

	cmd := &cobra.Command{
		Use:     "cumasach",
		Short:   "Reference CLI for the Cumasach packaging specification",
		Version: fmt.Sprintf("%s (%s, %s)", version, commit, date),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
	}

	cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Print command output as JSON when supported")
	cmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose command output")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable ANSI color in command output")

	cmd.AddCommand(newPackageCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newInstallCmd())
	cmd.AddCommand(newLockCmd())
	cmd.AddCommand(newRollbackCmd())
	cmd.AddCommand(newVerifyCmd())

	return cmd
}
