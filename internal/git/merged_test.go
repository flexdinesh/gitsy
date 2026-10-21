package git

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type mergedFixture struct {
	t        *testing.T
	upstream string
	repo     string
	worktree string
}

func newMergedFixture(t *testing.T) mergedFixture {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	for _, name := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(name, "Test")
	}
	for _, name := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(name, "test@example.com")
	}
	root := t.TempDir()
	fixture := mergedFixture{
		t:        t,
		upstream: filepath.Join(root, "upstream"),
		repo:     filepath.Join(root, "repo"),
		worktree: filepath.Join(root, "feature"),
	}
	fixture.git(root, "init", "-q", "-b", "main", fixture.upstream)
	fixture.commit(fixture.upstream, "a.txt", "one\ntwo\nthree\n", "base")
	fixture.git(root, "clone", "-q", fixture.upstream, fixture.repo)
	return fixture
}

func (fixture mergedFixture) git(dir string, args ...string) string {
	fixture.t.Helper()
	result := Run(dir, args...)
	if !result.OK {
		fixture.t.Fatalf("git %v: %s", args, result.Stderr)
	}
	return result.Stdout
}

func (fixture mergedFixture) commit(dir, file, content, message string) {
	fixture.t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
		fixture.t.Fatal(err)
	}
	fixture.git(dir, "add", file)
	fixture.git(dir, "commit", "-qm", message)
}

// feature adds a linked worktree on branch feat with two commits.
func (fixture mergedFixture) feature() {
	fixture.t.Helper()
	fixture.git(fixture.repo, "worktree", "add", "-q", "-b", "feat", fixture.worktree)
	fixture.commit(fixture.worktree, "a.txt", "one\nTWO\nthree\n", "change two")
	fixture.commit(fixture.worktree, "b.txt", "new\n", "add b")
}

// upstreamMerge lands feat on upstream main with the given commands, then fetches.
func (fixture mergedFixture) upstreamMerge(commands ...[]string) {
	fixture.t.Helper()
	fixture.git(fixture.upstream, "fetch", "-q", fixture.repo, "feat:feat")
	for _, args := range commands {
		fixture.git(fixture.upstream, args...)
	}
	fixture.git(fixture.repo, "fetch", "-q")
}

func (fixture mergedFixture) merged() string {
	fixture.t.Helper()
	into, result := MergedContext(context.Background(), fixture.worktree, BaseRefsContext(context.Background(), fixture.worktree, ""))
	if !result.OK {
		fixture.t.Fatalf("merged check failed: %s", result.Stderr)
	}
	return into
}

func TestMergedContextDetectsMergeWorkflows(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(mergedFixture)
		want  string
	}{
		{name: "merge commit", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.feature()
			fixture.upstreamMerge([]string{"merge", "-q", "--no-ff", "-m", "merge", "feat"})
		}},
		{name: "fast-forward", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.feature()
			fixture.upstreamMerge([]string{"merge", "-q", "--ff-only", "feat"})
		}},
		{name: "rebase merge", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.feature()
			fixture.commit(fixture.upstream, "c.txt", "other\n", "other work")
			fixture.upstreamMerge([]string{"cherry-pick", "main..feat"})
		}},
		{name: "squash merge", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.feature()
			fixture.commit(fixture.upstream, "c.txt", "other\n", "other work")
			fixture.upstreamMerge([]string{"merge", "-q", "--squash", "feat"}, []string{"commit", "-qm", "squash"})
		}},
		{name: "squash edited while merging", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.feature()
			fixture.upstreamMerge([]string{"merge", "-q", "--squash", "feat"})
			fixture.commit(fixture.upstream, "d.txt", "review fix\n", "squash with review fix")
			fixture.git(fixture.repo, "fetch", "-q")
		}},
		{name: "unmerged", setup: func(fixture mergedFixture) {
			fixture.feature()
		}},
		{name: "fresh branch", setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "worktree", "add", "-q", "-b", "fresh", fixture.worktree)
		}},
		{name: "fresh branch without reflog", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "worktree", "add", "-q", "-b", "fresh", fixture.worktree)
			if err := os.Remove(filepath.Join(fixture.repo, ".git", "logs", "refs", "heads", "fresh")); err != nil {
				fixture.t.Fatal(err)
			}
		}},
		{name: "detached at merged commit", want: "origin/main", setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "worktree", "add", "-q", "--detach", fixture.worktree, "origin/main")
		}},
		{name: "detached with new commit", setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "worktree", "add", "-q", "--detach", fixture.worktree, "origin/main")
			fixture.commit(fixture.worktree, "b.txt", "new\n", "add b")
		}},
		{name: "worktree on base branch", setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "switch", "-q", "-c", "other")
			fixture.git(fixture.repo, "worktree", "add", "-q", fixture.worktree, "main")
		}},
		{name: "local-only merge", want: "main", setup: func(fixture mergedFixture) {
			fixture.feature()
			fixture.git(fixture.repo, "merge", "-q", "--no-ff", "-m", "merge", "feat")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newMergedFixture(t)
			test.setup(fixture)
			if got := fixture.merged(); got != test.want {
				t.Fatalf("merged into %q, want %q", got, test.want)
			}
		})
	}
}

