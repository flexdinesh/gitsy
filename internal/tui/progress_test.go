package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/inspect"
)

func TestInspectionProgressUpdatesUniqueAndDirectoryCounts(t *testing.T) {
	model := groupedTestModel()
	model.noFetch = false
	assertProgress := func(want string, groups ...string) {
		t.Helper()
		if got := model.headerRight(); got != want {
			t.Fatalf("header = %q, want %q", got, want)
		}
		index := 0
		for _, row := range model.tableRows() {
			if row.group == "" {
				continue
			}
			got := renderRow(row, model.colWidths, model.selected)
			if !strings.HasSuffix(got, groups[index]) {
				t.Fatalf("directory %q = %q, want suffix %q", row.group, got, groups[index])
			}
			index++
		}
	}
	assertProgress("fetch · 0/2 in progress", "0/2 repos", "0/1 repo", "0/0 repos")
	updated, _ := model.Update(repoDoneMsg{index: 0, result: inspect.Result{Repo: model.results[0].Repo, Failed: true}})
	model = updated.(Model)
	assertProgress("fetch · 1/2 in progress", "1/2 repos", "1/1 repo", "0/0 repos")
	updated, _ = model.Update(repoDoneMsg{index: 0, result: inspect.Result{Repo: model.results[0].Repo}})
	model = updated.(Model)
	assertProgress("fetch · 1/2 in progress", "1/2 repos", "1/1 repo", "0/0 repos")
	updated, _ = model.Update(repoDoneMsg{index: 1, result: inspect.Result{Repo: model.results[1].Repo, Stale: true, Sync: &inspect.SyncOutcome{Kind: "failed"}}})
	model = updated.(Model)
	assertProgress("fetch · 2/2 done", "2/2 repos", "1/1 repo", "0/0 repos")
}

func TestProgressModesAndEmptyWorkspace(t *testing.T) {
	for _, test := range []struct {
		noFetch, sync bool
		mode          string
	}{
		{false, false, "fetch"},
		{true, false, "local"},
		{false, true, "sync"},
		{true, true, "sync"},
	} {
		model := NewModel(nil, test.noFetch, test.sync, nil)
		if got, want := model.headerRight(), test.mode+" · 0/0 done"; got != want {
			t.Fatalf("empty header = %q, want %q", got, want)
		}
	}
}

func TestWorktreeProgressFiltersSharedResults(t *testing.T) {
	model := worktreeTestModel()
	model.noFetch = false
	model.results[0].Loading = true
	model.results[1].Loading = true
	model, _ = keyModel(model, "tab")
	if got := model.headerRight(); got != "fetch · 0/1 in progress" {
		t.Fatal(got)
	}
	updated, _ := model.Update(repoDoneMsg{index: 1, result: inspect.Result{Repo: model.results[1].Repo}})
	model = updated.(Model)
	if model.pending() != 1 || model.headerRight() != "fetch · 1/1 done" {
		t.Fatal("worktree completion must exclude pending main checkout")
	}
	model.removed = map[string]bool{model.results[1].Repo.Path: true}
	model.refresh()
	if got := model.headerRight(); got != "fetch · 0/0 done" {
		t.Fatal(got)
	}
	model, _ = keyModel(model, "tab")
	if got := model.headerRight(); got != "fetch · 0/1 in progress" {
		t.Fatal("removed worktree must leave both counters: " + got)
	}
}

func TestProgressStaysRightAlignedAndFitsNarrowTerminals(t *testing.T) {
	for _, size := range [][2]int{{20, 6}, {31, 8}, {32, 10}, {40, 10}, {80, 10}, {120, 24}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			model := groupedTestModel()
			model.groups[0].Path = "/workspace/日本語/long-directory-name"
			model.width, model.height = size[0], size[1]
			model.refresh()
			view := model.View()
			if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
				t.Fatalf("overflow:\n%s", view)
			}
			lines := strings.Split(view, "\n")
			if !strings.Contains(lines[0], "0/2") {
				t.Fatalf("global progress missing:\n%s", view)
			}
			found := false
			for _, line := range lines[1:] {
				line = strings.SplitN(line, "│", 2)[0]
				if strings.HasSuffix(strings.TrimRight(line, " "), "0/2 repos") {
					found = true
					if size[0] < minWidth && lipgloss.Width(line) != size[0] {
						t.Fatal("compact directory progress must align right")
					}
				}
			}
			if !found {
				t.Fatalf("directory progress missing:\n%s", view)
			}
			model.offset = 1
			if model.capacity > 1 {
				sticky := model.visibleRows()[0]
				if sticky.groupDone != 0 || sticky.groupSize != 2 || sticky.group == "" {
					t.Fatal("sticky directory must retain progress")
				}
			}
		})
	}
}
