package charm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The footer advertises keys and so does the help: two places describing one
// thing, and therefore two places that can disagree. This is the seam.
//
// It cannot check that the words are still true — nothing can — but a key can
// never be silently undocumented, which is the failure that actually happens:
// someone adds a binding, puts it in the footer where they are already working,
// and the help quietly stops being every key on one screen.
func TestHelpCoversEveryFooterKey(t *testing.T) {
	documented := map[string]bool{}
	for _, g := range helpGroups {
		for _, e := range g.entries {
			// The key column holds several spellings of one binding: "↑ ↓  k j".
			for _, k := range strings.Fields(e.keys) {
				documented[k] = true
			}
			// And the second half of a two-key menu is described in the sentence, not
			// in the key column: "sort: then s size, n name…".
			for _, word := range strings.Fields(e.what) {
				documented[strings.Trim(word, ",:—")] = true
			}
		}
	}

	all := [][]keyHint{browseKeys, sortMenuKeys, colMenuKeys, scanKeys, confirmKeys, diskKeys, topKeys, helpKeys}
	all = append(all, []keyHint{undoKey})

	for _, table := range all {
		for _, hint := range table {
			// A hint's key can be a pair the help spells out separately: "↑↓" here is
			// "↑ ↓" there, because the help has room to.
			if documented[hint.key] {
				continue
			}
			var missing []string
			for _, r := range hint.key {
				if !documented[string(r)] {
					missing = append(missing, string(r))
				}
			}
			assert.Empty(t, missing,
				"the footer offers %q (%s) and the help does not mention %v",
				hint.key, hint.label, missing)
		}
	}
}

// Every binding the browse screen actually handles has to be in the help, not
// just the ones the footer has room for. The footer sheds hints on a narrow
// terminal; the help is where they all live.
// allHelpText is the help's whole text, for asserting a key or a word is
// documented — the footer no longer lists everything, so help is where a binding
// has to appear.
func allHelpText() string {
	var text strings.Builder
	for _, g := range helpGroups {
		text.WriteString(g.title + "\n")
		for _, e := range g.entries {
			text.WriteString(e.keys + " " + e.what + "\n")
		}
	}
	return text.String()
}

func TestHelpDocumentsTheKeysThatDoThings(t *testing.T) {
	help := allHelpText()

	for _, key := range []string{
		"d", "D", "e", "u", "r", // the disk
		"s", "t", "a", "B", "c", "m", // the view
		"T", "v", "?", "esc", "q", "/", // elsewhere
		"g", "G", "h", "j", "k", "l", // navigation
	} {
		assert.Contains(t, help, key, "the help never mentions %q", key)
	}
}

// The mock's help is not the spec. It predates most of these bindings, lists `s`
// as "sort by size" from before sorting became two keys, and gives `d` twice —
// as "delete selected" and as "list mounted disks" — which cannot both be true
// and never were, since -d is a flag.
func TestHelpDescribesCdusBindingsNotTheMocks(t *testing.T) {
	var keys []string
	for _, g := range helpGroups {
		for _, e := range g.entries {
			keys = append(keys, e.keys)
		}
	}
	all := strings.Join(keys, " ")

	// One meaning per key. The mock has d doing two different things.
	seen := map[string]int{}
	for _, k := range strings.Fields(all) {
		seen[k]++
	}
	for k, n := range seen {
		assert.Equal(t, 1, n, "%q is listed %d times — one key, one meaning", k, n)
	}

	// And the two-key sort is described as two keys.
	var sortWhat string
	for _, g := range helpGroups {
		for _, e := range g.entries {
			if e.keys == "s" {
				sortWhat = e.what
			}
		}
	}
	assert.Contains(t, sortWhat, "then", "sorting is two keys, and the help must say so")
}

