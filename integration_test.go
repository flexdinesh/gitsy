package gitsy_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/args"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestGroupedDiscoveryPreservesRootsAndSharedWorktrees(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	root := filepath.Join(dir, "workspace")
	one, two, empty := filepath.Join(root, "one"), filepath.Join(root, "two"), filepath.Join(root, "empty")
	api, web := filepath.Join(one, "api"), filepath.Join(two, "web")
	linked := filepath.Join(dir, "outside", "linked")
	for _, path := range []string{api, web, empty} {
		mkdirAll(t, path)
	}
	for _, repo := range []string{api, web} {
		runGit(t, repo, "init", "-q", "-b", "main")
	}
	configureGitUser(t, api)
	runGit(t, api, "commit", "--allow-empty", "-qm", "initial")
	runGit(t, api, "worktree", "add", "-q", "-b", "linked", linked)
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(one, alias); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		dirs  []string
		roots []string
		paths [][]string
	}{
		{name: "current directory", roots: []string{root}, paths: [][]string{{api, linked, web}}},
		{name: "overlap", dirs: []string{".", "one"}, roots: []string{root, one}, paths: [][]string{{api, linked, web}, {api, linked}}},
		{name: "repeated aliases and empty", dirs: []string{two, alias, one, empty, one}, roots: []string{two, alias, one, empty, one}, paths: [][]string{{web}, {api, linked}, {api, linked}, {}, {api, linked}}},
		{name: "linked worktree scanned first", dirs: []string{filepath.Dir(linked), one}, roots: []string{filepath.Dir(linked), one}, paths: [][]string{{api, linked}, {api, linked}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace, err := discover.DiscoverGroupedContext(context.Background(), discover.Options{Cwd: root, Dirs: test.dirs, MaxDepth: 3})
			if err != nil {
				t.Fatal(err)
			}
			if len(workspace.Groups) != len(test.roots) {
				t.Fatalf("expected one group per root: %+v", workspace.Groups)
			}
			seen := map[string]bool{}
			for _, repo := range workspace.Repos {
				if repo.RealPath == linked && (repo.Worktree == nil || repo.Worktree.MainPath != api || repo.Worktree.Branch != "linked") {
					t.Fatalf("linked worktree identity lost: %+v", repo)
				}
				if repo.RealPath == api && repo.Worktree != nil {
					t.Fatal("main repository must not be marked linked")
				}
				if seen[repo.RealPath] {
					t.Fatalf("inspection list duplicates %s", repo.RealPath)
				}
				seen[repo.RealPath] = true
			}
			for index, group := range workspace.Groups {
				if group.Path != test.roots[index] {
					t.Fatalf("group order/root changed: got %s want %s", group.Path, test.roots[index])
				}
				paths := []string{}
				for _, repoIndex := range group.RepoIndexes {
					paths = append(paths, workspace.Repos[repoIndex].RealPath)
				}
				if !reflect.DeepEqual(paths, test.paths[index]) {
					t.Fatalf("group %s: got %v want %v", group.Path, paths, test.paths[index])
				}
			}
		})
	}
}

