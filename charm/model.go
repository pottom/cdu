package charm

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/common"
	"github.com/pottom/cdu/internal/dup"
	"github.com/pottom/cdu/internal/theme"
	"github.com/pottom/cdu/pkg/device"
	"github.com/pottom/cdu/pkg/fs"
)

type screen int

const (
	screenScanning screen = iota
	screenBrowse
	screenConfirm
	screenViewer
	screenDisks
	screenTop
	// screenQueue is the delete queue: everything marked for removal, so a batch
	// delete can be looked over before it happens rather than fired blind.
	screenQueue
	// screenThemes is the theme picker (p): the list re-themes live as the cursor
	// moves, so the picker is its own preview.
	screenThemes
	screenHelp
	// screenHashing is the spinner while dup.Find reads the candidate files. It is
	// its own screen, not the scan spinner, because it comes back with a different
	// message and cancels back to a different place.
	screenHashing
	// screenDup is the duplicate groups, once found.
	screenDup
	// screenFind is the results of a filename search (f).
	screenFind
	screenError
)

// progressInterval is how often the scan progress is sampled. The analyzer
// exposes progress by polling (GetProgress) rather than pushing, so this is a
// ticker rather than a subscription.
const progressInterval = 100 * time.Millisecond

// blinkTicks is the cursor's half-period, counted in progress samples: the mock
// blinks it once a second.
const blinkTicks = 5

// The keys that mean the same thing on every screen. keyEscape and keyEnter in
// particular always back out of and accept, which is why nothing else may be
// bound to them; the rest are named because more than one screen has a list to
// move around in, and a list is a list.
const (
	keyEscape    = "esc"
	keyEnter     = "enter"
	keyBackspace = "backspace"
	keyLeft      = "left"
	keyRight     = "right"
	keyUp        = "up"
	keyDown      = "down"
	keyHome      = "home"
	keyEnd       = "end"
	keyPgUp      = "pgup"
	keyPgDown    = "pgdown"
	keyCtrlC     = "ctrl+c"
)

