package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestPlainProcess(t *testing.T) {
	if os.Getenv("GITSY_TEST_PROCESS") != "1" {
		return
	}
	for index, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"gitsy"}, os.Args[index+1:]...)
			main()
			os.Exit(0)
		}
	}
	t.Fatal("missing subprocess arguments")
}

func plainCommand(t *testing.T, argv ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestPlainProcess$", "--"}, argv...)...)
	command.Env = append(os.Environ(), "GITSY_TEST_PROCESS=1")
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("plain mode must finish without input: %v", ctx.Err())
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatal(err)
		}
		code = exitErr.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func plainGit(t *testing.T, cwd string, argv ...string) {
	t.Helper()
	result := git.Run(cwd, argv...)
	if !result.OK {
		t.Fatalf("git %v: %s", argv, result.Stderr)
	}
}

func TestPlainCommandReportsAndExitsWithoutTerminal(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	linked := filepath.Join(t.TempDir(), "linked")
	plainGit(t, root, "init", "-q", "-b", "main", repo)
	plainGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "initial")
	plainGit(t, repo, "worktree", "add", "-q", "-b", "feature", linked)
	if err := os.WriteFile(filepath.Join(linked, "hidden-filename.txt"), []byte("change"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := plainCommand(t, "--plain", "--no-fetch", "--dir", root, "--dir", root)
	if code != 0 || stderr != "" {
		t.Fatalf("successful report: exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"Checking repositories…\n", root, repo, linked, "worktree", "main", "feature", "1 untracked", "2 repos"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in report:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "hidden-filename") || strings.Contains(stdout, "\x1b") || strings.Count(stdout, linked) != 2 {
		t.Fatalf("report must show summary counts and preserve group appearances:\n%s", stdout)
	}
}

func TestPlainCommandFetchFailureStillReportsOtherRepos(t *testing.T) {
	root := t.TempDir()
	bad, good := filepath.Join(root, "bad"), filepath.Join(root, "good")
	for _, repo := range []string{bad, good} {
		plainGit(t, root, "init", "-q", "-b", "main", repo)
	}
	plainGit(t, bad, "remote", "add", "origin", filepath.Join(root, "missing-remote"))
	for _, verbose := range []bool{false, true} {
		argv := []string{"--plain", "--dir", root}
		if verbose {
			argv = append(argv, "--verbose")
		}
		stdout, stderr, code := plainCommand(t, argv...)
		if code != 1 || !strings.Contains(stdout, "stale") || !strings.Contains(stdout, good) || !strings.Contains(stderr, "git operations failed") {
			t.Fatalf("failed fetch must report available status before exiting 1: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if strings.Contains(stderr, "gitsy: warning:") != verbose {
			t.Fatalf("detailed warnings require --verbose: %q", stderr)
		}
	}
	_, stderr, code := plainCommand(t, "--plain", "--no-fetch", "--dir", root)
	if code != 0 || stderr != "" {
		t.Fatalf("--no-fetch must succeed with unavailable remote: exit=%d stderr=%q", code, stderr)
	}
}

func TestPlainCommandSyncAlwaysFetchesAndReportsFinalStatus(t *testing.T) {
	root, remotes := t.TempDir(), t.TempDir()
	origin, clone := filepath.Join(remotes, "origin"), filepath.Join(root, "clone")
	plainGit(t, remotes, "init", "-q", "-b", "main", origin)
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "initial")
	plainGit(t, root, "clone", "-q", origin, clone)
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
	stdout, stderr, code := plainCommand(t, "--plain", "--sync", "--no-fetch", "--dir", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "synced ↓1") || strings.Contains(stdout, "1 behind") {
		t.Fatalf("sync must fetch and show final state: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout != git.ResolveCommitContext(context.Background(), origin, "HEAD").Stdout {
		t.Fatal("sync must advance HEAD to upstream")
	}
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "another update")
	if err := os.WriteFile(filepath.Join(clone, "local.txt"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout
	stdout, stderr, code = plainCommand(t, "--plain", "--sync", "--dir", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "1 behind") || strings.Contains(stdout, "synced") {
		t.Fatalf("unsafe sync skip is a normal result: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if before != git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout {
		t.Fatal("sync must preserve dirty checkout")
	}
}

func TestPlainCommandEmptyAndInvalidRoots(t *testing.T) {
	root := t.TempDir()
	stdout, stderr, code := plainCommand(t, "--plain", "--dir", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "No child git repositories found.") {
		t.Fatalf("empty scan must succeed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	_, stderr, code = plainCommand(t, "--plain", "--dir", filepath.Join(root, "missing"))
	if code != 1 || !strings.Contains(stderr, "read scan directory") {
		t.Fatalf("invalid root must fail: exit=%d stderr=%q", code, stderr)
	}
}

func TestPlainResultsExitBehavior(t *testing.T) {
	repo := discover.Repo{Path: "/workspace/repo", DisplayName: "repo"}
	workspace := discover.Workspace{Repos: []discover.Repo{repo}, Groups: []discover.Group{{Path: "/workspace", RepoIndexes: []int{0}}}}
	for _, test := range []struct {
		name   string
		result inspect.Result
		failed bool
	}{
		{"dirty and behind", inspect.Result{Status: status.Parse("## main...origin/main [behind 2]\n M file")}, false},
		{"conflict", inspect.Result{Status: status.Parse("## main\nUU file")}, false},
		{"diverged", inspect.Result{Status: status.Parse("## main...origin/main [ahead 1, behind 2]")}, false},
		{"status failure", inspect.Result{Failed: true}, true},
		{"fetch failure", inspect.Result{Stale: true}, true},
		{"sync failure", inspect.Result{Sync: &inspect.SyncOutcome{Kind: "failed"}}, true},
		{"synced", inspect.Result{Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.result.Repo = repo
			var output bytes.Buffer
			err := printPlainResults(&output, workspace, []inspect.Result{test.result})
			if (err != nil) != test.failed || !strings.Contains(output.String(), repo.Path) {
				t.Fatalf("must print result before returning outcome: err=%v report=%q", err, output.String())
			}
		})
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestPlainOutputFailureAndCancellation(t *testing.T) {
	if err := printPlainResults(failedWriter{}, discover.Workspace{}, nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("must return report write errors: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if err := runPlain(ctx, &output, discover.Workspace{}, true, false, nil); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("cancellation must fail without claiming completed scan: err=%v report=%q", err, output.String())
	}
}
