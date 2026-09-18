package main

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	owner       = "flexdinesh"
	repo        = "gitsy"
	formulaName = "gitsy"
	homepage    = "https://github.com/flexdinesh/gitsy"
	desc        = "Scan directories for Git repositories and show a compact status table"
)

type target struct {
	os           string
	arch         string
	homebrewOS   string
	homebrewArch string
}

type formulaOptions struct {
	version   string
	tag       string
	checksums string
	output    string
}

type archive struct {
	target
	artifact string
	sha256   string
	url      string
}

var targets = []target{
	{os: "darwin", arch: "amd64", homebrewOS: "macos", homebrewArch: "intel"},
	{os: "darwin", arch: "arm64", homebrewOS: "macos", homebrewArch: "arm"},
	{os: "linux", arch: "amd64", homebrewOS: "linux", homebrewArch: "intel"},
	{os: "linux", arch: "arm64", homebrewOS: "linux", homebrewArch: "arm"},
}

var checksumLine = regexp.MustCompile(`^([a-f0-9]{64})\s+\*?(.+)$`)

func main() {
	options, err := parseArgs(os.Args[1:])
	if err != nil {
		fail(err)
	}
	checksumText, err := os.ReadFile(options.checksums)
	if err != nil {
		fail(err)
	}
	formula, err := generateFormula(options.version, options.tag, string(checksumText))
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(options.output), 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(options.output, []byte(formula), 0o644); err != nil {
		fail(err)
	}
}

func parseArgs(args []string) (formulaOptions, error) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	parsed := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		key := args[i]
		if !strings.HasPrefix(key, "--") || i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return formulaOptions{}, errors.New("usage: go run ./tools/homebrew-formula -- --version <version> --tag <tag> --checksums <path> --output <path>")
		}
		parsed[strings.TrimPrefix(key, "--")] = args[i+1]
	}
	version, err := requireOption(parsed, "version")
	if err != nil {
		return formulaOptions{}, err
	}
	tag, err := requireOption(parsed, "tag")
	if err != nil {
		return formulaOptions{}, err
	}
	checksums, err := requireOption(parsed, "checksums")
	if err != nil {
		return formulaOptions{}, err
	}
	output, err := requireOption(parsed, "output")
	if err != nil {
		return formulaOptions{}, err
	}
	return formulaOptions{version: version, tag: tag, checksums: checksums, output: output}, nil
}

func requireOption(parsed map[string]string, name string) (string, error) {
	value := strings.TrimSpace(parsed[name])
	if value == "" {
		return "", fmt.Errorf("missing required option --%s", name)
	}
	return value, nil
}

func generateFormula(version, tag, checksumText string) (string, error) {
	checksumByArtifact, err := parseChecksums(checksumText)
	if err != nil {
		return "", err
	}
	encodedTag := url.PathEscape(tag)

	archives := make([]archive, 0, len(targets))
	for _, t := range targets {
		artifact := fmt.Sprintf("%s_%s_%s_%s.tar.gz", formulaName, version, t.os, t.arch)
		sha256, ok := checksumByArtifact[artifact]
		if !ok {
			return "", fmt.Errorf("missing checksum for %s", artifact)
		}
		archives = append(archives, archive{
			target:   t,
			artifact: artifact,
			sha256:   sha256,
			url:      fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, encodedTag, artifact),
		})
	}

	byKey := map[string]archive{}
	for _, a := range archives {
		byKey[a.homebrewOS+"/"+a.homebrewArch] = a
	}
	macosIntel, err := requireArchive(byKey, "macos/intel")
	if err != nil {
		return "", err
	}
	macosArm, err := requireArchive(byKey, "macos/arm")
	if err != nil {
		return "", err
	}
	linuxIntel, err := requireArchive(byKey, "linux/intel")
	if err != nil {
		return "", err
	}
	linuxArm, err := requireArchive(byKey, "linux/arm")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`class Gitsy < Formula
  desc "%s"
  homepage "%s"

  on_macos do
    on_intel do
      url "%s"
      sha256 "%s"
    end

    on_arm do
      url "%s"
      sha256 "%s"
    end
  end

  on_linux do
    on_intel do
      url "%s"
      sha256 "%s"
    end

    on_arm do
      url "%s"
      sha256 "%s"
    end
  end

  def install
    bin.install "gitsy"
  end

  test do
    assert_match "gitsy #{version}", shell_output("#{bin}/gitsy --version")
  end
end
`, desc, homepage,
		macosIntel.url, macosIntel.sha256,
		macosArm.url, macosArm.sha256,
		linuxIntel.url, linuxIntel.sha256,
		linuxArm.url, linuxArm.sha256), nil
}

func requireArchive(archives map[string]archive, key string) (archive, error) {
	a, ok := archives[key]
	if !ok {
		return archive{}, fmt.Errorf("missing archive for %s", key)
	}
	return a, nil
}

func parseChecksums(text string) (map[string]string, error) {
	checksums := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		match := checksumLine.FindStringSubmatch(trimmed)
		if match == nil {
			return nil, fmt.Errorf("invalid checksum line: %s", line)
		}
		checksums[match[2]] = match[1]
	}
	return checksums, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
