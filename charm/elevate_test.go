package charm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/elevate"
	"github.com/pottom/cdu/pkg/analyze"
)

// A delete refused for lack of permission does not just report the wall — it opens a
// second modal offering to remove the item with elevated privileges, permanently and
// only after typing to confirm.
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
	assert.True(t, m.confirm.requireTyping, "an elevated delete always types to confirm")
	assert.Equal(t, actionDelete, m.confirm.act, "and it is permanent — the trash is no place for a root file")
	assert.Contains(t, modalTitle(m.confirm), "elevated privileges")
	assert.Contains(t, m.viewModal(), "sudo", "the consequence names what will run")
}

// Once sudo has returned successfully, the tree half runs like any other permanent
// delete: the row leaves and every parent's size follows.
func TestApplyElevatedDeleteUpdatesTheTree(t *testing.T) {
	m := benchModel(4)
	dir := m.currentDir.(*analyze.Dir)
	victim := m.rows[0]
	before := dir.GetUsage()

	cmd := m.applyElevatedDelete(elevatedDoneMsg{item: victim, parent: dir})

	assert.Len(t, m.rows, 3, "the row leaves the list")
	assert.Equal(t, before-victim.GetUsage(), dir.GetUsage(), "the parent shrinks by exactly what left")
	assert.False(t, m.statusIsError)
	assert.Contains(t, m.status, "elevated")
	assert.NotNil(t, cmd, "the freed space means the gauge is re-read")
}

// sudo rm exits nonzero when it cannot remove the item — a wrong password, a
// cancelled prompt, an immutable file even root cannot unlink — and that is reported,
// not a success invented for a row still on disk.
func TestApplyElevatedDeleteReportsFailure(t *testing.T) {
	m := benchModel(4)

	cmd := m.applyElevatedDelete(elevatedDoneMsg{
		item: m.rows[0], parent: m.currentDir, err: assertErr,
	})

	assert.Nil(t, cmd)
	assert.Len(t, m.rows, 4, "nothing was removed")
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "elevated delete failed")
}