// ? opens from anywhere, and closing returns there rather than to the browser.
func TestHelpOpensFromAnywhereAndReturnsThere(t *testing.T) {
	for _, from := range []screen{screenBrowse, screenTop, screenDisks} {
		m := benchModel(3)
		m.width, m.height, m.haveSize = 100, 24, true
		m.scr = from

		next, _ := m.Update(key("?"))
		m = next.(*model)
		require.Equal(t, screenHelp, m.scr, "? must open the help from screen %d", from)
		assert.Equal(t, from, m.helpFrom)

		m = press(t, m, "?")
		assert.Equal(t, from, m.scr, "closing must return to screen %d", from)
	}
}

// q closes the help rather than quitting, like the viewer: leaving the help you
// asked for should not also leave the program.
func TestQClosesTheHelpRatherThanQuitting(t *testing.T) {
	m := benchModel(3)
	m.width, m.height, m.haveSize = 100, 24, true
	m.scr = screenBrowse

	next, _ := m.Update(key("?"))
	m = next.(*model)

	next, cmd := m.Update(key("q"))
	assert.Nil(t, cmd, "q in the help must not quit the program")
	assert.Equal(t, screenBrowse, next.(*model).scr)
}

// A modal takes every key whole, including ?. While a delete is being confirmed,
// ? is not a way to the help — and in the filter it is a character being typed.
func TestTheModesSwallowTheHelpKey(t *testing.T) {
	m := benchModel(3)
	m.width, m.height, m.haveSize = 100, 24, true
	m.scr = screenConfirm
	m.confirm = &confirmState{item: m.rows[0], parent: m.currentDir, act: actionTrash}

	m = press(t, m, "?")
	assert.Equal(t, screenConfirm, m.scr, "the modal must keep every key")

	f := benchModel(3)
	f.width, f.height, f.haveSize = 100, 24, true
	f.scr = screenBrowse
	f = press(t, f, "/", "?")
	assert.Equal(t, screenBrowse, f.scr, "in the filter, ? is a character")
	assert.Equal(t, "?", f.filter)
}

// Wide, the groups sit two abreast as the mock draws them; narrow, they stack
// and the screen scrolls. Help you have to scroll is help you read half of.
func TestHelpUsesTwoColumnsWhenThereIsRoom(t *testing.T) {
	m := benchModel(3)
	m.height, m.haveSize = 40, true

	m.width = 120
	wide := len(m.helpLines())
	m.width = 60
	narrow := len(m.helpLines())

	assert.Less(t, wide, narrow, "two columns must be shorter than one")
}

// The same rules as every screen: exactly m.height lines, nothing wider than the
// terminal, at any size.
func TestHelpFitsTheTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := benchModel(3)
	m.haveSize = true
	m.scr = screenHelp

	for width := 0; width <= 140; width++ {
		for _, height := range []int{1, 2, 3, 8, 24, 40} {
			m.width, m.height = width, height
			m.clampHelp()
			lines := strings.Split(m.View(), "\n")
			assert.Len(t, lines, height, "frame must be %d lines at %dx%d", height, width, height)
			for i, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Errorf("at %dx%d: line %d is %d columns wide", width, height, i, got)
				}
			}
		}
	}
}

// A page taller than the terminal scrolls, and stops at both ends.
func TestHelpScrollClamps(t *testing.T) {
	m := benchModel(3)
	m.width, m.height, m.haveSize = 60, 10, true
	m.scr = screenHelp
	require.Greater(t, len(m.helpLines()), m.visibleLines(), "this test needs a page that overflows")

	m = press(t, m, "up", "up")
	assert.Equal(t, 0, m.helpOffset, "up at the top stays")

	m = press(t, m, "end")
	assert.Equal(t, len(m.helpLines())-m.visibleLines(), m.helpOffset, "end shows the last screenful, not past it")

	m = press(t, m, "down", "down")
	assert.Equal(t, len(m.helpLines())-m.visibleLines(), m.helpOffset, "down at the bottom stays")

	m = press(t, m, "home")
	assert.Equal(t, 0, m.helpOffset)
}
