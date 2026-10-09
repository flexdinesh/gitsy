package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/flexdinesh/gitsy/internal/args"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/tui"
	"github.com/flexdinesh/gitsy/internal/ui"
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
		fmt.Fprintf(os.Stdout, "gitsy %s\n", version.String())
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if options.Plain {
		signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx = signalCtx
		if _, err := fmt.Fprintln(os.Stdout, "Checking repositories…"); err != nil {
			return err
		}
	}

	warnings := &warningCollector{}

	workspace, err := discover.DiscoverGroupedContext(ctx, discover.Options{
		Cwd:      cwd,
		Dirs:     options.Dirs,
		MaxDepth: options.MaxDepth,
		Verbose:  options.Verbose,
		Warn:     warnings.Add,
	})
	if err != nil {
		return err
	}

	noFetch := options.NoFetch
	if options.Sync {
		noFetch = false
	}

	var processWarn func(message string)
	if options.Verbose {
		processWarn = warnings.Add
	}

	if options.Plain {
		err = runPlain(ctx, os.Stdout, workspace, noFetch, options.Sync, processWarn)
	} else {
		err = tui.Run(ctx, cancel, os.Stdout, workspace, noFetch, options.Sync, processWarn)
	}
	if options.Verbose {
		warnings.Print(os.Stderr)
	}
	return err
}

func runPlain(ctx context.Context, output io.Writer, workspace discover.Workspace, noFetch, syncRepos bool, warn func(string)) error {
	results := inspect.ReposContext(ctx, workspace.Repos, noFetch, syncRepos, warn)
	if err := ctx.Err(); err != nil {
		return err
	}
	return printPlainResults(output, workspace, results)
}

func printPlainResults(output io.Writer, workspace discover.Workspace, results []inspect.Result) error {
	if _, err := io.WriteString(output, ui.PlainReport(workspace, results)); err != nil {
		return err
	}
	for _, result := range results {
		if result.Failed || result.Stale || result.Sync != nil && result.Sync.Kind == "failed" {
			return errors.New("git operations failed (use --verbose for details)")
		}
	}
	return nil
}
