package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/flexdinesh/gitsy/internal/ui"
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

// removalResult runs a removal command and requires the spinner tick batched with it.
func removalResult(t *testing.T, command tea.Cmd) tea.Msg {
	t.Helper()
	return batchedRemoval(t, command())
}

func batchedRemoval(t *testing.T, message tea.Msg) tea.Msg {
	t.Helper()
	batch, ok := message.(tea.BatchMsg)
	if !ok {
		t.Fatal("removal must batch a spinner tick")
	}
	var result tea.Msg
	ticks := 0
	for _, part := range batch {
		switch message := part().(type) {
		case spinner.TickMsg:
			ticks++
		default:
			result = message
		}
	}
	if ticks != 1 || result == nil {
		t.Fatalf("removal batch: %d ticks, result %#v", ticks, result)
	}
	return result
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
	if view := model.View(); !strings.Contains(view, "[Worktrees]") || !strings.Contains(view, "1 unique worktree") || !strings.Contains(view, "x delete") {
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

func TestWorktreeDeleteRemovesExactTargetAndUpdatesBothTabs(t *testing.T) {
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
	if command != nil || model.confirm != nil || model.deleting != "" {
		t.Fatal("X must only act in Worktrees")
	}
	model, _ = keyModel(model, "tab")
	model, command = keyModel(model, "x")
	if command == nil || model.confirm != nil || model.deleting != path || calls != 0 || !strings.Contains(model.View(), path) {
		t.Fatal("x must start async removal without confirmation and show target")
	}
	for _, key := range []string{"x", "y", "down", "tab"} {
		var ignored tea.Cmd
		model, ignored = keyModel(model, key)
		if ignored != nil || !model.worktrees || model.selected != 0 {
			t.Fatal("deletion must freeze selection and not start duplicates")
		}
	}
	updated, _ := model.Update(removalResult(t, command))
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
			protected := true
			model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
				return git.WorktreeRemovalResult{Result: git.Result{OK: !protected, Stderr: "Worktree is protected"}}
			}
			model, command := keyModel(model, "x")
			if state == "loading" {
				if command != nil || model.deleting != "" || model.notice == "" {
					t.Fatal("must wait for inspections to avoid concurrent Git operations")
				}
				return
			}
			if command == nil {
				t.Fatal("deletion must check fresh Git state")
			}
			updated, _ := model.Update(removalResult(t, command))
			model = updated.(Model)
			if len(model.visibleResults()) != 1 || !strings.Contains(model.notice, "protected") {
				t.Fatal("protected worktree must remain and show the fresh refusal")
			}
			protected = false
			model, command = keyModel(model, "x")
			if command == nil {
				t.Fatal("external cleanup must permit retry despite cached status")
			}
			updated, _ = model.Update(removalResult(t, command))
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

func TestWorktreeDeletionShowsRowProgress(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 24}, {90, 24}, {30, 8}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			model := worktreeTestModel()
			model.width, model.height = size.width, size.height
			model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
				return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
			}
			model, _ = keyModel(model, "tab")
			header := model.headerRight()
			model, command := keyModel(model, "x")
			view := model.View()
			if len(model.actionLines()) != 0 || !strings.Contains(view, "deleting…") || strings.Contains(view, "Deleting worktree") ||
				!strings.Contains(view, "Deleting… · q quit") || model.headerRight() != header {
				t.Fatalf("row must carry deletion progress without an action bar:\n%s", view)
			}
			if size.width == 90 && (model.detailHeight(size.width) != 1 || !strings.Contains(view, "/tmp/web · origin/feature")) {
				t.Fatalf("detail strip must stay during removal:\n%s", view)
			}
			if size.width >= minWidth && !strings.Contains(view, model.spin.View()+" deleting…") {
				t.Fatalf("ledger row must show the spinner:\n%s", view)
			}
			if _, tick := model.Update(model.spin.Tick()); tick == nil {
				t.Fatal("spinner must tick while deleting")
			}
			updated, _ := model.Update(removalResult(t, command))
			model = updated.(Model)
			if _, tick := model.Update(model.spin.Tick()); tick != nil || strings.Contains(model.View(), "deleting…") {
				t.Fatal("spinner and row progress must stop after removal")
			}
		})
	}
}

