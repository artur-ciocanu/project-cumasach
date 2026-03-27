package resolve

import "testing"

func TestSelectVersion(t *testing.T) {
	t.Run("filters out non-semver tags", func(t *testing.T) {
		got, err := SelectVersion([]string{"latest", "1.2.0", "dev", "1.1.9"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.0")
		}
	})

	t.Run("ignores invalid tag forms such as leading v and partial bare versions", func(t *testing.T) {
		got, err := SelectVersion([]string{"v1.2.3", "1.2", "1.2.3"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.3" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.3")
		}
	})

	t.Run("chooses the highest satisfying stable version", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.4.0", "1.2.3", "1.3.9", "2.0.0"}, ">=1.2.0 <2.0.0")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.4.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.4.0")
		}
	})

	t.Run("accepts bare exact version constraints", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.2.4", "1.2.3", "1.2.2"}, "1.2.3")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.3" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.3")
		}
	})

	t.Run("refuses prereleases unless explicitly admitted", func(t *testing.T) {
		_, err := SelectVersion([]string{"1.3.0-beta.1", "1.3.0-rc.1"}, ">=1.3.0 <2.0.0")
		if err == nil {
			t.Fatal("SelectVersion() error = nil, want prerelease rejection")
		}
	})

	t.Run("prefers a stable release over a prerelease with the same base version", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.3.0-rc.1", "1.3.0", "1.2.9"}, ">=1.3.0-0 <2.0.0")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.3.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.3.0")
		}
	})

	t.Run("allows unconstrained prerelease-only selection", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.3.0-beta.1", "1.3.0-alpha.2"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.3.0-beta.1" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.3.0-beta.1")
		}
	})
}

func TestParseConstraint(t *testing.T) {
	t.Run("invalid empty constraint strings", func(t *testing.T) {
		if _, err := ParseConstraint(" \t "); err == nil {
			t.Fatal("ParseConstraint() error = nil, want invalid empty constraint")
		}
	})

	t.Run("invalid leading v constraint versions", func(t *testing.T) {
		if _, err := ParseConstraint("v1.2.3"); err == nil {
			t.Fatal("ParseConstraint() error = nil, want invalid leading v")
		}
	})

	t.Run("invalid unsupported shorthand and coercions", func(t *testing.T) {
		for _, raw := range []string{"=>1.2.3", "=<1.2.3", "~>1.2.3"} {
			if _, err := ParseConstraint(raw); err == nil {
				t.Fatalf("ParseConstraint(%q) error = nil, want invalid shorthand/coercion", raw)
			}
		}
	})

	t.Run("valid comparator sets", func(t *testing.T) {
		if _, err := ParseConstraint(">=1.0.0 <2.0.0"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})

	t.Run("valid caret ranges", func(t *testing.T) {
		if _, err := ParseConstraint("^1.2.3"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})

	t.Run("valid tilde ranges", func(t *testing.T) {
		if _, err := ParseConstraint("~1.4.2"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})

	t.Run("valid OR expressions", func(t *testing.T) {
		if _, err := ParseConstraint(">=1.0.0 <2.0.0 || ^3.0.0"); err != nil {
			t.Fatalf("ParseConstraint() error = %v", err)
		}
	})
}

func TestMergeConstraints(t *testing.T) {
	t.Run("merged compatible constraints", func(t *testing.T) {
		merged, err := MergeConstraints(">=1.0.0 <2.0.0", "^1.2.3")
		if err != nil {
			t.Fatalf("MergeConstraints() error = %v", err)
		}

		if !merged.Check("1.5.0") {
			t.Fatal("merged constraint should accept 1.5.0")
		}
		if merged.Check("2.0.0") {
			t.Fatal("merged constraint should reject 2.0.0")
		}
	})

	t.Run("merged incompatible constraints", func(t *testing.T) {
		merged, err := MergeConstraints(">=1.0.0 <2.0.0", "^2.1.0")
		if err != nil {
			t.Fatalf("MergeConstraints() error = %v", err)
		}

		if merged.Check("1.5.0") {
			t.Fatal("merged constraint should reject 1.5.0")
		}
		if merged.Check("2.1.0") {
			t.Fatal("merged constraint should reject 2.1.0")
		}
	})
}

func TestRootForms(t *testing.T) {
	t.Run("exact root-form invariants", func(t *testing.T) {
		root, err := NewExactRoot("oci://registry.example.com/agentskills/python-development@sha256:abc")
		if err != nil {
			t.Fatalf("NewExactRoot() error = %v", err)
		}

		if got := root.Reference(); got != "oci://registry.example.com/agentskills/python-development@sha256:abc" {
			t.Fatalf("Reference() = %q, want exact reference", got)
		}
		if root.Name() != "" {
			t.Fatalf("Name() = %q, want empty", root.Name())
		}
		if root.OCIBase() != "" {
			t.Fatalf("OCIBase() = %q, want empty", root.OCIBase())
		}
		if !root.IsExact() {
			t.Fatal("IsExact() = false, want true")
		}
	})

	t.Run("named root-form invariants", func(t *testing.T) {
		root, err := NewNamedRoot("python-development", "registry.example.com/agentskills")
		if err != nil {
			t.Fatalf("NewNamedRoot() error = %v", err)
		}

		if got := root.Name(); got != "python-development" {
			t.Fatalf("Name() = %q, want %q", got, "python-development")
		}
		if got := root.OCIBase(); got != "registry.example.com/agentskills" {
			t.Fatalf("OCIBase() = %q, want %q", got, "registry.example.com/agentskills")
		}
		if root.Reference() != "" {
			t.Fatalf("Reference() = %q, want empty", root.Reference())
		}
		if root.IsExact() {
			t.Fatal("IsExact() = true, want false")
		}
	})
}
