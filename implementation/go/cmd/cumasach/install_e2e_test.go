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
	unrelatedDir := buildDemoSkillDir(t, "scratchpad", "1.0.0", nil)
	unrelatedPackage := packageSkillWithCLI(t, unrelatedDir)
	pushSkillWithCLI(t, unrelatedPackage, "registry.example.com/agentskills/scratchpad")

	targetDir := t.TempDir()
	runRootCommand(t,
		"install",
		"scratchpad",
		"--from", "registry.example.com/agentskills",
		"--target", targetDir,
	)
	stdout := runRootCommand(t,
		"install",
		"workspace-summary",
		"--from", "registry.example.com/agentskills",
		"--target", targetDir,
	)

	if !strings.Contains(stdout, "installed workspace-summary 1.0.0") {
		t.Fatalf("stdout = %q, want install summary", stdout)
	}

	wantEntries := []string{".cumasach", "scratchpad", "workspace-notes", "workspace-summary"}
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
	if len(state.Active) != 3 {
		t.Fatalf("len(state.Active) = %d, want 3", len(state.Active))
	}

	activeNames := []string{state.Active[0].Name, state.Active[1].Name, state.Active[2].Name}
	sort.Strings(activeNames)
	if strings.Join(activeNames, ",") != "scratchpad,workspace-notes,workspace-summary" {
		t.Fatalf("active names = %v, want scratchpad, workspace-notes, and workspace-summary", activeNames)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "scratchpad")); err != nil {
		t.Fatalf("scratchpad should remain active after install, stat error = %v", err)
	}
	if len(state.History) != 2 {
		t.Fatalf("len(state.History) = %d, want 2", len(state.History))
	}
	if len(state.History[len(state.History)-1].Resolved) != 3 {
		t.Fatalf("len(newest history.Resolved) = %d, want 3", len(state.History[len(state.History)-1].Resolved))
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

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
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

func TestInstallLockfileEndToEnd(t *testing.T) {
	registry := oci.NewMemoryRegistry()
	restoreInstall := swapInstallRegistry(t, registry)
	restorePush := swapPushRegistry(t, registry)
	restoreLock := swapLockRegistry(t, registry)
	defer restoreInstall()
	defer restorePush()
	defer restoreLock()

	childDir := buildDemoSkillDir(t, "workspace-notes", "1.0.0", nil)
	rootDir := buildDemoSkillDir(t, "workspace-summary", "1.0.0", []manifestpkg.Dependency{
		{Name: "workspace-notes", Version: "^1.0.0"},
	})

	childPackage := packageSkillWithCLI(t, childDir)
	rootPackage := packageSkillWithCLI(t, rootDir)
	pushSkillWithCLI(t, childPackage, "registry.example.com/agentskills/workspace-notes")
	pushSkillWithCLI(t, rootPackage, "registry.example.com/agentskills/workspace-summary")
	unrelatedDir := buildDemoSkillDir(t, "scratchpad", "1.0.0", nil)
	unrelatedPackage := packageSkillWithCLI(t, unrelatedDir)
	pushSkillWithCLI(t, unrelatedPackage, "registry.example.com/agentskills/scratchpad")

	lockfilePath := filepath.Join(t.TempDir(), "skill.lock.json")
	lockStdout := runRootCommand(t,
		"lock",
		"workspace-summary",
		"--from", "registry.example.com/agentskills",
		"--output", lockfilePath,
	)
	if !strings.Contains(lockStdout, "locked workspace-summary") {
		t.Fatalf("lock stdout = %q, want lock summary", lockStdout)
	}

	lockTarget := t.TempDir()
	runRootCommand(t,
		"install",
		"scratchpad",
		"--from", "registry.example.com/agentskills",
		"--target", lockTarget,
	)
	installStdout := runRootCommand(t,
		"install",
		"--lockfile", lockfilePath,
		"--target", lockTarget,
	)
	if !strings.Contains(installStdout, "installed workspace-summary 1.0.0") {
		t.Fatalf("lockfile install stdout = %q, want install summary", installStdout)
	}

	liveTarget := t.TempDir()
	runRootCommand(t,
		"install",
		"scratchpad",
		"--from", "registry.example.com/agentskills",
		"--target", liveTarget,
	)
	runRootCommand(t,
		"install",
		"workspace-summary",
		"--from", "registry.example.com/agentskills",
		"--target", liveTarget,
	)

	if got, want := listDirNames(t, lockTarget), []string{".cumasach", "scratchpad", "workspace-notes", "workspace-summary"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("lock target entries = %v, want %v", got, want)
	}

	lockState, err := installpkg.LoadState(lockTarget)
	if err != nil {
		t.Fatalf("LoadState(lockTarget) error = %v", err)
	}
	liveState, err := installpkg.LoadState(liveTarget)
	if err != nil {
		t.Fatalf("LoadState(liveTarget) error = %v", err)
	}
	if len(lockState.History) != 2 || len(lockState.History[len(lockState.History)-1].Resolved) != len(lockState.Active) {
		t.Fatalf("lockfile state history = %#v, want newest snapshot matching active", lockState.History)
	}
	if normalizeResolved(lockState.Active) != normalizeResolved(liveState.Active) {
		t.Fatalf("lockfile active = %v, want live active %v", normalizeResolved(lockState.Active), normalizeResolved(liveState.Active))
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

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
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

func normalizeResolved(skills []installpkg.ResolvedSkill) string {
	parts := make([]string, 0, len(skills))
	for _, skill := range skills {
		parts = append(parts, skill.Name+"@"+skill.Reference)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
