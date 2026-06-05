package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

const (
	trustBuilderID    = "https://github.com/example/builders/cumasach"
	trustSourceRepo   = "https://github.com/example/project-cumasach"
	trustCertIdentity = "https://github.com/example/workflows/release.yml@refs/heads/main"
	trustOIDCIssuer   = "https://token.actions.githubusercontent.com"
)

// pushVerifiableArtifact pushes a structurally valid skill artifact (config ==
// mirrored manifest, correct media types) into the registry and returns its
// canonical digest-pinned reference. Trust signing is not performed here; tests
// drive the cosign runner separately.
func pushVerifiableArtifact(t *testing.T, registry oci.Registry) string {
	t.Helper()

	archivePath := buildNamedPackage(t, "list-directory", "1.2.3", nil)
	archiveBytes, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("ReadFile(archivePath) error = %v", err)
	}
	mirroredManifestBytes, mirroredManifest, err := archivepkg.ReadMirroredManifestTGZ(bytes.NewReader(archiveBytes))
	if err != nil {
		t.Fatalf("ReadMirroredManifestTGZ() error = %v", err)
	}
	refValue, err := oci.Push(context.Background(), registry, "registry.example.com/agentskills/list-directory", mirroredManifestBytes, archiveBytes, oci.PushOptions{Tag: mirroredManifest.Version})
	if err != nil {
		t.Fatalf("oci.Push() error = %v", err)
	}
	return refValue.Canonical()
}

func runVerifyCommandErr(t *testing.T, ref, builderID, sourceRepo string) error {
	t.Helper()

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"verify",
		ref,
		"--certificate-identity", trustCertIdentity,
		"--certificate-oidc-issuer", trustOIDCIssuer,
		"--builder-id", builderID,
		"--source-repo", sourceRepo,
	})
	return cmd.Execute()
}

func TestVerifyCommandRejectsUnsignedArtifact(t *testing.T) {
	installFailingCosignRunner(t, cosignFailSignature)
	registry := oci.NewMemoryRegistry()
	restore := swapVerifyRegistry(t, registry)
	defer restore()

	ref := pushVerifiableArtifact(t, registry)
	if err := runVerifyCommandErr(t, ref, trustBuilderID, trustSourceRepo); err == nil {
		t.Fatal("verify of unsigned artifact succeeded, want failure")
	}
}

func TestVerifyCommandRejectsMissingProvenance(t *testing.T) {
	installFailingCosignRunner(t, cosignFailProvenance)
	registry := oci.NewMemoryRegistry()
	restore := swapVerifyRegistry(t, registry)
	defer restore()

	ref := pushVerifiableArtifact(t, registry)
	if err := runVerifyCommandErr(t, ref, trustBuilderID, trustSourceRepo); err == nil {
		t.Fatal("verify of artifact without provenance succeeded, want failure")
	}
}

func TestVerifyCommandRejectsBuilderMismatch(t *testing.T) {
	installFakeCosignRunner(t, trustBuilderID, trustSourceRepo)
	registry := oci.NewMemoryRegistry()
	restore := swapVerifyRegistry(t, registry)
	defer restore()

	ref := pushVerifiableArtifact(t, registry)
	if err := runVerifyCommandErr(t, ref, "https://github.com/example/builders/other", trustSourceRepo); err == nil {
		t.Fatal("verify with mismatched builder id succeeded, want failure")
	}
}

func TestVerifyCommandRejectsSourceRepoMismatch(t *testing.T) {
	installFakeCosignRunner(t, trustBuilderID, trustSourceRepo)
	registry := oci.NewMemoryRegistry()
	restore := swapVerifyRegistry(t, registry)
	defer restore()

	ref := pushVerifiableArtifact(t, registry)
	if err := runVerifyCommandErr(t, ref, trustBuilderID, "https://github.com/example/other-repo"); err == nil {
		t.Fatal("verify with mismatched source repository succeeded, want failure")
	}
}

func runInstallCommandErr(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := newRootCmd("test", "abc1234", "2026-01-01")
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestInstallCommandRejectsUnsignedArtifact(t *testing.T) {
	installFakeCosignRunner(t, trustBuilderID, trustSourceRepo)
	registry := oci.NewMemoryRegistry()
	restoreInstall := swapInstallRegistry(t, registry)
	restorePush := swapPushRegistry(t, registry)
	defer restoreInstall()
	defer restorePush()

	skillDir := buildDemoSkillDir(t, "workspace-notes", "1.0.0", nil)
	pkg := packageSkillWithCLI(t, skillDir)
	pushSkillWithCLI(t, pkg, "registry.example.com/agentskills/workspace-notes")

	// Swap to a runner that fails signature verification for the install step.
	installFailingCosignRunner(t, cosignFailSignature)

	targetDir := t.TempDir()
	_, err := runInstallCommandErr(t,
		"install",
		"workspace-notes",
		"--from", "registry.example.com/agentskills",
		"--target", targetDir,
		"--certificate-identity", trustCertIdentity,
		"--certificate-oidc-issuer", trustOIDCIssuer,
		"--builder-id", trustBuilderID,
		"--source-repo", trustSourceRepo,
	)
	if err == nil {
		t.Fatal("install of unsigned artifact succeeded, want trust failure")
	}
}

func TestInstallCommandBypassesTrustWithNoVerify(t *testing.T) {
	installFakeCosignRunner(t, trustBuilderID, trustSourceRepo)
	registry := oci.NewMemoryRegistry()
	restoreInstall := swapInstallRegistry(t, registry)
	restorePush := swapPushRegistry(t, registry)
	defer restoreInstall()
	defer restorePush()

	skillDir := buildDemoSkillDir(t, "workspace-notes", "1.0.0", nil)
	pkg := packageSkillWithCLI(t, skillDir)
	pushSkillWithCLI(t, pkg, "registry.example.com/agentskills/workspace-notes")

	// Even with a runner that would fail trust, --no-verify must skip it and the
	// structurally valid artifact must install.
	installFailingCosignRunner(t, cosignFailSignature)

	targetDir := t.TempDir()
	stdout, err := runInstallCommandErr(t,
		"install",
		"workspace-notes",
		"--from", "registry.example.com/agentskills",
		"--target", targetDir,
		"--no-verify",
	)
	if err != nil {
		t.Fatalf("install with --no-verify error = %v", err)
	}
	if want := "installed workspace-notes 1.0.0"; !bytes.Contains([]byte(stdout), []byte(want)) {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
