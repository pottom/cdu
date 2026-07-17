package charm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/dup"
	"github.com/pottom/cdu/pkg/analyze"
)

// dupModel scans a real tree with real duplicate files, so the test drives the
// whole path — F, the hash, the screen — not a mock of the result.
func dupModel(t *testing.T, files map[string]string) *model {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o700))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}

	ui := CreateUI(nil, true, false, false, false)
	require.NoError(t, ui.AnalyzePath(root, nil))
	m := newModel(ui)
	m.width, m.height, m.haveSize = 100, 24, true

	m.startScan()
	done := scanCmd(ui)().(scanDoneMsg)
	m.Update(done)
	require.Equal(t, screenBrowse, m.scr)
	return m
}

// runSearch presses F and drives the hash command to completion.
func runSearch(t *testing.T, m *model) *model {
	t.Helper()
	next, cmd := m.Update(key("F"))
	m = next.(*model)
	require.Equal(t, screenHashing, m.scr, "F must start the search")
	require.NotNil(t, cmd)

	// The command batches the spinner tick and the search; find and run the one
	// that returns the result.
	msg := drainForDupDone(t, cmd)
	next, _ = m.Update(msg)
	return next.(*model)
}

func drainForDupDone(t *testing.T, cmd tea.Cmd) dupDoneMsg {
	t.Helper()
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if d, ok := c().(dupDoneMsg); ok {
				return d
			}
		}
		t.Fatal("no dupDoneMsg in the batch")
	}
	if d, ok := msg.(dupDoneMsg); ok {
		return d
	}
	t.Fatalf("expected a dupDoneMsg, got %T", msg)
	return dupDoneMsg{}
}

// The whole feature: two identical films in different directories are found and
// shown, largest first.
func TestFindingDuplicatesEndToEnd(t *testing.T) {
	m := dupModel(t, map[string]string{
		"Movies/wedding.mov":        "the very same bytes, twice over, on disk",
		"backup/Movies/wedding.mov": "the very same bytes, twice over, on disk",
		"notes.txt":                 "unique, unmatched, alone",
	})

	m = runSearch(t, m)
	require.Equal(t, screenDup, m.scr, "with duplicates found, the screen opens")
	require.Len(t, m.dupGroups, 1)
	assert.Len(t, m.dupGroups[0].Files, 2)
	assert.Contains(t, m.status, "reclaimable")

	// Both copies are marked for the browser.
	for _, f := range m.dupGroups[0].Files {
		assert.True(t, m.isDuplicate(f), "%s must be marked", f.GetName())
	}
}

// No duplicates is not an empty screen that reads as "broken"; it says so and
// stays in the browser.
func TestNoDuplicatesSaysSo(t *testing.T) {
	m := dupModel(t, map[string]string{
		"a.txt": "one of a kind",
		"b.txt": "also unique but a different length entirely here",
	})
	m = runSearch(t, m)

	assert.Equal(t, screenBrowse, m.scr)
	assert.Contains(t, m.status, "no duplicate")
	assert.Empty(t, m.dupGroups)
}

// F before a scan has nothing to search.
func TestFindDuplicatesBeforeAScanSaysSo(t *testing.T) {
	ui := CreateUI(nil, true, false, false, false)
	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 24, true
	m.scr = screenBrowse

	next, _ := m.findDuplicates()
	m = next.(*model)
	assert.Equal(t, screenBrowse, m.scr)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "nothing scanned")
}

// esc during the hash cancels back to the browser. The search checks the flag
// between files, so it lands.
func TestCancellingTheSearchReturnsToBrowse(t *testing.T) {
	m := dupModel(t, map[string]string{"a": "xx", "b": "xx"})

	next, _ := m.Update(key("F"))
	m = next.(*model)
	require.Equal(t, screenHashing, m.scr)

	m = press(t, m, "esc")
	assert.True(t, m.cancelling)
	assert.True(t, m.ui.cancel.Load(), "the search must be told to stop")

	// The cancelled search comes back with ErrCancelled, which lands in the browser.
	next, _ = m.Update(dupDoneMsg{err: dup.ErrCancelled})
	m = next.(*model)
	assert.Equal(t, screenBrowse, m.scr)
	assert.Contains(t, m.status, "cancelled")
}

// The cursor moves over files and never rests on a group header — a header is a
// label, and enter on one would do nothing.
func TestDupCursorSkipsGroupHeaders(t *testing.T) {
	m := dupModel(t, map[string]string{
		"x/1": "aaaa", "y/1": "aaaa", // group 1
		"x/2": "bbbbbb", "y/2": "bbbbbb", // group 2
	})
	m = runSearch(t, m)
	require.Len(t, m.dupGroups, 2)

	for i := range m.dupRows {
		if m.dupRows[i].isHeader() {
			require.NotEqual(t, i, m.dupCursor, "the cursor must not sit on a header")
		}
	}
	for range len(m.dupRows) + 2 {
		m = press(t, m, "down")
		assert.False(t, m.dupRows[m.dupCursor].isHeader(), "moving down must skip headers")
	}
}

