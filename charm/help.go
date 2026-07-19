package charm

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// `?` is every key on one screen.
//
// It describes cdu's bindings, not the mock's. cdu-4-help.html predates most of
// them: no D, no undo, no column menu, no largest-files screen; it lists `s` as
// "sort by size" from before sorting became two keys; and it gives `d` twice, as
// "delete selected" and as "list mounted disks", which cannot both be true and
// never were — `-d` is a flag.
//
// The list is a cursor away from being an index: arrow onto a binding and the
// pane at the foot describes it in full — the gotcha the one-line `what` has no
// room for (d recovers where D does not, which bar mode means what). The keys
// that scroll a static help page are the keys that move the cursor here, so
// there is nothing for a "press the key to describe it" to collide with.
//
// The footer advertises keys and so does this, which is two places describing
// one thing and therefore two places that can disagree.
// TestHelpCoversEveryFooterKey is the seam: every key any footer offers has to
// appear here. It cannot check that the *words* are still true — nothing can —
// but a key can never be silently undocumented.

type helpEntry struct {
	keys string
	what string
	// detail is the fuller description shown in the pane when this binding is
	// selected: the part that does not fit on the one line, and the part someone
	// reading the help actually came for.
	detail string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

// helpGroups is the keymap, grouped the way it is read: what moves, what changes
// the disk, what changes the view, and where else you can be.
var helpGroups = []helpGroup{
	{"Navigate", []helpEntry{
		{"↑ ↓  k j", "move", "Move the cursor one row. The list scrolls to keep it in view."},
		{"→ ↵  l", "enter directory", "Enter the directory under the cursor. On a file it does nothing — use v to view or o to open it."},
		{
			keys:   "← h",
			what:   "go to parent — at the root, scan the one above it",
			detail: "Go up to the parent directory. At the scan root it scans the directory one level up on disk instead.",
		},
		{"g G", "jump to top / bottom", "Jump straight to the first or last row."},
		{"pgup pgdn", "page", "Scroll a whole screen at a time."},
		{"/", "filter this directory (fuzzy, live)", "Filter this directory as you type — a fuzzy, live match on the names here. esc clears it."},
		{"f", "find files by name, any depth (*.mkv)", "Search the whole scanned tree by name, at any depth. Takes a glob like *.mkv or a plain substring."},
	}},
	{"Change the disk", []helpEntry{
		{"d", "trash it — does not free space", "Move to the trash — recoverable with u this session. It does not free disk space until the trash is emptied."},
		{"D", "delete for good — frees space, no undo", "Delete permanently and free the space now. There is no undo, so cdu asks first."},
		{"e", "empty a file", "Truncate the file to zero bytes but keep it in place — handy for a runaway log."},
		{"u", "undo the last trash", "Put back the last thing you trashed with d. Only a trash is recoverable; a D delete is gone."},
		{"r", "rescan", "Read the current directory from disk again, picking up whatever changed."},
		{"space", "mark for a batch delete", "Mark this row for a batch delete. Marked rows show struck through in red; M reviews the whole set."},
		{
			keys:   "M",
			what:   "the delete queue — review, then delete the marked set",
			detail: "Open the queue of everything marked so far. Review it, then delete the whole set behind one confirm.",
		},
	}},
	{"Change the view", []helpEntry{
		// The second keys are named but not explained in the one-liner: both menus
		// spell their own fields out in the footer the moment you enter them, which is
		// the rule that stops a mode being invisible. The pane has room to name them.
		{"s", "sort, then s n c m — or d for folders first", "Open the sort menu, then a field: s size, n name, c item count, m mtime. d toggles folders-first."},
		{"t", "columns, then a B c m — or s to save", "Open the column menu, then toggle one: a apparent size, B bars, c count, m mtime. s saves the layout."},
		{"a", "apparent size ⇄ disk usage", "Switch every size between apparent size (bytes in the file) and disk usage (the blocks it occupies)."},
		{"B", "bars: largest item ⇄ this directory", "Switch the bars between sizing against the largest item and against this whole directory's total."},
		{"c m", "item count, mtime", "Show the item-count and modified-time columns. Also reachable from the sort and column menus."},
		{"p", "themes — preview and keep one", "Open the theme picker. Moving the cursor previews each theme live; enter keeps and saves it, esc restores."},
	}},
	{"Elsewhere", []helpEntry{
		{"T", "the largest files, any depth", "List the largest files anywhere under here — deepest search, biggest first."},
		{"F", "find duplicate files (reads them)", "Find byte-identical files by hashing their contents, grouped so you can reclaim the copies. It reads files."},
		{"v", "view a file", "Open the file in a built-in read-only pager; q returns."},
		{"o", "open a file in its default app", "Hand the file to the operating system's default app (open / xdg-open / start)."},
		{"?", "this screen", "This help — every binding cdu has, on one screen."},
		{"esc", "back — cancel a scan, or clear the marks", "Step back one screen. During a scan it cancels it; with rows marked it clears the marks."},
		{"q", "quit", "Leave cdu. From the help or the file viewer it closes just that, not the program."},
	}},
}

const (
	// helpKeyWidth is the key column; "pgup pgdn" is the widest thing in it.
	helpKeyWidth = 10
	// helpGutter is the two columns each row keeps at its left: two spaces normally,
	// the "▸ " cursor marker on the selected row. Titles keep it too, so keys line up
	// under their heading.
	helpGutter = 2
	// minWidthForHelpColumns is where the groups stop stacking and sit side by
	// side, as the mock draws them. Below it the screen scrolls, which is the
	// honest answer for a terminal that cannot hold a page of help.
	minWidthForHelpColumns = 96
	helpColumnGap          = 4
	// minVisibleForHelpPane is the list height below which the detail pane is
	// dropped: on a short terminal the bindings themselves are worth more than the
	// prose, so the pane yields to them.
	minVisibleForHelpPane = 8
)

// helpLine is one line, twice: as text and as pixels.
//
// Both are built rune-for-rune from the same pieces, and only the plain one is
// ever measured. It is the same pattern plainKeys/renderKeys use in the footer,
// and for the same reason: runewidth counts escape bytes as columns, so a styled
// line cut to the terminal loses most of itself. The help is the last screen
// worth getting that wrong on — it is what someone reads when the rest has
// already confused them.
type helpLine struct {
	plain  string
	styled string
}

func (m *model) helpBlank() helpLine { return helpLine{} }

// helpBlankCell is a blank of exactly width columns, for padding a short column.
func (m *model) helpBlankCell(width int) helpLine {
	s := spaces(max(width, 0))
	return helpLine{plain: s, styled: s}
}

func (m *model) helpTitle(title string, width int) helpLine {
	t := cell(spaces(helpGutter)+title, width)
	return helpLine{plain: t, styled: m.st.dirName.Render(t)}
}

// helpRow is one binding, at exactly width columns, banded when it is the one the
// cursor is on.
func (m *model) helpRow(e helpEntry, width int, selected bool) helpLine {
	marker := spaces(helpGutter)
	if selected {
		marker = "▸ "
	}
	body := width - helpGutter

	var plain, styled string
	if body <= helpKeyWidth+1 {
		// Too narrow for both. The key is the part you cannot guess.
		k := cell(e.keys, max(body, 1))
		plain = marker + k
		styled = marker + m.st.accent.Render(k)
	} else {
		keys := cell(e.keys, helpKeyWidth)
		what := cell(e.what, body-helpKeyWidth-1)
		plain = marker + keys + " " + what
		styled = marker + m.st.accent.Render(keys) + " " + m.st.dim.Render(what)
	}
	if selected {
		// One continuous band across the whole cell, marker and all.
		styled = m.st.selected.Render(cell(plain, width))
	}
	return helpLine{plain: plain, styled: styled}
}

func (m *model) openHelp() (tea.Model, tea.Cmd) {
	m.helpFrom = m.scr
	m.helpOffset = 0
	m.helpCursor = 0
	m.scr = screenHelp
	return m, nil
}

func (m *model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape, "?", "q", keyLeft, "h", keyBackspace:
		// q closes rather than quits, like the viewer: leaving the help you asked
		// for should not also leave the program.
		m.scr = m.helpFrom
		return m, nil
	case keyUp, "k":
		m.helpCursor--
	case keyDown, "j":
		m.helpCursor++
	case keyPgUp:
		m.helpCursor -= m.helpListHeight()
	case keyPgDown:
		m.helpCursor += m.helpListHeight()
	case keyHome, "g":
		m.helpCursor = 0
	case keyEnd, "G":
		m.helpCursor = m.helpEntryCount() - 1
	}
	m.clampHelp()
	return m, nil
}

