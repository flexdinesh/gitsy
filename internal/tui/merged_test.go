package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/muesli/termenv"
)

// mergedTestModel opens the Worktrees tab with web clean merged, docs dirty
// merged and cli unmerged.
func mergedTestModel() Model {
	model := newTestModel(testRepos("api", "web", "docs", "cli"))
	for index := range model.results {
		model.results[index].Loading = false
		model.results[index].Status = status.Parse("## feature...origin/feature\n")
		if index > 0 {
			model.results[index].Repo.Worktree = &git.Worktree{Path: model.results[index].Repo.Path, MainPath: model.results[0].Repo.Path, Branch: "feature"}
		}
	}
	model.results[1].MergedInto = "origin/main"
	model.results[2].MergedInto = "origin/main"
	model.results[2].Status = status.Parse("## feature\n M dirty.go\n")
	model.next = len(model.results)
	model.width, model.height = 120, 24
	model.merged = func(context.Context, string, string) string { return "origin/main" }
	model.refresh()
	model, _ = keyModel(model, "tab")
	return model
}

// runBulk feeds each bulk step back into the model until deletion finishes.
func runBulk(t *testing.T, model Model, command tea.Cmd) Model {
	t.Helper()
	for steps := 0; command != nil; steps++ {
		if steps > 10 {
			t.Fatal("bulk deletion did not finish")
		}
		updated, next := model.Update(command())
		model, command = updated.(Model), next
	}
	return model
}

func TestMergedWorktreesRenderDimmedWithNote(t *testing.T) {
	model := mergedTestModel()
	view := model.View()
	for _, want := range []string{"merged · feature", "Merged into origin/main", "D merged"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	if strings.Count(view, "merged · feature") != 2 {
		t.Fatalf("only merged worktrees get the note:\n%s", view)
	}
	model.expanded = true
	model.refresh()
	for _, entry := range model.tableRows() {
		if entry.repoIndex >= 0 && entry.merged != (entry.repoIndex < 2) {
			t.Fatalf("every row of a merged worktree must dim: %+v", entry)
		}
	}

	profile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(profile) })
	lipgloss.SetColorProfile(termenv.TrueColor)
	cols := model.columns(100)
	entry := rowEntry{number: "1", repo: "web", worktree: "web", status: "merged · feature", tone: "green", repoIndex: 0}
	plain := renderRow(entry, cols, 1)
	entry.merged = true
	dimmed := renderRow(entry, cols, 1)
	if dimmed == plain || !strings.Contains(dimmed, headerMetaStyle().Render("web")) {
		t.Fatalf("merged rows must dim identity and status:\n%q\n%q", plain, dimmed)
	}
	if renderRow(entry, cols, 0) != renderRow(rowEntry{number: "1", repo: "web", worktree: "web", status: "merged · feature", tone: "green", repoIndex: 0}, cols, 0) {
		t.Fatal("selected merged rows keep selection styling")
	}

	model, _ = keyModel(model, "tab")
	if view := model.View(); strings.Contains(view, "merged") || strings.Contains(view, "Merged into") {
		t.Fatalf("Repositories tab must not show merged state:\n%s", view)
	}
}

func TestMergedNoteFollowsLockedAndCompactViews(t *testing.T) {
	model := mergedTestModel()
	model.results[1].Repo.Worktree.Locked = true
	model.refresh()
	if !strings.Contains(model.View(), "locked · merged · feature") {
		t.Fatal(model.View())
	}
	model.width, model.height = 30, 8
	model.refresh()
	if view := model.View(); !strings.Contains(view, "merged · feature") {
		t.Fatalf("compact view must show merged note:\n%s", view)
	}
}

func TestBulkDeleteMergedWorktrees(t *testing.T) {
	model := mergedTestModel()
	removed := []string{}
	model.remove = func(_ context.Context, owner, path string, force bool) git.WorktreeRemovalResult {
		if owner != model.results[0].Repo.Path || !force {
			t.Fatalf("bulk delete must force remove from the main checkout: %s %t", owner, force)
		}
		removed = append(removed, path)
		return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
	}
	checked := []string{}
	model.base = "develop"
	model.merged = func(_ context.Context, path, base string) string {
		checked = append(checked, path+"@"+base)
		return "origin/develop"
	}

	model, command := keyModel(model, "D")
	if command != nil || len(model.bulkConfirm) != 2 {
		t.Fatal("D must confirm before deleting")
	}
	view := model.View()
	prompt := strings.Join(model.actionLines(), " ")
	if prompt != "Delete 2 merged worktrees? 1 has local files that will be discarded. Branches kept." || !strings.Contains(view, "y force · esc cancel") {
		t.Fatalf("unexpected confirmation %q:\n%s", prompt, view)
	}
	for _, key := range []string{"x", "D", "down", "tab"} {
		model, command = keyModel(model, key)
		if command != nil || model.bulkConfirm == nil || !model.worktrees {
			t.Fatalf("%s must not act while confirming", key)
		}
	}

	model, command = keyModel(model, "y")
	if command == nil || model.deleting != "/tmp/web" || model.actionLines()[0] != "Deleting 1/2 merged worktrees…" {
		t.Fatalf("y must start sequential deletion: %v", model.actionLines())
	}
	model = runBulk(t, model, command)
	if !slices.Equal(removed, []string{"/tmp/web", "/tmp/docs"}) || !slices.Equal(checked, []string{"/tmp/web@develop", "/tmp/docs@develop"}) {
		t.Fatalf("removed %v checked %v", removed, checked)
	}
	if model.notice != "Deleted 2 merged worktrees. Branches kept." || model.bulk != nil || model.deleting != "" {
		t.Fatalf("unexpected completion notice %q", model.notice)
	}
	if results := model.displayResults(); len(results) != 1 || results[0].Repo.DisplayName != "cli" || strings.Contains(model.View(), "D merged") {
		t.Fatal("deleted worktrees must disappear and hide the D hint")
	}
}

