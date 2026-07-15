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

func TestFuzzyMatchIsASubsequence(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		want        bool
	}{
		{"node_modules", "nmd", true},
		{"node_modules", "node", true},
		{"Downloads", "dwn", true},
		{"Downloads", "DWN", true},     // case-insensitive
		{"report.pdf", "rpdf", true},   // spans the dot
		{"node_modules", "xyz", false}, // absent
		{"node_modules", "sd", false},  // out of order: s is last, no d follows it
		{"anything", "", true},         // empty query matches everything
	} {
		got, _ := fuzzyMatch(tc.name, tc.query)
		assert.Equal(t, tc.want, got, "%q against %q", tc.query, tc.name)
	}
}

func TestFuzzyMatchReportsPositions(t *testing.T) {
	_, pos := fuzzyMatch("node_modules", "nmd")
	// n(0) … m(5) … d(7): the first available run in order.
	assert.Equal(t, []int{0, 5, 7}, pos)
}

// A dir named the same in different cases must not need the case typed exactly,
// and the positions must be rune offsets so a multi-byte name is not mis-split.
func TestFuzzyMatchHandlesMultibyte(t *testing.T) {
	ok, pos := fuzzyMatch("café-münchen", "cmn")
	require.True(t, ok)
	for _, p := range pos {
		assert.Less(t, p, len([]rune("café-münchen")))
	}
}

func filterModel(t *testing.T, names ...string) *model {
	t.Helper()
	m := benchModel(0)
	m.width, m.height, m.haveSize = 100, 24, true
	m.scr = screenBrowse

	dir := m.currentDir.(*analyze.Dir)
	for _, n := range names {
		dir.AddFile(&analyze.File{Name: n, Size: 1024, Usage: 4096, Parent: dir})
	}
	m.reloadRows()
	return m
}

func TestFilterNarrowsTheList(t *testing.T) {
	m := filterModel(t, "node_modules", "Downloads", "notes.txt", "src")

	m = press(t, m, "/")
	require.True(t, m.filtering)
	assert.Len(t, m.items(), 4, "an empty filter shows everything")

	m = press(t, m, "n", "o")
	// "no" is a subsequence of node_modules, Downloads (D-o... no, needs n then o:
	// downloads has no 'n'... actually "Downloads" has no n), notes.txt.
	for _, it := range m.items() {
		ok, _ := fuzzyMatch(it.GetName(), "no")
		assert.True(t, ok, "%s should not have passed the filter", it.GetName())
	}
	assert.NotEmpty(t, m.items())
}

// The filter is a view, never a change to the tree. A delete under a filter must
// find and remove the real item, and the row must leave both lists.
func TestDeleteUnderAFilterActsOnTheRealItem(t *testing.T) {
	m := filterModel(t, "keep-me", "delete-me", "keep-me-too")
	fullBefore := len(m.rows)

	m = press(t, m, "/", "d", "e", "l")
	require.Len(t, m.items(), 1, "only delete-me matches del")
	victim := m.items()[0]

	m.applyDelete(deleteDoneMsg{item: victim, parent: m.currentDir, act: actionDelete})

	assert.Len(t, m.rows, fullBefore-1, "the item must be gone from the full list")
	for _, it := range m.rows {
		assert.NotEqual(t, victim, it, "the deleted item must not remain anywhere")
	}
}

// Navigating clears the filter: carrying it into another directory would hide the
// new one for a reason the user could not see.
func TestNavigatingClearsTheFilter(t *testing.T) {
	m := benchModel(0)
	m.width, m.height, m.haveSize = 100, 24, true
	m.scr = screenBrowse

	root := m.currentDir.(*analyze.Dir)
	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: root}}
	sub.AddFile(&analyze.File{Name: "inside", Size: 512, Parent: sub})
	root.AddFile(sub)
	m.reloadRows()

	for i, r := range m.rows {
		if r == fs.Item(sub) {
			m.cursor = i
		}
	}
	m = press(t, m, "/", "s")
	require.True(t, m.filter != "")

	m = press(t, m, "enter") // accept, still filtered
	m.descend()
	assert.False(t, m.filtering)
	assert.Empty(t, m.filter, "the filter must not follow us into the subdirectory")
	assert.Nil(t, m.filtered)
}

// The / input takes every key, including q. Otherwise filtering for a file called
// "query" would quit halfway through typing it.
func TestFilterInputSwallowsEveryKey(t *testing.T) {
	m := filterModel(t, "query.txt")
	m = press(t, m, "/")

	_, cmd := m.Update(key("q"))
	assert.Nil(t, cmd, "q must not quit while a filter is being typed")
	assert.Equal(t, "q", m.filter)
}

func TestEscapeClearsTheFilter(t *testing.T) {
	m := filterModel(t, "a", "b", "c")
	m = press(t, m, "/", "a")
	require.NotNil(t, m.filtered)

	m = press(t, m, "esc")
	assert.False(t, m.filtering)
	assert.Nil(t, m.filtered, "escape restores the whole directory")
	assert.Len(t, m.items(), 3)
}

// The highlight lights up matched runes without changing the cell's width — the
// same escape-blindness that once cut styled rows applies here.
func TestHighlightKeepsExactWidth(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	base := lipgloss.NewStyle()
	hl := lipgloss.NewStyle().Bold(true)

	for _, tc := range []struct{ cell, query string }{
		{"node_modules        ", "nmd"},
		{"Downloads           ", "dwn"},
		{"no-match-here       ", "zzz"},
		{"exact               ", ""},
	} {
		got := highlightMatch(tc.cell, tc.query, &base, &hl)
		assert.Equal(t, lipgloss.Width(tc.cell), lipgloss.Width(got),
			"highlight changed the width of %q", tc.cell)
	}
}
