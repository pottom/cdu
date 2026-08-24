package charm

import (
	"io"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// TestSymlinkTargetRendering checks that --show-symlink-target adds "name ->
// target" to a symlink row and only to a symlink row, and that the addition goes
// through the same width-exact path as everything else — the target is appended
// to the plain name before the single Truncate/FillRight, so the row still fills
// exactly the terminal width whether the target is short or clipped.
func TestSymlinkTargetRendering(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	ui := CreateUI(io.Discard, true, false, false, false)
	m := newModel(ui)

	dir := &analyze.Dir{File: &analyze.File{Name: "root"}, BasePath: "/"}
	dir.AddFile(&analyze.File{Name: "link", Symlink: "/etc/hosts", Size: 10, Usage: 10, Parent: dir})
	dir.AddFile(&analyze.File{Name: "regular", Size: 10, Usage: 10, Parent: dir})

	m.topDir = dir
	m.enterDir(dir)
	m.scr = screenBrowse
	m.width, m.height = 120, 40
	m.haveSize = true

	total := m.itemSize(m.currentDir)

	find := func(name string) fs.Item {
		for _, r := range m.rows {
			if r.GetName() == name {
				return r
			}
		}
		t.Fatalf("row %q not found", name)
		return nil
	}
	link, regular := find("link"), find("regular")

	// Off: no arrow, and the row still fills the width.
	m.ui.showSymlinkTarget = false
	off := m.viewRow(link, false, total)
	assert.NotContains(t, off, "->", "no target should show while the flag is off")
	assert.Equal(t, m.width, lipgloss.Width(off))

	// On: the symlink reads "name -> target", still width-exact.
	m.ui.showSymlinkTarget = true
	on := m.viewRow(link, false, total)
	assert.Contains(t, on, "link -> /etc/hosts")
	assert.Equal(t, m.width, lipgloss.Width(on))

	// A non-symlink row is untouched even with the flag on.
	assert.NotContains(t, m.viewRow(regular, false, total), "->")

	// A long target is clipped with the row, never past it: narrow the terminal and
	// the row must still be exactly the width, not overflow.
	m.width = 40
	long := &analyze.File{Name: "l", Symlink: "/a/very/long/symlink/target/that/will/not/fit", Parent: dir}
	assert.Equal(t, m.width, lipgloss.Width(m.viewRow(long, false, total)))
}
