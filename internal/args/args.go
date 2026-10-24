package args

import (
	"fmt"
	"strconv"
	"strings"
)

const DefaultMaxDepth = 3

const Usage = `Usage: gitsy [options] [command]

Show git status across child repositories and linked worktrees.
Fetch all remotes fresh every run; linked worktrees share fetching when safe.
Fast-forward repos that can safely update without conflicts.
Fast-forward skips repos if fetching fails, including failures from unrelated remotes.
Without a command, gitsy opens the TUI.

Commands:
  sync                       Fast-forward safe repos without the TUI, print a summary, and exit
  worktree delete-merged     Delete all merged linked worktrees; confirms first, keeps branches
    -y, --yes                Delete without confirming

Options:
  --max-depth <n>    Scan repository directories up to n nested levels (default: 3)
  --path <path>      Scan <path> instead of the current directory (repeatable)
  --verbose          Print per-repo details and warnings for failed git commands
  --base <branch>    Default branch for merged worktrees (default: gitsy.base config, then origin/HEAD)
  --help             Show this help message
  --version          Show package version

Exit codes:
  0                  Success, including dirty, behind, or skipped repos
  1                  A git operation failed, deletion was declined, or scanning failed
  2                  Invalid usage, or confirmation required without a terminal
  130                Interrupted

Worktrees tab keys:
  x                  Delete the selected worktree; confirms before discarding files
  X                  Force delete the selected worktree without confirmation
  D                  Delete all merged worktrees; confirms first
`

type Command string

const (
	CommandTUI          Command = ""
	CommandSync         Command = "sync"
	CommandDeleteMerged Command = "worktree delete-merged"
)

type Options struct {
	Command  Command
	MaxDepth int
	Verbose  bool
	Yes      bool
	Base     string
	Paths    []string
	Help     bool
	Version  bool
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
	words := []string{}

	for index := 0; index < len(argv); index++ {
		arg := argv[index]

		switch {
		case arg == "--":
			continue
		case arg == "--verbose":
			options.Verbose = true
		case arg == "-y" || arg == "--yes":
			options.Yes = true
		case arg == "--help":
			options.Help = true
		case arg == "--version":
			options.Version = true
		case arg == "--path":
			value, ok := nextValue(argv, index)
			if !ok {
				return parseError("Missing value for --path")
			}
			options.Paths = append(options.Paths, value)
			index++
		case strings.HasPrefix(arg, "--path="):
			value := strings.TrimPrefix(arg, "--path=")
			if value == "" {
				return parseError("Missing value for --path")
			}
			options.Paths = append(options.Paths, value)
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
		case !strings.HasPrefix(arg, "-"):
			words = append(words, arg)
		default:
			return parseError(fmt.Sprintf("Unknown argument: %s", arg))
		}
	}

	command, err := parseCommand(words)
	if err != nil {
		return ParseResult{OK: false, Err: err}
	}
	options.Command = command
	if err := validate(options); err != nil {
		return ParseResult{OK: false, Err: err}
	}

	if len(options.Paths) == 0 {
		options.Paths = []string{cwd}
	}
	return ParseResult{OK: true, Options: options}
}

func parseCommand(words []string) (Command, error) {
	if len(words) == 0 {
		return CommandTUI, nil
	}
	switch words[0] {
	case "sync":
		if len(words) > 1 {
			return "", fmt.Errorf("Unknown argument: %s", words[1])
		}
		return CommandSync, nil
	case "worktree":
		if len(words) == 1 {
			return "", fmt.Errorf("Missing worktree command (expected delete-merged)")
		}
		if words[1] != "delete-merged" {
			return "", fmt.Errorf("Unknown worktree command: %s", words[1])
		}
		if len(words) > 2 {
			return "", fmt.Errorf("Unknown argument: %s", words[2])
		}
		return CommandDeleteMerged, nil
	}
	return "", fmt.Errorf("Unknown command: %s", words[0])
}

func validate(options Options) error {
	if options.Yes && options.Command != CommandDeleteMerged {
		return fmt.Errorf("-y only applies to gitsy worktree delete-merged")
	}
	return nil
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
