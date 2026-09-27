package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRejectsInvalidScanRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{filepath.Join(dir, "missing"), file} {
		err := run([]string{"--dir", root, "--no-fetch"})
		if err == nil || !strings.Contains(err.Error(), "read scan directory") {
			t.Fatalf("invalid root must fail before starting the TUI: %v", err)
		}
	}
}

func TestRunReturnsUsageErrorForInvalidArgs(t *testing.T) {
	err := run([]string{"--max-depth", "0"})

	if err == nil {
		t.Fatal("expected invalid args to return an error")
	}
	if !strings.Contains(err.Error(), "Invalid --max-depth value: 0") {
		t.Fatalf("expected parse error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage: gitsy [options]") {
		t.Fatalf("expected usage text, got %q", err.Error())
	}
}
