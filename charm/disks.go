package charm

import (
	"fmt"
	"strings"

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

const (
	// diskSizeWidth is sizeColWidth for the same reason: "1023.9 GiB" is the
	// widest string formatSize can make, and it is ten columns. It was eight
	// once, and since padLeft pads without clipping, every row with a size over
	// 99.9 was quietly a column too wide — which tripped the too-narrow fallback
	// and painted the whole row, usage bar included, in one flat colour. It read
	// as a colour bug and was arithmetic.
	diskSizeWidth = sizeColWidth
	// diskTypeWidth holds the longest filesystem name worth showing: squashfs and
	// overlayfs on Linux, autofs on macOS.
	diskTypeWidth = 8
	diskPctWidth  = 5
	// diskBarCells is the usage bar, drawn by the same renderer the file rows use
	// so that a device reads like a directory.
	diskBarCells = 16

	minDiskNameWidth  = 8
	minDiskMountWidth = 10
)

// diskLayout is which columns fit and how wide the two elastic ones are.
//
// It is derived from the widths themselves rather than from a table of
// breakpoint constants. Breakpoints have to be kept in step with the widths by
// hand, and when they drift the row overflows its own budget silently — which is
// exactly how the bar turned white.
type diskLayout struct {
	name  int
	mount int // 0 when there is no room for the mount point
	used  bool
	free  bool
	ftype bool
	bar   bool
}

// treeGlyphs are the shapes the tree is drawn with. They cost two columns, and
// they are what says a volume belongs to the disk above it rather than being a
// disk of its own.
const (
	treeWidth  = 2
	treeBranch = "├ "
	treeLast   = "└ "

	asciiTreeBranch = "| "
	asciiTreeLast   = "` "
)

// treePrefix is the two columns before a device's icon: the branch it hangs off,
// or nothing when it hangs off nothing.
func (m *model) treePrefix(r *diskRow) string {
	if r.depth == 0 {
		return ""
	}
	branch, last := treeBranch, treeLast
	if m.ui.noUnicode {
		branch, last = asciiTreeBranch, asciiTreeLast
	}
	if r.last {
		return last
	}
	return branch
}

func (m *model) diskLayout() diskLayout {
	l := diskLayout{}

	// The core every width keeps: gutter, name, size, percent. A device you cannot
	// name is not a device you can choose, and the percentage is the answer to the
	// question the screen exists for.
	fixed := 1 + 1 + diskSizeWidth + 1 + diskPctWidth

	// Then each column while there is still room for a readable name, in the order
	// they stop being worth their columns. The bar goes before the mount point:
	// it is decoration for a percentage that is already there in figures, and
	// "/Volumes/Backup" says what a disk is *for* in a way /dev/disk4s2 never does.
	for _, opt := range []struct {
		cost int
		on   *bool
	}{
		{1 + diskSizeWidth, &l.used},
		{1 + diskSizeWidth, &l.free},
		{1 + diskTypeWidth, &l.ftype},
		{1 + diskBarCells, &l.bar},
	} {
		if m.width-fixed-opt.cost >= minDiskNameWidth {
			*opt.on = true
			fixed += opt.cost
		}
	}

	// The Device column carries the tree branch and the icon as well as the name,
	// so it is given their width on top of its share. Paying for the indent out of
	// the name is how /dev/disk3s1 and /dev/disk3s5 both became "/de…3s1" — the
	// same string for two volumes, in the column you tell them apart by.
	decor := treeWidth
	if m.width >= minWidthForIcon {
		decor += iconWidth
	}

	rest := m.width - fixed
	if rest >= minDiskNameWidth+decor+1+minDiskMountWidth {
		// Both elastic columns fit. The mount point gets the larger share of what is
		// left, for the same reason it outranks the bar.
		l.name = max(rest/3+decor, minDiskNameWidth+decor)
		l.mount = rest - l.name - 1
		if l.mount < minDiskMountWidth {
			l.mount = minDiskMountWidth
			l.name = rest - l.mount - 1
		}
		return l
	}
	l.name = max(rest, minDiskNameWidth)
	return l
}

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

	m.disks = msg.devices
	m.diskRows = groupDisks(msg.devices)
	m.diskCursor, m.diskOffset = 0, 0
	m.moveDiskCursor(0) // land on something selectable rather than on a header
	m.scr = screenDisks
}

