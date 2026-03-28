package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	cmd := newRootCmd()
	var stdout bytes.Buffer

	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Reference CLI for the Cumasach packaging specification") {
		t.Fatalf("help output did not include short description: %q", output)
	}

	if !strings.Contains(output, "cumasach") {
		t.Fatalf("help output did not include command name: %q", output)
	}

	for _, commandName := range []string{"lock", "rollback", "verify"} {
		if !strings.Contains(output, commandName) {
			t.Fatalf("help output did not include %q command: %q", commandName, output)
		}
	}
}

func TestRootHelpCommandStubsReturnNotImplemented(t *testing.T) {
	for _, commandName := range []string{"rollback", "verify"} {
		t.Run(commandName, func(t *testing.T) {
			cmd := newRootCmd()
			var stdout bytes.Buffer

			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)
			cmd.SetArgs([]string{commandName})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("Execute() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), "not implemented in this slice") {
				t.Fatalf("Execute() error = %q, want not implemented failure", err)
			}
		})
	}
}