type model struct {
	ui *UI

	width, height int
	haveSize      bool

	scr screen
	err error

	topDir     fs.Item
	currentDir fs.Item

	// landOnPath is the path to put the cursor on once the next scan lands, used when
	// ascending above the scan root: the parent is rescanned, and the cursor returns
	// to the directory it came out of. Empty for an ordinary scan, which opens at the
	// top of its list.
	landOnPath string

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

	// finding is the f input mode; findQuery is the pattern being typed. findResults
	// is the tree-wide match it produces, findPattern the pattern that produced it
	// (kept for the header). f is find, / is filter — different tools, different
	// names.
	finding     bool
	findQuery   string
	findResults fs.Files
	findPattern string
	findCursor  int
	findOffset  int

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
	// confirmFrom is the screen the modal was opened from, and the screen closing
	// it returns to. A delete asked for in the largest-files list must not drop
	// you into the browser when you cancel it.
	confirmFrom screen

	// lastTrashed is what undo would put back, one entry per item of the last trash.
	// A single delete is a batch of one, so undo has one code path; a batch delete
	// leaves several, and undo replays them. Only a trashed item can come back, so a
	// permanent delete leaves this empty — there is nothing to restore.
	lastTrashed []*trashed

	// marked is the set queued for a batch delete, keyed by item so a mark survives
	// moving between directories — gdu keys its marks by row and loses them the
	// moment the list reorders. Empty means the destructive keys act on the cursor
	// row alone, exactly as before.
	marked map[fs.Item]bool
	// queue is the marked set as a flat, biggest-first list for the queue screen. It
	// is a snapshot taken when the screen opens, like topFiles.
	queue       []fs.Item
	queueCursor int
	queueOffset int

	// deleteRemaining is the tail of a batch delete still to run: removals go one at
	// a time so two never race to resize the same parent, so the batch is a queue
	// drained one applyDelete at a time. deleteAct is the action the batch is doing;
	// batchTotal and batchFail count the run so its end can be reported honestly.
	// batchTotal is zero for an ordinary single delete, which keeps its own wording.
	deleteRemaining []fs.Item
	deleteAct       action
	batchTotal      int
	batchFail       int
	// elevateCandidates are the items in a batch that a plain delete could not remove
	// for want of permission; once the run is done they are offered to one sudo pass.
	elevateCandidates []fs.Item

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
	// viewerFrom is the screen v was pressed on, and the one closing returns to.
	viewerFrom screen

	// disks is the mount table, when cdu was started with -d. It is kept for the
	// life of the program rather than re-read: it is the scan's parent, and back
	// at the top of the tree returns to it. Nil means cdu was not started with -d,
	// which is also what makes back at the top do nothing instead.
	disks device.Devices
	// diskRows is that table as a tree — a row per physical disk, then a row per
	// device on it. The cursor indexes this, not disks: what is on screen is what
	// the arrow keys move over.
	diskRows   []diskRow
	diskCursor int
	diskOffset int

	// cancelling means esc was pressed during a scan and the walk is unwinding.
	// It cannot stop at once — what is already open still has to finish — so the
	// screen says so, and the tree that eventually arrives is thrown away.
	cancelling bool

	// topFiles is the biggest files anywhere in the scan (T). It is a snapshot,
	// taken when the key is pressed rather than kept up to date: it is a question
	// you ask, not a view that has to stay true.
	topFiles  fs.Files
	topCursor int
	topOffset int

	// homeDir is the user's home, resolved once, for shortening title paths to ~.
	homeDir string

	// latestVersion is the tag of a newer release, once the startup check finds one.
	// Empty means no newer version is known — not yet checked, up to date, or the
	// check could not run — and the header shows no update mark.
	latestVersion string

	// helpFrom is the screen ? was pressed on, and the one it returns to. The
	// help is reachable from all of them.
	helpFrom screen
	// helpOffset is the first help line drawn; helpCursor is the selected binding,
	// an index into helpEntries(). The cursor drives the detail pane and the offset
	// follows it, so the selected row is always on screen.
	helpOffset int
	helpCursor int

	// dupGroups is the result of the last duplicate search (F): sets of
	// byte-identical files, most reclaimable first. dupRows is that flattened for
	// the screen — a header per group, then its files. dupMarked is every file in
	// a group, for the ▲ the browser draws beside it.
	dupGroups []dup.Group
	dupRows   []dupRow
	// dupMarked maps a browser file to its duplicate group, so the ▲ can be drawn
	// and the cursor's row can say how many copies it has. A bool would draw the
	// mark but could not explain it.
	dupMarked map[fs.Item]*dup.Group
	dupCursor int
	dupOffset int

	// themeNames is every theme the picker (p) lists; themeOriginal is the theme it
	// opened on, restored if you esc out without keeping a choice. Moving the cursor
	// applies a theme live, so the picker itself is the preview.
	themeNames    []string
	themeCursor   int
	themeOriginal theme.Theme
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

	// Resolved once: a title path is home-shortened on every frame, and
	// os.UserHomeDir is a syscall not worth repeating. An error leaves it empty,
	// which shortPath treats as "no home to collapse" — the full path, not a crash.
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}

	return &model{
		ui:      ui,
		spinner: sp,
		frames:  frames,
		st:      st,
		bar:     newBarRenderer(&ui.theme, ui.UseColors, ui.noUnicode),
		blinkOn: true,
		homeDir: home,
		marked:  make(map[fs.Item]bool),
		// Where the modal and the viewer return to when they close. They are set
		// again on the way in, but the zero value of a screen is screenScanning —
		// so anything that missed a step would close onto the scan screen, which is
		// not a place you can be. The browser is the answer to "back" for everything
		// that does not say otherwise.
		confirmFrom: screenBrowse,
		viewerFrom:  screenBrowse,
		// Anything worth saying about the theme or the config is said here rather
		// than on stderr, which the alternate screen would wipe before it could be
		// read.
		status:        ui.notice(),
		statusIsError: ui.noticeIsError,
	}
}

