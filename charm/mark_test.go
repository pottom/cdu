package charm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/trash"
	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// Space marks the cursor row and steps down, so a run is marked by holding it.
// Space again on the same row takes it back out.
func TestSpaceMarksAndAdvances(t *testing.T) {
	m := benchModel(5)
	first := m.rows[0]

	m = press(t, m, " ")
	assert.True(t, m.isMarked(first), "space marks the cursor row")
	assert.Equal(t, 1, m.cursor, "and steps to the next row")

	// Back up onto the marked row and unmark it: the set is empty again.
	m = press(t, m, "up", " ")
	assert.False(t, m.isMarked(first), "space again unmarks")
	assert.Equal(t, 0, m.markedCount(), "nothing left marked")
}

// With nothing marked the destructive keys act on the cursor row alone — the
// one-key delete anyone already knows survives the batch being added.
func TestWithNoMarksDeleteActsOnTheCursor(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "d")

	require.Equal(t, screenConfirm, m.scr)
	require.NotNil(t, m.confirm)
	assert.Empty(t, m.confirm.batch, "no marks means a single-item confirm")
	assert.Equal(t, m.rows[0], m.confirm.item)
}

// With marks, the destructive keys act on the whole set, and the modal counts and
// sizes it rather than naming one row.
func TestWithMarksDeleteActsOnTheSet(t *testing.T) {
	m := benchModel(5)
	m.marked[m.rows[0]] = true
	m.marked[m.rows[2]] = true
	m.marked[m.rows[4]] = true

	m = press(t, m, "d")
	require.Equal(t, screenConfirm, m.scr)
	require.Len(t, m.confirm.batch, 3, "the set is what the key acts on")
	assert.Contains(t, modalTitle(m.confirm), "3 items")
}

// A marked file inside a marked directory is already covered by the directory, so
// it must not be counted a second time — nor deleted on its own.
func TestNestedMarksAreNotDoubleCounted(t *testing.T) {
	m := benchModel(0)
	dir := m.currentDir.(*analyze.Dir)

	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: dir}, BasePath: "/"}
	inner := &analyze.File{Name: "inner", Size: 100, Usage: 100, Parent: sub}
	sub.AddFile(inner)
	dir.AddFile(sub)
	dir.UpdateStats(make(fs.HardLinkedItems))
	m.reloadRows()

	m.marked[sub] = true
	m.marked[inner] = true

	eff := m.effectiveMarks(actionDelete)
	require.Len(t, eff, 1, "the inner file drops out — its directory already covers it")
	assert.Equal(t, sub, eff[0])
	assert.Equal(t, m.itemSize(sub), m.markedReclaimable(), "and it is counted once, not twice")
}

// Emptying is a file operation, so a set marked for emptying drops its directories
// rather than trying to truncate them.
func TestEmptyingASetSkipsDirectories(t *testing.T) {
	m := benchModel(0)
	dir := m.currentDir.(*analyze.Dir)

	sub := &analyze.Dir{File: &analyze.File{Name: "sub", Parent: dir}, BasePath: "/"}
	sub.AddFile(&analyze.File{Name: "x", Size: 10, Usage: 10, Parent: sub})
	file := &analyze.File{Name: "f", Size: 50, Usage: 50, Parent: dir}
	dir.AddFile(sub)
	dir.AddFile(file)
	dir.UpdateStats(make(fs.HardLinkedItems))
	m.reloadRows()

	m.marked[sub] = true
	m.marked[file] = true

	eff := m.effectiveMarks(actionEmpty)
	require.Len(t, eff, 1, "only the file can be emptied")
	assert.Equal(t, file, eff[0])
}

// A batch trash removes every marked item, clears the marks, and arms undo with the
// whole run — driven through the same applyDelete the interface uses, one message
// at a time, so the chaining is what is under test.
func TestBatchTrashRemovesEveryMarkedItemAndArmsUndo(t *testing.T) {
	m := benchModel(5)
	dir := m.currentDir.(*analyze.Dir)
	m.recomputeStats()

	victims := []fs.Item{m.rows[0], m.rows[1], m.rows[2]}
	for _, v := range victims {
		m.marked[v] = true
	}

	m.startBatchDelete(m.effectiveMarks(actionTrash), actionTrash)
	require.Equal(t, 0, m.markedCount(), "starting the batch consumes the marks")
	require.NotNil(t, m.pending, "the first item is being removed")

	drainBatch(m, actionTrash)

	assert.Len(t, m.rows, 2, "every marked row left the list")
	assert.Len(t, m.lastTrashed, 3, "undo is armed with the whole run")
	assert.Contains(t, m.status, "3 items moved to the trash")

	// And undo puts all three back.
	drainUndo(m)
	assert.Len(t, m.rows, 5, "the whole batch comes back")
	assert.Empty(t, m.lastTrashed, "nothing left to undo")
	assert.Equal(t, dir.GetItemCount(), int64(6), "the parent's count is whole again")
}

