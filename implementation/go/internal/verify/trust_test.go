package verify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const trustTestReference = "oci://registry.example.com/agentskills/list-directory@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

// scriptedCosignRunner is a CommandRunner whose behavior is settable per
// cosign subcommand, so tests can drive each fail-closed path.
type scriptedCosignRunner struct {
	verifyErr         error  // returned for "cosign verify"
	attestationStdout []byte // returned for "cosign verify-attestation" (may be empty)
	attestationErr    error
}

func (r scriptedCosignRunner) Run(_ context.Context, name string, args ...string) (CommandResult, error) {
	if name != "cosign" || len(args) == 0 {
		return CommandResult{}, nil
	}
	switch args[0] {
	case "verify":
		if r.verifyErr != nil {
			return CommandResult{Stderr: []byte("signature not found")}, r.verifyErr
		}
		return CommandResult{Stdout: []byte("{}\n")}, nil
	case "verify-attestation":
		if r.attestationErr != nil {
			return CommandResult{Stderr: []byte("attestation unreadable")}, r.attestationErr
		}
		return CommandResult{Stdout: r.attestationStdout}, nil
	default:
		return CommandResult{Stdout: []byte("{}\n")}, nil
	}
}

// provenanceEnvelope builds the cosign verify-attestation stdout (a newline
// terminated DSSE envelope) for the given digest/builder/source, mirroring the
// shape produced by SignPublishedArtifact and the CLI fake.
func provenanceEnvelope(t *testing.T, reference, builderID, sourceRepository string) []byte {
	t.Helper()
	digest := ""
	if index := strings.LastIndex(reference, "@sha256:"); index >= 0 {
		digest = reference[index+len("@sha256:"):]
	}
	payload, err := json.Marshal(map[string]any{
		"predicateType": "https://slsa.dev/provenance/v1",
		"subject": []map[string]any{
			{"digest": map[string]string{"sha256": digest}},
		},
		"predicate": map[string]any{
			"buildDefinition": map[string]any{
				"externalParameters": map[string]any{
					"sourceRepository": sourceRepository,
				},
				"resolvedDependencies": []map[string]any{
					{"uri": sourceRepository},
				},
			},
			"runDetails": map[string]any{
				"builder": map[string]any{"id": builderID},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal provenance payload: %v", err)
	}
	envelope, err := json.Marshal(map[string]string{
		"payload": base64.StdEncoding.EncodeToString(payload),
	})
	if err != nil {
		t.Fatalf("marshal provenance envelope: %v", err)
	}
	return append(envelope, '\n')
}

func validTrustPolicy() TrustPolicy {
	return TrustPolicy{
		CertificateIdentity:   "https://github.com/example/builder/.github/workflows/release.yml@refs/heads/main",
		CertificateOIDCIssuer: "https://token.actions.githubusercontent.com",
		BuilderID:             "https://github.com/example/builder",
		SourceRepository:      "https://github.com/example/skills",
	}
}

func installScriptedCosignRunner(t *testing.T, runner scriptedCosignRunner) {
	t.Helper()
	restore := SetCommandRunnerForTesting(runner)
	t.Cleanup(restore)
}

func TestVerifyTrustFailsWhenSignatureAbsent(t *testing.T) {
	installScriptedCosignRunner(t, scriptedCosignRunner{verifyErr: errors.New("exit status 1")})

	err := VerifyPublishedArtifactTrust(context.Background(), trustTestReference, validTrustPolicy())
	if err == nil {
		t.Fatal("VerifyPublishedArtifactTrust() error = nil, want signature failure")
	}
	if !strings.Contains(err.Error(), "verify signature") {
		t.Fatalf("error = %q, want signature context", err)
	}
}

func TestVerifyTrustFailsWhenProvenanceAbsent(t *testing.T) {
	installScriptedCosignRunner(t, scriptedCosignRunner{attestationStdout: []byte{}})

	err := VerifyPublishedArtifactTrust(context.Background(), trustTestReference, validTrustPolicy())
	if err == nil {
		t.Fatal("VerifyPublishedArtifactTrust() error = nil, want missing provenance failure")
	}
	if !strings.Contains(err.Error(), "no SLSA provenance attestation found") {
		t.Fatalf("error = %q, want missing provenance context", err)
	}
}

func TestVerifyTrustFailsWhenProvenanceUnreadable(t *testing.T) {
	installScriptedCosignRunner(t, scriptedCosignRunner{attestationErr: errors.New("exit status 1")})

	err := VerifyPublishedArtifactTrust(context.Background(), trustTestReference, validTrustPolicy())
	if err == nil {
		t.Fatal("VerifyPublishedArtifactTrust() error = nil, want unreadable provenance failure")
	}
	if !strings.Contains(err.Error(), "verify provenance attestation") {
		t.Fatalf("error = %q, want provenance attestation context", err)
	}
}

func TestVerifyTrustFailsOnBuilderMismatch(t *testing.T) {
	policy := validTrustPolicy()
	installScriptedCosignRunner(t, scriptedCosignRunner{
		attestationStdout: provenanceEnvelope(t, trustTestReference, "wrong-builder", policy.SourceRepository),
	})

	err := VerifyPublishedArtifactTrust(context.Background(), trustTestReference, policy)
	if err == nil {
		t.Fatal("VerifyPublishedArtifactTrust() error = nil, want builder mismatch failure")
	}
	if !strings.Contains(err.Error(), "builder") {
		t.Fatalf("error = %q, want builder context", err)
	}
}

func TestVerifyTrustFailsOnSourceRepoMismatch(t *testing.T) {
	policy := validTrustPolicy()
	installScriptedCosignRunner(t, scriptedCosignRunner{
		attestationStdout: provenanceEnvelope(t, trustTestReference, policy.BuilderID, "wrong-repo"),
	})

	err := VerifyPublishedArtifactTrust(context.Background(), trustTestReference, policy)
	if err == nil {
		t.Fatal("VerifyPublishedArtifactTrust() error = nil, want source repository mismatch failure")
	}
	if !strings.Contains(err.Error(), "source repository") {
		t.Fatalf("error = %q, want source repository context", err)
	}
}

func TestVerifyTrustSucceedsForValidSignatureAndProvenance(t *testing.T) {
	policy := validTrustPolicy()
	installScriptedCosignRunner(t, scriptedCosignRunner{
		attestationStdout: provenanceEnvelope(t, trustTestReference, policy.BuilderID, policy.SourceRepository),
	})

	if err := VerifyPublishedArtifactTrust(context.Background(), trustTestReference, policy); err != nil {
		t.Fatalf("VerifyPublishedArtifactTrust() error = %v, want nil", err)
	}
}
