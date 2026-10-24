package args

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseReturnsDefaults(t *testing.T) {
	result := Parse(nil, "/cwd")
	if !result.OK {
		t.Fatalf("Parse returned error: %v", result.Err)
	}
	if result.Options.MaxDepth != 3 {
		t.Fatalf("expected MaxDepth 3, got %d", result.Options.MaxDepth)
	}
	if !reflect.DeepEqual(result.Options.Paths, []string{"/cwd"}) {
		t.Fatalf("expected Paths [/cwd], got %v", result.Options.Paths)
	}
	if result.Options.Command != CommandTUI || result.Options.Verbose || result.Options.Yes || result.Options.Help || result.Options.Version {
		t.Fatalf("expected boolean flags to default false: %+v", result.Options)
	}
}

func TestParseSupportsFlags(t *testing.T) {
	result := Parse([]string{"--verbose", "--help", "--version"}, "/cwd")
	if !result.OK {
		t.Fatalf("Parse returned error: %v", result.Err)
	}
	if !result.Options.Verbose || !result.Options.Help || !result.Options.Version {
		t.Fatalf("expected all flags true: %+v", result.Options)
	}
}

func TestParseSupportsMaxDepth(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want int
	}{
		{name: "separate value", argv: []string{"--max-depth", "5"}, want: 5},
		{name: "equals value", argv: []string{"--max-depth=7"}, want: 7},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := Parse(test.argv, "/cwd")
			if !result.OK {
				t.Fatalf("Parse returned error: %v", result.Err)
			}
			if result.Options.MaxDepth != test.want {
				t.Fatalf("expected MaxDepth %d, got %d", test.want, result.Options.MaxDepth)
			}
		})
	}
}

