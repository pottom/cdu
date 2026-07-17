package charm

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/trash"
	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/device"
)

// key builds a KeyMsg from the name the model matches on.
//
// An unknown name would come back as KeyType(0) — "ctrl+@" — and press would
// then send a key nothing handles, so the test would assert against a model
// nobody touched and pass for the wrong reason. So it fails loudly instead, and
// every name the model switches on has to be in this map.
func key(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
	t, ok := map[string]tea.KeyType{
		"enter":     tea.KeyEnter,
		"esc":       tea.KeyEsc,
		"left":      tea.KeyLeft,
		"right":     tea.KeyRight,
		"up":        tea.KeyUp,
		"down":      tea.KeyDown,
		"home":      tea.KeyHome,
		"end":       tea.KeyEnd,
		"pgup":      tea.KeyPgUp,
		"pgdown":    tea.KeyPgDown,
		"backspace": tea.KeyBackspace,
		"tab":       tea.KeyTab,
		"ctrl+c":    tea.KeyCtrlC,
	}[s]
	if !ok {
		panic("key: no KeyType for " + s + " — add it, or the test presses ctrl+@ and passes for nothing")
	}
	return tea.KeyMsg{Type: t}
}

func press(t *testing.T, m *model, keys ...string) *model {
	t.Helper()
	for _, k := range keys {
		next, _ := m.Update(key(k))
		m = next.(*model)
	}
	return m
}

// Enter on entry must cancel. Someone who hits d by mistake and then hits Enter
// out of reflex must not have deleted anything.
func TestEnterOnEntryCancels(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "d")
	require.Equal(t, screenConfirm, m.scr)
	require.NotNil(t, m.confirm)
	assert.False(t, m.confirm.confirmFocused, "the destructive button must not hold the focus")

	m = press(t, m, "enter")
	assert.Equal(t, screenBrowse, m.scr)
	assert.Nil(t, m.confirm, "Enter on entry must cancel, not delete")
	assert.Len(t, m.rows, 5, "nothing may have been removed")
}

func TestEscapeCancels(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "D", "esc")
	assert.Equal(t, screenBrowse, m.scr)
	assert.Nil(t, m.confirm)
}

// q is a letter of the word being typed, not a way out of the program. If the
// modal let it through, the confirmation for deleting $HOME would quit instead.
func TestQuitIsNotReachableFromTheModal(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "d")

	_, cmd := m.Update(key("q"))
	assert.Nil(t, cmd, "q must not quit while a delete is being confirmed")
	assert.Equal(t, screenConfirm, m.scr)
}

// --no-delete is a promise. The keys stay bound and say they are disabled: a key
// that silently does nothing reads as a broken interface.
func TestNoDeleteMakesTheKeysInertAndSaysSo(t *testing.T) {
	m := benchModel(5)
	m.ui.noDelete = true

	for _, k := range []string{"d", "D", "e"} {
		m = press(t, m, k)
		assert.Equal(t, screenBrowse, m.scr, "%s must not open the modal", k)
		assert.Nil(t, m.confirm)
		assert.True(t, m.statusIsError)
		assert.Contains(t, m.status, "--no-delete")
	}
}

func TestOnlyAFileCanBeEmptied(t *testing.T) {
	m := benchModel(3)
	dir := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: m.currentDir.(*analyze.Dir)}}
	m.currentDir.(*analyze.Dir).AddFile(dir)
	m.reloadRows()

	for i, row := range m.rows {
		if row.IsDir() {
			m.cursor = i
		}
	}

	m = press(t, m, "e")
	assert.Equal(t, screenBrowse, m.scr)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "only a file")
}

// A protected path cannot be deleted by any sequence of single keypresses. This
// is the guard that stands between a stray D and a home directory.
func TestProtectedPathNeedsTheWordTyped(t *testing.T) {
	m := benchModel(3)
	m = press(t, m, "D")
	m.confirm.requireTyping = true

	// Focus cannot even reach the destructive button while the word is incomplete,
	// so Enter stays on Cancel and the worst a mashed keyboard can do is back out.
	m = press(t, m, "right", "right", "tab")
	assert.False(t, m.confirm.confirmFocused, "the button must stay locked")

	m = press(t, m, "enter")
	assert.Equal(t, screenBrowse, m.scr, "Enter must have cancelled, not deleted")
	assert.Len(t, m.rows, 3, "nothing may have been removed")

	// Typed out in full, the button unlocks — and only then.
	m = press(t, m, "D")
	m.confirm.requireTyping = true
	m = press(t, m, "D", "E", "L", "E", "T", "E")
	assert.True(t, m.confirm.typedFully())

	m = press(t, m, "right")
	assert.True(t, m.confirm.confirmFocused, "the button unlocks once the word is typed")
}