func TestDiscoversMultipleDirectoriesAndExternalWorktrees(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	one := filepath.Join(home, "org-one")
	two := filepath.Join(home, "org-two")
	repo := filepath.Join(one, "project")
	deep := filepath.Join(one, "nested", "deep")
	worktree := filepath.Join(dir, "outside", "linked")
	for _, path := range []string{one, repo, filepath.Join(two, "project"), deep, filepath.Join(home, "unselected")} {
		mkdirAll(t, path)
		runGit(t, path, "init", "-q", "-b", "main")
	}
	configureGitUser(t, repo)
	runGit(t, repo, "commit", "--allow-empty", "-qm", "initial")
	runGit(t, repo, "worktree", "add", "-q", "-b", "linked", worktree)
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(one, alias); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		argv []string
		deep bool
	}{
		{name: "absolute", argv: []string{"--dir", one, "--dir", two, "--max-depth", "1"}},
		{name: "relative", argv: []string{"--dir", "org-one", "--dir=org-two", "--max-depth=1"}},
		{name: "reversed", argv: []string{"--dir", two, "--dir", one, "--max-depth", "1"}},
		{name: "repeated", argv: []string{"--dir", one, "--dir", two, "--dir", one, "--max-depth", "1"}},
		{name: "overlapping", argv: []string{"--dir", one, "--dir", two, "--dir", repo, "--max-depth", "1"}},
		{name: "symlink", argv: []string{"--dir", one, "--dir", two, "--dir", alias, "--max-depth", "1"}},
		{name: "deeper", argv: []string{"--dir", one, "--dir", two, "--max-depth", "2"}, deep: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := args.Parse(test.argv, home)
			if !parsed.OK {
				t.Fatal(parsed.Err)
			}
			repos, err := discover.Discover(discover.Options{Cwd: home, Dirs: parsed.Options.Dirs, MaxDepth: parsed.Options.MaxDepth})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"linked", "project", "project"}
			wantPaths := []string{worktree, repo, filepath.Join(two, "project")}
			if test.deep {
				want = append(want, "deep")
				wantPaths = append(wantPaths, deep)
			}
			sort.Strings(want)
			sort.Strings(wantPaths)
			names := []string{}
			paths := []string{}
			for _, found := range repos {
				names = append(names, found.DisplayName)
				paths = append(paths, found.RealPath)
				if found.Path == worktree && found.Source != discover.SourceWorktree {
					t.Fatalf("external worktree must come from worktree discovery: %+v", found)
				}
			}
			if !reflect.DeepEqual(names, want) {
				t.Fatalf("expected selected repos and external worktree %v, got %v", want, names)
			}
			sort.Strings(paths)
			if !reflect.DeepEqual(paths, wantPaths) {
				t.Fatalf("expected selected paths %v, got %v", wantPaths, paths)
			}
			for index := 1; index < len(repos); index++ {
				previous, current := repos[index-1], repos[index]
				if previous.DisplayName == current.DisplayName && previous.Path > current.Path {
					t.Fatalf("duplicate names must be ordered by path: %v", repos)
				}
			}
		})
	}
}

func TestDiscoverUsesSelectedDirectories(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	cwd := filepath.Join(dir, "cwd")
	one := filepath.Join(dir, "one")
	two := filepath.Join(dir, "two")
	defaultRepo := filepath.Join(cwd, "default-repo")
	firstRepo := filepath.Join(one, "first-repo")
	secondRepo := filepath.Join(two, "second-repo")
	for _, path := range []string{defaultRepo, firstRepo, secondRepo} {
		mkdirAll(t, path)
		runGit(t, path, "init", "-q", "-b", "main")
	}
	tests := []struct {
		name string
		cwd  string
		argv []string
		want []string
	}{
		{name: "default", cwd: cwd, want: []string{defaultRepo}},
		{name: "single directory", cwd: cwd, argv: []string{"--dir", one}, want: []string{firstRepo}},
		{name: "multiple directories", cwd: cwd, argv: []string{"--dir", one, "--dir", two}, want: []string{firstRepo, secondRepo}},
		{name: "current repo inside selected directory", cwd: firstRepo, argv: []string{"--dir", one, "--dir", two}, want: []string{firstRepo, secondRepo}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := args.Parse(test.argv, test.cwd)
			if !parsed.OK {
				t.Fatal(parsed.Err)
			}
			repos, err := discover.Discover(discover.Options{Cwd: test.cwd, Dirs: parsed.Options.Dirs, MaxDepth: parsed.Options.MaxDepth})
			if err != nil {
				t.Fatal(err)
			}
			paths := []string{}
			for _, repo := range repos {
				paths = append(paths, repo.Path)
				if repo.DisplayName != filepath.Base(repo.Path) {
					t.Fatalf("expected repo name %q, got %q", filepath.Base(repo.Path), repo.DisplayName)
				}
			}
			if !reflect.DeepEqual(paths, test.want) {
				t.Fatalf("expected selected repos %v, got %v", test.want, paths)
			}
		})
	}
}

