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
}
