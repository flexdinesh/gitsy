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
	"github.com/mattn/go-runewidth"
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
	if got.pending() != 0 {
		t.Fatalf("expected no pending repos, got %d", got.pending())
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
	if got.next != maxInspecting+1 {
		t.Fatalf("expected next inspection index %d, got %d", maxInspecting+1, got.next)
	}
}

func TestWindowSizeBoundsRepoViewport(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	got := updated.(Model)

	if got.tableWidth <= 0 || got.tableWidth > 80 {
		t.Fatalf("expected table width within terminal, got %d", got.tableWidth)
	}
	if got.capacity <= 0 || got.capacity >= 20 {
		t.Fatalf("expected table viewport height within terminal, got %d", got.capacity)
	}
}

func TestUpdateScrollsWithKeyboard(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b", "repo-c", "repo-d", "repo-e", "repo-f", "repo-g", "repo-h"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = updated.(Model)
	initialView := model.View()

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 1 {
		t.Fatalf("expected down arrow to select repo 2, got repo %d", model.selected+1)
	}
	if model.View() == initialView {
		t.Fatal("expected down arrow to produce visible feedback")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	model = updated.(Model)
	if model.selected != 2 {
		t.Fatalf("expected j to select repo 3, got repo %d", model.selected+1)
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
	model.expanded = true
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	model = updated.(Model)

	updated, _ = model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	model = updated.(Model)
	if model.selected != 2 || model.offset != 3 {
		t.Fatalf("expected wheel down to select repo 3 at offset 3, got repo %d at offset %d", model.selected+1, model.offset)
	}

	updated, _ = model.Update(tea.MouseMsg{
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelUp,
	})
	model = updated.(Model)
	if model.selected != 0 || model.offset != 0 {
		t.Fatalf("expected wheel up to return to repo 1, got repo %d at offset %d", model.selected+1, model.offset)
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
	model.expanded = true
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

	if model.selected != 0 || model.offset != 1 {
		t.Fatalf("expected wheel to pan to offset 1 on same repo, got repo %d at offset %d", model.selected+1, model.offset)
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

func TestViewSeparatesChromeFromTablePanel(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	view := updated.(Model).View()
	lines := strings.Split(view, "\n")

	if !strings.Contains(lines[0], "gitsy") || !strings.Contains(lines[1], "2 repos") || strings.TrimSpace(lines[2]) != "" {
		t.Fatalf("expected title, summary, then breathing room:\n%s", view)
	}
	if !strings.Contains(lines[len(lines)-1], "q quit") {
		t.Fatalf("expected footer anchored below ledger:\n%s", view)
	}
	if got := lipgloss.Width(view); got != 80 {
		t.Fatalf("expected chrome zones to fit terminal width, got %d", got)
	}
}

func TestHeaderSplitsTitleAndMeta(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	header := updated.(Model).renderHeader(80)

	if !strings.Contains(header, "gitsy") || !strings.Contains(header, "pending") {
		t.Fatalf("expected title left and mode right in header, got %q", header)
	}
	if strings.Contains(header, "• fetch") && strings.Index(header, "repos") > strings.Index(header, "• fetch") {
		t.Fatalf("expected meta without leading separator, got %q", header)
	}
}

func TestFooterSplitsPositionAndHints(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	footer := updated.(Model).renderFooter(80)

	if !strings.Contains(footer, "1/2") || !strings.Contains(footer, "q") {
		t.Fatalf("expected position left and hints right in footer, got %q", footer)
	}
}

func TestTableRowsFillPanelWidth(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")})
	model.updateTableWithSize(80, 10)

	if model.lineWidth() != model.tableWidth {
		t.Fatalf("expected rows to fill panel width %d, got %d", model.tableWidth, model.lineWidth())
	}
}

func TestLayoutSplitsTableAndInfoPanels(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	got := updated.(Model)

	if !got.infoShown {
		t.Fatal("expected context panel at width 120")
	}
	if got.tableOuter+panelGap+got.infoOuter != 120 {
		t.Fatalf("expected panels to fill width, got table %d + info %d", got.tableOuter, got.infoOuter)
	}
	if got.tableOuter >= 100 {
		t.Fatalf("expected compact table narrower than terminal, got %d", got.tableOuter)
	}
	if got.tableWidth != got.tableOuter-2-padX*2 {
		t.Fatalf("expected table content within panel, got %d in %d", got.tableWidth, got.tableOuter)
	}
}

func TestLayoutReservesWidthForRepositoryStatus(t *testing.T) {
	tableOuter, infoOuter, shown := layoutWidths(120)
	if !shown || infoOuter != 30 || tableOuter != 88 {
		t.Fatalf("expected 88/30 split at 120, got %d/%d shown=%v", tableOuter, infoOuter, shown)
	}
	if _, infoOuter, _ := layoutWidths(300); infoOuter != infoMaxOuter {
		t.Fatalf("expected capped info %d at 300, got %d", infoMaxOuter, infoOuter)
	}
	if _, _, shown := layoutWidths(tableMinOuter + panelGap + infoMinOuter); !shown {
		t.Fatal("expected split exactly at the combined minimums")
	}
	if _, _, shown := layoutWidths(tableMinOuter + panelGap + infoMinOuter - 1); shown {
		t.Fatal("expected full-width table one below the combined minimums")
	}
}

func TestLayoutDropsInfoPanelOnNarrowTerminals(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	got := updated.(Model)

	if got.infoShown {
		t.Fatal("expected no info panel at width 60")
	}
	if got.tableOuter != 60 {
		t.Fatalf("expected table to take full width, got %d", got.tableOuter)
	}
}

func TestInfoPanelFollowsSelection(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b", "repo-c", "repo-d", "repo-e", "repo-f", "repo-g", "repo-h"))
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	got := updated.(Model)

	view := got.View()
	for _, tip := range []string{"Selected repository", "/tmp/repo-a", "Status", "tab files"} {
		if !strings.Contains(view, tip) {
			t.Fatalf("expected info tip %q in view:\n%s", tip, view)
		}
	}
	updated, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	after := updated.(Model).View()
	if !strings.Contains(after, "/tmp/repo-b") || strings.Contains(after, "/tmp/repo-a") {
		t.Fatalf("expected context to follow selected repository:\n%s", after)
	}
}

func TestViewShowsFooterAlwaysAndPositionOnNavigate(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if view := updated.(Model).View(); !strings.Contains(view, "↑/↓ j/k") {
		t.Fatalf("expected footer hint always, got %q", view)
	}

	model = newTestModel([]discover.Repo{testRepo("repo")})
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	if view := updated.(Model).View(); !strings.Contains(view, "q quit") {
		t.Fatalf("expected slim footer with info panel visible, got %q", view)
	}

	model = newTestModel(testRepos("repo-a", "repo-b", "repo-c", "repo-d"))
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	view := updated.(Model).View()
	if !strings.Contains(view, "q quit") {
		t.Fatalf("expected footer hints, got %q", view)
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
	model.expanded = true
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

	if model.selected != 1 {
		t.Fatalf("expected repo 2 to remain selected, got repo %d", model.selected+1)
	}
	if first := firstRowOf(model.rows, 1); first != 4 {
		t.Fatalf("expected repo 2 at new row 4, got row %d", first)
	}
	if model.offset > 4 || 4 >= model.offset+model.capacity {
		t.Fatalf("expected selected repo visible at offset %d capacity %d", model.offset, model.capacity)
	}
}

func TestViewUsesOpenGutters(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})

	view := updated.(Model).View()

	if strings.Contains(view, "╭") || strings.Contains(view, "╰") {
		t.Fatalf("expected an open ledger without enclosing boxes, got %q", view)
	}
	if width := lipgloss.Width(view); width != 80 {
		t.Fatalf("expected outer container to fit terminal width, got %d", width)
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
	if rows[0].number != "1" || !rows[0].marker || rows[0].repo != "repo" || !strings.Contains(rows[0].status, "fetching status…") {
		t.Fatalf("expected spinner in loading status cell, got %#v", rows[0])
	}
}

func TestTableRowsUseContinuationRowsForRepoStatus(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	model.expanded = true
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
	if rows[0].number != "1" || !rows[0].marker || rows[0].repo != "repo" {
		t.Fatalf("expected repo name in first column, got %#v", rows[0])
	}
	if rows[1].number != "" || rows[1].repo != "" || rows[2].number != "" || rows[2].repo != "" {
		t.Fatalf("expected continuation rows to leave repo column empty, got %#v", rows)
	}
	for _, row := range rows {
		if strings.Contains(row.status, "\n") {
			t.Fatalf("expected no embedded newlines in table cells, got %#v", rows)
		}
	}
	if !strings.Contains(rows[1].status, "modified changed.go") || !strings.Contains(rows[2].status, "untracked new.go") {
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
	if rows[0].number != "1" || !rows[0].marker || rows[0].repo != "repo" || !strings.Contains(rows[0].status, "main ✓ clean") {
		t.Fatalf("expected single branch summary clean row, got %#v", rows)
	}
}

func TestTableRowsSeparateReposWithDividers(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")})
	model.expanded = true

	rows := model.tableRows()

	if len(rows) != 3 {
		t.Fatalf("expected two repo rows separated by one divider, got %d rows: %#v", len(rows), rows)
	}
	if !rows[1].divider || rows[1].repoIndex != -1 {
		t.Fatalf("expected divider entry between repos, got %#v", rows)
	}
	if rows[2].number != "2" || rows[2].marker || rows[2].repo != "repo-b" {
		t.Fatalf("expected second repo after divider, got %#v", rows)
	}
}

func TestExpandedGroupsKeepSpacingAroundSelection(t *testing.T) {
	model := newTestModel(testRepos("repo-a", "repo-b", "repo-c"))
	model.expanded = true
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)

	rows := model.tableRows()
	if !rows[1].divider || !rows[3].divider {
		t.Fatalf("expected hairline dividers around selected repo, got %#v", rows)
	}
	if rows[1].repoIndex != -1 || rows[3].repoIndex != -1 {
		t.Fatalf("expected dividers to carry no repo, got %#v", rows)
	}
}

func TestRenderedRowsAlignToFullTableWidth(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo-a"), testRepo("repo-b")})
	model.updateTableWithSize(80, 10)

	for _, entry := range model.visibleRows() {
		if got := runewidth.StringWidth(model.renderRow(entry)); got != model.lineWidth() {
			t.Fatalf("expected rendered row width %d, got %d (%q)", model.lineWidth(), got, model.renderRow(entry))
		}
	}
}

func TestTableRowsShowChangeCountsInSummary(t *testing.T) {
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

	if !strings.Contains(rows[0].status, "main • 1 modified • 1 untracked") {
		t.Fatalf("expected change counts in repo summary, got %#v", rows[0])
	}
}

func TestRepoColumnCapsWidthOnWideTerminals(t *testing.T) {
	results := []ui.RepoResult{{Repo: testRepo("demo-07-a-very-long-service-name-that-truncates")}}
	_, repoWidth, _ := columnWidths(240, results)
	if repoWidth > maxRepoWidthCap {
		t.Fatalf("expected repo column capped at %d, got %d", maxRepoWidthCap, repoWidth)
	}
}

func TestTableColumnsHaveSpacingAndFitWidth(t *testing.T) {
	model := newTestModel([]discover.Repo{testRepo("repo")})
	model.updateTableWithSize(40, 10)

	widths := model.colWidths
	if widths[0]+widths[1]+widths[2]+columnGap*2 > model.tableWidth {
		t.Fatalf("expected columns plus spacing to fit table width, got %#v and width %d", widths, model.tableWidth)
	}
	if model.lineWidth() > model.tableWidth {
		t.Fatalf("expected rendered lines within table width, got %d over %d", model.lineWidth(), model.tableWidth)
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
