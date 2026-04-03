package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelp(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
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

func TestRootVersion(t *testing.T) {
	cmd := newRootCmd("1.2.3", "abc1234", "2026-01-01")
	var stdout bytes.Buffer

	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "cumasach version 1.2.3 (abc1234, 2026-01-01)") {
		t.Fatalf("version output did not match expected format: %q", output)
	}
}

func TestRootPersistentFlags(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")

	for _, flagName := range []string{"json", "verbose", "no-color"} {
		if cmd.PersistentFlags().Lookup(flagName) == nil {
			t.Fatalf("persistent flag %q was not registered", flagName)
		}
	}
}
