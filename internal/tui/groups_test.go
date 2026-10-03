package tui

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
)

func groupedTestModel() Model {
	model := newTestModel(testRepos("api", "web"))
	model.groups = []discover.Group{
		{Path: "/workspace/work", RepoIndexes: []int{1, 0}},
		{Path: "/workspace/personal", RepoIndexes: []int{0}},
		{Path: "/workspace/empty"},
	}
	model.width, model.height = 120, 24
	model.refresh()
	return model
}

func TestDirectoryGroupsKeepOrderAndEmptyRoots(t *testing.T) {
	model := groupedTestModel()
	view := model.View()
	previous := -1
	for _, text := range []string{"/workspace/work", "web", "api", "/workspace/personal", "api", "/workspace/empty", "No child git repositories found."} {
		index := strings.Index(view[previous+1:], text)
		if index < 0 {
			t.Fatalf("missing ordered group content %q:\n%s", text, view)
		}
		previous += index + 1
	}
	for _, text := range []string{"3 directories", "2 unique repos", "2 repos", "1 repo", "0 repos"} {
		if !strings.Contains(view, text) {
			t.Fatalf("missing count %q:\n%s", text, view)
		}
	}
}

func TestGroupNavigationSelectsAppearancesAndSkipsHeaders(t *testing.T) {
	model := groupedTestModel()
	for index, name := range []string{"web", "api", "api"} {
		model.selectRepo(index)
		if model.selectedResult().Repo.DisplayName != name {
			t.Fatalf("appearance %d selected wrong repo", index)
		}
		markers := 0
		for _, row := range model.tableRows() {
			if row.marker {
				markers++
				if row.repoIndex != index || row.group != "" {
					t.Fatal("selection must belong to one repo appearance")
				}
			}
		}
		if markers != 1 {
			t.Fatalf("expected one marker, got %d", markers)
		}
	}
	model.selectRepo(1)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.selected != 2 || model.selectedGroup() != "/workspace/personal" {
		t.Fatal("down must cross the group header to the repeated repo")
	}
	if !strings.Contains(model.View(), "3/3") || !strings.Contains(model.View(), "/tmp/api") {
		t.Fatal("position and context must follow the selected appearance")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyHome})
	model = updated.(Model)
	if model.selected != 0 || model.selectedResult().Repo.DisplayName != "web" {
		t.Fatal("Home must select the first appearance")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if updated.(Model).selected != 2 {
		t.Fatal("End must select the final appearance before empty groups")
	}
}

func TestGroupedInspectionsRunOnceAndUpdateEveryAppearance(t *testing.T) {
	calls := map[string]int{}
	model := newModel(context.Background(), nil, testRepos("api", "web"), false, true, nil,
		func(_ context.Context, repo discover.Repo, noFetch, syncRepos bool, _ func(string)) inspect.Result {
			calls[repo.Path]++
			if noFetch || !syncRepos {
				t.Fatal("grouped inspection lost fetch/sync options")
			}
			return inspect.Result{Repo: repo, Status: status.Parse("## main\n M changed.go"), Stale: true,
				Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}}
		})
	model.groups = groupedTestModel().groups
	commands := model.Init()().(tea.BatchMsg)
	for index := range model.results {
		updated, _ := model.Update(commands[index]())
		model = updated.(Model)
	}
	if calls["/tmp/api"] != 1 || calls["/tmp/web"] != 1 || model.pending() != 0 {
		t.Fatalf("each unique repo must be inspected once: calls=%v pending=%d", calls, model.pending())
	}
	model.width, model.height = 120, 24
	model.refresh()
	model.selectRepo(2)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(Model)
	view := model.View()
	if model.selected != 2 || strings.Count(view, "modified changed.go") != 3 || strings.Count(view, "synced ↓2") < 3 {
		t.Fatalf("completion and file toggle must update all appearances:\n%s", view)
	}
	updated, _ = model.Update(repoDoneMsg{index: 0, result: inspect.Result{Repo: testRepo("api"), Failed: true}})
	if updated.(Model).results[0].Failed {
		t.Fatal("duplicate completion must not replace a completed result")
	}
}

func TestGroupedScrollKeepsDirectoryAndSelectedRepoVisible(t *testing.T) {
	for _, height := range []int{7, 8, 9, 10, 12} {
		for _, expanded := range []bool{false, true} {
			t.Run(fmt.Sprintf("height=%d/files=%t", height, expanded), func(t *testing.T) {
				model := groupedTestModel()
				model.height = height
				model.expanded = expanded
				model.groups[0].RepoIndexes = []int{0, 1, 0, 1, 0, 1}
				model.refresh()
				for index := range model.displayResults() {
					model.selectRepo(index)
					visible := model.visibleRows()
					if firstRowOf(visible, index) < 0 {
						t.Fatalf("appearance %d invisible at offset %d: %#v", index, model.offset, visible)
					}
					if model.capacity > 1 && visible[0].group == "" {
						t.Fatalf("scrolled group must retain directory context: %#v", visible)
					}
				}
				model.scrollViewport(-100)
				if model.capacity > 1 && (model.selected != 0 || firstRowOf(model.visibleRows(), 0) < 0) {
					t.Fatal("scrolling to top must select first repo")
				}
				if model.capacity == 1 && (model.offset != 0 || model.visibleRows()[0].group == "") {
					t.Fatal("one-row viewport must show the top group header")
				}
			})
		}
	}
}

