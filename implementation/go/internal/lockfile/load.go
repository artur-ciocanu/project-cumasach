package lockfile

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"oras.land/oras-go/v2/registry/remote"
)

func LoadFile(path string) (File, error) {
	file, err := os.Open(path)
	if err != nil {
		return File{}, fmt.Errorf("open lockfile %q: %w", path, err)
	}
	defer file.Close()

	lockfile, err := LoadReader(file)
	if err != nil {
		return File{}, fmt.Errorf("load lockfile %q: %w", path, err)
	}

	return lockfile, nil
}

func LoadReader(r io.Reader) (File, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return File{}, fmt.Errorf("read lockfile: %w", err)
	}

	if err := validate(data); err != nil {
		return File{}, err
	}

	var lockfile File
	if err := json.Unmarshal(data, &lockfile); err != nil {
		return File{}, fmt.Errorf("decode lockfile JSON: %w", err)
	}

	if err := validateSemantics(lockfile); err != nil {
		return File{}, err
	}

	return lockfile, nil
}

func validateSemantics(lockfile File) error {
	packagesByName := make(map[string]Package, len(lockfile.Packages))
	for _, pkg := range lockfile.Packages {
		if _, exists := packagesByName[pkg.Name]; exists {
			return fmt.Errorf("lockfile semantic validation failed: duplicate package name %q", pkg.Name)
		}
		if err := validatePackageReference(pkg); err != nil {
			return err
		}
		packagesByName[pkg.Name] = pkg
	}

	rootPkg, ok := packagesByName[lockfile.Root.Name]
	if !ok {
		return fmt.Errorf("lockfile semantic validation failed: root package %q not found", lockfile.Root.Name)
	}
	if rootPkg.Version != lockfile.Root.Version {
		return fmt.Errorf("lockfile semantic validation failed: root version %q does not match selected package version %q", lockfile.Root.Version, rootPkg.Version)
	}
	if rootPkg.Reference != lockfile.Root.Reference {
		return fmt.Errorf("lockfile semantic validation failed: root reference %q does not match selected package reference %q", lockfile.Root.Reference, rootPkg.Reference)
	}
	if _, err := validateReference(lockfile.Root.Reference); err != nil {
		return fmt.Errorf("lockfile semantic validation failed: invalid root reference %q: %w", lockfile.Root.Reference, err)
	}

	graph := make(map[string][]string, len(packagesByName))
	for name := range packagesByName {
		graph[name] = nil
	}
	for _, edge := range lockfile.Edges {
		if _, ok := packagesByName[edge.From]; !ok {
			return fmt.Errorf("lockfile semantic validation failed: edge from %q references unknown package", edge.From)
		}
		if _, ok := packagesByName[edge.To]; !ok {
			return fmt.Errorf("lockfile semantic validation failed: edge to %q references unknown package", edge.To)
		}
		graph[edge.From] = append(graph[edge.From], edge.To)
	}

	if err := validateAcyclic(graph); err != nil {
		return err
	}

	return nil
}

func validatePackageReference(pkg Package) error {
	ref, err := validateReference(pkg.Reference)
	if err != nil {
		return fmt.Errorf("lockfile semantic validation failed: invalid package reference for %q: %w", pkg.Name, err)
	}
	if ref.Digest != pkg.Digest {
		return fmt.Errorf("lockfile semantic validation failed: package digest %q does not match reference %q for %q", pkg.Digest, pkg.Reference, pkg.Name)
	}
	return nil
}

func validateReference(raw string) (oci.Reference, error) {
	ref, err := oci.ParseReference(raw)
	if err != nil {
		return oci.Reference{}, err
	}
	if ref.Canonical() != raw {
		return oci.Reference{}, fmt.Errorf("reference %q is not canonical", raw)
	}
	if _, err := remote.NewRepository(ref.Repository); err != nil {
		return oci.Reference{}, fmt.Errorf("repository %q is not a valid OCI locator: %w", ref.Repository, err)
	}
	if strings.Contains(ref.Repository[strings.LastIndex(ref.Repository, "/")+1:], ":") {
		return oci.Reference{}, fmt.Errorf("reference %q must not include a tag-qualified repository name", raw)
	}
	return ref, nil
}

func validateAcyclic(graph map[string][]string) error {
	const (
		unseen = iota
		visiting
		visited
	)

	state := make(map[string]int, len(graph))
	var visit func(string) error
	visit = func(node string) error {
		switch state[node] {
		case visiting:
			return fmt.Errorf("lockfile semantic validation failed: dependency cycle detected at %q", node)
		case visited:
			return nil
		}

		state[node] = visiting
		for _, next := range graph[node] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[node] = visited
		return nil
	}

	for node := range graph {
		if state[node] != unseen {
			continue
		}
		if err := visit(node); err != nil {
			return err
		}
	}

	return nil
}
