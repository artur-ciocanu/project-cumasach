package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func TestInstallCommandInstallsArtifactIntoTarget(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapInstallRegistry(t, registry)
	defer restore()

	ref := pushFixtureArtifact(t, registry)
	targetDir := t.TempDir()

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		ref,
		"--target", targetDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "list-directory", "SKILL.md")); err != nil {
		t.Fatalf("Stat(active SKILL.md) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".cumasach", "install-state.json")); err != nil {
		t.Fatalf("Stat(install state) error = %v", err)
	}
	if output := stdout.String(); !strings.Contains(output, "installed list-directory 1.2.3") {
		t.Fatalf("stdout = %q, want installed summary", output)
	}
}

func TestInstallCommandRejectsPackageNameResolution(t *testing.T) {
	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"list-directory",
		"--target", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "package-name resolution is not implemented") {
		t.Fatalf("Execute() error = %q, want package-name resolution failure", err)
	}
}

func TestInstallCommandRejectsUnsupportedFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "from",
			args: []string{"install", "oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--target", t.TempDir(), "--from", "registry.example.com/agentskills"},
			want: "--from is not implemented",
		},
		{
			name: "lockfile",
			args: []string{"install", "oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--target", t.TempDir(), "--lockfile", "skill.lock.json"},
			want: "--lockfile is not implemented",
		},
		{
			name: "no-recommended",
			args: []string{"install", "oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--target", t.TempDir(), "--no-recommended"},
			want: "--no-recommended is not implemented",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newRootCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Fatal("Execute() error = nil, want failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Execute() error = %q, want %q", err, tt.want)
			}
		})
	}
}

func TestInstallCommandRequiresTarget(t *testing.T) {
	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "--target is required") {
		t.Fatalf("Execute() error = %q, want missing target failure", err)
	}
}

func TestInstallCommandRejectsMalformedArtifactReference(t *testing.T) {
	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"oci://registry.example.com/agentskills/list-directory@sha256:short",
		"--target", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if strings.Contains(err.Error(), "package-name resolution is not implemented") {
		t.Fatalf("Execute() error = %q, want malformed reference error, not package-name resolution", err)
	}
}

func TestInstallCommandRejectsLockfileOnlyMode(t *testing.T) {
	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"--target", t.TempDir(),
		"--lockfile", "skill.lock.json",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "--lockfile is not implemented") {
		t.Fatalf("Execute() error = %q, want lockfile-only not implemented failure", err)
	}
}

func pushFixtureArtifact(t *testing.T, registry oci.Registry) string {
	t.Helper()

	packagePath := buildFixturePackage(t)
	ref, err := pushPackage(t.Context(), registry, packagePath, "registry.example.com/agentskills/list-directory", "")
	if err != nil {
		t.Fatalf("pushPackage() error = %v", err)
	}
	return ref
}

func swapInstallRegistry(t *testing.T, registry oci.Registry) func() {
	t.Helper()

	previous := newInstallRegistry
	newInstallRegistry = func() oci.Registry {
		return registry
	}

	return func() {
		newInstallRegistry = previous
	}
}
