package charm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/device"
)

// deepTree builds a tree wide and deep enough that a full walk is obviously more
// work than a cancelled one.
func deepTree(t *testing.T, breadth, depth int) string {
	t.Helper()
	root := t.TempDir()

	var build func(path string, left int)
	build = func(path string, left int) {
		for i := range breadth {
			f := filepath.Join(path, fmt.Sprintf("file%d.txt", i))
			require.NoError(t, os.WriteFile(f, []byte("x"), 0o600))
		}
		if left == 0 {
			return
		}
		for i := range breadth {
			sub := filepath.Join(path, fmt.Sprintf("dir%d", i))
			require.NoError(t, os.Mkdir(sub, 0o700))
			build(sub, left-1)
		}
	}
	build(root, depth)
	return root
}

// The claim this feature rests on: cancelling really stops the walk.
//
// cdu has no context to cancel and no Stop to call — the analyzer offers
// neither, and pkg/analyze is upstream's. What it does offer is the ignore hook,
// which it consults before descending into each directory and which cdu supplies.
// Answering "ignore it" from the moment cancel is set makes the walk skip every
// directory it has not opened yet.
//
// So: a walk cancelled before it starts must come back with almost nothing,
// while the same tree walked normally comes back with all of it. If cancelling
// were a lie — a flag nobody reads — both numbers would be the same.
func TestCancellingReallyStopsTheWalk(t *testing.T) {
	root := deepTree(t, 4, 4) // 4^4 directories, 4 files each

	full := CreateUI(nil, false, false, false, false)
	require.NoError(t, full.AnalyzePath(root, nil))
	full.Analyzer.ResetProgress()
	whole := scanCmd(full)().(scanDoneMsg)
	require.NotNil(t, whole.dir)
	require.Greater(t, int(whole.dir.GetItemCount()), 300, "the fixture is not big enough to prove anything")

	stopped := CreateUI(nil, false, false, false, false)
	require.NoError(t, stopped.AnalyzePath(root, nil))
	stopped.Analyzer.ResetProgress()
	stopped.cancel.Store(true) // cancelled before it starts: the extreme case
	partial := scanCmd(stopped)().(scanDoneMsg)

	require.NotNil(t, partial.dir, "a cancelled walk still has to return, not hang")
	assert.Less(t, int(partial.dir.GetItemCount()), int(whole.dir.GetItemCount())/10,
		"the cancelled walk did nearly as much work as the full one — it is not stopping")
}

// A cancelled walk unwinds rather than hanging: the goroutines have to finish
// and the done channel has to close, or the next scan panics on it.
func TestACancelledScanCanBeFollowedByAnother(t *testing.T) {
	root := deepTree(t, 3, 3)

	ui := CreateUI(nil, false, false, false, false)
	require.NoError(t, ui.AnalyzePath(root, nil))

	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 24, true

	m.startScan()
	m.cancelScan()
	require.NotPanics(t, func() { scanCmd(ui)() })

	// And the analyzer is usable again afterwards.
	m.startScan()
	assert.False(t, ui.cancel.Load(), "startScan must clear the flag, or every later scan is cancelled")
	msg := scanCmd(ui)().(scanDoneMsg)
	require.NotNil(t, msg.dir)
	assert.Greater(t, int(msg.dir.GetItemCount()), 10, "the scan after a cancel must be a real one")
}

// esc cancels; q quits. q means quit on every other screen, and a key that means
// something else on one screen is a key you cannot trust.
func TestEscCancelsAndQStillQuits(t *testing.T) {
	m := benchModel(3)
	m.width, m.height, m.haveSize = 80, 24, true
	m.scr = screenScanning

	_, cmd := m.Update(key("esc"))
	assert.Nil(t, cmd, "esc must not quit the program")
	assert.True(t, m.cancelling)
	assert.True(t, m.ui.cancel.Load(), "the walk must be told")

	// The screen says so: the walk cannot stop mid-directory, and the gap between
	// the keypress and the screen changing must not read as a key that missed.
	assert.Contains(t, m.viewScanBody(), "cancelling")

	m2 := benchModel(3)
	m2.width, m2.height, m2.haveSize = 80, 24, true
	m2.scr = screenScanning
	_, cmd = m2.Update(key("q"))
	assert.NotNil(t, cmd, "q must still quit")
	assert.True(t, m2.ui.cancel.Load(), "and stop the walk on the way out")
}

// Where a cancelled scan lands is the same question the back key answers
// everywhere else: whatever it interrupted.
func TestACancelledScanGoesBackToWhereItCameFrom(t *testing.T) {
	// From `cdu -d`: the device list.
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))
	next, _ := m.analyzeDisk()
	m = next.(*model)
	require.Equal(t, screenScanning, m.scr)

	m.cancelScan()
	next, cmd := m.Update(scanDoneMsg{dir: nil})
	m = next.(*model)
	assert.Nil(t, cmd, "it must not quit — there is a list to go back to")
	assert.Equal(t, screenDisks, m.scr)
	assert.Contains(t, m.status, "cancelled")

	// From a rescan: the tree that was already there, which is still true.
	b := benchModel(3)
	b.width, b.height, b.haveSize = 80, 24, true
	b.scr = screenBrowse
	b.ui.scanPath = t.TempDir() // rescan declines without one — this view came from a file
	before := b.currentDir
	require.NotNil(t, before)

	b.rescan()
	require.Equal(t, screenScanning, b.scr)
	require.Equal(t, before, b.currentDir, "rescan must not throw the tree away before it has a new one")

	b.cancelScan()
	next, cmd = b.Update(scanDoneMsg{dir: nil})
	b = next.(*model)
	assert.Nil(t, cmd)
	assert.Equal(t, screenBrowse, b.scr)
	assert.Equal(t, before, b.currentDir, "and the tree is the one we had")
}

// Cancelling the only thing cdu was asked to do leaves nothing to show.
func TestCancellingTheFirstScanLeaves(t *testing.T) {
	ui := CreateUI(nil, false, false, false, false)
	require.NoError(t, ui.AnalyzePath(t.TempDir(), nil))

	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 24, true
	m.Init()
	require.Nil(t, m.currentDir)
	require.Nil(t, m.disks)

	m.cancelScan()
	_, cmd := m.Update(scanDoneMsg{dir: nil})
	require.NotNil(t, cmd, "with nothing behind it, there is nowhere to go but out")
	assert.Equal(t, tea.Quit(), cmd(), "and that is a quit")
}

// The partial tree is discarded, never shown. The directories the walk never
// opened are absent from it, so every parent above them reports less than it
// holds — and the next key along is `d`.
func TestACancelledScansTreeIsNotShown(t *testing.T) {
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))
	next, _ := m.analyzeDisk()
	m = next.(*model)

	m.cancelScan()
	// A tree does come back — a cancelled walk still returns one, just a short one.
	scanned := benchModel(5)
	next, _ = m.Update(scanDoneMsg{dir: scanned.topDir})
	m = next.(*model)

	assert.Equal(t, screenDisks, m.scr, "it must not land in the browser")
	assert.Nil(t, m.topDir, "and the half-walked tree must not be kept")
}

var _ = device.Devices{}
