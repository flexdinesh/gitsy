package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type releaseSeries struct {
	major string
	minor string
}

func main() {
	seriesText, err := os.ReadFile(".release-version")
	if err != nil {
		fail(err)
	}
	series, err := parseSeries(string(seriesText))
	if err != nil {
		fail(err)
	}
	tags, err := gitTags("--list")
	if err != nil {
		fail(err)
	}
	headTags, err := gitTags("--points-at", "HEAD", "--list")
	if err != nil {
		fail(err)
	}
	tag, err := selectTag(series, tags, headTags)
	if err != nil {
		fail(err)
	}
	fmt.Println(tag)
}

func parseSeries(value string) (releaseSeries, error) {
	parts := strings.Split(strings.TrimSpace(value), ".")
	if len(parts) != 2 || !validNumber(parts[0]) || !validNumber(parts[1]) {
		return releaseSeries{}, errors.New(".release-version must contain a major.minor series such as 0.1")
	}
	return releaseSeries{major: parts[0], minor: parts[1]}, nil
}

func validNumber(value string) bool {
	if value == "0" {
		return true
	}
	if value == "" || value[0] == '0' {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func selectTag(series releaseSeries, tags, headTags []string) (string, error) {
	if patch, ok := highestPatch(series, headTags); ok {
		return formatTag(series, patch), nil
	}
	patch, ok := highestPatch(series, tags)
	if !ok {
		return formatTag(series, 0), nil
	}
	if patch == math.MaxInt {
		return "", errors.New("release patch version overflow")
	}
	return formatTag(series, patch+1), nil
}

func highestPatch(series releaseSeries, tags []string) (int, bool) {
	prefix := "v" + series.major + "." + series.minor + "."
	highest, found := 0, false
	for _, tag := range tags {
		if !strings.HasPrefix(tag, prefix) {
			continue
		}
		value := strings.TrimPrefix(tag, prefix)
		if !validNumber(value) {
			continue
		}
		patch, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		if !found || patch > highest {
			highest, found = patch, true
		}
	}
	return highest, found
}

func formatTag(series releaseSeries, patch int) string {
	return fmt.Sprintf("v%s.%s.%d", series.major, series.minor, patch)
}

func gitTags(arguments ...string) ([]string, error) {
	output, err := exec.Command("git", append([]string{"tag"}, arguments...)...).Output()
	if err != nil {
		return nil, fmt.Errorf("list Git tags: %w", err)
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return nil, nil
	}
	return strings.Split(value, "\n"), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
