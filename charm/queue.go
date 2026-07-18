package charm

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/pkg/fs"
)

// `M` opens the delete queue: everything marked, on one screen, so a batch delete
// can be read over before it happens rather than fired from a count in the header.
// It is the largest-files screen's twin — same rows, same movement — because it
// answers the same kind of question, "what exactly is in this set", and there is
// no reason for the two to feel different.
//
// The queue is a snapshot taken when the screen opens, like the largest-files
// list. Space prunes it — an item you did not mean to mark comes straight back
// out — and the destructive keys take the whole set to the confirm.

// openQueue snapshots the marked set, biggest first, and shows it.
func (m *model) openQueue() (tea.Model, tea.Cmd) {
	if m.markedCount() == 0 {
		m.status, m.statusIsError = "nothing marked — space marks a row for deletion", true
		return m, nil
	}
	m.queue = m.effectiveMarks(actionDelete)
	m.queueCursor, m.queueOffset = 0, 0
	m.status, m.statusIsError = "", false
	m.scr = screenQueue
	return m, nil
}

func (m *model) selectedQueue() fs.Item {
	if m.queueCursor < 0 || m.queueCursor >= len(m.queue) {
		return nil
	}
	return m.queue[m.queueCursor]
}

func (m *model) handleQueueKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape, keyLeft, "h", "M":
		m.scr = screenBrowse
		return m, nil
	case keyUp, "k":
		m.moveQueueCursor(-1)
	case keyDown, "j":
		m.moveQueueCursor(1)
	case keyHome, "g":
		m.moveQueueCursor(-len(m.queue))
	case keyEnd, "G":
		m.moveQueueCursor(len(m.queue))
	case keyPgUp:
		m.moveQueueCursor(-m.visibleLines())
	case keyPgDown:
		m.moveQueueCursor(m.visibleLines())
	case " ":
		m.unmarkFromQueue()
	case keyEnter, keyRight, "l":
		return m.revealQueueItem()
	case "v":
		return m.openViewer()
	case "o":
		return m.openFile()
	case "d":
		m.askConfirm(actionTrash)
	case "D":
		m.askConfirm(actionDelete)
	case "e":
		m.askConfirm(actionEmpty)
	}
	return m, nil
}

func (m *model) moveQueueCursor(delta int) {
	if len(m.queue) == 0 {
		return
	}
	m.queueCursor = min(max(m.queueCursor+delta, 0), len(m.queue)-1)

	height := max(m.visibleLines(), 1)
	m.queueOffset = min(m.queueOffset, m.queueCursor)
	if m.queueCursor >= m.queueOffset+height {
		m.queueOffset = m.queueCursor - height + 1
	}
	m.queueOffset = min(max(m.queueOffset, 0), max(len(m.queue)-height, 0))
}

// unmarkFromQueue takes the cursor row back out of the set, in both the live marks
// and this snapshot, so pruning is immediate. Empty the queue this way and the
// screen says so rather than sitting blank.
func (m *model) unmarkFromQueue() {
	item := m.selectedQueue()
	if item == nil {
		return
	}
	delete(m.marked, item)
	m.queue = removeItem(m.queue, item)
	m.queueCursor = min(m.queueCursor, max(len(m.queue)-1, 0))
	m.moveQueueCursor(0)
}

// revealQueueItem opens the item's directory in the browser with the cursor on it —
// the same "where does this live" the largest-files list answers.
func (m *model) revealQueueItem() (tea.Model, tea.Cmd) {
	item := m.selectedQueue()
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

func (m *model) viewQueueList() string {
	lines := m.visibleLines()
	if len(m.queue) == 0 {
		return padLines(m.st.dim.Render(clipTo("  the queue is empty — space marks rows to delete", m.width)), lines)
	}

	end := min(m.queueOffset+lines, len(m.queue))
	rows := make([]string, 0, lines)
	for i := m.queueOffset; i < end; i++ {
		// The rows are the largest-files rows exactly: size, directory, name. Every
		// item here is marked, so no row needs the tick the browser draws.
		rows = append(rows, m.viewTopRow(m.queue[i], i == m.queueCursor))
	}
	return padLines(joinLines(rows), lines)
}

func (m *model) viewQueue() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewQueueList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
