package charm

import (
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/common"
	"github.com/pottom/cdu/pkg/device"
	"github.com/pottom/cdu/pkg/fs"
)

type screen int

const (
	screenScanning screen = iota
	screenBrowse
	screenConfirm
	screenViewer
	screenError
)

// progressInterval is how often the scan progress is sampled. The analyzer
// exposes progress by polling (GetProgress) rather than pushing, so this is a
// ticker rather than a subscription.
const progressInterval = 100 * time.Millisecond

// blinkTicks is the cursor's half-period, counted in progress samples: the mock
// blinks it once a second.
const blinkTicks = 5

// keyEscape and keyEnter back out of and accept the modes and modals. They are
// the two keys that always mean the same thing, which is why nothing else may be
// bound to them.
const (
	keyEscape    = "esc"
	keyEnter     = "enter"
	keyBackspace = "backspace"
	keyLeft      = "left"
)

type model struct {
	ui *UI

	width, height int
	haveSize      bool

	scr screen
	err error

	topDir     fs.Item
	currentDir fs.Item

	// dev is the volume the scan root lives on. Nil until it resolves, and nil
	// forever if it cannot be resolved — the disk line is decoration, so its
	// absence must never hold up the interface.
	dev *device.Device

	// rows is the current directory's children, materialised once per directory
	// rather than re-derived every frame: GetFiles returns an iterator, and
	// walking it on each render would put sorting on the hot path.
	// rows is every child of the current directory, in sort order. filtered is the
	// subset the fuzzy filter lets through — nil when no filter is active — and is
	// what the cursor, the window and every key actually operate on. The filter is
	// a view: it never touches the tree, so a delete under a filter still acts on
	// the real item.
	rows     []fs.Item
	filtered []fs.Item
	cursor   int
	offset   int

	// filtering is the / input mode; filter is the query so far.
	filtering bool
	filter    string

	// maxRowSize is the largest row in the current directory, measured when the
	// directory is entered. --show-relative-size draws the bars against it.
	maxRowSize int64

	spinner  spinner.Model
	progress common.CurrentProgress

	// ticks counts progress samples, which is also what drives the cursor blink —
	// one clock rather than two, so the scan line cannot beat against itself.
	ticks   int
	blinkOn bool

	// st is resolved once. It is several kilobytes of Lipgloss styles; rebuilding
	// or copying it per row would put it squarely on the render hot path.
	st styles

	// bar probes the terminal's colour profile when it is built, not per frame.
	bar barRenderer

	// confirm is the pending destructive operation, nil when there is none.
	confirm *confirmState

	// lastTrashed is what undo would put back. Only a trashed item can come back,
	// so a permanent delete leaves this nil — there is simply nothing to restore.
	lastTrashed *trashed

	// status is the last thing that happened, shown in the footer until the next
	// keystroke. A destructive action that reports nothing is indistinguishable
	// from one that silently failed.
	status        string
	statusIsError bool

	// pending is the item currently being removed from disk. Removal runs off the
	// render loop and can take seconds on a large tree, during which the row would
	// otherwise sit there looking untouched — indistinguishable from a key that
	// never registered. The row spins instead, and refuses further keys.
	pending fs.Item

	// frames is the spinner used both by the scan screen and by a row being
	// deleted, so the two never disagree about what "working" looks like.
	frames []string

	// sortPending and colPending mean a menu key has been pressed and the second
	// key is being chosen. They are modes, so the footer has to say so — a mode
	// nobody can see is a trap.
	sortPending bool
	colPending  bool

	// viewer holds the file being read with v: its lines, the scroll offset, and
	// whether the read was capped. Nil when not viewing.
	viewer *viewerState
}

type (
	scanDoneMsg struct{ dir fs.Item }
	scanErrMsg  struct{ err error }
	tickMsg     struct{}
	deviceMsg   struct{ dev *device.Device }
)

func newModel(ui *UI) *model {
	frames := []string{"◐", "◓", "◑", "◒"}
	if ui.noUnicode {
		frames = []string{"|", "/", "-", "\\"}
	}
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{Frames: frames, FPS: time.Second / 8}
	st := newStyles(&ui.theme, ui.UseColors)
	sp.Style = st.accent

	return &model{
		ui:      ui,
		spinner: sp,
		frames:  frames,
		st:      st,
		bar:     newBarRenderer(&ui.theme, ui.UseColors, ui.noUnicode),
		blinkOn: true,
	}
}