func (m *model) Init() tea.Cmd {
	// The update check rides along with whatever the interface opens on: it runs off
	// the render loop and lands a message later, so it never delays the first frame.
	update := m.checkUpdate()

	// -d opens on the device list: there is no path to walk until one is picked.
	if m.ui.showDisks {
		m.scr = screenScanning
		return tea.Batch(m.spinner.Tick, disksCmd(m.ui), update)
	}
	// A saved scan opened with -f is already in memory; there is nothing to walk.
	if m.ui.topDir != nil {
		m.enterDir(m.ui.topDir)
		m.topDir = m.ui.topDir
		m.scr = screenBrowse
		return tea.Batch(deviceCmd(m.ui), update)
	}
	return tea.Batch(m.startScan(), deviceCmd(m.ui), update)
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
// startScan resets the analyzer and returns the commands that walk a tree.
//
// The reset is not housekeeping. An analyzer's done-channel is *closed* when its
// walk finishes, and Broadcast is a close — so a second AnalyzeDir on the same
// analyzer closes a closed channel, which is a panic rather than an error. Only
// the very first scan works without this; gdu calls ResetProgress before every
// one of its own for the same reason.
//
// It runs here, on the render loop, rather than inside the command: Init swaps
// the analyzer's channels, and the loop reads its progress on every tick.
//
// Every scan goes through this function. Adding another call to scanCmd that
// does not is the whole bug, so there is no reason to have one.
func (m *model) startScan() tea.Cmd {
	m.ui.Analyzer.ResetProgress()
	m.ui.cancel.Store(false)
	m.cancelling = false
	m.scr = screenScanning
	m.progress = common.CurrentProgress{}
	return tea.Batch(m.spinner.Tick, scanCmd(m.ui), tickCmd())
}

func scanCmd(ui *UI) tea.Cmd {
	return func() tea.Msg {
		defer debug.FreeOSMemory()
		dir := ui.Analyzer.AnalyzeDir(ui.scanPath, ui.ignoreFunc(), ui.fileTypeFilter())
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
		if m.cancelling {
			return m.afterCancel()
		}
		m.topDir = msg.dir
		m.enterDir(msg.dir)
		m.scr = screenBrowse
		if m.landOnPath != "" {
			// Ascended above the old root: put the cursor back on the directory we came
			// out of, which is now one row among the parent's children.
			for i, r := range m.rows {
				if r.GetPath() == m.landOnPath {
					m.cursor = i
					break
				}
			}
			m.clampCursor()
			m.landOnPath = ""
			// The scan root moved, so its volume — and the header's disk gauge — may
			// have too.
			return m, deviceCmd(m.ui)
		}
		return m, nil

	case scanErrMsg:
		m.err = msg.err
		m.scr = screenError
		return m, nil

	case deviceMsg:
		m.dev = msg.dev
		return m, nil

	case spinner.TickMsg:
		if m.scr != screenScanning && m.scr != screenHashing {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m.handleResultMsg(msg)
}

// handleResultMsg takes the messages that carry the result of work done off the
// render loop. They are split out from Update only because it outgrew its length
// budget; there is no other line between them.
func (m *model) handleResultMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fileLoadedMsg:
		m.applyFileLoaded(msg)
		return m, nil

	case viewSavedMsg:
		m.applyViewSaved(msg)
		return m, nil

	case disksMsg:
		m.applyDisks(msg)
		return m, nil

	case dupDoneMsg:
		return m.applyDupDone(msg)

	case deleteDoneMsg:
		cmd := m.applyDelete(msg)
		return m, cmd

	case undoDoneMsg:
		cmd := m.applyUndo(msg)
		return m, cmd

	case elevatedDoneMsg:
		cmd := m.applyElevatedDelete(msg)
		return m, cmd

	case openedMsg:
		m.applyOpened(msg)
		return m, nil

	case updateAvailableMsg:
		// The background check found a newer release; the header now shows it.
		m.latestVersion = msg.tag
		return m, nil
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
	case m.finding:
		return m.handleFindInputKey(msg)
	case m.sortPending:
		m.handleSortKey(msg.String())
		return m, nil
	case m.colPending:
		cmd := m.handleColumnKey(msg.String())
		return m, cmd
	}

	if m.scr == screenScanning {
		return m.handleScanningKey(msg)
	}
	if m.scr == screenHelp {
		return m.handleHelpKey(msg)
	}
	// ? reaches the help from anywhere it is not already a letter being typed —
	// the modal, the filter and the menus are all handled above, and each of them
	// takes every key whole.
	if msg.String() == "?" {
		return m.openHelp()
	}

	switch msg.String() {
	case "q", keyCtrlC:
		return m, tea.Quit
	}

	if m.scr == screenDisks {
		return m.handleDisksKey(msg)
	}
	if m.scr == screenTop {
		return m.handleTopKey(msg)
	}
	if m.scr == screenQueue {
		return m.handleQueueKey(msg)
	}
	if m.scr == screenThemes {
		return m.handleThemeKey(msg)
	}
	if m.scr == screenHashing {
		return m.handleHashingKey(msg)
	}
	if m.scr == screenDup {
		return m.handleDupKey(msg)
	}
	if m.scr == screenFind {
		return m.handleFindKey(msg)
	}
	if m.scr != screenBrowse {
		return m, nil
	}
	return m.handleBrowseKey(msg)
}

// handleBrowseKey is the browse screen's movement, with everything that acts on
// the list or the selection handed to handleBrowseAction — two tables rather than
// one over-long one.
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
		cmd := m.descend()
		return m, cmd
	case keyLeft, "h", keyBackspace:
		cmd := m.ascend()
		return m, cmd
	default:
		return m.handleBrowseAction(msg)
	}
	return m, nil
}

