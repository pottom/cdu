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
	screenError
)

// progressInterval is how often the scan progress is sampled. The analyzer
// exposes progress by polling (GetProgress) rather than pushing, so this is a
// ticker rather than a subscription.
const progressInterval = 100 * time.Millisecond

// blinkTicks is the cursor's half-period, counted in progress samples: the mock
// blinks it once a second.
const blinkTicks = 5

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
	rows   []fs.Item
	cursor int
	offset int

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
	p := charmPalette()
	st := newStyles(p, ui.UseColors)
	sp.Style = st.accent

	return &model{
		ui:      ui,
		spinner: sp,
		st:      st,
		bar:     newBarRenderer(p, ui.UseColors, ui.noUnicode),
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

	case tickMsg:
		if m.scr != screenScanning {
			return m, nil
		}
		m.progress = m.ui.Analyzer.GetProgress()
		m.ticks++
		m.blinkOn = (m.ticks/blinkTicks)%2 == 0
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

	switch msg.String() {
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "home", "g":
		m.cursor = 0
		m.clampCursor()
	case "end", "G":
		m.cursor = len(m.rows) - 1
		m.clampCursor()
	case "pgup":
		m.moveCursor(-m.visibleRows())
	case "pgdown":
		m.moveCursor(m.visibleRows())
	case "right", "l", "enter":
		m.descend()
	case "left", "h", "backspace":
		m.ascend()
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

// enterDir materialises a directory's children and resets the selection.
func (m *model) enterDir(dir fs.Item) {
	m.currentDir = dir
	m.rows = m.rows[:0]
	for item := range dir.GetFiles(m.ui.sortBy, m.ui.sortOrder) {
		m.rows = append(m.rows, item)
	}
	m.cursor = 0
	m.offset = 0
}

func (m *model) selected() fs.Item {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	return m.rows[m.cursor]
}

func (m *model) moveCursor(delta int) {
	m.cursor += delta
	m.clampCursor()
}

// clampCursor keeps the cursor in range and scrolls the window to follow it.
// Every size here is derived from the current terminal, never hardcoded, so a
// resize mid-scroll reflows instead of leaving the selection off screen.
func (m *model) clampCursor() {
	if len(m.rows) == 0 {
		m.cursor, m.offset = 0, 0
		return
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > len(m.rows)-1 {
		m.cursor = len(m.rows) - 1
	}

	visible := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	maxOffset := len(m.rows) - visible
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