// A batch that cannot remove one item still removes the rest and says how many
// failed rather than claiming it all worked.
func TestBatchReportsPartialFailure(t *testing.T) {
	m := benchModel(4)
	for _, v := range []fs.Item{m.rows[0], m.rows[1]} {
		m.marked[v] = true
	}

	m.startBatchDelete(m.effectiveMarks(actionDelete), actionDelete)
	// First fails, second succeeds.
	m.applyDelete(deleteDoneMsg{item: m.pending, parent: m.pending.GetParent(), act: actionDelete, err: assertErr})
	require.NotNil(t, m.pending, "the run keeps going after a failure")
	m.applyDelete(deleteDoneMsg{item: m.pending, parent: m.pending.GetParent(), act: actionDelete})

	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "could not be removed")
}

// Opening the queue snapshots the marked set; space there prunes it in both the
// live marks and the snapshot; emptying it lands back in the browser.
func TestTheQueueListsAndPrunesTheSet(t *testing.T) {
	m := benchModel(5)
	m.marked[m.rows[0]] = true
	m.marked[m.rows[1]] = true
	m.marked[m.rows[2]] = true

	m = press(t, m, "M")
	require.Equal(t, screenQueue, m.scr)
	require.Len(t, m.queue, 3)

	// Unmark the cursor row: it leaves both the queue and the marks.
	removed := m.selectedQueue()
	m = press(t, m, " ")
	assert.Len(t, m.queue, 2)
	assert.False(t, m.isMarked(removed))
	assert.Equal(t, 2, m.markedCount())

	// M again closes back to the browser.
	m = press(t, m, "M")
	assert.Equal(t, screenBrowse, m.scr)
}

// Pressing M with nothing marked says so rather than opening an empty screen.
func TestTheQueueRefusesWhenNothingIsMarked(t *testing.T) {
	m := benchModel(5)
	m = press(t, m, "M")
	assert.Equal(t, screenBrowse, m.scr)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "nothing marked")
}

// The tick in a marked row's gutter must not cost the row a column: a marked row
// has to be exactly as wide as an unmarked one, cursor or not.
func TestAMarkedRowStaysExactWidth(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(5)
	m.marked[m.rows[0]] = true
	m.marked[m.rows[1]] = true

	for _, width := range []int{20, 40, 80, 120} {
		m.width = width
		lines := strings.Split(m.viewList(), "\n")
		for i, line := range lines {
			assert.LessOrEqual(t, lipgloss.Width(line), width,
				"width=%d line %d overflows", width, i)
		}
	}
}

// The header tally makes a growing queue visible while browsing.
func TestTheHeaderShowsTheMarkTally(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(5)
	assert.Empty(t, m.markTally(), "nothing marked, nothing to tally")

	m.marked[m.rows[0]] = true
	m.marked[m.rows[1]] = true
	tally := m.markTally()
	assert.Contains(t, tally, "2 items")
	assert.Contains(t, m.viewHeader(), "2 items", "and it reaches the header")
}

// Esc clears the whole selection at once — the unmark-all to space's mark-one — and
// says it did. It is the browser's answer to a queue built up by mistake.
func TestEscClearsEveryMark(t *testing.T) {
	m := benchModel(5)
	m.marked[m.rows[0]] = true
	m.marked[m.rows[3]] = true

	m = press(t, m, "esc")
	assert.Equal(t, 0, m.markedCount(), "esc clears every mark")
	assert.Equal(t, screenBrowse, m.scr, "and stays on the browser")
	assert.Contains(t, m.status, "marks cleared")
}

