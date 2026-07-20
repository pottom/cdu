package charm

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pane is on by default; i toggles it off and back on.
func TestInfoToggle(t *testing.T) {
	m := benchModel(5)
	require.True(t, m.ui.infoOpen, "the pane is on by default")
	require.Equal(t, infoPaneLines, m.infoPaneHeight(), "open with room: the pane's fixed height")

	m = press(t, m, "i")
	assert.False(t, m.ui.infoOpen, "i toggles it off")
	assert.Equal(t, 0, m.infoPaneHeight(), "closed: no pane")

	m = press(t, m, "i")
	assert.True(t, m.ui.infoOpen, "i again toggles it on")
}

// The pane takes its height out of the list, so scrolling shrinks with it — the whole
// reason it is not a modal drawn over the rows. Closing it gives the rows back.
func TestInfoPaneShrinksTheList(t *testing.T) {
	m := benchModel(5) // on by default
	open := m.visibleLines()

	m = press(t, m, "i")
	assert.Equal(t, open+infoPaneLines, m.visibleLines(), "closing returns exactly the pane's height to the list")
}

// The pane shows the selected item — at least its name and size, which come straight
// from the engine (mode and owner need a real file to stat).
func TestInfoPaneShowsTheSelectedItem(t *testing.T) {
	m := benchModel(5) // on by default

	it := m.infoTarget()
	require.NotNil(t, it)
	pane := m.infoPane()
	assert.Contains(t, pane, it.GetName(), "the pane names the item")
	assert.Contains(t, pane, m.ui.formatSize(it.GetUsage()), "and shows its disk usage")
}

// Moving the cursor re-points the pane at the new row, off the render path.
func TestInfoFollowsTheCursor(t *testing.T) {
	m := benchModel(5) // on by default
	first := m.infoTarget()

	m = press(t, m, "down")
	second := m.infoTarget()
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotEqual(t, first, second, "the pane follows the cursor to the next item")
	assert.Equal(t, second, m.infoStat.item, "and the cached stat is refreshed for it")
}

// The config's info key sets the pane's start state; pressing i persists the new state
// on the spot, so the choice survives a restart without a separate save step.
func TestInfoPanePersists(t *testing.T) {
	closed := CreateUI(io.Discard, true, false, false, false, WithInfoPane(false))
	assert.False(t, closed.infoOpen, "WithInfoPane(false) starts the pane closed")

	var saved []bool
	ui := CreateUI(io.Discard, true, false, false, false,
		WithInfoSaver(func(on bool) (string, error) { saved = append(saved, on); return "cfg", nil }))
	dir := benchDir(5)
	m := newModel(ui)
	m.topDir = dir
	m.enterDir(dir)
	m.scr = screenBrowse
	m.width, m.height, m.haveSize = 120, 40, true
	require.True(t, m.ui.infoOpen, "on by default")

	next, cmd := m.Update(key("i"))
	m = next.(*model)
	require.False(t, m.ui.infoOpen, "i toggles it off")
	require.NotNil(t, cmd, "and returns a save command")
	cmd() // run the persist
	assert.Equal(t, []bool{false}, saved, "i saved the off state immediately, no t-then-s needed")
}

// The ../ row has nothing to describe, so i does not open a blank pane on it.
func TestInfoDoesNotOpenOnTheParentRow(t *testing.T) {
	m, _ := subdirModel(t)
	m.cursor = 0 // the ../ row

	m = press(t, m, "i") // close the default-on pane
	require.False(t, m.ui.infoOpen)
	m = press(t, m, "i") // and it will not re-open where there is nothing to show
	assert.False(t, m.ui.infoOpen, "i is inert on the ../ row")
}

// statPath reads the mode (and, on Unix, the numeric owner) off a real file.
func TestStatPathReadsModeAndOwner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o640))

	s := statPath(path)
	require.True(t, s.ok, "a real file stats")
	assert.True(t, strings.HasPrefix(s.mode, "-rw"), "the mode string is the file's, got %q", s.mode)
	if runtime.GOOS != "windows" {
		assert.NotEmpty(t, s.uid, "Unix gives a numeric uid")
		assert.NotEmpty(t, s.gid)
		// The test runs as a real user, whose id is in /etc/passwd, so the name resolves.
		assert.NotEmpty(t, s.uname, "and resolves the user name")
	}

	assert.False(t, statPath(filepath.Join(dir, "does-not-exist")).ok, "a missing path does not stat")
}
