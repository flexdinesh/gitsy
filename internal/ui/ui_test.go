package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestBuildRowsShowsSyncedCleanRepo(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	rows := BuildRows([]RepoResult{{
		Repo:   repo,
		Status: status.Parse("## main...origin/main\n"),
		Sync:   &inspect.SyncOutcome{Kind: "synced", Pulled: 1},
	}})

	if len(rows) == 0 {
		t.Fatal("expected synced repo to remain visible")
	}
	if rows[0].Repo != "repo" {
		t.Fatalf("expected repo row, got %#v", rows[0])
	}
}

func TestBuildRowsShowsCleanRepoOnce(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	rows := BuildRows([]RepoResult{{
		Repo:   repo,
		Status: status.Parse("## main...origin/main\n"),
	}})

	if len(rows) != 1 {
		t.Fatalf("expected one clean repo row, got %d rows: %#v", len(rows), rows)
	}
	if rows[0].Repo != "repo" || rows[0].Text != "main ✓ clean" {
		t.Fatalf("expected branch summary clean row, got %#v", rows[0])
	}
}

func TestBuildRowsAppendsChangeCounts(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	rows := BuildRows([]RepoResult{{
		Repo: repo,
		Status: status.Parse(strings.Join([]string{
			"## main",
			" M changed.go",
			" M other.go",
			"?? new.go",
		}, "\n")),
	}})

	if len(rows) == 0 || rows[0].Text != "main · 2 modified · 1 untracked" {
		t.Fatalf("expected summary with change counts, got %#v", rows)
	}
	if !rows[1].Dim || !strings.HasPrefix(rows[1].Text, "  ") {
		t.Fatalf("expected dimmed indented detail row, got %#v", rows[1])
	}
}

func TestBuildRowsShowsLoadingRepo(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	for _, test := range []struct {
		fetching bool
		want     string
	}{
		{false, "checking status…"},
		{true, "fetching…"},
	} {
		rows := BuildRows([]RepoResult{{
			Repo:     repo,
			Loading:  true,
			Fetching: test.fetching,
		}})

		if len(rows) != 1 {
			t.Fatalf("expected one loading row, got %d", len(rows))
		}
		if rows[0].Repo != "repo" || rows[0].Text != test.want {
			t.Fatalf("expected loading row %q, got %#v", test.want, rows[0])
		}
		if compact := CompactSummary(RepoResult{Loading: true, Fetching: test.fetching}); compact != test.want {
			t.Fatalf("compact loading must match regular wording: %q", compact)
		}
	}
}

func TestCountPluralizesNouns(t *testing.T) {
	for _, test := range []struct {
		count int
		noun  string
		want  string
	}{
		{0, "repo", "0 repos"},
		{1, "repo", "1 repo"},
		{2, "repo", "2 repos"},
		{1, "unique worktree", "1 unique worktree"},
		{3, "commit", "3 commits"},
	} {
		if got := Count(test.count, test.noun); got != test.want {
			t.Fatalf("Count(%d, %q) = %q, want %q", test.count, test.noun, got, test.want)
		}
	}
	if got := Title([]RepoResult{{}}, 1); got != "gitsy · 1 repo" {
		t.Fatalf("single repo title = %q", got)
	}
}

func TestTitleSummarizesRepoStates(t *testing.T) {
	results := []RepoResult{
		{
			Status: status.Parse("## main...origin/main\n M changed.go\n"),
		},
		{
			Status: status.Parse("## main...origin/main [behind 2]\n"),
		},
		{
			Failed: true,
		},
		{
			Sync: &inspect.SyncOutcome{Kind: "failed"},
		},
	}

	got := Title(results, len(results))
	want := "gitsy · 4 repos · 2 failed · 1 behind · 1 changed"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatBranchSummary(t *testing.T) {
	parsed := status.Parse("## main...origin/main [ahead 1, behind 2]\n")
	got := FormatBranchSummary(parsed)
	want := BranchSummary{Text: "main ↑1 ↓2", Tone: "yellow"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestBuildRowsUsesGitStatusLabelsAndTones(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	rows := BuildRows([]RepoResult{{
		Repo: repo,
		Status: status.Parse(strings.Join([]string{
			"## main",
			"A  added.go",
			" D removed.go",
			"?? untracked.go",
		}, "\n")),
	}})

	got := map[string]string{}
	for _, row := range rows {
		got[strings.TrimSpace(row.Text)] = row.Tone
	}

	if got["+ added added.go"] != "green" {
		t.Fatalf("expected added row to be green, got %#v", got)
	}
	if got["- removed removed.go"] != "red" {
		t.Fatalf("expected removed row to be red, got %#v", got)
	}
	if got["+ untracked untracked.go"] != "muted" {
		t.Fatalf("expected untracked row to stay off danger red, got %#v", got)
	}
}

func TestTitleTalliesByPriority(t *testing.T) {
	for _, test := range []struct {
		name    string
		results []RepoResult
		want    string
	}{
		{"clean", []RepoResult{{Status: status.Parse("## main...origin/main")}}, "gitsy · 1 repo"},
		{"ahead", []RepoResult{{Status: status.Parse("## main...origin/main [ahead 2]")}, {Status: status.Parse("## main...origin/main [ahead 1, behind 1]")}}, "gitsy · 2 repos · 1 behind · 2 ahead"},
		{"sync", []RepoResult{
			{Status: status.Parse("## main...origin/main"), Sync: &inspect.SyncOutcome{Kind: "synced", Pulled: 2}},
			{Status: status.Parse("## main...origin/main [behind 1]\n M a.go"), Stale: true, Reason: inspect.ReasonAuth},
			{Failed: true},
		}, "gitsy · 3 repos · 1 failed · 1 behind · 1 fetch failed · 1 changed · 1 synced"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Title(test.results, len(test.results)); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestSummaryTonesMatchRowTones(t *testing.T) {
	changed := RowsForRepo(RepoResult{Status: status.Parse("## main...origin/main\n M a.go")})[0]
	if SummaryTone("changed") != changed.Tone {
		t.Fatalf("header changed tone %q must match row tone %q", SummaryTone("changed"), changed.Tone)
	}
	failed := RowsForRepo(RepoResult{Failed: true})[0]
	stale := RowsForRepo(RepoResult{Stale: true})[0]
	if SummaryTone("fetch failed") != stale.RemoteTone {
		t.Fatalf("header fetch failed tone %q must match row remote tone %q", SummaryTone("fetch failed"), stale.RemoteTone)
	}
	if SummaryTone("failed") != failed.Tone || SummaryTone("behind") != "yellow" || SummaryTone("synced") != "green" {
		t.Fatal("summary tones must follow row semantics")
	}
}
