package tui

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/ui"
	"github.com/mattn/go-runewidth"
)

type inspector func(context.Context, discover.Repo, bool, bool, func(string)) ui.RepoResult

const (
	minWidth       = 32
	minTableHeight = 4
	maxInspecting  = 8
)

type Model struct {
	ctx     context.Context
	cancel  context.CancelFunc
	results []ui.RepoResult
	showAll bool
	noFetch bool
	sync    bool
	warn    func(string)
	spin    spinner.Model
	repos   table.Model
	width   int
	height  int
	done    int
	next    int
	active  int
	inspect inspector
}

type repoDoneMsg struct {
	index  int
	result ui.RepoResult
}

func Run(ctx context.Context, cancel context.CancelFunc, output *os.File, repos []discover.Repo, showAll bool, noFetch bool, syncRepos bool, warn func(string)) error {
	program := tea.NewProgram(
		newModel(ctx, cancel, repos, showAll, noFetch, syncRepos, warn, inspect.RepoContext),
		tea.WithAltScreen(),
		tea.WithOutput(output),
	)
	_, err := program.Run()
	return err
}

func NewModel(repos []discover.Repo, showAll bool, noFetch bool, syncRepos bool, warn func(string)) Model {
	return newModel(context.Background(), nil, repos, showAll, noFetch, syncRepos, warn, inspect.RepoContext)
}

func newModel(ctx context.Context, cancel context.CancelFunc, repos []discover.Repo, showAll bool, noFetch bool, syncRepos bool, warn func(string), inspect inspector) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	results := make([]ui.RepoResult, len(repos))
	for index, repo := range repos {
		results[index] = ui.RepoResult{
			Repo:    repo,
			Loading: true,
		}
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	active := min(maxInspecting, len(repos))

	return Model{
		ctx:     ctx,
		cancel:  cancel,
		results: results,
		showAll: showAll,
		noFetch: noFetch,
		sync:    syncRepos,
		warn:    warn,
		spin:    spin,
		repos: table.New(
			table.WithFocused(true),
			table.WithStyles(tableStyles()),
		),
		next:    active,
		active:  active,
		inspect: inspect,
	}
}

func (model Model) Init() tea.Cmd {
	commands := make([]tea.Cmd, 0, model.active+1)
	for index := 0; index < model.next; index++ {
		result := model.results[index]
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
			if model.cancel != nil {
				model.cancel()
			}
			return model, tea.Quit
		}
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		model.updateTable()
		return model, nil
	case repoDoneMsg:
		if msg.index >= 0 && msg.index < len(model.results) && model.results[msg.index].Loading {
			model.results[msg.index] = msg.result
			model.done++
			model.active--
		}
		model.updateTable()
		return model, model.nextInspectCommands()
	case spinner.TickMsg:
		if model.done >= len(model.results) {
			return model, nil
		}
		var command tea.Cmd
		model.spin, command = model.spin.Update(msg)
		model.updateTable()
		return model, command
	}

	var command tea.Cmd
	model.repos, command = model.repos.Update(message)
	return model, command
}

func (model Model) View() string {
	width := model.width
	height := model.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}

	width = max(width, minWidth)
	title := model.renderTitle(width)
	titleHeight := lipgloss.Height(title)
	repoHeight := max(minTableHeight+2, height-titleHeight-1)

	model.updateTableWithSize(width, repoHeight-2)
	return strings.Join([]string{title, model.renderTable(width, repoHeight)}, "\n")
}

func (model Model) inspectRepo(index int, repo discover.Repo) tea.Cmd {
	return func() tea.Msg {
		return repoDoneMsg{
			index:  index,
			result: model.inspect(model.ctx, repo, model.noFetch, model.sync, model.warn),
		}
	}
}