func TestWorktreeForceDeleteRequiresConfirmation(t *testing.T) {
	for _, cancel := range []string{"", "esc", "n", "N"} {
		t.Run("cancel="+cancel, func(t *testing.T) {
			model := worktreeTestModel()
			target := model.results[1].Repo
			calls := 0
			model.remove = func(_ context.Context, owner, path string, force bool) git.WorktreeRemovalResult {
				calls++
				if owner != target.Worktree.MainPath || path != target.Path || force != (calls == 2) {
					t.Fatal("must revalidate same target, forcing only after Y")
				}
				if !force {
					return git.WorktreeRemovalResult{NeedsForce: true}
				}
				return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
			}
			model, _ = keyModel(model, "tab")
			model, command := keyModel(model, "x")
			updated, _ := model.Update(removalResult(t, command))
			model = updated.(Model)
			view := model.View()
			if calls != 1 || model.confirm == nil || model.deleting != "" ||
				!strings.Contains(view, target.Path) || !strings.Contains(view, "Discards changed, untracked, ignored, and submodule files") ||
				!strings.Contains(view, "y force") || strings.Contains(view, "deleting…") || len(model.visibleResults()) != 1 {
				t.Fatalf("changed files must show second warning and retain target: %s", view)
			}
			for _, key := range []string{"tab", "down", "f", "x", "enter"} {
				model, command = keyModel(model, key)
				if command != nil || !model.worktrees || model.selected != 0 || model.confirm == nil {
					t.Fatal("force confirmation must freeze navigation and ignore unrelated keys")
				}
			}
			if cancel != "" {
				model, _ = keyModel(model, cancel)
				if model.confirm != nil || calls != 1 || len(model.visibleResults()) != 1 {
					t.Fatal("cancellation must preserve worktree")
				}
				model, command = keyModel(model, "x")
				if command == nil || model.deleting != target.Path || model.confirm != nil {
					t.Fatal("retry must start with normal deletion")
				}
				return
			}
			model, command = keyModel(model, "Y")
			if command == nil || calls != 1 {
				t.Fatal("Y must start async force deletion")
			}
			updated, _ = model.Update(removalResult(t, command))
			model = updated.(Model)
			if calls != 2 || len(model.displayResults()) != 0 || model.confirm != nil {
				t.Fatal("force deletion must remove all appearances")
			}
		})
	}
}

func TestWorktreeShiftXForceDeletesWithoutConfirmation(t *testing.T) {
	model := worktreeTestModel()
	target := model.results[1].Repo
	model.results[1].Status = status.Parse("## feature\n?? untracked\n")
	calls := 0
	model.remove = func(_ context.Context, owner, path string, force bool) git.WorktreeRemovalResult {
		calls++
		if owner != target.Worktree.MainPath || path != target.Path || !force {
			t.Fatal("X must force remove the selected worktree")
		}
		return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
	}
	model, command := keyModel(model, "X")
	if command != nil || model.deleting != "" {
		t.Fatal("X must only act in Worktrees")
	}
	model, _ = keyModel(model, "tab")
	model, command = keyModel(model, "X")
	if command == nil || model.confirm != nil || model.deleting != target.Path || calls != 0 {
		t.Fatal("X must start async force removal without confirmation")
	}
	updated, _ := model.Update(removalResult(t, command))
	model = updated.(Model)
	if calls != 1 || model.confirm != nil || len(model.displayResults()) != 0 || !strings.Contains(model.View(), "Branch kept") {
		t.Fatal("X must remove dirty worktree without confirmation")
	}
}

