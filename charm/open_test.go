package charm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
)

// The opener is the platform's own, and the mapping has to be right for each — a
// test on macOS still has to know what Linux and Windows would run.
func TestOpenCommandPerPlatform(t *testing.T) {
	assert.Equal(t, []string{"open", "/tmp/x"}, openCommand("darwin", "/tmp/x").Args)
	assert.Equal(t, []string{"xdg-open", "/tmp/x"}, openCommand("linux", "/tmp/x").Args)
	assert.Equal(t, []string{"xdg-open", "/tmp/x"}, openCommand("freebsd", "/tmp/x").Args)
	// The empty title argument is load-bearing: without it start takes the path as a
	// window title and opens nothing.
	assert.Equal(t, []string{"cmd", "/c", "start", "", "/tmp/x"}, openCommand("windows", "/tmp/x").Args)
}

// o on a file hands it to the launcher; the status says so while it happens.
func TestOpenFileLaunchesForAFile(t *testing.T) {
	m := benchModel(5) // benchDir's children are files
	_, cmd := m.openFile()
	require.NotNil(t, cmd, "a file is handed to its default app")
	assert.Contains(t, m.status, "opening")
	assert.False(t, m.statusIsError)
}

// o on a directory does nothing but say why — → is how you enter one.
func TestOpenFileRefusesADirectory(t *testing.T) {
	m := benchModel(0)
	dir := m.currentDir.(*analyze.Dir)
	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: dir}}
	dir.AddFile(sub)
	m.reloadRows()
	m.cursor = 0
	require.True(t, m.selected().IsDir())

	_, cmd := m.openFile()
	assert.Nil(t, cmd, "no launcher for a directory")
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "opens a directory")
}

// The result of the launch is reported either way — a launcher that failed silently
// would look like a key that did nothing.
func TestApplyOpenedReports(t *testing.T) {
	m := benchModel(1)

	m.applyOpened(openedMsg{name: "photo.jpg"})
	assert.Equal(t, "opened photo.jpg", m.status)
	assert.False(t, m.statusIsError)

	m.applyOpened(openedMsg{name: "photo.jpg", err: assertErr})
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "could not open photo.jpg")
}
