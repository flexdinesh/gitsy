package ui

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
)

type RepoResult struct {
	Repo        discover.Repo
	Status      status.Parsed
	Failed      bool
	Stale       bool
	Loading     bool
	LoadingText string
	Deleting    bool   // worktree removal in progress; shares LoadingText
	Queued      bool   // waiting in a bulk worktree removal
	Reason      string // short explanation for failed, stale or skipped state
	Sync        *inspect.SyncOutcome
	MergedInto  string // base branch a linked worktree is merged into
	Remotes     inspect.Remotes
}

type Row struct {
	Kind string
	Repo string
	Text string
	Tone string
	Bold bool
	Dim  bool
	// Remote follows Text in its own tone: a fetch failure or a local-only tag.
	Remote     string
	RemoteTone string
}

// Line joins the local summary and remote segment for single-tone output.
func (row Row) Line() string {
	if row.Remote == "" {
		return row.Text
	}
	return row.Text + " · " + row.Remote
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
	status.Untracked: {icon: "+", label: "untracked", tone: "muted"},
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

// Count formats a count with a singular or regular plural noun.
func Count(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func Title(results []RepoResult, totalDiscovered int) string {
	visible := countVisible(results)
	repoCount := fmt.Sprintf("%d/%d repos", visible, totalDiscovered)
	if visible == totalDiscovered {
		repoCount = Count(totalDiscovered, "repo")
	}

	counts := map[string]int{}
	for _, result := range results {
		if result.Failed || result.Sync != nil && result.Sync.Kind == "failed" {
			counts["failed"]++
		}
		if branch := result.Status.Branch; branch != nil {
			if branch.Behind > 0 {
				counts["behind"]++
			}
			if branch.Ahead > 0 {
				counts["ahead"]++
			}
		}
		if result.Stale {
			counts["fetch failed"]++
		}
		if len(result.Status.Items) > 0 {
			counts["changed"]++
		}
		if result.Sync != nil && result.Sync.Kind == "synced" {
			counts["synced"]++
		}
	}

	parts := []string{"gitsy", repoCount}
	for _, label := range SummaryOrder {
		if counts[label] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[label], label))
		}
	}
	return strings.Join(parts, " · ")
}

// SummaryOrder ranks summary counts from most to least urgent; narrow
// headers drop counts from the end.
var SummaryOrder = []string{"failed", "behind", "ahead", "fetch failed", "changed", "synced"}

// ReasonTone marks failures red and fetch failures yellow.
func ReasonTone(result RepoResult) string {
	if result.Failed || result.Sync != nil && result.Sync.Kind == "failed" {
		return "red"
	}
	return "yellow"
}

