package charm

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/testdir"
)

// TestProgramRunsScansAndQuits drives the whole Bubble Tea program — not just
// the model — through a real scan and out again, with input and output injected
// so no TTY is needed.
func TestProgramRunsScansAndQuits(t *testing.T) {
	defer testdir.CreateTestDir()()

	var out bytes.Buffer
	ui := CreateUI(&out, true, false, false, false)
	require.NoError(t, ui.AnalyzePath("test_dir", nil))

	// "q" quits; the leading key events give the scan a moment to land.
	in := strings.NewReader("q")

	done := make(chan error, 1)
	go func() {
		_, err := ui.program(tea.WithInput(in)).Run()
		done <- err
	}()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("program did not exit on q")
	}

	assert.NotEmpty(t, out.String(), "program produced no output")
}

func TestReadAnalysisOpensSavedScan(t *testing.T) {
	saved := `[1,2,{"progname":"gdu","progver":"development","timestamp":1626806293},
	[{"name":"/tmp/test"},
	{"name":"a","asize":10,"dsize":512},
	[{"name":"b"},
	{"name":"c","asize":20,"dsize":1024}]]]`

	ui := CreateUI(io.Discard, true, false, false, false)
	require.NoError(t, ui.ReadAnalysis(strings.NewReader(saved)))

	m := newModel(ui)
	// Init still resolves the disk line, but it must not walk anything: the tree
	// is already in memory, so the browser opens on the first frame.
	m.Init()
	assert.Equal(t, screenBrowse, m.scr)
	assert.Equal(t, "/tmp/test", m.currentDir.GetPath())
	assert.NotEmpty(t, m.rows, "saved scan produced no rows")
}

func TestUnsupportedModesPointAtClassic(t *testing.T) {
	ui := CreateUI(io.Discard, true, false, false, false)

	err := ui.ReadFromStorage("/tmp/db", "/tmp")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--classic")
}

func TestFormatSize(t *testing.T) {
	binary := CreateUI(io.Discard, true, false, false, false)
	assert.Equal(t, "1.0 KiB", binary.formatSize(1024))
	assert.Equal(t, "512 B", binary.formatSize(512))

	si := CreateUI(io.Discard, true, false, false, true)
	assert.Equal(t, "1.0 kB", si.formatSize(1000))
}

func TestMiddleTruncateKeepsBothEnds(t *testing.T) {
	got := middleTruncate("/home/user/very/deep/path/to/thing", 20)
	assert.LessOrEqual(t, len([]rune(got)), 20)
	assert.True(t, strings.HasPrefix(got, "/home"), "lost the start: %q", got)
	assert.True(t, strings.HasSuffix(got, "thing"), "lost the end: %q", got)
}
