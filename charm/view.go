package charm

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/pkg/fs"
)

// Layout constants. Everything else is derived from the live terminal size.
const (
	sizeColWidth = 10
	pctColWidth  = 5
	iconWidth    = 2

	// The row is: gutter(1) + icon + size + gap(1) + name + pct. barIndent is
	// everything left of the name, minus the icon, which is not always drawn.
	barIndent = 1 + sizeColWidth + 1

	// Below these widths the layout sheds its least essential column rather
	// than wrapping or smearing. The bar goes first: it is decoration for the
	// percentage, and it costs a whole extra line per entry.
	minWidthForTagline = 92
	minWidthForBar     = 80
	minWidthForPct     = 70
	minWidthForIcon    = 44

	// The header's disk line is the first thing to go: it is decoration, and it
	// costs a row of the list at every size.
	minWidthForDiskLine  = 60
	minHeightForDiskLine = 8

	// Below this height the header and footer are dropped so the list still has
	// somewhere to live. Smaller than this and we clamp rather than crash.
	minHeightForChrome = 5

	// diskBarWidth is fixed: the disk line reads as a gauge, and a gauge that
	// changes length with the window is hard to compare against itself.
	diskBarWidth = 24

	minNameWidth = 4
)

func (m *model) headerHeight() int {
	if m.height < minHeightForChrome {
		return 0
	}
	if m.showDiskLine() {
		return 3 // brand, disk line, rule
	}
	return 2 // brand, rule
}

// showDiskLine gates the header's second line. It is the first thing the header
// gives up: it is the least essential row on screen, and it costs a line of the
// list on every terminal.
func (m *model) showDiskLine() bool {
	return m.dev != nil &&
		m.dev.Size > 0 &&
		m.width >= minWidthForDiskLine &&
		m.height >= minHeightForDiskLine
}

func (m *model) footerHeight() int {
	if m.height < minHeightForChrome {
		return 0
	}
	return 1
}

// linesPerEntry is 2 once the gradient bar is drawn beneath each entry, and 1
// below the bar's breakpoint. Scrolling and paging count entries; the renderer
// counts lines. Conflating the two is what makes a two-line list scroll by
// halves, so the distinction is kept explicit everywhere.
func (m *model) linesPerEntry() int {
	if m.width < minWidthForBar {
		return 1
	}
	return 2
}

// visibleLines is the height the list has to render into.
func (m *model) visibleLines() int {
	n := m.height - m.headerHeight() - m.footerHeight()
	if n < 1 {
		return 1
	}
	return n
}

// visibleRows is the number of entries that fit right now. It is always at
// least one, so a degenerate terminal clamps to a minimal layout instead of
// producing a negative height — even when a single entry is taller than the
// space available, in which case it is rendered and clipped rather than refused.
func (m *model) visibleRows() int {
	return max(m.visibleLines()/m.linesPerEntry(), 1)
}

func (m *model) View() string {
	// Bubble Tea sends WindowSizeMsg on startup; until it lands we have no
	// honest size to lay out against.
	if !m.haveSize {
		return ""
	}

	switch m.scr {
	case screenError:
		return m.st.danger.Render("error: ") + m.err.Error() + "\n"
	case screenScanning:
		return m.viewScanning()
	case screenBrowse:
		return m.viewBrowse()
	case screenConfirm:
		return m.viewConfirm()
	}
	return ""
}

// viewScanning keeps the same chrome the browser has, as the mock does: the
// header and footer are already the right shape, and swapping the whole screen
// for a bare spinner would make the interface appear to restart when the scan
// finishes.
func (m *model) viewScanning() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, padLines(m.viewScanBody(), m.visibleLines()))
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return strings.Join(parts, "\n")
}

