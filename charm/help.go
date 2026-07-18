package charm

import (
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
// The footer advertises keys and so does this, which is two places describing
// one thing and therefore two places that can disagree.
// TestHelpCoversEveryFooterKey is the seam: every key any footer offers has to
// appear here. It cannot check that the *words* are still true — nothing can —
// but a key can never be silently undocumented.

type helpEntry struct {
	keys string
	what string
}

type helpGroup struct {
	title   string
	entries []helpEntry
}

// helpGroups is the keymap, grouped the way it is read: what moves, what changes
// the disk, what changes the view, and where else you can be.
var helpGroups = []helpGroup{
	{"Navigate", []helpEntry{
		{"↑ ↓  k j", "move"},
		{"→ ↵  l", "enter directory"},
		{"← h", "go to parent — at the root, scan the one above it"},
		{"g G", "jump to top / bottom"},
		{"pgup pgdn", "page"},
		{"/", "filter this directory (fuzzy, live)"},
		{"f", "find files by name, any depth (*.mkv)"},
	}},
	{"Change the disk", []helpEntry{
		{"d", "trash it — does not free space"},
		{"D", "delete for good — frees space, no undo"},
		{"e", "empty a file"},
		{"u", "undo the last trash"},
		{"r", "rescan"},
		{"space", "mark for a batch delete"},
		{"M", "the delete queue — review, then delete the marked set"},
	}},
	{"Change the view", []helpEntry{
		// The second keys are named but not explained: both menus spell their own
		// fields out in the footer the moment you enter them, which is the rule that
		// stops a mode being invisible. Repeating the words here only made the line
		// too long to fit. Naming the keys is not optional though — they are
		// bindings, and this screen is every binding.
		{"s", "sort, then s n c m — or d for folders first"},
		{"t", "columns, then a B c m — or s to save"},
		{"a", "apparent size ⇄ disk usage"},
		{"B", "bars: largest item ⇄ this directory"},
		{"c m", "item count, mtime"},
	}},
	{"Elsewhere", []helpEntry{
		{"T", "the largest files, any depth"},
		{"F", "find duplicate files (reads them)"},
		{"v", "view a file"},
		{"o", "open a file in its default app"},
		{"?", "this screen"},
		{"esc", "back — cancel a scan, or clear the marks"},
		{"q", "quit"},
	}},
}

const (
	// helpKeyWidth is the key column; "pgup pgdn" is the widest thing in it.
	helpKeyWidth = 10
	// minWidthForHelpColumns is where the groups stop stacking and sit side by
	// side, as the mock draws them. Below it the screen scrolls, which is the
	// honest answer for a terminal that cannot hold a page of help.
	minWidthForHelpColumns = 96
	helpColumnGap          = 4
	helpIndent             = 2
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

func (m *model) helpTitle(title string, width int) helpLine {
	t := cell(title, width)
	return helpLine{plain: t, styled: m.st.dirName.Render(t)}
}

// helpRow is one binding, at exactly width columns.
func (m *model) helpRow(e helpEntry, width int) helpLine {
	if width <= helpKeyWidth+1 {
		// Too narrow for both. The key is the part you cannot guess.
		k := cell(e.keys, width)
		return helpLine{plain: k, styled: m.st.accent.Render(k)}
	}
	keys := cell(e.keys, helpKeyWidth)
	what := cell(e.what, width-helpKeyWidth-1)
	return helpLine{
		plain:  keys + " " + what,
		styled: m.st.accent.Render(keys) + " " + m.st.dim.Render(what),
	}
}

func (m *model) openHelp() (tea.Model, tea.Cmd) {
	m.helpFrom = m.scr
	m.helpOffset = 0
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
		m.helpOffset--
	case keyDown, "j":
		m.helpOffset++
	case keyPgUp:
		m.helpOffset -= m.visibleLines()
	case keyPgDown:
		m.helpOffset += m.visibleLines()
	case keyHome, "g":
		m.helpOffset = 0
	case keyEnd, "G":
		m.helpOffset = len(m.helpLines())
	}
	m.clampHelp()
	return m, nil
}

func (m *model) clampHelp() {
	over := len(m.helpLines()) - m.visibleLines()
	m.helpOffset = min(max(m.helpOffset, 0), max(over, 0))
}

func (m *model) helpLines() []helpLine {
	if m.width >= minWidthForHelpColumns {
		return m.helpColumns()
	}
	return m.helpStacked()
}

func (m *model) helpStacked() []helpLine {
	width := max(m.width-helpIndent, 1)

	var lines []helpLine
	for i, g := range helpGroups {
		if i > 0 {
			lines = append(lines, m.helpBlank())
		}
		lines = append(lines, m.indent(m.helpTitle(g.title, width)))
		for _, e := range g.entries {
			lines = append(lines, m.indent(m.helpRow(e, width)))
		}
	}
	return lines
}

// helpColumns puts the groups two abreast, as the mock draws them. Help you have
// to scroll is help you read half of.
func (m *model) helpColumns() []helpLine {
	colWidth := max((m.width-helpIndent-helpColumnGap)/2, 1)

	var left, right []helpLine
	for i, g := range helpGroups {
		block := []helpLine{m.helpTitle(g.title, colWidth)}
		for _, e := range g.entries {
			block = append(block, m.helpRow(e, colWidth))
		}
		block = append(block, m.helpRow(helpEntry{}, colWidth)) // a blank of the right width

		if i%2 == 0 {
			left = append(left, block...)
		} else {
			right = append(right, block...)
		}
	}

	blank := m.helpRow(helpEntry{}, colWidth)
	n := max(len(left), len(right))
	lines := make([]helpLine, 0, n)
	for i := range n {
		l, r := blank, blank
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		gap := spaces(helpColumnGap)
		lines = append(lines, m.indent(helpLine{
			plain:  l.plain + gap + r.plain,
			styled: l.styled + gap + r.styled,
		}))
	}
	return lines
}

func (m *model) indent(l helpLine) helpLine {
	pad := spaces(helpIndent)
	return helpLine{plain: pad + l.plain, styled: pad + l.styled}
}

func (m *model) viewHelpBody() string {
	lines := m.helpLines()
	height := m.visibleLines()

	end := min(m.helpOffset+height, len(lines))
	out := make([]string, 0, height)
	for i := m.helpOffset; i < end; i++ {
		out = append(out, m.helpFit(lines[i]))
	}
	return padLines(joinLines(out), height)
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
