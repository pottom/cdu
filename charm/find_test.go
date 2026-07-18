package charm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typeInto presses each rune of a string in turn.
func typeInto(t *testing.T, m *model, s string) *model {
	t.Helper()
	for _, r := range s {
		m = press(t, m, string(r))
	}
	return m
}

// matchName is the whole matching rule, and worth pinning on its own: glob when
// there is a wildcard, case-insensitive substring otherwise.
func TestMatchName(t *testing.T) {
	// Substring, case-insensitive — bare "mkv" finds .mkv without the glob.
	assert.True(t, matchName("film.mkv", "mkv"))
	assert.True(t, matchName("FILM.MKV", "mkv"), "case-insensitive")
	assert.True(t, matchName("my mkv notes.txt", "mkv"), "substring, anywhere in the name")
	assert.False(t, matchName("film.mp4", "mkv"))

	// Glob, when a wildcard is present.
	assert.True(t, matchName("film.mkv", "*.mkv"))
	assert.True(t, matchName("IMG_2024.JPG", "*.jpg"), "glob is case-insensitive too")
	assert.False(t, matchName("film.mkv", "*.mp4"))
	assert.True(t, matchName("report3.pdf", "report?.pdf"))

	// A bare "." is a substring, not a glob — most names contain one.
	assert.True(t, matchName("a.txt", "."))
	// The empty pattern matches nothing rather than everything.
	assert.False(t, matchName("anything", ""))
}

// The whole feature: f, type a pattern, enter, and every match in the subtree is
// listed — the thing / could not do, because / only saw one directory.
func TestFindListsMatchesAnywhereInTheSubtree(t *testing.T) {
	m := dupModel(t, map[string]string{
		"Movies/wedding.mkv":      "a",
		"backup/deep/holiday.mkv": "bb",
		"Downloads/note.txt":      "ccc",
		"Music/song.mp3":          "dddd",
	})

	next, _ := m.Update(key("f"))
	m = next.(*model)
	require.True(t, m.finding, "f opens the find prompt")

	m = typeInto(t, m, "*.mkv")
	m = press(t, m, "enter")

	require.Equal(t, screenFind, m.scr)
	require.Len(t, m.findResults, 2, "both .mkv files, in different directories")
	names := []string{m.findResults[0].GetName(), m.findResults[1].GetName()}
	assert.ElementsMatch(t, []string{"wedding.mkv", "holiday.mkv"}, names)
	assert.NotContains(t, m.headerPath(), "note", "the header names the pattern and count")
	assert.Contains(t, m.headerPath(), "*.mkv")
}

// Biggest first — a disk usage tool, so the largest match is the one you came
// for.
func TestFindResultsAreBiggestFirst(t *testing.T) {
	m := dupModel(t, map[string]string{
		"small.log": strings.Repeat("s", 10),
		"big.log":   strings.Repeat("b", 9000),
		"mid.log":   strings.Repeat("m", 500),
	})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "log")
	m = press(t, m, "enter")

	require.Len(t, m.findResults, 3)
	assert.Equal(t, "big.log", m.findResults[0].GetName())
	assert.Equal(t, "small.log", m.findResults[2].GetName())
}

// A bare word is a substring search — the common case, no glob to type.
func TestFindBySubstring(t *testing.T) {
	m := dupModel(t, map[string]string{
		"holiday-video.mkv": "a",
		"holiday-pics.zip":  "bb",
		"work.txt":          "ccc",
	})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "holiday")
	m = press(t, m, "enter")

	require.Len(t, m.findResults, 2)
}

// No match is not an empty screen; it says so and stays in the browser.
func TestFindWithNoMatchSaysSo(t *testing.T) {
	m := dupModel(t, map[string]string{"a.txt": "x"})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "*.mkv")
	m = press(t, m, "enter")

	assert.Equal(t, screenBrowse, m.scr)
	assert.Contains(t, m.status, "nothing matching")
	assert.Contains(t, m.status, "*.mkv")
}

// esc leaves the prompt without searching; an empty pattern does nothing.
func TestFindPromptCancels(t *testing.T) {
	m := dupModel(t, map[string]string{"a.mkv": "x"})

	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "abc")
	m = press(t, m, "esc")
	assert.False(t, m.finding, "esc closes the prompt")
	assert.Equal(t, screenBrowse, m.scr, "and does not search")

	// Enter on an empty pattern is a no-op, not an empty result screen.
	next, _ = m.Update(key("f"))
	m = next.(*model)
	m = press(t, m, "enter")
	assert.False(t, m.finding)
	assert.Equal(t, screenBrowse, m.scr)
}

