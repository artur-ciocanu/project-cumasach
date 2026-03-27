package resolve

import (
	"fmt"
	"strings"
	"unicode"

	semver "github.com/Masterminds/semver/v3"
)

// Constraint wraps Helm-compatible semver matching while enforcing the stricter v1 rules.
type Constraint struct {
	raw     string
	clauses []*semver.Constraints
}

func ParseConstraint(raw string) (Constraint, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return Constraint{}, fmt.Errorf("constraint must not be empty")
	}
	if err := validateConstraintSyntax(normalized); err != nil {
		return Constraint{}, err
	}

	parsed, err := semver.NewConstraint(normalized)
	if err != nil {
		return Constraint{}, fmt.Errorf("invalid constraint %q: %w", raw, err)
	}

	return Constraint{
		raw:     normalized,
		clauses: []*semver.Constraints{parsed},
	}, nil
}

func MergeConstraints(raws ...string) (Constraint, error) {
	merged := Constraint{}
	for _, raw := range raws {
		parsed, err := ParseConstraint(raw)
		if err != nil {
			return Constraint{}, err
		}
		if merged.raw == "" {
			merged.raw = parsed.raw
		} else {
			merged.raw += " " + parsed.raw
		}
		merged.clauses = append(merged.clauses, parsed.clauses...)
	}
	return merged, nil
}

func (c Constraint) Check(rawVersion string) bool {
	version, err := semver.StrictNewVersion(strings.TrimSpace(rawVersion))
	if err != nil {
		return false
	}
	return c.CheckVersion(version)
}

func (c Constraint) CheckVersion(version *semver.Version) bool {
	for _, clause := range c.clauses {
		if !clause.Check(version) {
			return false
		}
	}
	return true
}

func (c Constraint) IsZero() bool {
	return len(c.clauses) == 0
}

func validateConstraintSyntax(raw string) error {
	tokens := strings.Fields(strings.ReplaceAll(raw, "||", " || "))
	if len(tokens) == 0 {
		return fmt.Errorf("constraint must not be empty")
	}

	for i, token := range tokens {
		if token == "||" || token == "-" {
			continue
		}

		core := trimConstraintOperator(token)
		if core == "" {
			return fmt.Errorf("invalid constraint %q", raw)
		}
		if hasLeadingVVersion(core) {
			return fmt.Errorf("invalid constraint %q", raw)
		}
		if isPartialBareVersion(core) && !isHyphenRangeEndpoint(tokens, i) {
			return fmt.Errorf("invalid constraint %q", raw)
		}
	}

	return nil
}

func trimConstraintOperator(token string) string {
	switch {
	case strings.HasPrefix(token, ">="), strings.HasPrefix(token, "<="), strings.HasPrefix(token, "!="):
		return strings.TrimSpace(token[2:])
	case strings.HasPrefix(token, ">"), strings.HasPrefix(token, "<"), strings.HasPrefix(token, "="), strings.HasPrefix(token, "^"), strings.HasPrefix(token, "~"):
		return strings.TrimSpace(token[1:])
	default:
		return token
	}
}

func hasLeadingVVersion(token string) bool {
	if len(token) < 2 {
		return false
	}
	if token[0] != 'v' && token[0] != 'V' {
		return false
	}
	return token[1] >= '0' && token[1] <= '9'
}

func isPartialBareVersion(token string) bool {
	if strings.ContainsAny(token, "*xX") {
		return false
	}
	if strings.Contains(token, "-") || strings.Contains(token, "+") {
		return false
	}

	parts := strings.Split(token, ".")
	if len(parts) != 1 && len(parts) != 2 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if !unicode.IsDigit(r) {
				return false
			}
		}
	}
	return true
}

func isHyphenRangeEndpoint(tokens []string, index int) bool {
	return (index > 0 && tokens[index-1] == "-") || (index+1 < len(tokens) && tokens[index+1] == "-")
}