// Removal runs off the render loop and can take seconds on a large tree. Until it
// comes back the row must visibly be working, or it is indistinguishable from a
// keypress that never registered.
func TestARowBeingRemovedSaysSo(t *testing.T) {
	m := benchModel(4)
	m.width, m.height = 100, 24
	m.haveSize = true

	m = press(t, m, "D")
	m = press(t, m, "right", "enter")

	require.NotNil(t, m.pending, "the item must be marked as being removed")
	require.Len(t, m.rows, 4, "the row stays until the disk says it is gone")

	row := m.viewRow(m.rows[0], false, m.itemSize(m.currentDir))
	assert.Contains(t, row, "removing",
		"the state must be a word, not only a spinner: it has to survive --no-color")

	// The spinner has to actually move, or it reads as a hung interface.
	first := m.tickFrame()
	m = press(t, m)
	next, _ := m.Update(tickMsg{})
	m = next.(*model)
	assert.NotEqual(t, first, m.tickFrame(), "the spinner must advance on the tick")

	m.applyDelete(deleteDoneMsg{item: m.rows[0], parent: m.currentDir, act: actionDelete})
	assert.Nil(t, m.pending, "the mark must clear once the removal lands")
	assert.Len(t, m.rows, 3)
}

// The header's disk gauge is read once, at startup. After a delete it is stale —
// and a gauge that reports a full disk after you have just emptied it is worse
// than no gauge. Every destructive action must ask for it again.
func TestEveryDestructiveActionRefreshesTheDiskGauge(t *testing.T) {
	for _, act := range []action{actionTrash, actionDelete, actionEmpty} {
		m := benchModel(4)
		m.ui.getter = &fakeGetter{size: 1000, free: 400}

		cmd := m.applyDelete(deleteDoneMsg{item: m.rows[0], parent: m.currentDir, act: act})
		require.NotNil(t, cmd, "act %d must re-read the mount table", act)

		msg, ok := cmd().(deviceMsg)
		require.True(t, ok, "act %d must produce a deviceMsg", act)
		assert.NotNil(t, msg.dev)
	}
}

// A failed delete changed nothing, so there is nothing to re-read.
func TestAFailedDeleteDoesNotRefreshTheGauge(t *testing.T) {
	m := benchModel(4)
	m.ui.getter = &fakeGetter{size: 1000, free: 400}

	cmd := m.applyDelete(deleteDoneMsg{
		item: m.rows[0], parent: m.currentDir, act: actionDelete,
		err: os.ErrPermission,
	})
	assert.Nil(t, cmd)
	assert.True(t, m.statusIsError)
}

type fakeGetter struct{ size, free int64 }

func (g *fakeGetter) GetMounts() (device.Devices, error) { return g.GetDevicesInfo() }
func (g *fakeGetter) GetDevicesInfo() (device.Devices, error) {
	return device.Devices{{Name: "disk", MountPoint: "/", Size: g.size, Free: g.free}}, nil
}

// The undo hint appears only when there is something to undo. Advertising a key
// that would do nothing is the same fault as listing d before deletion existed.
func TestUndoHintAppearsOnlyWithSomethingToUndo(t *testing.T) {
	m := benchModel(4)
	m.width, m.height, m.haveSize = 120, 24, true
	m.scr = screenBrowse

	assert.NotContains(t, m.viewFooter(), "undo", "nothing has been trashed yet")

	m.applyDelete(deleteDoneMsg{
		item: m.rows[0], parent: m.currentDir, act: actionTrash,
		entry: &trash.Entry{OriginalPath: m.rows[0].GetPath()},
	})
	assert.Contains(t, m.viewFooter(), "undo", "a trashed item makes undo real")

	// Once it has been used, the hint goes again.
	m.applyUndo(undoDoneMsg{entry: m.lastTrashed.entry, item: m.lastTrashed.item, parent: m.lastTrashed.parent})
	assert.NotContains(t, m.viewFooter(), "undo", "nothing left to undo")
}

// Two overlapping removals would race to adjust the same parent's size, and there
// is nothing to gain from letting them.
func TestASecondDeleteIsRefusedWhileOneIsRunning(t *testing.T) {
	m := benchModel(4)
	m.pending = m.rows[0]

	m = press(t, m, "d")
	assert.Equal(t, screenBrowse, m.scr, "the modal must not open")
	assert.Nil(t, m.confirm)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "still removing")
}

// The whole point of the modal is what will be true afterwards, and the fact
// people most often do not know is that the trash does not free any space.
func TestTheModalStatesTheConsequence(t *testing.T) {
	assert.Contains(t, modalConsequence(actionTrash), "does not free disk space")
	assert.Contains(t, modalConsequence(actionDelete), "cannot be undone")
	assert.Contains(t, modalConsequence(actionEmpty), "cannot be undone")
}

