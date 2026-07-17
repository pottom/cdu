package charm

import (
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/pkg/device"
)

// `cdu -d` lists the mounted devices so you can pick one before scanning
// anything. It is the first screen, and the scan's parent: pressing back at the
// top of a tree returns here, exactly as it does in gdu.
//
// That is worth keeping rather than inventing a key for. The device list is not
// a mode you toggle — it is where the scan came from, so it sits where every
// other "where I came from" sits, on the back key.

// diskCols are the widths of the fixed columns. The device name and mount point
// take what is left, because they are the only two that vary in length.
const (
	diskSizeWidth  = 8
	diskUsageWidth = 5
	// The usage bar. It is the same renderer the rows use, so a device reads like
	// a directory does.
	diskBarCells = 16

	// Below these the table gives up a column rather than overflow.
	minWidthForDiskUsed  = 52
	minWidthForDiskFree  = 62
	minWidthForDiskBar   = 74
	minWidthForDiskMount = 90

	minDiskNameWidth = 6
)

type disksMsg struct {
	devices device.Devices
	err     error
}

// disksCmd reads the mount table off the render loop. It can block for a long
// time on a stale network mount, and the interface has to come up and say what
// it is waiting for rather than freeze before it draws anything.
func disksCmd(ui *UI) tea.Cmd {
	return func() tea.Msg {
		if ui.getter == nil {
			return disksMsg{err: errNoDeviceGetter}
		}
		devices, err := ui.getter.GetDevicesInfo()
		return disksMsg{devices: devices, err: err}
	}
}

func (m *model) applyDisks(msg disksMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.scr = screenError
		return
	}

	// Biggest first, like everything else here. gdu leaves them in mount-table
	// order, which is the order the kernel happens to hold them in and means
	// nothing to the person looking for the full disk.
	devices := make(device.Devices, len(msg.devices))
	copy(devices, msg.devices)
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].GetUsage() > devices[j].GetUsage()
	})

	m.disks = devices
	m.diskCursor, m.diskOffset = 0, 0
	m.scr = screenDisks
}

// selectedDisk is the device under the cursor, or nil when the list is empty.
func (m *model) selectedDisk() *device.Device {
	if m.diskCursor < 0 || m.diskCursor >= len(m.disks) {
		return nil
	}
	return m.disks[m.diskCursor]
}

// analyzeDisk scans the device under the cursor. The device list stays in
// memory: back at the top of the tree returns to it.
func (m *model) analyzeDisk() (tea.Model, tea.Cmd) {
	dev := m.selectedDisk()
	if dev == nil {
		return m, nil
	}

	m.ui.scanPath = dev.MountPoint
	m.topDir, m.currentDir = nil, nil
	m.rows, m.filtered = nil, nil
	m.cursor, m.offset = 0, 0
	m.dev = dev
	m.status, m.statusIsError = "", false
	m.scr = screenScanning

	return m, tea.Batch(m.spinner.Tick, scanCmd(m.ui), tickCmd())
}

func (m *model) handleDisksKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyUp, "k":
		m.moveDiskCursor(-1)
	case keyDown, "j":
		m.moveDiskCursor(1)
	case keyHome, "g":
		m.moveDiskCursor(-len(m.disks))
	case keyEnd, "G":
		m.moveDiskCursor(len(m.disks))
	case keyPgUp:
		m.moveDiskCursor(-m.visibleLines())
	case keyPgDown:
		m.moveDiskCursor(m.visibleLines())
	case keyEnter, keyRight, "l":
		return m.analyzeDisk()
	case "r":
		return m, disksCmd(m.ui)
	}
	return m, nil
}

func (m *model) moveDiskCursor(delta int) {
	if len(m.disks) == 0 {
		return
	}
	m.diskCursor = min(max(m.diskCursor+delta, 0), len(m.disks)-1)

	// One window, same rules as the list: keep the cursor on screen without
	// scrolling further than it has to.
	height := max(m.visibleLines(), 1)
	m.diskOffset = min(m.diskOffset, m.diskCursor)
	if m.diskCursor >= m.diskOffset+height {
		m.diskOffset = m.diskCursor - height + 1
	}
	m.diskOffset = min(max(m.diskOffset, 0), max(len(m.disks)-height, 0))
}

// diskNameWidth is what the device name and mount point columns share. Both are
// unbounded, so they take whatever the fixed columns leave.
func (m *model) diskNameWidth() int {
	used := 1 + diskSizeWidth // gutter + size
	if m.width >= minWidthForDiskUsed {
		used += 1 + diskSizeWidth
	}
	if m.width >= minWidthForDiskFree {
		used += 1 + diskSizeWidth
	}
	if m.width >= minWidthForDiskBar {
		used += 1 + diskBarCells
	}
	used += 1 + diskUsageWidth

	rest := m.width - used
	if m.width >= minWidthForDiskMount {
		// Split what is left between the name and the mount point. The mount point
		// is the more useful of the two — /Volumes/Backup says what a disk is for,
		// /dev/disk4s2 does not — so it gets the larger half.
		return max(rest/3, minDiskNameWidth)
	}
	return max(rest, minDiskNameWidth)
}

