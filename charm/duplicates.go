package charm

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/internal/dup"
	"github.com/pottom/cdu/pkg/fs"
)

// `F` finds files with identical content and marks them, so the same film in two
// directories stops hiding as two unrelated large files.
//
// It is the one thing in cdu that reads file contents — everything else stats —
// so it is opt-in and it reads off the render loop. The search hashes only files
// that share a size with another, which is most of the cost avoided before it is
// paid: unique sizes are never opened. It is cancellable with esc, like a scan.

// dupMark is the glyph a duplicated file carries in the browser. A geometric
// triangle rather than the warning sign ⚠: runewidth measures both as one cell,
// but ⚠ is in the emoji block and a colour terminal may draw it two cells wide,
// which would shift the row. The triangle is stable. It renders in the accent,
// like the rest of a duplicate's name.
const dupMark = "▲"

// dupRow is one line of the duplicate screen: a group header, or a file in one.
type dupRow struct {
	// group is set on a header row and nil on a file row.
	group *dup.Group
	// file and its group's size are set on a file row.
	file fs.Item
	size int64
	// last marks the final file in its group, for the tree glyph.
	last bool
}

func (r *dupRow) isHeader() bool { return r.group != nil }

type dupDoneMsg struct {
	groups []dup.Group
	err    error
}

// dupCmd runs the search off the render loop. It reads every candidate file,
// which can take a while on a tree full of large look-alikes, so it is a command
// and not a keystroke that freezes the interface — exactly the case the tea.Cmd
// rule is for. It shares the scan's cancel flag; the two never run at once.
func dupCmd(root fs.Item, cancel func() bool) tea.Cmd {
	return func() tea.Msg {
		groups, err := dup.Find(root, cancel)
		return dupDoneMsg{groups: groups, err: err}
	}
}

// findDuplicates starts the search.
func (m *model) findDuplicates() (tea.Model, tea.Cmd) {
	if m.topDir == nil {
		m.status, m.statusIsError = "nothing scanned yet", true
		return m, nil
	}
	m.ui.cancel.Store(false)
	m.cancelling = false
	m.status, m.statusIsError = "", false
	m.scr = screenHashing
	return m, tea.Batch(m.spinner.Tick, dupCmd(m.topDir, m.ui.cancel.Load))
}

// handleHashingKey mirrors the scan screen: esc cancels, q quits. The search
// checks the cancel flag between files, so esc lands within one file's read.
func (m *model) handleHashingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape:
		if !m.cancelling {
			m.cancelling = true
			m.ui.cancel.Store(true)
		}
		return m, nil
	case "q", keyCtrlC:
		m.ui.cancel.Store(true)
		return m, tea.Quit
	}
	return m, nil
}

// applyDupDone takes the result of the search.
func (m *model) applyDupDone(msg dupDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		// The only error the finder returns is cancellation, which is not a failure
		// — it is the user changing their mind. Back to the browser, quietly.
		m.scr = screenBrowse
		m.cancelling = false
		m.status, m.statusIsError = "duplicate search cancelled", false
		return m, nil
	}

	m.setDuplicates(msg.groups)
	if len(m.dupGroups) == 0 {
		m.scr = screenBrowse
		m.status, m.statusIsError = "no duplicate files found", false
		return m, nil
	}

	m.scr = screenDup
	m.dupCursor, m.dupOffset = 0, 0
	m.moveDupCursor(0) // land on a file, not a header
	m.status, m.statusIsError = m.dupSummary(), false
	return m, nil
}

// setDuplicates installs the groups: the flat rows the screen draws, and the set
// the browser marks against.
func (m *model) setDuplicates(groups []dup.Group) {
	m.dupGroups = groups
	m.dupRows = m.dupRows[:0]
	m.dupMarked = make(map[fs.Item]bool)

	for i := range groups {
		g := &groups[i]
		m.dupRows = append(m.dupRows, dupRow{group: g})
		for j, f := range g.Files {
			m.dupRows = append(m.dupRows, dupRow{file: f, size: g.Size, last: j == len(g.Files)-1})
			m.dupMarked[f] = true
		}
	}
}

// isDuplicate reports whether a browser row should carry the ▲. It is why the
// marked set is kept rather than recomputed: the browser asks this for every
// visible row, every frame.
func (m *model) isDuplicate(item fs.Item) bool {
	return m.dupMarked != nil && m.dupMarked[item]
}

func (m *model) dupSummary() string {
	var total int64
	for _, g := range m.dupGroups {
		total += g.Reclaimable()
	}
	groups := "groups"
	if len(m.dupGroups) == 1 {
		groups = "group"
	}
	return fmt.Sprintf("%d duplicate %s · %s reclaimable", len(m.dupGroups), groups, m.ui.formatSize(total))
}

func (m *model) selectedDup() fs.Item {
	if m.dupCursor < 0 || m.dupCursor >= len(m.dupRows) {
		return nil
	}
	return m.dupRows[m.dupCursor].file
}

func (m *model) handleDupKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape, keyLeft, "h":
		m.scr = screenBrowse
		return m, nil
	case keyUp, "k":
		m.moveDupCursor(-1)
	case keyDown, "j":
		m.moveDupCursor(1)
	case keyHome, "g":
		m.moveDupCursor(-len(m.dupRows))
	case keyEnd, "G":
		m.moveDupCursor(len(m.dupRows))
	case keyPgUp:
		m.moveDupCursor(-m.visibleLines())
	case keyPgDown:
		m.moveDupCursor(m.visibleLines())
	case keyEnter, keyRight, "l":
		return m.revealDup()
	case "v":
		return m.openViewer()
	case "d":
		m.askConfirm(actionTrash)
	case "D":
		m.askConfirm(actionDelete)
	}
	return m, nil
}