func (m *model) Init() tea.Cmd {
	// A saved scan opened with -f is already in memory; there is nothing to walk.
	if m.ui.topDir != nil {
		m.enterDir(m.ui.topDir)
		m.topDir = m.ui.topDir
		m.scr = screenBrowse
		return deviceCmd(m.ui)
	}
	return tea.Batch(m.spinner.Tick, scanCmd(m.ui), tickCmd(), deviceCmd(m.ui))
}

// deviceCmd resolves the volume the scan root sits on. It runs off the render
// loop because reading the mount table can block — on a stale network mount, for
// a long time — and the interface must come up regardless.
func deviceCmd(ui *UI) tea.Cmd {
	return func() tea.Msg {
		if ui.getter == nil {
			return nil
		}
		mounts, err := ui.getter.GetDevicesInfo()
		if err != nil {
			// The disk line is decoration. A machine that will not report its
			// mounts still gets a working browser, just without it.
			return nil
		}
		return deviceMsg{dev: deviceFor(ui.rootPath(), mounts)}
	}
}

// deviceFor picks the volume a path lives on: the mount point that is the
// longest prefix of it. Longest wins because mount points nest — /home is not
// the right answer for a path under /home/me/vault when that is its own mount.
func deviceFor(path string, mounts device.Devices) *device.Device {
	var best *device.Device
	for _, mount := range mounts {
		if !strings.HasPrefix(path, mount.MountPoint) {
			continue
		}
		// "/var" must not match "/variable": only a component boundary counts.
		rest := path[len(mount.MountPoint):]
		if rest != "" && !strings.HasPrefix(rest, string(filepath.Separator)) &&
			!strings.HasSuffix(mount.MountPoint, string(filepath.Separator)) {
			continue
		}
		if best == nil || len(mount.MountPoint) > len(best.MountPoint) {
			best = mount
		}
	}
	return best
}

// scanCmd runs the blocking walk off the render loop.
func scanCmd(ui *UI) tea.Cmd {
	return func() tea.Msg {
		defer debug.FreeOSMemory()
		dir := ui.Analyzer.AnalyzeDir(ui.scanPath, ui.CreateIgnoreFunc(), ui.CreateFileTypeFilter())
		if dir == nil {
			return scanErrMsg{err: notYetInCharmUI("scanning " + ui.scanPath)}
		}
		dir.UpdateStats(ui.linkedItems)
		return scanDoneMsg{dir: dir}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(progressInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.haveSize = true
		m.clampCursor()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tickMsg:
		// The same clock drives the scan line and a deleting row. Nothing is ticking
		// when neither is happening, so an idle cdu wakes up for nothing.
		if m.scr != screenScanning && m.pending == nil {
			return m, nil
		}
		m.ticks++
		if m.scr == screenScanning {
			m.progress = m.ui.Analyzer.GetProgress()
			m.blinkOn = (m.ticks/blinkTicks)%2 == 0
		}
		return m, tickCmd()

	case scanDoneMsg:
		m.topDir = msg.dir
		m.enterDir(msg.dir)
		m.scr = screenBrowse
		return m, nil

	case scanErrMsg:
		m.err = msg.err
		m.scr = screenError
		return m, nil

	case deviceMsg:
		m.dev = msg.dev
		return m, nil

	case fileLoadedMsg:
		m.applyFileLoaded(msg)
		return m, nil

	case deleteDoneMsg:
		cmd := m.applyDelete(msg)
		return m, cmd

	case undoDoneMsg:
		cmd := m.applyUndo(msg)
		return m, cmd

	case spinner.TickMsg:
		if m.scr != screenScanning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The modal takes every key, including q: while a delete is being confirmed,
	// q is a letter of the word being typed, not a way out of the program.
	if m.scr == screenConfirm {
		return m.handleConfirmKey(msg)
	}
	// The file viewer has its own keys, and closes rather than quits.
	if m.scr == screenViewer {
		return m.handleViewerKey(msg)
	}

	// The status line reports the last thing that happened, so it lives exactly as
	// long as the user's attention is still on it.
	m.status, m.statusIsError = "", false

	// A menu or the filter input takes the next key whatever it is, including q: a
	// mode that let some keys through would be a mode you could not trust.
	switch {
	case m.filtering:
		return m.handleFilterKey(msg)
	case m.sortPending:
		m.handleSortKey(msg.String())
		return m, nil
	case m.colPending:
		m.handleColumnKey(msg.String())
		return m, nil
	}

	switch msg.String() {
	case "q", "ctrl+c":
		// The analyzer exposes no cancellation — no context, no Stop. Quitting
		// mid-scan therefore tears down the program and lets the walk goroutine
		// die with the process, which is what gdu effectively does too.
		return m, tea.Quit
	}

	if m.scr != screenBrowse {
		return m, nil
	}
	return m.handleBrowseKey(msg)
}

// handleBrowseKey is the browse screen's key table: moving, opening, and the keys
// that act on the selected item.
func (m *model) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "home", "g":
		m.cursor = 0
		m.clampCursor()
	case "end", "G":
		m.cursor = len(m.items()) - 1
		m.clampCursor()
	case "pgup":
		m.moveCursor(-m.visibleRows())
	case "pgdown":
		m.moveCursor(m.visibleRows())
	case "right", "l", "enter":
		m.descend()
	case keyLeft, "h", keyBackspace:
		m.ascend()
	case "d":
		m.askConfirm(actionTrash)
	case "D":
		m.askConfirm(actionDelete)
	case "e":
		m.askConfirm(actionEmpty)
	case "v":
		return m.openViewer()
	case "u":
		cmd := m.askUndo()
		return m, cmd
	case "r":
		cmd := m.rescan()
		return m, cmd
	case "/":
		m.openFilter()
	case "s":
		m.sortPending = true
	case "t":
		m.colPending = true
	case "a", "B", "c", "m":
		// gdu binds these directly, and so do we: the t menu exists to make them
		// discoverable, not to make them harder to reach for anyone who knows them.
		m.handleToggle(msg.String())
	}
	return m, nil
}

