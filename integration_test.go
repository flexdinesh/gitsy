package gitsy_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestDiscoversCleanChildReposAndFiltersStatusByChangedFlag(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	mkdirAll(t, repo)
	runGit(t, repo, "init")

	repos := discover.Discover(discover.Options{Cwd: dir, MaxDepth: 3})
	if len(repos) != 1 {
		t.Fatalf("expected one repo, got %d", len(repos))
	}
	if repos[0].DisplayName != "repo" {
		t.Fatalf("expected display name repo, got %s", repos[0].DisplayName)
	}

	cleanStatus := status.Parse(git.ShortStatus(repo).Stdout)
	if cleanStatus.Changed {
		t.Fatal("expected clean repo to be unchanged")
	}

	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	dirtyStatus := status.Parse(git.ShortStatus(repo).Stdout)
	if !dirtyStatus.Changed {
		t.Fatal("expected dirty repo to be changed")
	}
}

func TestDiscoversLinkedWorktreesFromDiscoveredRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	worktree := filepath.Join(dir, "linked-worktree")
	mkdirAll(t, repo)
	runGit(t, repo, "init")
	configureGitUser(t, repo)
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	runGit(t, repo, "worktree", "add", "-b", "feature", worktree)

	repos := discover.Discover(discover.Options{Cwd: dir, MaxDepth: 3})
	names := []string{}
	for _, repo := range repos {
		names = append(names, repo.DisplayName)
	}
	if !reflect.DeepEqual(names, []string{"linked-worktree", "repo"}) {
		t.Fatalf("expected linked-worktree and repo, got %#v", names)
	}
}

func TestCLINoFetchRendersRepoStatus(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	mkdirAll(t, repo)
	runGit(t, repo, "init")
	writeFile(t, filepath.Join(repo, "README.md"), "hello\n")

	cmd := exec.Command("go", "run", filepath.Join(projectRoot(t), "cmd", "gitsy"), "--no-fetch")
	cmd.Env = append(os.Environ(), "FORCE_COLOR=0", "CI=true")
	cmd.Dir = dir
	result, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, result)
	}
	if !strings.Contains(string(result), "repo") {
		t.Fatalf("expected output to include repo name: %s", result)
	}
}

func TestSyncSafelyFastForwardsStrictlyBehindClone(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	origin := filepath.Join(dir, "origin")
	clone := filepath.Join(dir, "clone")
	mkdirAll(t, origin)
	runGit(t, origin, "init")
	configureGitUser(t, origin)
	writeFile(t, filepath.Join(origin, "README.md"), "one\n")
	runGit(t, origin, "add", "README.md")
	runGit(t, origin, "commit", "-m", "first")

	runGit(t, dir, "clone", origin, clone)
	configureGitUser(t, clone)

	writeFile(t, filepath.Join(origin, "README.md"), "one\ntwo\n")
	runGit(t, origin, "add", "README.md")
	runGit(t, origin, "commit", "-m", "second")

	runGit(t, clone, "fetch")
	before := status.Parse(git.ShortStatus(clone).Stdout)
	if before.Branch == nil || before.Branch.Behind != 1 || before.Branch.Ahead != 0 || !status.CanFastForward(before) {
		t.Fatalf("expected clone to be fast-forwardable: %+v", before.Branch)
	}

	ff := git.FastForward(clone)
	if !ff.OK {
		t.Fatalf("expected fast-forward to succeed: %s", ff.Stderr)
	}

	after := status.Parse(git.ShortStatus(clone).Stdout)
	if after.Branch == nil || after.Branch.Behind != 0 || status.CanFastForward(after) {
		t.Fatalf("expected clone to be up to date: %+v", after.Branch)
	}
}

func TestSyncRefusesDivergedClone(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	origin := filepath.Join(dir, "origin")
	clone := filepath.Join(dir, "clone")
	mkdirAll(t, origin)
	runGit(t, origin, "init")
	configureGitUser(t, origin)
	writeFile(t, filepath.Join(origin, "README.md"), "one\n")
	runGit(t, origin, "add", "README.md")
	runGit(t, origin, "commit", "-m", "first")

	runGit(t, dir, "clone", origin, clone)
	configureGitUser(t, clone)

	writeFile(t, filepath.Join(origin, "README.md"), "one\norigin\n")
	runGit(t, origin, "add", "README.md")
	runGit(t, origin, "commit", "-m", "origin change")

	writeFile(t, filepath.Join(clone, "LOCAL.md"), "local\n")
	runGit(t, clone, "add", "LOCAL.md")
	runGit(t, clone, "commit", "-m", "local change")

	runGit(t, clone, "fetch")
	parsed := status.Parse(git.ShortStatus(clone).Stdout)
	if parsed.Branch == nil || parsed.Branch.Ahead != 1 || parsed.Branch.Behind != 1 || status.CanFastForward(parsed) {
		t.Fatalf("expected diverged clone not to be fast-forwardable: %+v", parsed.Branch)
	}

	ff := git.FastForward(clone)
	if ff.OK {
		t.Fatal("expected fast-forward to fail")
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	result := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	output, err := result.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s failed: %v\n%s", cwd, strings.Join(args, " "), err, output)
	}
}

func configureGitUser(t *testing.T, repo string) {
	t.Helper()
	runGit(t, repo, "config", "user.email", "gitsy@example.com")
	runGit(t, repo, "config", "user.name", "Gitsy Test")
}

func mkdirAll(t *testing.T, path string) {
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

func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