// moveDupCursor moves by delta and skips group headers, which cannot be acted
// on — a header is a label, not a file.
func (m *model) moveDupCursor(delta int) {
	if len(m.dupRows) == 0 {
		return
	}
	want := min(max(m.dupCursor+delta, 0), len(m.dupRows)-1)

	step := 1
	if delta < 0 {
		step = -1
	}
	m.dupCursor = m.nextSelectableDup(want, step)

	height := max(m.visibleLines(), 1)
	top := m.dupCursor
	if top > 0 && m.dupRows[top-1].isHeader() {
		top-- // keep the header in view above its files
	}
	m.dupOffset = min(m.dupOffset, top)
	if m.dupCursor >= m.dupOffset+height {
		m.dupOffset = m.dupCursor - height + 1
	}
	m.dupOffset = min(max(m.dupOffset, 0), max(len(m.dupRows)-height, 0))
}

func (m *model) nextSelectableDup(from, step int) int {
	for _, dir := range []int{step, -step} {
		for i := from; i >= 0 && i < len(m.dupRows); i += dir {
			if !m.dupRows[i].isHeader() {
				return i
			}
		}
	}
	return from
}

// revealDup opens the file's directory in the browser with the cursor on it —
// the same move as reveal on the largest-files screen. From here you can see
// what is around a copy before you decide which to keep.
func (m *model) revealDup() (tea.Model, tea.Cmd) {
	item := m.selectedDup()
	if item == nil {
		return m, nil
	}
	parent := item.GetParent()
	if parent == nil {
		return m, nil
	}
	m.enterDir(parent)
	for i, r := range m.rows {
		if r == item {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
	m.scr = screenBrowse
	return m, nil
}

// dropDuplicate takes a deleted file out of the duplicate set, and dissolves any
// group that a deletion has left with a single copy — one file is not a
// duplicate of anything.
func (m *model) dropDuplicate(item fs.Item) {
	if !m.isDuplicate(item) {
		return
	}
	var survivors []dup.Group
	for _, g := range m.dupGroups {
		kept := g.Files[:0:0]
		for _, f := range g.Files {
			if f != item {
				kept = append(kept, f)
			}
		}
		if len(kept) >= 2 {
			survivors = append(survivors, dup.Group{Size: g.Size, Files: kept})
		}
	}
	m.setDuplicates(survivors)

	if m.scr == screenDup {
		if len(m.dupRows) == 0 {
			// The last duplicate is gone. There is nothing left to show.
			m.scr = screenBrowse
			m.status, m.statusIsError = "no more duplicates", false
			return
		}
		m.dupCursor = min(m.dupCursor, len(m.dupRows)-1)
		m.moveDupCursor(0)
	}
}

// viewHashing is the spinner while the search reads files. It borrows the scan
// screen's shape so the interface does not appear to jump to something else.
func (m *model) viewHashing() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}

	word := "searching for duplicate files"
	if m.cancelling {
		word = "cancelling · finishing the file being read"
	}
	line := m.spinner.View() + " " + m.st.dim.Render(word)
	if m.width < 1 {
		line = ""
	}
	parts = append(parts, padLines(line, m.visibleLines()))

	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}

func (m *model) viewDupList() string {
	lines := m.visibleLines()
	if len(m.dupRows) == 0 {
		return padLines(m.st.dim.Render(clipTo("  no duplicates", m.width)), lines)
	}

	end := min(m.dupOffset+lines, len(m.dupRows))
	rows := make([]string, 0, lines)
	for i := m.dupOffset; i < end; i++ {
		rows = append(rows, m.viewDupRow(&m.dupRows[i], i == m.dupCursor))
	}
	return padLines(joinLines(rows), lines)
}

// viewDupRow is a group header or a file under one.
func (m *model) viewDupRow(r *dupRow, selected bool) string {
	if m.width < 1 {
		return ""
	}
	if r.isHeader() {
		return m.viewDupHeader(r.group)
	}
	return m.viewDupFile(r, selected)
}

// viewDupHeader labels a group by what it costs: how many copies, each how big,
// and how much deleting the extras would free.
func (m *model) viewDupHeader(g *dup.Group) string {
	label := fmt.Sprintf("%d copies · %s each · reclaim %s",
		len(g.Files), m.ui.formatSize(g.Size), m.ui.formatSize(g.Reclaimable()))
	return m.st.dirName.Render(clipTo(" "+label, m.width))
}

func (m *model) viewDupFile(r *dupRow, selected bool) string {
	branch := treeBranch
	if r.last {
		branch = treeLast
	}
	if m.ui.noUnicode {
		branch = asciiTreeBranch
		if r.last {
			branch = asciiTreeLast
		}
	}

	sizeText := cellRight(m.ui.formatSize(r.size), sizeColWidth)
	path := r.file.GetPath()

	// gutter + branch + size + gap + path
	rest := m.width - 1 - runewidth.StringWidth(branch) - sizeColWidth - 1
	pathText := cellPath(path, max(rest, 0))

	plain := branch + sizeText + " " + pathText
	if selected {
		if m.width < 2 {
			return m.st.accent.Render("▌")
		}
		return m.st.accent.Render("▌") + m.st.selected.Render(clipTo(plain, m.width-1))
	}
	if 1+runewidth.StringWidth(plain) > m.width {
		return m.st.fileName.Render(clipTo(" "+plain, m.width))
	}
	return " " + m.st.dim.Render(branch) +
		m.st.size.Render(sizeText) + " " +
		m.st.fileName.Render(pathText)
}

func (m *model) viewDup() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewDupList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
