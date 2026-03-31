package verify

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	archivepkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/archive"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/filesha256"
)

func VerifyPackage(path string) (Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return Result{}, fmt.Errorf("open package archive %q: %w", path, err)
	}
	defer file.Close()

	return verifyPackageArchive(file, filepath.Dir(path), "")
}

func verifyPackageArchive(r io.Reader, parentDir, reference string) (Result, error) {
	extractedRoot, loaded, err := archivepkg.ExtractTGZTemp(r, parentDir)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(filepath.Dir(extractedRoot))

	verifiedFilesSHA256, err := verifyFilesSHA256(extractedRoot)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Mode:                "package",
		Name:                loaded.Name,
		Version:             loaded.Version,
		Reference:           reference,
		VerifiedFilesSHA256: verifiedFilesSHA256,
	}, nil
}

func verifyFilesSHA256(root string) (bool, error) {
	return filesha256.Validate(root)
}
