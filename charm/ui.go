// Package charm implements cdu's default interactive interface on the Charm
// stack (Bubble Tea, Lipgloss, Bubbles).
//
// It sits alongside gdu's original tview interface in tui/, which stays
// reachable via --classic. Both satisfy app.UI and drive the same analyzer, so
// nothing in the scanning engine is duplicated here.
package charm

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/common"
	"github.com/pottom/cdu/internal/theme"
	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/device"
	"github.com/pottom/cdu/pkg/fs"
	"github.com/pottom/cdu/report"
)

// UI is the Bubble Tea interface. Embedding common.UI is what makes this an
// in-tree fork rather than a module consumer: it hands us the analyzer field
// and gdu's whole ignore-pattern engine, and satisfies most of app.UI for free.
type UI struct {
	*common.UI
	output      io.Writer
	linkedItems fs.HardLinkedItems

	// scanPath is recorded by AnalyzePath and walked inside the Bubble Tea loop,
	// so progress arrives as messages rather than blocking startup.
	scanPath string
	// topDir is set ahead of the loop when a saved scan is opened with -f.
	topDir fs.Item

	// getter backs the header's disk line. It is optional: without it, or on a
	// path that belongs to no listed mount, the line is simply not drawn.
	getter device.DevicesInfoGetter
	// showDisks means -d: open on the device list rather than scanning a path.
	showDisks bool

	noUnicode  bool
	noDelete   bool
	noViewFile bool
	mouse      bool
	// icons draws Nerd Font glyphs in the icon cell instead of the plain markers.
	// Off unless asked for: the glyphs need a patched font, and cdu cannot tell
	// whether one is loaded.
	icons bool

	// Optional columns, off by default and toggled with c and m.
	showItemCount bool
	showMtime     bool

	sortBy    fs.SortBy
	sortOrder fs.SortOrder

	// save persists the view (t then s). It is a callback because charm cannot see
	// the config struct — cmd/cdu/app imports charm, not the reverse — and a writer
	// that only knew the fields here would drop the rest of the file. Nil means
	// saving is unavailable, and the key says so.
	save func(ViewSettings) (string, error)

	// theme supplies every colour the renderer uses. The constructor plants the
	// default, so no render path has to ask whether a theme was configured.
	theme theme.Theme

	// notices are things to tell the user once the interface is up: a theme that
	// could not be honoured, a config being read from gdu's path. They land on the
	// status line rather than stderr because cdu opens the alternate screen
	// immediately, and anything printed before that is wiped before it can be read.
	notices []string
	// noticeIsError is true when any notice is a complaint rather than a remark.
	noticeIsError bool
}

func (ui *UI) addNotice(s string, isError bool) {
	if s == "" {
		return
	}
	ui.notices = append(ui.notices, s)
	ui.noticeIsError = ui.noticeIsError || isError
}

// notice is every notice as one status line.
func (ui *UI) notice() string {
	return strings.Join(ui.notices, "; ")
}

// Option customises the UI.
type Option func(*UI)

// CreateUI builds the Charm interface.
func CreateUI(
	output io.Writer,
	useColors bool,
	showApparentSize bool,
	showRelativeSize bool,
	useSIPrefix bool,
	opts ...Option,
) *UI {
	ui := &UI{
		UI: &common.UI{
			UseColors:        useColors,
			ShowApparentSize: showApparentSize,
			ShowRelativeSize: showRelativeSize,
			UseSIPrefix:      useSIPrefix,
			Analyzer:         analyze.CreateAnalyzer(),
		},
		output:      output,
		linkedItems: make(fs.HardLinkedItems, 10),
		sortBy:      fs.SortBySize,
		sortOrder:   fs.SortDesc,
		theme:       theme.Charm(),
	}
	for _, o := range opts {
		o(ui)
	}

	// Sorting by size has to mean the size on screen, exactly as toggling the
	// column at runtime makes sure of. Without this, a config carrying both
	// show-apparent-size and sorting.by: size would open ordered by disk usage
	// while showing apparent size — the very inconsistency handleToggle guards
	// against, arriving through the front door instead.
	if ui.sortBy == fs.SortBySize && ui.ShowApparentSize {
		ui.sortBy = fs.SortByApparentSize
	}
	return ui
}

// UseOldSizeBar switches to ASCII runes for terminals without unicode.
func UseOldSizeBar() Option {
	return func(ui *UI) { ui.noUnicode = true }
}

// WithDeviceGetter supplies the mount table the header's disk line is drawn from.
func WithDeviceGetter(getter device.DevicesInfoGetter) Option {
	return func(ui *UI) { ui.getter = getter }
}

