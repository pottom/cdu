// Package charm implements cdu's default interactive interface on the Charm
// stack (Bubble Tea, Lipgloss, Bubbles).
//
// It sits alongside gdu's original tview interface in tui/, which stays
// reachable via --classic. Both satisfy app.UI and drive the same analyzer, so
// nothing in the scanning engine is duplicated here.
package charm

import (
	"fmt"
	"io"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/common"
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

	noUnicode bool
	noDelete  bool
	sortBy    fs.SortBy
	sortOrder fs.SortOrder
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
	}
	for _, o := range opts {
		o(ui)
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

// SetNoDelete disables every destructive key. The keys stay bound and say they
// are disabled: a key that silently does nothing reads as a broken interface.
func (ui *UI) SetNoDelete() {
	ui.noDelete = true
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

// ListDevices is not implemented in the Charm interface yet.
func (ui *UI) ListDevices(_ device.DevicesInfoGetter) error {
	return notYetInCharmUI("-d / --show-disks")
}

// SetCollapsePath is accepted but not yet honoured by the Charm interface.
func (ui *UI) SetCollapsePath(bool) {}

// program builds the Bubble Tea program. Tests drive it with injected input and
// output so the loop can run without a TTY.
func (ui *UI) program(opts ...tea.ProgramOption) *tea.Program {
	return tea.NewProgram(newModel(ui), append([]tea.ProgramOption{
		tea.WithAltScreen(),
		tea.WithOutput(ui.output),
	}, opts...)...)
}

// StartUILoop runs the Bubble Tea program.
func (ui *UI) StartUILoop() error {
	_, err := ui.program().Run()
	return err
}

func notYetInCharmUI(what string) error {
	return fmt.Errorf("%s is not available in the Charm interface yet; run with --classic", what)
}
