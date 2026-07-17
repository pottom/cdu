package charm

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/device"
)

// listGetter returns a whole mount table, unlike confirm_test's fakeGetter,
// which exists to answer one question about one device.
type listGetter struct {
	devices device.Devices
	err     error
}

func (g *listGetter) GetDevicesInfo() (device.Devices, error) { return g.devices, g.err }
func (g *listGetter) GetMounts() (device.Devices, error)      { return g.devices, g.err }

func testDevices() device.Devices {
	return device.Devices{
		{Name: "/dev/disk3s1", MountPoint: "/", Size: 994 << 30, Free: 367 << 30},
		{Name: "/dev/disk4s2", MountPoint: "/Volumes/Backup", Size: 2000 << 30, Free: 600 << 30},
		{Name: "/dev/disk3s5", MountPoint: "/System/Volumes/Data", Size: 994 << 30, Free: 482 << 30},
	}
}

func disksModel(t *testing.T, getter device.DevicesInfoGetter) *model {
	t.Helper()
	ui := CreateUI(nil, true, false, false, false)
	require.NoError(t, ui.ListDevices(getter))

	m := newModel(ui)
	m.width, m.height, m.haveSize = 100, 20, true
	return m
}

// -d used to send people to --classic. It opens the device list now.
func TestListDevicesOpensTheDiskScreen(t *testing.T) {
	m := disksModel(t, &listGetter{devices: testDevices()})
	assert.True(t, m.ui.showDisks)

	// The mount table is read inside the loop, not before it: it can block for a
	// long time on a stale mount, and gdu's version simply shows nothing until it
	// returns.
	require.NotNil(t, m.Init(), "Init must dispatch the read")
	assert.Equal(t, screenScanning, m.scr, "the interface comes up saying what it waits for")
	assert.Contains(t, m.headerPath(), "mount table")

	msg := disksCmd(m.ui)().(disksMsg)
	require.NoError(t, msg.err)
	m.applyDisks(msg)
	assert.Equal(t, screenDisks, m.scr)
	assert.Len(t, m.disks, 3)
}

// Biggest first, like everything else here. The mount table's own order is
// whatever the kernel happens to hold, which means nothing to someone looking
// for the disk that is full.
func TestDisksAreSortedByUsage(t *testing.T) {
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	require.Len(t, m.disks, 3)
	for i := 1; i < len(m.disks); i++ {
		assert.GreaterOrEqual(t, m.disks[i-1].GetUsage(), m.disks[i].GetUsage(),
			"device %d is used less than the one after it", i-1)
	}
	assert.Equal(t, "/dev/disk4s2", m.disks[0].Name, "the 1.4T backup disk is the fullest")
}

// A machine that will not report its mounts has nothing to offer -d, and saying
// so beats an empty list that looks like "no disks".
func TestAFailedMountTableReadIsAnError(t *testing.T) {
	m := disksModel(t, &listGetter{err: errors.New("mount: permission denied")})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	assert.Equal(t, screenError, m.scr)
	require.Error(t, m.err)
	assert.Contains(t, m.err.Error(), "permission denied")
}

func TestNoDeviceGetterIsReported(t *testing.T) {
	ui := CreateUI(nil, true, false, false, false)
	ui.showDisks = true
	m := newModel(ui)
	m.width, m.height, m.haveSize = 80, 20, true

	m.applyDisks(disksCmd(ui)().(disksMsg))
	assert.Equal(t, screenError, m.scr)
	require.ErrorIs(t, m.err, errNoDeviceGetter)
}

// Enter on a device scans it. That is the whole point of the screen.
func TestEnterAnalyzesTheSelectedDevice(t *testing.T) {
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))
	m = press(t, m, "down") // the second device

	want := m.disks[1]
	next, cmd := m.Update(key("enter"))
	m = next.(*model)

	require.NotNil(t, cmd, "a scan must be started")
	assert.Equal(t, screenScanning, m.scr)
	assert.Equal(t, want.MountPoint, m.ui.scanPath, "it scans the device under the cursor")
	assert.Equal(t, want, m.dev, "and the header's disk line is that device from the start")
}