func (m *model) descend() {
	item := m.selected()
	if item == nil || !item.IsDir() {
		return
	}
	m.enterDir(item)
}

func (m *model) ascend() {
	if m.currentDir == nil {
		return
	}
	parent := m.currentDir.GetParent()
	if parent == nil {
		return
	}
	child := m.currentDir
	m.enterDir(parent)
	// Land on the directory we came out of rather than at the top.
	for i, r := range m.rows {
		if r == child {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
}

// enterDir materialises a directory's children and resets the selection. The
// largest row is measured here, once, rather than in View — with
// --show-relative-size the bars are drawn against it, and finding it every frame
// would walk all ten thousand rows and undo the virtualization.
//
// The filter is per-directory, so navigating clears it: carrying "nmd" from one
// directory into the next would hide the new directory for no reason the user
// could see.
func (m *model) enterDir(dir fs.Item) {
	m.currentDir = dir
	m.rows = m.rows[:0]
	m.maxRowSize = 0
	for item := range dir.GetFiles(m.ui.sortBy, m.ui.sortOrder) {
		m.maxRowSize = max(m.maxRowSize, m.itemSize(item))
		m.rows = append(m.rows, item)
	}
	m.filtering, m.filter, m.filtered = false, "", nil
	m.cursor = 0
	m.offset = 0
}

// items is the list the cursor and window move over: the filtered subset when a
// filter is active, otherwise every row. A filtered slice is non-nil even when it
// matches nothing, so an over-narrow filter shows an empty list rather than the
// whole directory.
func (m *model) items() []fs.Item {
	if m.filtered != nil {
		return m.filtered
	}
	return m.rows
}

func (m *model) selected() fs.Item {
	items := m.items()
	if m.cursor < 0 || m.cursor >= len(items) {
		return nil
	}
	return items[m.cursor]
}

func (m *model) moveCursor(delta int) {
	m.cursor += delta
	m.clampCursor()
}

// clampCursor keeps the cursor in range and scrolls the window to follow it.
// Every size here is derived from the current terminal, never hardcoded, so a
// resize mid-scroll reflows instead of leaving the selection off screen.
func (m *model) clampCursor() {
	items := m.items()
	if len(items) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(items)-1 {
		m.cursor = len(items) - 1
	}

	visible := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	maxOffset := len(items) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
