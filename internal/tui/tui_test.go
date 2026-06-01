package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/flexdinesh/gitsy/internal/ui"
)

func TestNewModelInitializesReposAsLoading(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, false)

	if len(model.results) != 1 {
		t.Fatalf("expected one repo result, got %d", len(model.results))
	}
	if !model.results[0].Loading {
		t.Fatal("expected repo to start loading")
	}
}

func TestUpdateReplacesLoadingRepoWhenInspectionCompletes(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, false)

	updated, _ := model.Update(repoDoneMsg{
		index: 0,
		result: ui.RepoResult{
			Repo:   testRepo("repo"),
			Status: status.Parse("## main...origin/main\n"),
		},
	})
	got := updated.(Model)

	if got.results[0].Loading {
		t.Fatal("expected completed repo not to be loading")
	}
	if got.done != 1 {
		t.Fatalf("expected one completed repo, got %d", got.done)
	}
}

func TestViewFiltersCompletedCleanReposWhenAllIsFalse(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, false)
	updated, _ := model.Update(repoDoneMsg{
		index: 0,
		result: ui.RepoResult{
			Repo:   testRepo("repo"),
			Status: status.Parse("## main...origin/main\n"),
		},
	})

	if rows := ui.BuildRows(updated.(Model).results, false); len(rows) != 0 {
		t.Fatalf("expected clean completed repo to be filtered from rows, got %#v", rows)
	}
}

func TestUpdateQuitsOnQAndCtrlC(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, false)

	_, qCommand := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if qCommand == nil {
		t.Fatal("expected q to return a quit command")
	}

	_, upperQCommand := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Q'}})
	if upperQCommand == nil {
		t.Fatal("expected Q to return a quit command")
	}

	_, ctrlCCommand := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if ctrlCCommand == nil {
		t.Fatal("expected ctrl+c to return a quit command")
	}
}

func TestWindowSizeBoundsRepoViewport(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")}, false)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	got := updated.(Model)

	if got.repos.Width <= 0 || got.repos.Width > 76 {
		t.Fatalf("expected viewport width within repo section, got %d", got.repos.Width)
	}
	if got.repos.Height <= 0 || got.repos.Height >= 20 {
		t.Fatalf("expected viewport height within terminal, got %d", got.repos.Height)
	}
}

func newTestModel(repos []discover.Repo, showAll bool) Model {
	return newModel(repos, showAll, true, false, nil, func(repo discover.Repo, noFetch bool, syncRepos bool, warn func(string)) ui.RepoResult {
		return ui.RepoResult{
			Repo:   repo,
			Status: status.Parse("## main...origin/main\n"),
		}
	})
}

func testRepo(name string) discover.Repo {
	return discover.Repo{
		Path:        "/tmp/" + name,
		RealPath:    "/tmp/" + name,
		DisplayName: name,
		Source:      discover.SourceScan,
	}
}
