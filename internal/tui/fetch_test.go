package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
)

func TestModelSharesFetchAcrossInspectionBatchesAndFetchesFreshNextRun(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	root := t.TempDir()
	origin, clone := filepath.Join(root, "origin"), filepath.Join(root, "clone")
	runGit := func(path string, args ...string) {
		t.Helper()
		if result := git.Run(path, args...); !result.OK {
			t.Fatalf("git %v: %s", args, result.Stderr)
		}
	}
	commit := func() {
		t.Helper()
		runGit(origin, "-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "update")
	}
	runGit(root, "init", "-q", "-b", "main", origin)
	commit()
	runGit(root, "clone", "-q", origin, clone)
	repos := []discover.Repo{{Path: clone, DisplayName: "repo"}}
	for index := range maxInspecting {
		name := fmt.Sprintf("feature-%d", index)
		path := filepath.Join(root, name)
		runGit(clone, "worktree", "add", "-q", "-b", name, path)
		runGit(path, "branch", "--set-upstream-to=origin/main", name)
		repos = append(repos, discover.Repo{Path: path, DisplayName: "repo", Worktree: &git.Worktree{Path: path, MainPath: clone}})
	}
	if err := os.WriteFile(filepath.Join(repos[1].Path, "local.txt"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(root, "git.trace")
	t.Setenv("GIT_TRACE", trace)
	for invocation := 1; invocation <= 2; invocation++ {
		commit()
		model := NewModel(repos, nil)
		commands := []tea.Cmd{model.Init()}
		for len(commands) > 0 {
			command := commands[0]
			commands = commands[1:]
			switch message := command().(type) {
			case tea.BatchMsg:
				commands = append(commands, message...)
			case repoDoneMsg:
				updated, next := model.Update(message)
				model = updated.(Model)
				if next != nil {
					commands = append(commands, next)
				}
			}
		}
		for index, result := range model.results {
			if result.Loading || result.Failed || result.Stale || result.Status.Branch == nil {
				t.Fatalf("invocation %d: checkout %d must finish successfully: %+v", invocation, index, result)
			}
			if index == 1 {
				if result.Sync != nil || !result.Status.Changed || result.Status.Branch.Behind != invocation {
					t.Fatalf("dirty worktree must keep its independent sync decision: %+v", result)
				}
			} else if result.Sync == nil || result.Sync.Kind != "synced" || result.Sync.Pulled != 1 || result.Status.Branch.Behind != 0 {
				t.Fatalf("invocation %d: clean checkout %d must sync fresh upstream: %+v", invocation, index, result)
			}
		}
		data, err := os.ReadFile(trace)
		if err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(string(data), "built-in: git fetch --all"); count != invocation {
			t.Fatalf("invocation %d: expected one fresh fetch across both batches, cumulative count=%d", invocation, count)
		}
	}
}
