package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/ui"
	"github.com/mattn/go-runewidth"
)

type inspector func(context.Context, discover.Repo, bool, bool, func(string)) ui.RepoResult

// rowEntry is one rendered table body line. All text stays plain here;
// truncation and styling happen at render time so ANSI codes never
// pollute width measurement.
type rowEntry struct {
	number    string // right-aligned digits only; marker is separate
	marker    bool   // selected repo's first row shows ›
	repo      string
	status    string
	divider   bool
	tone      string
	bold      bool
	dim       bool
	repoIndex int // -1 for dividers and empty states
}

type Model struct {
	ctx        context.Context
	cancel     context.CancelFunc
	results    []ui.RepoResult
	noFetch    bool
	sync       bool
	warn       func(string)
	spin       spinner.Model
	width      int
	height     int
	overflow   bool
	selected   int
	cursorRow  int
	offset     int
	capacity   int
	rowScroll  bool
	rows       []rowEntry
	repoRows   []int
	rowRepos   []int
	tableWidth int
	tableOuter int
	infoOuter  int
	infoShown  bool
	done       int
	next       int
	active     int
	inspect    inspector
	colWidths  [3]int
}

type repoDoneMsg struct {
	index  int
	result ui.RepoResult
}

func Run(ctx context.Context, cancel context.CancelFunc, output *os.File, repos []discover.Repo, noFetch bool, syncRepos bool, warn func(string)) error {
	program := tea.NewProgram(
		newModel(ctx, cancel, repos, noFetch, syncRepos, warn, inspect.RepoContext),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithOutput(output),
	)
	_, err := program.Run()
	return err
}

func NewModel(repos []discover.Repo, noFetch bool, syncRepos bool, warn func(string)) Model {
	return newModel(context.Background(), nil, repos, noFetch, syncRepos, warn, inspect.RepoContext)
}

func newModel(ctx context.Context, cancel context.CancelFunc, repos []discover.Repo, noFetch bool, syncRepos bool, warn func(string), inspect inspector) Model {
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
	spin.Spinner = spinner.MiniDot
	active := min(maxInspecting, len(repos))

	return Model{
		ctx:      ctx,
		cancel:   cancel,
		results:  results,
		noFetch:  noFetch,
		sync:     syncRepos,
		warn:     warn,
		spin:     spin,
		next:     active,
		active:   active,
		inspect:  inspect,
		capacity: minTableHeight,
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
		if model.navigate(msg) {
			return model, nil
		}
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		model.updateTable()
		return model, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				model.scrollRows(-mouseWheelRows)
				return model, nil
			case tea.MouseButtonWheelDown:
				model.scrollRows(mouseWheelRows)
				return model, nil
			}
		}
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

	return model, nil
}

// View stacks three zones: borderless header bar, a panels row with the
// compact table left and a static info panel right, borderless footer.
// Only the table viewport scrolls; the info panel never moves.
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

	model.updateTableWithSize(width, tableViewportHeight(height))
	tableBox := model.renderTable()
	middle := tableBox
	if model.infoShown {
		infoBox := model.renderInfo(lipgloss.Height(tableBox))
		gap := lipgloss.NewStyle().
			Width(panelGap).
			Height(lipgloss.Height(tableBox)).
			Render("")
		middle = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, gap, infoBox)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		model.renderHeader(width),
		middle,
		model.renderFooter(width),
	)
}

