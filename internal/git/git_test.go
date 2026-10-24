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

func TestParseWorktreesRetainsProtectionAndBranch(t *testing.T) {
	got := ParseWorktrees("worktree /repo/main\x00HEAD abc\x00branch refs/heads/main\x00\x00worktree /repo/日本語\nlinked\x00HEAD abc\x00detached\x00locked travel\nreason\x00\x00worktree /repo/feature\x00branch refs/heads/feat/auth\x00locked\x00\x00")
	want := []Worktree{
		{Path: "/repo/main", MainPath: "/repo/main", Branch: "main"},
		{Path: "/repo/日本語\nlinked", MainPath: "/repo/main", Detached: true, Locked: true, LockReason: "travel\nreason"},
		{Path: "/repo/feature", MainPath: "/repo/main", Branch: "feat/auth", Locked: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRemoveWorktreeProtectsFilesAndKeepsBranch(t *testing.T) {
	for _, scenario := range []string{
		"clean", "untracked", "ignored", "modified", "locked", "main", "unregistered", "current", "current child", "canceled",
		"force clean", "force untracked", "force ignored", "force modified", "force locked", "force main", "force unregistered", "force current", "force current child", "force canceled",
	} {
		state, force := strings.CutPrefix(scenario, "force ")
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			directory := t.TempDir()
			mainPath := filepath.Join(directory, "main")
			linked := filepath.Join(directory, "linked space")
			mustGit := func(cwd string, args ...string) {
				t.Helper()
				if result := Run(cwd, args...); !result.OK {
					t.Fatal(result.Stderr)
				}
			}
			mustGit(directory, "init", "-q", "-b", "main", mainPath)
			if err := os.WriteFile(filepath.Join(mainPath, "tracked"), []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			mustGit(mainPath, "add", "tracked")
			mustGit(mainPath, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "initial")
			mustGit(mainPath, "worktree", "add", "-q", "-b", "feature", linked)
			target := linked
			ctx := context.Background()
			switch state {
			case "untracked", "modified", "ignored":
				name := "untracked"
				if state == "modified" {
					name = "tracked"
				}
				if state == "ignored" {
					if err := os.WriteFile(filepath.Join(mainPath, ".git", "info", "exclude"), []byte("untracked\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(linked, name), []byte("keep me"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "locked":
				mustGit(mainPath, "worktree", "lock", linked)
			case "main":
				target = mainPath
			case "unregistered":
				target = t.TempDir()
			case "current", "current child":
				cwd := linked
				if state == "current child" {
					cwd = filepath.Join(linked, "child")
					if err := os.Mkdir(cwd, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				t.Chdir(cwd)
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			result := RemoveWorktreeContext(ctx, mainPath, target, force)
			dirty := state == "untracked" || state == "modified"
			wantRemoved := state == "clean" || state == "ignored" || force && dirty
			if result.NeedsForce != (dirty && !force) {
				t.Fatalf("only changed files should offer force confirmation: %+v", result)
			}
			if result.OK != wantRemoved {
				t.Fatalf("unexpected removal result: %+v", result)
			}
			_, err := os.Stat(target)
			if wantRemoved {
				if !os.IsNotExist(err) {
					t.Fatalf("worktree still exists: %v", err)
				}
				if result := ResolveCommitContext(ctx, mainPath, "feature"); !result.OK {
					t.Fatal("removal deleted branch")
				}
				return
			} else if err != nil {
				t.Fatalf("protected directory lost: %v", err)
			}
			switch state {
			case "locked":
				mustGit(mainPath, "worktree", "unlock", linked)
			case "untracked", "ignored":
				if err := os.Remove(filepath.Join(linked, "untracked")); err != nil {
					t.Fatal(err)
				}
			case "modified":
				if err := os.WriteFile(filepath.Join(linked, "tracked"), []byte("original"), 0o600); err != nil {
					t.Fatal(err)
				}
			default:
				return
			}
			if result := RemoveWorktreeContext(ctx, mainPath, linked, false); !result.OK {
				t.Fatalf("retry after cleanup/unlock failed: %+v", result)
			}
		})
	}
}

func TestRemoveWorktreeRequiresForceForSubmodules(t *testing.T) {
	for _, scenario := range []string{"uninitialized", "initialized", "deinitialized", "force initialized"} {
		state, force := strings.CutPrefix(scenario, "force ")
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			directory := t.TempDir()
			mustGit := func(cwd string, args ...string) {
				t.Helper()
				args = append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "protocol.file.allow=always"}, args...)
				if result := Run(cwd, args...); !result.OK {
					t.Fatal(result.Stderr)
				}
			}
			subPath := filepath.Join(directory, "sub")
			mainPath := filepath.Join(directory, "main")
			linked := filepath.Join(directory, "linked")
			for _, path := range []string{subPath, mainPath} {
				mustGit(directory, "init", "-q", "-b", "main", path)
				mustGit(path, "commit", "--allow-empty", "-qm", "initial")
			}
			mustGit(mainPath, "submodule", "add", "-q", subPath, "sub")
			mustGit(mainPath, "commit", "-qm", "submodule")
			mustGit(mainPath, "worktree", "add", "-q", "-b", "feature", linked)
			if state != "uninitialized" {
				mustGit(linked, "submodule", "update", "-q", "--init")
			}
			if state == "deinitialized" {
				mustGit(linked, "submodule", "deinit", "-q", "--all")
			}
			result := RemoveWorktreeContext(context.Background(), mainPath, linked, force)
			wantForce := state != "uninitialized" && !force
			if result.NeedsForce != wantForce || result.OK == wantForce {
				t.Fatalf("submodules must require force confirmation only when Git requires --force: %+v", result)
			}
			if _, err := os.Stat(linked); wantForce == os.IsNotExist(err) {
				t.Fatalf("unexpected worktree presence: %v", err)
			}
		})
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

func TestRemoteURLsContextListsRemotes(t *testing.T) {
	repo := t.TempDir()
	mustFetchGit(t, repo, "init", "-q")
	none := RemoteURLsContext(context.Background(), repo)
	if none.OK || none.Status != 1 || len(ParseRemoteURLs(none.Stdout)) != 0 {
		t.Fatalf("repo without remotes must exit 1 with no URLs: %+v", none)
	}
	mustFetchGit(t, repo, "remote", "add", "origin", "git@github.com:owner/repo.git")
	mustFetchGit(t, repo, "remote", "add", "fork.name", "https://example.com/fork.git")
	result := RemoteURLsContext(context.Background(), repo)
	want := map[string]string{"origin": "git@github.com:owner/repo.git", "fork.name": "https://example.com/fork.git"}
	if !result.OK || !reflect.DeepEqual(ParseRemoteURLs(result.Stdout), want) {
		t.Fatalf("expected %v, got %+v", want, result)
	}
}

func TestParseRemoteURLsKeepsFirstURLAndSkipsOtherKeys(t *testing.T) {
	output := "remote.origin.url\na\x00remote.origin.url\nb\x00remote..url\nc\x00branch.main.url\nd\x00"
	if got := ParseRemoteURLs(output); !reflect.DeepEqual(got, map[string]string{"origin": "a"}) {
		t.Fatalf("unexpected URLs: %v", got)
	}
}
