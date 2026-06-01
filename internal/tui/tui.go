package tui

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/ui"
)

type inspector func(discover.Repo, bool, bool, func(string)) ui.RepoResult

const minWidth = 20

type Model struct {
	results []ui.RepoResult
	showAll bool
	noFetch bool
	sync    bool
	warn    func(string)
	spin    spinner.Model
	repos   viewport.Model
	width   int
	height  int
	done    int
	inspect inspector
}

type repoDoneMsg struct {
	index  int
	result ui.RepoResult
}

func Run(output *os.File, repos []discover.Repo, showAll bool, noFetch bool, syncRepos bool, warn func(string)) error {
	program := tea.NewProgram(
		NewModel(repos, showAll, noFetch, syncRepos, warn),
		tea.WithAltScreen(),
		tea.WithOutput(output),
	)
	_, err := program.Run()
	return err
}

func NewModel(repos []discover.Repo, showAll bool, noFetch bool, syncRepos bool, warn func(string)) Model {
	return newModel(repos, showAll, noFetch, syncRepos, warn, inspect.Repo)
}

func newModel(repos []discover.Repo, showAll bool, noFetch bool, syncRepos bool, warn func(string), inspect inspector) Model {
	results := make([]ui.RepoResult, len(repos))
	for index, repo := range repos {
		results[index] = ui.RepoResult{
			Repo:    repo,
			Loading: true,
		}
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot

	return Model{
		results: results,
		showAll: showAll,
		noFetch: noFetch,
		sync:    syncRepos,
		warn:    warn,
		spin:    spin,
		repos:   viewport.New(0, 0),
		inspect: inspect,
	}
}

func (model Model) Init() tea.Cmd {
	commands := make([]tea.Cmd, 0, len(model.results)+1)
	for index, result := range model.results {
		commands = append(commands, model.inspectRepo(index, result.Repo))
	}
	if len(model.results) > 0 {
		commands = append(commands, model.spin.Tick)
	}
	return tea.Batch(commands...)
}

func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.KeyMsg:
		if isQuitKey(msg) {
			return model, tea.Quit
		}
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		model.updateViewport()
		return model, nil
	case repoDoneMsg:
		if msg.index >= 0 && msg.index < len(model.results) && model.results[msg.index].Loading {
			model.results[msg.index] = msg.result
			model.done++
		}
		model.updateViewport()
		return model, nil
	case spinner.TickMsg:
		if model.done >= len(model.results) {
			return model, nil
		}
		var command tea.Cmd
		model.spin, command = model.spin.Update(msg)
		model.updateViewport()
		return model, command
	}

	var command tea.Cmd
	model.repos, command = model.repos.Update(message)
	return model, command
}

func (model Model) View() string {
	if model.width == 0 || model.height == 0 {
		return model.renderLegacy()
	}

	width := max(model.width, minWidth)
	title := model.renderTitle(width)
	titleHeight := lipgloss.Height(title)
	repoHeight := max(5, model.height-titleHeight-1)
	if repoHeight > model.height-titleHeight {
		repoHeight = max(1, model.height-titleHeight)
	}

	model.updateViewport()
	return strings.Join([]string{title, model.renderRepoSection(width, repoHeight)}, "\n")
}

func (model Model) inspectRepo(index int, repo discover.Repo) tea.Cmd {
	return func() tea.Msg {
		return repoDoneMsg{
			index:  index,
			result: model.inspect(repo, model.noFetch, model.sync, model.warn),
		}
	}
}

func isQuitKey(message tea.KeyMsg) bool {
	if message.Type == tea.KeyCtrlC {
		return true
	}
	if message.Type == tea.KeyRunes && len(message.Runes) == 1 {
		return message.Runes[0] == 'q' || message.Runes[0] == 'Q'
	}
	return message.String() == "q" || message.String() == "Q" || message.String() == "ctrl+c"
}

func (model *Model) updateViewport() {
	if model.width == 0 || model.height == 0 {
		return
	}

	width := max(model.width, minWidth)
	titleHeight := lipgloss.Height(model.renderTitle(width))
	repoHeight := max(5, model.height-titleHeight-1)
	viewportHeight := max(1, repoHeight-4)
	layout := ui.CreateFullWidthLayout(width, model.results)

	model.repos.Width = max(1, layout.Width-4)
	model.repos.Height = viewportHeight
	model.repos.SetContent(strings.Join(ui.RenderRows(layout, model.resultsWithSpinner(), model.showAll, ui.EmptyMessage(len(model.results), model.showAll)), "\n"))
}

func (model Model) renderTitle(width int) string {
	pending := len(model.results) - model.done
	mode := "fetch"
	if model.noFetch {
		mode = "local"
	}
	if model.sync {
		mode = "sync"
	}

	innerWidth := max(1, width-4)
	title := ui.FormatCell(ui.Title(model.results, model.showAll, len(model.results))+fmtStatus(mode, pending), innerWidth)

	return strings.Join([]string{
		style("cyan").Render("╭" + strings.Repeat("─", width-2) + "╮"),
		style("white").Render("│ " + title + " │"),
		style("cyan").Render("╰" + strings.Repeat("─", width-2) + "╯"),
	}, "\n")
}

func (model Model) renderRepoSection(width int, height int) string {
	layout := ui.CreateFullWidthLayout(width, model.results)
	sectionWidth := layout.Width
	contentWidth := max(1, sectionWidth-4)
	viewportHeight := max(1, height-4)

	model.repos.Width = contentWidth
	model.repos.Height = viewportHeight

	lines := []string{
		style("gray").Render("╭" + strings.Repeat("─", sectionWidth-2) + "╮"),
		style("white").Render("│ " + ui.FormatCell(ui.RenderHeader(layout), contentWidth) + " │"),
		style("gray").Render("├" + strings.Repeat("─", sectionWidth-2) + "┤"),
	}

	viewLines := strings.Split(model.repos.View(), "\n")
	for len(viewLines) < viewportHeight {
		viewLines = append(viewLines, "")
	}
	for _, line := range viewLines[:viewportHeight] {
		if line == "" {
			line = strings.Repeat(" ", contentWidth)
		}
		lines = append(lines, "│ "+line+" │")
	}

	lines = append(lines, style("gray").Render("╰"+strings.Repeat("─", sectionWidth-2)+"╯"))
	return strings.Join(lines, "\n")
}

func (model Model) renderLegacy() string {
	return ui.RenderString(model.resultsWithSpinner(), model.showAll, len(model.results), ui.EmptyMessage(len(model.results), model.showAll))
}

func (model Model) resultsWithSpinner() []ui.RepoResult {
	results := make([]ui.RepoResult, len(model.results))
	copy(results, model.results)
	for index := range results {
		if results[index].Loading {
			results[index].LoadingText = model.spin.View()
		}
	}
	return results
}

func fmtStatus(mode string, pending int) string {
	if pending <= 0 {
		return " • done"
	}
	return " • " + mode + " • " + strconv.Itoa(pending) + " pending"
}

func style(tone string) lipgloss.Style {
	base := lipgloss.NewStyle()
	switch tone {
	case "cyan":
		return base.Foreground(lipgloss.Color("14"))
	case "white":
		return base.Foreground(lipgloss.Color("15")).Bold(true)
	case "gray":
		return base.Foreground(lipgloss.Color("8"))
	default:
		return base
	}
}
