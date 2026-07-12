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
	sizeColWidth = 10
	pctColWidth  = 5
	iconColWidth = 2

	// Below these widths the layout sheds its least essential column rather
	// than wrapping or smearing.
	minWidthForPct  = 70
	minWidthForIcon = 44

	// Below this height the header and footer are dropped so the list still has
	// somewhere to live. Smaller than this and we clamp rather than crash.
	minHeightForChrome = 5

	minNameWidth = 4
)

func (m *model) headerHeight() int {
	if m.height < minHeightForChrome {
		return 0
	}
	return 2 // wordmark + breadcrumb line, then a rule
}

func (m *model) footerHeight() int {
	if m.height < minHeightForChrome {
		return 0
	}
	return 1
}

// visibleRows is the number of list rows that fit right now. It is always at
// least one, so a degenerate terminal clamps to a minimal layout instead of
// producing a negative height.
func (m *model) visibleRows() int {
	n := m.height - m.headerHeight() - m.footerHeight()
	if n < 1 {
		return 1
	}
	return n
}

func (m *model) View() string {
	// Bubble Tea sends WindowSizeMsg on startup; until it lands we have no
	// honest size to lay out against.
	if !m.haveSize {
		return ""
	}

	st := newStyles(charmPalette(), m.ui.UseColors)

	switch m.scr {
	case screenError:
		return st.danger.Render("error: ") + m.err.Error() + "\n"
	case screenScanning:
		return m.viewScanning(st)
	default:
		return m.viewBrowse(st)
	}
}

func (m *model) viewScanning(st styles) string {
	progress := fmt.Sprintf(
		"walking directories · %s items · %s",
		humanCount(m.progress.ItemCount),
		m.ui.formatSize(m.progress.TotalUsage),
	)

	name := m.progress.CurrentItemName
	if name != "" {
		avail := m.width - 4
		if avail > minNameWidth {
			name = st.dim.Render(middleTruncate(name, avail))
		} else {
			name = ""
		}
	}

	body := m.spinner.View() + " " + st.accent.Render(progress)
	if name != "" {
		body += "\n" + name
	}
	return body + "\n"
}

func (m *model) viewBrowse(st styles) string {
	var b strings.Builder

	if m.headerHeight() > 0 {
		b.WriteString(m.viewHeader(st))
		b.WriteByte('\n')
	}

	b.WriteString(m.viewList(st))

	if m.footerHeight() > 0 {
		b.WriteString(m.viewFooter(st))
	}
	return b.String()
}

func (m *model) viewHeader(st styles) string {
	wordmark := st.accent.Render("cdu ✦")

	path := ""
	if m.currentDir != nil {
		path = m.currentDir.GetPath()
	}

	// The breadcrumb gets whatever the wordmark leaves behind, and is
	// middle-truncated so both the root and the leaf stay readable.
	avail := m.width - lipgloss.Width(wordmark) - 5
	line := wordmark
	if avail > minNameWidth {
		line += "  " + st.dim.Render("at ") + st.size.Render(middleTruncate(path, avail))
	}

	rule := st.dim.Render(strings.Repeat("─", maxInt(m.width, 1)))
	return line + "\n" + rule
}

func (m *model) viewList(st styles) string {
	visible := m.visibleRows()

	if len(m.rows) == 0 {
		return padLines(st.dim.Render("  (empty)"), visible)
	}

	// The window, and only the window, is rendered. A directory can hold tens of
	// thousands of entries; building a string for all of them every frame is the
	// cost this whole design exists to avoid.
	end := minInt(m.offset+visible, len(m.rows))

	// The percentage is the entry's share of the parent directory total.
	total := int64(0)
	if m.currentDir != nil {
		total = m.itemSize(m.currentDir)
	}

	var b strings.Builder
	for i := m.offset; i < end; i++ {
		b.WriteString(m.viewRow(st, m.rows[i], i == m.cursor, total))
		b.WriteByte('\n')
	}
	return padLines(strings.TrimRight(b.String(), "\n"), visible)
}

func (m *model) viewRow(st styles, item fs.Item, selected bool, total int64) string {
	size := m.itemSize(item)

	icon := ""
	if m.width >= minWidthForIcon {
		switch {
		case m.ui.noUnicode && item.IsDir():
			icon = "> "
		case m.ui.noUnicode:
			icon = "  "
		case item.IsDir():
			icon = st.accent.Render("▸") + " "
		default:
			icon = st.dim.Render("·") + " "
		}
	}

	sizeCell := st.size.Render(padLeft(m.ui.formatSize(size), sizeColWidth))

	pct := ""
	if m.width >= minWidthForPct {
		pct = st.pct.Render(padLeft(formatPct(size, total), pctColWidth))
	}

	nameWidth := m.width - iconWidthFor(m.width) - sizeColWidth - lipgloss.Width(pct) - 3
	if nameWidth < minNameWidth {
		nameWidth = minNameWidth
	}

	name := item.GetName()
	if item.IsDir() {
		name += "/"
	}
	// Flags carry meaning that must survive mono, NO_COLOR and colourblindness,
	// so they are a glyph, not a colour.
	if f := item.GetFlag(); f == '!' {
		name += " !"
	} else if f == 'H' {
		name += " ⇉"
	}
	nameCell := runewidth.Truncate(name, nameWidth, "…")
	nameCell = runewidth.FillRight(nameCell, nameWidth)

	if item.IsDir() {
		nameCell = st.dirName.Render(nameCell)
	} else {
		nameCell = st.fileName.Render(nameCell)
	}

	row := icon + sizeCell + " " + nameCell + pct
	if selected {
		// No box-shadow in a terminal: the mock's glow becomes a filled
		// background plus a bright marker, and the marker is what survives
		// --no-color.
		marker := st.accent.Render("▌")
		return marker + st.selected.Render(runewidth.Truncate(
			stripTrailing(row), maxInt(m.width-1, 1), "",
		))
	}
	return " " + row
}

func (m *model) viewFooter(st styles) string {
	keys := []string{"↑↓ move", "→ open", "← back", "q quit"}
	left := st.dim.Render(strings.Join(keys, "  "))

	right := st.dim.Render("sorted by size · desc")
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		// Too narrow for both: the sort state is the more droppable of the two.
		return runewidth.Truncate(left, maxInt(m.width, 1), "")
	}
	return left + strings.Repeat(" ", gap) + right
}

// itemSize honours --apparent-size, which is a display choice: the engine
// always carries both figures on every item.
func (m *model) itemSize(item fs.Item) int64 {
	if m.ui.ShowApparentSize {
		return item.GetSize()
	}
	return item.GetUsage()
}
