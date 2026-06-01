package ui

import (
	"fmt"
	"strings"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/status"
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

type categoryStyle struct {
	icon  string
	label string
	tone  string
	bold  bool
}

var categoryStyles = map[status.Category]categoryStyle{
	status.Modified:  {icon: "●", label: "modified", tone: "yellow"},
	status.Staged:    {icon: "◆", label: "staged", tone: "green"},
	status.Untracked: {icon: "+", label: "untracked", tone: "red"},
	status.Deleted:   {icon: "-", label: "removed", tone: "red"},
	status.Renamed:   {icon: "➜", label: "renamed", tone: "magenta"},
	status.Conflict:  {icon: "‼", label: "conflict", tone: "red", bold: true},
	status.Other:     {icon: "•", label: "changed", tone: "white"},
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

func BuildRows(results []RepoResult, showAll bool) []Row {
	rows := []Row{}
	hasVisibleRow := false

	for _, result := range results {
		repoRows := RowsForRepo(result, showAll)
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

func RowsForRepo(result RepoResult, showAll bool) []Row {
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

func formatItemRow(item status.Item) Row {
	itemStyle := itemCategoryStyle(item)
	return Row{
		Kind: "data",
		Text: fmt.Sprintf("%s %s %s", itemStyle.icon, itemStyle.label, formatItemPath(item)),
		Tone: itemStyle.tone,
		Bold: itemStyle.bold,
	}
}

func itemCategoryStyle(item status.Item) categoryStyle {
	itemStyle := categoryStyles[item.Category]
	if strings.Contains(item.Code, "A") {
		itemStyle.icon = "+"
		itemStyle.label = "added"
		itemStyle.tone = "green"
		itemStyle.bold = false
	}
	if strings.Contains(item.Code, "D") {
		itemStyle.icon = "-"
		itemStyle.label = "removed"
		itemStyle.tone = "red"
		itemStyle.bold = false
	}
	return itemStyle
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

func countVisible(results []RepoResult, showAll bool) int {
	count := 0
	for _, result := range results {
		if result.Loading || result.Status.Changed || showAll || (result.Sync != nil && result.Sync.Kind == "synced") {
			count++
		}
	}
	return count
}

func filterLabel(showAll bool) string {
	if showAll {
		return "all repos"
	}
	return "changed repos"
}
