package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/flexdinesh/gitsy/internal/git"
)

func TestFindGitCandidatesFindsGitDirectoriesAndSkipsRoot(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, ".git"))
	mkdir(t, filepath.Join(dir, "repo", ".git"))

	candidates := FindGitCandidates(dir, 3, nil)
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, []string{"repo"}) {
		t.Fatalf("expected [repo], got %#v", got)
	}
}

func TestFindGitCandidatesFindsGitFiles(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "worktree"))
	writeFile(t, filepath.Join(dir, "worktree", ".git"), "gitdir: /tmp/example/.git/worktrees/worktree\n")

	candidates := FindGitCandidates(dir, 3, nil)
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, []string{"worktree"}) {
		t.Fatalf("expected [worktree], got %#v", got)
	}
}

func TestFindGitCandidatesRespectsMaxDepth(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "a", "b", "repo", ".git"))
	mkdir(t, filepath.Join(dir, "a", "b", "c", "too-deep", ".git"))

	candidates := FindGitCandidates(dir, 3, nil)
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

	candidates := FindGitCandidates(dir, 3, nil)
	if got := relativePaths(t, dir, candidates); !reflect.DeepEqual(got, []string{"real"}) {
		t.Fatalf("expected [real], got %#v", got)
	}
}

func TestParseWorktreePaths(t *testing.T) {
	got := git.ParseWorktreePaths("worktree /repo/main\nHEAD abc123\nbranch refs/heads/main\n\nworktree /repo/feature\nHEAD def456\n")
	want := []string{"/repo/main", "/repo/feature"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestDisplayNameForPath(t *testing.T) {
	dir := t.TempDir()
	if got := DisplayNameForPath(dir, filepath.Join(dir, "repo")); got != "repo" {
		t.Fatalf("expected repo, got %s", got)
	}
	parent := filepath.Dir(dir)
	if got := DisplayNameForPath(dir, parent); got != filepath.Clean(parent) {
		t.Fatalf("expected %s, got %s", filepath.Clean(parent), got)
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