func TestParseRejectsInvalidMaxDepth(t *testing.T) {
	for _, argv := range [][]string{{"--max-depth", "0"}, {"--max-depth", "abc"}, {"--max-depth"}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}

func TestParseSupportsPath(t *testing.T) {
	tests := []struct {
		argv []string
		want []string
	}{
		{argv: []string{"--path", "/some/path"}, want: []string{"/some/path"}},
		{argv: []string{"--path=/another/path"}, want: []string{"/another/path"}},
		{argv: []string{"--path", "/one", "--path", "/two"}, want: []string{"/one", "/two"}},
		{argv: []string{"--path=/one", "--verbose", "--path", "two", "--max-depth", "2"}, want: []string{"/one", "two"}},
	}

	for _, test := range tests {
		result := Parse(test.argv, "/cwd")
		if !result.OK {
			t.Fatalf("Parse returned error: %v", result.Err)
		}
		if !reflect.DeepEqual(result.Options.Paths, test.want) {
			t.Fatalf("expected Paths %v, got %v", test.want, result.Options.Paths)
		}
	}
}

func TestParseRejectsMissingPath(t *testing.T) {
	for _, argv := range [][]string{{"--path"}, {"--path", "--verbose"}, {"--path="}, {"--path", "/one", "--path"}, {"--path=/one", "--path="}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}

func TestParseRejectsUnknownArgs(t *testing.T) {
	for _, argv := range [][]string{{"--raw"}, {"--all"}, {"--non-interactive"}, {"-x"}, {"--no-fetch"}, {"--fast-forward"}, {"--dir", "/one"}, {"--dir=/one"}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}

func TestUsageDocumentsWorktreeKeys(t *testing.T) {
	for _, want := range []string{"  x  ", "  X  ", "Force delete the selected worktree without confirmation", "  D  ", "Delete all merged worktrees; confirms first", "--base <branch>"} {
		if !strings.Contains(Usage, want) {
			t.Fatalf("usage must document %q", want)
		}
	}
}

func TestParseSupportsBase(t *testing.T) {
	for _, test := range []struct {
		argv []string
		want string
		ok   bool
	}{
		{argv: nil, want: "", ok: true},
		{argv: []string{"--base", "develop"}, want: "develop", ok: true},
		{argv: []string{"--base=origin/trunk"}, want: "origin/trunk", ok: true},
		{argv: []string{"--base"}},
		{argv: []string{"--base", "--verbose"}},
		{argv: []string{"--base="}},
	} {
		result := Parse(test.argv, "/cwd")
		if result.OK != test.ok || result.Options.Base != test.want {
			t.Fatalf("Parse(%v) = ok %t base %q, want ok %t base %q", test.argv, result.OK, result.Options.Base, test.ok, test.want)
		}
	}
}

func TestParseCommands(t *testing.T) {
	for _, test := range []struct {
		argv    []string
		command Command
		yes     bool
		paths   []string
	}{
		{argv: []string{"sync"}, command: CommandSync, paths: []string{"/cwd"}},
		{argv: []string{"sync", "--path=/one", "--verbose"}, command: CommandSync, paths: []string{"/one"}},
		{argv: []string{"worktree", "delete-merged"}, command: CommandDeleteMerged, paths: []string{"/cwd"}},
		{argv: []string{"--path", "/ws", "worktree", "delete-merged", "-y"}, command: CommandDeleteMerged, yes: true, paths: []string{"/ws"}},
		{argv: []string{"worktree", "--yes", "delete-merged", "--base", "main"}, command: CommandDeleteMerged, yes: true, paths: []string{"/cwd"}},
	} {
		t.Run(strings.Join(test.argv, " "), func(t *testing.T) {
			result := Parse(test.argv, "/cwd")
			if !result.OK {
				t.Fatalf("Parse returned error: %v", result.Err)
			}
			options := result.Options
			if options.Command != test.command || options.Yes != test.yes || !reflect.DeepEqual(options.Paths, test.paths) {
				t.Fatalf("unexpected options: %+v", options)
			}
		})
	}
}

func TestParseRejectsInvalidCommands(t *testing.T) {
	for _, test := range []struct {
		argv []string
		want string
	}{
		{argv: []string{"stats"}, want: "Unknown command: stats"},
		{argv: []string{"status"}, want: "Unknown command: status"},
		{argv: []string{"fetch"}, want: "Unknown command: fetch"},
		{argv: []string{"sync", "now"}, want: "Unknown argument: now"},
		{argv: []string{"worktree"}, want: "Missing worktree command (expected delete-merged)"},
		{argv: []string{"worktree", "prune"}, want: "Unknown worktree command: prune"},
		{argv: []string{"worktree", "delete-merged", "all"}, want: "Unknown argument: all"},
		{argv: []string{"--fetch-only"}, want: "Unknown argument: --fetch-only"},
		{argv: []string{"sync", "--fetch-only"}, want: "Unknown argument: --fetch-only"},
		{argv: []string{"--sync"}, want: "Unknown argument: --sync"},
		{argv: []string{"-y"}, want: "-y only applies to gitsy worktree delete-merged"},
		{argv: []string{"sync", "--yes"}, want: "-y only applies to gitsy worktree delete-merged"},
		{argv: []string{"--no-fetch"}, want: "Unknown argument: --no-fetch"},
		{argv: []string{"--fast-forward"}, want: "Unknown argument: --fast-forward"},
		{argv: []string{"sync", "--dir", "/ws"}, want: "Unknown argument: --dir"},
	} {
		t.Run(strings.Join(test.argv, " "), func(t *testing.T) {
			result := Parse(test.argv, "/cwd")
			if result.OK || result.Err == nil || result.Err.Error() != test.want {
				t.Fatalf("Parse(%v) = ok %t err %v, want error %q", test.argv, result.OK, result.Err, test.want)
			}
		})
	}
}

func TestUsageDocumentsCommandsAndExitCodes(t *testing.T) {
	for _, removed := range []string{"--non-interactive", "--sync", "--fetch-only", "--no-fetch", "--fast-forward", "--dir", "  status ", "  fetch "} {
		if strings.Contains(Usage, removed) {
			t.Fatalf("usage must not document removed %s flag", removed)
		}
	}
	for _, want := range []string{"Usage: gitsy [options] [command]", "  sync ", "--path <path>", "  worktree delete-merged ", "-y, --yes", "Exit codes:", "  0 ", "  1 ", "  2 ", "  130 "} {
		if !strings.Contains(Usage, want) {
			t.Fatalf("usage must document %q", want)
		}
	}
}
