package ui

import (
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/status"
)

func TestExceptionSummaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		result RepoResult
		text   string
		tone   string
	}{
		{"failed", RepoResult{Failed: true}, "⚠ status failed", "red"},
		{"stale", RepoResult{Stale: true}, "⚠ stale", "yellow"},
		{"behind", RepoResult{Status: status.Parse("## main...origin/main [behind 2]")}, "main ↓2", "yellow"},
		{"conflict", RepoResult{Status: status.Parse("## main\nUU file.go")}, "main • 1 conflict", "red"},
		{"sync failed", RepoResult{Sync: &SyncOutcome{Kind: "failed"}}, "⚠ sync failed", "red"},
		{"status failed after sync", RepoResult{Failed: true, Sync: &SyncOutcome{Kind: "synced", Pulled: 2}}, "⚠ status failed", "red"},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := RowsForRepo(test.result)[0]
			if !strings.HasPrefix(row.Text, test.text) || row.Tone != test.tone || row.Dim {
				t.Fatalf("exception must be visible and explicit: %#v", row)
			}
			if test.result.Failed && strings.Contains(row.Text, "clean") {
				t.Fatal("failed status must not claim clean")
			}
		})
	}
}

func TestCompactSummaryPrioritizesState(t *testing.T) {
	for _, test := range []struct {
		name   string
		result RepoResult
		want   string
	}{
		{"changes", RepoResult{Status: status.Parse("## long-feature-branch\n M one.go\n?? two.go")}, "2 files · long-feature-branch"},
		{"conflict", RepoResult{Status: status.Parse("## long-feature-branch\nUU one.go")}, "1 conflict · long-feature-branch"},
		{"behind", RepoResult{Status: status.Parse("## long-feature-branch...origin/main [ahead 1, behind 3]")}, "↓3 · ↑1 · long-feature-branch"},
		{"synced", RepoResult{Sync: &SyncOutcome{Kind: "synced", Pulled: 2}}, "synced ↓2"},
		{"stale sync", RepoResult{Stale: true, Sync: &SyncOutcome{Kind: "synced", Pulled: 2}}, "⚠ stale · synced ↓2"},
		{"failed sync", RepoResult{Sync: &SyncOutcome{Kind: "failed"}}, "⚠ sync failed"},
		{"failed", RepoResult{Failed: true}, "⚠ status failed"},
		{"loading", RepoResult{Loading: true}, "checking status…"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := CompactSummary(test.result); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}
