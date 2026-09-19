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

// Model holds a single viewport over the repo table: selected is the
// repo index, offset is the first visible body row. Detail rows never
// own selection; scrolling the viewport re-points selection at the
// first visible repo.
type Model struct {
	ctx      context.Context
	cancel   context.CancelFunc
	results  []ui.RepoResult
	noFetch  bool
	sync     bool
	warn     func(string)
	spin     spinner.Model
	width    int
	height   int
	selected int
	offset   int
	capacity int
	rows     []rowEntry
	next     int
	inspect  inspector

	tableWidth int
	tableOuter int
	infoOuter  int
	infoShown  bool
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
	spin.Spinner = spinner.Line

	return Model{
		ctx:      ctx,
		cancel:   cancel,
		results:  results,
		noFetch:  noFetch,
		sync:     syncRepos,
		warn:     warn,
		spin:     spin,
		next:     min(maxInspecting, len(repos)),
		inspect:  inspect,
		capacity: minTableHeight,
	}
}

func (model Model) Init() tea.Cmd {
	commands := make([]tea.Cmd, 0, model.next+1)
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
		model.refresh()
		return model, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				model.scrollViewport(-mouseWheelRows)
				return model, nil
			case tea.MouseButtonWheelDown:
				model.scrollViewport(mouseWheelRows)
				return model, nil
			}
		}
	case repoDoneMsg:
		if msg.index >= 0 && msg.index < len(model.results) && model.results[msg.index].Loading {
			model.results[msg.index] = msg.result
		}
		model.refresh()
		return model, model.nextInspectCommands()
	case spinner.TickMsg:
		if model.pending() == 0 {
			return model, nil
		}
		var command tea.Cmd
		model.spin, command = model.spin.Update(msg)
		model.refresh()
		return model, command
	}

	return model, nil
}

