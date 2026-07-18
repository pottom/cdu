package charm

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/device"
	"github.com/pottom/cdu/pkg/fs"
)

// rootModelAt builds a browse model sitting at the top of a scan rooted at path,
// with one child so the list is not empty.
func rootModelAt(path string) *model {
	ui := CreateUI(io.Discard, true, false, false, false)
	ui.scanPath = path

	dir := &analyze.Dir{File: &analyze.File{Name: filepath.Base(path)}, BasePath: filepath.Dir(path)}
	dir.AddFile(&analyze.File{Name: "child", Size: 10, Usage: 10, Parent: dir})
	dir.UpdateStats(make(fs.HardLinkedItems))

	m := newModel(ui)
	m.topDir = dir
	m.enterDir(dir)
	m.scr = screenBrowse
	m.width, m.height, m.haveSize = 100, 24, true
	return m
}

// canAscendOnDisk is true only at the scan root, off the device list, from a real
// path with a parent to climb to.
func TestCanAscendOnDisk(t *testing.T) {
	m := rootModelAt("/home/user/project")
	assert.True(t, m.canAscendOnDisk(), "at the scan root there is a parent to scan")

	// Inside the tree there is a real parent, so ← stays in the tree.
	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: m.currentDir}}
	m.currentDir.(*analyze.Dir).AddFile(sub)
	m.enterDir(sub)
	assert.False(t, m.canAscendOnDisk(), "inside the tree ← ascends without a scan")

	// -d mode goes back to the device list, not up the disk.
	m2 := rootModelAt("/home/user/project")
	m2.disks = device.Devices{}
	assert.False(t, m2.canAscendOnDisk())

	// A saved scan has no path to walk.
	m3 := rootModelAt("/home/user/project")
	m3.ui.scanPath = ""
	assert.False(t, m3.canAscendOnDisk())

	// The filesystem root has no parent above it.
	m4 := rootModelAt("/")
	assert.False(t, m4.canAscendOnDisk(), "there is nothing above /")
}

// At the scan root, ← sets up a fresh scan of the parent and remembers where to put
// the cursor when it lands.
func TestAscendAtRootScansTheParent(t *testing.T) {
	m := rootModelAt("/home/user/project")

	cmd := m.ascend()
	require.NotNil(t, cmd, "a scan of the parent must be kicked off")
	assert.Equal(t, screenScanning, m.scr, "the progress screen shows the parent being read")
	assert.Equal(t, "/home/user", m.ui.scanPath, "the scan root moves up one level")
	assert.Equal(t, "/home/user/project", m.landOnPath, "and the cursor will return to where we came from")
}

// At the filesystem root there is nowhere up to go, and ← says so rather than
// scanning / again.
func TestAscendAtFilesystemRootRefuses(t *testing.T) {
	m := rootModelAt("/")

	cmd := m.ascend()
	assert.Nil(t, cmd, "no scan is started")
	assert.Equal(t, screenBrowse, m.scr, "and the screen does not change")
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "top of the filesystem")
}

// A view read from a saved file has no path on disk, so ← above its root cannot
// scan anything and says why.
func TestAscendFromASavedScanRefuses(t *testing.T) {
	m := rootModelAt("/home/user/project")
	m.ui.scanPath = ""

	cmd := m.ascend()
	assert.Nil(t, cmd)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "read from a file")
}

// When the parent scan lands, the cursor returns to the directory it came out of —
// now one row among the parent's children.
func TestLandOnPathReturnsCursorToTheOldRoot(t *testing.T) {
	parent := &analyze.Dir{File: &analyze.File{Name: "user"}, BasePath: "/home"}
	for _, name := range []string{"aaa", "project", "zzz"} {
		sub := &analyze.Dir{File: &analyze.File{Name: name, Parent: parent}}
		sub.AddFile(&analyze.File{Name: "x", Size: 10, Usage: 10, Parent: sub})
		parent.AddFile(sub)
	}
	parent.UpdateStats(make(fs.HardLinkedItems))

	m := rootModelAt("/home/user/project")
	m.landOnPath = "/home/user/project"

	next, _ := m.Update(scanDoneMsg{dir: parent})
	m = next.(*model)

	require.Equal(t, screenBrowse, m.scr)
	require.NotNil(t, m.selected())
	assert.Equal(t, "/home/user/project", m.selected().GetPath(), "the cursor lands on the old root")
	assert.Empty(t, m.landOnPath, "and the one-shot target is cleared")
}
