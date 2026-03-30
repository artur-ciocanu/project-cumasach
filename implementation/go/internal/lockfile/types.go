package lockfile

const schemaVersionV1 = "v1"

type File struct {
	SchemaVersion string    `json:"schemaVersion"`
	Root          Root      `json:"root"`
	Packages      []Package `json:"packages"`
	Edges         []Edge    `json:"edges"`
}

type Root struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Reference string `json:"reference"`
}

type Package struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Digest    string `json:"digest"`
	Reference string `json:"reference"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}