func TestWorktreeShiftXKeepsProtectionChecks(t *testing.T) {
	model := worktreeTestModel()
	model.remove = func(_ context.Context, _, _ string, force bool) git.WorktreeRemovalResult {
		if !force {
			t.Fatal("X must request force")
		}
		return git.WorktreeRemovalResult{Result: git.Result{Stderr: "Worktree is locked; unlock it before deleting."}}
	}
	model, _ = keyModel(model, "tab")
	model, command := keyModel(model, "X")
	updated, _ := model.Update(removalResult(t, command))
	model = updated.(Model)
	if len(model.visibleResults()) != 1 || model.confirm != nil || !strings.Contains(model.notice, "locked") {
		t.Fatal("force deletion refusal must retain row and explain problem")
	}
}

func TestForceConfirmationRequiresVisibleWarningAndPath(t *testing.T) {
	model := worktreeTestModel()
	model.width, model.height = 20, 6
	model, _ = keyModel(model, "tab")
	model.confirm = &model.results[1].Repo
	model, command := keyModel(model, "y")
	if command != nil || model.canConfirm() || !strings.Contains(model.View(), "Enlarge terminal") {
		t.Fatal("force confirmation must wait until full warning and path fit")
	}
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 40, Height: 16})
	model = updated.(Model)
	if !model.canConfirm() || strings.Join(model.actionLines(), " ") !=
		"Force delete worktree? Discards changed, untracked, ignored, and submodule files. Branch kept. "+model.confirm.Path {
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
	model, command := keyModel(model, "x")
	updated, _ := model.Update(removalResult(t, command))
	model = updated.(Model)
	if len(model.visibleResults()) != 1 || !strings.Contains(model.View(), "Delete failed: worktree has changes") {
		t.Fatal("failure must retain row and explain problem")
	}
	model, command = keyModel(model, "x")
	if command == nil {
		t.Fatal("failure must allow retry")
	}
}

func TestWorktreeConfirmationRequiresVisiblePath(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.Path = "/workspace/" + strings.Repeat("long-directory/", 12) + "target"
	model.width, model.height = 20, 6
	model, _ = keyModel(model, "tab")
	model.confirm = &model.results[1].Repo
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
		for _, state := range []string{"ready", "loading", "dirty", "failed", "stale", "sync", "locked", "empty", "force", "deleting", "error"} {
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
					model.results[1].Sync = &inspect.SyncOutcome{Kind: "synced", Pulled: 1}
				case "locked":
					model.results[1].Repo.Worktree.Locked = true
				case "empty":
					model.results[1].Repo.Worktree = nil
				case "force":
					model.confirm = &model.results[1].Repo
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
		for _, mode := range []string{"wide", "narrow", "compact", "force", "force-narrow", "force-compact", "empty", "error", "loading", "deleting"} {
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
			if mode == "narrow" || mode == "force-narrow" {
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
			if strings.HasPrefix(mode, "force") {
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

func TestKeyBurstNeverDeletesOrConfirms(t *testing.T) {
	for _, keys := range []string{"xx", "XX", "jx", "yY", "xy"} {
		for _, confirming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/confirm=%t", keys, confirming), func(t *testing.T) {
				model := worktreeTestModel()
				model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
					t.Fatal("burst must not remove worktrees")
					return git.WorktreeRemovalResult{}
				}
				model, _ = keyModel(model, "tab")
				if confirming {
					model.confirm = &model.results[1].Repo
				}
				updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keys)})
				model = updated.(Model)
				if command != nil {
					if message := command(); message != nil {
						t.Fatalf("burst started work: %#v", message)
					}
				}
				if model.deleting != "" || (model.confirm != nil) != confirming {
					t.Fatal("burst must leave deletion state unchanged")
				}
			})
		}
	}
}

