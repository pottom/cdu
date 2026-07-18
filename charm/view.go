package charm

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/pkg/fs"
)

// Layout constants. Everything else is derived from the live terminal size.
const (
	sizeColWidth  = 10
	pctColWidth   = 5
	iconWidth     = 2
	countColWidth = 9

	// Minute precision: the seconds are never what anyone is looking at in a disk
	// usage tool, and they cost three columns the name would rather have.
	mtimeLayout = "2006-01-02 15:04"

	// The optional columns only appear where they leave the name worth reading.
	// Asking for a column the terminal cannot fit and getting a four-character
	// name back is not what the keypress meant.
	minWidthForItemCount = 72
	minWidthForMtime     = 92

	// The row is: gutter(1) + icon + size + gap(1) + name + pct. barIndent is
	// everything left of the name, minus the icon, which is not always drawn.
	barIndent = 1 + sizeColWidth + 1

	// Below these widths the layout sheds its least essential column rather
	// than wrapping or smearing. The bar goes first: it is decoration for the
	// percentage, and it costs a whole extra line per entry.
	minWidthForTagline = 92
	// The mark tally replaces the tagline while anything is marked, and earns a
	// lower bar than the tagline it stands in for: it is live state, not decoration,
	// so it should survive to a narrower terminal than a subtitle would.
	minWidthForTally = 60
	minWidthForBar   = 80
	minWidthForPct   = 70
	minWidthForIcon  = 44

	// The header's disk line is the first thing to go: it is decoration, and it
	// costs a row of the list at every size.
	minWidthForDiskLine  = 60
	minHeightForDiskLine = 8

	// Below this height the header and footer are dropped so the list still has
	// somewhere to live. Smaller than this and we clamp rather than crash.
	minHeightForChrome = 5

	// minDiskBar is the shortest the header gauge is allowed to be. Above it the
	// bar takes whatever the line has left after the name and the figures, so on a
	// wide terminal it fills the header rather than sitting small in a corner.
	minDiskBar = 16

	minNameWidth = 4

	// minRightWidth is the room the footer keeps for the sort state, or for
	// whatever just happened. The key hints get everything else.
	minRightWidth = 22
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
// showDiskLine reports whether the header carries the volume gauge.
//
// The gauge describes the scan — "the disk this tree lives on, and how full it
// is". The device list is not a scan, so on that screen there is nothing for it
// to describe, and the last device analyzed is simply stale: it would sit above
// a table that already has every device's usage in it, claiming to be about one
// of them.
//
// It is a rule about what the line means rather than a matter of clearing m.dev
// on the way out, because a rule cannot be forgotten by whatever else learns to
// reach this screen.
func (m *model) showDiskLine() bool {
	return m.scr != screenDisks &&
		m.dev != nil &&
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
	// A terminal of no width has nowhere to put a column, and every component
	// below has a floor of at least one — they would each draw a single column
	// into a screen that has none, and the terminal would wrap all of them. Give
	// back the right number of empty lines instead and let the next resize sort
	// it out.
	if m.width < 1 {
		return padLines("", m.height)
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
	case screenViewer:
		return m.viewViewer()
	case screenDisks:
		return m.viewDisks()
	case screenTop:
		return m.viewTop()
	case screenQueue:
		return m.viewQueue()
	case screenHelp:
		return m.viewHelp()
	case screenHashing:
		return m.viewHashing()
	case screenDup:
		return m.viewDup()
	case screenFind:
		return m.viewFind()
	}
	return ""
}

// viewerHeight is the number of file lines that fit: the whole terminal, less a
// one-line header and a one-line footer.
func (m *model) viewerHeight() int {
	return max(m.height-2, 1)
}

// viewViewer renders the file pager: a header naming the file, the visible slice
// of its lines, and a footer with the scroll keys. Exactly m.height lines, like
// every other screen, so the frame never scrolls on its own.
func (m *model) viewViewer() string {
	v := m.viewer

	// The marker costs two columns. On a terminal that cannot hold it and a
	// column of path, the path is the part worth keeping — the marker is chrome.
	const markerWidth = 2
	header := m.st.dirName.Render(runewidth.Truncate(v.path, max(m.width, 1), "…"))
	if m.width > markerWidth {
		header = m.st.accent.Render("▏ ") +
			m.st.dirName.Render(runewidth.Truncate(v.path, m.width-markerWidth, "…"))
	}

	height := m.viewerHeight()
	end := min(v.offset+height, len(v.lines))

	var b strings.Builder
	for i := v.offset; i < end; i++ {
		b.WriteString(m.st.fileName.Render(truncateLine(v.lines[i], m.width)))
		b.WriteByte('\n')
	}
	body := padLines(strings.TrimRight(b.String(), "\n"), height)

	// padLines has the last word on height: on a terminal too short for the header,
	// a body line and the footer, the whole thing is clipped to exactly m.height so
	// the frame never scrolls on its own.
	return padLines(header+"\n"+body+"\n"+m.viewViewerFooter(), m.height)
}

// viewViewerFooter is the pager's key hints, with the scroll position on the
// right when there is room for it.
//
// The hints are measured and cut as plain text and styled afterwards, never the
// other way round. Truncating the styled string is what this used to do, and it
// was wrong in a way only a second theme could show: runewidth counts an escape
// sequence's bytes as visible columns, so the same footer cut to the same width
// kept a different amount of text under charm than under mono — and under a
// colour theme, on a narrow terminal, it cut away nearly all of it.
func (m *model) viewViewerFooter() string {
	if m.width < 1 {
		return ""
	}

	hints := []keyHint{
		{key: "↑↓", label: "scroll"},
		{key: "q", label: "close"},
	}

	right := ""
	rightStyle := m.st.dim
	switch {
	case m.viewer.truncated:
		right, rightStyle = "first "+m.ui.formatSize(viewerReadCap)+" shown", m.st.danger
	case len(m.viewer.lines) > 0:
		// The line range, so a long file's scroll position is legible.
		last := min(m.viewer.offset+m.viewerHeight(), len(m.viewer.lines))
		right = fmt.Sprintf("%d–%d of %d", m.viewer.offset+1, last, len(m.viewer.lines))
	}

	// The position is the first thing to go: the keys are what the footer is for.
	keysWidth := plainKeyWidth(hints)
	if keysWidth+1+runewidth.StringWidth(right) > m.width {
		right = ""
	}

	rendered := m.renderKeys(hints)
	gap := m.width - keysWidth - runewidth.StringWidth(right)
	if gap < 1 {
		// Not even the keys fit whole. Cut them as text, then style — every hint
		// dropped rather than a hint cut in half would leave the footer empty on a
		// narrow terminal, and an empty footer says nothing at all.
		return m.st.dim.Render(runewidth.Truncate(plainKeys(hints), m.width, ""))
	}
	return rendered + strings.Repeat(" ", gap) + rightStyle.Render(right)
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
	if m.cancelling {
		// The walk cannot stop mid-directory, so there is a moment between esc and
		// the screen changing. Saying so is what stops that moment reading as a key
		// that did not register.
		status = "cancelling · finishing the directories already open"
	}

	// spinner(1) + gap(1) + status + cursor(1)
	const chrome = 3
	if m.width < 1 {
		return ""
	}
	// At or below the chrome's own width there is no room for a word about the
	// scan, and Truncate would not give one anyway — asked for zero columns it
	// returns its ellipsis, which is one. The spinner alone still says the only
	// thing that matters here: the scan is alive.
	if m.width <= chrome {
		return m.spinner.View()
	}
	status = runewidth.Truncate(status, m.width-chrome, "…")

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

	// Everywhere but the browser, the header path is a *title* — what you are
	// looking at, "duplicate files under ~/Work" — so it leads, right after the
	// wordmark, bright, tagline dropped. On the browser it is a *breadcrumb* —
	// where you are — so it stays quiet on the right and the tagline fills the gap.
	if m.scr != screenBrowse {
		return m.viewTitle(wordmark, m.headerPath())
	}
	return m.viewBreadcrumb(wordmark, "charm disk usage", m.headerPath())
}

// viewTitle puts the wordmark and a bright title on the left. The title takes
// the size colour — green — the same colour the browser's breadcrumb has always
// used for the path, so "where you are" reads the same whichever screen you are
// on. A title that will not fit keeps its tail: the specific end (the pattern,
// the directory) matters more than the word "duplicate" it starts with.
func (m *model) viewTitle(wordmark, title string) string {
	if title == "" {
		return m.st.accent.Render(runewidth.Truncate(wordmark, max(m.width, 1), ""))
	}
	full := wordmark + "  " + title
	if runewidth.StringWidth(full) > m.width {
		return m.st.size.Render(runewidth.FillRight(middleTruncate(title, max(m.width, 1)), max(m.width, 1)))
	}
	pad := m.width - runewidth.StringWidth(full)
	return m.st.accent.Render(wordmark) + "  " + m.st.size.Render(title) + strings.Repeat(" ", max(pad, 0))
}

func (m *model) viewBreadcrumb(wordmark, tagline, path string) string {
	// While anything is marked the subtitle gives way to the running tally: what is
	// queued for deletion matters more than a tagline, and the header is where a
	// queue building up is least likely to be missed.
	sub, subStyle, subMin := tagline, m.st.dim, minWidthForTagline
	if tally := m.markTally(); tally != "" {
		sub, subStyle, subMin = tally, m.st.accent, minWidthForTally
	}

	left := wordmark
	if m.width >= subMin {
		left += "  " + sub
	}

	const gap = 2
	avail := m.width - runewidth.StringWidth(left) - gap
	if avail < minNameWidth || path == "" {
		return m.st.accent.Render(runewidth.Truncate(left, max(m.width, 1), ""))
	}

	path = middleTruncate(path, avail)
	pad := m.width - runewidth.StringWidth(left) - runewidth.StringWidth(path)

	brand := m.st.accent.Render(wordmark)
	if m.width >= subMin {
		brand += "  " + subStyle.Render(sub)
	}
	return brand + strings.Repeat(" ", max(pad, 1)) + m.st.size.Render(path)
}

// headerPath is what the top right corner says we are looking at: the directory
// while browsing, and the root under way while scanning — the mock puts the same
// breadcrumb in both states, which is what makes the scan read as the same
// screen filling up rather than a different one.
//
// An accepted filter that is no longer being typed is shown here too, so that a
// directory listing fewer rows than it holds is never a mystery.
func (m *model) headerPath() string {
	if m.scr == screenDisks {
		return "select a device to analyze"
	}
	if m.scr == screenTop {
		return "largest files" + m.searchScopeSuffix()
	}
	if m.scr == screenQueue {
		return fmt.Sprintf("delete queue · %d %s · %s frees",
			len(m.queue), itemNoun(len(m.queue)), m.ui.formatSize(m.markedReclaimable()))
	}
	if m.scr == screenHelp {
		return "every key, one screen"
	}
	if m.scr == screenHashing {
		return "reading files to compare"
	}
	if m.scr == screenDup {
		return "duplicate files" + m.searchScopeSuffix()
	}
	if m.scr == screenFind {
		return fmt.Sprintf("%d matching “%s”%s", len(m.findResults), m.findPattern, m.searchScopeWord())
	}
	if m.scr == screenScanning {
		// -d comes up on this screen while the mount table is being read, and has no
		// path to name until a device is picked — "scanning " alone reads like a bug.
		if m.ui.showDisks && m.ui.scanPath == "" {
			return "reading the mount table"
		}
		return "scanning " + m.ui.scanPath
	}
	if m.currentDir == nil {
		return ""
	}
	path := m.currentDir.GetPath()
	if !m.filtering && m.filter != "" {
		path += "  /" + m.filter
	}
	return path
}

// viewDiskLine is the volume gauge from the mock: how full the disk is that the
// scan root lives on. It answers the question the scan cannot — "how much of
// this machine is even at stake" — so the bar is drawn against the disk's own
// capacity, not against anything in the tree.
func (m *model) viewDiskLine() string {
	used, size := m.dev.GetUsage(), m.dev.Size
	// The figures say how much, the percentage says how full — the number you
	// actually read off a gauge. "627 GiB / 994 GiB · 63%".
	usage := fmt.Sprintf("%s / %s · %s", m.ui.formatSize(used), m.ui.formatSize(size), formatPct(used, size))

	const gaps = 2 // one space each side of the bar
	// The name keeps to its own length and the bar takes the rest, so the gauge
	// is long enough to read a level off rather than a stub beside a wide, empty
	// label — which is what a fixed-width bar left on a roomy terminal.
	name := m.dev.Name
	barWidth := m.width - runewidth.StringWidth(name) - runewidth.StringWidth(usage) - gaps

	// Too narrow for the whole name and a legible bar: the name gives up its
	// columns, so the gauge stays readable rather than the label staying whole.
	if barWidth < minDiskBar {
		barWidth = min(minDiskBar, max(m.width-runewidth.StringWidth(usage)-gaps, 0))
		nameW := max(m.width-barWidth-runewidth.StringWidth(usage)-gaps, 0)
		name = runewidth.FillRight(runewidth.Truncate(name, nameW, "…"), nameW)
	}

	bar := m.bar.render(fraction(used, size), max(barWidth, 0))
	return m.st.dim.Render(name) + " " + bar + " " + m.st.pct.Render(usage)
}

func (m *model) viewList() string {
	lines := m.visibleLines()
	items := m.items()

	if len(items) == 0 {
		empty := "  (empty)"
		if m.filtered != nil {
			// An active filter matching nothing is a different situation from an
			// empty directory, and saying so is what stops it reading as a bug.
			empty = "  no matches for “" + m.filter + "”"
		}
		return padLines(m.st.dim.Render(empty), lines)
	}

	// The window, and only the window, is rendered. A directory can hold tens of
	// thousands of entries; building a string for all of them every frame is the
	// cost this whole design exists to avoid.
	end := min(m.offset+m.visibleRows(), len(items))

	total := m.rowScale()

	var b strings.Builder
	for i := m.offset; i < end; i++ {
		b.WriteString(m.viewEntry(items[i], i == m.cursor, total))
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

	icon := m.rowIcon(item)

	sizeText := padLeft(m.ui.formatSize(size), sizeColWidth)

	pctText := ""
	if m.width >= minWidthForPct {
		pctText = padLeft(runewidth.Truncate(formatPct(size, total), pctColWidth, ""), pctColWidth)
	}

	// The optional columns are only drawn where they leave the name enough room to
	// be worth reading. Asking for a column the terminal cannot fit and getting a
	// four-character name back is not what the keypress meant, so the column simply
	// does not appear — and toggleLabel says the state changed regardless.
	extras := m.extraColumns(item)

	// The row is: gutter(1) + icon + size + gap(1) + extras + name + pct. The gutter
	// holds either the selection marker or a blank, so both variants are the same
	// width.
	const fixedCells = 2 // gutter + the gap between size and name
	rawNameWidth := m.width - runewidth.StringWidth(icon) - sizeColWidth -
		runewidth.StringWidth(extras) - runewidth.StringWidth(pctText) - fixedCells
	nameWidth := max(rawNameWidth, minNameWidth)
	// Floored means the name column hit its minimum and the row no longer adds up
	// to the terminal width — the selected branch clips it whole rather than
	// composing it, so the width stays exact.
	floored := rawNameWidth < minNameWidth

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
	// A file the duplicate search matched carries a mark and, beside it, the words
	// that explain the mark — "⧉ 3 copies" — so a first-time reader is never left
	// to guess what the glyph means. The glyph is a second cue on top of the
	// colour, so the row still reads under mono and to a colourblind eye.
	if tag := m.duplicateTag(item); tag != "" {
		name += " " + tag
	}
	if removing {
		// The word, not just the spinner: the state has to survive --no-color and a
		// terminal too narrow for the icon column.
		name = "removing " + name
	}
	nameText := runewidth.FillRight(runewidth.Truncate(name, nameWidth, "…"), nameWidth)

	plain := icon + sizeText + " " + extras + nameText + pctText

	if selected {
		return m.viewSelectedRow(plain, icon+sizeText+" "+extras, nameText, pctText, floored, m.isMarked(item))
	}

	// Floored: the terminal is narrower than the columns' own minimums add up to,
	// so the row cannot be composed to fit. Clip it whole, exactly as the selected
	// row already does — otherwise it overflows, the terminal soft-wraps it, and
	// the frame pushes itself down the screen on every render. That is the
	// horizontal twin of the bug padLines exists for.
	//
	// The per-column colour goes with it, which costs nothing: no colour here
	// carries meaning the text does not. Clipping happens before styling, never
	// after — a styled string measured by rune count loses most of its content.
	if floored {
		return m.st.fileName.Render(clipTo(" "+plain, m.width))
	}

	nameStyle := m.st.fileName
	iconStyle := m.st.dim
	switch {
	case removing:
		nameStyle, iconStyle = m.st.dim, m.st.accent
	case m.isDuplicate(item):
		// The mark and the whole name take the accent, so a duplicate stands out of
		// a long list rather than hiding a glyph at the end of a truncated name.
		nameStyle, iconStyle = m.st.accent, m.st.accent
	case item.IsDir():
		nameStyle, iconStyle = m.st.dirName, m.st.accent
	}

	// Under a filter, the runes the query matched are lit up so the reason a row is
	// here is visible.
	renderedName := nameStyle.Render(nameText)
	if m.filter != "" {
		renderedName = highlightMatch(nameText, m.filter, &nameStyle, &m.st.accent)
	}

	return m.rowGutter(item) + iconStyle.Render(icon) +
		m.st.size.Render(sizeText) + " " +
		m.st.dim.Render(extras) +
		renderedName +
		m.st.pct.Render(pctText)
}

// rowGutter is the one-cell head of a row: a tick when the row is queued for a
// batch delete, otherwise blank. It is always one cell, so a marked row still lines
// up with an unmarked one — the same reason the selection marker shares the column.
func (m *model) rowGutter(item fs.Item) string {
	if m.isMarked(item) {
		return m.st.accent.Render(m.markGlyph())
	}
	return " "
}

// viewSelectedRow draws the cursor row. No box-shadow in a terminal: the mock's
// glow becomes a filled background, a bold name, and a bright marker — the marker
// being what survives --no-color, NO_COLOR and the mono theme.
//
// With a filter on, the matched runes are lit up here too, composed from segments
// that all carry the selection background so the row stays one block. When the
// name column has been floored the row no longer adds up to the exact width, so it
// is clipped whole rather than composed.
func (m *model) viewSelectedRow(plain, prefix, nameText, pctText string, floored, marked bool) string {
	if m.width < 1 {
		return ""
	}
	// A marked cursor row shows the tick in the gutter, not the selection bar: the
	// filled background already says which row the cursor is on, so the one cell is
	// better spent saying the row is also queued for deletion.
	marker := m.st.accent.Render("▌")
	if marked {
		marker = m.st.accent.Render(m.markGlyph())
	}
	// One column: the marker alone. It is the whole of what the cursor row has to
	// say at this size, and it is the cue that survives --no-color anyway — there
	// is no room for it *and* a column of name.
	if m.width < 2 {
		return marker
	}

	if m.filter != "" && !floored {
		return marker +
			m.st.selected.Render(prefix) +
			highlightMatch(nameText, m.filter, &m.st.selected, &m.st.selectedMatch) +
			m.st.selected.Render(pctText)
	}
	return marker + m.st.selected.MaxWidth(max(m.width-1, 1)).Render(plain)
}

// extraColumns renders the optional item-count and mtime columns, in that order,
// and nothing at all where they would not fit.
func (m *model) extraColumns(item fs.Item) string {
	out := ""
	if m.ui.showItemCount && m.width >= minWidthForItemCount {
		count := item.GetItemCount()
		if item.IsDir() {
			// A directory counts itself. Showing "1 item" for an empty directory is
			// how gdu reads too, and it is not what anyone means by the column.
			count--
		}
		out += padLeft(runewidth.Truncate(humanCount(count), countColWidth, ""), countColWidth) + " "
	}
	if m.ui.showMtime && m.width >= minWidthForMtime {
		out += item.GetMtime().Format(mtimeLayout) + " "
	}
	return out
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

// keyHint is one binding in the footer, and how readily it is given up when the
// terminal is too narrow to show them all.
type keyHint struct {
	key   string
	label string
	// drop is the order hints are shed in: the highest goes first. Movement and the
	// way out are never dropped; a key you cannot discover is one you do not have.
	drop int
}

// The footer lists only keys that actually do something on the screen you are
// on. An interface that advertises a binding it does not have is worse than one
// that says nothing.
//
// Sorting costs one hint here rather than four, because the fields are asked for
// only once s has been pressed — and then the footer explains nothing else.
var (
	// The footer shows the essentials and the way to everything else. It used to
	// list a dozen keys and the fitKeys logic quietly dropped most of them on any
	// real terminal — including ? itself, so the one key that reveals the rest was
	// the first to go. Now it shows what you navigate with, the commonest action,
	// and ? — which opens the screen that has every key on it. Discover the rest
	// there, not by squinting at a truncated footer.
	browseKeys = []keyHint{
		{key: "↑↓", label: "move"},
		{key: "→", label: "open"},
		{key: "←", label: "back"},
		{key: "d", label: "trash", drop: 2},
		{key: "?", label: "help", drop: 1},
		{key: "q", label: "quit"},
	}
	// undoKey appears only when there is something to undo — see browseFooterKeys.
	// It sits after delete, which is where the eye is after a delete.
	undoKey = keyHint{key: "u", label: "undo", drop: 3}
	// markKey and queueKey grow the footer around delete: space marks a row for a
	// batch delete, M opens the queue of what is marked. queueKey shows only once
	// there is a queue to open. markKey sheds before trash itself — one down from d,
	// a step up from ? — so a narrow footer keeps the delete key over the mark hint.
	markKey  = keyHint{key: "space", label: "mark", drop: 3}
	queueKey = keyHint{key: "M", label: "queue", drop: 4}
	// The whole footer becomes the menu: while a mode is on, nothing else is worth
	// saying, and a mode nobody can see is a trap.
	sortMenuKeys = []keyHint{
		{key: "s", label: "size"},
		{key: "n", label: "name"},
		{key: "c", label: "count"},
		{key: "m", label: "mtime"},
		// d is not a field but a modifier on top of the field: it floats folders
		// above files whatever the list is sorted by. Its label flips to name what
		// pressing it would do, since it is the one key here that toggles rather than
		// picks. It sheds first — the four fields are the point of this menu.
		{key: "d", label: "dirs first", drop: 4},
		{key: "esc", label: "cancel"},
	}
	colMenuKeys = []keyHint{
		{key: "a", label: "apparent"},
		{key: "B", label: "relative"},
		{key: "c", label: "count"},
		{key: "m", label: "mtime"},
		// A key nobody can see is a key nobody has, and this is the only place the
		// save is advertised — so it sheds last, before only `cancel`.
		{key: "s", label: "save view", drop: 5},
		{key: "esc", label: "cancel"},
	}
	scanKeys = []keyHint{
		{key: "esc", label: "cancel"},
		{key: "q", label: "quit"},
	}
	diskKeys = []keyHint{
		{key: "↑↓", label: "move"},
		{key: "↵", label: "analyze"},
		{key: "r", label: "reread", drop: 3},
		{key: "?", label: "help", drop: 2},
		{key: "q", label: "quit"},
	}
	helpKeys = []keyHint{
		{key: "↑↓", label: "scroll"},
		{key: "?", label: "close"},
		{key: "esc", label: "back"},
	}
	hashingKeys = []keyHint{
		{key: "esc", label: "cancel"},
		{key: "q", label: "quit"},
	}
	dupKeys = []keyHint{
		{key: "↑↓", label: "move"},
		{key: "↵", label: "reveal"},
		{key: "v", label: "view", drop: 3},
		{key: "d", label: "trash", drop: 1},
		{key: "D", label: "delete", drop: 2},
		{key: "?", label: "help", drop: 4},
		{key: "esc", label: "back"},
		{key: "q", label: "quit", drop: 4},
	}
	// findKeys is the results screen; the input prompt has its own footer.
	findKeys = []keyHint{
		{key: "↑↓", label: "move"},
		{key: "↵", label: "reveal"},
		{key: "v", label: "view", drop: 3},
		{key: "d", label: "trash", drop: 1},
		{key: "D", label: "delete", drop: 2},
		{key: "?", label: "help", drop: 4},
		{key: "esc", label: "back"},
		{key: "q", label: "quit", drop: 4},
	}
	topKeys = []keyHint{
		{key: "↑↓", label: "move"},
		{key: "↵", label: "reveal"},
		{key: "v", label: "view", drop: 3},
		{key: "d", label: "trash", drop: 1},
		{key: "D", label: "delete", drop: 2},
		{key: "?", label: "help", drop: 4},
		{key: "esc", label: "back"},
		{key: "q", label: "quit", drop: 4},
	}
	confirmKeys = []keyHint{
		{key: "←→", label: "choose"},
		{key: "enter", label: "confirm"},
		{key: "esc", label: "cancel"},
	}
	queueKeys = []keyHint{
		{key: "↑↓", label: "move"},
		{key: "space", label: "unmark"},
		{key: "↵", label: "reveal"},
		{key: "d", label: "trash all", drop: 1},
		{key: "D", label: "delete all", drop: 2},
		{key: "?", label: "help", drop: 4},
		{key: "esc", label: "back"},
		{key: "q", label: "quit", drop: 4},
	}
)

// maxDropLevel is the number of shedding rounds fitKeys will run.
const maxDropLevel = 5

// fitKeys drops the least essential hints until the row fits. Truncating the
// string instead would leave a hint cut in half, which reads as a rendering fault
// rather than as a choice.
func fitKeys(keys []keyHint, budget int) []keyHint {
	for level := maxDropLevel; level > 0 && hintsWidth(keys) > budget; level-- {
		kept := make([]keyHint, 0, len(keys))
		for _, k := range keys {
			if k.drop == 0 || k.drop < level {
				kept = append(kept, k)
			}
		}
		keys = kept
	}
	return keys
}

func hintsWidth(keys []keyHint) int {
	width := 0
	for i, k := range keys {
		if i > 0 {
			width += 2
		}
		width += runewidth.StringWidth(k.key) + 1 + runewidth.StringWidth(k.label)
	}
	return width
}

// browseFooterKeys is the browse hints, grown around the delete key: space to
// mark always, the queue once something is marked, and undo once something has
// been trashed. Each appears only when it would do something — the footer promises
// only what it can keep, the same rule that keeps undo hidden until there is a
// trashed item to bring back.
func (m *model) browseFooterKeys() []keyHint {
	keys := make([]keyHint, 0, len(browseKeys)+3)
	for _, k := range browseKeys {
		keys = append(keys, k)
		if k.key != "d" {
			continue
		}
		keys = append(keys, markKey)
		if m.markedCount() > 0 {
			keys = append(keys, queueKey)
		}
		if len(m.lastTrashed) > 0 {
			keys = append(keys, undoKey)
		}
	}
	return keys
}

func (m *model) viewFooter() string {
	if m.filtering {
		return m.viewFilterFooter()
	}
	if m.finding {
		return m.viewFindFooter()
	}

	keys := m.browseFooterKeys()
	switch {
	case m.scr == screenScanning:
		keys = scanKeys
	case m.scr == screenDisks:
		keys = diskKeys
	case m.scr == screenTop:
		keys = topKeys
	case m.scr == screenQueue:
		keys = queueKeys
	case m.scr == screenHelp:
		keys = helpKeys
	case m.scr == screenHashing:
		keys = hashingKeys
	case m.scr == screenDup:
		keys = dupKeys
	case m.scr == screenFind:
		keys = findKeys
	case m.scr == screenConfirm:
		keys = confirmKeys
	case m.sortPending:
		keys = m.sortMenuKeys()
	case m.colPending:
		keys = colMenuKeys
	}

	right, rightStyle := m.footerRight()

	// The right side is reserved the room it will actually take, never below the
	// minimum, and the hints shed to leave it. Reserving a fixed minimum instead
	// dropped a long note — the duplicate explanation — whenever the keys ran wide
	// enough to collide with it, which is exactly when the note matters.
	reserve := minRightWidth
	if w := runewidth.StringWidth(right) + 2; w > reserve {
		reserve = w
	}
	keys = fitKeys(keys, max(m.width-reserve, 0))

	plain, styled := plainKeys(keys), m.renderKeys(keys)

	if m.width < 1 {
		return ""
	}
	gap := m.width - runewidth.StringWidth(plain) - runewidth.StringWidth(right)
	if gap < 1 {
		// Too narrow for both. The keys are the only way out of the screen, so they
		// are what survives — except when something just happened, which is the one
		// thing the user needs to read.
		if m.status != "" {
			return rightStyle.Render(runewidth.Truncate(right, m.width, "…"))
		}
		return m.st.dim.Render(runewidth.Truncate(plain, m.width, ""))
	}
	return styled + strings.Repeat(" ", gap) + rightStyle.Render(right)
}

// footerRight is the footer's right-hand side, in order of what matters most now:
// the menu you are in, then what just happened, then — while browsing — the note
// about a duplicate under the cursor, and failing all that, how the list is sorted.
// A destructive action that reported nothing would look the same as one that
// silently failed, which is why the status outranks the standing labels.
func (m *model) footerRight() (string, lipgloss.Style) {
	switch {
	case m.sortPending:
		// Naming the mode is what tells you the next keystroke means something other
		// than usual; the keys alone read as ordinary bindings.
		return "sort by…", m.st.accent
	case m.colPending:
		return "toggle column…", m.st.accent
	case m.status != "":
		if m.statusIsError {
			return m.status, m.st.danger
		}
		return m.status, m.st.accent
	case m.scr == screenBrowse && m.duplicateNote() != "":
		// The cursor is on a duplicate. Spell out what the mark means here, where it
		// can be read — it outranks the sort label, which says nothing about this row.
		return m.duplicateNote(), m.st.accent
	case m.scr == screenBrowse:
		return m.sortLabel(), m.st.dim
	}
	return "", m.st.dim
}

// plainKeys is the hints as bare text. It is the version that gets measured and
// cut: its width is what it looks like, which is not true of the styled one.
func plainKeys(keys []keyHint) string {
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(k.key)
		b.WriteByte(' ')
		b.WriteString(k.label)
	}
	return b.String()
}

func plainKeyWidth(keys []keyHint) int {
	return runewidth.StringWidth(plainKeys(keys))
}

// renderKeys is the same hints, styled. It is built rune-for-rune alongside
// plainKeys rather than by styling it afterwards, so the two cannot drift and
// neither ever has to be measured through its escapes.
func (m *model) renderKeys(keys []keyHint) string {
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(m.st.accent.Render(k.key))
		b.WriteByte(' ')
		b.WriteString(m.st.dim.Render(k.label))
	}
	return b.String()
}

// viewFilterFooter is the / input line: the query being typed on the left, and
// how many of the directory's entries it matches on the right. The count is the
// feedback that makes fuzzy typing legible — you can see the list narrowing.
func (m *model) viewFilterFooter() string {
	prompt := m.st.accent.Render("/") + m.st.dirName.Render(m.filter) + m.st.accent.Render("▏")

	count := fmt.Sprintf("%d of %d", len(m.items()), len(m.rows))
	if len(m.items()) == 0 {
		count = "no matches"
	}
	right := m.st.dim.Render(count)

	gap := m.width - lipgloss.Width(prompt) - lipgloss.Width(right)
	if gap < 1 {
		return runewidth.Truncate(prompt, max(m.width, 1), "")
	}
	return prompt + strings.Repeat(" ", gap) + right
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
