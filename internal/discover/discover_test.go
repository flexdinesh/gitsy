package discover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/flexdinesh/gitsy/internal/git"
)

func TestDiscoverRejectsInvalidRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	writeFile(t, file, "not a directory")
	for _, root := range []string{filepath.Join(dir, "missing"), file} {
		t.Run(filepath.Base(root), func(t *testing.T) {
			if _, err := Discover(Options{Cwd: root, MaxDepth: 3}); err == nil {
				t.Fatal("invalid scan root must fail")
			}
			if _, err := FindGitCandidates(root, 3, nil); err == nil {
				t.Fatal("invalid candidate root must fail")
			}
		})
	}
	repos, err := Discover(Options{Cwd: dir, MaxDepth: 3})
	if err != nil || len(repos) != 0 {
		t.Fatalf("valid empty directory must succeed: repos=%v err=%v", repos, err)
	}
}

func TestDiscoverPreservesRootError(t *testing.T) {
	_, err := Discover(Options{Cwd: filepath.Join(t.TempDir(), "missing"), MaxDepth: 3})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing-path error, got %v", err)
	}
}

func TestDiscoverCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DiscoverContext(ctx, Options{Cwd: t.TempDir(), MaxDepth: 3})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestFindGitCandidatesWarnsForUnreadableChildren(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	mkdir(t, blocked)
	mkdir(t, filepath.Join(dir, "repo", ".git"))
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o755) })
	if _, err := os.ReadDir(blocked); err == nil {
		t.Skip("filesystem does not enforce directory permissions")
	}
	warnings := []string{}
	candidates, err := findGitCandidates(context.Background(), dir, 3, nil, func(message string) {
		warnings = append(warnings, message)
	})
	if err != nil || len(candidates) != 1 || len(warnings) != 1 {
		t.Fatalf("expected readable sibling and skipped-child warning: candidates=%v warnings=%v err=%v", candidates, warnings, err)
	}
}

func TestFindGitCandidatesFindsGitDirectoriesAndSkipsRoot(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, ".git"))
	mkdir(t, filepath.Join(dir, "repo", ".git"))

	candidates, err := FindGitCandidates(dir, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, []string{"repo"}) {
		t.Fatalf("expected [repo], got %#v", got)
	}
}

func TestFindGitCandidatesFindsGitFiles(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "worktree"))
	writeFile(t, filepath.Join(dir, "worktree", ".git"), "gitdir: /tmp/example/.git/worktrees/worktree\n")

	candidates, err := FindGitCandidates(dir, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, []string{"worktree"}) {
		t.Fatalf("expected [worktree], got %#v", got)
	}
}

func TestFindGitCandidatesRespectsMaxDepth(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "a", "b", "repo", ".git"))
	mkdir(t, filepath.Join(dir, "a", "b", "c", "too-deep", ".git"))

	candidates, err := FindGitCandidates(dir, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join("a", "b", "repo")}
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestFindGitCandidatesIgnoresGeneratedDirectories(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "node_modules", "repo", ".git"))
	mkdir(t, filepath.Join(dir, "dist", "repo", ".git"))
	mkdir(t, filepath.Join(dir, "real", ".git"))

	candidates, err := FindGitCandidates(dir, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, []string{"real"}) {
		t.Fatalf("expected [real], got %#v", got)
	}
}

func TestParseWorktreePaths(t *testing.T) {
	got := git.ParseWorktreePaths("worktree /repo/main\x00HEAD abc123\x00branch refs/heads/main\x00\x00worktree /repo/feature\x00HEAD def456\x00")
	want := []string{"/repo/main", "/repo/feature"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestDisplayNameForPath(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "repo", path: filepath.Join(dir, "repo"), want: "repo"},
		{name: "nested", path: filepath.Join(dir, "org", "repo"), want: "repo"},
		{name: "external", path: filepath.Dir(dir), want: filepath.Base(filepath.Dir(dir))},
		{name: "current directory", path: ".", want: filepath.Base(cwd)},
		{name: "relative", path: filepath.Join("org", "repo"), want: "repo"},
		{name: "control characters", path: filepath.Join(dir, "linked\r\nwith\ttabs"), want: "linked\\r\\nwith\\ttabs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DisplayNameForPath(test.path); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func relativePaths(t *testing.T, root string, paths []string) []string {
	t.Helper()
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, relative)
	}
	return result
}
