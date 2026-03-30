package oci

import (
	"bytes"
	"context"
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

type manifestRecorder interface {
	recordManifest(string, StoredManifest)
}

func Push(ctx context.Context, registry Registry, repository string, manifestJSON, archive []byte, options PushOptions) (Reference, error) {
	normalizedRepo, err := normalizeRepository(repository)
	if err != nil {
		return Reference{}, err
	}

	target, err := registry.PushTarget(ctx, normalizedRepo)
	if err != nil {
		return Reference{}, err
	}

	configDesc, err := oras.PushBytes(ctx, target, ConfigMediaType, manifestJSON)
	if err != nil {
		return Reference{}, fmt.Errorf("push OCI config blob: %w", err)
	}

	layerDesc := content.NewDescriptorFromBytes(ContentLayerMediaType, archive)
	layerDesc.Annotations = map[string]string{
		ocispec.AnnotationTitle: "package.tgz",
	}
	if err := target.Push(ctx, layerDesc, bytes.NewReader(archive)); err != nil {
		return Reference{}, fmt.Errorf("push OCI content layer: %w", err)
	}

	manifestDesc, err := oras.PackManifest(ctx, target, oras.PackManifestVersion1_0, "", oras.PackManifestOptions{
		ConfigDescriptor: &configDesc,
		Layers:           []ocispec.Descriptor{layerDesc},
	})
	if err != nil {
		return Reference{}, fmt.Errorf("pack OCI manifest: %w", err)
	}

	if options.Tag != "" {
		if err := target.Tag(ctx, manifestDesc, options.Tag); err != nil {
			return Reference{}, fmt.Errorf("tag OCI manifest %q: %w", options.Tag, err)
		}
	}

	if recorder, ok := registry.(manifestRecorder); ok {
		recorder.recordManifest(normalizedRepo, StoredManifest{
			ManifestDescriptor: manifestDesc,
			ConfigDescriptor:   configDesc,
			LayerDescriptor:    layerDesc,
			ConfigMediaType:    configDesc.MediaType,
			LayerMediaType:     layerDesc.MediaType,
		})
	}

	return Reference{
		Repository: normalizedRepo,
		Digest:     manifestDesc.Digest.String(),
	}, nil
}