// The device list is the scan's parent. gdu treats back at the top of the tree
// as "return to the list", and that is why the screen needs no key of its own.
func TestBackAtTheTopOfTheTreeReturnsToTheDisks(t *testing.T) {
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	// Pretend the scan finished.
	scanned := benchModel(3)
	m.topDir, m.currentDir = scanned.topDir, scanned.currentDir
	m.rows = scanned.rows
	m.scr = screenBrowse

	m = press(t, m, "left")
	assert.Equal(t, screenDisks, m.scr)

	// And from the list, enter goes back into a scan — the two are a round trip.
	next, _ := m.Update(key("enter"))
	assert.Equal(t, screenScanning, next.(*model).scr)
}

// Without -d there is no list to go back to, and back at the top must stay put
// rather than blank the screen.
func TestBackAtTheTopWithoutDisksDoesNothing(t *testing.T) {
	m := benchModel(3)
	m.width, m.height, m.haveSize = 80, 20, true
	m.scr = screenBrowse
	require.Nil(t, m.disks)

	m = press(t, m, "left")
	assert.Equal(t, screenBrowse, m.scr, "back at the top of a plain scan is not a way out")
}

func TestDiskCursorClamps(t *testing.T) {
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	m = press(t, m, "up", "up")
	assert.Equal(t, 0, m.diskCursor, "up at the top stays at the top")

	m = press(t, m, "down", "down", "down", "down", "down")
	assert.Equal(t, len(m.disks)-1, m.diskCursor, "down at the bottom stays at the bottom")

	m = press(t, m, "home")
	assert.Equal(t, 0, m.diskCursor)
	m = press(t, m, "end")
	assert.Equal(t, len(m.disks)-1, m.diskCursor)
}

// An empty mount table is not an error, but it is not silence either.
func TestAnEmptyDeviceListSaysSo(t *testing.T) {
	m := disksModel(t, &listGetter{devices: device.Devices{}})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	assert.Equal(t, screenDisks, m.scr)
	assert.Nil(t, m.selectedDisk())
	assert.Contains(t, m.View(), "no mounted devices")

	// And the keys must not panic on it.
	m = press(t, m, "down", "up", "enter", "end")
	assert.Equal(t, screenDisks, m.scr)
}

// The same rule as every other screen: no line wider than the terminal, exactly
// m.height lines, at any size.
func TestDiskScreenFitsTheTerminal(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	for width := 0; width <= 120; width++ {
		for _, height := range []int{1, 2, 3, 8, 24} {
			m := disksModel(t, &listGetter{devices: testDevices()})
			m.width, m.height = width, height
			m.applyDisks(disksCmd(m.ui)().(disksMsg))
			m.width, m.height = width, height

			out := m.View()
			lines := strings.Split(out, "\n")
			assert.Len(t, lines, height, "frame must be %d lines at %dx%d", height, width, height)
			for i, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Errorf("at %dx%d: line %d is %d columns wide", width, height, i, got)
				}
			}
		}
	}
}

// The columns are given up in order as the terminal narrows, and the device name
// is the last thing standing — it is what you are choosing between.
func TestDiskColumnsAreGivenUpInOrder(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	m.width = 100
	wide := m.viewDiskRow(m.disks[0], false)
	assert.Contains(t, wide, "/Volumes/Backup", "a wide terminal shows the mount point")

	m.width = 40
	narrow := m.viewDiskRow(m.disks[0], false)
	assert.NotContains(t, narrow, "/Volumes/Backup", "a narrow one drops it")
	assert.Contains(t, narrow, "4s2", "but never the tail of the device name — see below")
}

// A device name is identified by its end. Cutting from the left, /dev/disk4s1
// and /dev/disk4s2 both come out as "/dev/disk4…" — one string for two disks, in
// the column you are choosing between them by.
func TestANarrowDeviceNameKeepsItsTail(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := disksModel(t, &listGetter{devices: device.Devices{
		{Name: "/dev/disk4s1", MountPoint: "/a", Size: 1 << 40, Free: 1 << 39},
		{Name: "/dev/disk4s2", MountPoint: "/b", Size: 1 << 40, Free: 1 << 39},
	}})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	for _, width := range []int{30, 40, 50, 60} {
		m.width = width
		first := m.viewDiskRow(m.disks[0], false)
		second := m.viewDiskRow(m.disks[1], false)
		assert.NotEqual(t, first, second, "at %d columns two devices render identically", width)
	}
}

