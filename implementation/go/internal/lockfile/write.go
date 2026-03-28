package lockfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/resolve"
)

func FromGraph(graph resolve.Graph) (File, error) {
	rootName := strings.TrimSpace(graph.Root)
	if rootName == "" {
		return File{}, fmt.Errorf("serialize lockfile: graph root must not be empty")
	}

	rootPkg, ok := graph.Packages[rootName]
	if !ok {
		return File{}, fmt.Errorf("serialize lockfile: root package %q not found", rootName)
	}
	rootPackageName, err := packageName(rootName, rootPkg)
	if err != nil {
		return File{}, err
	}

	packages := make([]Package, 0, len(graph.Packages))
	names := make([]string, 0, len(graph.Packages))
	for name := range graph.Packages {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		selected := graph.Packages[name]
		pkgName, err := packageName(name, selected)
		if err != nil {
			return File{}, err
		}
		reference, err := canonicalizeReference(selected.Reference)
		if err != nil {
			return File{}, fmt.Errorf("serialize lockfile package %q: %w", pkgName, err)
		}

		packages = append(packages, Package{
			Name:      pkgName,
			Version:   selected.Version,
			Digest:    selected.Digest,
			Reference: reference,
		})
	}

	edges, err := serializeEdges(graph)
	if err != nil {
		return File{}, err
	}

	rootReference, err := canonicalizeReference(rootPkg.Reference)
	if err != nil {
		return File{}, fmt.Errorf("serialize lockfile root %q: %w", rootName, err)
	}

	lockfile := File{
		SchemaVersion: schemaVersionV1,
		Root: Root{
			Name:      rootPackageName,
			Version:   rootPkg.Version,
			Reference: rootReference,
		},
		Packages: packages,
		Edges:    edges,
	}

	if err := validateSemantics(lockfile); err != nil {
		return File{}, err
	}

	return lockfile, nil
}

func Write(w io.Writer, graph resolve.Graph) error {
	lockfile, err := FromGraph(graph)
	if err != nil {
		return err
	}

	data, err := marshal(lockfile)
	if err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write lockfile: %w", err)
	}

	return nil
}

func marshal(lockfile File) ([]byte, error) {
	data, err := json.MarshalIndent(lockfile, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal lockfile JSON: %w", err)
	}
	data = append(data, '\n')

	if _, err := LoadReader(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("re-validate serialized lockfile: %w", err)
	}

	return data, nil
}

func packageName(key string, pkg resolve.SelectedPackage) (string, error) {
	name := strings.TrimSpace(pkg.Name)
	if name == "" {
		name = key
	}
	if name != key {
		return "", fmt.Errorf("serialize lockfile: package key %q does not match selected package name %q", key, name)
	}
	return name, nil
}

func serializeEdges(graph resolve.Graph) ([]Edge, error) {
	fromNames := make([]string, 0, len(graph.Edges))
	for from := range graph.Edges {
		fromNames = append(fromNames, from)
	}
	sort.Strings(fromNames)

	edges := make([]Edge, 0)
	for _, from := range fromNames {
		if _, ok := graph.Packages[from]; !ok {
			return nil, fmt.Errorf("serialize lockfile: edge source %q not found in packages", from)
		}

		toNames := append([]string(nil), graph.Edges[from]...)
		sort.Strings(toNames)
		for _, to := range toNames {
			if _, ok := graph.Packages[to]; !ok {
				return nil, fmt.Errorf("serialize lockfile: edge target %q not found in packages", to)
			}
			edges = append(edges, Edge{From: from, To: to})
		}
	}

	return edges, nil
}

func canonicalizeReference(raw string) (string, error) {
	ref, err := oci.ParseReference(raw)
	if err != nil {
		return "", err
	}
	return ref.Canonical(), nil
}
