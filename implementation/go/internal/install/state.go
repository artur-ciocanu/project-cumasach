package install

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/xeipuuv/gojsonschema"
)

const (
	SchemaVersion = "v1"
	metadataDir   = ".cumasach"
	stateFileName = "install-state.json"
)

//go:embed install-state-v1.schema.json
var schemaBytes []byte

type State struct {
	SchemaVersion string          `json:"schemaVersion"`
	Target        Target          `json:"target"`
	Active        []ResolvedSkill `json:"active"`
	History       []HistoryEntry  `json:"history"`
}

type Target struct {
	Path    string `json:"path"`
	Runtime string `json:"runtime,omitempty"`
}

type ResolvedSkill struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Digest    string `json:"digest"`
	Reference string `json:"reference"`
}

type HistoryEntry struct {
	Timestamp string          `json:"timestamp"`
	Action    string          `json:"action"`
	Resolved  []ResolvedSkill `json:"resolved"`
}

func StatePath(targetDir string) string {
	return filepath.Join(targetDir, metadataDir, stateFileName)
}

func LoadState(targetDir string) (State, error) {
	data, err := os.ReadFile(StatePath(targetDir))
	if err != nil {
		return State{}, fmt.Errorf("read install state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode install state JSON: %w", err)
	}

	if err := validateState(data); err != nil {
		return State{}, err
	}
	if err := validateStateSemantics(state); err != nil {
		return State{}, err
	}

	return state, nil
}

func LoadStateIfExists(targetDir string) (State, bool, error) {
	data, err := os.ReadFile(StatePath(targetDir))
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, false, nil
		}
		return State{}, false, fmt.Errorf("read install state: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false, fmt.Errorf("decode install state JSON: %w", err)
	}

	if err := validateState(data); err != nil {
		return State{}, false, err
	}
	if err := validateStateSemantics(state); err != nil {
		return State{}, false, err
	}

	return state, true, nil
}

func WriteState(targetDir string, state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal install state JSON: %w", err)
	}

	if err := validateState(data); err != nil {
		return err
	}
	if err := validateStateSemantics(state); err != nil {
		return err
	}

	stateDir := filepath.Dir(StatePath(targetDir))
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("create install state directory: %w", err)
	}

	tempFile, err := os.CreateTemp(stateDir, ".install-state-*.json")
	if err != nil {
		return fmt.Errorf("create temporary install state file: %w", err)
	}

	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("write install state: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close install state: %w", err)
	}

	if err := os.Rename(tempPath, StatePath(targetDir)); err != nil {
		return fmt.Errorf("move install state into place: %w", err)
	}

	return nil
}

func validateState(data []byte) error {
	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schemaBytes),
		gojsonschema.NewBytesLoader(data),
	)
	if err != nil {
		return fmt.Errorf("validate install state against schema: %w", err)
	}
	if result.Valid() {
		return nil
	}

	messages := make([]string, 0, len(result.Errors()))
	for _, validationError := range result.Errors() {
		messages = append(messages, validationError.String())
	}

	return fmt.Errorf("install state failed schema validation: %s", strings.Join(messages, "; "))
}

func validateStateSemantics(state State) error {
	if len(state.Active) > 0 {
		if err := validateResolvedSet(state.Active, "active"); err != nil {
			return err
		}
	}

	var previousTimestamp time.Time
	for i, entry := range state.History {
		if err := validateResolvedSet(entry.Resolved, fmt.Sprintf("history[%d].resolved", i)); err != nil {
			return err
		}

		timestamp, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return fmt.Errorf("install state semantic validation failed: invalid history[%d].timestamp: %w", i, err)
		}
		if i > 0 && timestamp.Before(previousTimestamp) {
			return fmt.Errorf("install state semantic validation failed: history timestamps are not ordered oldest to newest")
		}
		previousTimestamp = timestamp
	}

	if len(state.History) > 0 {
		newest := state.History[len(state.History)-1].Resolved
		if !equalResolvedSlices(state.Active, newest) {
			return fmt.Errorf("install state semantic validation failed: active does not match newest history snapshot")
		}
	}

	return nil
}

func validateResolvedSet(entries []ResolvedSkill, label string) error {
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, exists := seen[entry.Name]; exists {
			return fmt.Errorf("install state semantic validation failed: duplicate skill name %q in %s", entry.Name, label)
		}
		seen[entry.Name] = struct{}{}

		ref, err := oci.ParseReference(entry.Reference)
		if err != nil {
			return fmt.Errorf("install state semantic validation failed: invalid reference in %s: %w", label, err)
		}
		if ref.Digest != entry.Digest {
			return fmt.Errorf("install state semantic validation failed: digest %q does not match reference %q in %s", entry.Digest, entry.Reference, label)
		}
	}

	return nil
}

func equalResolvedSlices(left, right []ResolvedSkill) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
