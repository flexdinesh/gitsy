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
		"    fix/auth • 2 modified • 1 untracked\n\n" +
		"/worktrees\n" +
		"  api (/worktrees/api-fix · worktree)\n" +
		"    fix/auth • 2 modified • 1 untracked\n\n" +
		"/empty\n" +
		"  No child git repositories found.\n\n" +
		"gitsy • 2 repos • 1 changed • 1 behind\n"
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
		{"conflict", inspect.Result{Status: status.Parse("## main\nUU hidden.go")}, "main • 1 conflict"},
		{"categories", inspect.Result{Status: status.Parse("## main\n M hidden.go\nM  staged.go\nA  added.go\n?? untracked.go\n D removed.go\nR  old.go -> renamed.go")}, "main • 1 modified • 1 staged • 1 added • 1 untracked • 1 removed • 1 renamed"},
		{"status failed", inspect.Result{Failed: true}, "⚠ status failed"},
		{"fetch failed", inspect.Result{Status: status.Parse("## main"), Stale: true}, "⚠ stale · main ✓ clean"},
		{"synced", inspect.Result{Status: status.Parse("## main"), Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 3}}, "main ✓ clean ⤓ synced ↓3"},
		{"sync failed", inspect.Result{Status: status.Parse("## main...origin/main [behind 3]"), Sync: &inspect.SyncOutcome{Kind: "failed"}}, "⚠ sync failed · main ↓3"},
		{"status failed after sync", inspect.Result{Failed: true, Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 3}}, "⚠ status failed ⤓ synced ↓3"},
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
		"gitsy • 1 repos\n"
	if got := PlainReport(workspace, results); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
