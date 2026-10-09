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
	case "tab":
		message = tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		message = tea.KeyMsg{Type: tea.KeyShiftTab}
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
	model, _ = keyModel(model, "tab")
	if !model.worktrees || len(model.visibleResults()) != 1 || len(model.displayResults()) != 2 || model.selected != 0 {
		t.Fatalf("worktree tab must share overlapping groups: %+v", model.displayResults())
	}
	if view := model.View(); !strings.Contains(view, "[Worktrees]") || !strings.Contains(view, "1 unique worktrees") || !strings.Contains(view, "x delete") {
		t.Fatal(view)
	}
	model, _ = keyModel(model, "down")
	model, _ = keyModel(model, "tab")
	if model.worktrees || model.selected != 2 {
		t.Fatal("repository selection lost")
	}
	model, _ = keyModel(model, "tab")
	if model.selected != 1 {
		t.Fatal("worktree selection lost")
	}
}

func TestWorktreeDeleteConfirmsExactTargetAndUpdatesBothTabs(t *testing.T) {
	model := worktreeTestModel()
	path := model.results[1].Repo.Path
	calls := 0
	model.remove = func(ctx context.Context, owner, target string, force bool) git.WorktreeRemovalResult {
		calls++
		if owner != model.results[0].Repo.Path || target != path || force {
			t.Fatalf("wrong deletion target: %s, %s", owner, target)
		}
		return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
	}
	model, command := keyModel(model, "x")
	if command != nil || model.confirm != nil {
		t.Fatal("X must only act in Worktrees")
	}
	model, _ = keyModel(model, "tab")
	model, command = keyModel(model, "X")
	if command != nil || model.confirm == nil || calls != 0 || !strings.Contains(model.View(), path) {
		t.Fatal("X must show target without deleting")
	}
	model, _ = keyModel(model, "down")
	model, _ = keyModel(model, "tab")
	if !model.worktrees || model.selected != 0 {
		t.Fatal("confirmation must freeze selection")
	}
	model, _ = keyModel(model, "esc")
	if model.confirm != nil || calls != 0 {
		t.Fatal("Esc must cancel")
	}
	model, _ = keyModel(model, "x")
	model, command = keyModel(model, "y")
	if command == nil || model.deleting != path || calls != 0 {
		t.Fatal("Y must start async removal")
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
	model, _ = keyModel(model, "tab")
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
			model, _ = keyModel(model, "tab")
			model, command := keyModel(model, "x")
			if state == "loading" {
				if command != nil || model.confirm != nil || model.notice == "" {
					t.Fatal("must wait for inspections to avoid concurrent Git operations")
				}
				return
			}
			protected := true
			model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
				return git.WorktreeRemovalResult{Result: git.Result{OK: !protected, Stderr: "Worktree is protected"}}
			}
			model, command = keyModel(model, "y")
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
			model, command = keyModel(model, "y")
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

func TestTabSwitchPreservesDensity(t *testing.T) {
	for _, key := range []string{"tab", "shift+tab", "right"} {
		for _, expanded := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/files=%t", key, expanded), func(t *testing.T) {
				model := worktreeTestModel()
				model.expanded = expanded
				model, _ = keyModel(model, key)
				if !model.worktrees || model.expanded != expanded {
					t.Fatal("switching views must preserve file density")
				}
				model, _ = keyModel(model, "f")
				if !model.worktrees || model.expanded == expanded {
					t.Fatal("F must toggle files without switching views")
				}
			})
		}
	}
}

