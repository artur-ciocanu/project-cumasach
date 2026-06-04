package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
)

func TestPushCommandPushesPackageAndPrintsCanonicalReference(t *testing.T) {
	installFakeCosignRunner(t, "https://github.com/example/builders/cumasach", "https://github.com/example/project-cumasach")

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

			cmd := newRootCmd("test", "abc1234", "2026-01-01")
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stdout)
			cmd.SetArgs([]string{
				"push",
				packagePath,
				tt.repository,
				"--certificate-identity", "https://github.com/example/workflows/release.yml@refs/heads/main",
				"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
				"--builder-id", "https://github.com/example/builders/cumasach",
				"--source-repo", "https://github.com/example/project-cumasach",
			})

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
	installFakeCosignRunner(t, "https://github.com/example/builders/cumasach", "https://github.com/example/project-cumasach")

	registry := oci.NewMemoryRegistry()
	restore := swapPushRegistry(t, registry)
	defer restore()

	packagePath := buildFixturePackage(t)

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"push",
		packagePath,
		"registry.example.com/agentskills/list-directory",
		"--tag", "stable",
		"--certificate-identity", "https://github.com/example/workflows/release.yml@refs/heads/main",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--builder-id", "https://github.com/example/builders/cumasach",
		"--source-repo", "https://github.com/example/project-cumasach",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if _, _, err := registry.ResolveReference(context.Background(), "registry.example.com/agentskills/list-directory", "stable"); err != nil {
		t.Fatalf("ResolveReference(stable) error = %v", err)
	}
}

func TestPushCommandFailsForMissingArchive(t *testing.T) {
	installFakeCosignRunner(t, "https://github.com/example/builders/cumasach", "https://github.com/example/project-cumasach")

	registry := oci.NewMemoryRegistry()
	restore := swapPushRegistry(t, registry)
	defer restore()

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"push",
		filepath.Join(t.TempDir(), "missing.tgz"),
		"registry.example.com/agentskills/list-directory",
		"--certificate-identity", "https://github.com/example/workflows/release.yml@refs/heads/main",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--builder-id", "https://github.com/example/builders/cumasach",
		"--source-repo", "https://github.com/example/project-cumasach",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "read package archive") {
		t.Fatalf("Execute() error = %q, want read package archive failure", err)
	}
}

func TestPushCommandFailsForSemanticallyInvalidManifest(t *testing.T) {
	installFakeCosignRunner(t, "https://github.com/example/builders/cumasach", "https://github.com/example/project-cumasach")

	registry := oci.NewMemoryRegistry()
	restore := swapPushRegistry(t, registry)
	defer restore()

	packagePath := buildRawPackageWithManifest(t, "list-directory", []byte(`{
  "schemaVersion": "v1",
  "packageType": "skill",
  "name": "list-directory",
  "version": "1.2.3",
  "skill": {"entrypoint": "SKILL.md"},
  "dependencies": [{"name": "child", "version": "1.2"}]
}`))

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"push",
		packagePath,
		"registry.example.com/agentskills/list-directory",
		"--certificate-identity", "https://github.com/example/workflows/release.yml@refs/heads/main",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--builder-id", "https://github.com/example/builders/cumasach",
		"--source-repo", "https://github.com/example/project-cumasach",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want semantic validation failure")
	}
	if !strings.Contains(err.Error(), "constraint") {
		t.Fatalf("Execute() error = %q, want dependency constraint context", err)
	}
}

func TestPushCommandRequiresTrustInputs(t *testing.T) {
	installFakeCosignRunner(t, "https://github.com/example/builders/cumasach", "https://github.com/example/project-cumasach")

	registry := oci.NewMemoryRegistry()
	restore := swapPushRegistry(t, registry)
	defer restore()

	packagePath := buildFixturePackage(t)

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"push",
		packagePath,
		"registry.example.com/agentskills/list-directory",
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want missing trust input failure")
	}
	if !strings.Contains(err.Error(), "--certificate-identity") {
		t.Fatalf("Execute() error = %q, want trust input failure", err)
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
	defer func() { _ = archiveFile.Close() }()

	manifestBytes, _, err := archivepkg.ReadMirroredManifestTGZ(archiveFile)
	if err != nil {
		t.Fatalf("ReadMirroredManifestTGZ() error = %v", err)
	}

	return manifestBytes
}

func buildRawPackageWithManifest(t *testing.T, name string, manifestBytes []byte) string {
	t.Helper()

	sourceDir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(sourceDir, ".skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.skill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, ".skill", "manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest.json) error = %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), name+".tgz")
	outputFile, err := os.Create(outputPath)
	if err != nil {
		t.Fatalf("Create(output) error = %v", err)
	}
	defer func() { _ = outputFile.Close() }()

	gzipWriter := gzip.NewWriter(outputFile)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := filepath.Walk(sourceDir, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(filepath.Dir(sourceDir), currentPath)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if info.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(currentPath)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		_, err = io.Copy(tarWriter, file)
		return err
	}); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}

	return outputPath
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
