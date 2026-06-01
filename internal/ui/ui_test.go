package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/status"
)

func TestBuildRowsShowsSyncedCleanRepoWhenAllIsFalse(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	rows := BuildRows([]RepoResult{{
		Repo:   repo,
		Status: status.Parse("## main...origin/main\n"),
		Sync:   &SyncOutcome{Kind: "synced", Pulled: 1},
	}}, false)

	if len(rows) == 0 {
		t.Fatal("expected synced repo to remain visible")
	}
	if rows[0].Repo != "repo" {
		t.Fatalf("expected repo row, got %#v", rows[0])
	}
}

func TestBuildRowsShowsLoadingRepoWhenAllIsFalse(t *testing.T) {
	repo := discover.Repo{
		Path:        "/tmp/repo",
		RealPath:    "/tmp/repo",
		DisplayName: "repo",
		Source:      discover.SourceScan,
	}
	rows := BuildRows([]RepoResult{{
		Repo:    repo,
		Loading: true,
	}}, false)

	if len(rows) != 1 {
		t.Fatalf("expected one loading row, got %d", len(rows))
	}
	if rows[0].Repo != "repo" || rows[0].Text != "⏳ fetching status…" {
		t.Fatalf("expected loading row, got %#v", rows[0])
	}
}

func TestFormatBranchSummary(t *testing.T) {
	parsed := status.Parse("## main...origin/main [ahead 1, behind 2]\n")
	got := FormatBranchSummary(parsed)
	want := BranchSummary{Text: "main ↑1 ↓2", Tone: "blue"}
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
	}}, false)

	got := map[string]string{}
	for _, row := range rows {
		got[row.Text] = row.Tone
	}

	if got["+ added added.go"] != "green" {
		t.Fatalf("expected added row to be green, got %#v", got)
	}
	if got["- removed removed.go"] != "red" {
		t.Fatalf("expected removed row to be red, got %#v", got)
	}
	if got["+ untracked untracked.go"] != "red" {
		t.Fatalf("expected untracked row to use git untracked red, got %#v", got)
	}
}
