package verify

import (
	"bytes"
	"context"
	"fmt"
	"os"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

func VerifyReference(ctx context.Context, registry oci.Registry, reference string, policy TrustPolicy) (Result, error) {
	fetched, err := oci.Fetch(ctx, registry, reference)
	if err != nil {
		return Result{}, err
	}
	return VerifyFetchedArtifact(ctx, fetched, policy)
}

// VerifyFetchedArtifact validates an already-fetched OCI artifact: structural
// checks (config blob equals the mirrored manifest, package archive layout)
// run first, then published-artifact trust. Callers that have already fetched
// the bytes use this to avoid a redundant registry round-trip.
func VerifyFetchedArtifact(ctx context.Context, fetched oci.FetchedArtifact, policy TrustPolicy) (Result, error) {
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

	if err := VerifyPublishedArtifactTrust(ctx, fetched.Reference, policy); err != nil {
		return Result{}, err
	}

	result.Mode = "oci"
	result.Reference = fetched.Reference
	return result, nil
}
