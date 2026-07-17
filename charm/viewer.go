package charm

import (
	"bytes"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

// The file viewer is v: a read-only pager for the selected file. It reuses the
// list's own windowed scrolling rather than bubbles/viewport, so the exact-height
// invariant that keeps the frame from scrolling holds here too.
//
// Two things a naive "read the file and show it" would get wrong, and this does
// not: a multi-gigabyte file must not be pulled into memory, and a binary file
// must not be dumped as mojibake. So the read is capped and sniffed.

const (
	// viewerReadCap bounds how much of a file is read. A pager is for looking, not
	// for loading a database into RAM; past this the viewer says it is truncated.
	viewerReadCap = 1 << 20 // 1 MiB

	// binarySniffLen is how much of the head is checked for NUL bytes. A NUL in a
	// text file is vanishingly rare; a NUL in the first few KB of a binary is
	// almost certain.
	binarySniffLen = 8000
)

type viewerState struct {
	path      string
	lines     []string
	offset    int
	truncated bool // the file was longer than the read cap
}

// fileLoadedMsg carries the read back to the render loop.
type fileLoadedMsg struct {
	path      string
	lines     []string
	truncated bool
	err       error
	binary    bool
}

// openViewer starts reading the selected file, or explains why it will not.
func (m *model) openViewer() (tea.Model, tea.Cmd) {
	// target rather than selected: v works from the largest-files list too, where
	// the row under the cursor is not a row of the browser's list.
	item, _ := m.target()
	if item == nil {
		return m, nil
	}
	// Where closing the viewer goes back to.
	m.viewerFrom = m.scr
	if m.ui.noViewFile {
		// Disabled, but not silent: a key that does nothing reads as broken.
		m.status, m.statusIsError = "viewing files is disabled (--no-view-file)", true
		return m, nil
	}
	if item.IsDir() {
		m.status, m.statusIsError = "→ opens the directory; v views a file", true
		return m, nil
	}

	m.status, m.statusIsError = "opening "+item.GetName()+"…", false
	return m, readFileCmd(item.GetPath())
}

// readFileCmd reads the file off the render loop: it can be large, or on a slow or
// stalled disk, and the interface must stay responsive meanwhile.
func readFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(path)
		if err != nil {
			return fileLoadedMsg{path: path, err: err}
		}
		defer f.Close()

		// One byte past the cap distinguishes "exactly the cap" from "longer".
		data, err := io.ReadAll(io.LimitReader(f, viewerReadCap+1))
		if err != nil {
			return fileLoadedMsg{path: path, err: err}
		}

		truncated := false
		if len(data) > viewerReadCap {
			data = data[:viewerReadCap]
			truncated = true
		}

		if isBinary(data) {
			return fileLoadedMsg{path: path, binary: true}
		}

		return fileLoadedMsg{
			path:      path,
			lines:     splitLines(data),
			truncated: truncated,
		}
	}
}

// isBinary reports whether the head of the data contains a NUL byte, which text
// does not and binaries almost always do near the start.
func isBinary(data []byte) bool {
	head := data
	if len(head) > binarySniffLen {
		head = head[:binarySniffLen]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// splitLines turns the bytes into display lines, with tabs expanded so a leading
// tab does not collapse against the gutter, and carriage returns dropped so a
// CRLF file does not show a stray glyph at every line end.
func splitLines(data []byte) []string {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\t", "    ")
	return strings.Split(text, "\n")
}

// applyFileLoaded moves to the viewer screen once the read comes back.
func (m *model) applyFileLoaded(msg fileLoadedMsg) {
	switch {
	case msg.err != nil:
		m.status, m.statusIsError = "could not read "+baseName(msg.path)+": "+msg.err.Error(), true
		return
	case msg.binary:
		m.status, m.statusIsError = baseName(msg.path)+" looks like a binary file; not shown", true
		return
	}

	m.viewer = &viewerState{path: msg.path, lines: msg.lines, truncated: msg.truncated}
	m.scr = screenViewer
	m.status, m.statusIsError = "", false
}

func (m *model) handleViewerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	v := m.viewer
	page := max(m.viewerHeight()-1, 1)

	switch msg.String() {
	case keyEscape, "q", keyBackspace, keyLeft, "h":
		// Close returns to the list it was opened from; unlike q on the browse
		// screen, it does not quit.
		m.viewer = nil
		m.scr = m.viewerFrom
		return m, nil
	case "up", "k":
		v.offset--
	case "down", "j":
		v.offset++
	case "pgup":
		v.offset -= page
	case "pgdown", " ":
		v.offset += page
	case "home", "g":
		v.offset = 0
	case "end", "G":
		v.offset = len(v.lines)
	}
	m.clampViewer()
	return m, nil
}

// clampViewer keeps the scroll offset within the file, so paging past either end
// simply stops rather than showing blank space or a negative index.
func (m *model) clampViewer() {
	maxOffset := max(len(m.viewer.lines)-m.viewerHeight(), 0)
	m.viewer.offset = min(max(m.viewer.offset, 0), maxOffset)
}

func baseName(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		return path[i+1:]
	}
	return path
}

// truncateLine cuts a display line to the terminal width. The viewer does not
// scroll sideways yet, so an over-long line is cut with a marker rather than
// wrapped, which would break the line count the window math depends on.
func truncateLine(line string, width int) string {
	return runewidth.Truncate(line, max(width, 1), "…")
}