func TestDeleteNoticeNamesWorktree(t *testing.T) {
	for _, test := range []struct {
		path, want string
	}{
		{"/workspace/repo-clean-wt", "Deleted repo-clean-wt. Branch kept."},
		{"/workspace/" + strings.Repeat("x", 40), "Deleted " + strings.Repeat("x", maxRepoWidthCap-1) + "…. Branch kept."},
	} {
		model := worktreeTestModel()
		model.results[1].Repo.Path = test.path
		model.remove = func(context.Context, string, string, bool) git.WorktreeRemovalResult {
			return git.WorktreeRemovalResult{Result: git.Result{OK: true}}
		}
		model, _ = keyModel(model, "tab")
		model, command := keyModel(model, "x")
		updated, _ := model.Update(removalResult(t, command))
		if got := updated.(Model).notice; got != test.want {
			t.Fatalf("notice = %q, want %q", got, test.want)
		}
	}
}

func TestEnlargeInstructionStaysReadable(t *testing.T) {
	for _, size := range [][2]int{{20, 6}, {24, 8}, {31, 8}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			model := worktreeTestModel()
			model.results[1].Repo.Path = "/workspace/" + strings.Repeat("long-directory/", 6) + "target"
			model.width, model.height = size[0], size[1]
			model, _ = keyModel(model, "tab")
			model.confirm = &model.results[1].Repo
			model.refresh()
			view := model.View()
			if model.canConfirm() || !strings.Contains(strings.Join(strings.Fields(view), " "), "Enlarge terminal to review path.") {
				t.Fatalf("enlargement instruction must be fully readable:\n%s", view)
			}
			if !strings.HasSuffix(strings.TrimRight(strings.Split(view, "\n")[lipgloss.Height(view)-1], " "), "esc cancel") {
				t.Fatalf("cancellation must remain on the last line:\n%s", view)
			}
		})
	}
}

