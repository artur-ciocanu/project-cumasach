package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	verifypkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/verify"
)

type fakeCosignRunner struct {
	builderID        string
	sourceRepository string
}

func (r fakeCosignRunner) Run(_ context.Context, name string, args ...string) (verifypkg.CommandResult, error) {
	if name != "cosign" {
		return verifypkg.CommandResult{}, nil
	}
	if len(args) == 0 {
		return verifypkg.CommandResult{}, nil
	}

	switch args[0] {
	case "sign", "verify", "attest":
		return verifypkg.CommandResult{Stdout: []byte("{}\n")}, nil
	case "verify-attestation":
		ref := ""
		for _, arg := range args[1:] {
			if strings.Contains(arg, "@sha256:") {
				ref = strings.TrimPrefix(arg, "oci://")
				break
			}
		}
		digest := ""
		if index := strings.LastIndex(ref, "@sha256:"); index >= 0 {
			digest = ref[index+len("@sha256:"):]
		}
		payload, err := json.Marshal(map[string]any{
			"predicateType": "https://slsa.dev/provenance/v1",
			"subject": []map[string]any{
				{
					"digest": map[string]string{
						"sha256": digest,
					},
				},
			},
			"predicate": map[string]any{
				"buildDefinition": map[string]any{
					"externalParameters": map[string]any{
						"sourceRepository": r.sourceRepository,
					},
					"resolvedDependencies": []map[string]any{
						{"uri": r.sourceRepository},
					},
				},
				"runDetails": map[string]any{
					"builder": map[string]any{
						"id": r.builderID,
					},
				},
			},
		})
		if err != nil {
			return verifypkg.CommandResult{}, err
		}
		envelope, err := json.Marshal(map[string]string{
			"payload": base64.StdEncoding.EncodeToString(payload),
		})
		if err != nil {
			return verifypkg.CommandResult{}, err
		}
		return verifypkg.CommandResult{Stdout: append(envelope, '\n')}, nil
	default:
		return verifypkg.CommandResult{Stdout: []byte("{}\n")}, nil
	}
}

func installFakeCosignRunner(t *testing.T, builderID, sourceRepository string) {
	t.Helper()
	restore := verifypkg.SetCommandRunnerForTesting(fakeCosignRunner{
		builderID:        builderID,
		sourceRepository: sourceRepository,
	})
	t.Cleanup(restore)
}