// handleBrowseAction is the keys that change something: delete and undo, the marks
// and the queue, the other screens, the filter and the two menus.
func (m *model) handleBrowseAction(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d":
		m.askConfirm(actionTrash)
	case "D":
		m.askConfirm(actionDelete)
	case "e":
		m.askConfirm(actionEmpty)
	case "v":
		return m.openViewer()
	case "o":
		return m.openFile()
	case "u":
		cmd := m.askUndo()
		return m, cmd
	case "T":
		return m.collectTopFiles()
	case "f":
		m.openFind()
	case "F":
		return m.findDuplicates()
	case "r":
		cmd := m.rescan()
		return m, cmd
	case "/":
		m.openFilter()
	case "s":
		m.sortPending = true
	case "t":
		m.colPending = true
	case " ":
		// Space queues the row for a batch delete and steps down, so a run of rows is
		// marked by holding it — the whole reason marking beats deleting one at a time.
		m.markUnderCursor()
		m.moveCursor(1)
	case "M":
		return m.openQueue()
	case "p":
		return m.openThemePicker()
	case keyEscape:
		// Nothing is deeper than the browser to back out of, so esc here cancels the
		// selection instead — the whole marked set at once, the way esc drops any
		// other pending state. With nothing marked it does nothing, quietly.
		m.unmarkAll()
	case "a", "B", "c", "m":
		// gdu binds these directly, and so do we: the t menu exists to make them
		// discoverable, not to make them harder to reach for anyone who knows them.
		m.handleToggle(msg.String())
	}
	return m, nil
}

func (m *model) descend() tea.Cmd {
	item := m.selected()
	if item == nil {
		return nil
	}
	// → on the ../ row goes up, not in: it is the parent, and entering it is the one
	// move it stands for.
	if m.isParentRow(item) {
		return m.ascend()
	}
	if !item.IsDir() {
		return nil
	}
	m.enterDir(item)
	return nil
}

