package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/spf13/cobra"
)

var newPushRegistry = func() oci.Registry {
	return oci.RemoteRegistry{}
}

func newPushCmd() *cobra.Command {
	var tag string

	cmd := &cobra.Command{
		Use:   "push package.tgz oci-repo",
		Short: "Push a packaged skill artifact to an OCI registry",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			artifactRef, err := pushPackage(cmd.Context(), newPushRegistry(), args[0], args[1], tag)
			if err != nil {
				return err
			}

			if _, err := fmt.Fprintln(cmd.OutOrStdout(), artifactRef); err != nil {
				return fmt.Errorf("write pushed artifact reference: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&tag, "tag", "", "Tag to apply to the pushed artifact")

	return cmd
}

func pushPackage(ctx context.Context, registry oci.Registry, packagePath, repository, tag string) (string, error) {
	archiveBytes, err := os.ReadFile(packagePath)
	if err != nil {
		return "", fmt.Errorf("read package archive: %w", err)
	}

	mirroredManifestBytes, mirroredManifest, err := archivepkg.ReadMirroredManifestTGZ(bytes.NewReader(archiveBytes))
	if err != nil {
		return "", fmt.Errorf("read mirrored manifest from package archive: %w", err)
	}

	if tag == "" {
		tag = mirroredManifest.Version
	}

	pushed, err := oci.Push(ctx, registry, repository, mirroredManifestBytes, archiveBytes, oci.PushOptions{
		Tag: tag,
	})
	if err != nil {
		return "", err
	}

	return pushed.Canonical(), nil
}
