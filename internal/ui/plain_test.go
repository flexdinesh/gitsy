package ui

import (
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestPlainReportGroupsAndWorktrees(t *testing.T) {
	workspace := discover.Workspace{
		Repos: []discover.Repo{
			{DisplayName: "api", Path: "/workspace/api", Source: discover.SourceWorktree},
			{DisplayName: "api", Path: "/worktrees/api-fix", Source: discover.SourceScan, Worktree: &git.Worktree{}},
		},
		Groups: []discover.Group{
			{Path: "/workspace", RepoIndexes: []int{0, 1}},
			{Path: "/worktrees", RepoIndexes: []int{1}},
			{Path: "/empty"},
		},
	}
	results := []inspect.Result{
		{Status: status.Parse("## main...origin/main [behind 3]")},
		{Status: status.Parse("## fix/auth\n M secret-file.go\n M another-secret.go\n?? private.txt")},
	}
	want := "/workspace\n" +
		"  api (/workspace/api)\n" +
		"    main ↓3\n" +
		"  api (/worktrees/api-fix · worktree)\n" +
		"    fix/auth · 2 modified · 1 untracked\n\n" +
		"/worktrees\n" +
		"  api (/worktrees/api-fix · worktree)\n" +
		"    fix/auth · 2 modified · 1 untracked\n\n" +
		"/empty\n" +
		"  No child git repositories found.\n\n" +
		"gitsy · 2 repos · 1 behind · 1 changed\n"
	if got := PlainReport(workspace, results); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPlainReportSummaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		result inspect.Result
		want   string
	}{
		{"clean", inspect.Result{Status: status.Parse("## main")}, "main ✓ clean"},
		{"detached", inspect.Result{Status: status.Parse("## HEAD (no branch)")}, "HEAD (no branch) ✓ clean"},
		{"ahead and behind", inspect.Result{Status: status.Parse("## main...origin/main [ahead 1, behind 2]")}, "main ↑1 ↓2"},
		{"conflict", inspect.Result{Status: status.Parse("## main\nUU hidden.go")}, "main · 1 conflict"},
		{"conflicts", inspect.Result{Status: status.Parse("## main\nUU hidden.go\nAA other.go")}, "main · 2 conflicts"},
		{"categories", inspect.Result{Status: status.Parse("## main\n M hidden.go\nM  staged.go\nA  added.go\n?? untracked.go\n D removed.go\nR  old.go -> renamed.go")}, "main · 1 modified · 1 staged · 1 added · 1 untracked · 1 removed · 1 renamed"},
		{"status failed", inspect.Result{Failed: true}, "⚠ status failed"},
		{"fetch failed", inspect.Result{Status: status.Parse("## main"), Stale: true}, "⚠ stale · main ✓ clean"},
		{"synced", inspect.Result{Status: status.Parse("## main"), Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 3}}, "main ✓ clean · synced 3 commits"},
		{"sync failed", inspect.Result{Status: status.Parse("## main...origin/main [behind 3]"), Sync: &inspect.SyncOutcome{Kind: "failed"}}, "⚠ sync failed · main ↓3"},
		{"fetch failed skips sync", inspect.Result{Status: status.Parse("## main...origin/main [behind 3]"), Stale: true, Sync: &inspect.SyncOutcome{Kind: "skipped", Reason: "fetch failed"}}, "⚠ stale · main ↓3 · sync skipped: fetch failed"},
		{"status failed and sync skipped", inspect.Result{Failed: true, Stale: true, Sync: &inspect.SyncOutcome{Kind: "skipped", Reason: "fetch failed"}}, "⚠ status failed · sync skipped: fetch failed"},
		{"status failed after sync", inspect.Result{Failed: true, Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 3}}, "⚠ status failed · synced 3 commits"},
		{"long unicode branch", inspect.Result{Status: status.Parse("## " + strings.Repeat("修正", 80))}, strings.Repeat("修正", 80) + " ✓ clean"},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := discover.Workspace{
				Repos:  []discover.Repo{{DisplayName: "repo", Path: "/workspace/repo"}},
				Groups: []discover.Group{{Path: "/workspace", RepoIndexes: []int{0}}},
			}
			got := PlainReport(workspace, []inspect.Result{test.result})
			if !strings.HasPrefix(got, "/workspace\n  repo (/workspace/repo)\n    "+test.want+"\n\n") {
				t.Fatalf("unexpected summary: %q", got)
			}
			if strings.Contains(got, ".go") {
				t.Fatalf("plain report must omit filenames: %q", got)
			}
		})
	}
}

func TestPlainReportEmpty(t *testing.T) {
	if got := PlainReport(discover.Workspace{}, nil); got != "No child git repositories found.\n" {
		t.Fatalf("unexpected empty report: %q", got)
	}
}

