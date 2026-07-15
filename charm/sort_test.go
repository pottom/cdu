package charm

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/fs"
)

// A field is first shown in the direction the keypress actually meant: biggest,
// most and newest first, names from A. gdu resets everything to ascending, which
// in a disk usage tool means "sort by size" gives you the smallest file — never
// what was wanted, and a second press to undo.
func TestSortKeyPicksTheUsefulDirectionFirst(t *testing.T) {
	for field, want := range map[string]struct {
		by    fs.SortBy
		order fs.SortOrder
	}{
		"s": {fs.SortBySize, fs.SortDesc},
		"c": {fs.SortByItemCount, fs.SortDesc},
		"m": {fs.SortByMtime, fs.SortDesc},
		"n": {fs.SortByName, fs.SortAsc},
	} {
		m := benchModel(5)
		m.ui.sortBy, m.ui.sortOrder = fs.SortByMtime, fs.SortAsc // something else entirely

		m = press(t, m, "s", field)
		assert.False(t, m.sortPending, "the mode must end once the field is chosen")
		assert.Equal(t, want.by, m.ui.sortBy, "field %q", field)
		assert.Equal(t, want.order, m.ui.sortOrder, "field %q", field)
	}
}

// The same field again flips the order, so nothing is unreachable.
func TestTheSameSortFieldFlipsTheOrder(t *testing.T) {
	m := benchModel(5)

	m = press(t, m, "s", "n")
	require.Equal(t, fs.SortAsc, m.ui.sortOrder)

	m = press(t, m, "s", "n")
	assert.Equal(t, fs.SortDesc, m.ui.sortOrder)

	m = press(t, m, "s", "n")
	assert.Equal(t, fs.SortAsc, m.ui.sortOrder, "it flips back")
}

// Sort mode takes the next key whatever it is — including q. A mode that let some
// keys through would be a mode you could not trust, and q would quit the program
// while the user thought they were choosing a field.
func TestSortModeSwallowsEveryKey(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "s")
	require.True(t, m.sortPending)

	_, cmd := m.Update(key("q"))
	assert.Nil(t, cmd, "q must not quit while a sort field is being chosen")

	m = press(t, m, "s", "z")
	assert.False(t, m.sortPending, "an unknown key leaves the mode rather than sticking in it")
	assert.True(t, m.statusIsError, "and it says so rather than being swallowed silently")
}

func TestEscapeLeavesSortModeQuietly(t *testing.T) {
	m := benchModel(5)
	before := m.ui.sortBy

	m = press(t, m, "s", "esc")
	assert.False(t, m.sortPending)
	assert.Equal(t, before, m.ui.sortBy, "escape must change nothing")
	assert.Empty(t, m.status, "escape is not an error")
}

// Sorting by size must mean the size the rows are actually showing. Ordering the
// list by a number that is not on screen looks like a bug.
func TestSortBySizeFollowsApparentSize(t *testing.T) {
	m := benchModel(5)
	m.ui.ShowApparentSize = true

	m = press(t, m, "s", "s")
	assert.Equal(t, fs.SortByApparentSize, m.ui.sortBy)
}

// Re-sorting moves every row. The cursor must follow the item it was on, not stay
// at index 4 pointing at something else — the next key might be D.
func TestSortingKeepsTheCursorOnTheSameItem(t *testing.T) {
	m := benchModel(6)
	m.ui.sortBy, m.ui.sortOrder = fs.SortBySize, fs.SortDesc
	m.reloadRows()

	m.cursor = 1
	selected := m.selected()
	require.NotNil(t, selected)

	m = press(t, m, "s", "s") // flips to ascending: the list reverses
	require.NotEqual(t, 1, m.cursor, "the row number must have moved")
	assert.Equal(t, selected, m.selected(), "the selection must still be the same item")
}

// A mode nobody can see is a trap. While the field is being chosen the footer must
// name the mode and offer the fields, and it must shed hints whole rather than
// truncating one in half.
func TestTheFooterShowsTheSortMenu(t *testing.T) {
	m := benchModel(3)
	m.height, m.haveSize = 24, true
	m.scr = screenBrowse

	m.width = 80
	assert.Contains(t, m.viewFooter(), "sort", "s must be discoverable while browsing")

	m = press(t, m, "s")
	menu := m.viewFooter()
	for _, field := range []string{"size", "name", "count", "mtime", "cancel"} {
		assert.Contains(t, menu, field, "the menu must offer %s", field)
	}
	assert.Contains(t, menu, "sort by", "the mode must be named, not merely implied")

	// Whatever survives a narrow terminal, it never overflows, and movement and the
	// way out always do.
	m.sortPending = false
	for _, width := range []int{200, 120, 100, 80, 64, 50, 36, 20, 1} {
		m.width = width
		footer := m.viewFooter()
		assert.LessOrEqual(t, lipgloss.Width(footer), width, "footer overflows at %d", width)
		if width >= 36 {
			assert.Contains(t, footer, "quit", "the way out must survive at %d", width)
			assert.Contains(t, footer, "move", "movement must survive at %d", width)
		}
	}
}

func TestDefaultSortingComesFromTheConfig(t *testing.T) {
	ui := CreateUI(nil, true, false, false, false)
	ui.SetDefaultSorting("name", "desc")

	assert.Equal(t, fs.SortByName, ui.sortBy)
	assert.Equal(t, fs.SortDesc, ui.sortOrder)

	// An empty or unrecognised value must leave the default alone rather than
	// silently sorting by something nobody asked for.
	ui.SetDefaultSorting("", "sideways")
	assert.Equal(t, fs.SortByName, ui.sortBy)
	assert.Equal(t, fs.SortDesc, ui.sortOrder)
}
