package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
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

	t.Run("verify package archive path with at sign succeeds", func(t *testing.T) {
		archivePath := buildNamedPackage(t, "list-directory", "1.2.3", nil)
		atDir := filepath.Join(t.TempDir(), "build@v1")
		if err := os.MkdirAll(atDir, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		archiveWithAt := filepath.Join(atDir, filepath.Base(archivePath))
		if err := os.Rename(archivePath, archiveWithAt); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}

		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", archiveWithAt})

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

	t.Run("malformed artifact-like input returns parse error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "oci://registry.example.com/agentskills/list-directory@sha256:notadigest"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), `parse digest reference "oci://registry.example.com/agentskills/list-directory@sha256:notadigest"`) {
			t.Fatalf("Execute() error = %q, want parse failure", err)
		}
		if strings.Contains(err.Error(), "no such file") {
			t.Fatalf("Execute() error = %q, want OCI parse failure instead of filesystem fallback", err)
		}
	})

	t.Run("explicit OCI tgz-like input returns OCI validation error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "oci://registry.example.com/agentskills/list-directory.tgz"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), `OCI reference "oci://registry.example.com/agentskills/list-directory.tgz" must be digest-pinned`) {
			t.Fatalf("Execute() error = %q, want OCI validation failure", err)
		}
		if strings.Contains(err.Error(), "no such file") {
			t.Fatalf("Execute() error = %q, want OCI validation failure instead of filesystem fallback", err)
		}
	})

	t.Run("tag-qualified digest reference returns OCI validation error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "oci://registry.example.com/agentskills/list-directory:1.2.3@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "must not include a tag-qualified repository name") {
			t.Fatalf("Execute() error = %q, want repository validation failure", err)
		}
		if strings.Contains(err.Error(), "fetch") {
			t.Fatalf("Execute() error = %q, want failure before fetch", err)
		}
	})

	t.Run("malformed plain OCI-like input returns parse error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "registry.example.com/agentskills/list-directory@sha256"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), `parse digest reference "registry.example.com/agentskills/list-directory@sha256"`) {
			t.Fatalf("Execute() error = %q, want parse failure", err)
		}
		if strings.Contains(err.Error(), "no such file") {
			t.Fatalf("Execute() error = %q, want OCI parse failure instead of filesystem fallback", err)
		}
	})

	t.Run("plain malformed locator without path returns OCI validation error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "registry.example.com@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), `repository "registry.example.com" is not a valid OCI locator`) {
			t.Fatalf("Execute() error = %q, want OCI locator validation failure", err)
		}
		if strings.Contains(err.Error(), "no such file") {
			t.Fatalf("Execute() error = %q, want OCI validation failure instead of filesystem fallback", err)
		}
	})

	t.Run("plain tag-style OCI-looking input returns OCI validation error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "registry.example.com/agentskills/list-directory:1.2.3"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), `OCI reference "registry.example.com/agentskills/list-directory:1.2.3" must be digest-pinned`) {
			t.Fatalf("Execute() error = %q, want OCI validation failure", err)
		}
		if strings.Contains(err.Error(), "no such file") {
			t.Fatalf("Execute() error = %q, want OCI validation failure instead of filesystem fallback", err)
		}
	})

	t.Run("plain empty-digest OCI-looking input returns OCI validation error", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"verify", "registry.example.com/agentskills/list-directory@"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), `OCI reference "registry.example.com/agentskills/list-directory@" must be digest-pinned`) {
			t.Fatalf("Execute() error = %q, want OCI validation failure", err)
		}
		if strings.Contains(err.Error(), "no such file") {
			t.Fatalf("Execute() error = %q, want OCI validation failure instead of filesystem fallback", err)
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
