package charm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
)

// A binary file must never be dumped as mojibake. The NUL-byte sniff is what
// catches it — text does not contain NUL, a binary almost always does near the
// start.
func TestBinaryFilesAreRefused(t *testing.T) {
	assert.True(t, isBinary([]byte("ELF\x00\x01\x02binary")), "a NUL means binary")
	assert.False(t, isBinary([]byte("plain text, no nul here")), "text has no NUL")
	assert.False(t, isBinary(nil), "an empty file is not binary")

	dir := t.TempDir()
	path := filepath.Join(dir, "a.out")
	require.NoError(t, os.WriteFile(path, append([]byte("\x7fELF"), 0, 1, 2, 3), 0o600))

	msg := readFileCmd(path)().(fileLoadedMsg)
	assert.True(t, msg.binary, "the reader must flag it, not return its bytes")
	assert.Empty(t, msg.lines)
}

// The read is capped so a huge file cannot be pulled into memory. The cap is
// exercised with a small override would be ideal, but the real constant is what
// ships — so write just over it and check the flag and the length.
func TestReadIsCappedAndFlagged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	// Lines of printable text, comfortably over the cap.
	big := bytes.Repeat([]byte("all work and no play\n"), (viewerReadCap/21)+100)
	require.NoError(t, os.WriteFile(path, big, 0o600))

	msg := readFileCmd(path)().(fileLoadedMsg)
	require.NoError(t, msg.err)
	assert.True(t, msg.truncated, "a file over the cap must be flagged truncated")

	var total int
	for _, line := range msg.lines {
		total += len(line) + 1
	}
	assert.LessOrEqual(t, total, viewerReadCap+len(msg.lines),
		"no more than the cap should have been read")
}

func TestSplitLinesExpandsTabsAndDropsCR(t *testing.T) {
	lines := splitLines([]byte("a\tb\r\nc"))
	assert.Equal(t, []string{"a    b", "c"}, lines, "tabs expand, CRLF becomes one break")
}

// v on a directory does not view it — → opens it. Saying so beats doing nothing.
func TestViewingADirectoryIsRefused(t *testing.T) {
	m := benchModel(0)
	dir := m.currentDir.(*analyze.Dir)
	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: dir}}
	dir.AddFile(sub)
	m.reloadRows()

	for i, r := range m.rows {
		if r.IsDir() {
			m.cursor = i
		}
	}
	next, cmd := m.openViewer()
	m = next.(*model)
	assert.Nil(t, cmd, "no read may be started for a directory")
	assert.Equal(t, screenBrowse, m.scr, "a directory must not open the viewer")
	assert.True(t, m.statusIsError)
}

// --no-view-file is a promise: the key says it is disabled rather than silently
// doing nothing.
func TestNoViewFileSaysItIsDisabled(t *testing.T) {
	m := benchModel(3)
	m.ui.noViewFile = true

	next, cmd := m.openViewer()
	m = next.(*model)
	assert.Nil(t, cmd, "no read may be started")
	assert.Equal(t, screenBrowse, m.scr)
	assert.Contains(t, m.status, "--no-view-file")
}

func TestViewerScrollClampsToTheFile(t *testing.T) {
	m := benchModel(0)
	m.width, m.height, m.haveSize = 80, 12, true
	m.scr = screenViewer
	m.viewer = &viewerState{path: "/x", lines: []string{"a", "b", "c"}}

	// Paging past the end stops at the last screenful, never past it.
	m = press(t, m, "G")
	assert.LessOrEqual(t, m.viewer.offset, len(m.viewer.lines))
	assert.GreaterOrEqual(t, m.viewer.offset, 0)

	// Paging past the top stops at zero rather than going negative.
	m = press(t, m, "g")
	assert.Equal(t, 0, m.viewer.offset)
	m = press(t, m, "up", "up")
	assert.Equal(t, 0, m.viewer.offset, "scrolling above the top must clamp")
}

// The viewer closes rather than quitting, and it does not leave the file loaded.
func TestViewerClosesBackToBrowse(t *testing.T) {
	m := benchModel(0)
	m.width, m.height, m.haveSize = 80, 12, true
	m.scr = screenViewer
	m.viewer = &viewerState{path: "/x", lines: []string{"a"}}

	_, cmd := m.Update(key("q"))
	assert.Nil(t, cmd, "q must close the viewer, not quit the program")

	m2 := benchModel(0)
	m2.width, m2.height, m2.haveSize = 80, 12, true
	m2.scr = screenViewer
	m2.viewer = &viewerState{path: "/x", lines: []string{"a"}}
	m2 = press(t, m2, "esc")
	assert.Equal(t, screenBrowse, m2.scr)
	assert.Nil(t, m2.viewer, "closing must release the file")
}

func TestViewerReadsARealFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("line one\nline two\n"), 0o600))

	msg := readFileCmd(path)().(fileLoadedMsg)
	require.NoError(t, msg.err)
	assert.False(t, msg.binary)
	assert.False(t, msg.truncated)
	assert.Equal(t, []string{"line one", "line two", ""}, msg.lines)

	m := benchModel(0)
	m.width, m.height, m.haveSize = 80, 12, true
	m.applyFileLoaded(msg)
	assert.Equal(t, screenViewer, m.scr)
	assert.Contains(t, strings.Join(m.viewer.lines, "\n"), "line one")
}
