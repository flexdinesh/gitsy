package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestUpdateCancelsContextOnQuit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	model := newModel(ctx, cancel, []discover.Repo{testRepo("repo")}, false, true, false, nil, func(ctx context.Context, repo discover.Repo, noFetch bool, syncRepos bool, warn func(string)) ui.RepoResult {
		return ui.RepoResult{Repo: repo}
	})

	_, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if command == nil {
		t.Fatal("expected q to return a quit command")
	}
	if ctx.Err() == nil {
		t.Fatal("expected quit to cancel context")
	}
}

func TestNewModelLimitsActiveInspections(t *testing.T) {
	repos := []discover.Repo{}
	for index := 0; index < maxInspecting+2; index++ {
		repos = append(repos, testRepo("repo-"+string(rune('a'+index))))
	}

	model := newTestModel(repos, false)

	if model.active != maxInspecting {
		t.Fatalf("expected %d active inspections, got %d", maxInspecting, model.active)
	}
	if model.next != maxInspecting {
		t.Fatalf("expected next inspection index %d, got %d", maxInspecting, model.next)
	}
}

func TestUpdateStartsNextInspectionWhenOneCompletes(t *testing.T) {
	repos := []discover.Repo{}
	for index := 0; index < maxInspecting+1; index++ {
		repos = append(repos, testRepo("repo-"+string(rune('a'+index))))
	}
	model := newTestModel(repos, false)

	updated, command := model.Update(repoDoneMsg{
		index: 0,
		result: ui.RepoResult{
			Repo:   repos[0],
			Status: status.Parse("## main...origin/main\n"),
		},
	})
	got := updated.(Model)

	if command == nil {
		t.Fatal("expected next inspection command")
	}
	if got.active != maxInspecting {
		t.Fatalf("expected active inspections to stay at %d, got %d", maxInspecting, got.active)
	}
	if got.next != maxInspecting+1 {
		t.Fatalf("expected next inspection index %d, got %d", maxInspecting+1, got.next)
	}
}

func TestWindowSizeBoundsRepoViewport(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")}, false)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	got := updated.(Model)

	if got.repos.Width() <= 0 || got.repos.Width() > 80 {
		t.Fatalf("expected table width within terminal, got %d", got.repos.Width())
	}
	if got.repos.Height() <= 0 || got.repos.Height() >= 20 {
		t.Fatalf("expected table viewport height within terminal, got %d", got.repos.Height())
	}
}

func TestViewRendersContainerBorders(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, false)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})

	view := updated.(Model).View()

	if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Fatalf("expected title and table container borders, got %q", view)
	}
	if width := lipgloss.Width(updated.(Model).renderTitle(80)); width != 80 {
		t.Fatalf("expected title border to fit terminal width, got %d", width)
	}
	if !strings.Contains(view, "─") {
		t.Fatalf("expected table header border, got %q", view)
	}
}

func TestTableRowsShowSpinnerInLoadingStatusCell(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, false)
	model.spin.Spinner.Frames = []string{"."}

	rows := model.tableRows()

	if len(rows) != 1 {
		t.Fatalf("expected one table row, got %d", len(rows))
	}
	if rows[0][0] != "repo" || !strings.Contains(rows[0][1], ". fetching status...") {
		t.Fatalf("expected spinner in loading status cell, got %#v", rows[0])
	}
}

func TestTableRowsUseContinuationRowsForRepoStatus(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")}, true)
	model.results[0] = ui.RepoResult{
		Repo: testRepo("repo"),
		Status: status.Parse(strings.Join([]string{
			"## main",
			" M changed.go",
			"?? new.go",
		}, "\n")),
	}

	rows := model.tableRows()

	if len(rows) != 3 {
		t.Fatalf("expected branch plus two status rows, got %d rows: %#v", len(rows), rows)
	}
	if rows[0][0] != "repo" {
		t.Fatalf("expected repo name in first column, got %#v", rows[0])
	}
	if rows[1][0] != "" || rows[2][0] != "" {
		t.Fatalf("expected continuation rows to leave repo column empty, got %#v", rows)
	}
	for _, row := range rows {
		if strings.Contains(row[1], "\n") {
			t.Fatalf("expected no embedded newlines in bubbles table cells, got %#v", rows)
		}
	}
	if !strings.Contains(rows[1][1], "modified changed.go") || !strings.Contains(rows[2][1], "untracked new.go") {
		t.Fatalf("expected status entries in continuation rows, got %#v", rows)
	}
}

func newTestModel(repos []discover.Repo, showAll bool) Model {
	return newModel(context.Background(), nil, repos, showAll, true, false, nil, func(ctx context.Context, repo discover.Repo, noFetch bool, syncRepos bool, warn func(string)) ui.RepoResult {
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
