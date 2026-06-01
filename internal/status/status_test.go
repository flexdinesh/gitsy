package status

import "testing"

func TestCleanBranchOnlyStatusIsUnchanged(t *testing.T) {
	parsed := Parse("## main...origin/main\n")
	if parsed.Changed || HasChangesOrDivergence(parsed.Raw) {
		t.Fatal("expected clean branch-only status to be unchanged")
	}
	if parsed.Branch == nil || parsed.Branch.Name != "main" || parsed.Branch.Upstream != "origin/main" {
		t.Fatalf("unexpected branch: %+v", parsed.Branch)
	}
}

func TestAheadAndBehindCountsAsChanged(t *testing.T) {
	parsed := Parse("## main...origin/main [ahead 1, behind 2]\n")
	if !parsed.Changed {
		t.Fatal("expected ahead/behind to count as changed")
	}
	if parsed.Branch == nil || parsed.Branch.Ahead != 1 || parsed.Branch.Behind != 2 {
		t.Fatalf("unexpected branch: %+v", parsed.Branch)
	}
}

func TestGoneUpstreamCountsAsChanged(t *testing.T) {
	parsed := Parse("## feature...origin/feature [gone]\n")
	if !parsed.Changed || parsed.Branch == nil || !parsed.Branch.Gone {
		t.Fatalf("expected gone upstream: %+v", parsed.Branch)
	}
}

func TestParsesStatusCategories(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want Category
	}{
		{name: "modified", raw: "## main\n M src/index.ts\n", want: Modified},
		{name: "staged", raw: "## main\nA  README.md\n", want: Staged},
		{name: "untracked", raw: "## main\n?? README.md\n", want: Untracked},
		{name: "deleted", raw: "## main\n D old.ts\n", want: Deleted},
		{name: "renamed", raw: "## main\nR  old.ts -> new.ts\n", want: Renamed},
		{name: "conflict", raw: "## main\nUU conflicted.ts\n", want: Conflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed := Parse(test.raw)
			if len(parsed.Items) != 1 {
				t.Fatalf("expected one item, got %d", len(parsed.Items))
			}
			if parsed.Items[0].Category != test.want {
				t.Fatalf("expected %s, got %s", test.want, parsed.Items[0].Category)
			}
		})
	}
}
