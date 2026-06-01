package ui

import (
	"reflect"
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

func TestFormatCellTruncatesAndPadsByVisibleWidth(t *testing.T) {
	if got := FormatCell("abcdef", 4); got != "abc…" {
		t.Fatalf("expected abc…, got %q", got)
	}
	if got := FormatCell("ok", 4); got != "ok  " {
		t.Fatalf("expected padded cell, got %q", got)
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
