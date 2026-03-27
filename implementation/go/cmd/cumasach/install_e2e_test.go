package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	installpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/install"
	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/packagex"
)

func TestInstallCommandEndToEndPackagesPushesAndInstallsDependencies(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restoreInstall := swapInstallRegistry(t, registry)
	restorePush := swapPushRegistry(t, registry)
	defer restoreInstall()
	defer restorePush()

	childDir := buildDemoSkillDir(t, "workspace-notes", "1.0.0", nil)
	rootDir := buildDemoSkillDir(t, "workspace-summary", "1.0.0", []manifestpkg.Dependency{
		{Name: "workspace-notes", Version: "^1.0.0"},
	})

	childPackage := packageSkillWithCLI(t, childDir)
	rootPackage := packageSkillWithCLI(t, rootDir)
	pushSkillWithCLI(t, childPackage, "registry.example.com/agentskills/workspace-notes")
	pushSkillWithCLI(t, rootPackage, "registry.example.com/agentskills/workspace-summary")

	targetDir := t.TempDir()
	stdout := runRootCommand(t,
		"install",
		"workspace-summary",
		"--from", "registry.example.com/agentskills",
		"--target", targetDir,
	)

	if !strings.Contains(stdout, "installed workspace-summary 1.0.0") {
		t.Fatalf("stdout = %q, want install summary", stdout)
	}

	wantEntries := []string{".cumasach", "workspace-notes", "workspace-summary"}
	gotEntries := listDirNames(t, targetDir)
	if len(gotEntries) != len(wantEntries) {
		t.Fatalf("target entries = %v, want %v", gotEntries, wantEntries)
	}
	for i, want := range wantEntries {
		if gotEntries[i] != want {
			t.Fatalf("target entries = %v, want %v", gotEntries, wantEntries)
		}
	}

	state, err := installpkg.LoadState(targetDir)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}
	if len(state.Active) != 2 {
		t.Fatalf("len(state.Active) = %d, want 2", len(state.Active))
	}

	activeNames := []string{state.Active[0].Name, state.Active[1].Name}
	sort.Strings(activeNames)
	if strings.Join(activeNames, ",") != "workspace-notes,workspace-summary" {
		t.Fatalf("active names = %v, want workspace-notes and workspace-summary", activeNames)
	}
	if len(state.History) != 1 {
		t.Fatalf("len(state.History) = %d, want 1", len(state.History))
	}
	if len(state.History[0].Resolved) != 2 {
		t.Fatalf("len(history[0].Resolved) = %d, want 2", len(state.History[0].Resolved))
	}
	for _, resolved := range state.Active {
		if resolved.Reference == "" || resolved.Digest == "" {
			t.Fatalf("resolved skill = %#v, want reference and digest", resolved)
		}
	}
}

func TestInstallCommandEndToEndFailsWhenDependencyOnlyHasNonSemverTags(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restoreInstall := swapInstallRegistry(t, registry)
	restorePush := swapPushRegistry(t, registry)
	defer restoreInstall()
	defer restorePush()

	childDir := buildDemoSkillDir(t, "workspace-notes", "1.0.0", nil)
	rootDir := buildDemoSkillDir(t, "workspace-summary", "1.0.0", []manifestpkg.Dependency{
		{Name: "workspace-notes", Version: "^1.0.0"},
	})

	childPackage := packageSkillWithCLI(t, childDir)
	rootPackage := packageSkillWithCLI(t, rootDir)
	pushSkillWithCLI(t, childPackage, "registry.example.com/agentskills/workspace-notes", "--tag", "latest")
	pushSkillWithCLI(t, rootPackage, "registry.example.com/agentskills/workspace-summary")

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"install",
		"workspace-summary",
		"--from", "registry.example.com/agentskills",
		"--target", t.TempDir(),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want non-semver dependency failure")
	}
	if !strings.Contains(err.Error(), "workspace-notes") {
		t.Fatalf("Execute() error = %q, want dependency context", err)
	}
}

func packageSkillWithCLI(t *testing.T, skillDir string) string {
	t.Helper()

	outputPath := filepath.Join(t.TempDir(), filepath.Base(skillDir)+".tgz")
	runRootCommand(t,
		"package",
		skillDir,
		"--output", outputPath,
		"--files-sha256",
	)
	return outputPath
}

func pushSkillWithCLI(t *testing.T, packagePath, repository string, extraArgs ...string) string {
	t.Helper()

	args := []string{"push", packagePath, repository}
	args = append(args, extraArgs...)
	return strings.TrimSpace(runRootCommand(t, args...))
}

func runRootCommand(t *testing.T, args ...string) string {
	t.Helper()

	cmd := newRootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	return stdout.String()
}

func buildDemoSkillDir(t *testing.T, name, version string, dependencies []manifestpkg.Dependency) string {
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

	outputFile, err := os.Create(filepath.Join(t.TempDir(), name+".tgz"))
	if err != nil {
		t.Fatalf("Create(temp archive) error = %v", err)
	}
	if err := packagex.BuildTGZ(outputFile, skillDir, packagex.BuildOptions{IncludeFilesSHA256: true}); err != nil {
		_ = outputFile.Close()
		t.Fatalf("BuildTGZ() error = %v", err)
	}
	if err := outputFile.Close(); err != nil {
		t.Fatalf("Close(temp archive) error = %v", err)
	}

	return skillDir
}

func listDirNames(t *testing.T, root string) []string {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", root, err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}
