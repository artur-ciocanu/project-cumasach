package verify

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	cosignattest "github.com/sigstore/cosign/v3/cmd/cosign/cli/attest"
	cosignopts "github.com/sigstore/cosign/v3/cmd/cosign/cli/options"
	cosignsign "github.com/sigstore/cosign/v3/cmd/cosign/cli/sign"
	cosignverify "github.com/sigstore/cosign/v3/cmd/cosign/cli/verify"
)

type cosignRunner struct{}

func (cosignRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	if name != "cosign" {
		return CommandResult{}, fmt.Errorf("unsupported command %q", name)
	}
	if len(args) == 0 {
		return CommandResult{}, fmt.Errorf("missing cosign subcommand")
	}

	return captureCommandOutput(func() error {
		switch args[0] {
		case "sign":
			return runCosignSign(ctx, args[1:])
		case "attest":
			return runCosignAttest(ctx, args[1:])
		case "verify":
			return runCosignVerify(ctx, args[1:])
		case "verify-attestation":
			return runCosignVerifyAttestation(ctx, args[1:])
		default:
			return fmt.Errorf("unsupported cosign subcommand %q", args[0])
		}
	})
}

func captureCommandOutput(run func() error) (CommandResult, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return CommandResult{}, err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		_ = stdoutReader.Close()
		_ = stdoutWriter.Close()
		return CommandResult{}, err
	}

	origStdout := os.Stdout
	origStderr := os.Stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	runErr := run()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	stdoutBytes, stdoutErr := io.ReadAll(stdoutReader)
	stderrBytes, stderrErr := io.ReadAll(stderrReader)
	_ = stdoutReader.Close()
	_ = stderrReader.Close()
	if stdoutErr != nil {
		return CommandResult{}, stdoutErr
	}
	if stderrErr != nil {
		return CommandResult{}, stderrErr
	}
	return CommandResult{Stdout: stdoutBytes, Stderr: stderrBytes}, runErr
}

func runCosignSign(ctx context.Context, args []string) error {
	_, _, ref, err := parseSignArgs(args)
	if err != nil {
		return err
	}

	rootOpts := &cosignopts.RootOptions{Timeout: 3 * time.Minute}
	keyOpts := cosignopts.KeyOpts{
		NewBundleFormat: true,
	}
	signOpts := cosignopts.SignOptions{
		Upload:                  true,
		SkipConfirmation:        true,
		TlogUpload:              true,
		NewBundleFormat:         true,
		UseSigningConfig:        true,
		IssueCertificate:        false,
		TrustedRootPath:         "",
		BundlePath:              "",
		Output:                  "",
		OutputSignature:         "",
		OutputCertificate:       "",
		OutputPayload:           "",
		Registry:                cosignopts.RegistryOptions{},
		Attachment:              "",
		PayloadPath:             "",
		Recursive:               false,
		RecordCreationTimestamp: false,
	}

	return cosignsign.SignCmd(ctx, rootOpts, keyOpts, signOpts, []string{ref})
}

func runCosignAttest(ctx context.Context, args []string) error {
	predicatePath, predicateType, ref, err := parseAttestArgs(args)
	if err != nil {
		return err
	}

	cmd := cosignattest.AttestCommand{
		KeyOpts: cosignopts.KeyOpts{
			NewBundleFormat: true,
		},
		RegistryOptions: cosignopts.RegistryOptions{},
		PredicatePath:   predicatePath,
		PredicateType:   predicateType,
		NoUpload:        false,
		TlogUpload:      true,
		RekorEntryType:  "dsse",
	}
	return cmd.Exec(ctx, ref)
}

func runCosignVerify(ctx context.Context, args []string) error {
	ref, identity, issuer, err := parseVerifyArgs(args)
	if err != nil {
		return err
	}

	cmd := cosignverify.VerifyCommand{
		CertVerifyOptions: cosignopts.CertVerifyOptions{
			CertIdentity:   identity,
			CertOidcIssuer: issuer,
		},
		CommonVerifyOptions: cosignopts.CommonVerifyOptions{
			NewBundleFormat: true,
			MaxWorkers:      1,
		},
		CheckClaims: true,
		Output:      "json",
	}
	return cmd.Exec(ctx, []string{ref})
}

func runCosignVerifyAttestation(ctx context.Context, args []string) error {
	ref, predicateType, identity, issuer, err := parseVerifyAttestationArgs(args)
	if err != nil {
		return err
	}

	cmd := cosignverify.VerifyAttestationCommand{
		CertVerifyOptions: cosignopts.CertVerifyOptions{
			CertIdentity:   identity,
			CertOidcIssuer: issuer,
		},
		CommonVerifyOptions: cosignopts.CommonVerifyOptions{
			NewBundleFormat: true,
			MaxWorkers:      1,
		},
		CheckClaims:   true,
		PredicateType: predicateType,
		Output:        "text",
	}
	return cmd.Exec(ctx, []string{ref})
}

func parseSignArgs(args []string) (skipConfirmation bool, certificateIdentity, ref string, err error) {
	for _, arg := range args {
		switch arg {
		case "--yes":
			skipConfirmation = true
		default:
			if strings.HasPrefix(arg, "-") {
				return false, "", "", fmt.Errorf("unsupported sign flag %q", arg)
			}
			ref = arg
		}
	}
	if ref == "" {
		return false, "", "", fmt.Errorf("sign requires a reference")
	}
	return skipConfirmation, certificateIdentity, ref, nil
}

func parseAttestArgs(args []string) (predicatePath, predicateType, ref string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--yes":
		case "--type":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("missing value for --type")
			}
			predicateType = args[i]
		case "--predicate":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("missing value for --predicate")
			}
			predicatePath = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", "", fmt.Errorf("unsupported attest flag %q", args[i])
			}
			ref = args[i]
		}
	}
	if predicatePath == "" || predicateType == "" || ref == "" {
		return "", "", "", fmt.Errorf("attest requires --predicate, --type, and a reference")
	}
	return predicatePath, predicateType, ref, nil
}

func parseVerifyArgs(args []string) (ref, identity, issuer string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--certificate-identity":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("missing value for --certificate-identity")
			}
			identity = args[i]
		case "--certificate-oidc-issuer":
			i++
			if i >= len(args) {
				return "", "", "", fmt.Errorf("missing value for --certificate-oidc-issuer")
			}
			issuer = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", "", fmt.Errorf("unsupported verify flag %q", args[i])
			}
			ref = args[i]
		}
	}
	if ref == "" {
		return "", "", "", fmt.Errorf("verify requires a reference")
	}
	return ref, identity, issuer, nil
}

func parseVerifyAttestationArgs(args []string) (ref, predicateType, identity, issuer string, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--type":
			i++
			if i >= len(args) {
				return "", "", "", "", fmt.Errorf("missing value for --type")
			}
			predicateType = args[i]
		case "--certificate-identity":
			i++
			if i >= len(args) {
				return "", "", "", "", fmt.Errorf("missing value for --certificate-identity")
			}
			identity = args[i]
		case "--certificate-oidc-issuer":
			i++
			if i >= len(args) {
				return "", "", "", "", fmt.Errorf("missing value for --certificate-oidc-issuer")
			}
			issuer = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", "", "", fmt.Errorf("unsupported verify-attestation flag %q", args[i])
			}
			ref = args[i]
		}
	}
	if ref == "" || predicateType == "" {
		return "", "", "", "", fmt.Errorf("verify-attestation requires --type and a reference")
	}
	return ref, predicateType, identity, issuer, nil
}
