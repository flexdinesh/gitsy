package args

import (
	"fmt"
	"strconv"
	"strings"
)

const DefaultMaxDepth = 3

const Usage = `Usage: gitsy [options]

Show git status across child repositories and linked worktrees.

Options:
  --all              Show all discovered repositories, including clean repos
  --max-depth <n>    Scan repository directories up to n nested levels (default: 3)
  --dir <path>       Start scanning from <path> instead of the current directory
  --verbose          Print warnings for skipped repos and failed git commands
  --no-fetch         Skip fetching upstream changes (use local status only)
  --sync             Fast-forward repos that can safely update without conflicts (always fetches)
  --help             Show this help message
  --version          Show package version
`

type Options struct {
	All      bool
	MaxDepth int
	Verbose  bool
	NoFetch  bool
	Sync     bool
	Dir      string
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
		Dir:      cwd,
	}

	for index := 0; index < len(argv); index++ {
		arg := argv[index]

		switch {
		case arg == "--":
			continue
		case arg == "--all":
			options.All = true
		case arg == "--verbose":
			options.Verbose = true
		case arg == "--no-fetch":
			options.NoFetch = true
		case arg == "--sync":
			options.Sync = true
		case arg == "--help":
			options.Help = true
		case arg == "--version":
			options.Version = true
		case arg == "--dir":
			value, ok := nextValue(argv, index)
			if !ok {
				return parseError("Missing value for --dir")
			}
			options.Dir = value
			index++
		case strings.HasPrefix(arg, "--dir="):
			value := strings.TrimPrefix(arg, "--dir=")
			if value == "" {
				return parseError("Missing value for --dir")
			}
			options.Dir = value
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
