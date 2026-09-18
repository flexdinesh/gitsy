package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFormulaIncludesAllArchives(t *testing.T) {
	formula, err := generateFormula("0.1.2", "v0.1.2", strings.Join(completeChecksums(), "\n")+"\n")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"class Gitsy < Formula",
		`homepage "https://github.com/flexdinesh/gitsy"`,
		"releases/download/v0.1.2/gitsy_0.1.2_darwin_amd64.tar.gz",
		"releases/download/v0.1.2/gitsy_0.1.2_darwin_arm64.tar.gz",
		"releases/download/v0.1.2/gitsy_0.1.2_linux_amd64.tar.gz",
		"releases/download/v0.1.2/gitsy_0.1.2_linux_arm64.tar.gz",
		strings.Repeat("a", 64),
		strings.Repeat("b", 64),
		strings.Repeat("c", 64),
		strings.Repeat("d", 64),
		`bin.install "gitsy"`,
		`assert_match "gitsy #{version}"`,
	} {
		if !strings.Contains(formula, want) {
			t.Errorf("formula missing %q", want)
		}
	}
}

func TestGenerateFormulaRejectsMalformedChecksumLine(t *testing.T) {
	_, err := generateFormula("0.1.2", "v0.1.2", "not-a-checksum  gitsy_0.1.2_darwin_amd64.tar.gz\n")
	if err == nil || !strings.Contains(err.Error(), "invalid checksum line") {
		t.Fatalf("expected invalid checksum line error, got %v", err)
	}
}

func TestGenerateFormulaRejectsMissingArchive(t *testing.T) {
	checksums := []string{}
	for _, line := range completeChecksums() {
		if !strings.Contains(line, "linux_arm64") {
			checksums = append(checksums, line)
		}
	}
	_, err := generateFormula("0.1.2", "v0.1.2", strings.Join(checksums, "\n")+"\n")
	if err == nil || !strings.Contains(err.Error(), "missing checksum for gitsy_0.1.2_linux_arm64.tar.gz") {
		t.Fatalf("expected missing checksum error, got %v", err)
	}
}

func TestParseArgsAcceptsSeparatorAndRejectsMissingOption(t *testing.T) {
	options, err := parseArgs([]string{"--", "--version", "0.1.2", "--tag", "v0.1.2", "--checksums", "c.txt", "--output", "o.rb"})
	if err != nil {
		t.Fatal(err)
	}
	if options.version != "0.1.2" || options.tag != "v0.1.2" {
		t.Fatalf("unexpected options %+v", options)
	}

	if _, err := parseArgs([]string{"--version", "0.1.2", "--tag", "v0.1.2"}); err == nil {
		t.Fatal("parseArgs() succeeded with missing options")
	}
}

func TestMainWritesFormulaFile(t *testing.T) {
	dir := t.TempDir()
	checksumsPath := filepath.Join(dir, "checksums.txt")
	outputPath := filepath.Join(dir, "nested", "gitsy.rb")
	if err := os.WriteFile(checksumsPath, []byte(strings.Join(completeChecksums(), "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checksumText, err := os.ReadFile(checksumsPath)
	if err != nil {
		t.Fatal(err)
	}
	formula, err := generateFormula("0.1.2", "v0.1.2", string(checksumText))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte(formula), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatal(err)
	}
}

func completeChecksums() []string {
	return []string{
		strings.Repeat("a", 64) + "  gitsy_0.1.2_darwin_amd64.tar.gz",
		strings.Repeat("b", 64) + "  gitsy_0.1.2_darwin_arm64.tar.gz",
		strings.Repeat("c", 64) + "  gitsy_0.1.2_linux_amd64.tar.gz",
		strings.Repeat("d", 64) + "  gitsy_0.1.2_linux_arm64.tar.gz",
	}
}
