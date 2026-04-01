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

func TestParseReferenceRejectsTagQualifiedRepositoryName(t *testing.T) {
	raw := "oci://registry.example.com/agentskills/list-directory:1.2.3@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	if _, err := ParseReference(raw); err == nil {
		t.Fatal("ParseReference() error = nil, want error")
	}
}

func TestParsePersistedReferenceAcceptsCanonicalDigestReference(t *testing.T) {
	raw := "oci://registry.example.com/agentskills/list-directory@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	ref, err := ParsePersistedReference(raw)
	if err != nil {
		t.Fatalf("ParsePersistedReference() error = %v", err)
	}

	if got := ref.Canonical(); got != raw {
		t.Fatalf("Canonical() = %q, want %q", got, raw)
	}
}

func TestParsePersistedReferenceRejectsTagQualifiedRepositoryName(t *testing.T) {
	raw := "oci://registry.example.com/agentskills/list-directory:1.2.3@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	if _, err := ParsePersistedReference(raw); err == nil {
		t.Fatal("ParsePersistedReference() error = nil, want error")
	}
}

func TestParsePersistedReferenceRejectsNonCanonicalRawString(t *testing.T) {
	raw := "registry.example.com/agentskills/list-directory@sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	if _, err := ParsePersistedReference(raw); err == nil {
		t.Fatal("ParsePersistedReference() error = nil, want error")
	}
}

func TestLooksLikeReference(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "canonical OCI reference",
			input: "oci://registry.example.com/agentskills/list-directory@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:  true,
		},
		{
			name:  "explicit OCI tgz-like reference",
			input: "oci://registry.example.com/agentskills/list-directory.tgz",
			want:  true,
		},
		{
			name:  "plain OCI digest reference",
			input: "registry.example.com/agentskills/list-directory@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:  true,
		},
		{
			name:  "malformed OCI digest reference",
			input: "oci://registry.example.com/agentskills/list-directory@sha256:notadigest",
			want:  true,
		},
		{
			name:  "malformed plain OCI digest reference",
			input: "registry.example.com/agentskills/list-directory@sha256",
			want:  true,
		},
		{
			name:  "plain OCI locator without path",
			input: "registry.example.com@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			want:  true,
		},
		{
			name:  "plain OCI tag reference",
			input: "registry.example.com/agentskills/list-directory:1.2.3",
			want:  true,
		},
		{
			name:  "plain OCI empty digest reference",
			input: "registry.example.com/agentskills/list-directory@",
			want:  true,
		},
		{
			name:  "archive path with at sign",
			input: "/tmp/build@v1/list-directory-1.2.3.tgz",
			want:  false,
		},
		{
			name:  "package archive path",
			input: "/tmp/list-directory-1.2.3.tgz",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LooksLikeReference(tt.input); got != tt.want {
				t.Fatalf("LooksLikeReference(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
