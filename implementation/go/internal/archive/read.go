package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/artur-ciocanu/project-cumasach/implementation/go/internal/manifest"
)

var unixEpoch = time.Unix(0, 0).UTC()
var windowsDrivePathPattern = regexp.MustCompile(`^[A-Za-z]:([/\\]|$)`)

type archiveState struct {
	topLevel      string
	manifest      manifest.Manifest
	manifestBytes []byte
	manifestFound bool
	skillFound    bool
	manifestPath  string
}

func ReadManifestTGZ(r io.Reader) (manifest.Manifest, error) {
	state, err := inspectArchive(r, nil)
	if err != nil {
		return manifest.Manifest{}, err
	}

	return state.manifest, nil
}

func ReadMirroredManifestTGZ(r io.Reader) ([]byte, manifest.Manifest, error) {
	state, err := inspectArchive(r, nil)
	if err != nil {
		return nil, manifest.Manifest{}, err
	}

	return append([]byte(nil), state.manifestBytes...), state.manifest, nil
}

func inspectArchive(r io.Reader, onFile func(header *tar.Header, reader io.Reader, cleanName string) error) (archiveState, error) {
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return archiveState{}, fmt.Errorf("open gzip stream: %w", err)
	}
	defer func() { _ = gzipReader.Close() }()

	tarReader := tar.NewReader(gzipReader)
	state := archiveState{}
	seenEntries := map[string]struct{}{}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return archiveState{}, fmt.Errorf("read tar entry: %w", err)
		}

		cleanName, err := validateArchivePath(header.Name)
		if err != nil {
			return archiveState{}, err
		}
		if _, exists := seenEntries[cleanName]; exists {
			return archiveState{}, fmt.Errorf("invalid archive entry %q: duplicate path %q", header.Name, cleanName)
		}
		seenEntries[cleanName] = struct{}{}

		topLevel := topLevelName(cleanName)
		if topLevel == "" {
			return archiveState{}, fmt.Errorf("invalid archive entry %q: missing top-level directory", header.Name)
		}

		if state.topLevel == "" {
			state.topLevel = topLevel
		} else if state.topLevel != topLevel {
			return archiveState{}, fmt.Errorf("archive contains multiple top-level directories: %q and %q", state.topLevel, topLevel)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if onFile != nil {
				if err := onFile(header, tarReader, cleanName); err != nil {
					return archiveState{}, err
				}
			}
		case tar.TypeReg:
			if cleanName == path.Join(state.topLevel, ".skill", "manifest.json") {
				manifestBytes, err := io.ReadAll(tarReader)
				if err != nil {
					return archiveState{}, fmt.Errorf("read mirrored manifest: %w", err)
				}

				loaded, err := manifest.LoadReader(bytes.NewReader(manifestBytes))
				if err != nil {
					return archiveState{}, fmt.Errorf("load mirrored manifest: %w", err)
				}

				state.manifest = loaded
				state.manifestBytes = append([]byte(nil), manifestBytes...)
				state.manifestFound = true
				state.manifestPath = cleanName
				if onFile != nil {
					if err := onFile(header, bytes.NewReader(manifestBytes), cleanName); err != nil {
						return archiveState{}, err
					}
				}
			} else {
				if cleanName == path.Join(state.topLevel, "SKILL.md") {
					state.skillFound = true
				}

				if onFile != nil {
					if err := onFile(header, tarReader, cleanName); err != nil {
						return archiveState{}, err
					}
				}
			}
		case tar.TypeSymlink, tar.TypeLink:
			return archiveState{}, fmt.Errorf("invalid archive entry %q: links are not allowed", header.Name)
		default:
			return archiveState{}, fmt.Errorf("invalid archive entry %q: unsupported type %d", header.Name, header.Typeflag)
		}
	}

	if state.topLevel == "" {
		return archiveState{}, fmt.Errorf("archive is empty")
	}

	if !state.manifestFound {
		return archiveState{}, fmt.Errorf("archive missing %q", path.Join(state.topLevel, ".skill", "manifest.json"))
	}

	if !state.skillFound {
		return archiveState{}, fmt.Errorf("archive missing %q", path.Join(state.topLevel, "SKILL.md"))
	}

	if state.manifest.Name != state.topLevel {
		return archiveState{}, fmt.Errorf("archive top-level directory %q does not match manifest name %q", state.topLevel, state.manifest.Name)
	}

	return state, nil
}

func validateArchivePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("invalid archive entry %q: empty path", name)
	}

	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("invalid archive entry %q: absolute paths are not allowed", name)
	}
	if windowsDrivePathPattern.MatchString(name) {
		return "", fmt.Errorf("invalid archive entry %q: absolute paths are not allowed", name)
	}

	trimmed := strings.TrimSuffix(name, "/")
	for _, component := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if component == "" || component == "." {
			continue
		}
		if component == ".." {
			return "", fmt.Errorf("invalid archive entry %q: path traversal is not allowed", name)
		}
		if strings.ContainsAny(component, "\x00\r\n") {
			return "", fmt.Errorf("invalid archive entry %q: invalid path characters", name)
		}
	}

	cleanName := path.Clean(name)
	if cleanName == "." {
		return "", fmt.Errorf("invalid archive entry %q: empty path", name)
	}

	return cleanName, nil
}

func topLevelName(name string) string {
	before, _, found := strings.Cut(name, "/")
	if !found {
		return before
	}

	return before
}
