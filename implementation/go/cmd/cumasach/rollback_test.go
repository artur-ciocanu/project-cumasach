package main

import (
	"bytes"
	"strings"
	"testing"

	installpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/install"
	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func TestRollbackCommand(t *testing.T) {
	t.Run("rollback target succeeds", func(t *testing.T) {
		registry := oci.NewMemoryRegistry()
		restoreInstall := swapInstallRegistry(t, registry)
		restoreRollback := swapRollbackRegistry(t, registry)
		defer restoreInstall()
		defer restoreRollback()

		pushCommandSkill(t, registry, "child", "1.0.0", nil)
		pushCommandSkill(t, registry, "root", "1.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^1.0.0"}})

		targetDir := t.TempDir()
		runRootCommand(t,
			"install",
			"root",
			"--from", "registry.example.com/agentskills",
			"--target", targetDir,
		)

		pushCommandSkill(t, registry, "child", "2.0.0", nil)
		pushCommandSkill(t, registry, "root", "2.0.0", []manifestpkg.Dependency{{Name: "child", Version: "^2.0.0"}})
		runRootCommand(t,
			"install",
			"root",
			"--from", "registry.example.com/agentskills",
			"--target", targetDir,
		)

		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"rollback", "--target", targetDir})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		state, err := installpkg.LoadState(targetDir)
		if err != nil {
			t.Fatalf("LoadState() error = %v", err)
		}
		if got := activeSkillVersionFromState(t, state.Active, "root"); got != "1.0.0" {
			t.Fatalf("active root version = %q, want %q", got, "1.0.0")
		}
		if got := activeSkillVersionFromState(t, state.Active, "child"); got != "1.0.0" {
			t.Fatalf("active child version = %q, want %q", got, "1.0.0")
		}
		if got := state.History[len(state.History)-1].Action; got != "rollback" {
			t.Fatalf("newest history action = %q, want %q", got, "rollback")
		}
		if output := stdout.String(); !strings.Contains(output, "rolled back target "+targetDir) {
			t.Fatalf("stdout = %q, want rollback summary", output)
		}
	})

	t.Run("missing target fails", func(t *testing.T) {
		cmd := newRootCmd()
		var stdout bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stdout)
		cmd.SetArgs([]string{"rollback"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("Execute() error = nil, want failure")
		}
		if !strings.Contains(err.Error(), "--target is required") {
			t.Fatalf("Execute() error = %q, want missing target failure", err)
		}
	})
}

func swapRollbackRegistry(t *testing.T, registry oci.Registry) func() {
	t.Helper()

	previous := newRollbackRegistry
	newRollbackRegistry = func() oci.Registry {
		return registry
	}

	return func() {
		newRollbackRegistry = previous
	}
}

func activeSkillVersionFromState(t *testing.T, active []installpkg.ResolvedSkill, name string) string {
	t.Helper()
	for _, skill := range active {
		if skill.Name == name {
			return skill.Version
		}
	}
	t.Fatalf("active skill %q not found in %#v", name, active)
	return ""
}
