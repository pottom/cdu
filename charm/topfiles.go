package charm

import (
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// `T` shows the biggest files anywhere in the scan, largest first — the answer
// to "what one huge thing is eating my disk", which the tree makes you hunt for
// one directory at a time.
//
// It is not `--top`. That flag forces gdu's non-interactive mode, and cdu's
// non-interactive output is byte-for-byte gdu's — that is the promise the whole
// fork is built on, so the flag keeps its meaning and the screen gets a key.

const (
	// topFileCount is how many files are collected. More than a screenful, so the
	// list is worth scrolling; a fixed number rather than "what fits", because a
	// list whose *contents* changed when you resized the window would be a strange
	// thing to explain.
	topFileCount = 100

	// The path is split into its directory and its name, as the mock does: the
	// name is what you recognise a file by, so it never gives up its columns first.
	minTopNameWidth   = 12
	minTopDirWidth    = 8
	minWidthForTopDir = 44
)

// collectTopFiles walks the scanned tree for the largest files.
//
// It runs on the render loop, which is a deliberate exception to the rule that
// work belongs in a tea.Cmd. That rule is about I/O that can hang without bound
// — a stale network mount — and this is neither: the tree is already in memory,
// and the walk is measured at 3 ms for 12k items, 23 ms for 294k, 161 ms for
// 2.7M. A one-off hitch on a keypress, after a scan that took minutes.
//
// A goroutine would cost more than it saved. CollectTopFiles reads the tree
// through GetFiles, which takes no lock — the engine offers GetFilesLocked for
// exactly this reason — while the render loop is the one thread allowed to mutate
// that tree, in applyDelete. Off-loop, a delete landing mid-walk is a data race.
func (m *model) collectTopFiles() (tea.Model, tea.Cmd) {
	root := m.searchRoot()
	if root == nil {
		m.status, m.statusIsError = "nothing scanned yet", true
		return m, nil
	}

	m.topFiles = analyze.CollectTopFiles(root, topFileCount)
	m.topCursor, m.topOffset = 0, 0
	m.status, m.statusIsError = "", false
	m.scr = screenTop
	return m, nil
}

func (m *model) selectedTop() fs.Item {
	if m.topCursor < 0 || m.topCursor >= len(m.topFiles) {
		return nil
	}
	return m.topFiles[m.topCursor]
}

func (m *model) handleTopKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape, keyLeft, "h":
		m.scr = screenBrowse
		return m, nil
	case keyUp, "k":
		m.moveTopCursor(-1)
	case keyDown, "j":
		m.moveTopCursor(1)
	case keyHome, "g":
		m.moveTopCursor(-len(m.topFiles))
	case keyEnd, "G":
		m.moveTopCursor(len(m.topFiles))
	case keyPgUp:
		m.moveTopCursor(-m.visibleLines())
	case keyPgDown:
		m.moveTopCursor(m.visibleLines())
	case keyEnter, keyRight, "l":
		return m.revealTopFile()
	case "v":
		return m.openViewer()
	case "o":
		return m.openFile()
	case " ":
		m.markUnderCursor()
		m.moveTopCursor(1)
	case "u":
		m.unmarkAll()
	case "M":
		return m.openQueue()
	case "d":
		m.askConfirm(actionTrash)
	case "D":
		m.askConfirm(actionDelete)
	case "e":
		m.askConfirm(actionEmpty)
	}
	return m, nil
}

func (m *model) moveTopCursor(delta int) {
	if len(m.topFiles) == 0 {
		return
	}
	m.topCursor = min(max(m.topCursor+delta, 0), len(m.topFiles)-1)

	height := max(m.visibleLines(), 1)
	m.topOffset = min(m.topOffset, m.topCursor)
	if m.topCursor >= m.topOffset+height {
		m.topOffset = m.topCursor - height + 1
	}
	m.topOffset = min(max(m.topOffset, 0), max(len(m.topFiles)-height, 0))
}

