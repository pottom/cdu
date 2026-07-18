package charm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/elevate"
	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// A single delete refused for lack of permission opens a second modal offering to
// remove the item with elevated privileges — permanently, and only after typing.
func TestPermissionDeniedOffersElevation(t *testing.T) {
	if !elevate.Available() {
		t.Skip("no sudo on this machine to offer elevation")
	}
	m := benchModel(4)

	cmd := m.applyDelete(deleteDoneMsg{
		item: m.rows[0], parent: m.currentDir, act: actionDelete,
		err: os.ErrPermission,
	})

	assert.Nil(t, cmd, "opening a modal is not a command")
	require.Equal(t, screenConfirm, m.scr, "the elevation modal opens")
	require.NotNil(t, m.confirm)
	assert.True(t, m.confirm.elevated)
	assert.Len(t, m.confirm.elevateItems, 1)
	assert.True(t, m.confirm.requireTyping, "an elevated delete always types to confirm")
	assert.Equal(t, actionDelete, m.confirm.act, "and it is permanent — the trash is no place for a root file")
	assert.Contains(t, modalTitle(m.confirm), "elevated privileges")
	assert.Contains(t, m.viewModal(), "sudo", "the consequence names what will run")
}

// A batch collects what it could not remove for want of permission and, once the run
// is done, offers the whole set to one elevated pass — so a marked set of root-owned
// files still goes.
func TestABatchOffersElevationForItsPermissionFailures(t *testing.T) {
	if !elevate.Available() {
		t.Skip("no sudo on this machine to offer elevation")
	}
	m := benchModel(5)
	a, b := m.rows[0], m.rows[1]

	m.startBatchDelete([]fs.Item{a, b}, actionDelete)
	// Both come back permission-denied, the way root-owned files would.
	m.applyDelete(deleteDoneMsg{item: a, parent: a.GetParent(), act: actionDelete, err: os.ErrPermission})
	cmd := m.applyDelete(deleteDoneMsg{item: b, parent: b.GetParent(), act: actionDelete, err: os.ErrPermission})

	assert.Nil(t, cmd, "the batch ends by opening the elevation modal, not a command")
	require.Equal(t, screenConfirm, m.scr)
	require.NotNil(t, m.confirm)
	assert.True(t, m.confirm.elevated)
	assert.Len(t, m.confirm.elevateItems, 2, "both failures are offered together")
	assert.Contains(t, modalTitle(m.confirm), "2 items")
}

// Once sudo has returned, the tree half runs for the whole set like any other
// permanent delete: the rows leave and every parent's size follows.
func TestApplyElevatedDeleteUpdatesTheTree(t *testing.T) {
	m := benchModel(4)
	dir := m.currentDir.(*analyze.Dir)
	victim := m.rows[0]
	before := dir.GetUsage()

	cmd := m.applyElevatedDelete(elevatedDoneMsg{items: []fs.Item{victim}})

	assert.Len(t, m.rows, 3, "the row leaves the list")
	assert.Equal(t, before-victim.GetUsage(), dir.GetUsage(), "the parent shrinks by exactly what left")
	assert.False(t, m.statusIsError)
	assert.Contains(t, m.status, "elevated")
	assert.NotNil(t, cmd, "the freed space means the gauge is re-read")
}

// sudo rm exits nonzero when it cannot remove everything — a wrong password, a
// cancelled prompt, an immutable file even root cannot unlink — and that is reported,
// not a success invented for rows still on disk.
func TestApplyElevatedDeleteReportsFailure(t *testing.T) {
	m := benchModel(4)

	cmd := m.applyElevatedDelete(elevatedDoneMsg{items: []fs.Item{m.rows[0]}, err: assertErr})

	assert.Nil(t, cmd)
	assert.Len(t, m.rows, 4, "nothing was removed")
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "elevated delete failed")
}
