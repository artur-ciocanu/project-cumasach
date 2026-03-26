package manifest

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

func LoadFile(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("open manifest file %q: %w", path, err)
	}
	defer file.Close()

	manifest, err := LoadReader(file)
	if err != nil {
		return Manifest{}, fmt.Errorf("load manifest file %q: %w", path, err)
	}

	return manifest, nil
}

func LoadReader(r io.Reader) (Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}

	if err := validate(data); err != nil {
		return Manifest{}, err
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest JSON: %w", err)
	}

	return manifest, nil
}