// clampHelp keeps the cursor in range and slides the viewport to follow it, so the
// selected binding — the one the pane is describing — is always on screen.
func (m *model) clampHelp() {
	m.helpCursor = min(max(m.helpCursor, 0), max(m.helpEntryCount()-1, 0))

	lines, selLine := m.helpBody()
	listH := m.helpListHeight()
	if selLine < m.helpOffset {
		m.helpOffset = selLine
	}
	if selLine >= m.helpOffset+listH {
		m.helpOffset = selLine - listH + 1
	}
	over := len(lines) - listH
	m.helpOffset = min(max(m.helpOffset, 0), max(over, 0))

	// At the first binding, show the very top so its group title is not scrolled
	// off above it — the selected entry sits one line below the title.
	if m.helpCursor == 0 {
		m.helpOffset = 0
	}
}

func (m *model) helpEntryCount() int {
	n := 0
	for _, g := range helpGroups {
		n += len(g.entries)
	}
	return n
}

// helpCursorPos maps the flat cursor to (group, entry).
func (m *model) helpCursorPos() (group, entry int) {
	idx := m.helpCursor
	for gi, g := range helpGroups {
		if idx < len(g.entries) {
			return gi, idx
		}
		idx -= len(g.entries)
	}
	gi := len(helpGroups) - 1
	return gi, len(helpGroups[gi].entries) - 1
}

