package resolve

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"

	manifestpkg "github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/oci"
)

type packageResolver struct {
	ctx        context.Context
	registry   oci.Registry
	base       string
	constraint map[string][]string
	selected   map[string]SelectedPackage
	edges      map[string][]string
}

func ResolveGraph(ctx context.Context, registry oci.Registry, root Root) (Graph, error) {
	resolver := &packageResolver{
		ctx:        ctx,
		registry:   registry,
		constraint: make(map[string][]string),
		selected:   make(map[string]SelectedPackage),
		edges:      make(map[string][]string),
	}

	rootPkg, err := resolver.resolveRoot(root)
	if err != nil {
		return Graph{}, err
	}
	resolver.selected[rootPkg.Name] = rootPkg

	if err := resolver.resolvePackage(rootPkg.Name, nil); err != nil {
		return Graph{}, err
	}

	resolver.pruneUnreachable(rootPkg.Name)

	return Graph{
		Root:     rootPkg.Name,
		Packages: resolver.selected,
		Edges:    resolver.edges,
	}, nil
}

func (r *packageResolver) resolveRoot(root Root) (SelectedPackage, error) {
	if root.IsExact() {
		selected, err := r.fetchSelectedByReference(root.Reference())
		if err != nil {
			return SelectedPackage{}, err
		}
		if len(selected.Manifest.Dependencies) > 0 {
			if root.OCIBase() == "" {
				return SelectedPackage{}, fmt.Errorf("exact artifact reference %q has dependencies; --from is required to provide the dependency base", root.Reference())
			}
			r.base = root.OCIBase()
		}
		return selected, nil
	}

	r.base = root.OCIBase()
	repository, err := oci.DependencyRepository(r.base, root.Name())
	if err != nil {
		return SelectedPackage{}, err
	}
	return r.selectPackage(root.Name(), repository, nil)
}

func (r *packageResolver) resolvePackage(name string, stack []string) error {
	if slices.Contains(stack, name) {
		return fmt.Errorf("dependency cycle detected: %s -> %s", strings.Join(stack, " -> "), name)
	}

	selected, ok := r.selected[name]
	if !ok {
		return fmt.Errorf("selected package %q not found", name)
	}

	stack = append(stack, name)
	r.edges[name] = nil

	for _, dep := range selected.Manifest.Dependencies {
		if dep.Name == name {
			return fmt.Errorf("skill %q has a self-dependency", name)
		}
		if slices.Contains(stack, dep.Name) {
			return fmt.Errorf("dependency cycle detected: %s -> %s", strings.Join(stack, " -> "), dep.Name)
		}
		if err := ParseDependencyConstraint(dep.Version); err != nil {
			return fmt.Errorf("dependency %q in %q has invalid version constraint %q: %w", dep.Name, name, dep.Version, err)
		}

		repository, err := oci.DependencyRepository(r.base, dep.Name)
		if err != nil {
			return fmt.Errorf("resolve repository for dependency %q: %w", dep.Name, err)
		}

		changed, err := r.selectDependency(dep.Name, repository, dep.Version)
		if err != nil {
			return err
		}
		r.edges[name] = appendUnique(r.edges[name], dep.Name)
		if changed || len(r.edges[dep.Name]) == 0 {
			if err := r.resolvePackage(dep.Name, stack); err != nil {
				return err
			}
		}
	}

	slices.Sort(r.edges[name])
	return nil
}

func (r *packageResolver) selectDependency(name, repository, constraint string) (bool, error) {
	r.constraint[name] = appendUnique(r.constraint[name], constraint)
	selected, err := r.selectPackage(name, repository, r.constraint[name])
	if err != nil {
		return false, err
	}

	current, ok := r.selected[name]
	if ok && current.Reference == selected.Reference {
		return false, nil
	}
	r.selected[name] = selected
	return true, nil
}

func (r *packageResolver) selectPackage(name, repository string, constraints []string) (SelectedPackage, error) {
	tags, err := oci.ListTags(r.ctx, r.registry, repository)
	if err != nil {
		return SelectedPackage{}, fmt.Errorf("list tags for dependency %q in %q: %w", name, repository, err)
	}

	constraint, err := mergeOptionalConstraints(constraints)
	if err != nil {
		return SelectedPackage{}, fmt.Errorf("merge constraints for dependency %q: %w", name, err)
	}

	version, err := selectVersionWithConstraint(tags, constraint)
	if err != nil {
		return SelectedPackage{}, fmt.Errorf("select version for dependency %q: %w", name, err)
	}

	target, desc, err := r.registry.ResolveReference(r.ctx, repository, version)
	if err != nil {
		return SelectedPackage{}, fmt.Errorf("resolve version %q for dependency %q: %w", version, name, err)
	}
	_ = target

	reference := oci.Reference{Repository: repository, Digest: desc.Digest.String()}.Canonical()
	selected, err := r.fetchSelectedByReference(reference)
	if err != nil {
		return SelectedPackage{}, err
	}
	if selected.Name != name {
		return SelectedPackage{}, fmt.Errorf("resolved dependency repository %q to manifest name %q, want %q", repository, selected.Name, name)
	}
	return selected, nil
}

func (r *packageResolver) fetchSelectedByReference(reference string) (SelectedPackage, error) {
	fetched, err := oci.Fetch(r.ctx, r.registry, reference)
	if err != nil {
		return SelectedPackage{}, err
	}

	manifestValue, err := manifestpkg.LoadReader(bytes.NewReader(fetched.Config))
	if err != nil {
		return SelectedPackage{}, fmt.Errorf("load OCI config manifest from %q: %w", reference, err)
	}

	return SelectedPackage{
		Name:       manifestValue.Name,
		Version:    manifestValue.Version,
		Reference:  fetched.Reference,
		Digest:     fetched.Digest,
		Repository: fetched.Repository,
		Manifest:   manifestValue,
	}, nil
}

func ParseDependencyConstraint(raw string) error {
	_, err := ParseConstraint(raw)
	return err
}

func mergeOptionalConstraints(raws []string) (Constraint, error) {
	if len(raws) == 0 {
		return Constraint{}, nil
	}
	return MergeConstraints(raws...)
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (r *packageResolver) pruneUnreachable(root string) {
	reachable := make(map[string]struct{}, len(r.selected))
	stack := []string{root}

	for len(stack) > 0 {
		name := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := reachable[name]; ok {
			continue
		}
		reachable[name] = struct{}{}
		stack = append(stack, r.edges[name]...)
	}

	for name := range r.selected {
		if _, ok := reachable[name]; !ok {
			delete(r.selected, name)
		}
	}
	for name := range r.edges {
		if _, ok := reachable[name]; !ok {
			delete(r.edges, name)
		}
	}
}
