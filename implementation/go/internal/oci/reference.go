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

func RepositoryParent(rawRef string) (string, error) {
	ref, err := ParseReference(rawRef)
	if err != nil {
		return "", err
	}

	lastSlash := strings.LastIndex(ref.Repository, "/")
	if lastSlash <= 0 {
		return "", fmt.Errorf("derive dependency base from %q: ambiguous repository parent", rawRef)
	}
	parent := ref.Repository[:lastSlash]
	return parent, nil
}

func DependencyRepository(base, dependencyName string) (string, error) {
	normalizedBase, err := normalizeRepository(base)
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(dependencyName)
	if name == "" {
		return "", fmt.Errorf("dependency name is empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "@") {
		return "", fmt.Errorf("dependency name %q is invalid for OCI repository construction", dependencyName)
	}

	return normalizedBase + "/" + name, nil
}