func tableViewportHeight(height int) int {
	// Header bar (1) + table border (2) + column header (1) +
	// header rule (1) + footer bar (1).
	return max(minTableHeight, height-6)
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

func (model *Model) navigate(message tea.KeyMsg) bool {
	pressed := message.String()
	switch pressed {
	case "up", "k":
		model.moveSelection(-1)
	case "down", "j":
		model.moveSelection(1)
	case "pgup":
		model.scrollRows(-model.pageStep())
	case "pgdown":
		model.scrollRows(model.pageStep())
	case "u", "ctrl+u":
		model.scrollRows(-model.halfPageStep())
	case "d", "ctrl+d":
		model.scrollRows(model.halfPageStep())
	case "home", "g":
		model.selectRepo(0)
	case "end", "G":
		model.selectRepo(len(model.results) - 1)
	default:
		return false
	}
	return true
}

func (model Model) pageStep() int {
	return max(1, model.capacity)
}

func (model Model) halfPageStep() int {
	return max(1, model.capacity/2)
}

func (model *Model) moveSelection(delta int) {
	model.selectRepo(model.selected + delta)
}

func (model *Model) scrollRows(delta int) {
	if len(model.rowRepos) == 0 || delta == 0 {
		return
	}

	target := clamp(model.cursorRow+delta, 0, len(model.rowRepos)-1)
	step := 1
	if delta < 0 {
		step = -1
	}
	for target >= 0 && target < len(model.rowRepos) && model.rowRepos[target] < 0 {
		target += step
	}
	target = clamp(target, 0, len(model.rowRepos)-1)
	if model.rowRepos[target] >= 0 {
		model.selected = model.rowRepos[target]
	}
	model.cursorRow = target
	model.rowScroll = true
	model.ensureVisible()
}

func (model *Model) selectRepo(index int) {
	if len(model.results) == 0 {
		return
	}
	model.selected = clamp(index, 0, len(model.results)-1)
	model.rowScroll = false
	model.updateTable()
}

func (model *Model) updateTable() {
	if model.width == 0 || model.height == 0 {
		return
	}

	width := max(model.width, minWidth)
	model.updateTableWithSize(width, tableViewportHeight(model.height))
}

// renderHeader splits the bar: repo summary left, mode/pending right.
// Plain text is measured first, styled last, so ANSI never affects layout.
func (model Model) renderHeader(width int) string {
	content := max(1, width-spaceSM*2)
	plain := model.headerPlain(content)
	return headerBarStyle(width).Render(model.styleHeaderLine(plain, content))
}

// headerPlain lays out the header text to exactly content width.
func (model Model) headerPlain(content int) string {
	left := ui.Title(model.results, len(model.results))
	right := model.headerRight()
	if runewidth.StringWidth(left)+spaceSM+runewidth.StringWidth(right) <= content {
		return left + strings.Repeat(" ", content-runewidth.StringWidth(left)-runewidth.StringWidth(right)) + right
	}
	if runewidth.StringWidth(right)+1 <= content {
		left = truncateCell(left, content-runewidth.StringWidth(right)-spaceSM)
		return left + strings.Repeat(" ", content-runewidth.StringWidth(left)-runewidth.StringWidth(right)) + right
	}
	return truncateCell(left, content)
}

// headerRight is the header meta without the leading separator: the gap
// between title and meta already separates them.
func (model Model) headerRight() string {
	pending := len(model.results) - model.done
	mode := "fetch"
	if model.noFetch {
		mode = "local"
	}
	if model.sync {
		mode = "sync"
	}
	return strings.TrimPrefix(fmtStatus(mode, pending), " • ")
}

// styleHeaderLine colors the title bright and the trailing meta dim by
// re-splitting the already-laid-out plain line.
func (model Model) styleHeaderLine(plain string, content int) string {
	right := model.headerRight()
	if runewidth.StringWidth(right) < runewidth.StringWidth(plain) && strings.HasSuffix(plain, right) {
		leftPart := strings.TrimSuffix(plain[:len(plain)-len(right)], " ")
		gap := strings.Repeat(" ", content-runewidth.StringWidth(leftPart)-runewidth.StringWidth(right))
		return headerTitleStyle().Render(leftPart) + gap + headerMetaStyle().Render(right)
	}
	return headerTitleStyle().Render(plain)
}

// renderTable wraps the table body in the left bordered panel.
func (model Model) renderTable() string {
	return tableStyle(model.tableOuter).Render(model.renderTableBody())
}

// renderInfo draws the static right panel. boxHeight matches the table
// panel so both bottoms align; tips are padded to fill, never scrolled.
func (model Model) renderInfo(boxHeight int) string {
	content := max(1, model.infoOuter-2-padX*2)
	lines := model.infoLines(content)
	want := max(1, boxHeight-2)
	for len(lines) < want {
		lines = append(lines, "")
	}
	lines = lines[:min(len(lines), want)]
	return infoStyle(model.infoOuter).Render(strings.Join(lines, "\n"))
}

// infoLines builds the static tips panel: plain text, truncation-safe.
func (model Model) infoLines(width int) []string {
	lines := []string{
		columnHeaderStyle().Render(truncateCell("INFO", width)),
		dividerStyle().Render(strings.Repeat("─", max(1, width))),
	}
	sections := []struct {
		label   string
		entries []string
	}{
		{"NAVIGATE", []string{"↑/↓ j/k · move", "PgUp/PgDn · page", "g / G · ends", "wheel · scroll"}},
		{"SELECT", []string{"› · current repo"}},
		{"QUIT", []string{"q · quit"}},
	}
	for index, section := range sections {
		if index > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, infoSectionStyle().Render(truncateCell(section.label, width)))
		for _, entry := range section.entries {
			lines = append(lines, truncateCell(entry, width))
		}
	}
	return lines
}