func (m *model) viewScanBody() string {
	// The mock counts the scan down as a percentage. It cannot: the analyzer does
	// not know how much tree is left until it has walked it, so a percentage here
	// would be invented. What it does know — items seen and bytes so far — is what
	// the line reports instead.
	status := fmt.Sprintf(
		"walking directories · %s items · %s",
		humanCount(m.progress.ItemCount),
		m.ui.formatSize(m.progress.TotalUsage),
	)

	// spinner(1) + gap(1) + status + cursor(1)
	const chrome = 3
	status = runewidth.Truncate(status, max(m.width-chrome, 0), "…")

	cursor := " "
	if m.blinkOn {
		cursor = m.st.accent.Render("▊")
	}
	line := m.spinner.View() + " " + m.st.dim.Render(status) + cursor

	name := m.progress.CurrentItemName
	if name == "" || m.width < minNameWidth {
		return line
	}
	return line + "\n" + m.st.dim.Render(middleTruncate(name, m.width))
}

// viewBrowse renders exactly m.height lines, with no trailing newline. One line
// too many and the terminal scrolls on every frame.
func (m *model) viewBrowse() string {
	parts := make([]string, 0, 3)

	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return strings.Join(parts, "\n")
}

func (m *model) viewHeader() string {
	lines := []string{m.viewBrand()}
	if m.showDiskLine() {
		lines = append(lines, m.viewDiskLine())
	}
	lines = append(lines, m.st.dim.Render(strings.Repeat("─", max(m.width, 1))))
	return strings.Join(lines, "\n")
}

// viewBrand is the wordmark on the left and the current path on the right. The
// path is middle-truncated rather than cut: a breadcrumb whose root has been
// chopped off tells you nothing about where you are.
func (m *model) viewBrand() string {
	const wordmark = "cdu ✦"
	const tagline = "charm disk usage"

	left := wordmark
	if m.width >= minWidthForTagline {
		left += "  " + tagline
	}

	path := m.headerPath()

	const gap = 2
	avail := m.width - runewidth.StringWidth(left) - gap
	if avail < minNameWidth || path == "" {
		return m.st.accent.Render(runewidth.Truncate(left, max(m.width, 1), ""))
	}

	path = middleTruncate(path, avail)
	pad := m.width - runewidth.StringWidth(left) - runewidth.StringWidth(path)

	// The wordmark and the tagline are one plain string until here, so that the
	// padding is measured against columns rather than escape bytes; only now do
	// they get their own styles.
	brand := m.st.accent.Render(wordmark)
	if m.width >= minWidthForTagline {
		brand += "  " + m.st.dim.Render(tagline)
	}
	return brand + strings.Repeat(" ", max(pad, 1)) + m.st.size.Render(path)
}

// headerPath is what the top right corner says we are looking at: the directory
// while browsing, and the root under way while scanning — the mock puts the same
// breadcrumb in both states, which is what makes the scan read as the same
// screen filling up rather than a different one.
func (m *model) headerPath() string {
	if m.scr == screenScanning {
		return "scanning " + m.ui.scanPath
	}
	if m.currentDir != nil {
		return m.currentDir.GetPath()
	}
	return ""
}

// viewDiskLine is the volume gauge from the mock: how full the disk is that the
// scan root lives on. It answers the question the scan cannot — "how much of
// this machine is even at stake" — so the bar is drawn against the disk's own
// capacity, not against anything in the tree.
func (m *model) viewDiskLine() string {
	used, size := m.dev.GetUsage(), m.dev.Size
	usage := fmt.Sprintf("%s / %s", m.ui.formatSize(used), m.ui.formatSize(size))

	const gaps = 2
	labelWidth := m.width - diskBarWidth - runewidth.StringWidth(usage) - gaps
	label := runewidth.Truncate(m.dev.Name, max(labelWidth, 0), "…")
	label = runewidth.FillRight(label, max(labelWidth, 0))

	bar := m.bar.render(fraction(used, size), diskBarWidth)
	return m.st.dim.Render(label) + " " + bar + " " + m.st.pct.Render(usage)
}

