package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func TestVerifyCommand(t *testing.T) {
	t.Run("verify package archive succeeds", func(t *testing.T) {
		archivePath := buildNamedPackage(t, "list-directory", "1.2.3", nil)

		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", archivePath})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if output := stdout.String(); !strings.Contains(output, "verified package list-directory 1.2.3") {
			t.Fatalf("stdout = %q, want package verify summary", output)
		}
	})

	t.Run("verify OCI reference succeeds", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restore := swapVerifyRegistry(t, registry)
		defer restore()

		archivePath := buildNamedPackage(t, "list-directory", "1.2.3", nil)
		ref, err := pushPackage(context.Background(), registry, archivePath, "registry.example.com/agentskills/list-directory", "")
		if err != nil {
			t.Fatalf("pushPackage() error = %v", err)
		}

		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", ref})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if output := stdout.String(); !strings.Contains(output, "verified OCI artifact list-directory 1.2.3") {
			t.Fatalf("stdout = %q, want OCI verify summary", output)
		}
	})

	t.Run("missing argument fails", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "accepts 1 arg") {
			t.Fatalf("Execute() error = %q, want missing argument failure", err)
		}
	})
}

func swapVerifyRegistry(t *testing.T, registry oci.Registry) func() {
	t.Helper()

	previous := newVerifyRegistry
	newVerifyRegistry = func() oci.Registry {
		return registry
	}

	return func() {
		newVerifyRegistry = previous
	}
}
