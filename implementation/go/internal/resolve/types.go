package resolve

import (
	"fmt"
	"strings"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

// Root identifies the initial package to resolve.
type Root struct {
	reference string
	name      string
	ociBase   string
}

func NewExactRoot(reference string) (Root, error) {
	return NewExactRootWithBase(reference, "")
}

func NewExactRootWithBase(reference, ociBase string) (Root, error) {
	reference = strings.TrimSpace(reference)
	ociBase = strings.TrimSpace(ociBase)
	if reference == "" {
		return Root{}, fmt.Errorf("exact root reference must not be empty")
	}

	return Root{reference: reference, ociBase: ociBase}, nil
}

func NewNamedRoot(name, ociBase string) (Root, error) {
	name = strings.TrimSpace(name)
	ociBase = strings.TrimSpace(ociBase)
	if name == "" {
		return Root{}, fmt.Errorf("named root name must not be empty")
	}
	if ociBase == "" {
		return Root{}, fmt.Errorf("named root OCI base must not be empty")
	}

	return Root{name: name, ociBase: ociBase}, nil
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

func (r Root) IsExact() bool {
	return r.reference != ""
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