func (m *model) helpSelectedEntry() helpEntry {
	gi, ei := m.helpCursorPos()
	return helpGroups[gi].entries[ei]
}

// helpLines is the rendered page without regard to which line is selected — the
// count is what the layout tests ask for.
func (m *model) helpLines() []helpLine {
	lines, _ := m.helpBody()
	return lines
}

// helpBody renders the page and reports which line the selected binding is on, so
// the viewport can be slid to keep it visible.
func (m *model) helpBody() (lines []helpLine, selLine int) {
	selGi, selEi := m.helpCursorPos()
	if m.width >= minWidthForHelpColumns {
		return m.helpColumns(selGi, selEi)
	}
	return m.helpStacked(selGi, selEi)
}

func (m *model) helpStacked(selGi, selEi int) (lines []helpLine, selLine int) {
	width := max(m.width, 1)

	for gi, g := range helpGroups {
		if gi > 0 {
			lines = append(lines, m.helpBlank())
		}
		lines = append(lines, m.helpTitle(g.title, width))
		for ei, e := range g.entries {
			sel := gi == selGi && ei == selEi
			if sel {
				selLine = len(lines)
			}
			lines = append(lines, m.helpRow(e, width, sel))
		}
	}
	return lines, selLine
}

// helpColumns puts the groups two abreast, as the mock draws them. Help you have
// to scroll is help you read half of.
func (m *model) helpColumns(selGi, selEi int) (lines []helpLine, selLine int) {
	colWidth := max((m.width-helpColumnGap)/2, 1)

	type colCell struct {
		line helpLine
		sel  bool
	}
	var left, right []colCell
	for gi, g := range helpGroups {
		block := []colCell{{m.helpTitle(g.title, colWidth), false}}
		for ei, e := range g.entries {
			sel := gi == selGi && ei == selEi
			block = append(block, colCell{m.helpRow(e, colWidth, sel), sel})
		}
		block = append(block, colCell{m.helpBlankCell(colWidth), false})

		if gi%2 == 0 {
			left = append(left, block...)
		} else {
			right = append(right, block...)
		}
	}

	blank := colCell{m.helpBlankCell(colWidth), false}
	gap := spaces(helpColumnGap)
	n := max(len(left), len(right))
	lines = make([]helpLine, 0, n)
	for i := range n {
		l, r := blank, blank
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if l.sel || r.sel {
			selLine = i
		}
		lines = append(lines, helpLine{
			plain:  l.line.plain + gap + r.line.plain,
			styled: l.line.styled + gap + r.line.styled,
		})
	}
	return lines, selLine
}

