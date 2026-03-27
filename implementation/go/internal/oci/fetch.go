package oci

import (
	"context"
	"encoding/json"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func Fetch(ctx context.Context, registry Registry, rawRef string) (FetchedArtifact, error) {
	ref, err := ParseReference(rawRef)
	if err != nil {
		return FetchedArtifact{}, err
	}

	target, manifestDesc, err := registry.ResolveReference(ctx, ref.Repository, ref.Digest)
	if err != nil {
		return FetchedArtifact{}, err
	}
	if manifestDesc.MediaType != ocispec.MediaTypeImageManifest {
		return FetchedArtifact{}, fmt.Errorf("OCI manifest descriptor media type %q does not match %q", manifestDesc.MediaType, ocispec.MediaTypeImageManifest)
	}

	manifestBytes, err := content.FetchAll(ctx, target, manifestDesc)
	if err != nil {
		return FetchedArtifact{}, fmt.Errorf("fetch OCI manifest %q: %w", ref.Canonical(), err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return FetchedArtifact{}, fmt.Errorf("decode OCI manifest %q: %w", ref.Canonical(), err)
	}

	if manifest.Config.MediaType != ConfigMediaType {
		return FetchedArtifact{}, fmt.Errorf("OCI config media type %q does not match %q", manifest.Config.MediaType, ConfigMediaType)
	}
	if len(manifest.Layers) != 1 {
		return FetchedArtifact{}, fmt.Errorf("OCI manifest must contain exactly one layer, got %d", len(manifest.Layers))
	}
	if manifest.Layers[0].MediaType != ContentLayerMediaType {
		return FetchedArtifact{}, fmt.Errorf("OCI content layer media type %q does not match %q", manifest.Layers[0].MediaType, ContentLayerMediaType)
	}

	configBytes, err := content.FetchAll(ctx, target, manifest.Config)
	if err != nil {
		return FetchedArtifact{}, fmt.Errorf("fetch OCI config blob %q: %w", ref.Canonical(), err)
	}
	archiveBytes, err := content.FetchAll(ctx, target, manifest.Layers[0])
	if err != nil {
		return FetchedArtifact{}, fmt.Errorf("fetch OCI content layer %q: %w", ref.Canonical(), err)
	}

	return FetchedArtifact{
		Reference:  ref.Canonical(),
		Repository: ref.Repository,
		Digest:     ref.Digest,
		Config:     configBytes,
		Archive:    archiveBytes,
	}, nil
}

func ListTags(ctx context.Context, registry Registry, repository string) ([]string, error) {
	normalizedRepo, err := normalizeRepository(repository)
	if err != nil {
		return nil, err
	}

	tags, err := registry.ListTags(ctx, normalizedRepo)
	if err != nil {
		return nil, err
	}
	return tags, nil
}
