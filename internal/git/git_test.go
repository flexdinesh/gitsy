package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/flexdinesh/gitsy/internal/status"
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

func TestShortStatusIgnoresUserFormattingAndIncludesUntrackedFiles(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "initial"},
		{"config", "color.status", "always"},
		{"config", "status.showUntrackedFiles", "no"},
	} {
		if result := Run(repo, args...); !result.OK {
			t.Fatal(result.Stderr)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "file.go"), []byte("untracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := ShortStatus(repo)
	if !result.OK {
		t.Fatal(result.Stderr)
	}
	parsed := status.Parse(result.Stdout)
	if strings.Contains(result.Stdout, "\x1b") || parsed.Branch == nil || parsed.Branch.Name != "main" ||
		len(parsed.Items) != 1 || parsed.Items[0].Category != status.Untracked || parsed.Items[0].Path != "file.go" {
		t.Fatalf("expected plain, accurate status despite user configuration: %q", result.Stdout)
	}
}

func TestParseWorktreePathsPreservesSpecialCharacters(t *testing.T) {
	want := []string{"/repo/main", "/repo/日本語\r\nwith\ttabs ", "/repo/trailing\n"}
	porcelain := ""
	for _, path := range want {
		porcelain += "worktree " + path + "\x00HEAD abc123\x00branch refs/heads/main\x00\x00"
	}
	if got := ParseWorktreePaths(porcelain); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected paths %#v, got %#v", want, got)
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