func TestBulkDeleteReportsSkipsAndFailures(t *testing.T) {
	model := mergedTestModel()
	model.merged = func(_ context.Context, path, _ string) string {
		if path == "/tmp/web" {
			return ""
		}
		return "origin/main"
	}
	model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
		return git.WorktreeRemovalResult{Result: git.Result{Stderr: "Worktree is locked; unlock it before deleting.", Status: -1}}
	}
	model, _ = keyModel(model, "D")
	model, command := keyModel(model, "y")
	model = runBulk(t, model, command)
	want := "Deleted 0 merged worktrees. 1 skipped, 1 failed: Worktree is locked; unlock it before deleting. Branches kept."
	if model.notice != want || len(model.displayResults()) != 3 {
		t.Fatalf("got %q, want %q", model.notice, want)
	}
}

func TestBulkDeleteGuards(t *testing.T) {
	for _, test := range []struct {
		name   string
		setup  func(*Model)
		keys   []string
		notice string
	}{
		{name: "cancel", keys: []string{"D", "esc"}},
		{name: "decline", keys: []string{"D", "n"}},
		{name: "none merged", setup: func(model *Model) {
			for index := range model.results {
				model.results[index].MergedInto = ""
			}
		}, keys: []string{"D"}, notice: "No merged worktrees."},
		{name: "pending inspections", setup: func(model *Model) { model.results[3].Loading = true }, keys: []string{"D"}, notice: "Wait for inspections to finish before deleting."},
		{name: "repositories tab", setup: func(model *Model) { model.switchTab() }, keys: []string{"D", "y"}},
		{name: "terminal too small", setup: func(model *Model) { model.width, model.height = 20, 4 }, keys: []string{"D", "y", "esc"}},
		{name: "burst", keys: []string{"Dy"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := mergedTestModel()
			model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
				t.Fatal("must not remove worktrees")
				return git.WorktreeRemovalResult{}
			}
			if test.setup != nil {
				test.setup(&model)
				model.refresh()
			}
			for _, key := range test.keys {
				var command tea.Cmd
				model, command = keyModel(model, key)
				if command != nil {
					if message := command(); message != nil {
						t.Fatalf("%s started work: %#v", key, message)
					}
				}
			}
			if model.bulkConfirm != nil || model.bulk != nil || model.deleting != "" || model.notice != test.notice {
				t.Fatalf("unexpected state: confirm %v notice %q", model.bulkConfirm != nil, model.notice)
			}
		})
	}
}

func TestBulkDeleteRemovesSharedAppearanceOnce(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].MergedInto = "origin/main"
	model.merged = func(context.Context, string, string) string { return "origin/main" }
	calls := 0
	model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
		calls++
		return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
	}
	model, _ = keyModel(model, "tab")
	if len(model.displayResults()) != 2 {
		t.Fatal("fixture must show the worktree in two groups")
	}
	model, _ = keyModel(model, "D")
	model, command := keyModel(model, "y")
	model = runBulk(t, model, command)
	if calls != 1 || model.notice != "Deleted 1 merged worktree. Branches kept." {
		t.Fatalf("calls %d notice %q", calls, model.notice)
	}
}

func TestMergedViewsFitTerminals(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {20, 4}, {20, 6}, {31, 10}, {32, 7}, {40, 10}, {80, 24}, {120, 24}} {
		for _, state := range []string{"merged", "confirm", "deleting", "done"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				model := mergedTestModel()
				model.width, model.height = size[0], size[1]
				switch state {
				case "confirm":
					model.bulkConfirm = model.mergedResults()
				case "deleting":
					model.bulk = &bulkDeletion{repos: []discover.Repo{model.results[1].Repo, model.results[2].Repo}}
					model.deleting = model.results[1].Repo.Path
				case "done":
					model.notice = bulkDeletion{deleted: 1, skipped: 1, failed: 1, err: "Worktree is locked; unlock it before deleting."}.summary()
				}
				model.refresh()
				view := model.View()
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
					t.Fatalf("overflow:\n%s", view)
				}
			})
		}
	}
}