func TestBaseRefsContextResolvesDefaultBranch(t *testing.T) {
	for _, test := range []struct {
		name     string
		setup    func(mergedFixture)
		override string
		want     []string
	}{
		{name: "remote HEAD", want: []string{"refs/remotes/origin/main", "refs/heads/main"}},
		{name: "config", want: []string{"refs/remotes/origin/develop", "refs/heads/develop"}, setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "config", "gitsy.base", "develop")
		}},
		{name: "override beats config", override: "origin/develop", want: []string{"refs/remotes/origin/develop", "refs/heads/develop"}, setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "config", "gitsy.base", "main")
		}},
		{name: "local-only override", override: "trunk", want: []string{"refs/heads/trunk"}, setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "branch", "trunk")
		}},
		{name: "missing remote HEAD", want: []string{"refs/remotes/origin/main", "refs/heads/main"}, setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "remote", "set-head", "origin", "-d")
		}},
		{name: "remote HEAD names another branch", want: []string{"refs/remotes/origin/develop", "refs/heads/develop"}, setup: func(fixture mergedFixture) {
			fixture.git(fixture.repo, "remote", "set-head", "origin", "develop")
		}},
		{name: "missing override", override: "nope"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newMergedFixture(t)
			fixture.git(fixture.upstream, "branch", "develop")
			fixture.git(fixture.repo, "fetch", "-q")
			fixture.git(fixture.repo, "branch", "develop", "origin/develop")
			if test.setup != nil {
				test.setup(fixture)
			}
			if got := BaseRefsContext(context.Background(), fixture.repo, test.override); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestSquashDetectionPaths(t *testing.T) {
	for _, test := range []struct {
		name        string
		edit        bool
		wantSquash  bool
		wantContent bool
	}{
		{name: "squash matches branch patch", wantSquash: true, wantContent: true},
		{name: "squash edited while merging", edit: true, wantContent: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newMergedFixture(t)
			fixture.feature()
			fixture.commit(fixture.upstream, "c.txt", "other\n", "later work")
			fixture.upstreamMerge([]string{"merge", "-q", "--squash", "feat"})
			if test.edit {
				fixture.commit(fixture.upstream, "d.txt", "review fix\n", "squash with review fix")
			} else {
				fixture.git(fixture.upstream, "commit", "-qm", "squash")
			}
			fixture.git(fixture.repo, "fetch", "-q")
			ctx := context.Background()
			squashed, result := squashMerged(ctx, fixture.worktree, "refs/remotes/origin/main")
			if !result.OK || squashed != test.wantSquash {
				t.Fatalf("squash patch id matched %t (%s), want %t", squashed, result.Stderr, test.wantSquash)
			}
			content, result := contentMerged(ctx, fixture.worktree, "refs/remotes/origin/main")
			if !result.OK || content != test.wantContent {
				t.Fatalf("content merged %t (%s), want %t", content, result.Stderr, test.wantContent)
			}
		})
	}
}