// revealTopFile opens the file's directory in the browser, with the cursor on
// it. The list says a file is enormous; this is what says where it lives and
// what is around it.
func (m *model) revealTopFile() (tea.Model, tea.Cmd) {
	item := m.selectedTop()
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

// dropTopFile takes a deleted file out of the list.
//
// Recollecting would be the obvious thing and is the wrong one: it is the whole
// walk again — 161 ms on a large tree — to learn that one row is gone. The list
// is a snapshot of the biggest hundred, and one of them going is not news the
// other ninety-nine need.
func (m *model) dropTopFile(item fs.Item) {
	m.topFiles = removeItem(m.topFiles, item)
	m.topCursor = min(m.topCursor, max(len(m.topFiles)-1, 0))
	m.moveTopCursor(0)
}

func (m *model) viewTopList() string {
	lines := m.visibleLines()
	if len(m.topFiles) == 0 {
		return padLines(m.st.dim.Render(clipTo("  no files in this scan", m.width)), lines)
	}

	end := min(m.topOffset+lines, len(m.topFiles))
	rows := make([]string, 0, lines)
	for i := m.topOffset; i < end; i++ {
		rows = append(rows, m.viewTopRow(m.topFiles[i], i == m.topCursor))
	}
	return padLines(joinLines(rows), lines)
}

// viewTopRow is size, then the directory, then the name — the mock's own order,
// and the useful one: the size is why the row is here, and the name is what you
// recognise it by. The directory in between is context, and is the first thing
// to give up its columns.
func (m *model) viewTopRow(item fs.Item, selected bool) string {
	if m.width < 1 {
		return ""
	}

	icon := m.rowIcon(item)
	sizeText := cellRight(m.ui.formatSize(m.itemSize(item)), sizeColWidth)

	name := item.GetName()
	dir := filepath.Dir(item.GetPath())
	if dir != "/" {
		dir += "/"
	}

	// gutter + icon + size + gap + [dir + gap] + name
	rest := m.width - 1 - runewidth.StringWidth(icon) - sizeColWidth - 1
	dirWidth := 0
	if m.width >= minWidthForTopDir {
		// The name takes what it needs and no more; the directory gets the remainder,
		// down to a floor. A name cut to make room for the path of a file you can no
		// longer identify is a bad trade.
		nameWant := min(runewidth.StringWidth(name), max(rest-minTopDirWidth-1, minTopNameWidth))
		dirWidth = max(rest-nameWant-1, 0)
		if dirWidth < minTopDirWidth {
			dirWidth = 0
		}
	}

	nameWidth := rest
	dirText := ""
	if dirWidth > 0 {
		nameWidth = rest - dirWidth - 1
		dirText = cellPath(dir, dirWidth) + " "
	}
	nameText := cell(name, max(nameWidth, 0))

	plain := icon + sizeText + " " + dirText + nameText
	if selected {
		bar := m.st.accent.Render("▌")
		if m.width < 2 {
			return bar
		}
		if m.markOverlay(item) {
			return bar + m.markableGlyph(item, icon, &m.st.selected) +
				m.st.selected.Render(sizeText+" "+dirText) +
				m.renderMarkedName(nameText, &m.st.selected)
		}
		return bar + m.st.selected.Render(clipTo(plain, m.width-1))
	}
	if 1+runewidth.StringWidth(plain) > m.width {
		return m.st.fileName.Render(clipTo(" "+plain, m.width))
	}

	nameRender := m.st.fileName.Render(nameText)
	if m.markOverlay(item) {
		nameRender = m.renderMarkedName(nameText, &m.st.fileName)
	}
	return " " + m.markableGlyph(item, icon, &m.st.dim) +
		m.st.size.Render(sizeText) + " " +
		m.st.dim.Render(dirText) +
		nameRender
}

func (m *model) viewTop() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewTopList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
