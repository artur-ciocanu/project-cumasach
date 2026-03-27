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

	t.Run("chooses highest satisfying stable version", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.4.0", "1.2.3", "1.3.9", "2.0.0"}, ">=1.2.0 <2.0.0")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.4.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.4.0")
		}
	})

	t.Run("accepts a bare exact version constraint", func(t *testing.T) {
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

	t.Run("prefers stable over prerelease with same base version", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.3.0-rc.1", "1.3.0", "1.2.9"}, ">=1.3.0-0 <2.0.0")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.3.0" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.3.0")
		}
	})

	t.Run("preserves build metadata in selected stable tag", func(t *testing.T) {
		got, err := SelectVersion([]string{"1.2.3+meta", "1.2.2"}, "")
		if err != nil {
			t.Fatalf("SelectVersion() error = %v", err)
		}
		if got != "1.2.3+meta" {
			t.Fatalf("SelectVersion() = %q, want %q", got, "1.2.3+meta")
		}
	})

	t.Run("rejects malformed semver candidates with empty prerelease or build metadata", func(t *testing.T) {
		_, err := SelectVersion([]string{"1.2.3-", "1.2.3+"}, "")
		if err == nil {
			t.Fatal("SelectVersion() error = nil, want malformed tag rejection")
		}
	})
}

func TestRootForms(t *testing.T) {
	t.Run("exact root only carries a reference", func(t *testing.T) {
		root := NewExactRoot("oci://registry.example.com/agentskills/python-development@sha256:abc")

		if got := root.Reference(); got != "oci://registry.example.com/agentskills/python-development@sha256:abc" {
			t.Fatalf("Reference() = %q, want exact reference", got)
		}
		if root.Name() != "" {
			t.Fatalf("Name() = %q, want empty", root.Name())
		}
		if root.OCIBase() != "" {
			t.Fatalf("OCIBase() = %q, want empty", root.OCIBase())
		}
	})

	t.Run("named root carries name plus OCI base", func(t *testing.T) {
		root := NewNamedRoot("python-development", "registry.example.com/agentskills")

		if got := root.Name(); got != "python-development" {
			t.Fatalf("Name() = %q, want %q", got, "python-development")
		}
		if got := root.OCIBase(); got != "registry.example.com/agentskills" {
			t.Fatalf("OCIBase() = %q, want %q", got, "registry.example.com/agentskills")
		}
		if root.Reference() != "" {
			t.Fatalf("Reference() = %q, want empty", root.Reference())
		}
	})
}