// layoutWidths splits the terminal into table and info panels: roughly
// 2/3 table and 1/3 info. Either panel below its minimum collapses to a
// full-width table with the footer carrying the key hints instead.
func layoutWidths(termWidth int) (tableOuter int, infoOuter int, infoShown bool) {
	infoOuter = clamp(termWidth/3, infoMinOuter, infoMaxOuter)
	if termWidth-infoOuter-panelGap < tableMinOuter {
		return termWidth, 0, false
	}
	return termWidth - infoOuter - panelGap, infoOuter, true
}

// renderTitle is kept for tests; it renders the header text without chrome.
func (model Model) renderTitle(width int) string {
	return model.headerPlain(max(1, width-2-padX*2))
}

func (model *Model) updateTableWithSize(width int, height int) {
	model.tableOuter, model.infoOuter, model.infoShown = layoutWidths(width)
	tableWidth := max(1, model.tableOuter-2-padX*2)
	model.tableWidth = tableWidth
	numberWidth, repoWidth, statusWidth := columnWidths(tableWidth, model.results)
	model.colWidths = [3]int{numberWidth, repoWidth, statusWidth}
	entries, repoRows, rowRepos := model.buildRows()
	model.rows = entries
	model.repoRows = repoRows
	model.rowRepos = rowRepos

	model.capacity = max(1, height)
	model.overflow = len(entries) > model.capacity
	if len(model.repoRows) > 0 {
		model.selected = clamp(model.selected, 0, len(model.repoRows)-1)
		if model.rowScroll {
			model.cursorRow = clamp(model.cursorRow, 0, len(model.rowRepos)-1)
			if model.rowRepos[model.cursorRow] >= 0 {
				model.selected = model.rowRepos[model.cursorRow]
			}
		} else {
			model.cursorRow = model.repoRows[model.selected]
		}
	}
	model.ensureVisible()
}

