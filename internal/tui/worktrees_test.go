package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/muesli/termenv"
)

func worktreeTestModel() Model {
	model := groupedTestModel()
	model.groups[1].RepoIndexes = []int{1}
	model.results[1].Repo.Worktree = &git.Worktree{Path: model.results[1].Repo.Path, MainPath: model.results[0].Repo.Path, Branch: "feature"}
	for index := range model.results {
		model.results[index].Loading = false
		model.results[index].Status = status.Parse("## feature...origin/feature\n")
	}
	model.next = len(model.results)
	model.refresh()
	return model
}

func keyModel(model Model, key string) (Model, tea.Cmd) {
	message := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	switch key {
	case "right":
		message = tea.KeyMsg{Type: tea.KeyRight}
	case "enter":
		message = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		message = tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		message = tea.KeyMsg{Type: tea.KeyDown}
	}
	updated, command := model.Update(message)
	return updated.(Model), command
}

func TestWorktreeTabFiltersAndRestoresSelection(t *testing.T) {
	model := worktreeTestModel()
	model.selectRepo(2)
	model, _ = keyModel(model, "right")
	if !model.worktrees || len(model.visibleResults()) != 1 || len(model.displayResults()) != 2 || model.selected != 0 {
		t.Fatalf("worktree tab must share overlapping groups: %+v", model.displayResults())
	}
	if view := model.View(); !strings.Contains(view, "[Worktrees]") || !strings.Contains(view, "1 unique worktrees") || !strings.Contains(view, "x delete") {
		t.Fatal(view)
	}
	model, _ = keyModel(model, "down")
	model, _ = keyModel(model, "right")
	if model.worktrees || model.selected != 2 {
		t.Fatal("repository selection lost")
	}
	model, _ = keyModel(model, "right")
	if model.selected != 1 {
		t.Fatal("worktree selection lost")
	}
}

func TestWorktreeDeleteConfirmsExactTargetAndUpdatesBothTabs(t *testing.T) {
	model := worktreeTestModel()
	path := model.results[1].Repo.Path
	calls := 0
	model.remove = func(ctx context.Context, owner, target string) git.Result {
		calls++
		if owner != model.results[0].Repo.Path || target != path {
			t.Fatalf("wrong deletion target: %s, %s", owner, target)
		}
		return git.Result{OK: true}
	}
	model, command := keyModel(model, "x")
	if command != nil || model.confirm != nil {
		t.Fatal("X must only act in Worktrees")
	}
	model, _ = keyModel(model, "right")
	model, command = keyModel(model, "X")
	if command != nil || model.confirm == nil || calls != 0 || !strings.Contains(model.View(), path) {
		t.Fatal("X must show target without deleting")
	}
	model, _ = keyModel(model, "down")
	model, _ = keyModel(model, "right")
	if !model.worktrees || model.selected != 0 {
		t.Fatal("confirmation must freeze selection")
	}
	model, _ = keyModel(model, "esc")
	if model.confirm != nil || calls != 0 {
		t.Fatal("Esc must cancel")
	}
	model, _ = keyModel(model, "x")
	model, command = keyModel(model, "enter")
	if command == nil || model.deleting != path || calls != 0 {
		t.Fatal("Enter must start async removal")
	}
	model, ignored := keyModel(model, "x")
	if ignored != nil {
		t.Fatal("must not start duplicate deletion")
	}
	updated, _ := model.Update(command())
	model = updated.(Model)
	if calls != 1 || len(model.displayResults()) != 0 || model.deleting != "" || !strings.Contains(model.View(), "Branch kept") {
		t.Fatal("deletion must remove all appearances and acknowledge success")
	}
	model, _ = keyModel(model, "right")
	if len(model.visibleResults()) != 1 || len(model.displayResults()) != 1 || model.selected != 0 {
		t.Fatal("removed worktree must disappear from Repositories and clamp selection")
	}
}

func TestWorktreeDeletionChecksFreshStateAndAllowsRecovery(t *testing.T) {
	for _, state := range []string{"loading", "locked", "failed", "dirty"} {
		t.Run(state, func(t *testing.T) {
			model := worktreeTestModel()
			switch state {
			case "loading":
				model.results[0].Loading = true
			case "locked":
				model.results[1].Repo.Worktree.Locked = true
			case "failed":
				model.results[1].Failed = true
			case "dirty":
				model.results[1].Status = status.Parse("## feature\n?? untracked\n")
			}
			model, _ = keyModel(model, "right")
			model, command := keyModel(model, "x")
			if state == "loading" {
				if command != nil || model.confirm != nil || model.notice == "" {
					t.Fatal("must wait for inspections to avoid concurrent Git operations")
				}
				return
			}
			protected := true
			model.remove = func(context.Context, string, string) git.Result {
				return git.Result{OK: !protected, Stderr: "Worktree is protected"}
			}
			model, command = keyModel(model, "enter")
			if command == nil {
				t.Fatal("confirmed deletion must check fresh Git state")
			}
			updated, _ := model.Update(command())
			model = updated.(Model)
			if len(model.visibleResults()) != 1 || !strings.Contains(model.notice, "protected") {
				t.Fatal("protected worktree must remain and show the fresh refusal")
			}
			protected = false
			model, _ = keyModel(model, "x")
			model, command = keyModel(model, "enter")
			if command == nil {
				t.Fatal("external cleanup must permit retry despite cached status")
			}
			updated, _ = model.Update(command())
			if len(updated.(Model).visibleResults()) != 0 {
				t.Fatal("fresh unprotected state must permit removal")
			}
		})
	}
}

