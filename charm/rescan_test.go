package charm

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/device"
)

// Scanning twice used to panic, and had since the first slice.
//
// An analyzer's done-channel is closed when its walk finishes — SignalGroup's
// Broadcast *is* a close — so a second AnalyzeDir on the same analyzer closes a
// closed channel. That is a panic, not an error, and it took down the program.
// gdu calls ResetProgress before every scan; we called it before none.
//
// Only the first scan ever worked. `r` was broken from the slice that added it,
// and `cdu -d` — analyze a device, come back, analyze another — hit the same
// thing. 900 tests were green throughout, because every one of them stopped at
// the tea.Cmd rather than running it.
//
// So this test runs the command. That is the whole point of it.
func TestScanningTwiceDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0o600))

	ui := CreateUI(nil, false, false, false, false)
	require.NoError(t, ui.AnalyzePath(dir, nil))

	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 24, true

	// The first scan is the one that always worked: the analyzer is fresh.
	first := m.startScan()
	require.NotNil(t, first)
	msg := scanCmd(ui)()
	done, ok := msg.(scanDoneMsg)
	require.True(t, ok, "first scan: %T", msg)
	require.NotNil(t, done.dir)
	m.Update(done)

	// And the second, which is where it died.
	require.NotPanics(t, func() {
		m.rescan()
		msg = scanCmd(ui)()
	}, "a second scan must not close the analyzer's done channel twice")

	done, ok = msg.(scanDoneMsg)
	require.True(t, ok, "second scan: %T", msg)
	require.NotNil(t, done.dir, "the rescan must come back with a tree")
	// a.txt, sub/ and sub/b.txt, plus the root itself.
	assert.Equal(t, int64(4), int64(done.dir.GetItemCount()), "and it must be the same tree")

	// And a third, because "reset once" would also pass a two-scan test.
	require.NotPanics(t, func() {
		m.rescan()
		msg = scanCmd(ui)()
	})
	_, ok = msg.(scanDoneMsg)
	require.True(t, ok, "third scan: %T", msg)
}

// The same, through the disks screen: pick a device, go back, pick again. This
// is the path it was reported on.
func TestAnalyzingTwoDevicesInARowDoesNotPanic(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(first, "a.txt"), []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(second, "b.txt"), []byte("world"), 0o600))

	m := disksModel(t, &listGetter{devices: device.Devices{
		{Name: "/dev/first", MountPoint: first, Size: 1 << 30, Free: 1 << 29},
		{Name: "/dev/second", MountPoint: second, Size: 2 << 30, Free: 1 << 29},
	}})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	for i := range m.disks {
		m.diskCursor = i
		next, cmd := m.analyzeDisk()
		m = next.(*model)
		require.NotNil(t, cmd)

		var msg tea.Msg
		require.NotPanics(t, func() { msg = scanCmd(m.ui)() }, "device %d panicked", i)
		done, ok := msg.(scanDoneMsg)
		require.True(t, ok, "device %d: %T", i, msg)
		m.Update(done)
		assert.Equal(t, screenBrowse, m.scr)

		// Back to the list, as the report described.
		m = press(t, m, "left")
		require.Equal(t, screenDisks, m.scr)
	}
}

// startScan is the only way a scan may be launched, because it is the only thing
// that resets the analyzer. A scanCmd reached any other way is the bug coming
// back.
func TestEveryScanGoesThroughStartScan(t *testing.T) {
	dir := t.TempDir()
	ui := CreateUI(nil, false, false, false, false)
	require.NoError(t, ui.AnalyzePath(dir, nil))

	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 24, true

	// Every entry point that starts a walk: the first scan, the rescan key, and
	// the disks screen. If a fourth appears, it belongs here.
	require.NotNil(t, m.Init())
	require.NotNil(t, m.rescan())

	m.disks = device.Devices{{Name: "/dev/x", MountPoint: dir, Size: 1 << 30, Free: 1 << 29}}
	_, cmd := m.analyzeDisk()
	require.NotNil(t, cmd)

	// Each of those reset the analyzer, so each left it walkable.
	require.NotPanics(t, func() { scanCmd(ui)() })
}