func TestWorktreeForceDeleteRequiresSecondConfirmation(t *testing.T) {
	for _, cancel := range []string{"", "esc", "n", "N"} {
		t.Run("cancel="+cancel, func(t *testing.T) {
			model := worktreeTestModel()
			target := model.results[1].Repo
			calls := 0
			model.remove = func(_ context.Context, owner, path string, force bool) git.WorktreeRemovalResult {
				calls++
				if owner != target.Worktree.MainPath || path != target.Path || force != (calls == 2) {
					t.Fatal("must revalidate same target, forcing only on second Y")
				}
				if !force {
					return git.WorktreeRemovalResult{NeedsForce: true}
				}
				return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
			}
			model, _ = keyModel(model, "tab")
			model, _ = keyModel(model, "x")
			model, command := keyModel(model, "enter")
			if command != nil || model.confirm == nil {
				t.Fatal("Enter must not confirm deletion")
			}
			model, command = keyModel(model, "y")
			updated, _ := model.Update(command())
			model = updated.(Model)
			view := model.View()
			if calls != 1 || model.confirm == nil || !model.force || model.deleting != "" ||
				!strings.Contains(view, target.Path) || !strings.Contains(view, "Discards changed, untracked, and ignored files") ||
				!strings.Contains(view, "y force") || len(model.visibleResults()) != 1 {
				t.Fatalf("changed files must show second warning and retain target: %s", view)
			}
			for _, key := range []string{"tab", "down", "f", "x", "enter"} {
				model, command = keyModel(model, key)
				if command != nil || !model.worktrees || model.selected != 0 || !model.force || model.confirm == nil {
					t.Fatal("second confirmation must freeze navigation and ignore unrelated keys")
				}
			}
			if cancel != "" {
				model, _ = keyModel(model, cancel)
				if model.confirm != nil || model.force || calls != 1 || len(model.visibleResults()) != 1 {
					t.Fatal("cancellation must preserve worktree and clear force")
				}
				model, _ = keyModel(model, "x")
				model, command = keyModel(model, "y")
				if command == nil || model.force {
					t.Fatal("retry must start with normal deletion")
				}
				return
			}
			model, command = keyModel(model, "Y")
			if command == nil || calls != 1 {
				t.Fatal("second Y must start async force deletion")
			}
			updated, _ = model.Update(command())
			model = updated.(Model)
			if calls != 2 || len(model.displayResults()) != 0 || model.force || model.confirm != nil {
				t.Fatal("force deletion must remove all appearances")
			}
		})
	}
}