// WithTheme resolves the config's theme block against --theme and installs the
// result.
//
// A problem in either is carried into the interface as a status line rather than
// printed and lost: cdu opens the alternate screen immediately, which would wipe
// anything written to stderr before a user could read it.
func WithTheme(cfg *theme.Config, name string) Option {
	return func(ui *UI) {
		th, err := theme.Resolve(cfg, name)
		ui.theme = th
		if err != nil {
			// errors.Join separates with newlines, and the status line is one line.
			ui.addNotice(strings.ReplaceAll(err.Error(), "\n", "; "), true)
		}
	}
}

// WithNotice adds a remark to show on the status line once the interface is up.
func WithNotice(s string) Option {
	return func(ui *UI) { ui.addNotice(s, false) }
}

// WithWarnings adds complaints to show on the status line. Unlike a notice these
// are things the user asked for and did not get — a theme file of theirs that
// would not load — so they are coloured as errors.
func WithWarnings(warnings ...string) Option {
	return func(ui *UI) {
		for _, w := range warnings {
			ui.addNotice(w, true)
		}
	}
}

// SetNoDelete disables every destructive key. The keys stay bound and say they
// are disabled: a key that silently does nothing reads as a broken interface.
func (ui *UI) SetNoDelete() {
	ui.noDelete = true
}

// SetNoViewFile disables the file viewer (v), which then says it is disabled
// rather than doing nothing.
func (ui *UI) SetNoViewFile() {
	ui.noViewFile = true
}

// SetMouse enables mouse reporting (--mouse). It is off by default so that
// terminal text selection keeps working for anyone who does not ask for it.
func (ui *UI) SetMouse() {
	ui.mouse = true
}

// SetIcons draws Nerd Font glyphs in the icon cell (--icons). Off by default:
// without a patched font every row would start with a box, and cdu has no way to
// ask the terminal what font it has.
func (ui *UI) SetIcons() {
	ui.icons = true
}

// SetShowItemCount starts with the item-count column on (-C).
func (ui *UI) SetShowItemCount() {
	ui.showItemCount = true
}

// SetShowMTime starts with the mtime column on (-M).
func (ui *UI) SetShowMTime() {
	ui.showMtime = true
}

// rootPath is the absolute path the scan is rooted at, which is what the disk
// line is resolved against.
func (ui *UI) rootPath() string {
	path := ui.scanPath
	if path == "" && ui.topDir != nil {
		path = ui.topDir.GetPath()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

// AnalyzePath records the path to walk. The walk itself runs as a Bubble Tea
// command so the spinner and progress line stay live while it works.
func (ui *UI) AnalyzePath(path string, _ fs.Item) error {
	ui.scanPath = path
	return nil
}

// ReadAnalysis opens a scan previously exported with -o.
func (ui *UI) ReadAnalysis(input io.Reader) error {
	dir, err := report.ReadAnalysis(input)
	if err != nil {
		return err
	}
	dir.UpdateStats(ui.linkedItems)
	ui.topDir = dir
	return nil
}

// ReadFromStorage is not implemented in the Charm interface yet.
func (ui *UI) ReadFromStorage(_, _ string) error {
	return notYetInCharmUI("--read-from-storage")
}

// ListDevices records that cdu was started with -d. The mount table itself is
// read inside the Bubble Tea loop.
//
// gdu reads it here, before its interface exists. That is fine until a mount is
// stale, and then the terminal simply sits there with nothing on it — the read
// can block for a long time and there is no interface yet to say so. Deferring
// it costs the error return this signature offers, which is a fair trade: the
// failure is shown on the error screen instead of on stderr.
func (ui *UI) ListDevices(getter device.DevicesInfoGetter) error {
	ui.getter = getter
	ui.showDisks = true
	return nil
}

// errNoDeviceGetter is what -d hits when no mount-table reader was supplied,
// which is a wiring mistake rather than anything the user did.
var errNoDeviceGetter = errors.New("no device getter: cdu cannot read the mount table")

// SetCollapsePath is accepted but not yet honoured by the Charm interface.
func (ui *UI) SetCollapsePath(bool) {}

// program builds the Bubble Tea program. Tests drive it with injected input and
// output so the loop can run without a TTY.
func (ui *UI) program(opts ...tea.ProgramOption) *tea.Program {
	base := []tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithOutput(ui.output),
	}
	if ui.mouse {
		// Cell motion, not all motion: we only care about clicks and the wheel, and
		// all-motion floods the loop with events for no benefit here.
		base = append(base, tea.WithMouseCellMotion())
	}
	return tea.NewProgram(newModel(ui), append(base, opts...)...)
}

// StartUILoop runs the Bubble Tea program.
func (ui *UI) StartUILoop() error {
	_, err := ui.program().Run()
	return err
}

func notYetInCharmUI(what string) error {
	return fmt.Errorf("%s is not available in the Charm interface yet; run with --classic", what)
}
