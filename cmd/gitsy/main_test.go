package main

import (
	"strings"
	"testing"
)

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
