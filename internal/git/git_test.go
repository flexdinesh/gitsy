package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestFetchAllContextDoesNotPromptForHTTPCredentials(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_TERMINAL_PROMPT", "1")

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("WWW-Authenticate", `Basic realm="test"`)
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	repoPath := filepath.Join(t.TempDir(), "repo")
	if result := Run(t.TempDir(), "init", "-q", repoPath); !result.OK {
		t.Fatalf("git init failed: %s", result.Stderr)
	}
	if result := Run(repoPath, "remote", "add", "origin", server.URL+"/repo.git"); !result.OK {
		t.Fatalf("git remote add failed: %s", result.Stderr)
	}

	result := FetchAllContext(context.Background(), repoPath, 3*time.Second)
	if result.OK || !strings.Contains(result.Stderr, "terminal prompts disabled") {
		t.Fatalf("expected noninteractive authentication failure, got %+v", result)
	}
}
