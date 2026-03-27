package manifest

// Manifest is the typed v1 skill manifest shape used by the initial CLI slice.
type Manifest struct {
	SchemaVersion string       `json:"schemaVersion"`
	PackageType   string       `json:"packageType"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Description   string       `json:"description,omitempty"`
	Skill         Skill        `json:"skill"`
	Dependencies  []Dependency `json:"dependencies,omitempty"`
	Requirements  Requirements `json:"requirements,omitempty"`
	Source        Source       `json:"source,omitempty"`
	Publisher     Publisher    `json:"publisher,omitempty"`
}

type Skill struct {
	Entrypoint string `json:"entrypoint"`
}

type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Requirements struct {
	Binaries []string `json:"binaries,omitempty"`
	OS       []string `json:"os,omitempty"`
	Env      []string `json:"env,omitempty"`
}

type Source struct {
	URL        string `json:"url,omitempty"`
	Repository string `json:"repository,omitempty"`
}

type Publisher struct {
	Name string `json:"name,omitempty"`
}
