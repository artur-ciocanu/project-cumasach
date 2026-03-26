package oci

import "testing"

func TestParseReferenceAcceptsCanonicalDigestReference(t *testing.T) {
	raw := "oci://registry.example.com/agentskills/list-directory@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ref, err := ParseReference(raw)
	if err != nil {
		t.Fatalf("ParseReference() error = %v", err)
	}

	if got := ref.Repository; got != "registry.example.com/agentskills/list-directory" {
		t.Fatalf("Repository = %q, want %q", got, "registry.example.com/agentskills/list-directory")
	}
	if got := ref.Digest; got != "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" {
		t.Fatalf("Digest = %q, want digest", got)
	}
	if got := ref.Canonical(); got != raw {
		t.Fatalf("Canonical() = %q, want %q", got, raw)
	}
}

func TestParseReferenceAcceptsPlainDigestReferenceAndNormalizesToCanonical(t *testing.T) {
	raw := "registry.example.com/agentskills/list-directory@sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	ref, err := ParseReference(raw)
	if err != nil {
		t.Fatalf("ParseReference() error = %v", err)
	}

	want := "oci://registry.example.com/agentskills/list-directory@sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	if got := ref.Canonical(); got != want {
		t.Fatalf("Canonical() = %q, want %q", got, want)
	}
}

func TestParseReferenceRejectsNonDigestReference(t *testing.T) {
	if _, err := ParseReference("registry.example.com/agentskills/list-directory:1.2.3"); err == nil {
		t.Fatal("ParseReference() error = nil, want error")
	}
}

func TestParseReferenceRejectsNonSHA256Digest(t *testing.T) {
	raw := "registry.example.com/agentskills/list-directory@sha512:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	if _, err := ParseReference(raw); err == nil {
		t.Fatal("ParseReference() error = nil, want error")
	}
}