func TestConfirmationWrapsWordsAndPathSegments(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.Path = "/workspace/team/feature-auth"
	model.width, model.height = 40, 24
	model, _ = keyModel(model, "tab")
	model.confirm = &model.results[1].Repo
	model.refresh()
	want := []string{"Force delete worktree? Discards", "changed, untracked, ignored, and", "submodule files. Branch kept.", "/workspace/team/feature-auth"}
	if got := model.actionLines(); strings.Join(got, "|") != strings.Join(want, "|") || !model.canConfirm() {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCompactViewPinsFooterToLastLine(t *testing.T) {
	for _, size := range [][2]int{{20, 6}, {31, 8}, {31, 10}, {40, 8}} {
		for _, state := range []string{"repos", "worktrees", "empty", "notice", "deleting"} {
			t.Run(fmt.Sprintf("%dx%d/%s", size[0], size[1], state), func(t *testing.T) {
				model := worktreeTestModel()
				model.width, model.height = size[0], size[1]
				if state != "repos" {
					model, _ = keyModel(model, "tab")
				}
				switch state {
				case "empty":
					model.results[1].Repo.Worktree = nil
				case "notice":
					model.notice = "Delete failed: locked"
				case "deleting":
					model.deleting = model.results[1].Repo.Path
				}
				model.refresh()
				view := model.View()
				lines := strings.Split(view, "\n")
				if len(lines) != size[1] || strings.TrimSpace(lines[len(lines)-1]) != strings.TrimSpace(model.footerPlain(size[0])) {
					t.Fatalf("footer must sit on line %d:\n%s", size[1], view)
				}
			})
		}
	}
}

func TestCompactSummaryDropsWholeSegmentsByPriority(t *testing.T) {
	parts := []string{"27 repos", "1 failed", "17 changed", "2 behind", "1 fetch failed"}
	for _, test := range []struct {
		width int
		want  string
	}{
		{80, "gitsy · 27 repos · 1 failed · 17 changed · 2 behind · 1 fetch failed"},
		{53, "27 repos · 1 failed · 2 behind · 1 fetch failed"},
		{40, "27 repos · 1 failed · 2 behind"},
		{30, "27 repos · 1 failed · 2 behind"},
		{24, "27 repos · 1 failed"},
		{10, "27 repos"},
		{5, "27 r…"},
	} {
		if got := compactSummary(parts, test.width); got != test.want {
			t.Fatalf("width %d: got %q, want %q", test.width, got, test.want)
		}
	}
	if got := compactSummary([]string{"28 repos"}, 24); got != "gitsy · 28 repos" {
		t.Fatalf("compact summary must keep the title when it fits: %q", got)
	}
}

func TestContextRailShowsRemoteStateOnceAndAbbreviatesPaths(t *testing.T) {
	github := inspect.Remotes{Known: true, Count: 1, OriginURL: "https://github.com/o/r"}
	for _, test := range []struct {
		name   string
		result func(*ui.RepoResult)
		want   []string
		once   string
	}{
		{"fetch failed", func(result *ui.RepoResult) {
			result.Stale, result.Reason, result.Remotes = true, inspect.ReasonAuth, github
		}, []string{"Remote", "origin · github.com", "Hint: run gh auth setup-git, or switch origin to SSH"}, "remote auth needed"},
		{"no upstream", func(result *ui.RepoResult) {
			result.Status, result.Remotes = status.Parse("## feature\n"), github
		}, []string{"Remote", "origin · github.com"}, "No upstream"},
		{"local only", func(result *ui.RepoResult) {
			result.Status, result.Remotes = status.Parse("## feature\n"), inspect.Remotes{Known: true}
		}, []string{"Remote"}, "No remote"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := worktreeTestModel()
			model.home = "/home/user"
			model.results[1].Repo.Path = "/home/user/work/feature"
			model.results[1].Repo.Worktree.MainPath = "/home/user/work/main"
			test.result(&model.results[1])
			model, _ = keyModel(model, "tab")
			info := strings.Join(model.infoLines(80), "\n")
			for _, want := range test.want {
				if !strings.Contains(info, want) {
					t.Fatalf("rail lost %q:\n%s", want, info)
				}
			}
			if strings.Count(info, test.once) != 1 || strings.Contains(strings.ToLower(info), "skipped") {
				t.Fatalf("rail must show %q once without skipped-sync text:\n%s", test.once, info)
			}
			if !strings.Contains(info, "~/work/feature") || !strings.Contains(info, "~/work/main") || strings.Contains(info, "/home/user") {
				t.Fatalf("rail paths must use ~:\n%s", info)
			}
		})
	}
}

func TestEmptyWorktreesHideDeleteHintsAndRailDuplicate(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.Worktree = nil
	model, _ = keyModel(model, "tab")
	view := model.View()
	if strings.Contains(view, "x delete") || strings.Contains(view, "X force") {
		t.Fatalf("delete hints need a selectable worktree:\n%s", view)
	}
	info := strings.Join(model.infoLines(30), "\n")
	if strings.Contains(info, "No linked worktrees found.") || !strings.Contains(info, "Nothing selected.") {
		t.Fatalf("rail must not repeat the ledger explanation:\n%s", info)
	}
	if !strings.Contains(view, "No linked worktrees found.") {
		t.Fatalf("ledger must keep the explanation:\n%s", view)
	}
}

func TestWorktreeCountsUseSingularNouns(t *testing.T) {
	model := worktreeTestModel()
	model.groups = model.groups[1:2]
	model, _ = keyModel(model, "tab")
	view := model.View()
	if !strings.Contains(view, "1 worktree ") || !strings.Contains(view, "1/1 worktree ") || strings.Contains(view, "1 worktrees") {
		t.Fatalf("single worktree counts must be singular:\n%s", view)
	}
}

func TestDetailStripShowsSelectedContextWhenRailHidden(t *testing.T) {
	for _, test := range []struct {
		name   string
		width  int
		reason string
		want   string
	}{
		{"path and upstream", 80, "", "~/work/feature · origin/feature"},
		{"reason before upstream", 80, "fetch failed: auth", "~/work/feature · fetch failed: auth · origin/feature"},
		{"narrow keeps reason", 40, "fetch timed out after 30s", "…feature · fetch timed out after 30s"},
		{"rail shown", 120, "fetch failed: auth", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := worktreeTestModel()
			model.home = "/home/user"
			model.results[1].Repo.Path = "/home/user/work/feature"
			model.results[1].Reason = test.reason
			model.results[1].Stale = test.reason != ""
			model.width, model.height = test.width, 24
			model, _ = keyModel(model, "tab")
			lines := strings.Split(model.View(), "\n")
			if len(lines) != 24 {
				t.Fatalf("view must fill the terminal: %d lines", len(lines))
			}
			strip := strings.TrimSpace(lines[22])
			if test.want == "" {
				if model.detailHeight(test.width) != 0 || strings.Contains(strip, "fetch failed") {
					t.Fatalf("rail layouts must not add a strip: %q", strip)
				}
				return
			}
			if strip != test.want {
				t.Fatalf("strip = %q, want %q", strip, test.want)
			}
		})
	}
}

