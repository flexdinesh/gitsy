package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
)

func plainResults(workspace discover.Workspace, results []inspect.Result) []RepoResult {
	repoResults := make([]RepoResult, len(results))
	for index, result := range results {
		repoResults[index] = RepoResult{
			Repo:    workspace.Repos[index],
			Status:  result.Status,
			Failed:  result.Failed,
			Stale:   result.Stale,
			Sync:    result.Sync,
			Reason:  result.Reason,
			Remotes: result.Remotes,
		}
	}
	return repoResults
}

// PlainGroups renders each scan directory and its repositories, ending with a blank line.
func PlainGroups(workspace discover.Workspace, results []inspect.Result) string {
	repoResults := plainResults(workspace, results)
	var report strings.Builder
	for _, group := range workspace.Groups {
		fmt.Fprintln(&report, plainText(group.Path))
		if len(group.RepoIndexes) == 0 {
			fmt.Fprintln(&report, "  "+EmptyMessage(0))
		}
		for _, index := range group.RepoIndexes {
			repo := workspace.Repos[index]
			path := plainText(repo.Path)
			if repo.Worktree != nil {
				path += " · worktree"
			}
			fmt.Fprintf(&report, "  %s (%s)\n", plainText(repo.DisplayName), path)
			fmt.Fprintln(&report, "    "+plainText(RowsForRepo(repoResults[index])[0].Line()))
		}
		fmt.Fprintln(&report)
	}
	return report.String()
}

// Failure returns why a fetch, status, or sync command failed for a repository.
func Failure(result inspect.Result) (string, bool) {
	if !result.Failed && !result.Stale && (result.Sync == nil || result.Sync.Kind != "failed") {
		return "", false
	}
	if result.Reason != "" {
		return result.Reason, true
	}
	if result.Stale {
		return inspect.ReasonFetchFailed, true
	}
	return "failed", true
}

// RepoLine identifies a repository by name and path on one line.
func RepoLine(repo discover.Repo) string {
	return fmt.Sprintf("%s (%s)", plainText(repo.DisplayName), plainText(repo.Path))
}

// SyncSummary counts sync outcomes; skipped repos could not fast-forward safely.
func SyncSummary(results []inspect.Result) string {
	synced, current, skipped, failed := 0, 0, 0, 0
	for _, result := range results {
		branch := result.Status.Branch
		_, isFailure := Failure(result)
		switch {
		case isFailure:
			failed++
		case result.Sync != nil && result.Sync.Kind == "synced":
			synced++
		case branch != nil && branch.Upstream != "" && !branch.Gone && branch.Behind == 0:
			current++
		default:
			skipped++
		}
	}
	return outcomeSummary(fmt.Sprintf("Synced %d of %s.", synced, Count(len(results), "repo")), map[string]int{"up to date": current, "skipped": skipped, "failed": failed}, "up to date", "skipped", "failed")
}

// DeletionSummary reports bulk merged worktree deletion; detail follows the failed count.
func DeletionSummary(deleted, skipped, failed int, detail string) string {
	message := "Deleted " + Count(deleted, "merged worktree") + "."
	parts := []string{}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if failed > 0 {
		part := fmt.Sprintf("%d failed", failed)
		if detail != "" {
			part += ": " + strings.TrimSuffix(detail, ".")
		}
		parts = append(parts, part)
	}
	if len(parts) > 0 {
		message += " " + strings.Join(parts, ", ") + "."
	}
	return message + " Branches kept."
}

// DeletionPrompt asks to delete merged worktrees, warning when local files will be discarded.
func DeletionPrompt(results []RepoResult) string {
	prompt := "Delete " + Count(len(results), "merged worktree") + "?"
	dirty := 0
	for _, result := range results {
		if len(result.Status.Items) > 0 {
			dirty++
		}
	}
	if dirty == 1 {
		prompt += " 1 has local files that will be discarded."
	} else if dirty > 1 {
		prompt += fmt.Sprintf(" %d have local files that will be discarded.", dirty)
	}
	return prompt + " Branches kept."
}

func outcomeSummary(message string, counts map[string]int, order ...string) string {
	parts := []string{}
	for _, label := range order {
		if counts[label] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[label], label))
		}
	}
	if len(parts) == 0 {
		return message
	}
	return message + " " + strings.Join(parts, ", ") + "."
}

func plainText(text string) string {
	var escaped strings.Builder
	for _, character := range text {
		switch character {
		case '\r':
			escaped.WriteString("\\r")
		case '\n':
			escaped.WriteString("\\n")
		case '\t':
			escaped.WriteString("\\t")
		default:
			if unicode.IsControl(character) {
				fmt.Fprintf(&escaped, "\\u%04x", character)
			} else {
				escaped.WriteRune(character)
			}
		}
	}
	return escaped.String()
}
