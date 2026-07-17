package charm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// topModel is a small tree with the big files buried, which is the situation the
// screen exists for: the tree makes you hunt for them one directory at a time.
func topModel(t *testing.T) *model {
	t.Helper()
	ui := CreateUI(nil, true, false, false, false)
	m := newModel(ui)
	m.width, m.height, m.haveSize = 100, 24, true

	root := &analyze.Dir{File: &analyze.File{Name: "root"}, BasePath: "/"}
	movies := &analyze.Dir{File: &analyze.File{Name: "Movies", Parent: root}}
	deep := &analyze.Dir{File: &analyze.File{Name: "Caches", Parent: movies}}
	root.AddFile(movies)
	movies.AddFile(deep)

	root.AddFile(&analyze.File{Name: "small.txt", Size: 1 << 10, Usage: 1 << 10, Parent: root})
	movies.AddFile(&analyze.File{Name: "wedding-4k-master.mov", Size: 8 << 30, Usage: 8 << 30, Parent: movies})
	deep.AddFile(&analyze.File{Name: "buried.iso", Size: 4 << 30, Usage: 4 << 30, Parent: deep})
	for i := range 20 {
		deep.AddFile(&analyze.File{
			Name: fmt.Sprintf("chunk%02d.bin", i),
			Size: int64(i) << 20, Usage: int64(i) << 20, Parent: deep,
		})
	}
	root.UpdateStats(make(fs.HardLinkedItems))

	m.topDir = root
	m.enterDir(root)
	m.scr = screenBrowse
	return m
}

// The whole point: the biggest files anywhere, largest first, however deep.
func TestTopFindsTheBiggestFilesAtAnyDepth(t *testing.T) {
	m := topModel(t)

	next, _ := m.Update(key("T"))
	m = next.(*model)

	require.Equal(t, screenTop, m.scr)
	require.NotEmpty(t, m.topFiles)
	assert.Equal(t, "wedding-4k-master.mov", m.topFiles[0].GetName(), "biggest first")
	assert.Equal(t, "buried.iso", m.topFiles[1].GetName(), "two directories down, and still found")

	for i := 1; i < len(m.topFiles); i++ {
		assert.GreaterOrEqual(t, m.topFiles[i-1].GetSize(), m.topFiles[i].GetSize())
	}
	// Directories are not files. The screen is about one huge thing, not a folder.
	for _, f := range m.topFiles {
		assert.False(t, f.IsDir(), "%s is a directory", f.GetName())
	}
}

// It is a snapshot taken on the keypress, not a view kept up to date. The header
// says how many, and esc goes back.
func TestTopIsASnapshotAndEscGoesBack(t *testing.T) {
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)

	assert.Contains(t, m.headerPath(), "largest")
	assert.Contains(t, m.headerPath(), "any depth")

	m = press(t, m, "esc")
	assert.Equal(t, screenBrowse, m.scr)
	assert.NotEmpty(t, m.topFiles, "the snapshot survives — T is not re-collected on the way back")
}

// T before a scan has nothing to look through, and says so rather than showing
// an empty list that reads as "no big files".
func TestTopBeforeAScanSaysSo(t *testing.T) {
	ui := CreateUI(nil, true, false, false, false)
	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 24, true
	m.scr = screenBrowse

	next, _ := m.collectTopFiles()
	m = next.(*model)
	assert.Equal(t, screenBrowse, m.scr)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "nothing scanned")
}

// Reveal is what turns "this file is enormous" into "and here is where it
// lives". The tree is where you can do something about it.
func TestRevealOpensTheFilesDirectory(t *testing.T) {
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)
	m = press(t, m, "down") // buried.iso, two directories down

	want := m.selectedTop()
	require.Equal(t, "buried.iso", want.GetName())

	m = press(t, m, "enter")
	assert.Equal(t, screenBrowse, m.scr)
	assert.Equal(t, "Caches", m.currentDir.GetName(), "it opens the directory the file is in")
	assert.Equal(t, want, m.selected(), "with the cursor on the file")
}

// Deleting from here has to act on the file's own directory — there is no
// "current directory" on this screen, so the browser's answer would be wrong.
func TestDeletingFromTheTopListTargetsTheFilesOwnParent(t *testing.T) {
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)
	m = press(t, m, "down") // buried.iso, in Caches — not the directory being browsed

	item, parent := m.target()
	require.NotNil(t, item)
	require.NotNil(t, parent)
	assert.Equal(t, "buried.iso", item.GetName())
	assert.Equal(t, "Caches", parent.GetName(), "not the browser's current directory, which is root")

	// And the modal comes back here rather than dropping you in the browser.
	m = press(t, m, "D")
	require.Equal(t, screenConfirm, m.scr)
	assert.Equal(t, screenTop, m.confirmFrom)
	m = press(t, m, "esc")
	assert.Equal(t, screenTop, m.scr, "cancelling must return to the list you were in")
}

// A deleted file leaves the list. Recollecting would be the whole walk again to
// learn that one row is gone.
func TestADeletedFileLeavesTheTopList(t *testing.T) {
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)

	gone := m.topFiles[0]
	before := len(m.topFiles)
	m.dropTopFile(gone)

	assert.Len(t, m.topFiles, before-1)
	for _, f := range m.topFiles {
		assert.NotEqual(t, gone, f)
	}
	assert.NotNil(t, m.selectedTop(), "the cursor must land somewhere real")
}

// v works from here too, and closing returns here rather than to the browser.
func TestViewingFromTheTopListReturnsToIt(t *testing.T) {
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)

	next, cmd := m.Update(key("v"))
	m = next.(*model)
	require.NotNil(t, cmd, "a read must be started")
	assert.Equal(t, screenTop, m.viewerFrom)

	m.scr = screenViewer
	m.viewer = &viewerState{path: "/x", lines: []string{"a"}}
	m = press(t, m, "esc")
	assert.Equal(t, screenTop, m.scr)
}

// The same rules as every screen: exactly m.height lines, nothing wider than the
// terminal, at any size.
func TestTopScreenFitsTheTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)

	for width := 0; width <= 120; width++ {
		for _, height := range []int{1, 2, 3, 8, 24} {
			m.width, m.height = width, height
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

// The name is what you recognise a file by, so the directory gives up its
// columns first. A path you can read attached to a name you cannot is the wrong
// trade.
func TestTheNameSurvivesANarrowTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := topModel(t)
	next, _ := m.Update(key("T"))
	m = next.(*model)

	m.width = 100
	assert.Contains(t, m.viewTopRow(m.topFiles[0], false), "wedding-4k-master.mov")
	assert.Contains(t, m.viewTopRow(m.topFiles[0], false), "Movies", "and its directory, when there is room")

	m.width = 44
	narrow := m.viewTopRow(m.topFiles[0], false)
	assert.Contains(t, narrow, "wedding", "the name survives")
}
