package resolve

import (
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

// Root identifies the initial package to resolve.
type Root struct {
	reference string
	name      string
	ociBase   string
}

func NewExactRoot(reference string) Root {
	return Root{reference: reference}
}

func NewNamedRoot(name, ociBase string) Root {
	return Root{name: name, ociBase: ociBase}
}

func (r Root) Reference() string {
	return r.reference
}

func (r Root) Name() string {
	return r.name
}

func (r Root) OCIBase() string {
	return r.ociBase
}

// SelectedPackage captures the resolved package metadata shared with install flows.
type SelectedPackage struct {
	Name       string
	Version    string
	Reference  string
	Digest     string
	Repository string
	Manifest   manifest.Manifest
}

// Graph records one selected package per skill name and the required dependency edges.
type Graph struct {
	Root     string
	Packages map[string]SelectedPackage
	Edges    map[string][]string
}