func TestForceConfirmationRequiresVisibleWarningAndPath(t *testing.T) {
	model := worktreeTestModel()
	model.width, model.height = 20, 6
	model, _ = keyModel(model, "tab")
	model, _ = keyModel(model, "x")
	model.force = true
	model, command := keyModel(model, "y")
	if command != nil || model.canConfirm() || !strings.Contains(model.View(), "Enlarge terminal") {
		t.Fatal("force confirmation must wait until full warning and path fit")
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	model = updated.(Model)
	if !model.canConfirm() || strings.Join(model.actionLines(), "") !=
		"Force delete worktree? Discards changed, untracked, and ignored files. Branch kept."+model.confirm.Path {
		t.Fatal("resize must reveal complete warning and path")
	}
}

func TestWorktreeContextMatchesSelectedIdentity(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.DisplayName = "origin-name"
	model.results[1].Repo.Path = "/workspace/feature-auth"
	model, _ = keyModel(model, "tab")
	info := strings.Join(model.infoLines(30), "\n")
	if !strings.Contains(info, "feature-auth") || !strings.Contains(info, "origin-name") {
		t.Fatal("worktree context must match row identity")
	}
}

func TestWorktreeRemovalErrorRetainsRowAndAllowsRetry(t *testing.T) {
	model := worktreeTestModel()
	model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
		return git.WorktreeRemovalResult{Result: git.Result{Stderr: "worktree has changes"}}
	}
	model, _ = keyModel(model, "tab")
	model, _ = keyModel(model, "x")
	model, command := keyModel(model, "y")
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
	model, _ = keyModel(model, "tab")
	model, _ = keyModel(model, "x")
	model, command := keyModel(model, "y")
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
	for _, size := range [][2]int{{1, 1}, {20, 6}, {31, 10}, {32, 7}, {32, 9}, {40, 10}, {80, 24}, {104, 24}, {120, 24}} {
		for _, state := range []string{"ready", "loading", "dirty", "failed", "stale", "sync", "locked", "empty", "confirm", "force", "deleting", "error"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				model := worktreeTestModel()
				model.width, model.height = size[0], size[1]
				model, _ = keyModel(model, "tab")
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
				case "confirm", "force":
					model, _ = keyModel(model, "x")
					model.force = state == "force"
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
	model, _ = keyModel(model, "tab")
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
		for _, mode := range []string{"wide", "narrow", "compact", "confirm", "confirm-narrow", "force", "force-narrow", "force-compact", "empty", "error", "loading", "deleting"} {
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
			if mode == "narrow" || mode == "confirm-narrow" || mode == "force-narrow" {
				model.width, model.height = 40, 16
			}
			if mode == "compact" || mode == "force-compact" {
				model.width, model.height = 28, 6
			}
			if mode == "empty" {
				for index := range model.results {
					model.results[index].Repo.Worktree = nil
				}
			}
			if mode == "confirm" || mode == "confirm-narrow" || strings.HasPrefix(mode, "force") {
				model.confirm = &model.results[1].Repo
				model.force = strings.HasPrefix(mode, "force")
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

func TestWorktreeRowsDistinguishRepositories(t *testing.T) {
	for _, expanded := range []bool{false, true} {
		t.Run(fmt.Sprintf("expanded=%t", expanded), func(t *testing.T) {
			model := worktreeTestModel()
			model.groups = nil
			model.expanded = expanded
			for index := range model.results {
				model.results[index].Repo.DisplayName = fmt.Sprintf("repo-%d", index)
				model.results[index].Repo.Path = fmt.Sprintf("/workspace/repo-%d/feature", index)
				model.results[index].Repo.Worktree = &git.Worktree{MainPath: fmt.Sprintf("/workspace/repo-%d/main", index)}
				model.results[index].Status = status.Parse("## topic\n M tracked.go\n")
			}
			model, _ = keyModel(model, "tab")
			for _, row := range model.tableRows() {
				if row.repoIndex < 0 {
					continue
				}
				if row.number == "" {
					if row.repo != "" || row.worktree != "" {
						t.Fatal("file detail must leave both identity columns empty")
					}
					continue
				}
				want := fmt.Sprintf("repo-%d", row.repoIndex)
				if row.repo != want || row.worktree != "feature" || !strings.Contains(row.status, "topic") {
					t.Fatalf("must distinguish repositories sharing a worktree basename: %+v", row)
				}
				rendered := model.renderRow(row)
				if !strings.Contains(rendered, want) || !strings.Contains(rendered, "feature") {
					t.Fatalf("both identities must render: %s", rendered)
				}
			}
			model, _ = keyModel(model, "tab")
			if strings.Contains(model.View(), "Worktree  ") || model.colWidths.worktree != 0 {
				t.Fatal("repository tab must retain its original columns")
			}
		})
	}
}

func TestWorktreeCompactIdentity(t *testing.T) {
	for _, name := range []string{"gitsy", "日本語の長いリポジトリ名"} {
		model := worktreeTestModel()
		model.width, model.height = 31, 8
		model.results[1].Repo.DisplayName = name
		model.results[1].Repo.Path = "/workspace/auth"
		model, _ = keyModel(model, "tab")
		view := model.View()
		if !strings.Contains(view, " / auth") || !strings.Contains(view, string([]rune(name)[:2])) {
			t.Fatalf("compact selection must show repository and worktree: %s", view)
		}
	}
}

func TestWorktreeColumnsFitLongUnicodeIdentities(t *testing.T) {
	for _, width := range []int{32, 40, 80, 104, 120, 240} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			model := worktreeTestModel()
			model.width = width
			model.results[1].Repo.DisplayName = strings.Repeat("日本語", 12)
			model.results[1].Repo.Path = "/workspace/" + strings.Repeat("機能", 20) + "\tfix"
			model, _ = keyModel(model, "tab")
			cols := model.colWidths
			if cols.status < 8 || cols.repo > maxRepoWidthCap || cols.worktree > maxRepoWidthCap {
				t.Fatalf("must cap identities and reserve status: %+v", cols)
			}
			for _, row := range model.tableRows() {
				if got := lipgloss.Width(model.renderRow(row)); got != model.lineWidth() {
					t.Fatalf("misaligned row: got %d, want %d", got, model.lineWidth())
				}
				if strings.Contains(row.worktree, "\t") {
					t.Fatal("worktree control characters must be escaped")
				}
			}
			if lipgloss.Width(model.View()) > width {
				t.Fatal("view must fit terminal")
			}
		})
	}
}
