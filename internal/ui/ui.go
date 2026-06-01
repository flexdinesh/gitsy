package ui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/status"
	"github.com/mattn/go-runewidth"
)

type SyncOutcome struct {
	Kind   string
	Pulled int
	Reason string
}

type RepoResult struct {
	Repo        discover.Repo
	Status      status.Parsed
	Failed      bool
	Stale       bool
	Loading     bool
	LoadingText string
	Sync        *SyncOutcome
}

type Row struct {
	Kind string
	Repo string
	Text string
	Tone string
	Bold bool
	Dim  bool
}

type Layout struct {
	Width       int
	RepoWidth   int
	StatusWidth int
}

type categoryStyle struct {
	icon  string
	label string
	tone  string
	bold  bool
}

var categoryStyles = map[status.Category]categoryStyle{
	status.Modified:  {icon: "●", label: "modified", tone: "yellow"},
	status.Staged:    {icon: "◆", label: "staged", tone: "green"},
	status.Untracked: {icon: "+", label: "untracked", tone: "green"},
	status.Deleted:   {icon: "✖", label: "deleted", tone: "red"},
	status.Renamed:   {icon: "➜", label: "renamed", tone: "magenta"},
	status.Conflict:  {icon: "‼", label: "conflict", tone: "red", bold: true},
	status.Other:     {icon: "•", label: "changed", tone: "white"},
}

func Render(writer io.Writer, results []RepoResult, showAll bool, totalDiscovered int, emptyMessage string) {
	layout := CreateLayout(terminalWidth(), reposFromResults(results))
	rows := BuildRows(results, showAll)
	title := Title(results, showAll, totalDiscovered)

	fmt.Fprintln(writer, style("gray", false, false).Render(topBorder(layout)))
	fmt.Fprintln(writer, style("cyan", true, false).Render(titleLine(layout, title)))
	fmt.Fprintln(writer, style("gray", false, false).Render(divider(layout)))
	fmt.Fprintln(writer, style("white", true, false).Render(dataLine(layout, "REPO", "STATUS")))
	fmt.Fprintln(writer, style("gray", false, false).Render(divider(layout)))

	if len(rows) == 0 {
		if emptyMessage == "" {
			emptyMessage = "No repositories to display."
		}
		row := Row{Kind: "data", Text: emptyMessage, Tone: "yellow"}
		fmt.Fprintln(writer, renderRow(layout, row))
	} else {
		for _, row := range rows {
			fmt.Fprintln(writer, renderRow(layout, row))
		}
	}

	fmt.Fprintln(writer, style("gray", false, false).Render(bottomBorder(layout)))
}

func RenderString(results []RepoResult, showAll bool, totalDiscovered int, emptyMessage string) string {
	var buffer bytes.Buffer
	Render(&buffer, results, showAll, totalDiscovered, emptyMessage)
	return buffer.String()
}

func EmptyMessage(totalDiscovered int, showAll bool) string {
	if totalDiscovered == 0 {
		return "No child git repositories found."
	}
	if !showAll {
		return "No child git repositories with changes or branch divergence found."
	}
	return ""
}

func Title(results []RepoResult, showAll bool, totalDiscovered int) string {
	return fmt.Sprintf("gitsy • %d/%d %s", countVisible(results, showAll), totalDiscovered, filterLabel(showAll))
}

func CreateLayout(terminalWidth int, repos []discover.Repo) Layout {
	width := clamp(terminalWidth, 60, 140)
	return createLayout(width, repos)
}

func CreateFullWidthLayout(width int, results []RepoResult) Layout {
	return createLayout(max(width, 20), reposFromResults(results))
}

func RenderHeader(layout Layout) string {
	return contentLine(layout, "REPO", "STATUS")
}

func RenderRows(layout Layout, results []RepoResult, showAll bool, emptyMessage string) []string {
	rows := BuildRows(results, showAll)
	if len(rows) == 0 {
		if emptyMessage == "" {
			emptyMessage = "No repositories to display."
		}
		rows = []Row{{Kind: "data", Text: emptyMessage, Tone: "yellow"}}
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Kind == "separator" {
			lines = append(lines, style("gray", false, false).Render(contentDivider(layout)))
			continue
		}
		lines = append(lines, renderContentRow(layout, row))
	}
	return lines
}

