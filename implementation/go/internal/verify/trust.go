package verify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

type TrustPolicy struct {
	NoVerify              bool
	CertificateIdentity   string
	CertificateOIDCIssuer string
	BuilderID             string
	SourceRepository      string
}

func (p TrustPolicy) ValidateForOCI() error {
	if p.NoVerify {
		return nil
	}

	missing := make([]string, 0, 4)
	if strings.TrimSpace(p.CertificateIdentity) == "" {
		missing = append(missing, "--certificate-identity")
	}
	if strings.TrimSpace(p.CertificateOIDCIssuer) == "" {
		missing = append(missing, "--certificate-oidc-issuer")
	}
	if strings.TrimSpace(p.BuilderID) == "" {
		missing = append(missing, "--builder-id")
	}
	if strings.TrimSpace(p.SourceRepository) == "" {
		missing = append(missing, "--source-repo")
	}
	if len(missing) > 0 {
		return fmt.Errorf("verification requires %s unless --no-verify is set", strings.Join(missing, ", "))
	}
	return nil
}

type CommandResult struct {
	Stdout []byte
	Stderr []byte
}

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

var cosignCommands CommandRunner = cosignRunner{}

func SetCommandRunnerForTesting(runner CommandRunner) func() {
	previous := cosignCommands
	cosignCommands = runner
	return func() {
		cosignCommands = previous
	}
}

func VerifyPublishedArtifactTrust(ctx context.Context, reference string, policy TrustPolicy) error {
	if policy.NoVerify {
		return nil
	}
	if err := policy.ValidateForOCI(); err != nil {
		return err
	}

	artifactRef := strings.TrimPrefix(reference, "oci://")
	if _, err := runCosign(ctx, "verify",
		artifactRef,
		"--certificate-identity", policy.CertificateIdentity,
		"--certificate-oidc-issuer", policy.CertificateOIDCIssuer,
	); err != nil {
		return fmt.Errorf("verify signature for %q: %w", reference, err)
	}

	attestStdout, err := runCosign(ctx, "verify-attestation",
		artifactRef,
		"--type", "slsaprovenance",
		"--certificate-identity", policy.CertificateIdentity,
		"--certificate-oidc-issuer", policy.CertificateOIDCIssuer,
	)
	if err != nil {
		return fmt.Errorf("verify provenance attestation for %q: %w", reference, err)
	}

	statements, err := decodeAttestationStatements(attestStdout)
	if err != nil {
		return fmt.Errorf("decode provenance attestation for %q: %w", reference, err)
	}
	if len(statements) == 0 {
		return fmt.Errorf("no SLSA provenance attestation found for %q", reference)
	}

	ref, err := oci.ParseReference(reference)
	if err != nil {
		return fmt.Errorf("parse artifact reference for trust verification: %w", err)
	}

	for _, statement := range statements {
		if !strings.Contains(statement.PredicateType, "slsa.dev/provenance") {
			continue
		}
		if !statement.hasSubjectDigest(ref.Digest) {
			continue
		}
		if statement.BuilderID() != policy.BuilderID {
			continue
		}
		if statement.SourceRepository() != policy.SourceRepository {
			continue
		}
		return nil
	}

	return fmt.Errorf("no verified provenance statement matched digest %q, builder %q, and source repository %q", ref.Digest, policy.BuilderID, policy.SourceRepository)
}

func SignPublishedArtifact(ctx context.Context, reference string, policy TrustPolicy) error {
	if err := policy.ValidateForOCI(); err != nil {
		return err
	}

	artifactRef := strings.TrimPrefix(reference, "oci://")
	if _, err := runCosign(ctx, "sign", "--yes", artifactRef); err != nil {
		return fmt.Errorf("sign %q: %w", reference, err)
	}

	predicatePath, err := writeProvenancePredicate(policy, reference, time.Now().UTC())
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(predicatePath) }()

	if _, err := runCosign(ctx, "attest",
		"--yes",
		"--type", "slsaprovenance",
		"--predicate", predicatePath,
		artifactRef,
	); err != nil {
		return fmt.Errorf("attest %q: %w", reference, err)
	}

	return nil
}

func runCosign(ctx context.Context, args ...string) ([]byte, error) {
	result, err := cosignCommands.Run(ctx, "cosign", args...)
	if err != nil {
		stderr := strings.TrimSpace(string(result.Stderr))
		if stderr == "" {
			stderr = strings.TrimSpace(string(result.Stdout))
		}
		if stderr == "" {
			stderr = err.Error()
		}
		return nil, fmt.Errorf("cosign %s: %s", strings.Join(args, " "), stderr)
	}
	return result.Stdout, nil
}

