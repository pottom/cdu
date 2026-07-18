package charm

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/pkg/fs"
)

// The filter is a fuzzy search over the current directory, opened with /. It is a
// view, never a change to the tree: applyFilter only ever rebuilds m.filtered, so
// a delete under a filter still finds and removes the real item.

// openFilter starts the / input. An empty query matches everything, so opening it
// changes nothing on screen until the user types — the list does not blink away.
func (m *model) openFilter() {
	m.filtering = true
	m.filter = ""
	m.applyFilter()
	m.status, m.statusIsError = "", false
}

// closeFilter ends the input and shows the whole directory again.
func (m *model) closeFilter() {
	m.filtering = false
	m.filter = ""
	m.filtered = nil
	m.clampCursor()
}

// handleFilterKey drives the / input. It takes every key, as the sort and column
// menus do: while typing a filter, q is a letter, not a way out.
func (m *model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape:
		m.closeFilter()
		return m, nil

	case keyEnter:
		// Enter accepts the filter and leaves input mode, but keeps the matches on
		// screen so they can be navigated and acted on. Escape is what clears it.
		m.filtering = false
		if m.filter == "" {
			m.filtered = nil
		}
		return m, nil

	case keyBackspace:
		if m.filter != "" {
			m.filter = m.filter[:len(m.filter)-1]
			m.applyFilter()
		} else {
			// Backspace on an empty query closes the filter, so the key that opened
			// nothing also closes it.
			m.closeFilter()
		}
		return m, nil
	}

	if len(msg.Runes) == 1 {
		m.filter += string(msg.Runes)
		m.applyFilter()
	}
	return m, nil
}

// applyFilter rebuilds the filtered view from the current query. The selection
// jumps to the top: after narrowing, the best-fitting match a user is looking for
// is far more often the first one than whatever index the cursor happened to hold.
func (m *model) applyFilter() {
	if !m.filtering && m.filter == "" {
		m.filtered = nil
		return
	}

	out := make([]fs.Item, 0, len(m.rows))
	for _, item := range m.rows {
		// The ../ row is a way out, not a child to search — and it would match on the
		// parent's real name, not on "..", so filtering hides it rather than letting
		// it flicker in and out on an unrelated word.
		if m.isParentRow(item) {
			continue
		}
		if ok, _ := fuzzyMatch(item.GetName(), m.filter); ok {
			out = append(out, item)
		}
	}
	m.filtered = out
	m.cursor, m.offset = 0, 0
}