// selectedDisk is the device under the cursor, or nil when there is none — an
// empty table, or a cursor that has nowhere selectable to be.
func (m *model) selectedDisk() *device.Device {
	if m.diskCursor < 0 || m.diskCursor >= len(m.diskRows) {
		return nil
	}
	return m.diskRows[m.diskCursor].dev
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

	cmd := m.startScan()
	return m, cmd
}

func (m *model) handleDisksKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyUp, "k":
		m.moveDiskCursor(-1)
	case keyDown, "j":
		m.moveDiskCursor(1)
	case keyHome, "g":
		m.moveDiskCursor(-len(m.diskRows))
	case keyEnd, "G":
		m.moveDiskCursor(len(m.diskRows))
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

// moveDiskCursor moves by delta and lands on something selectable.
//
// A disk header is not selectable: a container has no mount point, so there is
// nothing to analyze. Stopping on one would be a cursor that does nothing when
// you press enter, so the cursor steps over them — in the direction of travel,
// and then back the other way if it ran out of table.
func (m *model) moveDiskCursor(delta int) {
	if len(m.diskRows) == 0 {
		return
	}
	want := min(max(m.diskCursor+delta, 0), len(m.diskRows)-1)

	step := 1
	if delta < 0 {
		step = -1
	}
	m.diskCursor = m.nextSelectableDisk(want, step)

	// One window, same rules as the list: keep the cursor on screen without
	// scrolling further than it has to. The header above it is worth keeping in
	// view — a volume with no disk over it is the thing the tree exists to avoid —
	// so scrolling up stops one row early where it can.
	height := max(m.visibleLines(), 1)
	top := m.diskCursor
	if top > 0 && m.diskRows[top-1].isHeader() {
		top--
	}
	m.diskOffset = min(m.diskOffset, top)
	if m.diskCursor >= m.diskOffset+height {
		m.diskOffset = m.diskCursor - height + 1
	}
	m.diskOffset = min(max(m.diskOffset, 0), max(len(m.diskRows)-height, 0))
}

// nextSelectableDisk finds the nearest row that can actually be analyzed,
// looking in the direction of travel first.
func (m *model) nextSelectableDisk(from, step int) int {
	for _, dir := range []int{step, -step} {
		for i := from; i >= 0 && i < len(m.diskRows); i += dir {
			if !m.diskRows[i].isHeader() {
				return i
			}
		}
	}
	return from // nothing but headers, which cannot happen: a header has volumes
}

// cell fits a string into exactly n columns, left-aligned. Both halves matter:
// padding alone lets a long value push the row over its budget, and truncating
// alone lets a short one pull the next column left.
func cell(s string, n int) string {
	if n < 1 {
		return ""
	}
	return runewidth.FillRight(runewidth.Truncate(s, n, "…"), n)
}

// cellRight is cell, right-aligned, for figures.
func cellRight(s string, n int) string {
	if n < 1 {
		return ""
	}
	return padLeft(runewidth.Truncate(s, n, ""), n)
}

// cellPath is cell for a device name or a mount point, which are the two things
// here whose *end* is what identifies them. Cut from the left, /dev/disk4s1 and
// /dev/disk4s2 both become "/dev/disk4…" — the same string for two different
// disks, in the one column you are choosing between them by.
func cellPath(s string, n int) string {
	if n < 1 {
		return ""
	}
	return runewidth.FillRight(middleTruncate(s, n), n)
}

func (m *model) viewDisksHeader() string {
	if m.width < 1 {
		return ""
	}
	l := m.diskLayout()

	line := " " + cell("Device", l.name) + " " + cellRight("Size", diskSizeWidth)
	if l.used {
		line += " " + cellRight("Used", diskSizeWidth)
	}
	if l.free {
		line += " " + cellRight("Free", diskSizeWidth)
	}
	if l.ftype {
		line += " " + cell("Type", diskTypeWidth)
	}
	if l.bar {
		line += " " + cell("Usage", diskBarCells)
	}
	line += " " + cellRight("%", diskPctWidth)
	if l.mount > 0 {
		line += " " + cell("Mounted on", l.mount)
	}
	return m.st.dim.Render(clipTo(line, m.width))
}