func TestDetailStripKeepsLayoutStableWithoutSelection(t *testing.T) {
	model := worktreeTestModel()
	model.width, model.height = 80, 24
	model, _ = keyModel(model, "tab")
	capacity := model.capacity
	model.results[1].Repo.Worktree = nil
	model.refresh()
	lines := strings.Split(model.View(), "\n")
	if model.capacity != capacity || strings.TrimSpace(lines[22]) != "" {
		t.Fatalf("empty selection must keep the reserved strip blank: capacity %d->%d %q", capacity, model.capacity, lines[22])
	}
	model.results[1].Repo.Worktree = &git.Worktree{MainPath: model.results[0].Repo.Path}
	model.confirm = &model.results[1].Repo
	model.refresh()
	if model.detailHeight(80) != 0 {
		t.Fatal("confirmation replaces the strip")
	}
}

func TestShortReasonAppearsInRailAndCompactView(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Stale = true
	model.results[1].Reason = "remote unreachable"
	model, _ = keyModel(model, "tab")
	if info := strings.Join(model.infoLines(30), "\n"); strings.Count(info, "remote unreachable") != 1 {
		t.Fatalf("rail must show the short reason once:\n%s", info)
	}
	model.width, model.height = 31, 8
	model.refresh()
	if view := model.View(); !strings.Contains(view, "remote unreachable") {
		t.Fatalf("compact view must show the short reason:\n%s", view)
	}
}

func TestRepositoriesTabTagsLinkedWorktrees(t *testing.T) {
	model := worktreeTestModel()
	model.results[1].Repo.DisplayName = "api"
	model.results[1].Repo.Path = "/workspace/spike-wt"
	model.refresh()
	view := model.View()
	if !strings.Contains(view, "api · spike-wt") {
		t.Fatalf("linked worktree rows need a worktree tag:\n%s", view)
	}
	model, _ = keyModel(model, "tab")
	if strings.Contains(model.View(), "api · spike-wt") {
		t.Fatal("Worktrees tab already has a Worktree column")
	}
}

func TestIdentityPartsKeepNameFirst(t *testing.T) {
	for _, test := range []struct {
		name, tag string
		width     int
		want      string
	}{
		{"repo-ahead", "spike-wt", 28, "repo-ahead · spike-wt"},
		{"repo-ahead", "spike-wt", 18, "repo-ahead · spik…"},
		{"a-very-long-repository-name", "spike-wt", 20, "a-very-long-… · spi…"},
		{"repo", "", 10, "repo"},
		{"repo-ahead", "spike-wt", 7, "repo-a…"},
	} {
		name, tag := identityParts(test.name, test.tag, test.width)
		if got := name + tag; got != test.want || lipgloss.Width(got) > test.width {
			t.Fatalf("identityParts(%q, %q, %d) = %q, want %q", test.name, test.tag, test.width, got, test.want)
		}
	}
}

func TestWorktreeFooterHidesForceHint(t *testing.T) {
	model := worktreeTestModel()
	model, _ = keyModel(model, "tab")
	for _, width := range []int{40, 80, 120, 200} {
		if footer := model.footerPlain(width); strings.Contains(footer, "X") {
			t.Fatalf("footer must not advertise X at %d: %q", width, footer)
		}
	}
}
