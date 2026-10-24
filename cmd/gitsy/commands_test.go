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

	"github.com/flexdinesh/gitsy/internal/args"
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

// plainCommand runs gitsy in a subprocess without a terminal on stdin.
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
		t.Fatalf("command must finish without input: %v", ctx.Err())
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

func commit(t *testing.T, repo, message string) {
	t.Helper()
	plainGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", message)
}

func TestStatusCommandSummarizesAndReportsWithVerbose(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	linked := filepath.Join(t.TempDir(), "linked")
	plainGit(t, root, "init", "-q", "-b", "main", repo)
	commit(t, repo, "initial")
	plainGit(t, repo, "worktree", "add", "-q", "-b", "feature", linked)
	if err := os.WriteFile(filepath.Join(linked, "hidden-filename.txt"), []byte("change"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := plainCommand(t, "--no-fetch", "--dir", root, "status")
	if code != 0 || stderr != "" || stdout != "gitsy · 2 repos · 1 changed\n" {
		t.Fatalf("status must print only a summary by default: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = plainCommand(t, "status", "--no-fetch", "--verbose", "--dir", root, "--dir", root)
	if code != 0 || stderr != "" {
		t.Fatalf("successful report: exit=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{root, repo, linked, "worktree", "main", "feature", "1 untracked", "\ngitsy · 2 repos · 1 changed\n"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in report:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "hidden-filename") || strings.Contains(stdout, "\x1b") || strings.Contains(stdout, "Checking") || strings.Count(stdout, linked) != 2 {
		t.Fatalf("report must show summary counts and preserve group appearances:\n%s", stdout)
	}
}

func TestStatusCommandFetchFailureStillReportsOtherRepos(t *testing.T) {
	root := t.TempDir()
	bad, good := filepath.Join(root, "bad"), filepath.Join(root, "good")
	for _, repo := range []string{bad, good} {
		plainGit(t, root, "init", "-q", "-b", "main", repo)
	}
	plainGit(t, bad, "remote", "add", "origin", filepath.Join(root, "missing-remote"))
	for _, verbose := range []bool{false, true} {
		argv := []string{"status", "--dir", root}
		if verbose {
			argv = append(argv, "--verbose")
		}
		stdout, stderr, code := plainCommand(t, argv...)
		if code != 1 || !strings.HasSuffix(stdout, "gitsy · 2 repos · 1 stale\n") || !strings.Contains(stderr, "gitsy: missing-remote ("+bad+"): fetch failed") || strings.Contains(stderr, good) {
			t.Fatalf("failed fetch must summarize and name the failed repo before exiting 1: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if strings.Contains(stdout, good) != verbose {
			t.Fatalf("per-repo report requires --verbose: %q", stdout)
		}
		if strings.Contains(stderr, "gitsy: warning:") != verbose {
			t.Fatalf("detailed warnings require --verbose: %q", stderr)
		}
	}
	_, stderr, code := plainCommand(t, "status", "--no-fetch", "--dir", root)
	if code != 0 || stderr != "" {
		t.Fatalf("--no-fetch must succeed with unavailable remote: exit=%d stderr=%q", code, stderr)
	}
}

func TestSyncCommandFastForwardsAndSkipsUnsafeRepos(t *testing.T) {
	root, remotes := t.TempDir(), t.TempDir()
	origin, clone := filepath.Join(remotes, "origin"), filepath.Join(root, "clone")
	plainGit(t, remotes, "init", "-q", "-b", "main", origin)
	commit(t, origin, "initial")
	plainGit(t, root, "clone", "-q", origin, clone)
	commit(t, origin, "update")
	stdout, stderr, code := plainCommand(t, "--dir", root, "sync")
	if code != 0 || stderr != "" || stdout != "Synced 1 of 1 repo.\n" {
		t.Fatalf("sync must fast-forward and summarize: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout != git.ResolveCommitContext(context.Background(), origin, "HEAD").Stdout {
		t.Fatal("sync must advance HEAD to upstream")
	}
	stdout, stderr, code = plainCommand(t, "--dir", root, "sync")
	if code != 0 || stderr != "" || stdout != "Synced 0 of 1 repo. 1 up to date.\n" {
		t.Fatalf("up-to-date sync: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	commit(t, origin, "another update")
	if err := os.WriteFile(filepath.Join(clone, "local.txt"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout
	stdout, stderr, code = plainCommand(t, "sync", "--verbose", "--dir", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "main ↓1 · 1 untracked") || strings.Contains(stdout, "synced") || !strings.HasSuffix(stdout, "\nSynced 0 of 1 repo. 1 skipped.\n") {
		t.Fatalf("unsafe sync skip is a normal result: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if before != git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout {
		t.Fatal("sync must preserve dirty checkout")
	}
}

func TestFetchUpdatesRemoteRefsWithoutLocalBranches(t *testing.T) {
	root, remotes := t.TempDir(), t.TempDir()
	origin, clone, bad := filepath.Join(remotes, "origin"), filepath.Join(root, "clone"), filepath.Join(root, "bad")
	plainGit(t, remotes, "init", "-q", "-b", "main", origin)
	commit(t, origin, "initial")
	plainGit(t, root, "clone", "-q", origin, clone)
	commit(t, origin, "update")
	before := git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout
	stdout, stderr, code := plainCommand(t, "--dir", root, "fetch")
	if code != 0 || stderr != "" || stdout != "Fetched 1 of 1 repo.\n" {
		t.Fatalf("fetch must summarize fetches: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if git.ResolveCommitContext(context.Background(), clone, "HEAD").Stdout != before {
		t.Fatal("fetch must not update the local branch")
	}
	if git.ResolveCommitContext(context.Background(), clone, "origin/main").Stdout != git.ResolveCommitContext(context.Background(), origin, "HEAD").Stdout {
		t.Fatal("fetch must update remote-tracking refs")
	}
	plainGit(t, root, "init", "-q", "-b", "main", bad)
	plainGit(t, bad, "remote", "add", "origin", filepath.Join(root, "missing-remote"))
	stdout, stderr, code = plainCommand(t, "--dir", root, "fetch")
	if code != 1 || stdout != "Fetched 1 of 2 repos. 1 failed.\n" || stderr != "gitsy: missing-remote ("+bad+"): fetch failed: repo not found\n" {
		t.Fatalf("fetch failure must summarize and name the repo: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestCommandsEmptyAndInvalidRoots(t *testing.T) {
	root := t.TempDir()
	for _, command := range [][]string{{"status"}, {"sync"}, {"fetch"}} {
		stdout, stderr, code := plainCommand(t, append(command, "--dir", root)...)
		if code != 0 || stderr != "" || stdout != "No child git repositories found.\n" {
			t.Fatalf("%v: empty scan must succeed: exit=%d stdout=%q stderr=%q", command, code, stdout, stderr)
		}
	}
	stdout, stderr, code := plainCommand(t, "worktree", "delete-merged", "--dir", root)
	if code != 0 || stderr != "" || stdout != "No merged worktrees.\n" {
		t.Fatalf("empty delete-merged must succeed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	_, stderr, code = plainCommand(t, "status", "--dir", filepath.Join(root, "missing"))
	if code != 1 || !strings.Contains(stderr, "read scan directory") {
		t.Fatalf("invalid root must fail: exit=%d stderr=%q", code, stderr)
	}
}

func TestUsageErrorsExitTwo(t *testing.T) {
	for _, argv := range [][]string{{"--non-interactive"}, {"stats"}, {"worktree"}, {"status", "--fetch-only"}, {"sync", "--fetch-only"}, {"--sync"}, {"sync", "-y"}, {"sync", "--no-fetch"}, {"fetch", "--no-fetch"}} {
		stdout, stderr, code := plainCommand(t, argv...)
		if code != 2 || stdout != "" || !strings.HasPrefix(stderr, "gitsy: ") || !strings.Contains(stderr, "Usage: gitsy [options] [command]") {
			t.Fatalf("%v must be a usage error: exit=%d stdout=%q stderr=%q", argv, code, stdout, stderr)
		}
	}
}

// mergedWorkspace creates a repo with one merged and one unmerged linked worktree.
func mergedWorkspace(t *testing.T) (root, repo, merged, unmerged string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root = t.TempDir()
	repo = filepath.Join(root, "repo")
	merged, unmerged = filepath.Join(t.TempDir(), "done"), filepath.Join(t.TempDir(), "wip")
	plainGit(t, root, "init", "-q", "-b", "main", repo)
	commit(t, repo, "initial")
	plainGit(t, repo, "worktree", "add", "-q", "-b", "done", merged)
	commit(t, merged, "finished work")
	plainGit(t, repo, "merge", "-q", "--ff-only", "done")
	plainGit(t, repo, "worktree", "add", "-q", "-b", "wip", unmerged)
	if err := os.WriteFile(filepath.Join(unmerged, "wip.txt"), []byte("wip"), 0o600); err != nil {
		t.Fatal(err)
	}
	plainGit(t, unmerged, "add", "wip.txt")
	commit(t, unmerged, "unfinished work")
	return root, repo, merged, unmerged
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestDeleteMergedRequiresConfirmationWithoutTerminal(t *testing.T) {
	root, _, merged, unmerged := mergedWorkspace(t)
	stdout, stderr, code := plainCommand(t, "--dir", root, "worktree", "delete-merged")
	if code != 2 || stdout != "" || stderr != "gitsy: confirmation required to delete 1 merged worktree; rerun with -y\n" {
		t.Fatalf("delete-merged without -y must refuse: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !exists(merged) || !exists(unmerged) {
		t.Fatal("refused deletion must keep worktrees")
	}
}

func TestDeleteMergedWithYesDeletesOnlyMergedWorktrees(t *testing.T) {
	root, repo, merged, unmerged := mergedWorkspace(t)
	stdout, stderr, code := plainCommand(t, "--dir", root, "worktree", "delete-merged", "-y", "--verbose")
	if code != 0 || stderr != "" || stdout != "Deleted repo ("+merged+")\nDeleted 1 merged worktree. Branches kept.\n" {
		t.Fatalf("delete-merged -y: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if exists(merged) || !exists(unmerged) {
		t.Fatal("only merged worktrees must be deleted")
	}
	if !git.ResolveCommitContext(context.Background(), repo, "refs/heads/done").OK {
		t.Fatal("deletion must keep the branch")
	}
	stdout, stderr, code = plainCommand(t, "--dir", root, "worktree", "delete-merged", "-y")
	if code != 0 || stderr != "" || stdout != "No merged worktrees.\n" {
		t.Fatalf("nothing left to delete: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestDeleteMergedReportsFailedDeletion(t *testing.T) {
	root, repo, merged, _ := mergedWorkspace(t)
	plainGit(t, repo, "worktree", "lock", merged)
	stdout, stderr, code := plainCommand(t, "--dir", root, "worktree", "delete-merged", "--yes")
	if code != 1 || stdout != "Deleted 0 merged worktrees. 1 failed. Branches kept.\n" || stderr != "gitsy: repo ("+merged+"): delete failed: Worktree is locked; unlock it before deleting.\n" {
		t.Fatalf("locked worktree must fail deletion: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !exists(merged) {
		t.Fatal("locked worktree must remain")
	}
}

func TestDeleteMergedConfirmsOnTerminal(t *testing.T) {
	for _, test := range []struct {
		answer  string
		deleted bool
	}{
		{answer: "y\n", deleted: true},
		{answer: "YES\n", deleted: true},
		{answer: "n\n"},
		{answer: "\n"},
		{answer: ""},
	} {
		t.Run(strings.TrimSpace(test.answer), func(t *testing.T) {
			root, _, merged, _ := mergedWorkspace(t)
			workspace, err := discover.DiscoverGroupedContext(context.Background(), discover.Options{Cwd: root, Dirs: []string{root}, MaxDepth: args.DefaultMaxDepth})
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			out := console{stdin: strings.NewReader(test.answer), stdout: &stdout, stderr: &stderr, terminal: true}
			err = runDeleteMerged(context.Background(), out, workspace, args.Options{Command: args.CommandDeleteMerged, NoFetch: true}, nil)
			prompt := "Merged worktrees:\n  repo (" + merged + ") · merged into main\nDelete 1 merged worktree? Branches kept. [y/N] "
			if stderr.String() != prompt {
				t.Fatalf("unexpected prompt: %q", stderr.String())
			}
			if test.deleted {
				if err != nil || stdout.String() != "Deleted 1 merged worktree. Branches kept.\n" || exists(merged) {
					t.Fatalf("confirmed deletion: err=%v stdout=%q", err, stdout.String())
				}
				return
			}
			if exitCode(io.Discard, err) != 1 || err.Error() != "aborted; no worktrees deleted" || stdout.Len() != 0 || !exists(merged) {
				t.Fatalf("declined deletion must abort with exit 1: err=%v stdout=%q", err, stdout.String())
			}
		})
	}
}

func TestDeleteMergedPromptStopsOnInterrupt(t *testing.T) {
	root, _, merged, _ := mergedWorkspace(t)
	workspace, err := discover.DiscoverGroupedContext(context.Background(), discover.Options{Cwd: root, Dirs: []string{root}, MaxDepth: args.DefaultMaxDepth})
	if err != nil {
		t.Fatal(err)
	}
	reader, writer := io.Pipe()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	var stderr bytes.Buffer
	prompted := make(chan struct{})
	out := console{stdin: reader, stdout: io.Discard, stderr: notifyWriter{&stderr, prompted}, terminal: true}
	go func() {
		<-prompted
		cancel()
	}()
	err = runDeleteMerged(ctx, out, workspace, args.Options{Command: args.CommandDeleteMerged, NoFetch: true}, nil)
	if !errors.Is(err, context.Canceled) || !exists(merged) {
		t.Fatalf("interrupted prompt must cancel without deleting: err=%v", err)
	}
}

type notifyWriter struct {
	output   io.Writer
	notified chan struct{}
}

func (writer notifyWriter) Write(data []byte) (int, error) {
	count, err := writer.output.Write(data)
	if strings.Contains(string(data), "[y/N]") {
		close(writer.notified)
	}
	return count, err
}

func TestPrintFailuresNamesEachFailedRepo(t *testing.T) {
	repo := discover.Repo{Path: "/workspace/repo", DisplayName: "repo"}
	for _, test := range []struct {
		name   string
		result inspect.Result
		want   string
	}{
		{"dirty and behind", inspect.Result{Status: status.Parse("## main...origin/main [behind 2]\n M file")}, ""},
		{"conflict", inspect.Result{Status: status.Parse("## main\nUU file")}, ""},
		{"diverged", inspect.Result{Status: status.Parse("## main...origin/main [ahead 1, behind 2]")}, ""},
		{"synced", inspect.Result{Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}}, ""},
		{"status failure", inspect.Result{Failed: true, Reason: "status failed: timed out"}, "gitsy: repo (/workspace/repo): status failed: timed out\n"},
		{"fetch failure", inspect.Result{Stale: true, Reason: "fetch failed: auth"}, "gitsy: repo (/workspace/repo): fetch failed: auth\n"},
		{"fetch failure without reason", inspect.Result{Stale: true}, "gitsy: repo (/workspace/repo): fetch failed\n"},
		{"sync failure", inspect.Result{Sync: &inspect.SyncOutcome{Kind: "failed"}, Reason: "sync failed: not possible"}, "gitsy: repo (/workspace/repo): sync failed: not possible\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.result.Repo = repo
			var output bytes.Buffer
			err := printFailures(&output, []inspect.Result{test.result})
			if output.String() != test.want || (err != nil) != (test.want != "") {
				t.Fatalf("err=%v output=%q, want %q", err, output.String(), test.want)
			}
		})
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCommandOutputFailureAndCancellation(t *testing.T) {
	repo := discover.Repo{Path: t.TempDir(), DisplayName: "repo"}
	workspace := discover.Workspace{Repos: []discover.Repo{repo}, Groups: []discover.Group{{Path: "/", RepoIndexes: []int{0}}}}
	summary := func([]inspect.Result) string { return "summary" }
	out := console{stdout: failedWriter{}, stderr: io.Discard}
	if err := runInspection(context.Background(), out, workspace, inspect.NewSession(), true, false, summary, nil); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("must return report write errors: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	out = console{stdout: &stdout, stderr: &stderr}
	if err := runInspection(ctx, out, workspace, inspect.NewSession(), true, false, summary, nil); !errors.Is(err, context.Canceled) || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("cancellation must fail without claiming completed scan: err=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
}

func TestExitCodes(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		code    int
		message string
	}{
		{"success", nil, 0, ""},
		{"runtime error", errors.New("read scan directory"), 1, "gitsy: read scan directory\n"},
		{"reported failure", errFailed, 1, ""},
		{"usage", usageError{err: errors.New("Unknown command: x")}, 2, "gitsy: Unknown command: x\n\n" + args.Usage + "\n"},
		{"confirmation required", exitError{code: exitUsage, err: errors.New("confirmation required")}, 2, "gitsy: confirmation required\n"},
		{"interrupted", errInterrupted, 130, "gitsy: interrupted\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if code := exitCode(&output, test.err); code != test.code || output.String() != test.message {
				t.Fatalf("exitCode = %d %q, want %d %q", code, output.String(), test.code, test.message)
			}
		})
	}
}
