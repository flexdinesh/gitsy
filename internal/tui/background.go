package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// detectBackground settles the light or dark palette before Bubble Tea reads
// input. termenv skips its OSC 11 background query when TERM names tmux or
// screen, so light terminals there fell back to the dark palette. Querying
// with an xterm TERM lets tmux answer with the pane or client background.
// termenv follows the query with a cursor position request that multiplexers
// always answer, so an unanswered OSC 11 returns at once and falls back to
// COLORFGBG, then dark.
func detectBackground(output *os.File) {
	if !isMultiplexer(os.Getenv("TERM")) {
		return
	}
	query := termenv.NewOutput(output, termenv.WithEnvironment(queryEnviron{getenv: os.Getenv}))
	lipgloss.SetHasDarkBackground(query.HasDarkBackground())
}

func isMultiplexer(term string) bool {
	return strings.HasPrefix(term, "tmux") || strings.HasPrefix(term, "screen")
}

// queryEnviron reports multiplexer TERM values as xterm for the background query.
type queryEnviron struct {
	getenv func(string) string
}

func (environ queryEnviron) Environ() []string {
	return os.Environ()
}

func (environ queryEnviron) Getenv(key string) string {
	value := environ.getenv(key)
	if key == "TERM" && isMultiplexer(value) {
		return "xterm-256color"
	}
	return value
}
