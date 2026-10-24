package ui

import (
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestExceptionSummaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		result RepoResult
		text   string
		tone   string
		remote string
	}{
		{"failed", RepoResult{Failed: true}, "⚠ status failed", "red", ""},
		{"behind", RepoResult{Status: status.Parse("## main...origin/main [behind 2]")}, "main ↓2", "yellow", ""},
		{"conflict", RepoResult{Status: status.Parse("## main\nUU file.go")}, "main · 1 conflict", "red", ""},
		{"sync failed", RepoResult{Sync: &inspect.SyncOutcome{Kind: "failed"}}, "⚠ sync failed", "red", ""},
		{"status failed after sync", RepoResult{Failed: true, Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}}, "⚠ status failed", "red", ""},
		{"status and fetch failed", RepoResult{Failed: true, Stale: true, Reason: "status failed: index file corrupt"}, "⚠ status failed", "red", ""},
		{"fetch failed behind", RepoResult{Stale: true, Reason: inspect.ReasonNotFound, Status: status.Parse("## main...origin/main [behind 2]")}, "main ↓2", "yellow", "⚠ remote repo not found"},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := RowsForRepo(test.result)[0]
			if !strings.HasPrefix(row.Text, test.text) || row.Tone != test.tone || row.Dim || row.Remote != test.remote {
				t.Fatalf("exception must be visible and explicit: %#v", row)
			}
			if test.result.Failed && strings.Contains(row.Text, "clean") {
				t.Fatal("failed status must not claim clean")
			}
		})
	}
}

func TestRemoteSegments(t *testing.T) {
	clean := status.Parse("## main...origin/main")
	for _, test := range []struct {
		name   string
		result RepoResult
		text   string
		tone   string
		dim    bool
		remote string
		rtone  string
	}{
		{"fetch failed keeps clean tone", RepoResult{Status: clean, Stale: true, Reason: inspect.ReasonAuth}, "main ✓ clean", "green", true, "⚠ remote auth needed", "yellow"},
		{"fetch failed without reason", RepoResult{Status: clean, Stale: true}, "main ✓ clean", "green", true, "⚠ fetch failed", "yellow"},
		{"fetch failed hides gone", RepoResult{Status: status.Parse("## main...origin/main [gone]"), Stale: true, Reason: inspect.ReasonAuth}, "main ✓ clean", "green", true, "⚠ remote auth needed", "yellow"},
		{"gone without fetch failure", RepoResult{Status: status.Parse("## main...origin/main [gone]")}, "main ⚠ upstream gone", "yellow", false, "", ""},
		{"unborn", RepoResult{Status: status.Parse("## No commits yet on main")}, "main · no commits yet", "gray", true, "", ""},
		{"unborn with files", RepoResult{Status: status.Parse("## No commits yet on main\n?? a.go")}, "main · no commits yet · 1 untracked", "blue", false, "", ""},
		{"local only", RepoResult{Status: status.Parse("## main"), Remotes: inspect.Remotes{Known: true}}, "main ✓ clean", "green", true, "local only", "gray"},
		{"no upstream", RepoResult{Status: status.Parse("## feat/x"), Remotes: inspect.Remotes{Known: true, Count: 1}}, "feat/x ✓ clean", "green", true, "no upstream", "gray"},
		{"tracked", RepoResult{Status: clean, Remotes: inspect.Remotes{Known: true, Count: 1}}, "main ✓ clean", "green", true, "", ""},
		{"detached has no tag", RepoResult{Status: status.Parse("## HEAD (no branch)"), Remotes: inspect.Remotes{Known: true, Count: 1}}, "HEAD (no branch) ✓ clean", "green", true, "", ""},
		{"unknown remotes have no tag", RepoResult{Status: status.Parse("## main")}, "main ✓ clean", "green", true, "", ""},
		{"loading has no tag", RepoResult{Loading: true, Remotes: inspect.Remotes{Known: true}}, "checking status…", "cyan", false, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := RowsForRepo(test.result)[0]
			if row.Text != test.text || row.Tone != test.tone || row.Dim != test.dim || row.Remote != test.remote || row.RemoteTone != test.rtone {
				t.Fatalf("unexpected row: %#v", row)
			}
		})
	}
}