func (m *model) viewList() string {
	lines := m.visibleLines()

	if len(m.rows) == 0 {
		return padLines(m.st.dim.Render("  (empty)"), lines)
	}

	// The window, and only the window, is rendered. A directory can hold tens of
	// thousands of entries; building a string for all of them every frame is the
	// cost this whole design exists to avoid.
	end := min(m.offset+m.visibleRows(), len(m.rows))

	// The percentage is the entry's share of the parent directory total.
	total := int64(0)
	if m.currentDir != nil {
		total = m.itemSize(m.currentDir)
	}

	var b strings.Builder
	for i := m.offset; i < end; i++ {
		b.WriteString(m.viewEntry(m.rows[i], i == m.cursor, total))
		b.WriteByte('\n')
	}
	// The last entry can overrun a list height that is not a whole number of
	// entries; padLines trims as well as pads, so the frame stays the right size.
	return padLines(strings.TrimRight(b.String(), "\n"), lines)
}

// viewEntry is one list entry: the data row, and beneath it the usage bar when
// there is width for it.
func (m *model) viewEntry(item fs.Item, selected bool, total int64) string {
	row := m.viewRow(item, selected, total)
	if m.linesPerEntry() == 1 {
		return row
	}
	return row + "\n" + m.viewBar(item, selected, total)
}

// viewBar draws the gradient bar under an entry, aligned with the name column —
// the mock spans it across the name and percentage cells rather than the whole
// row, so it reads as belonging to the name rather than to the icon.
func (m *model) viewBar(item fs.Item, selected bool, total int64) string {
	indent := barIndent
	if m.width >= minWidthForIcon {
		indent += iconWidth
	}

	width := max(m.width-indent, 0)
	bar := m.bar.render(fraction(m.itemSize(item), total), width)

	// The gutter marker is repeated on the bar line so the selection reads as one
	// block two lines tall rather than two unrelated things.
	gutter := " "
	if selected {
		gutter = m.st.accent.Render("▌")
	}
	return gutter + strings.Repeat(" ", indent-1) + bar
}

// fraction is an entry's share of its parent. An empty parent yields 0/0, so the
// zero total is answered here rather than left to produce a NaN downstream.
func fraction(size, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(size) / float64(total)
}

// viewRow builds the row as plain text at an exact width first, and only then
// applies styles. Never truncate or measure an already-styled string: escape
// sequences are invisible on screen but very much visible to a rune counter, and
// a styled row cut with runewidth loses most of its columns.
func (m *model) viewRow(item fs.Item, selected bool, total int64) string {
	size := m.itemSize(item)

	removing := item == m.pending

	icon := ""
	if m.width >= minWidthForIcon {
		switch {
		case removing:
			// The removal is happening off the render loop and can take seconds. The
			// row spins so that the wait is visible rather than looking like a key
			// that never registered.
			icon = m.tickFrame() + " "
		case m.ui.noUnicode:
			icon = "  "
			if item.IsDir() {
				icon = "> "
			}
		case item.IsDir():
			icon = "▸ "
		default:
			icon = "· "
		}
	}

	sizeText := padLeft(m.ui.formatSize(size), sizeColWidth)

	pctText := ""
	if m.width >= minWidthForPct {
		pctText = padLeft(runewidth.Truncate(formatPct(size, total), pctColWidth, ""), pctColWidth)
	}

	// The row is: gutter(1) + icon + size + gap(1) + name + pct. The gutter holds
	// either the selection marker or a blank, so both variants are the same width.
	const fixedCells = 2 // gutter + the gap between size and name
	nameWidth := max(
		m.width-runewidth.StringWidth(icon)-sizeColWidth-runewidth.StringWidth(pctText)-fixedCells,
		minNameWidth,
	)

	name := item.GetName()
	if item.IsDir() {
		name += "/"
	}
	// Flags carry meaning that must survive mono, NO_COLOR and colourblindness,
	// so they are a glyph, not a colour.
	switch item.GetFlag() {
	case '!':
		name += " !"
	case 'H':
		name += " ⇉"
	}
	if removing {
		// The word, not just the spinner: the state has to survive --no-color and a
		// terminal too narrow for the icon column.
		name = "removing " + name
	}
	nameText := runewidth.FillRight(runewidth.Truncate(name, nameWidth, "…"), nameWidth)

	plain := icon + sizeText + " " + nameText + pctText

	if selected {
		// No box-shadow in a terminal: the mock's glow becomes a filled
		// background plus a bold name and a bright marker. The marker is what
		// survives --no-color, NO_COLOR and the mono theme.
		return m.st.accent.Render("▌") +
			m.st.selected.MaxWidth(max(m.width-1, 1)).Render(plain)
	}

	nameStyle := m.st.fileName
	iconStyle := m.st.dim
	switch {
	case removing:
		nameStyle, iconStyle = m.st.dim, m.st.accent
	case item.IsDir():
		nameStyle, iconStyle = m.st.dirName, m.st.accent
	}
	return " " + iconStyle.Render(icon) +
		m.st.size.Render(sizeText) + " " +
		nameStyle.Render(nameText) +
		m.st.pct.Render(pctText)
}

