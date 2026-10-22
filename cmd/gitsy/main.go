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

	"github.com/charmbracelet/x/term"
	"github.com/flexdinesh/gitsy/internal/args"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/tui"
	"github.com/flexdinesh/gitsy/internal/version"
)

const (
	exitFailed      = 1
	exitUsage       = 2
	exitInterrupted = 130
)

type usageError struct {
	err error
}

func (err usageError) Error() string {
	return fmt.Sprintf("%s\n\n%s", err.err, args.Usage)
}

// exitError carries an exit code; a nil err means details were already printed.
type exitError struct {
	code int
	err  error
}

func (err exitError) Error() string {
	if err.err == nil {
		return ""
	}
	return err.err.Error()
}

var errInterrupted = exitError{code: exitInterrupted, err: errors.New("interrupted")}

type warningCollector struct {
	mutex    sync.Mutex
	messages []string
}

func (collector *warningCollector) Add(message string) {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()
	collector.messages = append(collector.messages, message)
}

func (collector *warningCollector) Print(output io.Writer) {
	collector.mutex.Lock()
	defer collector.mutex.Unlock()
	for _, message := range collector.messages {
		fmt.Fprintf(output, "gitsy: warning: %s\n", message)
	}
}

func main() {
	os.Exit(exitCode(os.Stderr, run(os.Args[1:])))
}

// exitCode prints err, if it has a message, and maps it to a process exit code.
func exitCode(output io.Writer, err error) int {
	if err == nil {
		return 0
	}
	if message := err.Error(); message != "" {
		fmt.Fprintf(output, "gitsy: %s\n", message)
	}
	var usage usageError
	var exit exitError
	switch {
	case errors.As(err, &usage):
		return exitUsage
	case errors.As(err, &exit):
		return exit.code
	}
	return exitFailed
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
	interrupted := func() bool { return false }
	if options.Command != args.CommandTUI {
		signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()
		ctx = signalCtx
		interrupted = func() bool { return signalCtx.Err() != nil }
	}

	warnings := &warningCollector{}

	workspace, err := discover.DiscoverGroupedContext(ctx, discover.Options{
		Cwd:      cwd,
		Dirs:     options.Dirs,
		MaxDepth: options.MaxDepth,
		Verbose:  options.Verbose,
		Warn:     warnings.Add,
	})
	if interrupted() {
		return errInterrupted
	}
	if err != nil {
		return err
	}

	var processWarn func(message string)
	if options.Verbose {
		processWarn = warnings.Add
	}

	if options.Command == args.CommandTUI {
		noFetch := options.NoFetch && !options.Sync
		err = tui.Run(ctx, cancel, os.Stdout, workspace, noFetch, options.Sync, options.Base, processWarn)
	} else {
		err = runCommand(ctx, console{
			stdin:    os.Stdin,
			stdout:   os.Stdout,
			stderr:   os.Stderr,
			terminal: term.IsTerminal(os.Stdin.Fd()),
			verbose:  options.Verbose,
		}, workspace, options, processWarn)
	}
	if options.Verbose {
		warnings.Print(os.Stderr)
	}
	if interrupted() {
		return errInterrupted
	}
	return err
}
