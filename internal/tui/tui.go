package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flexdinesh/gitsy/internal/discover"
	"github.com/flexdinesh/gitsy/internal/git"
	"github.com/flexdinesh/gitsy/internal/inspect"
	"github.com/flexdinesh/gitsy/internal/ui"
	"github.com/mattn/go-runewidth"
)

type inspector func(context.Context, discover.Repo, bool, bool, func(string)) inspect.Result

// rowEntry is one rendered table body line. All text stays plain here;
// truncation and styling happen at render time so ANSI codes never
// pollute width measurement.
type rowEntry struct {
	number     string // right-aligned digits only; marker is separate
	marker     bool   // selected repo's first row shows ›
	repo       string
	worktree   string
	status     string
	compact    string
	divider    bool
	tone       string
	bold       bool
	dim        bool
	repoIndex  int // appearance index; -1 for group headers, dividers and empty states
	group      string
	groupSize  int
	groupDone  int
	groupLabel string
}

type tableColumns struct {
	number, repo, worktree, status int
}

// Model holds unique results and a viewport over repo appearances: selected
// is the appearance index, offset the first body row. Detail rows never
// own selection; scrolling the viewport re-points selection at the
// first visible repo.
type Model struct {
	ctx         context.Context
	cancel      context.CancelFunc
	results     []ui.RepoResult
	groups      []discover.Group
	home        string
	noFetch     bool
	sync        bool
	warn        func(string)
	spin        spinner.Model
	width       int
	height      int
	selected    int
	expanded    bool
	browseEmpty bool
	offset      int
	capacity    int
	rows        []rowEntry
	next        int
	inspect     inspector
	worktrees   bool
	inactive    navigationState
	removed     map[string]bool
	confirm     *discover.Repo
	force       bool
	deleting    string
	notice      string
	remove      func(context.Context, string, string, bool) git.WorktreeRemovalResult

	tableWidth int
	tableOuter int
	infoOuter  int
	infoShown  bool
	colWidths  tableColumns
}

type repoDoneMsg struct {
	index  int
	result inspect.Result
}

