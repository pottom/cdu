package charm

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Mouse support is behind --mouse, and stays deliberately small: the wheel
// scrolls, and a left click selects. There is no drag, no hover — those cost the
// user their terminal's own text selection, which is too much to take for too
// little.

func (m *model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// The viewer scrolls; nothing there is clickable.
	if m.scr == screenViewer {
		switch msg.Button { //nolint:exhaustive // only the wheel scrolls the viewer
		case tea.MouseButtonWheelUp:
			m.viewer.offset--
			m.clampViewer()
		case tea.MouseButtonWheelDown:
			m.viewer.offset++
			m.clampViewer()
		}
		return m, nil
	}

	// A menu, a modal or the filter input owns the keyboard; the mouse does not
	// reach around them to move a selection the user cannot currently see act on.
	if m.scr != screenBrowse || m.sortPending || m.colPending || m.filtering {
		return m, nil
	}

	switch msg.Button { //nolint:exhaustive // only the wheel and the left button are bound
	case tea.MouseButtonWheelUp:
		m.moveCursor(-1)
	case tea.MouseButtonWheelDown:
		m.moveCursor(1)
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			return m.handleClick(msg.Y)
		}
	}
	return m, nil
}

// handleClick selects the row under the pointer. Clicking the row that is already
// selected opens it — this is what stands in for a double-click, which the
// terminal does not report as such: a first click selects, a second on the same
// row descends.
func (m *model) handleClick(y int) (tea.Model, tea.Cmd) {
	idx, ok := m.rowAt(y)
	if !ok {
		return m, nil
	}
	if idx == m.cursor {
		m.descend()
		return m, nil
	}
	m.cursor = idx
	m.clampCursor()
	m.status, m.statusIsError = "", false
	return m, nil
}

// rowAt maps a screen row to an entry index, or reports that the click landed on
// the header, the footer, or empty space below the list.
func (m *model) rowAt(y int) (int, bool) {
	top := m.headerHeight()
	if y < top {
		return 0, false
	}
	idx := m.offset + (y-top)/m.linesPerEntry()
	if idx < 0 || idx >= len(m.items()) {
		return 0, false
	}
	return idx, true
}