func createLayout(width int, repos []discover.Repo) Layout {
	longestRepoName := 4
	for _, repo := range repos {
		longestRepoName = max(longestRepoName, runewidth.StringWidth(repo.DisplayName))
	}
	repoWidth := clamp(longestRepoName, min(18, max(4, width*35/100)), max(4, width*35/100))
	statusWidth := max(4, width-repoWidth-7)
	return Layout{
		Width:       repoWidth + statusWidth + 7,
		RepoWidth:   repoWidth,
		StatusWidth: statusWidth,
	}
}

func BuildRows(results []RepoResult, showAll bool) []Row {
	rows := []Row{}
	hasVisibleRow := false

	for _, result := range results {
		repoRows := rowsForRepo(result, showAll)
		if len(repoRows) == 0 {
			continue
		}
		if hasVisibleRow {
			rows = append(rows, Row{Kind: "separator"})
		}
		rows = append(rows, repoRows...)
		hasVisibleRow = true
	}

	return rows
}

func rowsForRepo(result RepoResult, showAll bool) []Row {
	if result.Loading {
		text := "⏳ fetching status…"
		if result.LoadingText != "" {
			text = fmt.Sprintf("%s fetching status...", result.LoadingText)
		}
		return []Row{{
			Kind: "data",
			Repo: result.Repo.DisplayName,
			Text: text,
			Tone: "cyan",
			Bold: true,
		}}
	}

	if !result.Status.Changed && !showAll && (result.Sync == nil || result.Sync.Kind != "synced") {
		return nil
	}

	rows := []Row{}
	summary := FormatBranchSummary(result.Status)
	if result.Stale {
		summary.Text += " ⚠ stale"
		summary.Tone = "yellow"
		summary.Dim = false
	}
	if result.Failed {
		summary.Text += " ⚠ status failed"
		summary.Tone = "yellow"
		summary.Dim = false
	}

	if result.Sync != nil {
		switch result.Sync.Kind {
		case "synced":
			summary.Text += fmt.Sprintf(" ⤓ synced ↓%d", result.Sync.Pulled)
			summary.Tone = "green"
			summary.Dim = false
		case "failed":
			summary.Text += " ⚠ sync failed"
			summary.Tone = "yellow"
			summary.Dim = false
		}
	}

	rows = append(rows, Row{
		Kind: "data",
		Repo: result.Repo.DisplayName,
		Text: summary.Text,
		Tone: summary.Tone,
		Bold: true,
		Dim:  summary.Dim,
	})

	if len(result.Status.Items) == 0 {
		if !result.Status.Changed {
			rows = append(rows, Row{Kind: "data", Text: "✓ clean", Tone: "green", Dim: true})
		}
		return rows
	}

	for _, item := range result.Status.Items {
		rows = append(rows, formatItemRow(item))
	}
	return rows
}

type BranchSummary struct {
	Text string
	Tone string
	Dim  bool
}

func FormatBranchSummary(parsed status.Parsed) BranchSummary {
	branch := parsed.Branch
	if branch == nil {
		if parsed.Changed {
			return BranchSummary{Text: "changes", Tone: "yellow"}
		}
		return BranchSummary{Text: "✓ clean", Tone: "green", Dim: true}
	}

	parts := []string{branch.Name}
	if branch.Name == "" {
		parts[0] = "detached"
	}
	if branch.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", branch.Ahead))
	}
	if branch.Behind > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", branch.Behind))
	}
	if branch.Gone {
		parts = append(parts, "⚠ upstream gone")
	}
	if branch.Metadata != "" && branch.Ahead == 0 && branch.Behind == 0 && !branch.Gone {
		parts = append(parts, fmt.Sprintf("[%s]", branch.Metadata))
	}

	if !parsed.Changed {
		parts = append(parts, "✓ clean")
		return BranchSummary{Text: strings.Join(parts, " "), Tone: "green", Dim: true}
	}
	tone := "blue"
	if branch.Gone {
		tone = "yellow"
	}
	return BranchSummary{Text: strings.Join(parts, " "), Tone: tone}
}

func FormatCell(value string, width int) string {
	return padEndVisible(truncateVisible(value, width), width)
}

func topBorder(layout Layout) string {
	return "╭" + strings.Repeat("─", layout.RepoWidth+2) + "┬" + strings.Repeat("─", layout.StatusWidth+2) + "╮"
}

