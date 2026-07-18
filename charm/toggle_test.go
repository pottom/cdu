package charm

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

func TestTogglesFlipTheirColumns(t *testing.T) {
	m := benchModel(5)
	m.width, m.height, m.haveSize = 120, 24, true

	m = press(t, m, "a")
	assert.True(t, m.ui.ShowApparentSize)
	m = press(t, m, "a")
	assert.False(t, m.ui.ShowApparentSize)

	m = press(t, m, "B")
	assert.True(t, m.ui.ShowRelativeSize)

	m = press(t, m, "c")
	assert.True(t, m.ui.showItemCount)

	m = press(t, m, "m")
	assert.True(t, m.ui.showMtime)
}

// The t menu exists so the toggles can be found, not as a second way to reach
// them: the direct keys still work, as they do in gdu.
func TestTheColumnMenuIsDiscoverableAndSwallowsEveryKey(t *testing.T) {
	m := benchModel(5)
	m.width, m.height, m.haveSize = 120, 24, true
	m.scr = screenBrowse

	// The footer no longer lists every key — it shows the essentials and ?, which
	// opens the screen that has every key on it. So the t menu is discovered
	// through help, not the footer.
	assert.Contains(t, m.viewFooter(), "help", "? must be in the footer, as the way to everything else")
	assert.Contains(t, allHelpText(), "columns", "and the t menu must be documented in the help")

	m = press(t, m, "t")
	require.True(t, m.colPending)

	menu := m.viewFooter()
	for _, label := range []string{"apparent", "relative", "count", "mtime", "cancel"} {
		assert.Contains(t, menu, label)
	}
	assert.Contains(t, menu, "toggle column", "the mode must be named")

	_, cmd := m.Update(key("q"))
	assert.Nil(t, cmd, "q must not quit while a column is being chosen")

	m = press(t, m, "t", "a")
	assert.False(t, m.colPending)
	assert.True(t, m.ui.ShowApparentSize, "the menu applies the same toggle the direct key does")
}

// Toggling the apparent-size column must carry the sort with it. Otherwise the
// list stays ordered by a number that is no longer on screen, which reads as a
// sorting bug rather than a display choice.
func TestApparentSizeToggleCarriesTheSort(t *testing.T) {
	m := benchModel(5)
	require.Equal(t, fs.SortBySize, m.ui.sortBy)

	m = press(t, m, "a")
	assert.Equal(t, fs.SortByApparentSize, m.ui.sortBy)

	m = press(t, m, "a")
	assert.Equal(t, fs.SortBySize, m.ui.sortBy)
}

// Sorting by name has nothing to do with which size is displayed, so it must
// survive the toggle untouched.
func TestApparentSizeToggleLeavesOtherSortsAlone(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "s", "n")
	require.Equal(t, fs.SortByName, m.ui.sortBy)

	m = press(t, m, "a")
	assert.Equal(t, fs.SortByName, m.ui.sortBy)
}

// A column turned on but with no room to appear would look like a broken key —
// and a narrow terminal is exactly where someone reaches for a column and cannot
// have one. So it says so.
func TestAColumnWithNoRoomSaysSo(t *testing.T) {
	m := benchModel(5)
	m.width, m.height, m.haveSize = 60, 24, true

	m = press(t, m, "c")
	assert.True(t, m.ui.showItemCount, "the state still flips")
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "too narrow")

	assert.NotContains(t, m.viewRow(m.rows[0], false, 100), ",",
		"and the column is not drawn at this width")
}

// --show-relative-size measures every bar against the largest item rather than
// the directory total, so the biggest row fills its bar.
func TestRelativeSizeScalesToTheLargestItem(t *testing.T) {
	m := benchModel(5)
	m.width, m.height, m.haveSize = 120, 24, true

	total := m.rowScale()
	assert.Equal(t, m.itemSize(m.currentDir), total, "by default a row is a share of the directory")

	m = press(t, m, "B")
	assert.Equal(t, m.maxRowSize, m.rowScale(), "relative sizes are measured against the largest row")
	assert.Equal(t, 1.0, fraction(m.itemSize(m.rows[0]), m.rowScale()),
		"the largest row must fill its bar exactly")
}

// The largest row is found once per directory. Finding it in View would walk all
// ten thousand rows on every frame and undo the virtualization the whole design
// exists for.
func TestTheLargestRowIsMeasuredOncePerDirectory(t *testing.T) {
	m := benchModel(500)
	m.ui.ShowRelativeSize = true
	require.NotZero(t, m.maxRowSize)

	// Whatever the window shows, the scale is the same: it does not depend on what
	// happens to be rendered.
	m.offset, m.cursor = 400, 400
	assert.Equal(t, m.maxRowSize, m.rowScale())
}

// A directory counts itself. Reporting "1 item" for an empty directory is not
// what anyone means by the column.
func TestTheItemCountColumnExcludesTheDirectoryItself(t *testing.T) {
	withProfile(t, termenv.Ascii)

	m := benchModel(3)
	m.width, m.height, m.haveSize = 120, 24, true
	m.ui.showItemCount = true

	for _, row := range m.rows {
		if !row.IsDir() {
			assert.Equal(t, "        1 ", m.extraColumns(row), "a file is one item")
		}
	}
}

// Whatever the columns, the row still has to be exactly as wide as the terminal.
func TestRowWidthSurvivesEveryColumnCombination(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	for _, width := range []int{60, 80, 100, 140, 200} {
		for _, count := range []bool{false, true} {
			for _, mtime := range []bool{false, true} {
				m := benchModel(5)
				m.width, m.height, m.haveSize = width, 24, true
				m.ui.showItemCount, m.ui.showMtime = count, mtime

				for _, selected := range []bool{false, true} {
					row := m.viewRow(m.rows[0], selected, m.rowScale())
					assert.Equal(t, width, lipgloss.Width(row),
						"width=%d count=%v mtime=%v selected=%v", width, count, mtime, selected)
				}
			}
		}
	}
}

// Folders-first floats directories to the top, keeping the engine's order within
// each group — a stable partition, not a re-sort. Off, biggest-first mixes them.
func TestFoldersFirstToggle(t *testing.T) {
	m := benchModel(0)
	dir := m.currentDir.(*analyze.Dir)
	// A small folder and a big file: by size the file leads; folders-first floats
	// the folder above it.
	sub := &analyze.Dir{File: &analyze.File{Name: "asub", Parent: dir}}
	sub.AddFile(&analyze.File{Name: "x", Size: 10, Usage: 10, Parent: sub})
	dir.AddFile(sub)
	dir.AddFile(&analyze.File{Name: "big.bin", Size: 9000, Usage: 9000, Parent: dir})
	dir.UpdateStats(make(fs.HardLinkedItems))
	m.reloadRows()

	require.False(t, m.ui.foldersFirst)
	assert.Equal(t, "big.bin", m.rows[0].GetName(), "off: biggest first, the file leads")

	m.sortPending = true
	m.handleSortKey("d")
	require.True(t, m.ui.foldersFirst, "d in the sort menu toggles it")
	assert.True(t, m.rows[0].IsDir(), "on: the folder floats above the bigger file")
	assert.Equal(t, "big.bin", m.rows[1].GetName(), "and the file follows")

	// The toggle is saved with the rest of the view.
	assert.True(t, m.ui.viewSettings().FoldersFirst)
}
