package oci

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
)

func TestLoadORASInteropConfigFromEnv(t *testing.T) {
	tests := []struct {
		name       string
		env        map[string]string
		want       orasInteropConfig
		wantSkip   string
	}{
		{
			name: "generic conformance env",
			env: map[string]string{
				"CUMASACH_ORAS_CONFORMANCE_REPOSITORY": "registry.example.com/agentskills/demo",
				"CUMASACH_ORAS_CONFORMANCE_USERNAME":   "robot",
				"CUMASACH_ORAS_CONFORMANCE_PASSWORD":   "secret",
				"CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP": "1",
			},
			want: orasInteropConfig{
				repository: "registry.example.com/agentskills/demo",
				username:   "robot",
				password:   "secret",
				plainHTTP:  true,
			},
		},
		{
			name:     "missing repository requests skip",
			env:      map[string]string{},
			wantSkip: "CUMASACH_ORAS_CONFORMANCE_REPOSITORY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, skipReason := loadORASInteropConfigFromEnv(func(name string) string {
				return tt.env[name]
			})
			if tt.wantSkip != "" {
				if !strings.Contains(skipReason, tt.wantSkip) {
					t.Fatalf("skipReason = %q, want %q", skipReason, tt.wantSkip)
				}
				return
			}
			if skipReason != "" {
				t.Fatalf("skipReason = %q, want empty", skipReason)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("config = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestORASConformanceRoundTrip(t *testing.T) {
	config := loadORASInteropConfig(t)

	t.Run("fetches an oras-pushed artifact from configured registry", func(t *testing.T) {
		ctx := context.Background()
		tag := "cumasach-oras-fetch-" + randomSuffix(t)
		referenceWithTag := config.repository + ":" + tag
		manifestJSON := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"oras-fetch","version":"1.0.0","skill":{"entrypoint":"SKILL.md"}}`)
		archive := []byte("oras conformance payload bytes")

		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "manifest.json")
		archivePath := filepath.Join(tempDir, "skill.tgz")
		if err := os.WriteFile(configPath, manifestJSON, 0o644); err != nil {
			t.Fatalf("WriteFile(config) error = %v", err)
		}
		if err := os.WriteFile(archivePath, archive, 0o644); err != nil {
			t.Fatalf("WriteFile(archive) error = %v", err)
		}

		digestText := strings.TrimSpace(runORAS(t, config, tempDir,
			"push",
			referenceWithTag,
			"--config", "manifest.json:"+ConfigMediaType,
			"skill.tgz:"+ContentLayerMediaType,
			"--format", "go-template={{.digest}}",
		))
		fetched, err := Fetch(ctx, newAuthenticatedRemoteRegistry(config), "oci://"+config.repository+"@"+digestText)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}
		if !bytes.Equal(fetched.Config, manifestJSON) {
			t.Fatalf("Fetch() config = %q, want %q", fetched.Config, manifestJSON)
		}
		if !bytes.Equal(fetched.Archive, archive) {
			t.Fatalf("Fetch() archive = %q, want %q", fetched.Archive, archive)
		}
	})

	t.Run("an implementation-pushed artifact can be pulled by oras from configured registry", func(t *testing.T) {
		ctx := context.Background()
		registry := newAuthenticatedRemoteRegistry(config)
		tag := "cumasach-oras-pull-" + randomSuffix(t)
		manifestJSON := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"oras-pull","version":"1.0.0","skill":{"entrypoint":"SKILL.md"}}`)
		archive := []byte("implementation artifactory payload bytes")

		pushed, err := Push(ctx, registry, config.repository, manifestJSON, archive, PushOptions{Tag: tag})
		if err != nil {
			t.Fatalf("Push() error = %v", err)
		}

		layerDigest, err := publishedLayerDigest(ctx, registry, pushed.Canonical())
		if err != nil {
			t.Fatalf("publishedLayerDigest() error = %v", err)
		}

		pullDir := t.TempDir()
		runORAS(t, config, pullDir,
			"pull",
			strings.TrimPrefix(pushed.Canonical(), "oci://"),
			"--output", ".",
			"--config", "config.json",
		)

		pulledArchive, err := onlyPulledFileBytes(pullDir)
		if err != nil {
			t.Fatalf("onlyPulledFileBytes() error = %v", err)
		}
		if got := digest.FromBytes(pulledArchive).String(); got != layerDigest {
			t.Fatalf("pulled payload digest = %q, want %q", got, layerDigest)
		}
		if !bytes.Equal(pulledArchive, archive) {
			t.Fatalf("pulled payload = %q, want %q", pulledArchive, archive)
		}

		pulledConfig, err := os.ReadFile(filepath.Join(pullDir, "config.json"))
		if err != nil {
			t.Fatalf("ReadFile(config) error = %v", err)
		}
		if !bytes.Equal(pulledConfig, manifestJSON) {
			t.Fatalf("oras pull config = %q, want %q", pulledConfig, manifestJSON)
		}

		fetchConfigDir := t.TempDir()
		runORAS(t, config, fetchConfigDir,
			"manifest", "fetch-config",
			strings.TrimPrefix(pushed.Canonical(), "oci://"),
			"--output", "fetch-config.json",
		)
		fetchedConfig, err := os.ReadFile(filepath.Join(fetchConfigDir, "fetch-config.json"))
		if err != nil {
			t.Fatalf("ReadFile(fetch-config) error = %v", err)
		}
		if !bytes.Equal(fetchedConfig, manifestJSON) {
			t.Fatalf("oras manifest fetch-config = %q, want %q", fetchedConfig, manifestJSON)
		}
	})
}