func Run(ctx context.Context, cancel context.CancelFunc, output *os.File, workspace discover.Workspace, noFetch bool, syncRepos bool, warn func(string)) error {
	model := newModel(ctx, cancel, workspace.Repos, noFetch, syncRepos, warn, inspect.RepoContext)
	model.groups = workspace.Groups
	program := tea.NewProgram(
		model,
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
	spin.Spinner = spinner.Dot
	home, _ := os.UserHomeDir()

	return Model{
		ctx:      ctx,
		cancel:   cancel,
		results:  results,
		home:     home,
		noFetch:  noFetch,
		sync:     syncRepos,
		warn:     warn,
		spin:     spin,
		next:     min(maxInspecting, len(repos)),
		inspect:  inspect,
		remove:   git.RemoveWorktreeContext,
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
		if handled, command := model.worktreeKey(msg); handled {
			return model, command
		}
		if model.navigate(msg) {
			return model, nil
		}
	case tea.WindowSizeMsg:
		model.width = msg.Width
		model.height = msg.Height
		model.refresh()
		if model.browseEmpty {
			if index := firstRepoAt(model.visibleRows(), 0, -1); index >= 0 {
				model.selectRepo(index)
			}
		}
		return model, nil
	case tea.MouseMsg:
		if model.confirm != nil || model.deleting != "" {
			return model, nil
		}
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
			model.results[msg.index] = ui.RepoResult{
				Repo:   msg.result.Repo,
				Status: msg.result.Status,
				Failed: msg.result.Failed,
				Stale:  msg.result.Stale,
				Sync:   msg.result.Sync,
			}
		}
		model.refresh()
		return model, model.nextInspectCommands()
	case worktreeRemovedMsg:
		model.deleting = ""
		if msg.result.OK {
			if model.removed == nil {
				model.removed = map[string]bool{}
			}
			model.removed[msg.repo.Path] = true
			model.notice = "Deleted worktree. Branch kept."
		} else if msg.result.NeedsForce {
			model.confirm = &msg.repo
			model.force = true
			model.notice = ""
		} else {
			model.notice = "Delete failed: " + strings.TrimSpace(msg.result.Stderr)
			if strings.TrimSpace(msg.result.Stderr) == "" {
				model.notice = fmt.Sprintf("Delete failed: git exited %d", msg.result.Status)
			}
		}
		model.browseEmpty = false
		model.refresh()
		return model, nil
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

// View derives the ledger and selected-repository context without I/O.
func (model Model) View() string {
	width := model.width
	height := model.height
	if width == 0 {
		width = 80
	}
	if height == 0 {
		height = 24
	}
	if width < minWidth || height < 9+model.actionHeight() {
		return model.compactView(max(1, width), max(1, height))
	}

	tableOuter, infoOuter, infoShown := layoutWidths(width)
	tableWidth := max(1, tableOuter-2-padX*2)
	cols := model.columns(tableWidth)
	entries := model.tableRows()
	capacity := model.viewportHeight(height)
	offset := clamp(model.offset, 0, maxViewportOffset(entries, capacity))

	tableBox := tableStyle(tableOuter).Render(renderTableBody(entries, cols, offset, capacity, model.selected))
	middle := tableBox
	if infoShown {
		infoBox := model.renderInfo(infoOuter, lipgloss.Height(tableBox))
		gap := lipgloss.NewStyle().
			Width(panelGap).
			Height(lipgloss.Height(tableBox)).
			Render("")
		middle = lipgloss.JoinHorizontal(lipgloss.Top, tableBox, gap, infoBox)
	}
	sections := []string{model.renderHeader(width), middle}
	if action := model.renderAction(width); action != "" {
		sections = append(sections, action)
	}
	return lipgloss.JoinVertical(lipgloss.Left, append(sections, model.renderFooter(width))...)
}

func tableViewportHeight(height int) int {
	// Header, summary and tabs (4), column heading and rule (2), footer (1).
	return max(1, height-7)
}

func (model Model) compactView(width, height int) string {
	done, total := model.progress()
	count := fmt.Sprintf("%d/%d", done, total)
	lines := []string{padCell(truncateCell(model.title(), max(0, width-len(count)-spaceSM)), max(0, width-len(count))) + count}
	if len(model.displayResults()) == 0 || model.browseEmpty {
		message := model.emptyMessage()
		if model.browseEmpty && len(model.visibleRows()) > 0 {
			group := model.visibleRows()[0]
			if group.group == "" {
				for index := min(model.offset, len(model.rows)-1); index >= 0; index-- {
					if model.rows[index].group != "" {
						group = model.rows[index]
						break
					}
				}
			}
			lines = append(lines, renderGroup(group, width, false))
			if group.groupSize > 0 {
				message = fmt.Sprintf("%d repos", group.groupSize)
				if model.worktrees {
					message = fmt.Sprintf("%d worktrees", group.groupSize)
				}
			}
		} else if groups := model.displayGroups(); len(groups) > 0 {
			lines = append(lines, renderGroup(model.groupEntry(groups[0]), width, false))
		}
		lines = append(lines, message)
	} else {
		result := model.selectedResult()
		if path := model.selectedGroup(); path != "" {
			for _, group := range model.displayGroups() {
				if group.Path == path {
					lines = append(lines, renderGroup(model.groupEntry(group), width, false))
					break
				}
			}
		}
		name := result.Repo.DisplayName
		if model.worktrees {
			name = worktreeIdentity(result.Repo, max(1, width-2))
		}
		lines = append(lines, iconSelected+" "+name)
		for _, row := range ui.RowsForRepo(result) {
			lines = append(lines, strings.TrimSpace(row.Text))
		}
	}
	if height > 1 {
		if action := model.actionLines(); len(action) > 0 {
			lines = action
		} else {
			lines = append(lines[:1], append([]string{model.tabPlain()}, lines[1:]...)...)
		}
		lines = lines[:min(len(lines), height-1)]
		lines = append(lines, model.footerPlain(width))
	} else {
		lines = lines[:1]
	}
	for index := range lines {
		lines[index] = truncateCell(lines[index], width)
	}
	return strings.Join(lines, "\n")
}

func wrapPlain(value string, width int) []string {
	return strings.Split(runewidth.Wrap(value, max(1, width)), "\n")
}

func wrapInfo(value string, width int, color lipgloss.TerminalColor) []string {
	lines := wrapPlain(value, width)
	for index := range lines {
		lines[index] = lipgloss.NewStyle().Foreground(color).Render(lines[index])
	}
	return lines
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

func (model Model) progress() (done, total int) {
	for _, result := range model.results {
		if model.includes(result) {
			total++
			if !result.Loading {
				done++
			}
		}
	}
	return done, total
}

func (model Model) groupEntry(group discover.Group) rowEntry {
	entry := rowEntry{group: displayGroupPath(group.Path, model.home), groupSize: len(group.RepoIndexes), repoIndex: -1}
	if model.worktrees {
		entry.groupLabel = "worktrees"
	}
	for _, index := range group.RepoIndexes {
		if !model.results[index].Loading {
			entry.groupDone++
		}
	}
	return entry
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
	case "f":
		model.expanded = !model.expanded
		model.browseEmpty = false
		model.offset = 0
		model.refresh()
		if model.expanded {
			model.offset = max(0, firstRowOf(model.rows, model.selected))
			model.refresh()
		}
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
		model.selectRepo(len(model.displayResults()) - 1)
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
	if len(model.displayResults()) == 0 {
		return
	}
	model.selected = clamp(index, 0, len(model.displayResults())-1)
	model.browseEmpty = false
	model.refresh()
}

// scrollViewport pans the table and re-points selection at the first
// visible repo, so selection always matches what is on screen.
func (model *Model) scrollViewport(delta int) {
	if len(model.rows) == 0 || delta == 0 {
		return
	}
	maxOffset := maxViewportOffset(model.rows, model.capacity)
	model.offset = clamp(model.offset+delta, 0, maxOffset)
	visibleRepo := firstRepoAt(model.visibleRows(), 0, -1)
	model.browseEmpty = visibleRepo < 0
	if visibleRepo >= 0 {
		model.selected = visibleRepo
	}
	model.refresh()
}

// refresh rebuilds cached rows and keeps the selected repo visible.
// Selected repo sticks across rebuilds when earlier repos expand.
func (model *Model) refresh() {
	if model.width == 0 || model.height == 0 {
		return
	}
	width := max(model.width, minWidth)
	model.updateTableWithSize(width, model.viewportHeight(model.height))
}

func (model *Model) updateTableWithSize(width int, height int) {
	model.tableOuter, model.infoOuter, model.infoShown = layoutWidths(width)
	tableWidth := max(1, model.tableOuter-2-padX*2)
	model.tableWidth = tableWidth
	model.colWidths = model.columns(tableWidth)
	model.rows = model.tableRows()
	model.capacity = max(1, height)
	if len(model.displayResults()) > 0 {
		model.selected = clamp(model.selected, 0, len(model.displayResults())-1)
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
	if model.browseEmpty {
		model.offset = clamp(model.offset, 0, maxViewportOffset(model.rows, model.capacity))
		return
	}
	first := firstRowOf(model.rows, model.selected)
	if first < 0 {
		model.offset = clamp(model.offset, 0, maxViewportOffset(model.rows, model.capacity))
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
	visibleCapacity := model.capacity - stickyHeaderSize(model.rows, model.offset, model.capacity)
	if last >= model.offset && first < model.offset+visibleCapacity {
		model.offset = clamp(model.offset, 0, maxViewportOffset(model.rows, model.capacity))
		return
	}
	if first < model.offset {
		model.offset = first
	}
	if first >= model.offset+visibleCapacity {
		model.offset = first - model.capacity + 1
		model.offset += stickyHeaderSize(model.rows, model.offset, model.capacity)
	}
	model.offset = clamp(model.offset, 0, maxViewportOffset(model.rows, model.capacity))
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

func (model Model) tableRows() []rowEntry {
	results := append([]ui.RepoResult(nil), model.displayResults()...)
	for index := range results {
		if results[index].Loading {
			results[index].LoadingText = model.spin.View()
		}
	}
	entries := buildEntries(results, model.selected)
	if model.worktrees {
		for index := range entries {
			entry := &entries[index]
			if entry.number == "" {
				continue
			}
			entry.worktree = displayGroupPath(filepath.Base(results[entry.repoIndex].Repo.Path), "")
			if results[entry.repoIndex].Repo.Worktree.Locked {
				entry.status = "locked · " + entry.status
				entry.compact = "locked · " + entry.compact
				entry.tone = "yellow"
			}
		}
	}
	if !model.expanded {
		compact := make([]rowEntry, 0, len(results))
		for _, entry := range entries {
			if entry.number != "" || entry.repoIndex < 0 && !entry.divider {
				compact = append(compact, entry)
			}
		}
		entries = compact
	}
	groups := model.displayGroups()
	if len(groups) == 0 {
		if len(results) == 0 {
			return []rowEntry{{status: model.emptyMessage(), repoIndex: -1}}
		}
		return entries
	}
	grouped := []rowEntry{}
	position := 0
	for _, group := range groups {
		grouped = append(grouped, model.groupEntry(group))
		if len(group.RepoIndexes) == 0 {
			grouped = append(grouped, rowEntry{status: model.emptyMessage(), repoIndex: -1})
		}
		end := position + len(group.RepoIndexes)
		for index, entry := range entries {
			if entry.repoIndex >= position && entry.repoIndex < end {
				if model.expanded && entry.number != "" && entry.repoIndex > position && index > 0 && entries[index-1].divider {
					grouped = append(grouped, entries[index-1])
				}
				grouped = append(grouped, entry)
			}
		}
		position = end
	}
	return grouped
}

func (model Model) displayResults() []ui.RepoResult {
	if len(model.groups) == 0 {
		return model.visibleResults()
	}
	results := []ui.RepoResult{}
	for _, group := range model.displayGroups() {
		for _, index := range group.RepoIndexes {
			results = append(results, model.results[index])
		}
	}
	return results
}

func (model Model) selectedResult() ui.RepoResult {
	results := model.displayResults()
	return results[clamp(model.selected, 0, len(results)-1)]
}

func (model Model) selectedGroup() string {
	position := 0
	for _, group := range model.displayGroups() {
		position += len(group.RepoIndexes)
		if model.selected < position {
			return group.Path
		}
	}
	return ""
}

func displayGroupPath(path string, home string) string {
	if home != "" {
		if path == home {
			path = "~"
		} else if strings.HasPrefix(path, home+string(filepath.Separator)) {
			path = "~" + strings.TrimPrefix(path, home)
		}
	}
	return strings.NewReplacer("\r", "\\r", "\n", "\\n", "\t", "\\t").Replace(path)
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
		compact := ui.CompactSummary(result)
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
				compact:   compact,
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

func lineWidthFor(cols tableColumns) int {
	width := cols.number + cols.repo + cols.status + columnGap*2
	if cols.worktree > 0 {
		width += cols.worktree + columnGap
	}
	return width
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

// renderHeader splits the bar: repo summary left, mode/progress right.
// Plain text is measured first, styled last, so ANSI never affects layout.
func (model Model) renderHeader(width int) string {
	content := max(1, width-spaceSM*2)
	left := "gitsy"
	right := model.headerRight()
	if len(model.groups) > 0 {
		directoryLabel := "directories"
		if len(model.groups) == 1 {
			directoryLabel = "directory"
		}
		left += fmt.Sprintf(" · %d %s", len(model.groups), directoryLabel)
		if runewidth.StringWidth(left)+spaceSM+runewidth.StringWidth(right) > content {
			left = fmt.Sprintf("gitsy · %d dirs", len(model.groups))
		}
	}
	if runewidth.StringWidth(left)+spaceSM+runewidth.StringWidth(right) > content {
		done, total := model.progress()
		right = fmt.Sprintf("%s · %d/%d", model.mode(), done, total)
	}
	if runewidth.StringWidth(left)+spaceSM+runewidth.StringWidth(right) > content {
		if runewidth.StringWidth(right)+1 <= content {
			left = truncateCell(left, content-runewidth.StringWidth(right)-spaceSM)
		} else {
			left = truncateCell(left, content)
			return headerBarStyle(width).Render(headerTitleStyle().Render(left))
		}
	}
	gap := strings.Repeat(" ", content-runewidth.StringWidth(left)-runewidth.StringWidth(right))
	title := headerTitleStyle().Foreground(brand).Render(left) + gap + headerMetaStyle().Render(right)
	summary := strings.TrimPrefix(model.title(), "gitsy • ")
	parts := strings.Split(summary, " • ")
	// Failures retain priority even when a very narrow summary must omit fields.
	for index, part := range parts {
		if strings.Contains(part, "failed") && index > 1 {
			copy(parts[2:index+1], parts[1:index])
			parts[1] = part
			break
		}
	}
	lines := []string{""}
	used := 0
	for _, part := range parts {
		needed := runewidth.StringWidth(part)
		if used > 0 && used+3+needed > content {
			if len(lines) == 2 {
				break
			}
			lines = append(lines, "")
			used = 0
		}
		color := textLo
		if strings.Contains(part, "changed") || strings.Contains(part, "behind") || strings.Contains(part, "stale") {
			color = warning
		}
		if strings.Contains(part, "failed") {
			color = danger
		}
		if used > 0 {
			lines[len(lines)-1] += headerMetaStyle().Render(" · ")
			used += 3
		}
		lines[len(lines)-1] += lipgloss.NewStyle().Foreground(color).Render(truncateCell(part, content-used))
		used += needed
	}
	if len(lines) == 1 {
		lines = append(lines, "")
	}
	return headerBarStyle(width).Render(title + "\n" + strings.Join(lines, "\n") + "\n" + model.renderTabs(content))
}

func (model Model) title() string {
	results := model.visibleResults()
	title := ui.Title(results, len(results))
	if len(model.displayResults()) > len(results) {
		title = strings.Replace(title, fmt.Sprintf("%d repos", len(results)), fmt.Sprintf("%d unique repos", len(results)), 1)
	}
	if model.worktrees {
		title = strings.Replace(title, "repos", "worktrees", 1)
	}
	return title
}

// headerRight is the header meta without the leading separator: the gap
// between title and meta already separates them.
func (model Model) headerRight() string {
	done, total := model.progress()
	state := "in progress"
	if done == total {
		state = "done"
	}
	return fmt.Sprintf("%s · %d/%d %s", model.mode(), done, total, state)
}

func (model Model) mode() string {
	mode := "fetch"
	if model.noFetch {
		mode = "local"
	}
	if model.sync {
		mode = "sync"
	}
	return mode
}

// The context rail shares the ledger's height.
func (model Model) renderInfo(infoOuter int, boxHeight int) string {
	content := max(1, infoOuter-1-padX*2)
	lines := model.infoLines(content)
	want := max(1, boxHeight)
	for len(lines) < want {
		lines = append(lines, "")
	}
	lines = lines[:min(len(lines), want)]
	return infoStyle(infoOuter).Render(strings.Join(lines, "\n"))
}

func (model Model) infoLines(width int) []string {
	label := "Selected repository"
	if model.worktrees {
		label = "Selected worktree"
	}
	lines := []string{
		columnHeaderStyle().Render(truncateCell(label, width)),
		dividerStyle().Render(strings.Repeat("─", max(1, width))),
	}
	if model.browseEmpty {
		return append(lines, "", truncateCell(model.emptyMessage(), width))
	}
	if len(model.displayResults()) == 0 {
		lines = append(lines, "")
		return append(lines, wrapPlain(model.emptyMessage(), width)...)
	}
	result := model.selectedResult()
	name := result.Repo.DisplayName
	if model.worktrees {
		name = displayGroupPath(filepath.Base(result.Repo.Path), "")
	}
	lines = append(lines, "", headerTitleStyle().Render(truncateCell(name, width)))
	lines = append(lines, wrapInfo(result.Repo.Path, width, textLo)...)
	if worktree := result.Repo.Worktree; model.worktrees && worktree != nil {
		lines = append(lines, "", infoSectionStyle().Render("Repository"))
		lines = append(lines, wrapInfo(result.Repo.DisplayName, width, textHi)...)
		lines = append(lines, wrapInfo(displayGroupPath(worktree.MainPath, model.home), width, textLo)...)
		if worktree.Locked {
			lines = append(lines, wrapInfo("Locked: "+worktree.LockReason, width, warning)...)
		}
	}
	if branch := result.Status.Branch; branch != nil {
		lines = append(lines, "", infoSectionStyle().Render("Branch"))
		name := branch.Name
		if name == "" {
			name = "detached"
		}
		lines = append(lines, wrapInfo(name, width, textHi)...)
		if branch.Upstream != "" {
			lines = append(lines, wrapInfo(branch.Upstream, width, textLo)...)
		}
	}
	lines = append(lines, "", infoSectionStyle().Render("Status"))
	rows := ui.RowsForRepo(result)
	if len(rows) > 0 {
		for _, line := range wrapPlain(rows[0].Text, width) {
			lines = append(lines, toneStyle(rows[0].Tone, false, false).Render(line))
		}
	}
	if result.Sync != nil && result.Sync.Kind == "skipped" {
		lines = append(lines, wrapInfo("Sync skipped: "+result.Sync.Reason, width, warning)...)
	}
	return lines
}

// Context collapses when it would crowd repository state.
func layoutWidths(termWidth int) (tableOuter int, infoOuter int, infoShown bool) {
	infoOuter = clamp(termWidth/4, infoMinOuter, infoMaxOuter)
	if termWidth-infoOuter-panelGap < tableMinOuter {
		return termWidth, 0, false
	}
	return termWidth - infoOuter - panelGap, infoOuter, true
}

// renderTableBody draws a real table: column header, full-width rule,
// then the visible window of rows. Plain text is measured first, styles
// applied last, so ANSI never affects layout.
func renderTableBody(entries []rowEntry, cols tableColumns, offset int, capacity int, selected int) string {
	numberWidth, repoWidth, statusWidth := cols.number, cols.repo, cols.status
	gap := strings.Repeat(" ", columnGap)
	header := padCell(truncateCell("#", numberWidth), numberWidth) + gap +
		padCell(truncateCell("Repository", repoWidth), repoWidth) + gap
	if cols.worktree > 0 {
		header += padCell(truncateCell("Worktree", cols.worktree), cols.worktree) + gap
	}
	header += padCell(truncateCell("Branch / status", statusWidth), statusWidth)
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
	for len(lines) < capacity+2 {
		lines = append(lines, strings.Repeat(" ", lineWidthFor(cols)))
	}
	return strings.Join(lines, "\n")
}

func visibleSlice(entries []rowEntry, offset int, capacity int) []rowEntry {
	if len(entries) == 0 || capacity <= 0 {
		return nil
	}
	start := clamp(offset, 0, max(0, len(entries)-1))
	if stickyHeaderSize(entries, start, capacity) > 0 {
		for index := start - 1; index >= 0; index-- {
			if entries[index].group != "" {
				visible := []rowEntry{entries[index]}
				return append(visible, entries[start:min(start+capacity-1, len(entries))]...)
			}
		}
	}
	end := min(start+max(1, capacity), len(entries))
	return entries[start:end]
}

func stickyHeaderSize(entries []rowEntry, offset int, capacity int) int {
	if capacity <= 1 || offset <= 0 || offset >= len(entries) || entries[offset].group != "" {
		return 0
	}
	for index := offset - 1; index >= 0; index-- {
		if entries[index].group != "" {
			return 1
		}
	}
	return 0
}

func maxViewportOffset(entries []rowEntry, capacity int) int {
	offset := max(0, len(entries)-capacity)
	return offset + stickyHeaderSize(entries, offset, capacity)
}

func (model Model) visibleRows() []rowEntry {
	return visibleSlice(model.rows, model.offset, model.capacity)
}

// Keep the selection marker visible even without terminal color support.
func renderRow(entry rowEntry, cols tableColumns, selected int) string {
	if entry.group != "" {
		return renderGroup(entry, lineWidthFor(cols), true)
	}
	if entry.divider {
		return strings.Repeat(" ", max(1, lineWidthFor(cols)))
	}
	if entry.repoIndex < 0 {
		return headerMetaStyle().Render(padCell(truncateCell(entry.status, lineWidthFor(cols)), lineWidthFor(cols)))
	}
	numberWidth, repoWidth, statusWidth := cols.number, cols.repo, cols.status
	gap := strings.Repeat(" ", columnGap)
	marker := " "
	if entry.marker {
		marker = iconSelected
	}
	number := padCell(truncateCell(marker+" "+entry.number, numberWidth), numberWidth)
	repo := padCell(truncateCell(entry.repo, repoWidth), repoWidth)
	if cols.worktree > 0 {
		repo += gap + padCell(truncateCell(entry.worktree, cols.worktree), cols.worktree)
	}
	statusText := entry.status
	if entry.number != "" && entry.compact != "" && runewidth.StringWidth(statusText) > statusWidth {
		statusText = entry.compact
	}
	status := padCell(truncateCell(statusText, statusWidth), statusWidth)
	if entry.repoIndex == selected && entry.repoIndex >= 0 {
		mark := selectedNumStyle().Background(selection).Render(" ")
		if entry.marker {
			mark = selectedMarkerStyle().Background(selection).Render(iconSelected)
		}
		return mark +
			selectedNumStyle().Background(selection).Render(padCell(truncateCell(" "+entry.number, numberWidth-1), numberWidth-1)+gap+repo+gap) +
			toneStyle(entry.tone, entry.bold, false).Background(selection).Render(status)
	}
	return headerMetaStyle().Render(number) + gap + lipgloss.NewStyle().Foreground(textHi).Render(repo) + gap + toneStyle(entry.tone, false, entry.dim).Render(status)
}

func renderGroup(entry rowEntry, width int, styled bool) string {
	label := "repos"
	if entry.groupSize == 1 {
		label = "repo"
	}
	if entry.groupLabel != "" {
		label = entry.groupLabel
	}
	count := fmt.Sprintf("%d/%d", entry.groupDone, entry.groupSize)
	if len(count)+1+len(label)+columnGap+1 <= width {
		count += " " + label
	}
	pathWidth := width - len(count) - columnGap
	if pathWidth < 1 {
		return truncateCell(count, width)
	}
	path := entry.group
	if fullWidth := runewidth.StringWidth(path); fullWidth > pathWidth {
		path = runewidth.TruncateLeft(path, fullWidth-pathWidth+1, "…")
	}
	path = padCell(path, pathWidth) + strings.Repeat(" ", columnGap)
	if styled {
		return infoSectionStyle().Render(path) + headerMetaStyle().Render(count)
	}
	return path + count
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

// renderFooter is always shown: position left, key hints right.
// Both parts collapse gracefully at narrow widths.
func (model Model) renderFooter(width int) string {
	return footerStyle(width).Render(model.footerPlain(max(1, width-spaceSM*2)))
}

func (model Model) footerPlain(content int) string {
	if model.confirm != nil {
		if !model.canConfirm() {
			return truncateCell("esc cancel", content)
		}
		if model.force {
			return truncateCell("y force · esc cancel", content)
		}
		return truncateCell("y delete · esc cancel", content)
	}
	if model.deleting != "" {
		return truncateCell("Deleting… · q quit", content)
	}
	position := ""
	if len(model.displayResults()) > 0 && !model.browseEmpty {
		position = strconv.Itoa(model.selected+1) + "/" + strconv.Itoa(len(model.displayResults()))
	}
	hints := []string{
		"↑/↓ j/k move · tab views · f files · PgUp/PgDn · q quit",
		"↑/↓ j/k · tab views · f files · q quit",
		"↑↓ · tab views · f files · q",
		"tab views · q",
		"q quit",
	}
	if model.worktrees {
		hints = []string{"↑/↓ j/k move · tab views · f files · x delete · q quit", "↑↓ · tab views · x delete · q quit", "tab views · x delete · q", "tab · x · q", "q"}
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