// A marked row wears a red ✗ and a struck-through name, so it reads as bound for
// deletion — and differently from the very same row unmarked.
func TestAMarkedRowIsStruckThrough(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(5)
	total := m.rowScale()
	plain := m.viewRow(m.rows[0], false, total)

	m.marked[m.rows[0]] = true
	marked := m.viewRow(m.rows[0], false, total)

	// The strike combines into one SGR with the name's colour, so match the exact
	// opening lipgloss emits for a struck fileName rather than a bare \x1b[9m.
	strike := m.st.fileName.Strikethrough(true).Render("z")
	open := strike[:strings.Index(strike, "z")]

	assert.NotEqual(t, plain, marked, "a marked row must look different from an unmarked one")
	assert.Contains(t, marked, m.markGlyph(), "the gutter carries the ✗")
	assert.Contains(t, marked, open, "the name is struck through")
	assert.NotContains(t, plain, open, "an unmarked row is not")
}

// The strike is on the name whether or not the cursor is on the row — which is what
// makes a marked cursor row unmistakable, the thing a shared background could not do.
func TestAMarkedCursorRowIsStillStruckThrough(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(5)
	m.marked[m.rows[0]] = true
	row := m.viewRow(m.rows[0], true, m.rowScale()) // selected AND marked

	strike := m.st.selected.Strikethrough(true).Render("z")
	open := strike[:strings.Index(strike, "z")]

	assert.Contains(t, row, open, "the cursor row's name is struck when marked")
	assert.Contains(t, row, m.markGlyph(), "and the gutter shows the ✗, not the cursor bar")
}

// Marking is not the browser's alone: space marks on the largest-files screen too,
// and the destructive keys act on the set from there.
func TestMarkingWorksOnTheLargestFilesScreen(t *testing.T) {
	m := benchModel(5)
	m.collectTopFiles()
	require.Equal(t, screenTop, m.scr)
	require.NotEmpty(t, m.topFiles)

	first := m.topFiles[0]
	m = press(t, m, " ")
	assert.True(t, m.isMarked(first), "space marks the file under the cursor")
	assert.Equal(t, 1, m.topCursor, "and steps down")

	m = press(t, m, "d")
	require.Equal(t, screenConfirm, m.scr, "d from here still opens a confirm")
	require.NotNil(t, m.confirm)
	assert.NotEmpty(t, m.confirm.batch, "and it acts on the marked set, not one row")
}

// The mark overlay — the tick and the band — is drawn on the browsing lists but not
// on the queue, where every row is marked and a screen of bands would be noise.
func TestMarkOverlayIsOffOnTheQueue(t *testing.T) {
	m := benchModel(5)
	item := m.rows[0]
	m.marked[item] = true

	m.scr = screenTop
	assert.True(t, m.markOverlay(item), "a marked row stands out on the largest-files list")
	m.scr = screenQueue
	assert.False(t, m.markOverlay(item), "but not on the queue, which is all marked")
}

// The set is what the destructive keys act on from every list, not just the browser.
func TestMarksActOnTheSetFromEveryList(t *testing.T) {
	m := benchModel(1)
	for _, scr := range []screen{screenBrowse, screenTop, screenDup, screenFind, screenQueue} {
		m.scr = scr
		assert.True(t, m.marksActOnSet(), "marks act on the set on %v", scr)
	}
	for _, scr := range []screen{screenScanning, screenConfirm, screenViewer, screenDisks, screenHelp} {
		m.scr = scr
		assert.False(t, m.marksActOnSet(), "no list to mark on %v", scr)
	}
}

// assertErr is a stand-in filesystem error for the partial-failure test.
var assertErr = &fsError{"permission denied"}

type fsError struct{ s string }

func (e *fsError) Error() string { return e.s }

// drainBatch feeds a synthetic success message for each item still pending, the way
// the render loop would as each removal returns — without touching a real disk.
func drainBatch(m *model, act action) {
	for m.pending != nil {
		cur := m.pending
		m.applyDelete(deleteDoneMsg{
			item: cur, parent: cur.GetParent(), act: act,
			entry: &trash.Entry{OriginalPath: cur.GetPath()},
		})
	}
}

func drainUndo(m *model) {
	for len(m.lastTrashed) > 0 {
		last := m.lastTrashed[len(m.lastTrashed)-1]
		m.applyUndo(undoDoneMsg{entry: last.entry, item: last.item, parent: last.parent})
	}
}
