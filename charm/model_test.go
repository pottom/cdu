package charm

import (
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/testdir"
	"github.com/pottom/cdu/pkg/analyze"
)

// scannedModel walks the shared test tree and returns a model sitting in the
// browse screen, as if the scan command had just completed.
func scannedModel(t *testing.T) *model {
	t.Helper()

	ui := CreateUI(io.Discard, true, false, false, false)
	dir := analyze.CreateAnalyzer().AnalyzeDir("test_dir", func(_, _ string) bool { return false }, nil)
	require.NotNil(t, dir)
	dir.UpdateStats(ui.linkedItems)

	m := newModel(ui)
	next, _ := m.Update(scanDoneMsg{dir: dir})
	m = next.(*model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return next.(*model)
}

func TestScanPopulatesBrowseScreen(t *testing.T) {
	defer testdir.CreateTestDir()()

	m := scannedModel(t)

	assert.Equal(t, screenBrowse, m.scr)
	assert.Equal(t, "test_dir", m.currentDir.GetName())
	assert.NotEmpty(t, m.rows)

	view := m.View()
	assert.Contains(t, view, "cdu")
	assert.Contains(t, view, "nested")
}

func TestNavigateIntoAndBackOut(t *testing.T) {
	defer testdir.CreateTestDir()()

	m := scannedModel(t)
	root := m.currentDir

	// Descend into the first directory in the listing.
	for i, r := range m.rows {
		if r.IsDir() {
			m.cursor = i
			break
		}
	}
	target := m.selected()
	require.True(t, target.IsDir())

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(*model)
	assert.Equal(t, target.GetName(), m.currentDir.GetName())

	// Going back should land on the directory we came out of, not at the top.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(*model)
	assert.Equal(t, root.GetName(), m.currentDir.GetName())
	assert.Equal(t, target, m.selected())
}

func TestEnterOnFileDoesNothing(t *testing.T) {
	defer testdir.CreateTestDir()()

	m := scannedModel(t)

	// Descend until we reach a directory that actually holds a file.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(*model)

	found := false
	for i, r := range m.rows {
		if !r.IsDir() {
			m.cursor = i
			found = true
			break
		}
	}
	require.True(t, found, "expected a file inside %s", m.currentDir.GetName())

	before := m.currentDir
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, before, next.(*model).currentDir)
}

// TestNeverPanicsAtAnySize is the enforcement of the "never panic on tiny
// terminals" rule: degenerate sizes must clamp to a minimal layout, and the
// view must never be wider or taller than it was told it could be.
func TestNeverPanicsAtAnySize(t *testing.T) {
	defer testdir.CreateTestDir()()

	sizes := []tea.WindowSizeMsg{
		{Width: 0, Height: 0},
		{Width: 1, Height: 1},
		{Width: 3, Height: 2},
		{Width: 20, Height: 3},
		{Width: 40, Height: 5},
		{Width: 80, Height: 24},
		{Width: 300, Height: 100},
	}

	for _, size := range sizes {
		m := scannedModel(t)
		next, _ := m.Update(size)
		m = next.(*model)

		assert.NotPanics(t, func() { _ = m.View() }, "size %dx%d", size.Width, size.Height)
		assert.GreaterOrEqual(t, m.visibleRows(), 1, "size %dx%d", size.Width, size.Height)

		// Navigating at a degenerate size must be safe too.
		assert.NotPanics(t, func() {
			for range 5 {
				n, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = n.(*model)
				_ = m.View()
			}
		}, "size %dx%d", size.Width, size.Height)
	}
}

// TestListIsWindowed proves the render is virtualized: a directory with far more
// entries than fit on screen must still only produce a screenful of rows.
func TestListIsWindowed(t *testing.T) {
	defer testdir.CreateTestDir()()

	m := scannedModel(t)

	// Stand in a wide, short terminal and fake a large directory listing by
	// reusing the rows we have.
	for len(m.rows) < 5000 {
		m.rows = append(m.rows, m.rows...)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = next.(*model)

	lines := strings.Count(m.View(), "\n")
	assert.Less(t, lines, 30, "view rendered %d lines for %d rows — not windowed", lines, len(m.rows))
}

func TestCursorFollowsWindow(t *testing.T) {
	defer testdir.CreateTestDir()()

	m := scannedModel(t)
	for len(m.rows) < 200 {
		m.rows = append(m.rows, m.rows...)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = next.(*model)

	for range 100 {
		n, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = n.(*model)
	}

	assert.GreaterOrEqual(t, m.cursor, m.offset)
	assert.Less(t, m.cursor, m.offset+m.visibleRows())
	assert.Less(t, m.cursor, len(m.rows))
}

func TestQuitDuringScan(t *testing.T) {
	ui := CreateUI(io.Discard, true, false, false, false)
	m := newModel(ui)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	assert.NotNil(t, cmd, "q must quit even while the scan is still running")
	assert.Equal(t, screenScanning, next.(*model).scr)
}

func TestViewBeforeWindowSizeIsEmpty(t *testing.T) {
	ui := CreateUI(io.Discard, true, false, false, false)
	m := newModel(ui)

	// Bubble Tea has not told us the size yet; there is nothing honest to draw.
	assert.Empty(t, m.View())
}