func (model *Model) ensureVisible() {
	if len(model.rows) == 0 {
		model.offset = 0
		return
	}
	model.cursorRow = clamp(model.cursorRow, 0, len(model.rows)-1)
	if model.cursorRow < model.offset {
		model.offset = model.cursorRow
	}
	if model.cursorRow >= model.offset+model.capacity {
		model.offset = model.cursorRow - model.capacity + 1
	}
	maxOffset := max(0, len(model.rows)-model.capacity)
	model.offset = clamp(model.offset, 0, maxOffset)
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

func (model Model) tableRows() []rowEntry {
	rows, _, _ := model.buildRows()
	return rows
}

func (model Model) buildRows() ([]rowEntry, []int, []int) {
	results := model.resultsWithSpinner()
	entries := []rowEntry{}
	repoRows := make([]int, 0, len(results))
	rowRepos := []int{}
	for resultIndex, result := range results {
		rows := ui.RowsForRepo(result)
		if len(rows) == 0 {
			continue
		}
		if len(entries) > 0 {
			entries = append(entries, rowEntry{divider: true, repoIndex: -1})
			rowRepos = append(rowRepos, -1)
		}

		repoRows = append(repoRows, len(entries))
		digits := len(strconv.Itoa(max(1, len(results))))
		for rowIndex, row := range rows {
			number := ""
			marker := false
			repo := ""
			if rowIndex == 0 {
				marker = resultIndex == model.selected
				number = fmt.Sprintf("%*d", digits, resultIndex+1)
				repo = row.Repo
			}
			entries = append(entries, rowEntry{
				number:    number,
				marker:    marker,
				repo:      repo,
				status:    row.Text,
				tone:      row.Tone,
				bold:      row.Bold,
				dim:       row.Dim,
				repoIndex: resultIndex,
			})
			rowRepos = append(rowRepos, resultIndex)
		}
	}

	if len(entries) == 0 {
		message := ui.EmptyMessage(len(model.results))
		if message == "" {
			message = "No repositories to display."
		}
		return []rowEntry{{status: message, repoIndex: -1}}, nil, []int{-1}
	}

	return entries, repoRows, rowRepos
}

// lineWidth is the full table body width: columns plus gaps.
func (model Model) lineWidth() int {
	return model.colWidths[0] + model.colWidths[1] + model.colWidths[2] + columnGap*2
}

func truncateCell(value string, width int) string {
	return runewidth.Truncate(value, max(0, width), "…")
}

func padCell(value string, width int) string {
	if missing := width - runewidth.StringWidth(value); missing > 0 {
		return value + strings.Repeat(" ", missing)
	}
	return value
}

// renderTableBody draws a real table: column header, full-width rule,
// then the visible window of rows. Plain text is measured first, styles
// applied last, so ANSI never affects layout.
func (model Model) renderTableBody() string {
	numberWidth, repoWidth, statusWidth := model.colWidths[0], model.colWidths[1], model.colWidths[2]
	gap := strings.Repeat(" ", columnGap)
	header := padCell(truncateCell("#", numberWidth), numberWidth) + gap +
		padCell(truncateCell("REPO", repoWidth), repoWidth) + gap +
		padCell(truncateCell("STATUS", statusWidth), statusWidth)
	lines := []string{
		columnHeaderStyle().Render(header),
		dividerStyle().Render(strings.Repeat("─", max(1, model.lineWidth()))),
	}

	visible := model.visibleRows()
	if len(visible) == 0 {
		message := ui.EmptyMessage(len(model.results))
		if message == "" {
			message = "No repositories to display."
		}
		lines = append(lines, truncateCell(message, max(1, model.lineWidth())))
		return strings.Join(lines, "\n")
	}
	for _, entry := range visible {
		lines = append(lines, model.renderRow(entry))
	}
	return strings.Join(lines, "\n")
}

func (model Model) visibleRows() []rowEntry {
	if len(model.rows) == 0 || model.capacity <= 0 {
		return nil
	}
	start := clamp(model.offset, 0, max(0, len(model.rows)-1))
	end := min(start+max(1, model.capacity), len(model.rows))
	return model.rows[start:end]
}

// renderRow draws one body line. Selected rows get per-segment styles
// that each carry the background, keeping the highlight continuous
// across the full row while preserving status tones.
func (model Model) renderRow(entry rowEntry) string {
	if entry.divider {
		return dividerStyle().Render(strings.Repeat("─", max(1, model.lineWidth())))
	}
	numberWidth, repoWidth, statusWidth := model.colWidths[0], model.colWidths[1], model.colWidths[2]
	gap := strings.Repeat(" ", columnGap)
	marker := " "
	if entry.marker {
		marker = iconSelected
	}
	number := padCell(truncateCell(marker+" "+entry.number, numberWidth), numberWidth)
	repo := padCell(truncateCell(entry.repo, repoWidth), repoWidth)
	status := padCell(truncateCell(entry.status, statusWidth), statusWidth)
	if entry.repoIndex == model.selected && entry.repoIndex >= 0 {
		mark := " "
		if entry.marker {
			mark = selectedMarkerStyle().Render(iconSelected)
		}
		return mark +
			selectedNumStyle().Render(padCell(truncateCell(" "+entry.number, numberWidth-1), numberWidth-1)) +
			gap +
			selectedNumStyle().Render(repo) +
			gap +
			toneStyle(entry.tone, entry.bold, entry.dim).Render(status)
	}
	return number + gap + repo + gap + toneStyle(entry.tone, entry.bold, entry.dim).Render(status)
}

func columnWidths(width int, results []ui.RepoResult) (int, int, int) {
	// Marker + space + right-aligned digits, e.g. "› 1" / "  26".
	numberWidth := max(3, len(strconv.Itoa(max(1, len(results))))+2)
	contentWidth := max(16, width-numberWidth-columnGap*2)
	longestRepoName := runewidth.StringWidth("REPO")
	for _, result := range results {
		longestRepoName = max(longestRepoName, runewidth.StringWidth(result.Repo.DisplayName))
	}

	maxRepoWidth := min(maxRepoWidthCap, max(8, contentWidth*35/100))
	minRepoWidth := min(18, maxRepoWidth)
	repoWidth := clamp(longestRepoName, minRepoWidth, maxRepoWidth)
	statusWidth := max(8, contentWidth-repoWidth)
	return numberWidth, repoWidth, statusWidth
}

func fmtStatus(mode string, pending int) string {
	if pending <= 0 {
		return " • done"
	}
	return " • " + mode + " • " + strconv.Itoa(pending) + " pending"
}

// renderFooter is always shown: position left, key hints right.
// Both parts collapse gracefully at narrow widths.
func (model Model) renderFooter(width int) string {
	return footerStyle(width).Render(model.footerPlain(max(1, width-spaceSM*2)))
}

// footerPlain lays out the footer text to exactly content width. When
// the info panel is visible it already carries the key hints, so the
// footer keeps just position and quit.
func (model Model) footerPlain(content int) string {
	position := ""
	if len(model.results) > 0 {
		position = strconv.Itoa(model.selected+1) + "/" + strconv.Itoa(len(model.results))
	}
	hints := []string{
		"↑/↓ j/k • PgUp/PgDn • wheel • q quit",
		"↑/↓ j/k • pg • wheel • q",
		"↑↓ • wheel • q",
	}
	if model.infoShown {
		hints = []string{"q quit"}
	}
	hint := hints[len(hints)-1]
	for _, option := range hints {
		if runewidth.StringWidth(option) <= max(1, content-runewidth.StringWidth(position)-spaceSM) || position == "" && runewidth.StringWidth(option) <= content {
			hint = option
			break
		}
	}
	if position == "" {
		return truncateCell(hint, content)
	}
	if runewidth.StringWidth(position)+spaceSM+runewidth.StringWidth(hint) <= content {
		return position + strings.Repeat(" ", content-runewidth.StringWidth(position)-runewidth.StringWidth(hint)) + hint
	}
	if runewidth.StringWidth(position) <= content {
		return truncateCell(position+" • "+hint, content)
	}
	return truncateCell(hint, content)
}

func (model Model) renderScrollHint(width int) string {
	return model.footerPlain(max(1, width))
}

func clamp(value int, minValue int, maxValue int) int {
	return max(minValue, min(value, maxValue))
}
