package git

import (
	"context"
	"strings"
	"testing"
)

func TestRunContextReturnsCanceledResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := RunContext(ctx, ".", "--version")

	if result.OK {
		t.Fatal("expected canceled command to fail")
	}
	if !strings.Contains(result.Stderr, context.Canceled.Error()) {
		t.Fatalf("expected canceled stderr, got %q", result.Stderr)
	}
}
