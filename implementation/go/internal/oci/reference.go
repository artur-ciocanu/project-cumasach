package oci

import (
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
	"oras.land/oras-go/v2/registry/remote"
)

func LooksLikeReference(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	if strings.HasPrefix(value, "oci://") {
		return true
	}
	if strings.HasSuffix(value, ".tgz") {
		return false
	}
	if strings.Contains(value, "@") {
		return true
	}

	repository, digestValue, ok := strings.Cut(value, "@")
	if !ok || repository == "" || digestValue == "" {
		lastSlash := strings.LastIndex(value, "/")
		if lastSlash < 0 || strings.HasPrefix(value, "/") {
			return false
		}
		return strings.Contains(value[lastSlash+1:], ":")
	}

	return true
}

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
	if err := validateReferenceRepository(repository, raw); err != nil {
		return Reference{}, err
	}

	return Reference{
		Repository: repository,
		Digest:     parsedDigest.String(),
	}, nil
}

func ParsePersistedReference(raw string) (Reference, error) {
	ref, err := ParseReference(raw)
	if err != nil {
		return Reference{}, err
	}
	if ref.Canonical() != raw {
		return Reference{}, fmt.Errorf("reference %q is not canonical", raw)
	}
	return ref, nil
}

func validateReferenceRepository(repository, raw string) error {
	if _, err := remote.NewRepository(repository); err != nil {
		return fmt.Errorf("repository %q is not a valid OCI locator: %w", repository, err)
	}
	if strings.Contains(repository[strings.LastIndex(repository, "/")+1:], ":") {
		return fmt.Errorf("reference %q must not include a tag-qualified repository name", raw)
	}
	return nil
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
