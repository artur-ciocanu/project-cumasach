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

	for _, flagName := range []string{"--json", "--verbose", "--no-color"} {
		if !strings.Contains(output, flagName) {
			t.Fatalf("help output did not include %q flag: %q", flagName, output)
		}
	}

	if strings.Contains(output, "completion") {
		t.Fatalf("help output unexpectedly included completion command: %q", output)
	}
}

func TestRootPersistentFlags(t *testing.T) {
	cmd := newRootCmd()

	for _, flagName := range []string{"json", "verbose", "no-color"} {
		if cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Fatalf("persistent flag %q was not registered", flagName)
		}
	}
}