// tickFrame is the spinner frame for a row being removed. It runs off the same
// 100 ms tick as the scan progress, so there is one clock in the interface rather
// than two drifting against each other.
func (m *model) tickFrame() string {
	if len(m.frames) == 0 {
		return " "
	}
	return m.frames[m.ticks%len(m.frames)]
}

type keyHint struct{ key, label string }

// The footer lists only keys that actually do something on the screen you are
// on. An interface that advertises a binding it does not have is worse than one
// that says nothing.
var (
	browseKeys = []keyHint{
		{"↑↓", "move"},
		{"→", "open"},
		{"←", "back"},
		{"d", "trash"},
		{"D", "delete"},
		{"q", "quit"},
	}
	scanKeys = []keyHint{
		{"q", "quit"},
	}
	confirmKeys = []keyHint{
		{"←→", "choose"},
		{"enter", "confirm"},
		{"esc", "cancel"},
	}
)

func (m *model) viewFooter() string {
	keys := browseKeys
	switch m.scr {
	case screenScanning:
		keys = scanKeys
	case screenConfirm:
		keys = confirmKeys
	case screenBrowse, screenError:
	}

	var plain, styled strings.Builder
	for i, k := range keys {
		if i > 0 {
			plain.WriteString("  ")
			styled.WriteString("  ")
		}
		plain.WriteString(k.key)
		plain.WriteByte(' ')
		plain.WriteString(k.label)

		styled.WriteString(m.st.accent.Render(k.key))
		styled.WriteByte(' ')
		styled.WriteString(m.st.dim.Render(k.label))
	}

	// The right-hand side is whichever matters more right now: what just happened,
	// or — when nothing has — how the list is sorted. A destructive action that
	// reported nothing would be indistinguishable from one that silently failed.
	right, rightStyle := "", m.st.dim
	switch {
	case m.status != "":
		right = m.status
		if m.statusIsError {
			rightStyle = m.st.danger
		} else {
			rightStyle = m.st.accent
		}
	case m.scr == screenBrowse:
		right = m.sortLabel()
	}

	gap := m.width - runewidth.StringWidth(plain.String()) - runewidth.StringWidth(right)
	if gap < 1 {
		// Too narrow for both. The keys are the only way out of the screen, so they
		// are what survives — except when something just happened, which is the one
		// thing the user needs to read.
		if m.status != "" {
			return rightStyle.Render(runewidth.Truncate(right, max(m.width, 1), "…"))
		}
		return m.st.dim.Render(runewidth.Truncate(plain.String(), max(m.width, 1), ""))
	}
	return styled.String() + strings.Repeat(" ", gap) + rightStyle.Render(right)
}

func (m *model) sortLabel() string {
	field := "size"
	switch m.ui.sortBy {
	case fs.SortBySize:
		field = "size"
	case fs.SortByApparentSize:
		field = "apparent size"
	case fs.SortByName:
		field = "name"
	case fs.SortByItemCount:
		field = "items"
	case fs.SortByMtime:
		field = "mtime"
	}

	order := "desc"
	if m.ui.sortOrder == fs.SortAsc {
		order = "asc"
	}
	return "sorted by " + field + " · " + order
}

// itemSize honours --apparent-size, which is a display choice: the engine
// always carries both figures on every item.
func (m *model) itemSize(item fs.Item) int64 {
	if m.ui.ShowApparentSize {
		return item.GetSize()
	}
	return item.GetUsage()
}
