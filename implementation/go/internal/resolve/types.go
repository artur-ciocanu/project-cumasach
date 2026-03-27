package resolve

import (
	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

// Root identifies the initial package to resolve.
type Root struct {
	Reference string
	Name      string
	OCIBase   string
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
