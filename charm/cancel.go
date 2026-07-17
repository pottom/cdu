package charm

import (
	tea "github.com/charmbracelet/bubbletea"
)

// esc stops a scan; q still quits.
//
// esc means "out of this" everywhere else in cdu — out of a mode, out of a
// modal, out of the viewer — and a scan is a state you can want out of. Making q
// mean cancel here instead would give it a different meaning on one screen than
// on every other, which is exactly how a binding stops being trustworthy.

func (m *model) handleScanningKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape:
		m.cancelScan()
		return m, nil
	case "q", keyCtrlC:
		// Stop the walk on the way out. The process is about to end and would take
		// the goroutine with it either way, but a walk that keeps reading the disk
		// while cdu tears down is work nobody asked for.
		m.ui.cancel.Store(true)
		return m, tea.Quit
	}
	return m, nil
}

// cancelScan asks the walk to stop.
//
// It cannot stop at once: the directories already open still have to finish, and
// the analyzer has no way to be interrupted mid-directory. So this sets the flag
// and says so on screen, and the tree is dealt with when it arrives.
func (m *model) cancelScan() {
	if m.cancelling {
		return
	}
	m.cancelling = true
	m.ui.cancel.Store(true)
}

// afterCancel leaves the screen the cancelled scan was filling.
//
// The partial tree is thrown away rather than shown. The directories the walk
// never opened are simply *absent* from it — not marked, not empty, absent — so
// every parent above them reports less than it holds. A disk usage tool quietly
// showing sizes that are too small is worse than one showing nothing, and the
// next key along is `d`.
//
// Where "back" is depends on what the scan interrupted, which is the same
// question the back key answers everywhere else:
func (m *model) afterCancel() (tea.Model, tea.Cmd) {
	m.cancelling = false
	m.ui.cancel.Store(false)
	m.status, m.statusIsError = "scan cancelled", false

	switch {
	case m.currentDir != nil:
		// A rescan. The tree from before is still there and still true — rescan is
		// careful not to throw it away until a new one arrives.
		m.scr = screenBrowse
		return m, nil
	case m.disks != nil:
		// A device picked from `cdu -d`. The list is where it came from.
		m.scr = screenDisks
		return m, nil
	}
	// The first scan of a path given on the command line. There is nothing behind
	// it: cancelling the only thing cdu was asked to do leaves nothing to show.
	return m, tea.Quit
}
