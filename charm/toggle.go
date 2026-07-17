package charm

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/pkg/fs"
)

// toggleKeys are gdu's: a apparent size, B relative size, c item count, m mtime.
// They work directly, as in gdu, and they are also what the t menu offers — the
// menu exists so that they can be discovered, not as a second way to reach them.
var toggleKeys = map[string]bool{"a": true, "B": true, "c": true, "m": true}

// handleColumnKey is the second half of the t menu. As with sorting, an unknown
// key leaves the mode and says so rather than being swallowed.
func (m *model) handleColumnKey(key string) tea.Cmd {
	m.colPending = false

	// s saves the view. It lives in this menu because these are the settings it
	// writes; at the top level s is already sort, and the two would collide.
	if key == "s" {
		return m.saveView()
	}

	if !toggleKeys[key] {
		if key != keyEscape {
			m.status, m.statusIsError = "no such column: "+key, true
		}
		return nil
	}
	m.handleToggle(key)
	return nil
}

func (m *model) handleToggle(key string) {
	switch key {
	case "a":
		m.ui.ShowApparentSize = !m.ui.ShowApparentSize
		// Sorting by size has to keep meaning the size on screen. Without this,
		// toggling the column would leave the list ordered by the number that is no
		// longer shown, which reads as a sorting bug rather than a display choice.
		switch m.ui.sortBy {
		case fs.SortBySize:
			if m.ui.ShowApparentSize {
				m.ui.sortBy = fs.SortByApparentSize
			}
		case fs.SortByApparentSize:
			if !m.ui.ShowApparentSize {
				m.ui.sortBy = fs.SortBySize
			}
		case fs.SortByName, fs.SortByItemCount, fs.SortByMtime:
		}

	case "B":
		m.ui.ShowRelativeSize = !m.ui.ShowRelativeSize

	case "c":
		m.ui.showItemCount = !m.ui.showItemCount

	case "m":
		m.ui.showMtime = !m.ui.showMtime
	}

	m.reloadRows()
	m.status, m.statusIsError = m.toggleLabel(key)
}

// toggleLabel says what changed, and — this is the point — says when the column
// was turned on but there is no room to draw it. Otherwise the key would look
// broken on a narrow terminal, which is exactly where someone is most likely to
// reach for a column and least likely to get one.
func (m *model) toggleLabel(key string) (status string, isError bool) {
	switch key {
	case "a":
		return "sizes: " + onOff(m.ui.ShowApparentSize, "apparent", "disk usage"), false
	case "B":
		return "bars: " + onOff(m.ui.ShowRelativeSize, "relative to the largest item", "share of the directory"), false
	case "c":
		if m.ui.showItemCount && m.width < minWidthForItemCount {
			return "item count is on, but the terminal is too narrow to show it", true
		}
		return "item count " + onOff(m.ui.showItemCount, "on", "off"), false
	case "m":
		if m.ui.showMtime && m.width < minWidthForMtime {
			return "mtime is on, but the terminal is too narrow to show it", true
		}
		return "mtime " + onOff(m.ui.showMtime, "on", "off"), false
	}
	return "", false
}

func onOff(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

// rowScale is what a row's bar and percentage are measured against.
//
// By default that is the directory's own total, so the column reads as "share of
// this directory". With --show-relative-size it is the largest item instead, so
// the biggest row fills the bar and the rest are read against it — which is the
// more useful comparison when one item dwarfs everything else.
//
// It is computed once per directory rather than per frame: doing it in View would
// walk every row on every render and undo the virtualization.
func (m *model) rowScale() int64 {
	if m.currentDir == nil {
		return 0
	}
	if m.ui.ShowRelativeSize {
		return m.maxRowSize
	}
	return m.itemSize(m.currentDir)
}