// diskNameCell is the Device column: the tree branch, the icon, and the name, in
// exactly l.name columns however much of it there is.
//
// The column's total width is fixed even though the prefix is not — a volume
// spends two of its columns on the branch and has that much less name, which is
// what an indent *is*. Letting the column grow instead would push Size two
// columns right on every volume and the table would stagger.
func (m *model) diskNameCell(r *diskRow, width int) string {
	prefix := m.treePrefix(r)
	icon := m.diskIcon(r)
	rest := width - runewidth.StringWidth(prefix) - runewidth.StringWidth(icon)
	return prefix + icon + cellPath(diskRowName(r), max(rest, 0))
}

// diskRowName is what goes in the Device column.
//
// A device under a disk header loses its /dev/ — five columns of prefix that
// every row has and none of them needs, in the column that is already paying for
// the tree and the icon. The tree says which disk it is on, which is the only
// thing /dev/ was contributing. Anything ungrouped keeps its name whole, because
// there is nothing above it to supply the context: `tmpfs` and `map auto_home`
// are the name.
func diskRowName(r *diskRow) string {
	if r.dev == nil {
		return r.disk
	}
	if r.depth > 0 {
		if short, ok := strings.CutPrefix(r.dev.Name, "/dev/"); ok {
			return short
		}
	}
	return r.dev.Name
}

// diskRowPlain is the row as bare text, at exactly the width the layout says.
// Everything is measured here, where the string's length is what it looks like;
// the styled version below is built from the same pieces, never measured.
func (m *model) diskRowPlain(r *diskRow, l *diskLayout) string {
	dev := r.dev
	s := m.diskNameCell(r, l.name) + " " + cellRight(m.ui.formatSize(dev.Size), diskSizeWidth)
	if l.used {
		s += " " + cellRight(m.ui.formatSize(dev.GetUsage()), diskSizeWidth)
	}
	if l.free {
		s += " " + cellRight(m.ui.formatSize(dev.Free), diskSizeWidth)
	}
	if l.ftype {
		s += " " + cell(dev.Fstype, diskTypeWidth)
	}
	if l.bar {
		s += " " + m.bar.plainCells(diskFrac(dev), diskBarCells)
	}
	s += " " + cellRight(formatPct(dev.GetUsage(), dev.Size), diskPctWidth)
	if l.mount > 0 {
		s += " " + cellPath(dev.MountPoint, l.mount)
	}
	return s
}

func diskFrac(dev *device.Device) float64 {
	if dev.Size <= 0 {
		return 0
	}
	return float64(dev.GetUsage()) / float64(dev.Size)
}

// viewDiskHeaderRow is a physical disk: a label over its volumes, with no
// numbers of its own.
//
// It has none to give. A container is not in the mount table — only what is
// mounted from it is — so there is no size to report here that is not already on
// the rows below. What it can say is *why* those rows look the way they do: four
// volumes each honestly reporting 460 GiB are one pool seen four times, and
// saying so is the entire reason this row exists.
func (m *model) viewDiskHeaderRow(r *diskRow) string {
	l := m.diskLayout()

	note := fmt.Sprintf("%d volume", r.volumes)
	if r.volumes != 1 {
		note += "s"
	}
	if r.shared {
		note += " sharing one pool of space"
	}

	name := m.diskNameCell(r, l.name)
	gap := max(m.width-1-runewidth.StringWidth(name)-1-runewidth.StringWidth(note), 0)
	if gap == 0 {
		// No room to say why. The name still marks where the group starts.
		return m.st.accent.Render(clipTo(" "+name, m.width))
	}
	return m.st.accent.Render(" "+name) + strings.Repeat(" ", gap+1) + m.st.dim.Render(note)
}

