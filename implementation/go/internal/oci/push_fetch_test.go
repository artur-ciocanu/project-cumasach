package oci

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

func TestPushAndFetchRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := NewMemoryRegistry()
	manifestJSON := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"list-directory","version":"1.2.3","skill":{"entrypoint":"SKILL.md"}}`)
	archive := []byte("fake tgz bytes")

	pushed, err := Push(ctx, registry, "registry.example.com/agentskills/list-directory", manifestJSON, archive, PushOptions{
		Tag: "1.2.3",
	})
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	if pushed.Digest == "" {
		t.Fatal("Push() digest is empty")
	}
	if got := pushed.Canonical(); got[:6] != "oci://" {
		t.Fatalf("Push() canonical reference = %q, want oci:// prefix", got)
	}

	stored, err := registry.Resolve(pushed.Repository, pushed.Digest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := stored.ConfigMediaType; got != ConfigMediaType {
		t.Fatalf("config media type = %q, want %q", got, ConfigMediaType)
	}
	if got := stored.LayerMediaType; got != ContentLayerMediaType {
		t.Fatalf("layer media type = %q, want %q", got, ContentLayerMediaType)
	}

	fetched, err := Fetch(ctx, registry, pushed.Canonical())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if got := fetched.Reference; got != pushed.Canonical() {
		t.Fatalf("Reference = %q, want %q", got, pushed.Canonical())
	}
	if !bytes.Equal(fetched.Config, manifestJSON) {
		t.Fatalf("Config = %q, want %q", fetched.Config, manifestJSON)
	}
	if !bytes.Equal(fetched.Archive, archive) {
		t.Fatalf("Archive = %q, want %q", fetched.Archive, archive)
	}
}

func TestFetchRejectsNonManifestDescriptorMediaType(t *testing.T) {
	t.Parallel()

	registry := newStaticRegistry(t, ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.index.v1+json",
		Digest:    "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Size:      int64(len(mustJSON(t, ocispec.Manifest{}))),
	}, mustJSON(t, ocispec.Manifest{}))

	_, err := Fetch(context.Background(), registry, "oci://registry.example.com/agentskills/list-directory@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Fatal("Fetch() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "descriptor media type") {
		t.Fatalf("Fetch() error = %q, want descriptor media type failure", err)
	}
}

func TestFetchRejectsManifestWithWrongLayerCount(t *testing.T) {
	t.Parallel()

	registry := newStaticRegistry(t, manifestDescriptor(t, ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: ConfigMediaType},
		Layers: []ocispec.Descriptor{
			{MediaType: ContentLayerMediaType},
			{MediaType: ContentLayerMediaType},
		},
	}), mustJSON(t, ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: ConfigMediaType},
		Layers: []ocispec.Descriptor{
			{MediaType: ContentLayerMediaType},
			{MediaType: ContentLayerMediaType},
		},
	}))

	_, err := Fetch(context.Background(), registry, "oci://registry.example.com/agentskills/list-directory@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err == nil {
		t.Fatal("Fetch() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "exactly one layer") {
		t.Fatalf("Fetch() error = %q, want wrong layer count failure", err)
	}
}

func TestFetchRejectsManifestWithWrongLayerMediaType(t *testing.T) {
	t.Parallel()

	registry := newStaticRegistry(t, manifestDescriptor(t, ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: ConfigMediaType},
		Layers: []ocispec.Descriptor{
			{MediaType: "application/octet-stream"},
		},
	}), mustJSON(t, ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: ConfigMediaType},
		Layers: []ocispec.Descriptor{
			{MediaType: "application/octet-stream"},
		},
	}))

	_, err := Fetch(context.Background(), registry, "oci://registry.example.com/agentskills/list-directory@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err == nil {
		t.Fatal("Fetch() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "content layer media type") {
		t.Fatalf("Fetch() error = %q, want wrong layer media type failure", err)
	}
}

func TestFetchRejectsManifestWithWrongConfigMediaType(t *testing.T) {
	t.Parallel()

	registry := newStaticRegistry(t, manifestDescriptor(t, ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: "application/octet-stream"},
		Layers: []ocispec.Descriptor{
			{MediaType: ContentLayerMediaType},
		},
	}), mustJSON(t, ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: "application/octet-stream"},
		Layers: []ocispec.Descriptor{
			{MediaType: ContentLayerMediaType},
		},
	}))

	_, err := Fetch(context.Background(), registry, "oci://registry.example.com/agentskills/list-directory@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	if err == nil {
		t.Fatal("Fetch() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "config media type") {
		t.Fatalf("Fetch() error = %q, want wrong config media type failure", err)
	}
}

func TestRepositoryParentFromExactReference(t *testing.T) {
	t.Parallel()

	ref := "oci://registry.example.com/agentskills/python-development@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	base, err := RepositoryParent(ref)
	if err != nil {
		t.Fatalf("RepositoryParent() error = %v", err)
	}
	if got, want := base, "registry.example.com/agentskills"; got != want {
		t.Fatalf("RepositoryParent() = %q, want %q", got, want)
	}
}

func TestRepositoryParentAcceptsSingleSegmentRepositoryPath(t *testing.T) {
	t.Parallel()

	ref := "oci://registry.example.com/python-development@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	base, err := RepositoryParent(ref)
	if err != nil {
		t.Fatalf("RepositoryParent() error = %v", err)
	}
	if got, want := base, "registry.example.com"; got != want {
		t.Fatalf("RepositoryParent() = %q, want %q", got, want)
	}
}

func TestRepositoryParentBuildsDependencyRepository(t *testing.T) {
	t.Parallel()

	repository, err := DependencyRepository("registry.example.com/agentskills", "tdd")
	if err != nil {
		t.Fatalf("DependencyRepository() error = %v", err)
	}
	if got, want := repository, "registry.example.com/agentskills/tdd"; got != want {
		t.Fatalf("DependencyRepository() = %q, want %q", got, want)
	}
}

func TestListTags(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	registry := NewMemoryRegistry()
	repository := "registry.example.com/agentskills/list-directory"

	for _, tag := range []string{"1.0.0", "latest", "1.2.3"} {
		manifestJSON := []byte(`{"schemaVersion":"v1","packageType":"skill","name":"list-directory","version":"` + tag + `","skill":{"entrypoint":"SKILL.md"}}`)
		archive := []byte("fake tgz bytes for " + tag)
		if _, err := Push(ctx, registry, repository, manifestJSON, archive, PushOptions{Tag: tag}); err != nil {
			t.Fatalf("Push(%q) error = %v", tag, err)
		}
	}

	tags, err := ListTags(ctx, registry, repository)
	if err != nil {
		t.Fatalf("ListTags() error = %v", err)
	}
	if want := []string{"1.0.0", "1.2.3", "latest"}; !reflect.DeepEqual(tags, want) {
		t.Fatalf("ListTags() = %#v, want %#v", tags, want)
	}
}

type staticRegistry struct {
	desc  ocispec.Descriptor
	store *staticReadOnlyTarget
}

func newStaticRegistry(t *testing.T, desc ocispec.Descriptor, manifest []byte) staticRegistry {
	t.Helper()
	return staticRegistry{
		desc: desc,
		store: &staticReadOnlyTarget{
			blobs: map[string][]byte{
				desc.Digest.String(): manifest,
			},
		},
	}
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

func manifestDescriptor(t *testing.T, manifest ocispec.Manifest) ocispec.Descriptor {
	t.Helper()
	return content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, mustJSON(t, manifest))
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}
