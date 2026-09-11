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
	model := newTestModel([]discover.Repo{testRepo("repo")})

	if len(model.results) != 1 {
		t.Fatalf("expected one repo result, got %d", len(model.results))
	}
	if !model.results[0].Loading {
		t.Fatal("expected repo to start loading")
	}
}

func TestUpdateReplacesLoadingRepoWhenInspectionCompletes(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})

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

func TestViewShowsCompletedCleanRepos(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	updated, _ := model.Update(repoDoneMsg{
		index: 0,
		result: ui.RepoResult{
			Repo:   testRepo("repo"),
			Status: status.Parse("## main...origin/main\n"),
		},
	})

	if rows := ui.BuildRows(updated.(Model).results); len(rows) == 0 {
		t.Fatal("expected clean completed repo to remain visible")
	}
}

func TestUpdateQuitsOnQAndCtrlC(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})

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
	model := newModel(ctx, cancel, []discover.Repo{testRepo("repo")}, true, false, nil, func(ctx context.Context, repo discover.Repo, noFetch bool, syncRepos bool, warn func(string)) ui.RepoResult {
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

	model := newTestModel(repos)

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
	model := newTestModel(repos)

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
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	got := updated.(Model)

	if got.repos.Width() <= 0 || got.repos.Width() > 80 {
		t.Fatalf("expected table width within terminal, got %d", got.repos.Width())
	}
	if got.repos.Height() <= 0 || got.repos.Height() >= 20 {
		t.Fatalf("expected table viewport height within terminal, got %d", got.repos.Height())
	}
}

func TestUpdateScrollsWithKeyboard(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b", "repo-c", "repo-d", "repo-e", "repo-f", "repo-g", "repo-h"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(Model)
	initialView := model.View()

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 1 || model.repos.Cursor() != 2 {
		t.Fatalf("expected down arrow to select repo 2 at row 2, got repo %d at row %d", model.selected+1, model.repos.Cursor())
	}
	if model.View() == initialView {
		t.Fatal("expected down arrow to produce visible feedback")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	if model.selected != 2 || model.repos.Cursor() != 4 {
		t.Fatalf("expected j to select repo 3 at row 4, got repo %d at row %d", model.selected+1, model.repos.Cursor())
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	model = updated.(Model)
	if model.selected != 0 {
		t.Fatalf("expected up arrow and k to return to repo 1, got %d", model.selected+1)
	}
}

func TestUpdateScrollsWithMouseWheel(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b", "repo-c", "repo-d"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	updated, _ = model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	model = updated.(Model)
	if model.selected != 2 || model.repos.Cursor() != 4 {
		t.Fatalf("expected wheel down to select repo 3 at row 4, got repo %d at row %d", model.selected+1, model.repos.Cursor())
	}

	updated, _ = model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	model = updated.(Model)
	if model.selected != 0 || model.repos.Cursor() != 0 {
		t.Fatalf("expected wheel up to return to repo 1, got repo %d at row %d", model.selected+1, model.repos.Cursor())
	}

	updated, _ = model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	if got := updated.(Model).selected; got != 0 {
		t.Fatalf("expected wheel up to stop at repo 1, got %d", got+1)
	}
}

func TestMouseWheelScrollsWithinRepoDetails(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	model.results[0] = ui.RepoResult{
		Repo: testRepo("repo"),
		Status: status.Parse(strings.Join([]string{
			"## main",
			" M one.go",
			" M two.go",
			" M three.go",
			" M four.go",
		}, "\n")),
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	updated, _ = model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	model = updated.(Model)

	if model.selected != 0 || model.repos.Cursor() != mouseWheelRows {
		t.Fatalf("expected wheel to reach repo detail row %d, got repo %d at row %d", mouseWheelRows, model.selected+1, model.repos.Cursor())
	}
	if view := model.View(); !strings.Contains(view, "three.go") {
		t.Fatalf("expected scrolled detail row, got %q", view)
	}
}

func TestUpdateIgnoresOtherMouseEvents(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b", "repo-c"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	for _, message := range []tea.MouseMsg{
		{Action: tea.MouseActionPress, Button: tea.MouseButtonLeft},
		{Action: tea.MouseActionRelease, Button: tea.MouseButtonWheelDown},
		{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelLeft},
	} {
		updated, _ = model.Update(message)
		model = updated.(Model)
	}

	if model.selected != 0 {
		t.Fatalf("expected other mouse events not to scroll, got repo %d", model.selected+1)
	}
}

func TestViewShowsScrollHintOnlyOnOverflow(t *testing.T) {
	const hint = "↑/↓ j/k"

	model := newTestModel([]discover.Repo{testRepo("repo")})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if view := updated.(Model).View(); strings.Contains(view, hint) {
		t.Fatalf("expected no scroll hint when rows fit, got %q", view)
	}

	model = newTestModel(testRepos("repo-a", "repo-b", "repo-c", "repo-d"))
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	view := updated.(Model).View()
	if !strings.Contains(view, hint) {
		t.Fatalf("expected scroll hint for overflow, got %q", view)
	}
	if height := lipgloss.Height(view); height > 10 {
		t.Fatalf("expected overflow view within terminal height, got %d", height)
	}

	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if view := updated.(Model).View(); !strings.Contains(view, "2/4") {
		t.Fatalf("expected selected repo position in scroll hint, got %q", view)
	}
}

func TestSelectionTracksRepoWhenEarlierRowsExpand(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	updated, _ = model.Update(repoDoneMsg{
		index: 0,
		result: ui.RepoResult{
			Repo: testRepo("repo-a"),
			Status: status.Parse(strings.Join([]string{
				"## main",
				" M changed.go",
				"?? new.go",
			}, "\n")),
		},
	})
	model = updated.(Model)

	if model.selected != 1 || model.repos.Cursor() != 4 {
		t.Fatalf("expected repo 2 to remain selected at new row 4, got repo %d at row %d", model.selected+1, model.repos.Cursor())
	}
}

func TestViewRendersContainerBorders(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
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
	model := newTestModel([]discover.Repo{testRepo("repo")})
	model.spin.Spinner.Frames = []string{"."}

	rows := model.tableRows()

	if len(rows) != 1 {
		t.Fatalf("expected one table row, got %d", len(rows))
	}
	if rows[0][0] != "›1" || rows[0][1] != "repo" || !strings.Contains(rows[0][2], ". fetching status...") {
		t.Fatalf("expected spinner in loading status cell, got %#v", rows[0])
	}
}

func TestTableRowsUseContinuationRowsForRepoStatus(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
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
	if rows[0][0] != "›1" || rows[0][1] != "repo" {
		t.Fatalf("expected repo name in first column, got %#v", rows[0])
	}
	if rows[1][0] != "" || rows[1][1] != "" || rows[2][0] != "" || rows[2][1] != "" {
		t.Fatalf("expected continuation rows to leave repo column empty, got %#v", rows)
	}
	for _, row := range rows {
		if strings.Contains(row[2], "\n") {
			t.Fatalf("expected no embedded newlines in bubbles table cells, got %#v", rows)
		}
	}
	if !strings.Contains(rows[1][2], "modified changed.go") || !strings.Contains(rows[2][2], "untracked new.go") {
		t.Fatalf("expected status entries in continuation rows, got %#v", rows)
	}
}

func TestTableRowsShowCleanRepoOnce(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	model.results[0] = ui.RepoResult{
		Repo:   testRepo("repo"),
		Status: status.Parse("## main...origin/main\n"),
	}

	rows := model.tableRows()

	if len(rows) != 1 {
		t.Fatalf("expected one clean repo table row, got %d rows: %#v", len(rows), rows)
	}
	if rows[0][0] != "›1" || rows[0][1] != "repo" || !strings.Contains(rows[0][2], "main ✓ clean") {
		t.Fatalf("expected single branch summary clean row, got %#v", rows)
	}
}

func TestTableRowsAddSpacingBetweenRepos(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")})

	rows := model.tableRows()

	if len(rows) != 3 {
		t.Fatalf("expected two repo groups separated by one blank row, got %d rows: %#v", len(rows), rows)
	}
	if rows[1][0] != "" || rows[1][1] != "" || rows[1][2] != "" {
		t.Fatalf("expected blank spacer row between repos, got %#v", rows)
	}
	if rows[2][0] != " 2" || rows[2][1] != "repo-b" {
		t.Fatalf("expected second repo after spacer row, got %#v", rows)
	}
}

func TestTableColumnsHaveSpacingAndFitWidth(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	model.updateTableWithSize(40, 10)

	columns := model.repos.Columns()
	if len(columns) != 3 {
		t.Fatalf("expected three columns, got %#v", columns)
	}
	if columns[0].Width+columns[1].Width+columnGap*len(columns) > model.repos.Width() {
		t.Fatalf("expected columns plus spacing to fit table width, got columns %#v and width %d", columns, model.repos.Width())
	}

	cell := lipgloss.NewStyle().Width(columns[1].Width).MaxWidth(columns[1].Width).Inline(true).Render("repo")
	if got := tableStyles().Cell.Render(cell); !strings.HasSuffix(got, strings.Repeat(" ", columnGap)) {
		t.Fatalf("expected repo cell to end with column spacing, got %q", got)
	}
}

func newTestModel(repos []discover.Repo) Model {
	return newModel(context.Background(), nil, repos, true, false, nil, func(ctx context.Context, repo discover.Repo, noFetch bool, syncRepos bool, warn func(string)) ui.RepoResult {
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

func testRepos(names ...string) []discover.Repo {
	repos := make([]discover.Repo, 0, len(names))
	for _, name := range names {
		repos = append(repos, testRepo(name))
	}
	return repos
}
