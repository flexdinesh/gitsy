package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/flexdinesh/gitsy/internal/args"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/tui"
	"github.com/flexdinesh/gitsy/internal/version"
)

type usageError struct {
	err error
}

func (err usageError) Error() string {
	return fmt.Sprintf("%s\n\n%s", err.err, args.Usage)
}

type warningCollector struct {
	mutex    sync.Mutex
	messages []string
}

func (collector *warningCollector) Add(message string) {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()
	collector.messages = append(collector.messages, message)
}

func (collector *warningCollector) Print(output *os.File) {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()
	for _, message := range collector.messages {
		fmt.Fprintf(output, "gitsy: warning: %s\n", message)
	}
}

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
		return usageError{err: parsed.Err}
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	warnings := &warningCollector{}

	repos := discover.DiscoverContext(ctx, discover.Options{
		Cwd:      options.Dir,
		MaxDepth: options.MaxDepth,
		Verbose:  options.Verbose,
		Warn:     warnings.Add,
	})

	noFetch := options.NoFetch
	if options.Sync {
		noFetch = false
	}

	var processWarn func(message string)
	if options.Verbose {
		processWarn = warnings.Add
	}

	err = tui.Run(ctx, cancel, os.Stdout, repos, noFetch, options.Sync, processWarn)
	if options.Verbose {
		warnings.Print(os.Stderr)
	}
	return err
}
