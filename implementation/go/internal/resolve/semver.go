package resolve

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

func SelectVersion(tags []string, constraint string) (string, error) {
	comparators, err := parseConstraintSet(constraint)
	if err != nil {
		return "", err
	}

	var candidates []version
	for _, tag := range tags {
		parsed, ok := parseVersion(tag)
		if !ok {
			continue
		}
		if !matchesAll(parsed, comparators) {
			continue
		}
		if parsed.isPrerelease() && !admitsPrerelease(comparators) {
			continue
		}
		candidates = append(candidates, parsed)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no matching semver version")
	}

	slices.SortFunc(candidates, func(a, b version) int {
		return compareVersion(b, a)
	})

	return candidates[0].raw, nil
}

type version struct {
	raw        string
	major      int
	minor      int
	patch      int
	prerelease []string
}

func (v version) isPrerelease() bool {
	return len(v.prerelease) > 0
}

type comparator struct {
	operator string
	version  version
}

func parseConstraintSet(raw string) ([]comparator, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	parts := strings.Fields(value)
	comparators := make([]comparator, 0, len(parts))
	for _, part := range parts {
		operator, versionText, ok := cutComparator(part)
		if !ok {
			return nil, fmt.Errorf("invalid constraint %q", raw)
		}
		parsed, ok := parseVersion(versionText)
		if !ok {
			return nil, fmt.Errorf("invalid constraint %q", raw)
		}
		comparators = append(comparators, comparator{
			operator: operator,
			version:  parsed,
		})
	}
	return comparators, nil
}

func cutComparator(raw string) (string, string, bool) {
	for _, operator := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(raw, operator) {
			return operator, strings.TrimSpace(strings.TrimPrefix(raw, operator)), true
		}
	}
	return "", "", false
}

func matchesAll(candidate version, comparators []comparator) bool {
	for _, comparator := range comparators {
		order := compareVersion(candidate, comparator.version)
		switch comparator.operator {
		case ">":
			if order <= 0 {
				return false
			}
		case ">=":
			if order < 0 {
				return false
			}
		case "<":
			if order >= 0 {
				return false
			}
		case "<=":
			if order > 0 {
				return false
			}
		case "=":
			if order != 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func admitsPrerelease(comparators []comparator) bool {
	for _, comparator := range comparators {
		if comparator.version.isPrerelease() {
			return true
		}
	}
	return false
}

func parseVersion(raw string) (version, bool) {
	original := raw
	if raw == "" {
		return version{}, false
	}

	main, build, hasBuild := strings.Cut(raw, "+")
	if hasBuild {
		if build == "" {
			return version{}, false
		}
		for _, id := range strings.Split(build, ".") {
			if !validBuildIdentifier(id) {
				return version{}, false
			}
		}
	}

	prerelease := ""
	main, prerelease, hasPrerelease := strings.Cut(main, "-")
	if hasPrerelease && prerelease == "" {
		return version{}, false
	}

	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return version{}, false
	}

	major, ok := parseNumericIdentifier(parts[0])
	if !ok {
		return version{}, false
	}
	minor, ok := parseNumericIdentifier(parts[1])
	if !ok {
		return version{}, false
	}
	patch, ok := parseNumericIdentifier(parts[2])
	if !ok {
		return version{}, false
	}

	parsed := version{
		raw:   original,
		major: major,
		minor: minor,
		patch: patch,
	}
	if prerelease != "" {
		ids := strings.Split(prerelease, ".")
		for _, id := range ids {
			if !validPrereleaseIdentifier(id) {
				return version{}, false
			}
		}
		parsed.prerelease = ids
	}
	return parsed, true
}

func parseNumericIdentifier(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	if len(raw) > 1 && raw[0] == '0' {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

func validPrereleaseIdentifier(raw string) bool {
	if raw == "" {
		return false
	}
	allDigits := true
	for _, r := range raw {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
		if r < '0' || r > '9' {
			allDigits = false
		}
	}
	if allDigits && len(raw) > 1 && raw[0] == '0' {
		return false
	}
	return true
}

func validBuildIdentifier(raw string) bool {
	if raw == "" {
		return false
	}
	for _, r := range raw {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func compareVersion(a, b version) int {
	if a.major != b.major {
		return cmpInt(a.major, b.major)
	}
	if a.minor != b.minor {
		return cmpInt(a.minor, b.minor)
	}
	if a.patch != b.patch {
		return cmpInt(a.patch, b.patch)
	}
	return comparePrerelease(a.prerelease, b.prerelease)
}

func comparePrerelease(a, b []string) int {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	if len(a) == 0 {
		return 1
	}
	if len(b) == 0 {
		return -1
	}

	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := range limit {
		left, right := a[i], b[i]
		leftNum, leftIsNum := numericPrerelease(left)
		rightNum, rightIsNum := numericPrerelease(right)
		switch {
		case leftIsNum && rightIsNum:
			if leftNum != rightNum {
				return cmpInt(leftNum, rightNum)
			}
		case leftIsNum:
			return -1
		case rightIsNum:
			return 1
		default:
			if left != right {
				return strings.Compare(left, right)
			}
		}
	}

	return cmpInt(len(a), len(b))
}

func numericPrerelease(raw string) (int, bool) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return value, true
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