// The bug that made this screen unusable: diskSizeWidth was 8 and "460.4 GiB" is
// 9, so padLeft — which pads without clipping — made every row with a real disk
// in it a column wider than its own budget. That tripped the too-narrow
// fallback, which paints the row flat, and the usage bar became a white smear.
// It looked like a colour bug and was arithmetic.
func TestRowsWithRealSizesAreNotFloored(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := disksModel(t, &listGetter{devices: device.Devices{
		// The widest formatSize can go: "1023.9 GiB", ten columns.
		{Name: "/dev/disk3s1s1", Fstype: "apfs", MountPoint: "/", Size: 1023<<30 + 900<<20, Free: 1 << 30},
	}})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	for _, width := range []int{80, 100, 120, 200} {
		m.width = width
		l := m.diskLayout()
		plain := m.diskRowPlain(m.disks[0], &l)
		assert.LessOrEqual(t, 1+runewidth.StringWidth(plain), width,
			"at %d columns the row overflows its own layout by %d",
			width, 1+runewidth.StringWidth(plain)-width)

		if l.bar {
			// A bar drawn cell by cell carries many colours. One flat colour across the
			// whole row is exactly what the bug looked like.
			row := m.viewDiskRow(m.disks[0], false)
			assert.Greater(t, strings.Count(row, "\x1b[38;2;"), 4,
				"at %d columns the bar is not being drawn as a gradient", width)
		}
	}
}

// The cursor row's bar has to stay a gradient and still carry the row's
// background.
//
// Rendering the row whole in the selection's style paints the bar's block
// characters in the selection's foreground — a gradient becomes a white smear,
// which is what it was. Leaving the bar on the terminal's own background instead
// punches a strip of terminal through the middle of the block.
func TestTheCursorRowsBarKeepsItsGradientAndItsBackground(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := disksModel(t, &listGetter{devices: device.Devices{
		{Name: "/dev/disk3s1", Fstype: "apfs", MountPoint: "/", Size: 994 << 30, Free: 100 << 30},
	}})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))
	m.width = 120

	row := m.viewDiskRow(m.disks[0], true)
	require.True(t, m.diskLayout().bar, "this test is pointless without a bar")

	assert.Greater(t, strings.Count(row, "\x1b[38;2;"), 4,
		"the cursor row's bar is one flat colour, not a gradient")

	// #241c34, the charm theme's panel: the bar's cells must carry it too, or the
	// selection is a block with a hole in it.
	assert.Contains(t, row, "48;2;36;28;52", "the bar does not carry the row's background")

	assert.Equal(t, 120, lipgloss.Width(row), "and it still measures exactly the terminal")
}

// The header's volume gauge describes the scan. The device list is not a scan,
// so the gauge has nothing to describe there — and the device you last looked at
// would sit above a table that already has every device's usage in it, claiming
// to be about one of them.
//
// It comes up right on a fresh -d, because nothing has been analyzed yet. The
// bug was on the way *back*: m.dev survives the return, so the gauge stayed.
func TestTheDiskScreenHasNoVolumeGauge(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := disksModel(t, &listGetter{devices: testDevices()})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	assert.False(t, m.showDiskLine(), "nothing has been analyzed yet")
	assert.Equal(t, 2, m.headerHeight(), "brand and rule, no gauge")

	// Analyze a device: now there is a scan, and the gauge describes it.
	next, _ := m.Update(key("enter"))
	m = next.(*model)
	scanned := benchModel(3)
	m.topDir, m.currentDir, m.rows = scanned.topDir, scanned.currentDir, scanned.rows
	m.scr = screenBrowse
	require.NotNil(t, m.dev)
	assert.True(t, m.showDiskLine(), "a scan has a volume to report")
	assert.Equal(t, 3, m.headerHeight())

	// And back to the list, where it must go again.
	m = press(t, m, "left")
	require.Equal(t, screenDisks, m.scr)
	assert.False(t, m.showDiskLine(), "the gauge must not survive the return to the list")
	assert.Equal(t, 2, m.headerHeight())
	assert.NotContains(t, m.View(), "Macintosh", "no stale device in the header")
}

// The fstype answers "what is this thing" — apfs, tmpfs, autofs — and none of
// the other columns do.
func TestTheFilesystemTypeIsShown(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	m := disksModel(t, &listGetter{devices: device.Devices{
		{Name: "/dev/disk3s1", Fstype: "apfs", MountPoint: "/", Size: 1 << 40, Free: 1 << 39},
	}})
	m.applyDisks(disksCmd(m.ui)().(disksMsg))

	m.width = 120
	assert.Contains(t, m.viewDiskRow(m.disks[0], false), "apfs")
	assert.Contains(t, m.viewDisksHeader(), "Type")
}