type orasInteropConfig struct {
	repository string
	username   string
	password   string
	plainHTTP  bool
}

func loadORASInteropConfig(t *testing.T) orasInteropConfig {
	t.Helper()

	config, skipReason := loadORASInteropConfigFromEnv(os.Getenv)
	if skipReason != "" {
		t.Skip(skipReason)
	}
	return config
}

func loadORASInteropConfigFromEnv(getenv func(string) string) (orasInteropConfig, string) {
	repository := getenv("CUMASACH_ORAS_CONFORMANCE_REPOSITORY")
	if repository == "" {
		return orasInteropConfig{}, "set CUMASACH_ORAS_CONFORMANCE_REPOSITORY to run ORAS conformance tests"
	}

	username := getenv("CUMASACH_ORAS_CONFORMANCE_USERNAME")
	password := getenv("CUMASACH_ORAS_CONFORMANCE_PASSWORD")
	if username == "" || password == "" {
		return orasInteropConfig{}, "set CUMASACH_ORAS_CONFORMANCE_USERNAME and CUMASACH_ORAS_CONFORMANCE_PASSWORD to run ORAS conformance tests"
	}

	return orasInteropConfig{
		repository: repository,
		username:   username,
		password:   password,
		plainHTTP:  getenv("CUMASACH_ORAS_CONFORMANCE_PLAIN_HTTP") == "1",
	}, ""
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

type authenticatedRemoteRegistry struct {
	config orasInteropConfig
}

func newAuthenticatedRemoteRegistry(config orasInteropConfig) authenticatedRemoteRegistry {
	return authenticatedRemoteRegistry{config: config}
}

func (r authenticatedRemoteRegistry) PushTarget(_ context.Context, repository string) (oras.Target, error) {
	repo, err := r.repository(repository)
	if err != nil {
		return nil, err
	}
	return repo, nil
}

func (r authenticatedRemoteRegistry) ResolveReference(ctx context.Context, repository, reference string) (oras.ReadOnlyTarget, ocispec.Descriptor, error) {
	repo, err := r.repository(repository)
	if err != nil {
		return nil, ocispec.Descriptor{}, err
	}

	desc, err := repo.Resolve(ctx, reference)
	if err != nil {
		return nil, ocispec.Descriptor{}, fmt.Errorf("resolve reference %q in %q: %w", reference, repository, err)
	}
	return repo, desc, nil
}

func (r authenticatedRemoteRegistry) ListTags(ctx context.Context, repository string) ([]string, error) {
	repo, err := r.repository(repository)
	if err != nil {
		return nil, err
	}

	var tags []string
	if err := repo.Tags(ctx, "", func(batch []string) error {
		tags = append(tags, batch...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list tags in %q: %w", repository, err)
	}
	return tags, nil
}

func (r authenticatedRemoteRegistry) repository(repository string) (*remote.Repository, error) {
	repo, err := remote.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("create remote repository %q: %w", repository, err)
	}
	repo.PlainHTTP = r.config.plainHTTP

	client := *auth.DefaultClient
	client.Credential = auth.StaticCredential(repo.Reference.Registry, auth.Credential{
		Username: r.config.username,
		Password: r.config.password,
	})
	repo.Client = &client
	return repo, nil
}

func runORAS(t *testing.T, config orasInteropConfig, workingDir string, args ...string) string {
	t.Helper()

	completeArgs := append([]string(nil), args...)
	if config.plainHTTP {
		completeArgs = append(completeArgs, "--plain-http")
	}
	completeArgs = append(completeArgs, "--username", config.username, "--password-stdin")

	cmd := exec.Command("oras", completeArgs...)
	cmd.Dir = workingDir
	cmd.Stdin = strings.NewReader(config.password + "\n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oras %s error = %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func publishedLayerDigest(ctx context.Context, registry authenticatedRemoteRegistry, reference string) (string, error) {
	ref, err := ParseReference(reference)
	if err != nil {
		return "", err
	}

	target, manifestDesc, err := registry.ResolveReference(ctx, ref.Repository, ref.Digest)
	if err != nil {
		return "", err
	}
	manifestBytes, err := content.FetchAll(ctx, target, manifestDesc)
	if err != nil {
		return "", fmt.Errorf("fetch OCI manifest %q: %w", reference, err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return "", fmt.Errorf("decode OCI manifest %q: %w", reference, err)
	}
	if len(manifest.Layers) != 1 {
		return "", fmt.Errorf("OCI manifest must contain exactly one layer, got %d", len(manifest.Layers))
	}
	return manifest.Layers[0].Digest.String(), nil
}

func onlyPulledFileBytes(root string) ([]byte, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read pull output directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("expected pulled payload file, got directory %q", entry.Name())
		}
		if entry.Name() == "config.json" {
			continue
		}
		files = append(files, entry.Name())
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("expected exactly one pulled payload file, got %d", len(files))
	}
	return os.ReadFile(filepath.Join(root, files[0]))
}

func randomSuffix(t *testing.T) string {
	t.Helper()

	var raw [4]byte
	if _, err := rand.Read(raw[:]); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	return hex.EncodeToString(raw[:])
}