// helpPaneHeight is one rule line plus the tallest detail wraps to at this width,
// so the pane does not reflow the list as the cursor moves between short and long
// entries. It is dropped on a short terminal and never eats more than half of it.
func (m *model) helpPaneHeight() int {
	if m.visibleLines() < minVisibleForHelpPane {
		return 0
	}
	textLines := 1
	for i := range m.helpEntryCount() {
		if n := len(m.helpDetailWrap(m.helpEntryAt(i))); n > textLines {
			textLines = n
		}
	}
	h := 1 + textLines
	if half := m.visibleLines() / 2; h > half {
		h = half
	}
	return h
}

func (m *model) helpListHeight() int {
	return max(m.visibleLines()-m.helpPaneHeight(), 1)
}

func (m *model) helpEntryAt(i int) helpEntry {
	idx := i
	for _, g := range helpGroups {
		if idx < len(g.entries) {
			return g.entries[idx]
		}
		idx -= len(g.entries)
	}
	g := helpGroups[len(helpGroups)-1]
	return g.entries[len(g.entries)-1]
}

func (m *model) helpDetailText(e helpEntry) string {
	d := e.detail
	if d == "" {
		d = e.what
	}
	return strings.TrimSpace(e.keys) + " — " + d
}

func (m *model) helpDetailWrap(e helpEntry) []string {
	return wrapWords(m.helpDetailText(e), m.width)
}

// helpPane is the block beneath the list: a rule, then the selected binding's full
// description. It stays exactly helpPaneHeight lines so the frame does not move.
func (m *model) helpPane() string {
	h := m.helpPaneHeight()
	if h < 1 {
		return ""
	}
	lines := []string{m.st.dim.Render(strings.Repeat("─", max(m.width, 0)))}
	for _, seg := range m.helpDetailWrap(m.helpSelectedEntry()) {
		if len(lines) >= h {
			break
		}
		lines = append(lines, m.st.fileName.Render(clipTo(seg, m.width)))
	}
	return padLines(joinLines(lines), h)
}

func (m *model) viewHelpBody() string {
	total := m.visibleLines()
	lines, _ := m.helpBody()
	listH := m.helpListHeight()

	end := min(m.helpOffset+listH, len(lines))
	out := make([]string, 0, listH)
	for i := m.helpOffset; i < end; i++ {
		out = append(out, m.helpFit(lines[i]))
	}
	body := padLines(joinLines(out), listH)

	if pane := m.helpPane(); pane != "" {
		body = joinLines([]string{body, pane})
	}
	return padLines(body, total)
}

// helpFit is the last guard: a line built for a width the terminal no longer has
// is clipped as text rather than let out to wrap.
func (m *model) helpFit(l helpLine) string {
	if lineWidth(l.plain) <= m.width {
		return l.styled
	}
	return m.st.dim.Render(clipTo(l.plain, m.width))
}

func (m *model) viewHelp() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewHelpBody())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
