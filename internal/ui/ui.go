package ui

import (
	"fmt"
	"sort"
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
	status.Renamed:   {icon: "→", label: "renamed", tone: "magenta"},
	status.Conflict:  {icon: "!", label: "conflict", tone: "red", bold: true},
	status.Other:     {icon: "•", label: "changed", tone: "white"},
}

func EmptyMessage(totalDiscovered int) string {
	if totalDiscovered == 0 {
		return "No child git repositories found."
	}
	return ""
}

func Title(results []RepoResult, totalDiscovered int) string {
	visible := countVisible(results)
	repoCount := fmt.Sprintf("%d/%d repos", visible, totalDiscovered)
	if visible == totalDiscovered {
		repoCount = fmt.Sprintf("%d repos", totalDiscovered)
	}

	changed := 0
	behind := 0
	failed := 0
	stale := 0
	for _, result := range results {
		if len(result.Status.Items) > 0 {
			changed++
		}
		if result.Status.Branch != nil && result.Status.Branch.Behind > 0 {
			behind++
		}
		if result.Failed || result.Sync != nil && result.Sync.Kind == "failed" {
			failed++
		}
		if result.Stale {
			stale++
		}
	}

	parts := []string{"gitsy", repoCount}
	if changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", changed))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", behind))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if stale > 0 {
		parts = append(parts, fmt.Sprintf("%d stale", stale))
	}
	return strings.Join(parts, " • ")
}

func BuildRows(results []RepoResult) []Row {
	rows := []Row{}

	for _, result := range results {
		repoRows := RowsForRepo(result)
		if len(repoRows) == 0 {
			continue
		}
		rows = append(rows, repoRows...)
	}

	return rows
}

func RowsForRepo(result RepoResult) []Row {
	if result.Loading {
		text := "fetching status…"
		if result.LoadingText != "" {
			text = fmt.Sprintf("%s fetching status…", result.LoadingText)
		}
		return []Row{{
			Kind: "data",
			Repo: result.Repo.DisplayName,
			Text: text,
			Tone: "cyan",
			Bold: true,
		}}
	}

	rows := []Row{}
	summary := FormatBranchSummary(result.Status)
	if counts := formatChangeCounts(result.Status.Items); counts != "" {
		summary.Text += " • " + counts
	}
	if result.Stale {
		summary.Text = "⚠ stale · " + summary.Text
		summary.Tone = "yellow"
		summary.Dim = false
	}
	if result.Failed {
		summary.Text = "⚠ status failed"
		summary.Tone = "red"
		summary.Dim = false
	}

	if result.Sync != nil {
		switch result.Sync.Kind {
		case "synced":
			summary.Text += fmt.Sprintf(" ⤓ synced ↓%d", result.Sync.Pulled)
			if !result.Failed && !result.Stale {
				summary.Tone = "green"
			}
			summary.Dim = false
		case "failed":
			summary.Text = "⚠ sync failed · " + summary.Text
			summary.Tone = "red"
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

// CompactSummary keeps actionable state ahead of branch names in narrow columns.
func CompactSummary(result RepoResult) string {
	if result.Loading {
		return "checking status…"
	}
	if result.Failed {
		return "⚠ status failed"
	}
	parts := []string{}
	if result.Sync != nil && result.Sync.Kind == "failed" {
		parts = append(parts, "⚠ sync failed")
	}
	if result.Stale {
		parts = append(parts, "⚠ stale")
	}
	conflicts := 0
	for _, item := range result.Status.Items {
		if item.Category == status.Conflict {
			conflicts++
		}
	}
	if conflicts > 0 {
		parts = append(parts, fmt.Sprintf("%d conflict", conflicts))
	} else if count := len(result.Status.Items); count > 0 {
		parts = append(parts, fmt.Sprintf("%d files", count))
	}
	if result.Sync != nil && result.Sync.Kind == "synced" {
		parts = append(parts, fmt.Sprintf("synced ↓%d", result.Sync.Pulled))
	}
	branch := result.Status.Branch
	if branch != nil {
		if branch.Behind > 0 {
			parts = append(parts, fmt.Sprintf("↓%d", branch.Behind))
		}
		if branch.Ahead > 0 {
			parts = append(parts, fmt.Sprintf("↑%d", branch.Ahead))
		}
		if branch.Gone {
			parts = append(parts, "upstream gone")
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "✓ clean")
	}
	if branch != nil {
		name := branch.Name
		if name == "" {
			name = "detached"
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " · ")
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
	if branch.Gone || branch.Behind > 0 {
		tone = "yellow"
	}
	for _, item := range parsed.Items {
		if item.Category == status.Conflict {
			tone = "red"
			break
		}
	}
	return BranchSummary{Text: strings.Join(parts, " "), Tone: tone}
}

func formatItemRow(item status.Item) Row {
	itemStyle := itemCategoryStyle(item)
	return Row{
		Kind: "data",
		Text: fmt.Sprintf("  %s %s %s", itemStyle.icon, itemStyle.label, formatItemPath(item)),
		Tone: itemStyle.tone,
		Bold: itemStyle.bold,
		Dim:  !itemStyle.bold,
	}
}

// countLabelOrder keeps change counts deterministic.
var countLabelOrder = []string{"modified", "staged", "added", "untracked", "removed", "renamed", "conflict", "changed"}

func formatChangeCounts(items []status.Item) string {
	counts := map[string]int{}
	for _, item := range items {
		counts[itemCategoryStyle(item).label]++
	}
	parts := []string{}
	for _, label := range countLabelOrder {
		if counts[label] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[label], label))
			delete(counts, label)
		}
	}
	rest := []string{}
	for label := range counts {
		rest = append(rest, label)
	}
	sort.Strings(rest)
	for _, label := range rest {
		parts = append(parts, fmt.Sprintf("%d %s", counts[label], label))
	}
	return strings.Join(parts, " • ")
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

func countVisible(results []RepoResult) int {
	return len(results)
}