func TestRemoteHint(t *testing.T) {
	single := func(url string) inspect.Remotes { return inspect.Remotes{Known: true, Count: 1, OriginURL: url} }
	for _, test := range []struct {
		name   string
		result RepoResult
		want   string
	}{
		{"github https auth", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Remotes: single("https://github.com/o/r")}, "run gh auth setup-git, or switch origin to SSH"},
		{"other https auth", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Remotes: single("https://user@git.example.com:8443/o/r.git")}, "configure a credential helper for git.example.com, or use SSH"},
		{"scp ssh auth", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Remotes: single("git@github.com:o/r.git")}, "check SSH key access to github.com"},
		{"ssh url auth", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Remotes: single("ssh://git@host.example/o/r")}, "check SSH key access to host.example"},
		{"multiple remotes auth", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Remotes: inspect.Remotes{Known: true, Count: 2, OriginURL: "https://github.com/o/r"}}, "check credentials for the remote"},
		{"local path auth", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Remotes: single("/srv/git/r.git")}, "check credentials for the remote"},
		{"not found", RepoResult{Stale: true, Reason: inspect.ReasonNotFound, Remotes: single("git@github.com:o/r.git")}, "origin deleted or renamed; update or remove the remote"},
		{"not found other remote", RepoResult{Stale: true, Reason: inspect.ReasonNotFound, Remotes: inspect.Remotes{Known: true, Count: 1}}, "a remote was deleted or renamed; update or remove it"},
		{"unreachable", RepoResult{Stale: true, Reason: inspect.ReasonUnreachable, Remotes: single("git@gitlab.lan:o/r.git")}, "check network/VPN for gitlab.lan"},
		{"unreachable unknown host", RepoResult{Stale: true, Reason: inspect.ReasonUnreachable}, "check network/VPN"},
		{"timeout", RepoResult{Stale: true, Reason: inspect.ReasonFetchTimeout}, "retry, or check network"},
		{"generic", RepoResult{Stale: true, Reason: inspect.ReasonFetchFailed}, ""},
		{"not stale", RepoResult{Reason: inspect.ReasonAuth}, ""},
		{"status failed", RepoResult{Failed: true, Stale: true, Reason: "status failed: x"}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := RemoteHint(test.result); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestRemoteLabel(t *testing.T) {
	for _, test := range []struct {
		remotes inspect.Remotes
		want    string
	}{
		{inspect.Remotes{}, ""},
		{inspect.Remotes{Known: true}, "No remote"},
		{inspect.Remotes{Known: true, Count: 1, OriginURL: "https://git.example.com/o/r.git"}, "origin · git.example.com"},
		{inspect.Remotes{Known: true, Count: 1, OriginURL: "../local.git"}, "origin"},
		{inspect.Remotes{Known: true, Count: 2}, "2 remotes"},
	} {
		if got := RemoteLabel(test.remotes); got != test.want {
			t.Fatalf("RemoteLabel(%+v) = %q, want %q", test.remotes, got, test.want)
		}
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
		{"synced", RepoResult{Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}}, "synced 2 commits"},
		{"fetch failed", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Status: status.Parse("## main...origin/main")}, "⚠ remote auth needed · main"},
		{"fetch failed hides gone", RepoResult{Stale: true, Reason: inspect.ReasonAuth, Status: status.Parse("## No commits yet on main...origin/main [gone]")}, "⚠ remote auth needed · no commits yet · main"},
		{"gone", RepoResult{Status: status.Parse("## main...origin/main [gone]")}, "upstream gone · main"},
		{"local only", RepoResult{Status: status.Parse("## main"), Remotes: inspect.Remotes{Known: true}}, "✓ clean · local only · main"},
		{"no upstream with files", RepoResult{Status: status.Parse("## main\n M a.go"), Remotes: inspect.Remotes{Known: true, Count: 1}}, "1 file · no upstream · main"},
		{"failed sync", RepoResult{Sync: &inspect.SyncOutcome{Kind: "failed"}}, "⚠ sync failed"},
		{"failed", RepoResult{Failed: true}, "⚠ status failed"},
		{"fetch failed behind", RepoResult{Stale: true, Status: status.Parse("## long-feature-branch...origin/main [behind 2]")}, "⚠ fetch failed · ↓2 · long-feature-branch"},
		{"status and fetch failed", RepoResult{Failed: true, Stale: true}, "⚠ status failed"},
		{"ordinary sync skip", RepoResult{Sync: &inspect.SyncOutcome{Kind: "skipped", Reason: "no upstream"}}, "✓ clean"},
		{"loading", RepoResult{Loading: true}, "checking status…"},
		{"fetching", RepoResult{Loading: true, Fetching: true}, "fetching…"},
		{"one file", RepoResult{Status: status.Parse("## main\n M one.go")}, "1 file · main"},
		{"conflicts", RepoResult{Status: status.Parse("## main\nUU one.go\nUU two.go")}, "2 conflicts · main"},
		{"synced one", RepoResult{Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 1}}, "synced 1 commit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := CompactSummary(test.result); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestAllConflictsKeepTheirLabels(t *testing.T) {
	for _, code := range []string{"DD", "AU", "UD", "UA", "DU", "AA", "UU"} {
		t.Run(code, func(t *testing.T) {
			rows := RowsForRepo(RepoResult{Status: status.Parse("## main\n" + code + " file.go")})
			if len(rows) != 2 || rows[0].Text != "main · 1 conflict" || rows[1].Text != "  ! conflict file.go" {
				t.Fatalf("conflict must remain explicit in summary and detail: %#v", rows)
			}
			if rows[1].Tone != "red" || !rows[1].Bold || rows[1].Dim {
				t.Fatalf("conflict detail must remain emphasized: %#v", rows[1])
			}
		})
	}
}
