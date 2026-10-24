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
	if !reflect.DeepEqual(result.Options.Dirs, []string{"/cwd"}) {
		t.Fatalf("expected Dirs [/cwd], got %v", result.Options.Dirs)
	}
	if result.Options.Command != CommandTUI || result.Options.Verbose || result.Options.NoFetch || result.Options.Sync || result.Options.Yes || result.Options.Help || result.Options.Version {
		t.Fatalf("expected boolean flags to default false: %+v", result.Options)
	}
}

func TestParseSupportsFlags(t *testing.T) {
	result := Parse([]string{"--verbose", "--no-fetch", "--fast-forward", "--help", "--version"}, "/cwd")
	if !result.OK {
		t.Fatalf("Parse returned error: %v", result.Err)
	}
	if !result.Options.Verbose || !result.Options.NoFetch || !result.Options.Sync || !result.Options.Help || !result.Options.Version {
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

func TestParseSupportsDir(t *testing.T) {
	tests := []struct {
		argv []string
		want []string
	}{
		{argv: []string{"--dir", "/some/path"}, want: []string{"/some/path"}},
		{argv: []string{"--dir=/another/path"}, want: []string{"/another/path"}},
		{argv: []string{"--dir", "/one", "--dir", "/two"}, want: []string{"/one", "/two"}},
		{argv: []string{"--dir=/one", "--no-fetch", "--dir", "two", "--max-depth", "2"}, want: []string{"/one", "two"}},
	}

	for _, test := range tests {
		result := Parse(test.argv, "/cwd")
		if !result.OK {
			t.Fatalf("Parse returned error: %v", result.Err)
		}
		if !reflect.DeepEqual(result.Options.Dirs, test.want) {
			t.Fatalf("expected Dirs %v, got %v", test.want, result.Options.Dirs)
		}
	}
}

func TestParseRejectsMissingDir(t *testing.T) {
	for _, argv := range [][]string{{"--dir"}, {"--dir", "--verbose"}, {"--dir="}, {"--dir", "/one", "--dir"}, {"--dir=/one", "--dir="}} {
		if Parse(argv, "/cwd").OK {
			t.Fatalf("expected Parse(%v) to fail", argv)
		}
	}
}

func TestParseRejectsUnknownArgs(t *testing.T) {
	for _, argv := range [][]string{{"--raw"}, {"--all"}, {"--non-interactive"}, {"-x"}} {
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
		dirs    []string
	}{
		{argv: []string{"status"}, command: CommandStatus, dirs: []string{"/cwd"}},
		{argv: []string{"--dir", "/ws", "status", "--no-fetch"}, command: CommandStatus, dirs: []string{"/ws"}},
		{argv: []string{"sync"}, command: CommandSync, dirs: []string{"/cwd"}},
		{argv: []string{"fetch"}, command: CommandFetch, dirs: []string{"/cwd"}},
		{argv: []string{"--dir", "/ws", "fetch", "--verbose"}, command: CommandFetch, dirs: []string{"/ws"}},
		{argv: []string{"sync", "--dir=/one", "--verbose"}, command: CommandSync, dirs: []string{"/one"}},
		{argv: []string{"worktree", "delete-merged"}, command: CommandDeleteMerged, dirs: []string{"/cwd"}},
		{argv: []string{"--dir", "/ws", "worktree", "delete-merged", "-y"}, command: CommandDeleteMerged, yes: true, dirs: []string{"/ws"}},
		{argv: []string{"worktree", "--yes", "delete-merged", "--no-fetch", "--base", "main"}, command: CommandDeleteMerged, yes: true, dirs: []string{"/cwd"}},
	} {
		t.Run(strings.Join(test.argv, " "), func(t *testing.T) {
			result := Parse(test.argv, "/cwd")
			if !result.OK {
				t.Fatalf("Parse returned error: %v", result.Err)
			}
			options := result.Options
			if options.Command != test.command || options.Yes != test.yes || !reflect.DeepEqual(options.Dirs, test.dirs) {
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
		{argv: []string{"status", "extra"}, want: "Unknown argument: extra"},
		{argv: []string{"sync", "now"}, want: "Unknown argument: now"},
		{argv: []string{"worktree"}, want: "Missing worktree command (expected delete-merged)"},
		{argv: []string{"worktree", "prune"}, want: "Unknown worktree command: prune"},
		{argv: []string{"worktree", "delete-merged", "all"}, want: "Unknown argument: all"},
		{argv: []string{"fetch", "now"}, want: "Unknown argument: now"},
		{argv: []string{"--fetch-only"}, want: "Unknown argument: --fetch-only"},
		{argv: []string{"sync", "--fetch-only"}, want: "Unknown argument: --fetch-only"},
		{argv: []string{"--sync"}, want: "Unknown argument: --sync"},
		{argv: []string{"-y"}, want: "-y only applies to gitsy worktree delete-merged"},
		{argv: []string{"sync", "--yes"}, want: "-y only applies to gitsy worktree delete-merged"},
		{argv: []string{"status", "--fast-forward"}, want: "--fast-forward only applies to the TUI; use gitsy sync"},
		{argv: []string{"fetch", "--fast-forward"}, want: "--fast-forward only applies to the TUI; use gitsy sync"},
		{argv: []string{"sync", "--no-fetch"}, want: "--no-fetch cannot be used with gitsy sync; sync always fetches"},
		{argv: []string{"fetch", "--no-fetch"}, want: "--no-fetch cannot be used with gitsy fetch"},
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
	for _, removed := range []string{"--non-interactive", "--sync", "--fetch-only"} {
		if strings.Contains(Usage, removed) {
			t.Fatalf("usage must not document removed %s flag", removed)
		}
	}
	for _, want := range []string{"Usage: gitsy [options] [command]", "  status ", "  sync ", "  fetch ", "--fast-forward", "  worktree delete-merged ", "-y, --yes", "Exit codes:", "  0 ", "  1 ", "  2 ", "  130 "} {
		if !strings.Contains(Usage, want) {
			t.Fatalf("usage must document %q", want)
		}
	}
}