// While typing, q is a letter of the pattern, not a way out — the find prompt
// swallows every key like the filter and the menus do.
func TestFindPromptSwallowsEveryKey(t *testing.T) {
	m := dupModel(t, map[string]string{"quiz.txt": "x"})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "quiz")
	assert.Equal(t, "quiz", m.findQuery, "q is a letter here")
	assert.True(t, m.finding)
}

// find searches from the current directory, like T and F.
func TestFindSearchesFromTheCurrentDirectory(t *testing.T) {
	m := dupModel(t, map[string]string{
		"top.mkv":      "a",
		"sub/deep.mkv": "bb",
	})

	// Into sub, then find: only sub's match.
	for i, r := range m.rows {
		if r.IsDir() && r.GetName() == "sub" {
			m.cursor = i
		}
	}
	m = press(t, m, "right")
	require.Equal(t, "sub", m.currentDir.GetName())

	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "*.mkv")
	m = press(t, m, "enter")

	require.Len(t, m.findResults, 1)
	assert.Equal(t, "deep.mkv", m.findResults[0].GetName())
	assert.Contains(t, m.headerPath(), "/sub")
}

// Reveal opens the match's directory, and delete targets the file's own parent.
func TestFindRevealAndDelete(t *testing.T) {
	m := dupModel(t, map[string]string{
		"a/deep/target.iso": "content",
	})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "target")
	m = press(t, m, "enter")
	require.Equal(t, screenFind, m.scr)

	// Delete targets the file's own directory — there is no current directory here.
	item, parent := m.target()
	require.NotNil(t, item)
	assert.Equal(t, item.GetParent(), parent)
	m = press(t, m, "D")
	assert.Equal(t, screenConfirm, m.scr)
	assert.Equal(t, screenFind, m.confirmFrom, "cancelling returns to the results")
	m = press(t, m, "esc")
	require.Equal(t, screenFind, m.scr)

	// Reveal opens the directory.
	want := m.selectedFind()
	m = press(t, m, "enter")
	assert.Equal(t, screenBrowse, m.scr)
	assert.Equal(t, want.GetParent(), m.currentDir)
	assert.Equal(t, want, m.selected())
}

// A deleted match leaves the list, and the list closes when the last one goes.
func TestDeletedMatchLeavesTheList(t *testing.T) {
	m := dupModel(t, map[string]string{"only.mkv": "x"})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "mkv")
	m = press(t, m, "enter")
	require.Len(t, m.findResults, 1)

	m.dropFindResult(m.findResults[0])
	assert.Empty(t, m.findResults)
	assert.Equal(t, screenBrowse, m.scr, "with nothing left, the screen closes")
}

// / is unchanged: still the fuzzy, local filter. The two tools stay separate.
func TestFilterIsStillLocalAndFuzzy(t *testing.T) {
	m := dupModel(t, map[string]string{
		"apple.txt":   "a",
		"grape.txt":   "b",
		"sub/apricot": "c",
	})
	m = press(t, m, "/")
	require.True(t, m.filtering, "/ still opens the filter")
	m = typeInto(t, m, "ap")

	// Fuzzy, and local: apple matches, and the subdirectory's apricot is not
	// pulled up — / only ever sees this directory.
	var shown []string
	for _, it := range m.items() {
		shown = append(shown, it.GetName())
	}
	assert.Contains(t, shown, "apple.txt")
	assert.NotContains(t, shown, "apricot", "/ does not reach into subdirectories")
}

// The results screen holds the exact-height and no-overflow rules at any size.
func TestFindScreenFitsTheTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := dupModel(t, map[string]string{
		"one/big.mkv":   strings.Repeat("z", 3000),
		"two/small.mkv": "tiny",
	})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "mkv")
	m = press(t, m, "enter")
	require.Equal(t, screenFind, m.scr)

	for width := 0; width <= 120; width++ {
		for _, height := range []int{1, 2, 3, 8, 24} {
			m.width, m.height = width, height
			m.moveFindCursor(0)
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

// The prompt line also holds the width rule at any size.
func TestFindPromptFitsTheTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := dupModel(t, map[string]string{"a": "x"})
	next, _ := m.Update(key("f"))
	m = next.(*model)
	m = typeInto(t, m, "a rather long pattern to type")

	for width := 0; width <= 100; width++ {
		m.width = width
		assert.LessOrEqual(t, lipgloss.Width(m.viewFindFooter()), width, "prompt overflows at width %d", width)
	}
}
