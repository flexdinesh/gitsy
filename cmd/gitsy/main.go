package main

import (
	"fmt"
	"os"

	"github.com/flexdinesh/gitsy/internal/args"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/tui"
	"github.com/flexdinesh/gitsy/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gitsy: %s\n", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	parsed := args.Parse(argv, cwd)
	if !parsed.OK {
		fmt.Fprintf(os.Stderr, "gitsy: %s\n\n%s", parsed.Err, args.Usage)
		os.Exit(1)
	}
	options := parsed.Options

	if options.Help {
		fmt.Fprint(os.Stdout, args.Usage)
		return nil
	}

	if options.Version {
		fmt.Fprintf(os.Stdout, "gitsy %s\n", version.Version)
		return nil
	}

	warn := func(message string) {
		fmt.Fprintf(os.Stderr, "gitsy: warning: %s\n", message)
	}

	repos := discover.Discover(discover.Options{
		Cwd:      options.Dir,
		MaxDepth: options.MaxDepth,
		Verbose:  options.Verbose,
		Warn:     warn,
	})

	noFetch := options.NoFetch
	if options.Sync {
		noFetch = false
	}

	var processWarn func(message string)
	if options.Verbose {
		processWarn = warn
	}

	return tui.Run(os.Stdout, repos, options.All, noFetch, options.Sync, processWarn)
}