type provenancePredicate struct {
	BuildDefinition struct {
		BuildType          string                 `json:"buildType"`
		ExternalParameters map[string]interface{} `json:"externalParameters"`
		ResolvedDeps       []resourceDescriptor   `json:"resolvedDependencies,omitempty"`
	} `json:"buildDefinition"`
	RunDetails struct {
		Builder struct {
			ID string `json:"id"`
		} `json:"builder"`
		Metadata struct {
			InvocationID string `json:"invocationId"`
			StartedOn    string `json:"startedOn"`
			FinishedOn   string `json:"finishedOn"`
		} `json:"metadata"`
	} `json:"runDetails"`
}

type resourceDescriptor struct {
	URI string `json:"uri,omitempty"`
}

func writeProvenancePredicate(policy TrustPolicy, reference string, now time.Time) (string, error) {
	predicate := provenancePredicate{}
	predicate.BuildDefinition.BuildType = "https://cumasach.dev/buildtypes/oci-push/v1"
	predicate.BuildDefinition.ExternalParameters = map[string]interface{}{
		"sourceRepository": policy.SourceRepository,
	}
	predicate.BuildDefinition.ResolvedDeps = []resourceDescriptor{{URI: policy.SourceRepository}}
	predicate.RunDetails.Builder.ID = policy.BuilderID
	predicate.RunDetails.Metadata.InvocationID = reference
	predicate.RunDetails.Metadata.StartedOn = now.Format(time.RFC3339)
	predicate.RunDetails.Metadata.FinishedOn = now.Format(time.RFC3339)

	body, err := json.Marshal(predicate)
	if err != nil {
		return "", fmt.Errorf("marshal provenance predicate: %w", err)
	}

	file, err := os.CreateTemp("", "cumasach-provenance-*.json")
	if err != nil {
		return "", fmt.Errorf("create provenance predicate file: %w", err)
	}
	path := file.Name()
	if _, err := file.Write(body); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write provenance predicate file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close provenance predicate file: %w", err)
	}
	return path, nil
}

type attestationEnvelope struct {
	Payload string `json:"payload"`
}

type attestationStatement struct {
	PredicateType string `json:"predicateType"`
	Subject       []struct {
		Digest map[string]string `json:"digest"`
	} `json:"subject"`
	Predicate struct {
		BuildDefinition struct {
			ExternalParameters map[string]interface{} `json:"externalParameters"`
			ResolvedDeps       []resourceDescriptor   `json:"resolvedDependencies,omitempty"`
		} `json:"buildDefinition"`
		RunDetails struct {
			Builder struct {
				ID string `json:"id"`
			} `json:"builder"`
		} `json:"runDetails"`
	} `json:"predicate"`
}

func (s attestationStatement) BuilderID() string {
	return s.Predicate.RunDetails.Builder.ID
}

func (s attestationStatement) SourceRepository() string {
	if value, ok := s.Predicate.BuildDefinition.ExternalParameters["sourceRepository"]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}

	for _, dep := range s.Predicate.BuildDefinition.ResolvedDeps {
		if dep.URI != "" {
			return dep.URI
		}
	}
	return ""
}

func (s attestationStatement) hasSubjectDigest(digest string) bool {
	want := strings.TrimPrefix(digest, "sha256:")
	for _, subject := range s.Subject {
		if subject.Digest["sha256"] == want {
			return true
		}
	}
	return false
}

func decodeAttestationStatements(stdout []byte) ([]attestationStatement, error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if trimmed[0] == '[' {
		var envelopes []attestationEnvelope
		if err := json.Unmarshal(trimmed, &envelopes); err != nil {
			return nil, fmt.Errorf("decode attestation envelope array: %w", err)
		}
		return decodeEnvelopes(envelopes)
	}

	lines := bytes.Split(trimmed, []byte("\n"))
	envelopes := make([]attestationEnvelope, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var envelope attestationEnvelope
		if err := json.Unmarshal(line, &envelope); err != nil {
			return nil, fmt.Errorf("decode attestation envelope: %w", err)
		}
		envelopes = append(envelopes, envelope)
	}
	return decodeEnvelopes(envelopes)
}

func decodeEnvelopes(envelopes []attestationEnvelope) ([]attestationStatement, error) {
	statements := make([]attestationStatement, 0, len(envelopes))
	for _, envelope := range envelopes {
		payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode attestation payload: %w", err)
		}

		var statement attestationStatement
		if err := json.Unmarshal(payload, &statement); err != nil {
			return nil, fmt.Errorf("decode attestation statement: %w", err)
		}
		statements = append(statements, statement)
	}
	return statements, nil
}