func (m *model) viewDisksHeader() string {
	if m.width < 1 {
		return ""
	}
	nameWidth := m.diskNameWidth()

	line := " " + runewidth.FillRight(runewidth.Truncate("Device", nameWidth, "…"), nameWidth)
	line += " " + padLeft("Size", diskSizeWidth)
	if m.width >= minWidthForDiskUsed {
		line += " " + padLeft("Used", diskSizeWidth)
	}
	if m.width >= minWidthForDiskFree {
		line += " " + padLeft("Free", diskSizeWidth)
	}
	if m.width >= minWidthForDiskBar {
		line += " " + runewidth.FillRight("Usage", diskBarCells)
	}
	line += " " + padLeft("%", diskUsageWidth)
	if m.width >= minWidthForDiskMount {
		mountWidth := max(m.width-runewidth.StringWidth(line)-1, 1)
		line += " " + runewidth.FillRight(runewidth.Truncate("Mounted on", mountWidth, "…"), mountWidth)
	}
	return m.st.dim.Render(clipTo(line, m.width))
}

// viewDiskRow is one device. It is composed as plain text at an exact width and
// styled after, like every other row here — runewidth counts escape bytes as
// columns, so a styled string cut to the terminal loses most of itself.
func (m *model) viewDiskRow(dev *device.Device, selected bool) string {
	if m.width < 1 {
		return ""
	}
	nameWidth := m.diskNameWidth()
	frac := 0.0
	if dev.Size > 0 {
		frac = float64(dev.GetUsage()) / float64(dev.Size)
	}

	name := runewidth.FillRight(runewidth.Truncate(dev.Name, nameWidth, "…"), nameWidth)
	sizeText := padLeft(m.ui.formatSize(dev.Size), diskSizeWidth)

	rest := ""
	if m.width >= minWidthForDiskUsed {
		rest += " " + padLeft(m.ui.formatSize(dev.GetUsage()), diskSizeWidth)
	}
	if m.width >= minWidthForDiskFree {
		rest += " " + padLeft(m.ui.formatSize(dev.Free), diskSizeWidth)
	}

	pct := padLeft(formatPct(dev.GetUsage(), dev.Size), diskUsageWidth)

	mount := ""
	if m.width >= minWidthForDiskMount {
		fixed := 1 + nameWidth + 1 + diskSizeWidth + runewidth.StringWidth(rest) + 1 + diskUsageWidth
		if m.width >= minWidthForDiskBar {
			fixed += 1 + diskBarCells
		}
		mountWidth := max(m.width-fixed-1, 1)
		mount = " " + runewidth.FillRight(runewidth.Truncate(dev.MountPoint, mountWidth, "…"), mountWidth)
	}

	// The plain row, in full. Every column here has a floor, and on a narrow
	// enough terminal they add up to more than there is — so this is measured
	// first and the row is clipped whole when it will not fit, exactly as a file
	// row is. Composing it anyway would overflow and wrap the frame.
	plain := name + " " + sizeText + rest
	if m.width >= minWidthForDiskBar {
		plain += " " + m.bar.plainCells(frac, diskBarCells)
	}
	plain += " " + pct + mount
	floored := 1+runewidth.StringWidth(plain) > m.width

	if selected {
		// One column: the marker alone. There is no room for it and anything to mark.
		if m.width < 2 {
			return m.st.accent.Render("▌")
		}
		return m.st.accent.Render("▌") + m.st.selected.Render(clipTo(plain, m.width-1))
	}
	if floored {
		return m.st.dirName.Render(clipTo(" "+plain, m.width))
	}

	out := " " + m.st.dirName.Render(name) +
		" " + m.st.size.Render(sizeText) +
		m.st.dim.Render(rest)
	if m.width >= minWidthForDiskBar {
		out += " " + m.bar.render(frac, diskBarCells)
	}
	out += " " + m.st.pct.Render(pct) + m.st.dim.Render(mount)
	return out
}

// viewDiskList is exactly visibleLines() lines: the column header, then the
// window onto the devices.
func (m *model) viewDiskList() string {
	lines := m.visibleLines()

	if len(m.disks) == 0 {
		return padLines(m.st.dim.Render(clipTo("  no mounted devices to show", m.width)), lines)
	}
	// Too short for a column header and a device both. The devices are the point;
	// the header is a label on them.
	if lines < 2 {
		return padLines(m.viewDiskRow(m.disks[m.diskCursor], true), lines)
	}

	height := lines - 1
	end := min(m.diskOffset+height, len(m.disks))
	rows := make([]string, 0, height)
	for i := m.diskOffset; i < end; i++ {
		rows = append(rows, m.viewDiskRow(m.disks[i], i == m.diskCursor))
	}
	return m.viewDisksHeader() + "\n" + padLines(joinLines(rows), height)
}

// viewDisks has the same shape as viewBrowse, and gives up its chrome at the
// same sizes: the interface must not appear to change identity depending on
// which screen a short terminal happens to be on.
func (m *model) viewDisks() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewDiskList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
