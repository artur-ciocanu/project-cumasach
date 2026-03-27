package resolve

import (
	"fmt"
	"slices"
	"strings"

	semver "github.com/Masterminds/semver/v3"
)

func SelectVersion(tags []string, constraint string) (string, error) {
	var matcher *semver.Constraints
	if strings.TrimSpace(constraint) != "" {
		parsed, err := semver.NewConstraint(constraint)
		if err != nil {
			return "", fmt.Errorf("invalid constraint %q: %w", constraint, err)
		}
		matcher = parsed
	}

	var candidates []selectedVersion
	for _, tag := range tags {
		parsed, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if matcher != nil && !matcher.Check(parsed) {
			continue
		}
		candidates = append(candidates, selectedVersion{
			raw:     tag,
			version: parsed,
		})
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no matching semver version")
	}

	slices.SortFunc(candidates, func(a, b selectedVersion) int {
		return b.version.Compare(a.version)
	})

	return candidates[0].raw, nil
}

type selectedVersion struct {
	raw     string
	version *semver.Version
}
