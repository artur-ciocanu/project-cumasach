package oci

import (
	"bytes"
	"context"
	"testing"
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