func TestPlainReportEscapesControls(t *testing.T) {
	workspace := discover.Workspace{
		Repos:  []discover.Repo{{DisplayName: "repo\x1b[31m", Path: "/workspace/repo\t\r\n"}},
		Groups: []discover.Group{{Path: "/workspace\x07", RepoIndexes: []int{0}}},
	}
	results := []inspect.Result{{Status: status.Parsed{Branch: &status.BranchStatus{Name: "main\u009b\x1b[2J"}}}}
	want := "/workspace\\u0007\n" +
		"  repo\\u001b[31m (/workspace/repo\\t\\r\\n)\n" +
		"    main\\u009b\\u001b[2J ✓ clean\n\n" +
		"gitsy · 1 repo\n"
	if got := PlainReport(workspace, results); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSyncSummary(t *testing.T) {
	synced := inspect.Result{Status: status.Parse("## main...origin/main"), Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}}
	current := inspect.Result{Status: status.Parse("## main...origin/main [ahead 1]")}
	dirty := inspect.Result{Status: status.Parse("## main...origin/main [behind 1]\n M file")}
	noUpstream := inspect.Result{Status: status.Parse("## main")}
	gone := inspect.Result{Status: status.Parse("## main...origin/main [gone]")}
	stale := inspect.Result{Status: status.Parse("## main...origin/main [behind 1]"), Stale: true, Sync: &inspect.SyncOutcome{Kind: "skipped", Reason: "fetch failed"}}
	failed := inspect.Result{Status: status.Parse("## main...origin/main [behind 1]"), Sync: &inspect.SyncOutcome{Kind: "failed"}}
	for _, test := range []struct {
		name    string
		results []inspect.Result
		want    string
	}{
		{"all synced", []inspect.Result{synced}, "Synced 1 of 1 repo."},
		{"up to date", []inspect.Result{current, current}, "Synced 0 of 2 repos. 2 up to date."},
		{"mixed", []inspect.Result{synced, current, dirty, noUpstream, gone, stale, failed}, "Synced 1 of 7 repos. 1 up to date, 3 skipped, 2 failed."},
		{"status failed after sync", []inspect.Result{{Failed: true, Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 1}}}, "Synced 0 of 1 repo. 1 failed."},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := SyncSummary(test.results); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestFetchSummary(t *testing.T) {
	ok, stale := inspect.Result{Status: status.Parse("## main")}, inspect.Result{Stale: true}
	for _, test := range []struct {
		results []inspect.Result
		want    string
	}{
		{[]inspect.Result{ok}, "Fetched 1 of 1 repo."},
		{[]inspect.Result{ok, stale, stale}, "Fetched 1 of 3 repos. 2 failed."},
	} {
		if got := FetchSummary(test.results); got != test.want {
			t.Fatalf("got %q, want %q", got, test.want)
		}
	}
}

func TestDeletionSummaryAndPrompt(t *testing.T) {
	for _, test := range []struct {
		deleted, skipped, failed int
		detail                   string
		want                     string
	}{
		{1, 0, 0, "", "Deleted 1 merged worktree. Branches kept."},
		{2, 1, 0, "", "Deleted 2 merged worktrees. 1 skipped. Branches kept."},
		{0, 1, 2, "", "Deleted 0 merged worktrees. 1 skipped, 2 failed. Branches kept."},
		{0, 0, 1, "Worktree is locked.", "Deleted 0 merged worktrees. 1 failed: Worktree is locked. Branches kept."},
	} {
		if got := DeletionSummary(test.deleted, test.skipped, test.failed, test.detail); got != test.want {
			t.Fatalf("got %q, want %q", got, test.want)
		}
	}
	clean, dirty := RepoResult{Status: status.Parse("## main")}, RepoResult{Status: status.Parse("## main\n M file")}
	for _, test := range []struct {
		results []RepoResult
		want    string
	}{
		{[]RepoResult{clean}, "Delete 1 merged worktree? Branches kept."},
		{[]RepoResult{clean, dirty}, "Delete 2 merged worktrees? 1 has local files that will be discarded. Branches kept."},
		{[]RepoResult{dirty, dirty}, "Delete 2 merged worktrees? 2 have local files that will be discarded. Branches kept."},
	} {
		if got := DeletionPrompt(test.results); got != test.want {
			t.Fatalf("got %q, want %q", got, test.want)
		}
	}
}

func TestStatusSummaryAndRepoLine(t *testing.T) {
	workspace := discover.Workspace{Repos: []discover.Repo{{DisplayName: "api\x1b", Path: "/workspace/api\n"}}}
	if got := StatusSummary(workspace, []inspect.Result{{Status: status.Parse("## main...origin/main [behind 1]")}}); got != "gitsy · 1 repo · 1 behind" {
		t.Fatalf("unexpected status summary: %q", got)
	}
	if got := StatusSummary(discover.Workspace{}, nil); got != "No child git repositories found." {
		t.Fatalf("unexpected empty summary: %q", got)
	}
	if got := RepoLine(workspace.Repos[0]); got != "api\\u001b (/workspace/api\\n)" {
		t.Fatalf("repo line must escape controls: %q", got)
	}
}
