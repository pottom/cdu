package charm

import (
	"github.com/pottom/cdu/pkg/fs"
)

// Sorting is two keys: s, then the field. gdu binds a key per field directly —
// s, C, n, M — which is one keystroke faster but costs four lines of footer, and
// a hint nobody can read is a key nobody has. Asking for the field second means
// the footer only has to explain the choice while it is being made.
//
// It also unpicks a collision gdu lives with: there, c toggles the item-count
// column and C sorts by it. Here the same letter means the same thing in each
// mode — c is the count, once as a column and once as a field.
var sortFieldKeys = map[string]fs.SortBy{
	"s": fs.SortBySize,
	"n": fs.SortByName,
	"c": fs.SortByItemCount,
	"m": fs.SortByMtime,
}

// naturalOrder is the direction a field is worth reading in the first time you
// ask for it: biggest, most, and newest first, but names from A.
//
// This is a deliberate break from gdu, which resets to ascending for every field.
// In a disk usage tool "sort by size, smallest first" is never what the keypress
// meant, and it costs a second press to undo. Pressing the key again still flips,
// so nothing is unreachable.
func naturalOrder(by fs.SortBy) fs.SortOrder {
	if by == fs.SortByName {
		return fs.SortAsc
	}
	return fs.SortDesc
}

// sortMenuKeys is the static sortMenuKeys with d relabelled to what pressing it
// would do — the one key in the menu that toggles a modifier rather than picking a
// field, so its label has to track state the way handleToggle's status line does.
func (m *model) sortMenuKeys() []keyHint {
	keys := make([]keyHint, len(sortMenuKeys))
	copy(keys, sortMenuKeys)
	for i := range keys {
		if keys[i].key == "d" {
			keys[i].label = onOff(m.ui.foldersFirst, "biggest first", "dirs first")
		}
	}
	return keys
}

// handleSortKey is the second half of the two-key sort. Anything that is not a
// field simply leaves sort mode: a stray keypress must not silently reorder the
// list, and it must not be swallowed either.
func (m *model) handleSortKey(key string) {
	m.sortPending = false

	// d is not a sort field but a modifier that sits on top of one: it floats
	// folders above files whatever the field is. It lives in this menu, not the t
	// menu, because it changes the order and not a column — the t menu toggles
	// columns.
	if key == "d" {
		m.ui.foldersFirst = !m.ui.foldersFirst
		m.reloadRows()
		m.status, m.statusIsError = "order: "+onOff(m.ui.foldersFirst, "folders first", "biggest first"), false
		return
	}

	by, ok := sortFieldKeys[key]
	if !ok {
		if key != keyEscape {
			m.status, m.statusIsError = "no such sort field: "+key, true
		}
		return
	}
	m.setSorting(by)
}

// setSorting applies a sort key. Sorting by size means apparent size when that is
// what the rows are showing — otherwise the list would be ordered by a number that
// is not on screen.
func (m *model) setSorting(by fs.SortBy) {
	if by == fs.SortBySize && m.ui.ShowApparentSize {
		by = fs.SortByApparentSize
	}

	if by == m.ui.sortBy {
		m.ui.sortOrder = flipOrder(m.ui.sortOrder)
	} else {
		m.ui.sortBy = by
		m.ui.sortOrder = naturalOrder(by)
	}

	m.reloadRows()
	m.status, m.statusIsError = "", false
}

func flipOrder(order fs.SortOrder) fs.SortOrder {
	if order == fs.SortAsc {
		return fs.SortDesc
	}
	return fs.SortAsc
}

// SetDefaultSorting takes the `sorting:` block from the config. It is the same
// signature gdu's tui uses, so app.go configures both interfaces the same way.
func (ui *UI) SetDefaultSorting(by, order string) {
	if by != "" {
		ui.sortBy = fs.ParseSortBy(by)
	}
	switch order {
	case yamlAsc:
		ui.sortOrder = fs.SortAsc
	case yamlDesc:
		ui.sortOrder = fs.SortDesc
	}
}