func (m *model) ascend() tea.Cmd {
	if m.currentDir == nil {
		return nil
	}
	parent := m.currentDir.GetParent()
	if parent == nil {
		// At the top of the scanned tree. If cdu was started with -d, the device list
		// is where this scan came from, so it is what "back" means — the same rule gdu
		// follows, and the reason the list needs no key of its own.
		if m.disks != nil {
			m.scr = screenDisks
			m.status, m.statusIsError = "", false
			return nil
		}
		// Otherwise "up" means the directory above the scan root, which is not in the
		// tree — so it is scanned, one level up. This is how you walk out of a scan
		// you rooted too deep without restarting cdu.
		return m.ascendOnDisk()
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
	return nil
}

// canAscendOnDisk reports whether ← would leave the scanned tree and walk the
// parent on disk: we are at the scan root, not in -d's device list, the scan came
// from a path rather than a file, and that path has a parent to climb to.
func (m *model) canAscendOnDisk() bool {
	if m.currentDir == nil || m.currentDir.GetParent() != nil || m.disks != nil || m.ui.scanPath == "" {
		return false
	}
	p := m.currentDir.GetPath()
	return filepath.Dir(p) != p
}

// ascendOnDisk rescans the directory above the scan root, so navigation can leave
// the subtree cdu was started in. It is a fresh scan one level up — the parent is
// not in the tree to walk into — and it lands the cursor on the old root, so where
// you came from is where the eye already is.
func (m *model) ascendOnDisk() tea.Cmd {
	if m.ui.scanPath == "" {
		// A saved scan opened with -f: there is no path on disk to walk.
		m.status, m.statusIsError = "this view was read from a file; there is no parent to scan", true
		return nil
	}
	current := m.currentDir.GetPath()
	parent := filepath.Dir(current)
	if parent == current {
		m.status, m.statusIsError = "already at the top of the filesystem", true
		return nil
	}

	// Land on the old root once the parent's listing is up, and walk the parent with
	// a fresh hard-link ledger, the way any new scan starts.
	m.landOnPath = current
	m.ui.scanPath = parent
	m.ui.linkedItems = make(fs.HardLinkedItems, 10)
	return m.startScan()
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
	// With folders-first on, directories float to the top, keeping the engine's
	// order within each group — a stable partition, not a re-sort. It is the file
	// manager's habit rather than the disk usage default: off, the biggest thing
	// leads whether it is a folder or a file, which is the "what is eating my
	// disk" answer. On, it is the "where do I go" answer instead.
	if m.ui.foldersFirst {
		sort.SliceStable(m.rows, func(i, j int) bool {
			return m.rows[i].IsDir() && !m.rows[j].IsDir()
		})
	}
	// A ../ row leads the list whenever there is a parent in the tree, added after
	// the sort so it never sinks into it. It is the real parent item — → enters it
	// and the cursor can rest on it — but it is not a child: it carries no bar and
	// cannot be deleted or marked. The cursor opens on the first real row, not on
	// the way out.
	m.cursor = 0
	if dir.GetParent() != nil {
		m.rows = append([]fs.Item{dir.GetParent()}, m.rows...)
		if len(m.rows) > 1 {
			m.cursor = 1
		}
	}
	m.filtering, m.filter, m.filtered = false, "", nil
	m.offset = 0
}

// isParentRow reports whether an item is the ../ row — the current directory's
// parent, shown at the head of the list. It is a real directory but not one of the
// current directory's children, so every action that treats the list as children
// (delete, mark, the usage bar) has to let it out.
func (m *model) isParentRow(item fs.Item) bool {
	return item != nil && m.currentDir != nil && item == m.currentDir.GetParent()
}

// items is the list the cursor and window move over: the filtered subset when a
// filter is active, otherwise every row. A filtered slice is non-nil even when it
// matches nothing, so an over-narrow filter shows an empty list rather than the
// whole directory.
// searchRoot is the subtree the whole-tree analyses (T, F) work over: the
// directory you are standing in, so they act on what you are looking at rather
// than always the whole scan. At the top of the tree that is the scan root, so
// nothing changes there. It falls back to the scan root when there is no current
// directory — a saved scan not yet entered, say.
func (m *model) searchRoot() fs.Item {
	if m.currentDir != nil {
		return m.currentDir
	}
	return m.topDir
}

// searchScopeSuffix names the subtree a T/F result covers: the full path it
// searched under, home-shortened. It always says where, root or not, so the
// title answers "what am I looking at, and from where" without the reader having
// to remember which directory they were in when they pressed the key.
func (m *model) searchScopeSuffix() string {
	root := m.searchRoot()
	if root == nil {
		return ", any depth"
	}
	return " under " + m.shortPath(root.GetPath()) + ", any depth"
}

// shortPath collapses the home directory to ~, the way a person names a path.
func (m *model) shortPath(path string) string {
	if m.homeDir != "" && path == m.homeDir {
		return "~"
	}
	if m.homeDir != "" && strings.HasPrefix(path, m.homeDir+"/") {
		return "~" + path[len(m.homeDir):]
	}
	return path
}

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
