package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/git"
)

func plainRemoteClone(t *testing.T, root, name string) (string, string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "missing-config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	remotes := t.TempDir()
	origin, clone := filepath.Join(remotes, "origin"), filepath.Join(root, name)
	plainGit(t, remotes, "init", "-q", "-b", "main", origin)
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "initial")
	plainGit(t, root, "clone", "-q", origin, clone)
	return origin, clone
}

func plainHead(t *testing.T, repo string) string {
	t.Helper()
	result := git.ResolveCommitContext(context.Background(), repo, "HEAD")
	if !result.OK {
		t.Fatalf("resolve HEAD: %s", result.Stderr)
	}
	return result.Stdout
}

func plainFetchCount(t *testing.T, trace string) int {
	t.Helper()
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "built-in: git fetch --all")
}

func TestPlainCommandSharesFetchWithinRunAndFetchesFreshNextRun(t *testing.T) {
	root := t.TempDir()
	origin, clone := plainRemoteClone(t, root, "clone")
	linked := filepath.Join(t.TempDir(), "linked")
	plainGit(t, clone, "worktree", "add", "-q", "-b", "feature", linked)
	plainGit(t, linked, "branch", "--set-upstream-to=origin/main", "feature")
	if err := os.WriteFile(filepath.Join(linked, "local.txt"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := plainHead(t, linked)
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	for invocation := 1; invocation <= 2; invocation++ {
		plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
		stdout, stderr, code := plainCommand(t, "--non-interactive", "--sync", "--dir", root, "--dir", root)
		if code != 0 || stderr != "" || !strings.Contains(stdout, "synced ↓1") || !strings.Contains(stdout, "1 untracked") || !strings.Contains(stdout, linked) {
			t.Fatalf("invocation %d must sync clean checkout and report dirty worktree: exit=%d stdout=%q stderr=%q", invocation, code, stdout, stderr)
		}
		if plainHead(t, clone) != plainHead(t, origin) || plainHead(t, linked) != before {
			t.Fatalf("invocation %d must advance clean HEAD to latest upstream and preserve dirty HEAD", invocation)
		}
		if count := plainFetchCount(t, trace); count != invocation {
			t.Fatalf("invocation %d: want one fresh fetch per run across worktrees and overlapping groups, total=%d", invocation, count)
		}
	}
}

func TestPlainCommandFetchesSeparateClonesOfSameRemote(t *testing.T) {
	root := t.TempDir()
	origin, first := plainRemoteClone(t, root, "first")
	second := filepath.Join(root, "second")
	plainGit(t, root, "clone", "-q", origin, second)
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	stdout, stderr, code := plainCommand(t, "--non-interactive", "--sync", "--dir", root)
	if code != 0 || stderr != "" || strings.Count(stdout, "synced ↓1") != 2 {
		t.Fatalf("independent clones must both sync: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if plainHead(t, first) != plainHead(t, origin) || plainHead(t, second) != plainHead(t, origin) {
		t.Fatal("each clone must fetch its own tracking refs before syncing")
	}
	if count := plainFetchCount(t, trace); count != 2 {
		t.Fatalf("same remote URL must not share fetch across independent clones: count=%d", count)
	}
}

func TestPlainCommandFailedSharedFetchSkipsSyncAgainstCachedUpstream(t *testing.T) {
	root := t.TempDir()
	origin, clone := plainRemoteClone(t, root, "clone")
	linked := filepath.Join(t.TempDir(), "linked")
	plainGit(t, clone, "worktree", "add", "-q", "-b", "feature", linked)
	plainGit(t, linked, "branch", "--set-upstream-to=origin/main", "feature")
	before := plainHead(t, clone)
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
	plainGit(t, clone, "fetch", "--all")
	plainGit(t, clone, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing-remote"))
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	stdout, stderr, code := plainCommand(t, "--non-interactive", "--sync", "--verbose", "--dir", root)
	if code != 1 || strings.Count(stdout, "sync skipped: fetch failed") != 2 || strings.Count(stdout, "⚠ stale") != 2 || !strings.Contains(stdout, "↓1") || !strings.Contains(stderr, "git operations failed") {
		t.Fatalf("failed fetch must report stale cached status and skip both syncs: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if plainHead(t, clone) != before || plainHead(t, linked) != before {
		t.Fatal("failed fetch must preserve both HEADs despite cached fast-forwardable upstream")
	}
	if count := plainFetchCount(t, trace); count != 1 {
		t.Fatalf("shared fetch failure must not trigger another attempt in this run: count=%d", count)
	}
}

func TestPlainCommandFetchesEachWorktreeWhenSharingIsUnsafe(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*testing.T, string, string, string)
		extra     bool
	}{
		{
			name: "worktree configuration",
			configure: func(t *testing.T, clone, linked, extra string) {
				plainGit(t, clone, "config", "extensions.worktreeConfig", "true")
				plainGit(t, linked, "config", "--worktree", "remote.extra.url", extra)
				plainGit(t, linked, "config", "--worktree", "remote.extra.fetch", "+refs/heads/*:refs/remotes/extra/*")
			},
			extra: true,
		},
		{
			name: "conditional include",
			configure: func(t *testing.T, clone, linked, extra string) {
				included := filepath.Join(t.TempDir(), "feature.config")
				plainGit(t, clone, "config", "--file", included, "remote.extra.url", extra)
				plainGit(t, clone, "config", "--file", included, "remote.extra.fetch", "+refs/heads/*:refs/remotes/extra/*")
				plainGit(t, clone, "config", "includeIf.onbranch:feature.path", included)
			},
			extra: true,
		},
		{
			name: "submodule metadata",
			configure: func(t *testing.T, clone, linked, extra string) {
				if err := os.WriteFile(filepath.Join(clone, ".gitmodules"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			_, clone := plainRemoteClone(t, root, "clone")
			linked := filepath.Join(t.TempDir(), "linked")
			plainGit(t, clone, "worktree", "add", "-q", "-b", "feature", linked)
			extra := filepath.Join(t.TempDir(), "extra")
			plainGit(t, root, "init", "-q", "-b", "main", extra)
			plainGit(t, extra, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "extra")
			test.configure(t, clone, linked, extra)
			trace := filepath.Join(t.TempDir(), "git.trace")
			t.Setenv("GIT_TRACE", trace)
			stdout, stderr, code := plainCommand(t, "--non-interactive", "--dir", root)
			if code != 0 || stderr != "" || !strings.Contains(stdout, clone) || !strings.Contains(stdout, linked) {
				t.Fatalf("checkout-specific fetching must succeed: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if count := plainFetchCount(t, trace); count != 2 {
				t.Fatalf("unsafe sharing must fetch both worktrees: count=%d", count)
			}
			if test.extra {
				fetched := git.ResolveCommitContext(context.Background(), linked, "refs/remotes/extra/main")
				if !fetched.OK || fetched.Stdout != plainHead(t, extra) {
					t.Fatal("linked worktree's additional remote must be fetched")
				}
			}
		})
	}
}

func TestPlainCommandUnrelatedRemoteFailureSkipsSyncAfterUpstreamFetch(t *testing.T) {
	root := t.TempDir()
	origin, clone := plainRemoteClone(t, root, "clone")
	linked := filepath.Join(t.TempDir(), "linked")
	plainGit(t, clone, "worktree", "add", "-q", "-b", "feature", linked)
	plainGit(t, linked, "branch", "--set-upstream-to=origin/main", "feature")
	before := plainHead(t, clone)
	plainGit(t, origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
	plainGit(t, clone, "remote", "add", "unrelated", filepath.Join(t.TempDir(), "missing-remote"))
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	stdout, stderr, code := plainCommand(t, "--non-interactive", "--sync", "--dir", root)
	if code != 1 || strings.Count(stdout, "sync skipped: fetch failed") != 2 || strings.Count(stdout, "⚠ stale") != 2 || !strings.Contains(stderr, "git operations failed") {
		t.Fatalf("any remote failure must skip both syncs: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	fetched := git.ResolveCommitContext(context.Background(), clone, "refs/remotes/origin/main")
	if !fetched.OK || fetched.Stdout != plainHead(t, origin) {
		t.Fatal("successful origin fetch must still update shared upstream refs")
	}
	if plainHead(t, clone) != before || plainHead(t, linked) != before {
		t.Fatal("unrelated remote failure must preserve both HEADs even with freshly fetched upstream")
	}
	if count := plainFetchCount(t, trace); count != 1 {
		t.Fatalf("partial fetch failure must be shared across worktrees: count=%d", count)
	}
}

func TestPlainCommandRelativeRemoteFetchesFromEachCheckout(t *testing.T) {
	parentA, parentB := t.TempDir(), t.TempDir()
	publisher, clone := plainRemoteClone(t, parentA, "clone")
	origin := filepath.Join(parentA, "origin")
	plainGit(t, parentA, "clone", "-q", "--bare", publisher, origin)
	linked := filepath.Join(parentB, "linked")
	plainGit(t, clone, "worktree", "add", "-q", "-b", "feature", linked)
	plainGit(t, linked, "branch", "--set-upstream-to=origin/main", "feature")
	plainGit(t, clone, "remote", "set-url", "origin", "../origin")
	before := plainHead(t, linked)
	plainGit(t, publisher, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
	plainGit(t, publisher, "push", "-q", origin, "main")
	trace := filepath.Join(t.TempDir(), "git.trace")
	t.Setenv("GIT_TRACE", trace)
	stdout, stderr, code := plainCommand(t, "--non-interactive", "--sync", "--dir", parentA)
	if code != 1 || strings.Count(stdout, "synced ↓1") != 1 || strings.Count(stdout, "sync skipped: fetch failed") != 1 || strings.Count(stdout, "⚠ stale") != 1 || !strings.Contains(stderr, "git operations failed") {
		t.Fatalf("relative URL must resolve independently for each checkout: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if plainHead(t, clone) != plainHead(t, publisher) || plainHead(t, linked) != before {
		t.Fatal("main checkout must sync from its existing relative remote; linked checkout with missing relative remote must stay untouched")
	}
	if count := plainFetchCount(t, trace); count != 2 {
		t.Fatalf("relative remote URL must prevent sharing across checkout directories: count=%d", count)
	}
}