// The tree half of a delete runs on the render loop and goes through the engine's
// own RemoveFile, so every parent's size and item count follows the disk.
func TestDeleteUpdatesTheTreeAndTheParentSizes(t *testing.T) {
	m := benchModel(4)
	dir := m.currentDir.(*analyze.Dir)

	before := dir.GetUsage()
	victim := m.rows[0]

	m.applyDelete(deleteDoneMsg{item: victim, parent: dir, act: actionDelete})

	assert.Len(t, m.rows, 3, "the row must leave the list")
	assert.Equal(t, before-victim.GetUsage(), dir.GetUsage(),
		"the parent's size must drop by exactly what left it")
	assert.Empty(t, m.status == "", "the delete must be reported")
	assert.False(t, m.statusIsError)
}

// Undo must leave the tree exactly as it found it — including every parent's size,
// which Dir.AddFile does not restore on its own. It must do so without touching the
// disk: an undo that triggered a rescan of a million-file tree would be unusable.
func TestUndoRestoresTheTreeAndTheParentSizes(t *testing.T) {
	m := benchModel(4)
	dir := m.currentDir.(*analyze.Dir)

	// A real scan leaves the tree already summed. The synthetic fixture does not,
	// so start from the same footing a scan would have left behind.
	m.recomputeStats()

	sizeBefore := dir.GetUsage()
	countBefore := dir.ItemCount
	victim := m.rows[0]

	m.applyDelete(deleteDoneMsg{
		item: victim, parent: dir, act: actionTrash,
		entry: &trash.Entry{OriginalPath: victim.GetPath()},
	})
	require.NotNil(t, m.lastTrashed, "a trashed item must arm the undo")
	require.Less(t, dir.GetUsage(), sizeBefore)

	m.applyUndo(undoDoneMsg{
		entry:  m.lastTrashed.entry,
		item:   m.lastTrashed.item,
		parent: m.lastTrashed.parent,
	})

	assert.Equal(t, sizeBefore, dir.GetUsage(), "the parent's size must come all the way back")
	assert.Equal(t, countBefore, dir.ItemCount, "the parent's item count must come back")
	assert.Len(t, m.rows, 4, "the row must return to the list")
	assert.Nil(t, m.lastTrashed, "there is nothing left to undo")
}

// The engine's hard-link ledger records every inode it is shown, so re-running
// UpdateStats over a *used* ledger finds every hard-linked file already in it and
// counts it as zero bytes — the tree silently shrinks on every undo. recomputeStats
// must start from a fresh ledger, exactly as a scan does.
func TestRecomputeIsStableAcrossHardLinks(t *testing.T) {
	m := benchModel(0)
	dir := m.currentDir.(*analyze.Dir)

	// Two names, one inode: the second must not be counted twice, and neither must
	// be lost the second time round.
	for _, name := range []string{"link-a", "link-b"} {
		dir.AddFile(&analyze.File{
			Name: name, Size: 8192, Usage: 8192, Mli: 42, Parent: dir,
		})
	}

	m.recomputeStats()
	first := dir.GetUsage()

	m.recomputeStats()
	assert.Equal(t, first, dir.GetUsage(),
		"recomputing twice must not change the tree — the ledger has to be reset")
}

// Undo exists only for the trash. A permanent delete has nothing to put back, and
// offering a key that quietly does nothing would be worse than saying so.
func TestUndoWithoutATrashedItemSaysSo(t *testing.T) {
	m := benchModel(3)
	m.applyDelete(deleteDoneMsg{item: m.rows[0], parent: m.currentDir, act: actionDelete})
	require.Nil(t, m.lastTrashed)

	cmd := m.askUndo()
	assert.Nil(t, cmd)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "nothing to undo")
}

// A cross-volume trash is refused rather than turned into a silent multi-gigabyte
// copy, and the error points at the key that does work.
func TestCrossVolumeTrashPointsAtThePermanentDelete(t *testing.T) {
	m := benchModel(3)
	m.applyDelete(deleteDoneMsg{
		item: m.rows[0], parent: m.currentDir, act: actionTrash,
		err: trash.ErrCrossVolume,
	})

	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "another volume")
	assert.Contains(t, m.status, "D deletes permanently")
	assert.Len(t, m.rows, 3, "a failed trash must not remove the row")
}

func TestDeleteKeysAreRealFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "victim.txt")
	require.NoError(t, os.WriteFile(path, []byte("bye"), 0o600))

	item := &analyze.File{Name: "victim.txt", Parent: &analyze.Dir{
		File: &analyze.File{Name: filepath.Base(dir)}, BasePath: filepath.Dir(dir),
	}}

	msg := deleteCmd(item.Parent, item, actionDelete)().(deleteDoneMsg)
	require.NoError(t, msg.err)
	assert.NoFileExists(t, path, "the file must actually be gone")
}