func TestWorktreeContextMatchesSelectedIdentity(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.DisplayName = "origin-name"
	model.results[1].Repo.Path = "/workspace/feature-auth"
	model, _ = keyModel(model, "right")
	info := strings.Join(model.infoLines(30), "\n")
	if !strings.Contains(info, "feature-auth") || strings.Contains(info, "origin-name") {
		t.Fatal("worktree context must match row identity")
	}
}

func TestWorktreeRemovalErrorRetainsRowAndAllowsRetry(t *testing.T) {
	model := worktreeTestModel()
	model.remove = func(context.Context, string, string) git.Result { return git.Result{Stderr: "worktree has changes"} }
	model, _ = keyModel(model, "right")
	model, _ = keyModel(model, "x")
	model, command := keyModel(model, "enter")
	updated, _ := model.Update(command())
	model = updated.(Model)
	if len(model.visibleResults()) != 1 || !strings.Contains(model.View(), "Delete failed: worktree has changes") {
		t.Fatal("failure must retain row and explain problem")
	}
	model, _ = keyModel(model, "x")
	if model.confirm == nil {
		t.Fatal("failure must allow retry")
	}
}

func TestWorktreeConfirmationRequiresVisiblePath(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.Path = "/workspace/" + strings.Repeat("long-directory/", 12) + "target"
	model.width, model.height = 20, 6
	model, _ = keyModel(model, "right")
	model, _ = keyModel(model, "x")
	model, command := keyModel(model, "enter")
	if command != nil || model.confirm == nil || !strings.Contains(model.View(), "Enlarge terminal") {
		t.Fatal("must not confirm deletion with a hidden target")
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model = updated.(Model)
	if !model.canConfirm() || !strings.Contains(model.View(), "target") {
		t.Fatal("resize must reveal path and enable confirmation")
	}
}

func TestWorktreeViewsFitTerminals(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {20, 6}, {31, 10}, {32, 7}, {40, 10}, {80, 24}, {120, 24}} {
		for _, state := range []string{"ready", "loading", "dirty", "failed", "stale", "sync", "locked", "empty", "confirm", "deleting", "error"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				model := worktreeTestModel()
				model.width, model.height = size[0], size[1]
				model, _ = keyModel(model, "right")
				switch state {
				case "loading":
					model.results[1].Loading = true
				case "dirty":
					model.results[1].Status = status.Parse("## feature\n M 日本語.go\n")
				case "failed":
					model.results[1].Failed = true
				case "stale":
					model.results[1].Stale = true
				case "sync":
					model.sync = true
				case "locked":
					model.results[1].Repo.Worktree.Locked = true
				case "empty":
					model.results[1].Repo.Worktree = nil
				case "confirm":
					model, _ = keyModel(model, "x")
				case "deleting":
					model.deleting = model.results[1].Repo.Path
				case "error":
					model.notice = "Delete failed: changed files"
				}
				model.refresh()
				view := model.View()
				if lipgloss.Width(view) > size[0] || lipgloss.Height(view) > size[1] {
					t.Fatalf("overflow: %s", view)
				}
			})
		}
	}
}

func TestWorktreesWithoutGroups(t *testing.T) {
	model := worktreeTestModel()
	model.groups = nil
	model, _ = keyModel(model, "right")
	if len(model.displayResults()) != 1 {
		t.Fatal("ungrouped models must filter linked worktrees")
	}
	model.results[1].Repo.Worktree = nil
	model.refresh()
	if !strings.Contains(model.View(), "No linked worktrees found.") {
		t.Fatal("missing empty state")
	}
}

func TestRenderWorktreePreviews(t *testing.T) {
	directory := os.Getenv("GITSY_PREVIEW_DIR")
	if directory == "" {
		t.Skip("set GITSY_PREVIEW_DIR to capture terminal renders")
	}
	profile, dark := lipgloss.ColorProfile(), lipgloss.HasDarkBackground()
	t.Cleanup(func() {
		lipgloss.SetColorProfile(profile)
		lipgloss.SetHasDarkBackground(dark)
	})
	lipgloss.SetColorProfile(termenv.TrueColor)
	captures := map[string]string{}
	for _, theme := range []string{"dark", "light"} {
		lipgloss.SetHasDarkBackground(theme == "dark")
		for _, mode := range []string{"wide", "narrow", "compact", "confirm", "confirm-narrow", "empty", "error", "loading", "deleting"} {
			model := previewModel()
			model.width, model.height = 120, 24
			for index := range model.results {
				model.results[index].Repo.Worktree = &git.Worktree{Path: model.results[index].Repo.Path, MainPath: "/workspace/" + model.results[index].Repo.DisplayName}
			}
			model.results[2].Repo.Worktree.Locked = true
			model.results[2].Repo.Worktree.LockReason = "Release in progress"
			model.results[0].Repo.Path = "/workspace/api-auth"
			model.groups = []discover.Group{
				{Path: "/workspace/work", RepoIndexes: []int{0, 1, 2, 3}},
				{Path: "/workspace/personal", RepoIndexes: []int{4, 5, 6, 7}},
			}
			model.worktrees = true
			if mode == "narrow" || mode == "confirm-narrow" {
				model.width, model.height = 40, 16
			}
			if mode == "compact" {
				model.width, model.height = 28, 6
			}
			if mode == "empty" {
				for index := range model.results {
					model.results[index].Repo.Worktree = nil
				}
			}
			if mode == "confirm" || mode == "confirm-narrow" {
				model.confirm = &model.results[1].Repo
				model.selected = 1
			}
			if mode == "error" {
				model.notice = "Delete failed: worktree has ignored files; remove them before deleting."
			}
			if mode == "deleting" {
				model.deleting = model.results[0].Repo.Path
			}
			model.refresh()
			captures[theme+"-worktrees-"+mode] = model.View()
		}
	}
	data, err := json.Marshal(captures)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "worktrees-renders.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
