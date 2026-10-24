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
		{name: "sync fetch failure", sync: true, before: behind, fetchFails: true, wantStale: true, wantWarning: "Fetch failed"},
		{name: "clean sync fetch failure", sync: true, before: clean, fetchFails: true, wantStale: true, wantWarning: "Fetch failed"},
		{name: "sync fetch and status failure", sync: true, fetchFails: true, statusFails: true, wantFailed: true, wantStale: true, wantWarning: "Fetch failed"},
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
					if test.fetchFails {
						t.Fatal("failed fetch must not resolve commits for sync")
					}
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
			if test.fetchFails && merged {
				t.Fatal("failed fetch must skip sync")
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

func TestFetchReasonClassifiesErrors(t *testing.T) {
	for _, test := range []struct {
		stderr string
		want   string
	}{
		{"git fetch timed out", "fetch timed out"},
		{"fatal: Authentication failed for 'https://example.com/org/repo.git/'", "remote auth needed"},
		{"git@example.com: Permission denied (publickey).\r\nfatal: Could not read from remote repository.", "remote auth needed"},
		{"fatal: could not read Username for 'https://example.com': terminal prompts disabled", "remote auth needed"},
		{"fatal: unable to access 'https://example.com/': The requested URL returned error: 403", "remote auth needed"},
		{"remote: Repository not found.\nfatal: repository 'https://example.com/x.git/' not found", "remote repo not found"},
		{"fatal: '/missing' does not appear to be a git repository\nfatal: Could not read from remote repository.", "remote repo not found"},
		{"fatal: unable to access 'https://example.com/': Could not resolve host: example.com", "remote unreachable"},
		{"ssh: connect to host example.com port 22: Connection timed out\nfatal: Could not read from remote repository.", "remote unreachable"},
		{"ssh: connect to host example.com port 22: Connection refused", "remote unreachable"},
		{"context canceled", "fetch canceled"},
		{"", "fetch failed"},
		{"error: something unexpected", "fetch failed"},
	} {
		t.Run(test.want+"/"+test.stderr, func(t *testing.T) {
			if got := fetchReason(git.Result{Stderr: test.stderr, Status: 1}); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestStatusReasonKeepsShortFirstLine(t *testing.T) {
	for _, test := range []struct {
		result git.Result
		want   string
	}{
		{git.Result{Stderr: "git status timed out", Status: -1}, "status failed: timed out"},
		{git.Result{Stderr: "fatal: not a git repository (or any of the parent directories): .git\nmore", Status: 128}, "status failed: not a git repository (or any of the parent dire…"},
		{git.Result{Stderr: "\n  error: index file corrupt  \n", Status: 128}, "status failed: index file corrupt"},
		{git.Result{Status: 129}, "status failed: git exited 129"},
	} {
		if got := statusReason(test.result); got != test.want {
			t.Fatalf("got %q, want %q", got, test.want)
		}
	}
}

func TestRepoReasonsFollowPriority(t *testing.T) {
	failure := func(stderr string) git.Result { return git.Result{Stderr: stderr, Status: 1} }
	for _, test := range []struct {
		name                   string
		sync, fetch, statusErr bool
		merge                  bool
		want                   string
	}{
		{name: "clean"},
		{name: "fetch", fetch: true, want: "remote unreachable"},
		{name: "sync skipped keeps fetch reason", sync: true, fetch: true, want: "remote unreachable"},
		{name: "status beats fetch", fetch: true, statusErr: true, want: "status failed: index file corrupt"},
		{name: "sync failure", sync: true, merge: true, want: "sync failed: Not possible to fast-forward, aborting."},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands := gitCommands{
				fetch: func(context.Context, string, time.Duration) git.Result {
					if test.fetch {
						return failure("fatal: Could not resolve host: example.com")
					}
					return git.Result{OK: true}
				},
				status: func(context.Context, string) git.Result {
					if test.statusErr {
						return failure("fatal: index file corrupt")
					}
					return git.Result{OK: true, Stdout: "## main...origin/main [behind 1]\n"}
				},
				merge: func(context.Context, string, string) git.Result {
					return failure("fatal: Not possible to fast-forward, aborting.")
				},
				resolve: func(context.Context, string, string) git.Result {
					return git.Result{OK: true, Stdout: "commit\n"}
				},
			}
			got := repoContext(context.Background(), discover.Repo{Path: "/repo"}, false, test.sync, nil, commands)
			if got.Reason != test.want {
				t.Fatalf("reason = %q, want %q", got.Reason, test.want)
			}
		})
	}
}

func TestRepoReportsRemotes(t *testing.T) {
	for _, test := range []struct {
		name   string
		result git.Result
		unset  bool
		want   Remotes
		warned bool
	}{
		{name: "unset lookup", unset: true},
		{name: "no remotes", result: git.Result{Status: 1}, want: Remotes{Known: true}},
		{name: "origin", result: git.Result{OK: true, Stdout: "remote.origin.url\ngit@github.com:o/r.git\x00remote.fork.url\nhttps://example.com/r\x00"}, want: Remotes{Known: true, Count: 2, OriginURL: "git@github.com:o/r.git"}},
		{name: "no origin", result: git.Result{OK: true, Stdout: "remote.fork.url\nhttps://example.com/r\x00"}, want: Remotes{Known: true, Count: 1}},
		{name: "lookup failure", result: git.Result{Stderr: "fatal: bad config", Status: 128}, warned: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			commands := gitCommands{
				fetch:  func(context.Context, string, time.Duration) git.Result { return git.Result{OK: true} },
				status: func(context.Context, string) git.Result { return git.Result{OK: true, Stdout: "## main\n"} },
			}
			if !test.unset {
				commands.remotes = func(context.Context, string) git.Result { return test.result }
			}
			warnings := []string{}
			got := repoContext(context.Background(), discover.Repo{Path: "/repo", DisplayName: "repo"}, true, false, func(message string) {
				warnings = append(warnings, message)
			}, commands)
			if got.Remotes != test.want || (len(warnings) > 0) != test.warned {
				t.Fatalf("remotes=%+v warnings=%v, want %+v warned=%t", got.Remotes, warnings, test.want, test.warned)
			}
		})
	}
}
