package charm

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wheel(dir tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{Button: dir, Action: tea.MouseActionPress}
}

func clickAt(y int) tea.MouseMsg {
	return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: y}
}

func browseModel(t *testing.T, n int) *model {
	t.Helper()
	m := benchModel(n)
	m.width, m.height, m.haveSize = 100, 24, true
	m.scr = screenBrowse
	m.dev = nil // keep the header two lines high, so the row math is easy to reason about
	return m
}

func TestWheelMovesTheCursor(t *testing.T) {
	m := browseModel(t, 20)
	require.Equal(t, 0, m.cursor)

	next, _ := m.Update(wheel(tea.MouseButtonWheelDown))
	m = next.(*model)
	assert.Equal(t, 1, m.cursor, "wheel down moves down one")

	next, _ = m.Update(wheel(tea.MouseButtonWheelUp))
	m = next.(*model)
	assert.Equal(t, 0, m.cursor, "wheel up moves back")

	// Above the top, it clamps rather than going negative.
	next, _ = m.Update(wheel(tea.MouseButtonWheelUp))
	m = next.(*model)
	assert.Equal(t, 0, m.cursor)
}

// A first click selects; a second click on the same row opens it. That is what
// stands in for a double-click, which the terminal does not report as one.
func TestClickSelectsThenOpens(t *testing.T) {
	m := browseModel(t, 20)

	// The list starts below a two-line header. Row 0 sits at y == headerHeight().
	top := m.headerHeight()
	clickRow := 3

	next, _ := m.Update(clickAt(top + clickRow*m.linesPerEntry()))
	m = next.(*model)
	assert.Equal(t, clickRow, m.cursor, "the clicked row becomes selected")

	// Second click on the same row: it opens. The synthetic rows are files, so
	// descend is a no-op, but the cursor must not move — the click was consumed.
	before := m.currentDir
	next, _ = m.Update(clickAt(top + clickRow*m.linesPerEntry()))
	m = next.(*model)
	assert.Equal(t, before, m.currentDir, "opening a file changes nothing, but does not move the cursor")
	assert.Equal(t, clickRow, m.cursor)
}

// A click on the header selects nothing: the row math must not map chrome onto an
// entry.
func TestClickOnHeaderIsIgnored(t *testing.T) {
	m := browseModel(t, 20)
	m.cursor = 5

	next, _ := m.Update(clickAt(0))
	m = next.(*model)
	assert.Equal(t, 5, m.cursor, "clicking the header must not move the selection")
}

// A click below the last row hits empty space, not the last row.
func TestClickBelowTheListIsIgnored(t *testing.T) {
	m := browseModel(t, 3)
	m.cursor = 1

	next, _ := m.Update(clickAt(m.height - 1)) // the footer line
	m = next.(*model)
	assert.Equal(t, 1, m.cursor)
}

// The mouse does not reach around a modal or a menu to move a hidden selection.
func TestMouseIsInertWhileAModeIsOpen(t *testing.T) {
	m := browseModel(t, 20)
	m.sortPending = true

	next, _ := m.Update(wheel(tea.MouseButtonWheelDown))
	m = next.(*model)
	assert.Equal(t, 0, m.cursor, "the wheel must not scroll behind the sort menu")
}

// In the viewer the wheel scrolls the file rather than a selection.
func TestWheelScrollsTheViewer(t *testing.T) {
	m := browseModel(t, 0)
	m.scr = screenViewer
	m.viewer = &viewerState{path: "/x", lines: make([]string, 100)}

	next, _ := m.Update(wheel(tea.MouseButtonWheelDown))
	m = next.(*model)
	assert.Equal(t, 1, m.viewer.offset)
}