// viewDiskRow is one row of the table, composed as plain text at an exact width
// and styled after — runewidth counts escape bytes as columns, so a styled
// string cut to the terminal loses most of itself.
func (m *model) viewDiskRow(r *diskRow, selected bool) string {
	if m.width < 1 {
		return ""
	}
	if r.isHeader() {
		return m.viewDiskHeaderRow(r)
	}

	dev := r.dev
	l := m.diskLayout()

	if selected {
		if m.width < 2 {
			return m.st.accent.Render("▌") // no room for a marker and anything to mark
		}
		return m.viewSelectedDiskRow(r, &l)
	}

	// Narrower than the columns' own floors add up to. Clip whole rather than
	// overflow — but without the bar: painting block characters in a text colour
	// is what made it a white smear rather than a bar.
	if 1+runewidth.StringWidth(m.diskRowPlain(r, &l)) > m.width {
		flat := l
		flat.bar = false
		return m.st.dirName.Render(clipTo(" "+m.diskRowPlain(r, &flat), m.width))
	}

	out := " " + m.st.dirName.Render(m.diskNameCell(r, l.name)) +
		" " + m.st.size.Render(cellRight(m.ui.formatSize(dev.Size), diskSizeWidth))
	if l.used {
		out += " " + m.st.dim.Render(cellRight(m.ui.formatSize(dev.GetUsage()), diskSizeWidth))
	}
	if l.free {
		out += " " + m.st.dim.Render(cellRight(m.ui.formatSize(dev.Free), diskSizeWidth))
	}
	if l.ftype {
		out += " " + m.st.dim.Render(cell(dev.Fstype, diskTypeWidth))
	}
	if l.bar {
		out += " " + m.bar.render(diskFrac(dev), diskBarCells)
	}
	out += " " + m.st.pct.Render(cellRight(formatPct(dev.GetUsage(), dev.Size), diskPctWidth))
	if l.mount > 0 {
		out += " " + m.st.dim.Render(cellPath(dev.MountPoint, l.mount))
	}
	return out
}

// viewSelectedDiskRow is the cursor row, and it is composed rather than clipped
// whole for one reason: the bar.
//
// Rendering the whole row in the selection's style paints the bar's block
// characters in the selection's foreground, and a gradient becomes a white
// smear. Leaving the bar on the terminal's own background instead punches a
// strip of terminal through the middle of the block. So the bar is drawn from
// its own ramp, built on the panel — the row keeps its background and the bar
// keeps its gradient.
//
// The file list dodges this by putting the bar on a line of its own; a table
// with a bar column cannot.
func (m *model) viewSelectedDiskRow(r *diskRow, l *diskLayout) string {
	dev := r.dev
	// Everything left of the bar, and everything right of it, as plain text at
	// exact widths — measured before styling, as always.
	left := m.diskNameCell(r, l.name) + " " + cellRight(m.ui.formatSize(dev.Size), diskSizeWidth)
	if l.used {
		left += " " + cellRight(m.ui.formatSize(dev.GetUsage()), diskSizeWidth)
	}
	if l.free {
		left += " " + cellRight(m.ui.formatSize(dev.Free), diskSizeWidth)
	}
	if l.ftype {
		left += " " + cell(dev.Fstype, diskTypeWidth)
	}

	right := " " + cellRight(formatPct(dev.GetUsage(), dev.Size), diskPctWidth)
	if l.mount > 0 {
		right += " " + cellPath(dev.MountPoint, l.mount)
	}

	// The marker takes the first column; the rest has m.width-1 to live in.
	budget := m.width - 1
	if !l.bar {
		return m.st.accent.Render("▌") + m.st.selected.Render(clipTo(left+right, budget))
	}

	// The bar's own cells are exact, so the two text halves are clipped around it
	// rather than the whole row being clipped after the fact.
	barWidth := min(diskBarCells, max(budget-runewidth.StringWidth(left)-1-runewidth.StringWidth(right), 0))
	leftWidth := max(budget-barWidth-1-runewidth.StringWidth(right), 0)

	return m.st.accent.Render("▌") +
		m.st.selected.Render(clipTo(left, leftWidth)) +
		m.st.selected.Render(" ") +
		m.bar.renderSelected(diskFrac(dev), barWidth) +
		m.st.selected.Render(clipTo(right, max(budget-leftWidth-barWidth-1, 0)))
}

// viewDiskList is exactly visibleLines() lines: the column header, then the
// window onto the devices.
func (m *model) viewDiskList() string {
	lines := m.visibleLines()

	if len(m.diskRows) == 0 {
		return padLines(m.st.dim.Render(clipTo("  no mounted devices to show", m.width)), lines)
	}
	// Too short for a column header and a device both. The devices are the point;
	// the header is a label on them.
	if lines < 2 {
		return padLines(m.viewDiskRow(&m.diskRows[m.diskCursor], true), lines)
	}

	height := lines - 1
	end := min(m.diskOffset+height, len(m.diskRows))
	rows := make([]string, 0, height)
	for i := m.diskOffset; i < end; i++ {
		rows = append(rows, m.viewDiskRow(&m.diskRows[i], i == m.diskCursor))
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
