package verify

import (
	"bytes"
	"context"
	"fmt"
	"os"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func VerifyReference(ctx context.Context, registry oci.Registry, reference string) (Result, error) {
	fetched, err := oci.Fetch(ctx, registry, reference)
	if err != nil {
		return Result{}, err
	}

	mirroredManifestBytes, _, err := archivepkg.ReadMirroredManifestTGZ(bytes.NewReader(fetched.Archive))
	if err != nil {
		return Result{}, fmt.Errorf("read mirrored manifest from fetched archive: %w", err)
	}
	if !bytes.Equal(fetched.Config, mirroredManifestBytes) {
		return Result{}, fmt.Errorf("OCI config blob does not match mirrored manifest")
	}

	result, err := verifyPackageArchive(bytes.NewReader(fetched.Archive), os.TempDir(), fetched.Reference)
	if err != nil {
		return Result{}, err
	}
	result.Mode = "oci"
	result.Reference = fetched.Reference
	return result, nil
}