// View stacks three zones: borderless header bar, a panels row with the
// compact table left and a static info panel right, borderless footer.
// Pure: recomputes layout into locals and never mutates the model.
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

	tableOuter, infoOuter, infoShown := layoutWidths(width)
	tableWidth := max(1, tableOuter-2-padX*2)
	numberWidth, repoWidth, statusWidth := columnWidths(tableWidth, model.results)
	cols := [3]int{numberWidth, repoWidth, statusWidth}
	entries := buildEntries(model.spinnerResults(), model.selected)
	capacity := tableViewportHeight(height)
	offset := clamp(model.offset, 0, max(0, len(entries)-capacity))

	tableBox := tableStyle(tableOuter).Render(renderTableBody(entries, cols, offset, capacity, model.selected))
	middle := tableBox
	if infoShown {
		infoBox := renderInfo(infoOuter, lipgloss.Height(tableBox))
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

func (model Model) pending() int {
	pending := 0
	for _, result := range model.results {
		if result.Loading {
			pending++
		}
	}
	return pending
}

func (model Model) inFlight() int {
	return model.next - (len(model.results) - model.pending())
}

func (model *Model) nextInspectCommands() tea.Cmd {
	commands := []tea.Cmd{}
	for model.inFlight() < maxInspecting && model.next < len(model.results) {
		index := model.next
		model.next++
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
		model.moveRepo(-1)
	case "down", "j":
		model.moveRepo(1)
	case "pgup":
		model.scrollViewport(-model.pageStep())
	case "pgdown":
		model.scrollViewport(model.pageStep())
	case "u", "ctrl+u":
		model.scrollViewport(-model.halfPageStep())
	case "d", "ctrl+d":
		model.scrollViewport(model.halfPageStep())
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

func (model *Model) moveRepo(delta int) {
	model.selectRepo(model.selected + delta)
}

func (model *Model) selectRepo(index int) {
	if len(model.results) == 0 {
		return
	}
	model.selected = clamp(index, 0, len(model.results)-1)
	model.refresh()
}

// scrollViewport pans the table and re-points selection at the first
// visible repo, so selection always matches what is on screen.
func (model *Model) scrollViewport(delta int) {
	if len(model.rows) == 0 || delta == 0 {
		return
	}
	maxOffset := max(0, len(model.rows)-model.capacity)
	model.offset = clamp(model.offset+delta, 0, maxOffset)
	model.selected = firstRepoAt(model.rows, model.offset, model.selected)
	model.refresh()
}

// refresh rebuilds cached rows and keeps the selected repo visible.
// Selected repo sticks across rebuilds when earlier repos expand.
func (model *Model) refresh() {
	if model.width == 0 || model.height == 0 {
		return
	}
	width := max(model.width, minWidth)
	model.updateTableWithSize(width, tableViewportHeight(model.height))
}

func (model *Model) updateTableWithSize(width int, height int) {
	model.tableOuter, model.infoOuter, model.infoShown = layoutWidths(width)
	tableWidth := max(1, model.tableOuter-2-padX*2)
	model.tableWidth = tableWidth
	numberWidth, repoWidth, statusWidth := columnWidths(tableWidth, model.results)
	model.colWidths = [3]int{numberWidth, repoWidth, statusWidth}
	model.rows = buildEntries(model.spinnerResults(), model.selected)
	model.capacity = max(1, height)
	if len(model.results) > 0 {
		model.selected = clamp(model.selected, 0, len(model.results)-1)
	} else {
		model.selected = 0
	}
	model.ensureSelectedVisible()
}

func (model *Model) ensureSelectedVisible() {
	if len(model.rows) == 0 {
		model.offset = 0
		return
	}
	first := firstRowOf(model.rows, model.selected)
	if first < 0 {
		model.offset = clamp(model.offset, 0, max(0, len(model.rows)-model.capacity))
		return
	}
	last := first
	for index := first + 1; index < len(model.rows); index++ {
		if model.rows[index].divider {
			break
		}
		if model.rows[index].repoIndex != model.selected {
			break
		}
		last = index
	}
	// Keep offset if any row of the selected repo is already visible,
	// so panning into detail rows doesn't snap back to the top.
	if last >= model.offset && first < model.offset+model.capacity {
		model.offset = clamp(model.offset, 0, max(0, len(model.rows)-model.capacity))
		return
	}
	if first < model.offset {
		model.offset = first
	}
	if first >= model.offset+model.capacity {
		model.offset = first - model.capacity + 1
	}
	model.offset = clamp(model.offset, 0, max(0, len(model.rows)-model.capacity))
}

// firstRowOf returns the first body row belonging to a repo, or -1.
func firstRowOf(rows []rowEntry, repo int) int {
	for index, entry := range rows {
		if !entry.divider && entry.repoIndex == repo {
			return index
		}
	}
	return -1
}

// firstRepoAt returns the first repo at or after offset, scanning
// forward past dividers then falling back to the previous repo.
func firstRepoAt(rows []rowEntry, offset int, fallback int) int {
	for index := clamp(offset, 0, max(0, len(rows)-1)); index < len(rows); index++ {
		if !rows[index].divider && rows[index].repoIndex >= 0 {
			return rows[index].repoIndex
		}
	}
	for index := min(clamp(offset, 0, max(0, len(rows)-1)), len(rows)-1); index >= 0; index-- {
		if !rows[index].divider && rows[index].repoIndex >= 0 {
			return rows[index].repoIndex
		}
	}
	return fallback
}

func (model Model) spinnerResults() []ui.RepoResult {
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
	return buildEntries(model.spinnerResults(), model.selected)
}

func buildEntries(results []ui.RepoResult, selected int) []rowEntry {
	return buildEntryList(results, selected)
}

func buildEntryList(results []ui.RepoResult, selected int) []rowEntry {
	entries := []rowEntry{}
	for resultIndex, result := range results {
		rows := ui.RowsForRepo(result)
		if len(rows) == 0 {
			continue
		}
		if len(entries) > 0 {
			entries = append(entries, rowEntry{divider: true, repoIndex: -1})
		}

		digits := len(strconv.Itoa(max(1, len(results))))
		for rowIndex, row := range rows {
			number := ""
			marker := false
			repo := ""
			if rowIndex == 0 {
				marker = resultIndex == selected
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
		}
	}

	if len(entries) == 0 {
		message := ui.EmptyMessage(len(results))
		if message == "" {
			message = "No repositories to display."
		}
		return []rowEntry{{status: message, repoIndex: -1}}
	}

	return entries
}

// lineWidth is the full table body width: columns plus gaps.
func (model Model) lineWidth() int {
	return lineWidthFor(model.colWidths)
}

func lineWidthFor(cols [3]int) int {
	return cols[0] + cols[1] + cols[2] + columnGap*2
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

// renderHeader splits the bar: repo summary left, mode/pending right.
// Plain text is measured first, styled last, so ANSI never affects layout.
func (model Model) renderHeader(width int) string {
	content := max(1, width-spaceSM*2)
	left := ui.Title(model.results, len(model.results))
	right := model.headerRight()
	if runewidth.StringWidth(left)+spaceSM+runewidth.StringWidth(right) > content {
		if runewidth.StringWidth(right)+1 <= content {
			left = truncateCell(left, content-runewidth.StringWidth(right)-spaceSM)
		} else {
			left = truncateCell(left, content)
			return headerBarStyle(width).Render(headerTitleStyle().Render(left))
		}
	}
	gap := strings.Repeat(" ", content-runewidth.StringWidth(left)-runewidth.StringWidth(right))
	return headerBarStyle(width).Render(headerTitleStyle().Render(left) + gap + headerMetaStyle().Render(right))
}

// headerRight is the header meta without the leading separator: the gap
// between title and meta already separates them.
func (model Model) headerRight() string {
	pending := model.pending()
	mode := "fetch"
	if model.noFetch {
		mode = "local"
	}
	if model.sync {
		mode = "sync"
	}
	return strings.TrimPrefix(fmtStatus(mode, pending), " • ")
}

// renderInfo draws the static right panel. boxHeight matches the table
// panel so both bottoms align; tips are padded to fill, never scrolled.
func renderInfo(infoOuter int, boxHeight int) string {
	content := max(1, infoOuter-2-padX*2)
	lines := infoLines(content)
	want := max(1, boxHeight-2)
	for len(lines) < want {
		lines = append(lines, "")
	}
	lines = lines[:min(len(lines), want)]
	return infoStyle(infoOuter).Render(strings.Join(lines, "\n"))
}

// infoLines builds the static tips panel: plain text, truncation-safe.
func infoLines(width int) []string {
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

// renderTableBody draws a real table: column header, full-width rule,
// then the visible window of rows. Plain text is measured first, styles
// applied last, so ANSI never affects layout.
func renderTableBody(entries []rowEntry, cols [3]int, offset int, capacity int, selected int) string {
	numberWidth, repoWidth, statusWidth := cols[0], cols[1], cols[2]
	gap := strings.Repeat(" ", columnGap)
	header := padCell(truncateCell("#", numberWidth), numberWidth) + gap +
		padCell(truncateCell("REPO", repoWidth), repoWidth) + gap +
		padCell(truncateCell("STATUS", statusWidth), statusWidth)
	lines := []string{
		columnHeaderStyle().Render(header),
		dividerStyle().Render(strings.Repeat("─", max(1, lineWidthFor(cols)))),
	}

	visible := visibleSlice(entries, offset, capacity)
	if len(visible) == 0 {
		message := ui.EmptyMessage(len(entries))
		if message == "" {
			message = "No repositories to display."
		}
		lines = append(lines, truncateCell(message, max(1, lineWidthFor(cols))))
		return strings.Join(lines, "\n")
	}
	for _, entry := range visible {
		lines = append(lines, renderRow(entry, cols, selected))
	}
	return strings.Join(lines, "\n")
}

func visibleSlice(entries []rowEntry, offset int, capacity int) []rowEntry {
	if len(entries) == 0 || capacity <= 0 {
		return nil
	}
	start := clamp(offset, 0, max(0, len(entries)-1))
	end := min(start+max(1, capacity), len(entries))
	return entries[start:end]
}

func (model Model) visibleRows() []rowEntry {
	return visibleSlice(model.rows, model.offset, model.capacity)
}

// renderRow draws one body line. Selected rows get per-segment styles
// that each carry no background fill, keeping the highlight as bold
// text plus the › marker.
func renderRow(entry rowEntry, cols [3]int, selected int) string {
	if entry.divider {
		return dividerStyle().Render(strings.Repeat("─", max(1, lineWidthFor(cols))))
	}
	numberWidth, repoWidth, statusWidth := cols[0], cols[1], cols[2]
	gap := strings.Repeat(" ", columnGap)
	marker := " "
	if entry.marker {
		marker = iconSelected
	}
	number := padCell(truncateCell(marker+" "+entry.number, numberWidth), numberWidth)
	repo := padCell(truncateCell(entry.repo, repoWidth), repoWidth)
	status := padCell(truncateCell(entry.status, statusWidth), statusWidth)
	if entry.repoIndex == selected && entry.repoIndex >= 0 {
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

func (model Model) renderTable() string {
	return tableStyle(model.tableOuter).Render(renderTableBody(model.rows, model.colWidths, model.offset, model.capacity, model.selected))
}

func (model Model) renderRow(entry rowEntry) string {
	return renderRow(entry, model.colWidths, model.selected)
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
	_, _, infoShown := layoutWidths(max(width, minWidth))
	return footerStyle(width).Render(model.footerPlain(max(1, width-spaceSM*2), infoShown))
}

// footerPlain lays out the footer text to exactly content width. When
// the info panel is visible it already carries the key hints, so the
// footer keeps just position and quit.
func (model Model) footerPlain(content int, infoShown bool) string {
	position := ""
	if len(model.results) > 0 {
		position = strconv.Itoa(model.selected+1) + "/" + strconv.Itoa(len(model.results))
	}
	hints := []string{
		"↑/↓ j/k • PgUp/PgDn • wheel • q quit",
		"↑/↓ j/k • pg • wheel • q",
		"↑↓ • wheel • q",
	}
	if infoShown {
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

func clamp(value int, minValue int, maxValue int) int {
	return max(minValue, min(value, maxValue))
}