// SummaryTone maps a summary count label to a row tone.
func SummaryTone(label string) string {
	switch label {
	case "failed":
		return "red"
	case "behind", "fetch failed":
		return "yellow"
	case "ahead", "changed":
		return "blue"
	case "synced":
		return "green"
	}
	return "gray"
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
	if result.Queued {
		return []Row{{
			Kind: "data",
			Repo: result.Repo.DisplayName,
			Text: "queued",
			Tone: "gray",
			Dim:  true,
		}}
	}
	if result.Loading || result.Deleting {
		text := loadingText(result)
		if result.LoadingText != "" {
			text = result.LoadingText + " " + text
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
	summary := FormatBranchSummary(reliableStatus(result))
	if counts := formatChangeCounts(result.Status.Items); counts != "" {
		summary.Text += " · " + counts
	}
	if result.Failed {
		summary.Text = "⚠ status failed"
		summary.Tone = "red"
		summary.Dim = false
	}

	if result.Sync != nil {
		switch result.Sync.Kind {
		case "synced":
			summary.Text += " · synced " + Count(result.Sync.Pulled, "commit")
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

	remote, remoteTone := remoteSegment(result)
	rows = append(rows, Row{
		Kind:       "data",
		Repo:       result.Repo.DisplayName,
		Text:       summary.Text,
		Tone:       summary.Tone,
		Bold:       true,
		Dim:        summary.Dim,
		Remote:     remote,
		RemoteTone: remoteTone,
	})

	if len(result.Status.Items) == 0 {
		return rows
	}

	for _, item := range result.Status.Items {
		rows = append(rows, formatItemRow(item))
	}
	return rows
}

// reliableStatus drops "upstream gone" after a failed fetch, when the
// missing remote-tracking ref may only mean it was never fetched.
func reliableStatus(result RepoResult) status.Parsed {
	parsed := result.Status
	if !result.Stale || parsed.Branch == nil || !parsed.Branch.Gone {
		return parsed
	}
	branch := *parsed.Branch
	branch.Gone = false
	if branch.Metadata == "gone" {
		branch.Metadata = ""
	}
	parsed.Branch = &branch
	parsed.Changed = len(parsed.Items) > 0 || branch.Ahead > 0 || branch.Behind > 0
	return parsed
}

// remoteSegment explains fetch failures, or tags branches nothing upstream tracks.
func remoteSegment(result RepoResult) (string, string) {
	if result.Loading || result.Failed {
		return "", ""
	}
	if result.Stale {
		reason := result.Reason
		if reason == "" {
			reason = inspect.ReasonFetchFailed
		}
		return "⚠ " + reason, "yellow"
	}
	if tag := remoteTag(result); tag != "" {
		return tag, "gray"
	}
	return "", ""
}

// remoteTag marks checkouts that look current only because nothing tracks them.
func remoteTag(result RepoResult) string {
	if !result.Remotes.Known {
		return ""
	}
	if result.Remotes.Count == 0 {
		return "local only"
	}
	branch := result.Status.Branch
	if branch != nil && branch.Upstream == "" && !detached(branch) {
		return "no upstream"
	}
	return ""
}

func detached(branch *status.BranchStatus) bool {
	return branch.Name == "" || strings.HasPrefix(branch.Name, "HEAD (no branch)")
}

// RemoteHint suggests one next step for a failed fetch. Host-specific advice
// only applies with a single remote, because any remote can fail the fetch.
func RemoteHint(result RepoResult) string {
	if !result.Stale || result.Failed {
		return ""
	}
	scheme, host := "", ""
	single := result.Remotes.Known && result.Remotes.Count == 1 && result.Remotes.OriginURL != ""
	if single {
		scheme, host = remoteHost(result.Remotes.OriginURL)
	}
	switch result.Reason {
	case inspect.ReasonAuth:
		switch {
		case (scheme == "https" || scheme == "http") && host == "github.com":
			return "run gh auth setup-git, or switch origin to SSH"
		case (scheme == "https" || scheme == "http") && host != "":
			return "configure a credential helper for " + host + ", or use SSH"
		case scheme == "ssh" && host != "":
			return "check SSH key access to " + host
		}
		return "check credentials for the remote"
	case inspect.ReasonNotFound:
		if single {
			return "origin deleted or renamed; update or remove the remote"
		}
		return "a remote was deleted or renamed; update or remove it"
	case inspect.ReasonUnreachable:
		if host != "" {
			return "check network/VPN for " + host
		}
		return "check network/VPN"
	case inspect.ReasonFetchTimeout:
		return "retry, or check network"
	}
	return ""
}

// RemoteLabel names the remote for display: "origin · host", or a count.
func RemoteLabel(remotes inspect.Remotes) string {
	switch {
	case !remotes.Known:
		return ""
	case remotes.Count == 0:
		return "No remote"
	case remotes.OriginURL != "":
		if _, host := remoteHost(remotes.OriginURL); host != "" {
			return "origin · " + host
		}
		return "origin"
	}
	return Count(remotes.Count, "remote")
}

// remoteHost returns "https", "http" or "ssh" and the host for a remote URL.
func remoteHost(value string) (string, string) {
	if strings.Contains(value, "::") {
		return "", ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", ""
		}
		switch parsed.Scheme {
		case "https", "http":
			return parsed.Scheme, parsed.Hostname()
		case "ssh", "git+ssh", "ssh+git":
			return "ssh", parsed.Hostname()
		}
		return "", ""
	}
	// scp-like syntax: [user@]host:path, with the colon before any slash.
	colon, slash := strings.IndexByte(value, ':'), strings.IndexByte(value, '/')
	if colon <= 0 || slash >= 0 && slash < colon {
		return "", ""
	}
	host := value[:colon]
	if at := strings.LastIndexByte(host, '@'); at >= 0 {
		host = host[at+1:]
	}
	if host == "" {
		return "", ""
	}
	return "ssh", host
}

type BranchSummary struct {
	Text string
	Tone string
	Dim  bool
}

func loadingText(result RepoResult) string {
	if result.Deleting {
		return "deleting…"
	}
	return "fetching…"
}

// CompactSummary keeps actionable state ahead of branch names in narrow columns.
func CompactSummary(result RepoResult) string {
	if result.Queued {
		return "queued"
	}
	if result.Loading || result.Deleting {
		return loadingText(result)
	}
	if result.Failed {
		return "⚠ status failed"
	}
	parts := []string{}
	if result.Sync != nil && result.Sync.Kind == "failed" {
		parts = append(parts, "⚠ sync failed")
	}
	remote, _ := remoteSegment(result)
	if result.Stale {
		parts = append(parts, remote)
	}
	conflicts := 0
	for _, item := range result.Status.Items {
		if item.Category == status.Conflict {
			conflicts++
		}
	}
	if conflicts > 0 {
		parts = append(parts, Count(conflicts, "conflict"))
	} else if count := len(result.Status.Items); count > 0 {
		parts = append(parts, Count(count, "file"))
	}
	if result.Sync != nil && result.Sync.Kind == "synced" {
		parts = append(parts, "synced "+Count(result.Sync.Pulled, "commit"))
	}
	branch := reliableStatus(result).Branch
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
		if branch.Unborn {
			parts = append(parts, "no commits yet")
		}
	}
	if len(parts) == 0 {
		parts = append(parts, "✓ clean")
	}
	if !result.Stale && remote != "" {
		parts = append(parts, remote)
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
	if branch.Unborn {
		parts[0] += " · no commits yet"
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

	if !parsed.Changed && branch.Unborn {
		return BranchSummary{Text: strings.Join(parts, " "), Tone: "gray", Dim: true}
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
			if label == "conflict" {
				parts = append(parts, Count(counts[label], label))
			} else {
				parts = append(parts, fmt.Sprintf("%d %s", counts[label], label))
			}
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
	return strings.Join(parts, " · ")
}

func itemCategoryStyle(item status.Item) categoryStyle {
	itemStyle := categoryStyles[item.Category]
	if item.Category == status.Conflict {
		return itemStyle
	}
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