func TestGroupedViewsFitTerminalAndKeepStates(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {20, 6}, {31, 10}, {32, 7}, {40, 10}, {80, 24}, {104, 24}, {120, 24}} {
		for _, state := range []string{"loading", "files", "failed", "empty"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				model := groupedTestModel()
				model.groups[0].Path = "/workspace/日本語/long-directory-name\nwith\ttabs"
				model.width, model.height = size[0], size[1]
				if state == "empty" {
					model.results = nil
					for index := range model.groups {
						model.groups[index].RepoIndexes = nil
					}
				} else if state != "loading" {
					for index := range model.results {
						model.results[index].Loading = false
						model.results[index].Failed = state == "failed"
						model.results[index].Status = status.Parse("## main\n M 日本語.go")
					}
					model.expanded = state == "files"
				}
				model.refresh()
				before := snapshotResults(model)
				view := model.View()
				if !reflect.DeepEqual(before, snapshotResults(model)) {
					t.Fatal("rendering must not mutate inspection results")
				}
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
					t.Fatalf("view overflows terminal:\n%s", view)
				}
				if size[0] >= 80 && state == "failed" && !strings.Contains(view, "status failed") {
					t.Fatal("grouping must retain failure text")
				}
			})
		}
	}
}

type uiRepoSnapshot struct {
	loadingText string
	loading     bool
}

func snapshotResults(model Model) []uiRepoSnapshot {
	results := []uiRepoSnapshot{}
	for _, result := range model.results {
		results = append(results, uiRepoSnapshot{result.LoadingText, result.Loading})
	}
	return results
}

func TestDisplayGroupPath(t *testing.T) {
	for _, test := range []struct{ path, home, want string }{
		{"/home/dee", "/home/dee", "~"},
		{"/home/dee/work", "/home/dee", "~/work"},
		{"/home/dee-other/work", "/home/dee", "/home/dee-other/work"},
		{"/workspace/日本語\nwith\ttabs", "", "/workspace/日本語\\nwith\\ttabs"},
	} {
		if got := displayGroupPath(test.path, test.home); got != test.want {
			t.Fatalf("path %q: got %q want %q", test.path, got, test.want)
		}
	}
}

func TestNarrowGroupedHeaderKeepsExceptionCounts(t *testing.T) {
	model := previewModel()
	model.groups = []discover.Group{
		{Path: "/workspace/work", RepoIndexes: []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{Path: "/workspace/personal", RepoIndexes: []int{0}},
	}
	header := model.renderHeader(40)
	for _, label := range []string{"2 dirs", "8 unique repos", "1 failed", "2 changed", "1 behind", "1 stale"} {
		if !strings.Contains(header, label) {
			t.Fatalf("group count must not crowd out %q:\n%s", label, header)
		}
	}
}

func TestScrollCanReachEmptyGroupsAndResumeRepoNavigation(t *testing.T) {
	for _, key := range []tea.Msg{
		tea.KeyMsg{Type: tea.KeyPgDown},
		tea.MouseMsg{Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown},
	} {
		model := groupedTestModel()
		model.height = 8
		model.refresh()
		model.selectRepo(2)
		updated, _ := model.Update(key)
		model = updated.(Model)
		if !model.browseEmpty || !strings.Contains(model.View(), "/workspace/empty") || !strings.Contains(model.View(), "No child git repositories found.") {
			t.Fatalf("page/wheel must reach trailing empty group:\n%s", model.View())
		}
		model.refresh()
		if !model.browseEmpty || !strings.Contains(model.View(), "/workspace/empty") {
			t.Fatal("inspection/spinner refresh must preserve empty-group viewport")
		}
		resized, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
		larger := resized.(Model)
		if larger.browseEmpty || firstRowOf(larger.visibleRows(), larger.selected) < 0 || !strings.Contains(larger.View(), "1/3") {
			t.Fatal("growing viewport must restore selection when repos become visible")
		}
		updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
		model = updated.(Model)
		if model.browseEmpty || model.selected != 1 || firstRowOf(model.visibleRows(), 1) < 0 {
			t.Fatal("repo navigation must restore a visible selected appearance")
		}
	}
}

func TestCompactGroupedSummaryLabelsUniqueRepos(t *testing.T) {
	model := groupedTestModel()
	model.width, model.height = 31, 6
	model.refresh()
	if !strings.Contains(model.View(), "2 unique repos") {
		t.Fatalf("compact overlap count must identify unique repos:\n%s", model.View())
	}
}
