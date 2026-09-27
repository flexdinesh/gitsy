package inspect

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
)

func TestRepoOutcomes(t *testing.T) {
	behind := "## main...origin/main [behind 2]\n"
	clean := "## main...origin/main\n"
	failure := git.Result{Stderr: "command failed", Status: 1}
	for _, test := range []struct {
		name        string
		noFetch     bool
		sync        bool
		before      string
		fetchFails  bool
		statusFails bool
		mergeFails  bool
		targetFails bool
		headFails   bool
		head        string
		postFails   bool
		wantFailed  bool
		wantStale   bool
		wantSync    string
		wantWarning string
	}{
		{name: "local clean", noFetch: true, before: clean},
		{name: "fetch failure", before: clean, fetchFails: true, wantStale: true, wantWarning: "Fetch failed"},
		{name: "status failure", statusFails: true, wantFailed: true, wantWarning: "Failed to read status"},
		{name: "sync always fetches", noFetch: true, sync: true, before: behind, wantSync: "synced"},
		{name: "dirty skipped", sync: true, before: behind + " M file.go\n"},
		{name: "untracked skipped", sync: true, before: behind + "?? file.go\n"},
		{name: "diverged skipped", sync: true, before: "## main...origin/main [ahead 1, behind 2]\n"},
		{name: "gone skipped", sync: true, before: "## main...origin/main [gone]\n"},
		{name: "merge failure", sync: true, before: behind, mergeFails: true, wantSync: "failed", wantWarning: "Sync failed"},
		{name: "target unavailable", sync: true, before: behind, targetFails: true, wantSync: "failed", wantWarning: "Sync failed"},
		{name: "head unavailable", sync: true, before: behind, headFails: true, wantSync: "failed", wantWarning: "Sync failed"},
		{name: "head unchanged", sync: true, before: behind, head: "old-commit", wantSync: "failed", wantWarning: "HEAD did not reach"},
		{name: "status failure after verified sync", sync: true, before: behind, postFails: true, wantFailed: true, wantSync: "synced", wantWarning: "Failed to read status after sync"},
	} {
		t.Run(test.name, func(t *testing.T) {
			merged := false
			fetched := false
			commands := gitCommands{
				fetch: func(context.Context, string, time.Duration) git.Result {
					fetched = true
					if test.fetchFails {
						return failure
					}
					return git.Result{OK: true}
				},
				status: func(context.Context, string) git.Result {
					if test.statusFails || merged && test.postFails {
						return failure
					}
					raw := test.before
					if merged {
						raw = clean
					}
					return git.Result{OK: true, Stdout: raw}
				},
				merge: func(_ context.Context, _ string, commit string) git.Result {
					if commit != "upstream-commit" {
						t.Fatalf("must merge the resolved target, got %q", commit)
					}
					merged = true
					if test.mergeFails {
						return failure
					}
					return git.Result{OK: true}
				},
				resolve: func(_ context.Context, _ string, revision string) git.Result {
					if revision == "@{upstream}" && test.targetFails || revision == "HEAD" && test.headFails {
						return failure
					}
					commit := "upstream-commit"
					if revision == "HEAD" && test.head != "" {
						commit = test.head
					}
					return git.Result{OK: true, Stdout: commit + "\n"}
				},
			}
			warnings := []string{}
			got := repoContext(context.Background(), discover.Repo{Path: "/repo", DisplayName: "repo"}, test.noFetch, test.sync, func(message string) {
				warnings = append(warnings, message)
			}, commands)
			if got.Failed != test.wantFailed || got.Stale != test.wantStale {
				t.Fatalf("unexpected inspection outcome: %+v", got)
			}
			kind := ""
			if got.Sync != nil {
				kind = got.Sync.Kind
			}
			if kind != test.wantSync {
				t.Fatalf("sync=%+v, want %q", got.Sync, test.wantSync)
			}
			if kind == "synced" && got.Sync.Pulled != 2 {
				t.Fatalf("expected two pulled commits, got %+v", got.Sync)
			}
			if test.wantSync == "" && merged {
				t.Fatal("ineligible repo must never be merged")
			}
			if fetched != (!test.noFetch || test.sync) {
				t.Fatal("fetch must respect local mode and sync requirements")
			}
			if test.wantWarning != "" && !strings.Contains(strings.Join(warnings, "\n"), test.wantWarning) {
				t.Fatalf("expected warning %q, got %v", test.wantWarning, warnings)
			}
			if !test.wantFailed && test.wantSync == "synced" && (got.Status.Branch == nil || got.Status.Branch.Behind != 0) {
				t.Fatal("successful sync must return updated status")
			}
		})
	}
}

func TestRepoContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := RepoContext(ctx, discover.Repo{Path: t.TempDir(), DisplayName: "repo"}, false, true, nil)
	if !got.Failed || !got.Stale || got.Sync != nil {
		t.Fatalf("canceled inspection must fail without syncing: %+v", got)
	}
}

func TestRepoSyncOverridesSquashAndAdvancesHead(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	for _, mergeOptions := range []string{"", "--squash", "--squash --no-commit"} {
		t.Run(mergeOptions, func(t *testing.T) {
			root := t.TempDir()
			origin := filepath.Join(root, "origin")
			clone := filepath.Join(root, "clone")
			runGit := func(path string, args ...string) {
				t.Helper()
				if result := git.Run(path, args...); !result.OK {
					t.Fatalf("git %v: %s", args, result.Stderr)
				}
			}
			commit := func() {
				t.Helper()
				runGit(origin, "add", "README.md")
				runGit(origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "update")
			}
			runGit(root, "init", "-q", "-b", "main", origin)
			if err := os.WriteFile(filepath.Join(origin, "README.md"), []byte("initial\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			commit()
			runGit(root, "clone", "-q", origin, clone)
			if err := os.WriteFile(filepath.Join(origin, "README.md"), []byte("advanced\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			commit()
			if mergeOptions != "" {
				runGit(clone, "config", "branch.main.mergeoptions", mergeOptions)
			}
			got := RepoContext(context.Background(), discover.Repo{Path: clone, DisplayName: "clone"}, true, true, nil)
			head := git.ResolveCommitContext(context.Background(), clone, "HEAD")
			target := git.ResolveCommitContext(context.Background(), origin, "HEAD")
			if got.Failed || got.Stale || got.Sync == nil || got.Sync.Kind != "synced" || got.Sync.Pulled != 1 ||
				!head.OK || !target.OK || head.Stdout != target.Stdout || got.Status.Branch == nil || got.Status.Branch.Behind != 0 || len(got.Status.Items) != 0 {
				t.Fatalf("sync must fast-forward HEAD and return a clean result: %+v head=%+v target=%+v", got, head, target)
			}
		})
	}
}
