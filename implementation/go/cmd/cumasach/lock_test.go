package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/lockfile"
	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func TestLockCommand(t *testing.T) {
	t.Run("lock artifact reference succeeds", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restore := swapLockRegistry(t, registry)
		defer restore()

		pushCommandSkill(t, registry, "child", "1.0.0", nil)
		rootRef := pushCommandSkill(t, registry, "root", "1.0.0", nil)
		outputPath := filepath.Join(t.TempDir(), "root.lock.json")

		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"lock", rootRef, "--output", outputPath})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		loaded := loadWrittenLockfile(t, outputPath)
		if loaded.Root.Name != "root" {
			t.Fatalf("Root.Name = %q, want %q", loaded.Root.Name, "root")
		}
		if output := stdout.String(); !strings.Contains(output, "locked root to "+outputPath) {
			t.Fatalf("stdout = %q, want success summary", output)
		}
	})

	t.Run("lock artifact reference with dependencies uses explicit from", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restore := swapLockRegistry(t, registry)
		defer restore()

		pushCommandSkillToRepository(t, registry, "registry.example.com/catalog/child", "child", "1.0.0", nil)
		rootRef := pushCommandSkillToRepository(t, registry, "registry.example.com/published/root-artifact", "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})
		outputPath := filepath.Join(t.TempDir(), "root.lock.json")

		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"lock", rootRef, "--from", "registry.example.com/catalog", "--output", outputPath})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		loaded := loadWrittenLockfile(t, outputPath)
		foundChild := false
		for _, pkg := range loaded.Packages {
			if pkg.Name == "child" {
				foundChild = true
				break
			}
		}
		if !foundChild {
			t.Fatalf("lockfile packages = %#v, want child dependency", loaded.Packages)
		}
	})

	t.Run("lock package name with from succeeds", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restore := swapLockRegistry(t, registry)
		defer restore()

		pushCommandSkill(t, registry, "child", "1.0.0", nil)
		outputPath := filepath.Join(t.TempDir(), "named.lock.json")
		pushCommandSkill(t, registry, "root", "1.0.0", nil)

		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"lock", "root", "--from", "registry.example.com/agentskills", "--output", outputPath})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		loaded := loadWrittenLockfile(t, outputPath)
		if loaded.Root.Name != "root" {
			t.Fatalf("Root.Name = %q, want %q", loaded.Root.Name, "root")
		}
		if output := stdout.String(); !strings.Contains(output, "locked root to "+outputPath) {
			t.Fatalf("stdout = %q, want success summary", output)
		}
	})

	t.Run("package name without from fails", func(t *testing.T) {
		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"lock", "root"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "--from is required") {
			t.Fatalf("Execute() error = %q, want missing --from failure", err)
		}
	})

	t.Run("output defaults to skill lock json", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restore := swapLockRegistry(t, registry)
		defer restore()

		rootRef := pushCommandSkill(t, registry, "root", "1.0.0", nil)
		workingDir := t.TempDir()
		previousWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd() error = %v", err)
		}
		if err := os.Chdir(workingDir); err != nil {
			t.Fatalf("Chdir() error = %v", err)
		}
		t.Cleanup(func() {
			if err := os.Chdir(previousWD); err != nil {
				t.Fatalf("restore Chdir() error = %v", err)
			}
		})

		cmd := newRootCmd("test", "abc1234", "2026-01-01")
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"lock", rootRef})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		outputPath := filepath.Join(workingDir, "skill.lock.json")
		loadWrittenLockfile(t, outputPath)
		if output := stdout.String(); !strings.Contains(output, "locked root to ./skill.lock.json") {
			t.Fatalf("stdout = %q, want default output path in success summary", output)
		}
	})

	t.Run("plain oci and canonical inputs are both accepted", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restore := swapLockRegistry(t, registry)
		defer restore()

		rootRef := pushCommandSkill(t, registry, "root", "1.0.0", nil)
		plainRef := strings.TrimPrefix(rootRef, "oci://")

		for name, input := range map[string]string{
			"plain":     plainRef,
			"canonical": rootRef,
		} {
			t.Run(name, func(t *testing.T) {
				outputPath := filepath.Join(t.TempDir(), name+".lock.json")
				cmd := newRootCmd("test", "abc1234", "2026-01-01")
				var stdout bytes.Buffer
				cmd.SetOut(&stdout)
				cmd.SetErr(&stdout)
				cmd.SetArgs([]string{"lock", input, "--output", outputPath})

				if err := cmd.Execute(); err != nil {
					t.Fatalf("Execute() error = %v", err)
				}

				loaded := loadWrittenLockfile(t, outputPath)
				if loaded.Root.Reference != rootRef {
					t.Fatalf("Root.Reference = %q, want %q", loaded.Root.Reference, rootRef)
				}
			})
		}
	})
}

func loadWrittenLockfile(t *testing.T, path string) lockfile.File {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	loaded, err := lockfile.LoadReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("LoadReader(%q) error = %v", path, err)
	}

	return loaded
}

func swapLockRegistry(t *testing.T, registry oci.Registry) func() {
	t.Helper()

	previous := newLockRegistry
	newLockRegistry = func() oci.Registry {
		return registry
	}

	return func() {
		newLockRegistry = previous
	}
}
