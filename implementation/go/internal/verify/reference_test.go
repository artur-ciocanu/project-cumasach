package verify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

func TestVerifyReference(t *testing.T) {
	t.Run("valid pushed OCI artifact succeeds", func(t *testing.T) {
		ctx := context.Background()
		registry := oci.NewMemoryRegistry()
		sourceDir := copySkillFixture(t, "list-directory")
		archivePath := buildPackageArchive(t, sourceDir, true)
		archiveBytes := mustReadFile(t, archivePath)
		manifestBytes := mustReadFile(t, filepath.Join(sourceDir, ".skill", "manifest.json"))

		ref, err := oci.Push(ctx, registry, "registry.example.com/agentskills/list-directory", manifestBytes, archiveBytes, oci.PushOptions{})
		if err != nil {
			t.Fatalf("Push() error = %v", err)
		}

		result, err := VerifyReference(ctx, registry, ref.Canonical())
		if err != nil {
			t.Fatalf("VerifyReference() error = %v", err)
		}
		if got := result.Mode; got != "oci" {
			t.Fatalf("Mode = %q, want %q", got, "oci")
		}
		if got := result.Name; got != "list-directory" {
			t.Fatalf("Name = %q, want %q", got, "list-directory")
		}
		if got := result.Reference; got != ref.Canonical() {
			t.Fatalf("Reference = %q, want %q", got, ref.Canonical())
		}
	})

	t.Run("config and mirrored manifest mismatch fails", func(t *testing.T) {
		ctx := context.Background()
		registry := oci.NewMemoryRegistry()
		sourceDir := copySkillFixture(t, "list-directory")
		archivePath := buildPackageArchive(t, sourceDir, true)
		archiveBytes := mustReadFile(t, archivePath)
		configBytes := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"list-directory","version":"9.9.9","skill":{"entrypoint":"SKILL.md"}}`)

		ref, err := oci.Push(ctx, registry, "registry.example.com/agentskills/list-directory", configBytes, archiveBytes, oci.PushOptions{})
		if err != nil {
			t.Fatalf("Push() error = %v", err)
		}

		_, err = VerifyReference(ctx, registry, ref.Canonical())
		if err == nil {
			t.Fatal("VerifyReference() error = nil, want config mismatch failure")
		}
		if !strings.Contains(err.Error(), "does not match mirrored manifest") {
			t.Fatalf("VerifyReference() error = %q, want config mismatch context", err)
		}
	})

	t.Run("wrong OCI media type fails", func(t *testing.T) {
		manifestBytes := mustJSON(t, ocispec.Manifest{
			Config: ocispec.Descriptor{
				MediaType: "application/octet-stream",
				Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				Size:      int64(len([]byte(`{}`))),
			},
			Layers: []ocispec.Descriptor{
				{
					MediaType: oci.ContentLayerMediaType,
					Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
					Size:      int64(len([]byte("archive"))),
				},
			},
		})
		registry := staticRegistry{
			desc: content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, manifestBytes),
			store: &staticReadOnlyTarget{
				blobs: map[string][]byte{
					content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, manifestBytes).Digest.String(): manifestBytes,
					"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": []byte(`{}`),
					"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": []byte("archive"),
				},
			},
		}

		_, err := VerifyReference(context.Background(), registry, "oci://registry.example.com/agentskills/list-directory@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
		if err == nil {
			t.Fatal("VerifyReference() error = nil, want media type failure")
		}
		if !strings.Contains(err.Error(), "media type") {
			t.Fatalf("VerifyReference() error = %q, want media type context", err)
		}
	})
}

type staticRegistry struct {
	desc  ocispec.Descriptor
	store *staticReadOnlyTarget
}

func (r staticRegistry) PushTarget(context.Context, string) (oras.Target, error) {
	return nil, io.EOF
}

func (r staticRegistry) ResolveReference(context.Context, string, string) (oras.ReadOnlyTarget, ocispec.Descriptor, error) {
	return r.store, r.desc, nil
}

func (r staticRegistry) ListTags(context.Context, string) ([]string, error) {
	return nil, io.EOF
}

type staticReadOnlyTarget struct {
	blobs map[string][]byte
}

func (s *staticReadOnlyTarget) Exists(context.Context, ocispec.Descriptor) (bool, error) {
	return false, nil
}

func (s *staticReadOnlyTarget) Fetch(_ context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	body, ok := s.blobs[target.Digest.String()]
	if !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func (s *staticReadOnlyTarget) Resolve(context.Context, string) (ocispec.Descriptor, error) {
	return ocispec.Descriptor{}, io.EOF
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}