func TestDiscoverNamesReposFromOrigin(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "servediff", "main")
	other := filepath.Join(dir, "other", "main")
	worktree := filepath.Join(t.TempDir(), "feature")
	for _, path := range []string{repo, other} {
		mkdirAll(t, path)
		runGit(t, path, "init", "-q", "-b", "main")
	}
	runGit(t, repo, "remote", "add", "origin", "git@github.com:flexdinesh/servediff.git")
	runGit(t, other, "remote", "add", "origin", "https://example.com/org/another-repo.git")
	configureGitUser(t, repo)
	runGit(t, repo, "commit", "--allow-empty", "-qm", "initial")
	runGit(t, repo, "worktree", "add", "-q", "-b", "feature", worktree)

	for _, root := range []string{dir, filepath.Dir(worktree)} {
		repos, err := discover.Discover(discover.Options{Cwd: root, MaxDepth: 3})
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{repo: "servediff", worktree: "servediff"}
		if root == dir {
			want[other] = "another-repo"
		}
		if len(repos) != len(want) {
			t.Fatalf("expected %d repos, got %v", len(want), repos)
		}
		for index, found := range repos {
			if found.DisplayName != want[found.Path] {
				t.Fatalf("expected label %q for %s, got %q", want[found.Path], found.Path, found.DisplayName)
			}
			if index > 0 {
				previous := repos[index-1]
				if previous.DisplayName > found.DisplayName || (previous.DisplayName == found.DisplayName && previous.Path > found.Path) {
					t.Fatalf("repos must sort by name, then path: %v", repos)
				}
			}
		}
	}
}

func TestDiscoverFallsBackForUnusableOrigin(t *testing.T) {
	requireGit(t)
	for _, origin := range []string{"", "https://example.com", "https://example.com/%zz"} {
		t.Run(origin, func(t *testing.T) {
			dir := t.TempDir()
			repo := filepath.Join(dir, "local-repo")
			mkdirAll(t, repo)
			runGit(t, repo, "init", "-q", "-b", "main")
			runGit(t, repo, "config", "remote.origin.url", origin)
			repos, err := discover.Discover(discover.Options{Cwd: dir, MaxDepth: 1})
			if err != nil {
				t.Fatal(err)
			}
			if len(repos) != 1 || repos[0].DisplayName != "local-repo" {
				t.Fatalf("expected directory name fallback, got %v", repos)
			}
		})
	}
}

func TestDiscoversCleanChildReposAndFiltersStatusByChangedFlag(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	mkdirAll(t, repo)
	runGit(t, repo, "init")

	repos, err := discover.Discover(discover.Options{Cwd: dir, MaxDepth: 3})
	if err != nil {
		t.Fatal(err)
	}
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

	repos, err := discover.Discover(discover.Options{Cwd: dir, MaxDepth: 3})
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, repo := range repos {
		names = append(names, repo.DisplayName)
	}
	if !reflect.DeepEqual(names, []string{"linked-worktree", "repo"}) {
		t.Fatalf("expected linked-worktree and repo, got %#v", names)
	}
}

func TestDiscoversExternalWorktreesWithSpecialPaths(t *testing.T) {
	requireGit(t)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	for _, name := range []string{"linked\nrepo", "linked\r\nrepo", "linked\n", "linked "} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			repo := filepath.Join(dir, "repo")
			worktree := filepath.Join(t.TempDir(), name)
			runGit(t, dir, "init", "-q", "-b", "main", repo)
			configureGitUser(t, repo)
			runGit(t, repo, "commit", "--allow-empty", "-qm", "initial")
			runGit(t, repo, "worktree", "add", "-q", "-b", "linked", worktree)
			repos, err := discover.Discover(discover.Options{Cwd: dir, MaxDepth: 3})
			if err != nil || len(repos) != 2 {
				t.Fatalf("expected main and external worktree: repos=%v err=%v", repos, err)
			}
			found := false
			for _, discovered := range repos {
				if discovered.Path == worktree {
					found = true
				}
			}
			if !found {
				t.Fatalf("worktree path must be preserved: %q", worktree)
			}
		})
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