func (model *Model) nextInspectCommands() tea.Cmd {
	commands := []tea.Cmd{}
	for model.active < maxInspecting && model.next < len(model.results) {
		index := model.next
		model.next++
		model.active++
		commands = append(commands, model.inspectRepo(index, model.results[index].Repo))
	}
	return tea.Batch(commands...)
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

func (model *Model) updateTable() {
	if model.width == 0 || model.height == 0 {
		return
	}

	width := max(model.width, minWidth)
	titleHeight := lipgloss.Height(model.renderTitle(width))
	repoHeight := max(minTableHeight+2, model.height-titleHeight-1)
	model.updateTableWithSize(width, repoHeight-2)
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

	title := ui.Title(model.results, model.showAll, len(model.results)) + fmtStatus(mode, pending)
	return frameStyle(width, 1).Render(titleStyle(width).Render(title))
}

func (model *Model) updateTableWithSize(width int, height int) {
	tableWidth := max(1, width-4)
	repoWidth, statusWidth := columnWidths(tableWidth, model.results)

	model.repos.SetColumns([]table.Column{
		{Title: "REPO", Width: repoWidth},
		{Title: "STATUS", Width: statusWidth},
	})
	model.repos.SetRows(model.tableRows())
	model.repos.SetWidth(tableWidth)
	model.repos.SetHeight(max(minTableHeight, height))
}

func (model Model) renderTable(width int, height int) string {
	return frameStyle(width, max(minTableHeight, height-2)).Render(model.repos.View())
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

func (model Model) tableRows() []table.Row {
	rows := ui.BuildRows(model.resultsWithSpinner(), model.showAll)
	tableRows := make([]table.Row, 0, len(rows))
	previousRepo := ""
	for _, row := range rows {
		repo := ""
		if row.Repo != "" && row.Repo != previousRepo {
			if previousRepo != "" {
				tableRows = append(tableRows, table.Row{"", ""})
			}
			repo = row.Repo
			previousRepo = row.Repo
		}
		tableRows = append(tableRows, table.Row{
			repo,
			model.renderStatusCell(row),
		})
	}

	if len(tableRows) == 0 {
		message := ui.EmptyMessage(len(model.results), model.showAll)
		if message == "" {
			message = "No repositories to display."
		}
		return []table.Row{{"", message}}
	}

	return tableRows
}

func (model Model) renderStatusCell(row ui.Row) string {
	return toneStyle(row.Tone, row.Bold, row.Dim).Render(row.Text)
}

func columnWidths(width int, results []ui.RepoResult) (int, int) {
	contentWidth := max(16, width-4)
	longestRepoName := runewidth.StringWidth("REPO")
	for _, result := range results {
		longestRepoName = max(longestRepoName, runewidth.StringWidth(result.Repo.DisplayName))
	}

	maxRepoWidth := max(8, contentWidth*35/100)
	minRepoWidth := min(18, maxRepoWidth)
	repoWidth := clamp(longestRepoName, minRepoWidth, maxRepoWidth)
	statusWidth := max(8, contentWidth-repoWidth)
	return repoWidth, statusWidth
}

func fmtStatus(mode string, pending int) string {
	if pending <= 0 {
		return " • done"
	}
	return " • " + mode + " • " + strconv.Itoa(pending) + " pending"
}

func tableStyles() table.Styles {
	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("8")).
		BorderBottom(true)
	styles.Cell = lipgloss.NewStyle()
	styles.Selected = styles.Cell
	return styles
}

func frameStyle(width int, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Width(max(1, width-2)).
		Height(max(1, height)).
		Padding(0, 1)
}

func titleStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("14")).
		Bold(true).
		MaxWidth(max(1, width-4))
}

func toneStyle(tone string, bold bool, dim bool) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(bold).Faint(dim)
	switch tone {
	case "red":
		return style.Foreground(lipgloss.Color("9"))
	case "green":
		return style.Foreground(lipgloss.Color("10"))
	case "yellow":
		return style.Foreground(lipgloss.Color("11"))
	case "blue":
		return style.Foreground(lipgloss.Color("12"))
	case "magenta":
		return style.Foreground(lipgloss.Color("13"))
	case "cyan":
		return style.Foreground(lipgloss.Color("14"))
	case "gray":
		return style.Foreground(lipgloss.Color("8"))
	case "white":
		return style.Foreground(lipgloss.Color("15"))
	default:
		return style
	}
}

func clamp(value int, minValue int, maxValue int) int {
	return max(minValue, min(value, maxValue))
}
