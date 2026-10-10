package ui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
)

// PlainReport renders completed inspections in discovery order without file details.
func PlainReport(workspace discover.Workspace, results []inspect.Result) string {
	if len(workspace.Repos) == 0 {
		return EmptyMessage(0) + "\n"
	}

	repoResults := make([]RepoResult, len(results))
	for index, result := range results {
		repoResults[index] = RepoResult{
			Repo:   workspace.Repos[index],
			Status: result.Status,
			Failed: result.Failed,
			Stale:  result.Stale,
			Sync:   result.Sync,
		}
	}

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
			fmt.Fprintln(&report, "    "+plainText(RowsForRepo(repoResults[index])[0].Text))
		}
		fmt.Fprintln(&report)
	}
	fmt.Fprintln(&report, Title(repoResults, len(workspace.Repos)))
	return report.String()
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
