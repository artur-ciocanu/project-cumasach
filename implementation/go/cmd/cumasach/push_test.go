package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
)

func TestPushCommandPushesPackageAndPrintsCanonicalReference(t *testing.T) {
	tests := []struct {
		name       string
		repository string
	}{
		{
			name:       "plain repository reference",
			repository: "registry.example.com/agentskills/list-directory",
		},
		{
			name:       "canonical oci repository reference",
			repository: "oci://registry.example.com/agentskills/list-directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := oci.NewMemoryRegistry()
			restore := swapPushRegistry(t, registry)
			defer restore()

			packagePath := buildFixturePackage(t)

			cmd := newRootCmd()
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)
			cmd.SetArgs([]string{"push", packagePath, tt.repository})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			output := strings.TrimSpace(stdout.String())
			wantPrefix := "oci://registry.example.com/agentskills/list-directory@sha256:"
			if !strings.HasPrefix(output, wantPrefix) {
				t.Fatalf("push output = %q, want prefix %q", output, wantPrefix)
			}

			if _, _, err := registry.ResolveReference(context.Background(), "registry.example.com/agentskills/list-directory", "1.2.3"); err != nil {
				t.Fatalf("ResolveReference(tag) error = %v", err)
			}

			wantManifestBytes := readMirroredManifestBytes(t, packagePath)
			fetched, err := oci.Fetch(context.Background(), registry, output)
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if !bytes.Equal(fetched.Config, wantManifestBytes) {
				t.Fatalf("fetched config bytes = %q, want mirrored manifest bytes %q", fetched.Config, wantManifestBytes)
			}
		})
	}
}

func TestPushCommandUsesExplicitTag(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapPushRegistry(t, registry)
	defer restore()

	packagePath := buildFixturePackage(t)

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"push",
		packagePath,
		"registry.example.com/agentskills/list-directory",
		"--tag", "stable",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, _, err := registry.ResolveReference(context.Background(), "registry.example.com/agentskills/list-directory", "stable"); err != nil {
		t.Fatalf("ResolveReference(stable) error = %v", err)
	}
}

func TestPushCommandFailsForMissingArchive(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapPushRegistry(t, registry)
	defer restore()

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"push",
		filepath.Join(t.TempDir(), "missing.tgz"),
		"registry.example.com/agentskills/list-directory",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "read package archive") {
		t.Fatalf("Execute() error = %q, want read package archive failure", err)
	}
}

func buildFixturePackage(t *testing.T) string {
	t.Helper()

	outputPath := filepath.Join(t.TempDir(), "list-directory.tgz")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("Create(output) error = %v", err)
	}

	if err := packagex.BuildTGZ(outputFile, fixtureSkillDir(t), packagex.BuildOptions{
		IncludeFilesSHA256: true,
	}); err != nil {
		_ = outputFile.Close()
		t.Fatalf("BuildTGZ() error = %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("Close(output) error = %v", err)
	}

	return outputPath
}

func readMirroredManifestBytes(t *testing.T, packagePath string) []byte {
	t.Helper()

	archiveFile, err := os.Open(packagePath)
	if err != nil {
		t.Fatalf("Open(package) error = %v", err)
	}
	defer archiveFile.Close()

	manifestBytes, _, err := archivepkg.ReadMirroredManifestTGZ(archiveFile)
	if err != nil {
		t.Fatalf("ReadMirroredManifestTGZ() error = %v", err)
	}

	return manifestBytes
}

func swapPushRegistry(t *testing.T, registry oci.Registry) func() {
	t.Helper()

	previous := newPushRegistry
	newPushRegistry = func() oci.Registry {
		return registry
	}

	return func() {
		newPushRegistry = previous
	}
}
