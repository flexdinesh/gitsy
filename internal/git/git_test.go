package git

import (
	"context"
	"strings"
	"testing"
	"time"
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

func TestShortStatusContextReturnsTimeoutResult(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result := ShortStatusContext(ctx, ".")
	if result.OK {
		t.Fatal("expected timed out status to fail")
	}
	if result.Stderr != "git status timed out" {
		t.Fatalf("expected timeout error, got %q", result.Stderr)
	}
}
