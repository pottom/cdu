package charm

import (
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/pkg/fs"
)

// `o` opens the selected file in whatever application the OS opens it with —
// the image viewer for an image, the PDF reader for a PDF. It is the companion to
// `v`, which reads a file inside cdu: `v` is for a quick look at text without
// leaving, `o` is for handing the file to the tool that is actually meant for it.
//
// Directories are not opened this way — → already enters them — and cdu never
// blocks on the launcher: open, xdg-open and start all hand the file to the desktop
// and return, so the tree stays live.

type openedMsg struct {
	name string
	err  error
}

// openCommand builds the platform's "open with the default application" command.
// goos is a parameter rather than read here so the mapping can be tested for every
// platform from any one of them.
func openCommand(goos, path string) *exec.Cmd {
	switch goos {
	case "darwin":
		return exec.Command("open", path)
	case "windows":
		// The empty "" is start's title argument. Without it, start reads a quoted
		// path as the window title and opens nothing.
		return exec.Command("cmd", "/c", "start", "", path)
	default:
		// Linux, the BSDs, everything else with a freedesktop opener.
		return exec.Command("xdg-open", path)
	}
}

func openFileCmd(item fs.Item) tea.Cmd {
	return func() tea.Msg {
		err := openCommand(runtime.GOOS, item.GetPath()).Run()
		return openedMsg{name: item.GetName(), err: err}
	}
}

// openFile hands the selected file to its default application. It works from every
// list that v works from, so it reads the cursor through target() rather than the
// browser's own selection.
func (m *model) openFile() (tea.Model, tea.Cmd) {
	item, _ := m.target()
	if item == nil {
		return m, nil
	}
	if item.IsDir() {
		// A directory has no "default app"; → is how you go into one, and saying so
		// is better than opening a file manager over the top of cdu.
		m.status, m.statusIsError = "→ opens a directory; o opens a file in its app", true
		return m, nil
	}
	m.status, m.statusIsError = "opening "+item.GetName()+"…", false
	return m, openFileCmd(item)
}

func (m *model) applyOpened(msg openedMsg) {
	if msg.err != nil {
		m.status, m.statusIsError = "could not open "+msg.name+": "+msg.err.Error(), true
		return
	}
	m.status, m.statusIsError = "opened "+msg.name, false
}
