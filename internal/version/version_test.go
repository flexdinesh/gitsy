package version

import "testing"

func TestStringReturnsInjectedVersion(t *testing.T) {
	originalVersion := Version
	t.Cleanup(func() {
		Version = originalVersion
	})

	Version = "1.2.3"

	if got := String(); got != "1.2.3" {
		t.Fatalf("expected injected version, got %q", got)
	}
}

func TestStringReturnsDevFallback(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = "dev"
	Commit = "none"

	got := String()
	if got == "" {
		t.Fatal("expected non-empty version")
	}
}

func TestStringReturnsShortInjectedDevCommit(t *testing.T) {
	originalVersion := Version
	originalCommit := Commit
	t.Cleanup(func() {
		Version = originalVersion
		Commit = originalCommit
	})

	Version = "dev"
	Commit = "1234567890abcdef"

	if got := String(); got != "dev-1234567890ab" {
		t.Fatalf("expected short dev commit, got %q", got)
	}
}
