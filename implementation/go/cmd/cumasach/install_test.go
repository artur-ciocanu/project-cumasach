package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
)

func TestInstallCommandInstallsArtifactAndDependenciesIntoTarget(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapInstallRegistry(t, registry)
	defer restore()

	pushCommandSkillToRepository(t, registry, "registry.example.com/catalog/child", "child", "1.0.0", nil)
	ref := pushCommandSkillToRepository(t, registry, "registry.example.com/published/root-artifact", "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
	targetDir := t.TempDir()

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		ref,
		"--from", "registry.example.com/catalog",
		"--target", targetDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	for _, name := range []string{"root", "child"} {
		if _, err := os.Stat(filepath.Join(targetDir, name, "SKILL.md")); err != nil {
			t.Fatalf("Stat(%s/SKILL.md) error = %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".cumasach", "install-state.json")); err != nil {
		t.Fatalf("Stat(install state) error = %v", err)
	}
	if output := stdout.String(); !strings.Contains(output, "installed root 1.0.0") {
		t.Fatalf("stdout = %q, want installed summary", output)
	}
}

func TestInstallCommandExactArtifactWithDependenciesRequiresFrom(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapInstallRegistry(t, registry)
	defer restore()

	ref := pushCommandSkillToRepository(t, registry, "registry.example.com/published/root-artifact", "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		ref,
		"--target", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want missing --from failure")
	}
	if !strings.Contains(err.Error(), "--from") {
		t.Fatalf("Execute() error = %q, want missing --from context", err)
	}
}

func TestInstallCommandPackageNameRequiresFrom(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"root",
		"--target", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "--from is required") {
		t.Fatalf("Execute() error = %q, want missing --from failure", err)
	}
}

func TestInstallCommandResolvesPackageNameDependenciesFromBase(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapInstallRegistry(t, registry)
	defer restore()

	pushCommandSkill(t, registry, "child", "1.0.0", nil)
	pushCommandSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})

	targetDir := t.TempDir()
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"root",
		"--from", "registry.example.com/agentskills",
		"--target", targetDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "child", "SKILL.md")); err != nil {
		t.Fatalf("Stat(child/SKILL.md) error = %v", err)
	}
}

func TestInstallCommandSurfacesUnresolvedDependencyFailures(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapInstallRegistry(t, registry)
	defer restore()

	pushCommandSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "missing", Version: "^1.0.0"}})

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"root",
		"--from", "registry.example.com/agentskills",
		"--target", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("Execute() error = %q, want unresolved dependency context", err)
	}
}

func TestInstallCommandRequiresTarget(t *testing.T) {
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
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
	cmd := newRootCmd("test", "abc1234", "2026-01-01")
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

func TestInstallCommandLockfileMode(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restore := swapInstallRegistry(t, registry)
	restoreLock := swapLockRegistry(t, registry)
	defer restore()
	defer restoreLock()

	pushCommandSkill(t, registry, "child", "1.0.0", nil)
	rootRef := pushCommandSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
	lockfilePath := writeLockfileForRoot(t, registry, rootRef, "registry.example.com/agentskills")

	t.Run("lockfile only uses lockfile root", func(t *testing.T) {
		targetDir := t.TempDir()
		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{
			"install",
			"--lockfile", lockfilePath,
			"--target", targetDir,
		})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		for _, name := range []string{"root", "child"} {
			if _, err := os.Stat(filepath.Join(targetDir, name, "SKILL.md")); err != nil {
				t.Fatalf("Stat(%s/SKILL.md) error = %v", name, err)
			}
		}
	})

	t.Run("package name mixed form requires from and matching root name", func(t *testing.T) {
		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{
			"install",
			"root",
			"--lockfile", lockfilePath,
			"--target", t.TempDir(),
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want missing --from failure")
		}
		if !strings.Contains(err.Error(), "--from is required") {
			t.Fatalf("Execute() error = %q, want missing --from context", err)
		}
	})

	t.Run("package name mixed form rejects root mismatch", func(t *testing.T) {
		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{
			"install",
			"other",
			"--from", "registry.example.com/agentskills",
			"--lockfile", lockfilePath,
			"--target", t.TempDir(),
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want root mismatch failure")
		}
		if !strings.Contains(err.Error(), "does not match lockfile root") {
			t.Fatalf("Execute() error = %q, want root mismatch context", err)
		}
	})

	t.Run("artifact reference mixed form rejects canonical root mismatch", func(t *testing.T) {
		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{
			"install",
			"oci://registry.example.com/agentskills/root@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"--lockfile", lockfilePath,
			"--target", t.TempDir(),
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want canonical root mismatch failure")
		}
		if !strings.Contains(err.Error(), "does not match lockfile root") {
			t.Fatalf("Execute() error = %q, want root mismatch context", err)
		}
	})

	t.Run("package name mixed form rejects repository mismatch", func(t *testing.T) {
		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{
			"install",
			"root",
			"--from", "registry.example.com/other",
			"--lockfile", lockfilePath,
			"--target", t.TempDir(),
		})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want root mismatch failure")
		}
		if !strings.Contains(err.Error(), "does not match lockfile root") {
			t.Fatalf("Execute() error = %q, want root mismatch context", err)
		}
	})
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

func pushCommandSkill(t *testing.T, registry oci.Registry, name, version string, dependencies []manifestpkg.Dependency) string {
	t.Helper()
	return pushCommandSkillToRepository(t, registry, "registry.example.com/agentskills/"+name, name, version, dependencies)
}

func pushCommandSkillToRepository(t *testing.T, registry oci.Registry, repository, name, version string, dependencies []manifestpkg.Dependency) string {
	t.Helper()

	packagePath := buildNamedPackage(t, name, version, dependencies)
	ref, err := pushPackage(context.Background(), registry, packagePath, repository, "")
	if err != nil {
		t.Fatalf("pushPackage() error = %v", err)
	}
	return ref
}

func writeLockfileForRoot(t *testing.T, registry oci.Registry, reference, from string) string {
	t.Helper()

	outputPath := filepath.Join(t.TempDir(), "skill.lock.json")
	restore := swapLockRegistry(t, registry)
	defer restore()

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	args := []string{
		"lock",
		reference,
		"--output", outputPath,
	}
	if from != "" {
		args = append(args, "--from", from)
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(lock) error = %v", err)
	}
	return outputPath
}

func buildNamedPackage(t *testing.T, name, version string, dependencies []manifestpkg.Dependency) string {
	t.Helper()

	skillDir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(skillDir, ".skill"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.skill) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	manifestBytes, err := json.MarshalIndent(manifestpkg.Manifest{
		SchemaVersion: "v1",
		PackageType:   "skill",
		Name:          name,
		Version:       version,
		Skill:         manifestpkg.Skill{Entrypoint: "SKILL.md"},
		Dependencies:  dependencies,
	}, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(manifest) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, ".skill", "manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}

	packagePath := filepath.Join(t.TempDir(), name+".tgz")
	outputFile, err := os.Create(packagePath)
	if err != nil {
		t.Fatalf("Create(output) error = %v", err)
	}
	if err := packagex.BuildTGZ(outputFile, skillDir, packagex.BuildOptions{IncludeFilesSHA256: true}); err != nil {
		_ = outputFile.Close()
		t.Fatalf("BuildTGZ() error = %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("Close(output) error = %v", err)
	}
	return packagePath
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