// Reveal opens the copy's directory, so you can see what is around it before
// choosing which to keep.
func TestRevealingADuplicateOpensItsDirectory(t *testing.T) {
	m := dupModel(t, map[string]string{
		"here/song.mp3":  "same tune",
		"there/song.mp3": "same tune",
	})
	m = runSearch(t, m)

	want := m.selectedDup()
	require.NotNil(t, want)
	m = press(t, m, "enter")
	assert.Equal(t, screenBrowse, m.scr)
	assert.Equal(t, want.GetParent(), m.currentDir, "it opens the file's own directory")
	assert.Equal(t, want, m.selected(), "with the cursor on the file")
}

// Deleting a copy dissolves a group once only one copy is left — one file is a
// duplicate of nothing.
func TestDeletingUntilOneCopyDissolvesTheGroup(t *testing.T) {
	m := dupModel(t, map[string]string{
		"a/f": "twinned bytes here",
		"b/f": "twinned bytes here",
	})
	m = runSearch(t, m)
	require.Len(t, m.dupGroups, 1)

	victim := m.dupGroups[0].Files[0]
	m.dropDuplicate(victim)

	assert.Empty(t, m.dupGroups, "a lone survivor is not a duplicate")
	assert.False(t, m.isDuplicate(victim))
	assert.Equal(t, screenBrowse, m.scr, "with nothing left, the screen closes")
}

// A three-copy group survives one deletion, as a two-copy group.
func TestDeletingOneOfThreeKeepsTheGroup(t *testing.T) {
	m := dupModel(t, map[string]string{
		"a/f": "three of these", "b/f": "three of these", "c/f": "three of these",
	})
	m = runSearch(t, m)
	require.Len(t, m.dupGroups[0].Files, 3)

	m.dropDuplicate(m.dupGroups[0].Files[0])
	require.Len(t, m.dupGroups, 1)
	assert.Len(t, m.dupGroups[0].Files, 2, "two copies are still duplicates of each other")
}

// A duplicate is marked in the browser, and the mark is a glyph so it survives
// mono and a colourblind eye.
func TestDuplicatesAreMarkedInTheBrowser(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := dupModel(t, map[string]string{
		"a/dup.bin": "identical content for the marker test",
		"b/dup.bin": "identical content for the marker test",
	})
	m = runSearch(t, m)
	m = press(t, m, "esc") // back to the browser, into a directory with a marked file

	// Navigate into a directory that holds a marked copy.
	for i, r := range m.rows {
		if r.IsDir() {
			m.cursor = i
		}
	}
	m = press(t, m, "right")

	total := m.itemSize(m.currentDir)
	var sawMark bool
	for i, r := range m.rows {
		if m.isDuplicate(r) {
			assert.Contains(t, m.viewRow(r, false, total), dupMark, "a marked file shows the glyph")
			_ = i
			sawMark = true
		}
	}
	assert.True(t, sawMark, "the directory must contain a marked duplicate")
}

// d and D work from the duplicate screen, targeting the file's own directory —
// there is no "current directory" here.
func TestDeletingFromTheDupScreenTargetsTheFilesParent(t *testing.T) {
	m := dupModel(t, map[string]string{
		"deep/nested/a.bin": "twin bytes",
		"other/b.bin":       "twin bytes",
	})
	m = runSearch(t, m)

	item, parent := m.target()
	require.NotNil(t, item)
	require.NotNil(t, parent)
	assert.Equal(t, item.GetParent(), parent)

	m = press(t, m, "D")
	require.Equal(t, screenConfirm, m.scr)
	assert.Equal(t, screenDup, m.confirmFrom, "cancelling returns to the dup screen")
}

// The screen holds the exact-height and no-overflow rules at any size, like
// every other.
func TestDupScreenFitsTheTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := dupModel(t, map[string]string{
		"one/big.iso":  strings.Repeat("z", 2000),
		"two/big.iso":  strings.Repeat("z", 2000),
		"a/small.txt":  "tiny twin",
		"bb/small.txt": "tiny twin",
	})
	m = runSearch(t, m)
	require.Equal(t, screenDup, m.scr)

	for width := 0; width <= 120; width++ {
		for _, height := range []int{1, 2, 3, 8, 24} {
			m.width, m.height = width, height
			m.moveDupCursor(0)
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

var _ = analyze.CreateAnalyzer
