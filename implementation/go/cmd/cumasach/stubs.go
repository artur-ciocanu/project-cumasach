package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRollbackCmd() *cobra.Command {
	return newNotImplementedCmd("rollback", "Roll back to a previous install-state snapshot")
}

func newVerifyCmd() *cobra.Command {
	return newNotImplementedCmd("verify", "Verify a skill package or installed skill")
}

func newNotImplementedCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s is not implemented in this slice", use)
		},
	}
}
