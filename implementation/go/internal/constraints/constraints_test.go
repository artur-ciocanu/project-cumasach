package constraints

import (
	"testing"
)

func TestParseConstraintRejectsEmpty(t *testing.T) {
	for _, raw := range []string{"", " ", "\t", "  \t  "} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want empty constraint rejection", raw)
		}
	}
}

func TestParseConstraintRejectsComma(t *testing.T) {
	if _, err := ParseConstraint(">=1.0.0, <2.0.0"); err == nil {
		t.Fatal("ParseConstraint() error = nil, want comma rejection")
	}
}

func TestParseConstraintRejectsLeadingV(t *testing.T) {
	for _, raw := range []string{"v1.2.3", "V1.2.3", "^v1.2.3", ">=V1.0.0"} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want leading v rejection", raw)
		}
	}
}

func TestParseConstraintRejectsUnsupportedOperators(t *testing.T) {
	for _, raw := range []string{"=>1.2.3", "=<1.2.3", "~>1.2.3"} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want unsupported operator rejection", raw)
		}
	}
}

func TestParseConstraintRejectsPartialBareVersions(t *testing.T) {
	for _, raw := range []string{"1", "1.2"} {
		if _, err := ParseConstraint(raw); err == nil {
			t.Fatalf("ParseConstraint(%q) error = nil, want partial bare version rejection", raw)
		}
	}
}

func TestParseConstraintAcceptsPartialBareVersionInHyphenRange(t *testing.T) {
	c, err := ParseConstraint("1.0.0 - 2.0")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	if !c.Check("1.5.0") {
		t.Fatal("hyphen range should accept 1.5.0")
	}
}

func TestParseConstraintAcceptsValidForms(t *testing.T) {
	valid := []string{
		"^1.2.3",
		"~1.4.2",
		">=1.0.0 <2.0.0",
		">=1.0.0 <2.0.0 || ^3.0.0",
		"1.2.3",
		"!=1.0.0",
		"1.0.0-alpha",
	}
	for _, raw := range valid {
		if _, err := ParseConstraint(raw); err != nil {
			t.Fatalf("ParseConstraint(%q) error = %v, want success", raw, err)
		}
	}
}

func TestCheckMatchesExpectedVersions(t *testing.T) {
	c, err := ParseConstraint("^1.2.3")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	if !c.Check("1.5.0") {
		t.Fatal("^1.2.3 should accept 1.5.0")
	}
	if c.Check("2.0.0") {
		t.Fatal("^1.2.3 should reject 2.0.0")
	}
	if c.Check("1.2.2") {
		t.Fatal("^1.2.3 should reject 1.2.2")
	}
}

func TestCheckRejectsInvalidVersionStrings(t *testing.T) {
	c, err := ParseConstraint("^1.0.0")
	if err != nil {
		t.Fatalf("ParseConstraint() error = %v", err)
	}
	for _, bad := range []string{"", "latest", "v1.0.0", "1.0"} {
		if c.Check(bad) {
			t.Fatalf("Check(%q) = true, want false for invalid version", bad)
		}
	}
}

func TestMergeConstraintsAppliesAll(t *testing.T) {
	merged, err := MergeConstraints(">=1.0.0 <2.0.0", "^1.5.0")
	if err != nil {
		t.Fatalf("MergeConstraints() error = %v", err)
	}
	if !merged.Check("1.5.0") {
		t.Fatal("merged should accept 1.5.0")
	}
	if merged.Check("1.4.0") {
		t.Fatal("merged should reject 1.4.0 (fails ^1.5.0)")
	}
	if merged.Check("2.0.0") {
		t.Fatal("merged should reject 2.0.0")
	}
}

func TestMergeConstraintsRejectsInvalidInput(t *testing.T) {
	if _, err := MergeConstraints("^1.0.0", ""); err == nil {
		t.Fatal("MergeConstraints() error = nil, want empty constraint rejection")
	}
}

func TestIsZero(t *testing.T) {
	var c Constraint
	if !c.IsZero() {
		t.Fatal("zero Constraint.IsZero() = false, want true")
	}
	parsed, _ := ParseConstraint("^1.0.0")
	if parsed.IsZero() {
		t.Fatal("parsed Constraint.IsZero() = true, want false")
	}
}

func TestValidateConstraintSyntaxInternals(t *testing.T) {
	t.Run("rejects empty token after operator", func(t *testing.T) {
		if err := validateConstraintSyntax(">="); err == nil {
			t.Fatal("validateConstraintSyntax(\">=\") error = nil, want rejection")
		}
	})

	t.Run("rejects uppercase V prefix", func(t *testing.T) {
		if !hasLeadingVVersion("V1.0.0") {
			t.Fatal("hasLeadingVVersion(\"V1.0.0\") = false, want true")
		}
	})

	t.Run("single character not a leading v", func(t *testing.T) {
		if hasLeadingVVersion("v") {
			t.Fatal("hasLeadingVVersion(\"v\") = true, want false")
		}
	})

	t.Run("partial bare version detection", func(t *testing.T) {
		if !isPartialBareVersion("1") {
			t.Fatal("isPartialBareVersion(\"1\") = false, want true")
		}
		if !isPartialBareVersion("1.2") {
			t.Fatal("isPartialBareVersion(\"1.2\") = false, want true")
		}
		if isPartialBareVersion("1.2.3") {
			t.Fatal("isPartialBareVersion(\"1.2.3\") = true, want false (full semver)")
		}
		if isPartialBareVersion("1.2.3-alpha") {
			t.Fatal("isPartialBareVersion(\"1.2.3-alpha\") = true, want false (has prerelease)")
		}
		if isPartialBareVersion("*") {
			t.Fatal("isPartialBareVersion(\"*\") = true, want false (wildcard)")
		}
	})

	t.Run("hyphen range endpoint detection", func(t *testing.T) {
		tokens := []string{"1.0.0", "-", "2.0"}
		if !isHyphenRangeEndpoint(tokens, 0) {
			t.Fatal("isHyphenRangeEndpoint(tokens, 0) = false, want true (before hyphen)")
		}
		if !isHyphenRangeEndpoint(tokens, 2) {
			t.Fatal("isHyphenRangeEndpoint(tokens, 2) = false, want true (after hyphen)")
		}
	})
}
