package args

import (
	"fmt"
	"strconv"
	"strings"
)

const DefaultMaxDepth = 3

const Usage = `Usage: gitsy [options]

Show git status across child repositories and linked worktrees.
Fetch all remotes fresh every run; linked worktrees share fetching when safe.
Sync skips repos if fetching fails, including failures from unrelated remotes.

Options:
  --max-depth <n>    Scan repository directories up to n nested levels (default: 3)
  --dir <path>       Scan <path> instead of the current directory (repeatable)
  --verbose          Print warnings for skipped repos and failed git commands
  --no-fetch         Skip fetching upstream changes (use local status only)
  --sync             Fast-forward repos that can safely update without conflicts (always fetches)
  --base <branch>    Default branch for merged worktrees (default: gitsy.base config, then origin/HEAD)
  --non-interactive  Print a status report and exit without opening the TUI
  --help             Show this help message
  --version          Show package version

Worktrees tab keys:
  x                  Delete the selected worktree; confirms before discarding files
  X                  Force delete the selected worktree without confirmation
  D                  Delete all merged worktrees; confirms first
`

type Options struct {
	MaxDepth       int
	Verbose        bool
	NoFetch        bool
	Sync           bool
	NonInteractive bool
	Base           string
	Dirs           []string
	Help           bool
	Version        bool
}

type ParseResult struct {
	OK      bool
	Options Options
	Err     error
}

func Parse(argv []string, cwd string) ParseResult {
	options := Options{
		MaxDepth: DefaultMaxDepth,
	}

	for index := 0; index < len(argv); index++ {
		arg := argv[index]

		switch {
		case arg == "--":
			continue
		case arg == "--verbose":
			options.Verbose = true
		case arg == "--no-fetch":
			options.NoFetch = true
		case arg == "--sync":
			options.Sync = true
		case arg == "--non-interactive":
			options.NonInteractive = true
		case arg == "--help":
			options.Help = true
		case arg == "--version":
			options.Version = true
		case arg == "--dir":
			value, ok := nextValue(argv, index)
			if !ok {
				return parseError("Missing value for --dir")
			}
			options.Dirs = append(options.Dirs, value)
			index++
		case strings.HasPrefix(arg, "--dir="):
			value := strings.TrimPrefix(arg, "--dir=")
			if value == "" {
				return parseError("Missing value for --dir")
			}
			options.Dirs = append(options.Dirs, value)
		case arg == "--base":
			value, ok := nextValue(argv, index)
			if !ok {
				return parseError("Missing value for --base")
			}
			options.Base = value
			index++
		case strings.HasPrefix(arg, "--base="):
			value := strings.TrimPrefix(arg, "--base=")
			if value == "" {
				return parseError("Missing value for --base")
			}
			options.Base = value
		case arg == "--max-depth":
			value, ok := nextValue(argv, index)
			if !ok {
				return parseError("Missing value for --max-depth")
			}
			parsed, ok := parseMaxDepth(value)
			if !ok {
				return parseError(fmt.Sprintf("Invalid --max-depth value: %s", value))
			}
			options.MaxDepth = parsed
			index++
		case strings.HasPrefix(arg, "--max-depth="):
			value := strings.TrimPrefix(arg, "--max-depth=")
			parsed, ok := parseMaxDepth(value)
			if !ok {
				return parseError(fmt.Sprintf("Invalid --max-depth value: %s", value))
			}
			options.MaxDepth = parsed
		default:
			return parseError(fmt.Sprintf("Unknown argument: %s", arg))
		}
	}

	if len(options.Dirs) == 0 {
		options.Dirs = []string{cwd}
	}
	return ParseResult{OK: true, Options: options}
}

func nextValue(argv []string, index int) (string, bool) {
	if index+1 >= len(argv) || strings.HasPrefix(argv[index+1], "--") {
		return "", false
	}
	return argv[index+1], true
}

func parseMaxDepth(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, false
	}
	return parsed, true
}

func parseError(message string) ParseResult {
	return ParseResult{OK: false, Err: fmt.Errorf("%s", message)}
}
