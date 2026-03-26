package oci

import (
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

func ParseReference(raw string) (Reference, error) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "oci://")
	if value == "" {
		return Reference{}, fmt.Errorf("OCI reference is empty")
	}

	repository, digestValue, ok := strings.Cut(value, "@")
	if !ok || repository == "" || digestValue == "" {
		return Reference{}, fmt.Errorf("OCI reference %q must be digest-pinned", raw)
	}

	parsedDigest, err := digest.Parse(digestValue)
	if err != nil {
		return Reference{}, fmt.Errorf("parse digest reference %q: %w", raw, err)
	}
	if parsedDigest.Algorithm() != digest.SHA256 {
		return Reference{}, fmt.Errorf("OCI reference %q must use sha256 digest", raw)
	}

	return Reference{
		Repository: repository,
		Digest:     parsedDigest.String(),
	}, nil
}

func normalizeRepository(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "oci://")
	if value == "" {
		return "", fmt.Errorf("OCI repository is empty")
	}
	if strings.Contains(value, "@") {
		return "", fmt.Errorf("OCI repository %q must not include a digest reference", raw)
	}
	return strings.TrimSuffix(value, "/"), nil
}