func divider(layout Layout) string {
	return "├" + strings.Repeat("─", layout.RepoWidth+2) + "┼" + strings.Repeat("─", layout.StatusWidth+2) + "┤"
}

func bottomBorder(layout Layout) string {
	return "╰" + strings.Repeat("─", layout.RepoWidth+2) + "┴" + strings.Repeat("─", layout.StatusWidth+2) + "╯"
}

func titleLine(layout Layout, text string) string {
	innerWidth := layout.RepoWidth + layout.StatusWidth + 3
	return "│ " + FormatCell(text, innerWidth) + " │"
}

func dataLine(layout Layout, repo string, text string) string {
	return "│ " + FormatCell(repo, layout.RepoWidth) + " │ " + FormatCell(text, layout.StatusWidth) + " │"
}

func contentLine(layout Layout, repo string, text string) string {
	return FormatCell(repo, layout.RepoWidth) + " │ " + FormatCell(text, layout.StatusWidth)
}

func contentDivider(layout Layout) string {
	return strings.Repeat("─", layout.RepoWidth) + "─┼─" + strings.Repeat("─", layout.StatusWidth)
}

func renderRow(layout Layout, row Row) string {
	if row.Kind == "separator" {
		return style("gray", false, false).Render(divider(layout))
	}
	return style(row.Tone, row.Bold, row.Dim).Render(dataLine(layout, row.Repo, row.Text))
}

func renderContentRow(layout Layout, row Row) string {
	return style(row.Tone, row.Bold, row.Dim).Render(contentLine(layout, row.Repo, row.Text))
}

func formatItemRow(item status.Item) Row {
	itemStyle := categoryStyles[item.Category]
	return Row{
		Kind: "data",
		Text: fmt.Sprintf("%s %s %s", itemStyle.icon, itemStyle.label, formatItemPath(item)),
		Tone: itemStyle.tone,
		Bold: itemStyle.bold,
	}
}

func formatItemPath(item status.Item) string {
	if item.Category == status.Renamed {
		return strings.ReplaceAll(item.Path, " -> ", " → ")
	}
	if item.Path != "" {
		return item.Path
	}
	return item.Raw
}

func truncateVisible(value string, maxWidth int) string {
	if runewidth.StringWidth(value) <= maxWidth {
		return value
	}
	if maxWidth <= 1 {
		return "…"
	}

	output := ""
	for _, char := range value {
		next := output + string(char) + "…"
		if runewidth.StringWidth(next) > maxWidth {
			break
		}
		output += string(char)
	}
	return output + "…"
}

func padEndVisible(value string, width int) string {
	padding := width - runewidth.StringWidth(value)
	if padding < 0 {
		padding = 0
	}
	return value + strings.Repeat(" ", padding)
}

func countVisible(results []RepoResult, showAll bool) int {
	count := 0
	for _, result := range results {
		if result.Loading || result.Status.Changed || showAll || (result.Sync != nil && result.Sync.Kind == "synced") {
			count++
		}
	}
	return count
}

func reposFromResults(results []RepoResult) []discover.Repo {
	repos := make([]discover.Repo, 0, len(results))
	for _, result := range results {
		repos = append(repos, result.Repo)
	}
	return repos
}

func filterLabel(showAll bool) string {
	if showAll {
		return "all repos"
	}
	return "changed repos"
}

func terminalWidth() int {
	if value := os.Getenv("COLUMNS"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			return parsed
		}
	}
	return 80
}

func style(tone string, bold bool, dim bool) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(bold).Faint(dim)
	switch tone {
	case "black":
		return base.Foreground(lipgloss.Color("0"))
	case "red":
		return base.Foreground(lipgloss.Color("9"))
	case "green":
		return base.Foreground(lipgloss.Color("10"))
	case "yellow":
		return base.Foreground(lipgloss.Color("11"))
	case "blue":
		return base.Foreground(lipgloss.Color("12"))
	case "magenta":
		return base.Foreground(lipgloss.Color("13"))
	case "cyan":
		return base.Foreground(lipgloss.Color("14"))
	case "gray":
		return base.Foreground(lipgloss.Color("8"))
	default:
		return base
	}
}

func clamp(value int, minValue int, maxValue int) int {
	return max(minValue, min(value, maxValue))
}
