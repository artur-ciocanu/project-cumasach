package resolve

import (
	"fmt"
	"slices"
	"strings"

	semver "github.com/Masterminds/semver/v3"
)

type versionCandidate struct {
	raw     string
	version *semver.Version
}

func SelectVersion(tags []string, rawConstraint string) (string, error) {
	constraint, err := parseOptionalConstraint(rawConstraint)
	if err != nil {
		return "", err
	}

	candidates := filterVersionCandidates(tags)
	if !constraint.IsZero() {
		candidates = selectMatchingCandidates(candidates, constraint)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no matching semver version")
	}

	slices.SortFunc(candidates, func(a, b versionCandidate) int {
		return b.version.Compare(a.version)
	})

	return candidates[0].raw, nil
}

func parseOptionalConstraint(raw string) (Constraint, error) {
	if strings.TrimSpace(raw) == "" {
		return Constraint{}, nil
	}
	return ParseConstraint(raw)
}

func filterVersionCandidates(tags []string) []versionCandidate {
	candidates := make([]versionCandidate, 0, len(tags))
	for _, tag := range tags {
		version, err := semver.StrictNewVersion(tag)
		if err != nil {
			continue
		}
		candidates = append(candidates, versionCandidate{
			raw:     tag,
			version: version,
		})
	}
	return candidates
}

func selectMatchingCandidates(candidates []versionCandidate, constraint Constraint) []versionCandidate {
	selected := make([]versionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if constraint.CheckVersion(candidate.version) {
			selected = append(selected, candidate)
		}
	}
	return selected
}
