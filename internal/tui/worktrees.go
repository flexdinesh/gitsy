package tui

import (
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/ui"
	"github.com/mattn/go-runewidth"
)

type navigationState struct {
	selected    int
	offset      int
	browseEmpty bool
}

type worktreeRemovedMsg struct {
	repo   discover.Repo
	result git.WorktreeRemovalResult
}

func (model Model) includes(result ui.RepoResult) bool {
	return !model.removed[result.Repo.Path] && (!model.worktrees || result.Repo.Worktree != nil)
}

func (model Model) visibleResults() []ui.RepoResult {
	results := []ui.RepoResult{}
	for _, result := range model.results {
		if model.includes(result) {
			results = append(results, result)
		}
	}
	return results
}

func (model Model) displayGroups() []discover.Group {
	groups := make([]discover.Group, len(model.groups))
	for index, group := range model.groups {
		groups[index].Path = group.Path
		for _, repoIndex := range group.RepoIndexes {
			if model.includes(model.results[repoIndex]) {
				groups[index].RepoIndexes = append(groups[index].RepoIndexes, repoIndex)
			}
		}
	}
	return groups
}

func (model *Model) switchTab() {
	previous := navigationState{model.selected, model.offset, model.browseEmpty}
	model.selected, model.offset, model.browseEmpty = model.inactive.selected, model.inactive.offset, model.inactive.browseEmpty
	model.inactive = previous
	model.worktrees = !model.worktrees
	model.notice = ""
	model.refresh()
}

func (model *Model) worktreeKey(key tea.KeyMsg) (bool, tea.Cmd) {
	if model.deleting != "" {
		return true, nil
	}
	if model.confirm != nil {
		switch key.String() {
		case "esc", "n", "N":
			model.confirm = nil
			model.force = false
			model.refresh()
		case "y", "Y":
			if !model.canConfirm() {
				return true, nil
			}
			repo := *model.confirm
			force := model.force
			model.force = false
			model.confirm = nil
			model.deleting = repo.Path
			model.refresh()
			return true, func() tea.Msg {
				return worktreeRemovedMsg{repo: repo, result: model.remove(model.ctx, repo.Worktree.MainPath, repo.Path, force)}
			}
		}
		return true, nil
	}
	switch key.String() {
	case "tab", "shift+tab", "left", "right":
		model.switchTab()
		return true, nil
	case "esc":
		model.notice = ""
		model.refresh()
		return true, nil
	case "x", "X":
		if !model.worktrees || model.browseEmpty || len(model.displayResults()) == 0 {
			return true, nil
		}
		result := model.selectedResult()
		switch {
		case model.pending() > 0:
			model.notice = "Wait for inspections to finish before deleting."
		default:
			repo := result.Repo
			model.notice = ""
			model.confirm = &repo
			model.force = false
		}
		model.refresh()
		return true, nil
	}
	return false, nil
}

func (model Model) tabPlain() string {
	if model.worktrees {
		return "Repositories  [Worktrees]"
	}
	return "[Repositories]  Worktrees"
}

func (model Model) renderTabs(width int) string {
	first, second := "[Repositories]", "Worktrees"
	if model.worktrees {
		first, second = "Repositories", "[Worktrees]"
	}
	if len(first)+len(second)+2 > width {
		return selectedMarkerStyle().Render(truncateCell(model.tabPlain(), width))
	}
	firstStyle, secondStyle := selectedMarkerStyle(), headerMetaStyle()
	if model.worktrees {
		firstStyle, secondStyle = secondStyle, firstStyle
	}
	return firstStyle.Render(first) + "  " + secondStyle.Render(second) + headerMetaStyle().Render(truncateCell("  tab", max(0, width-len(first)-len(second)-2)))
}

func (model Model) emptyMessage() string {
	if model.worktrees {
		return "No linked worktrees found."
	}
	return ui.EmptyMessage(0)
}

func (model Model) actionLines() []string {
	width := model.width
	if width == 0 {
		width = 80
	}
	content := max(1, width-spaceSM*2)
	if model.confirm != nil {
		path := displayGroupPath(model.confirm.Path, model.home)
		lines := model.confirmationLines(content)
		if !model.canConfirm() {
			return []string{"Enlarge terminal to review path.", runewidth.TruncateLeft(path, max(0, runewidth.StringWidth(path)-content), "…")}
		}
		return lines
	}
	if model.deleting != "" {
		return []string{"Deleting worktree…", displayGroupPath(model.deleting, model.home)}
	}
	if model.notice != "" {
		lines := wrapPlain(displayGroupPath(strings.Join(strings.Fields(model.notice), " "), ""), content)
		return lines[:min(len(lines), 3)]
	}
	return nil
}

func (model Model) confirmationLines(content int) []string {
	prompt := "Delete worktree? Branch kept."
	if model.force {
		prompt = "Force delete worktree? Discards changed, untracked, and ignored files. Branch kept."
	}
	return append(wrapPlain(prompt, content), wrapPlain(displayGroupPath(model.confirm.Path, model.home), content)...)
}

func (model Model) canConfirm() bool {
	width, height := model.width, model.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	if model.confirm == nil {
		return false
	}
	lines := model.confirmationLines(max(1, width-spaceSM*2))
	return width >= 20 && height >= 4 && len(lines)+1 <= height
}

func (model Model) actionHeight() int { return len(model.actionLines()) }

func (model Model) viewportHeight(height int) int {
	return max(1, tableViewportHeight(height)-model.actionHeight())
}

func (model Model) renderAction(width int) string {
	lines := model.actionLines()
	tone := "yellow"
	if model.force {
		tone = "red"
	}
	for index := range lines {
		lines[index] = toneStyle(tone, index == 0, false).Render(truncateCell(lines[index], max(1, width-spaceSM*2)))
	}
	if len(lines) == 0 {
		return ""
	}
	return headerBarStyle(width).Render(strings.Join(lines, "\n"))
}

func (model Model) columns(width int) tableColumns {
	results := model.displayResults()
	number, repo, status := columnWidths(width, results)
	if !model.worktrees {
		return tableColumns{number: number, repo: repo, status: status}
	}
	content := width - number - columnGap*3
	repo, worktree := runewidth.StringWidth("Repository"), runewidth.StringWidth("Worktree")
	for _, result := range results {
		repo = max(repo, runewidth.StringWidth(result.Repo.DisplayName))
		worktree = max(worktree, runewidth.StringWidth(displayGroupPath(filepath.Base(result.Repo.Path), "")))
	}
	repo = min(repo, maxRepoWidthCap, max(4, content/4))
	worktree = min(worktree, maxRepoWidthCap, max(4, content/3))
	return tableColumns{number: number, repo: repo, worktree: worktree, status: max(1, content-repo-worktree)}
}

func worktreeIdentity(repo discover.Repo, width int) string {
	name := displayGroupPath(filepath.Base(repo.Path), "")
	repoWidth := min(runewidth.StringWidth(repo.DisplayName), max(1, (width-3)/2))
	return truncateCell(repo.DisplayName, repoWidth) + " / " + truncateCell(name, max(1, width-repoWidth-3))
}
