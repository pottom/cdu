package charm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/internal/trash"
	"github.com/pottom/cdu/pkg/fs"
)

// action is what the confirmation is about to do.
type action int

const (
	// actionTrash moves the item to the OS trash. Recoverable, and the default,
	// because a disk usage tool is exactly where a delete is easiest to regret.
	actionTrash action = iota
	// actionDelete removes the item for good. The only one that frees disk space.
	actionDelete
	// actionEmpty truncates a file to zero bytes.
	actionEmpty
)

// confirmState is the pending destructive operation.
//
// It is one of two shapes: a single item (item, parent set, batch nil), or a batch
// of marked items (batch set, item/parent nil). The batch carries no parents
// because each item is asked for its own — the set spans directories.
type confirmState struct {
	item   fs.Item
	parent fs.Item
	batch  []fs.Item
	act    action

	// confirmFocused is false on entry: the destructive button is never what a
	// reflexive Enter lands on.
	confirmFocused bool

	// requireTyping guards protected paths, where a single keypress is not enough.
	requireTyping bool
	typed         string
}

// deleteDoneMsg carries the result back to the render loop, which is the only
// goroutine allowed to touch the tree.
type deleteDoneMsg struct {
	item   fs.Item
	parent fs.Item
	act    action
	entry  *trash.Entry
	err    error
}

// trashed is what an undo needs: where the item went, and where it belongs in the
// tree. The tree is not reachable from the trash entry, and the disk is not
// reachable from the tree, so undo needs both halves.
type trashed struct {
	entry  *trash.Entry
	item   fs.Item
	parent fs.Item
}

// undoDoneMsg is the result of putting a trashed item back.
type undoDoneMsg struct {
	entry  *trash.Entry
	item   fs.Item
	parent fs.Item
	err    error
}

// target is the item a destructive key acts on, and the directory it lives in.
//
// The browser's answer is the row under the cursor and the directory being
// shown. The largest-files list has the same rows and no directory being shown,
// so it asks the item where it lives — which is exactly the question that list
// exists to answer.
func (m *model) target() (item, parent fs.Item) {
	//nolint:exhaustive // every other screen falls through to the browser's row
	switch m.scr {
	case screenTop:
		it := m.selectedTop()
		if it == nil {
			return nil, nil
		}
		return it, it.GetParent()
	case screenDup:
		it := m.selectedDup()
		if it == nil {
			return nil, nil
		}
		return it, it.GetParent()
	case screenFind:
		it := m.selectedFind()
		if it == nil {
			return nil, nil
		}
		return it, it.GetParent()
	}
	return m.selected(), m.currentDir
}

// askConfirm opens the modal for the selected item, or explains why it will not.
//
// When something is marked and we are somewhere marks apply — the browser or the
// queue screen — the whole marked set is what the key acts on instead. The single
// row is the fallback, not the rule that has to be worked around.
func (m *model) askConfirm(act action) {
	if m.markedCount() > 0 && (m.scr == screenBrowse || m.scr == screenQueue) {
		m.askConfirmBatch(act)
		return
	}

	item, parent := m.target()
	if item == nil || parent == nil {
		return
	}

	// --no-delete is a promise, so the keys are inert and say so. Silently doing
	// nothing would read as a broken interface.
	if m.ui.noDelete {
		m.status, m.statusIsError = "deletion is disabled (--no-delete)", true
		return
	}
	// One removal at a time. Two overlapping deletes would race to mutate the same
	// parent's size, and there is nothing to gain from the concurrency.
	if m.pending != nil {
		m.status, m.statusIsError = "still removing "+m.pending.GetName()+"…", true
		return
	}
	if act == actionEmpty && item.IsDir() {
		m.status, m.statusIsError = "only a file can be emptied", true
		return
	}
	if act == actionTrash && !trash.Supported() {
		m.status, m.statusIsError = "this platform has no trash cdu can use; D deletes permanently", true
		return
	}

	// Where to go back to when the modal closes. A confirm opened from the
	// largest-files list must return there, not drop you into the browser.
	m.confirmFrom = m.scr
	m.confirm = &confirmState{
		item:          item,
		parent:        parent,
		act:           act,
		requireTyping: isProtected(item.GetPath()),
	}
	m.scr = screenConfirm
}

// askConfirmBatch opens the modal for the marked set. The set is deduped and, for
// emptying, filtered to files — so the modal counts and sizes exactly what will be
// removed, not the raw marks.
func (m *model) askConfirmBatch(act action) {
	items := m.effectiveMarks(act)
	if len(items) == 0 {
		// The only way here with marks but no effective items is `e` over a set that
		// is all directories: nothing a truncation can act on.
		m.status, m.statusIsError = "nothing marked that "+actionGerund(act)+" can act on", true
		return
	}

	if m.ui.noDelete {
		m.status, m.statusIsError = "deletion is disabled (--no-delete)", true
		return
	}
	if m.pending != nil {
		m.status, m.statusIsError = "still removing "+m.pending.GetName()+"…", true
		return
	}
	if act == actionTrash && !trash.Supported() {
		m.status, m.statusIsError = "this platform has no trash cdu can use; D deletes permanently", true
		return
	}

	requireTyping := false
	for _, it := range items {
		if isProtected(it.GetPath()) {
			requireTyping = true
			break
		}
	}

	m.confirmFrom = m.scr
	m.confirm = &confirmState{batch: items, act: act, requireTyping: requireTyping}
	m.scr = screenConfirm
}

// deleteCmd does the filesystem work, and only the filesystem work.
//
// It cannot call pkg/remove, which fuses the removal with the tree update: the
// removal can take seconds on a large tree and so must run off the render loop,
// while the tree is read by View and so must only be mutated on it. So the two
// halves are split — the engine's RemoveFile still performs the tree half, in the
// message handler.
func deleteCmd(parent, item fs.Item, act action) tea.Cmd {
	return func() tea.Msg {
		done := deleteDoneMsg{item: item, parent: parent, act: act}
		path := item.GetPath()

		switch act {
		case actionTrash:
			done.entry, done.err = trash.MoveToTrash(path)
		case actionDelete:
			done.err = os.RemoveAll(path)
		case actionEmpty:
			done.err = os.Truncate(path, 0)
		}
		return done
	}
}

func undoCmd(last *trashed) tea.Cmd {
	return func() tea.Msg {
		return undoDoneMsg{
			entry:  last.entry,
			item:   last.item,
			parent: last.parent,
			err:    trash.Restore(last.entry),
		}
	}
}

// askUndo puts the last trashed item back. Undo exists only for the trash: a
// permanent delete cannot be taken back, and neither can a truncation, so rather
// than a key that quietly does nothing, the interface says why.
func (m *model) askUndo() tea.Cmd {
	switch {
	case !trash.RestoreSupported():
		m.status, m.statusIsError = "cdu cannot restore from this platform's trash", true
		return nil
	case len(m.lastTrashed) == 0:
		m.status, m.statusIsError = "nothing to undo — only a trashed item can come back", true
		return nil
	}

	// A batch is undone the way it was done: one at a time, applyUndo chaining the
	// rest. Restoring starts from the last item trashed, which reads as the run
	// rewinding.
	last := m.lastTrashed[len(m.lastTrashed)-1]
	if len(m.lastTrashed) == 1 {
		m.status, m.statusIsError = "restoring "+filepath.Base(last.entry.OriginalPath)+"…", false
	} else {
		m.status, m.statusIsError = fmt.Sprintf("restoring %d items…", len(m.lastTrashed)), false
	}
	return undoCmd(last)
}

// popTrashed drops one restored entry from the undo list, matched by the trash
// entry it came from.
func (m *model) popTrashed(entry *trash.Entry) {
	for i, t := range m.lastTrashed {
		if t.entry == entry {
			m.lastTrashed = append(m.lastTrashed[:i], m.lastTrashed[i+1:]...)
			return
		}
	}
}

// applyUndo puts the item back into the tree.
//
// The engine's Dir.AddFile appends without touching the size or item count of any
// ancestor — only RemoveFile walks up the tree — so the numbers above the restored
// item would all understate it. UpdateStats is the engine's own fix for that: it
// recomputes the whole tree from its children, and it does so entirely in memory,
// so an undo costs no disk I/O at all.
func (m *model) applyUndo(msg undoDoneMsg) tea.Cmd {
	name := filepath.Base(msg.entry.OriginalPath)
	m.popTrashed(msg.entry)

	if msg.err != nil {
		m.status, m.statusIsError = "could not restore "+name+": "+msg.err.Error(), true
	} else {
		msg.parent.AddFile(msg.item)
		m.recomputeStats()
		if m.currentDir == msg.parent {
			m.reloadRows()
		}
		m.status, m.statusIsError = name+" restored", false
	}

	// More of a batch to put back: restore the next, the mirror of the batch delete.
	if len(m.lastTrashed) > 0 {
		return undoCmd(m.lastTrashed[len(m.lastTrashed)-1])
	}
	return deviceCmd(m.ui)
}

// recomputeStats rebuilds every directory's size and item count from its children.
//
// The hard-link ledger has to be thrown away first. alreadyCounted *records* each
// inode it sees, so running UpdateStats again over the same ledger would find every
// hard-linked file already in it and count it as zero bytes — the tree would appear
// to shrink. A fresh ledger is what the scan itself starts with.
func (m *model) recomputeStats() {
	if m.topDir == nil {
		return
	}
	m.ui.linkedItems = make(fs.HardLinkedItems, 10)
	m.topDir.UpdateStats(m.ui.linkedItems)
}

// rescan walks the tree again. It is the honest response to the disk having
// changed underneath us, and the only way to get every parent's size right again
// after a restore.
func (m *model) rescan() tea.Cmd {
	if m.ui.scanPath == "" {
		// Opened from a saved scan with -f: there is nothing to walk.
		m.status, m.statusIsError = m.status+" · this view is from a file and is now out of date", true
		return nil
	}

	// The tree we have is kept until a new one arrives. It is still true — the
	// rescan is asking whether it still is — and it is what esc goes back to if
	// the walk is cancelled. The scanning screen does not draw the list anyway, so
	// there is nothing to gain by clearing it early. enterDir replaces the lot when
	// the new tree lands.
	return m.startScan()
}

func (m *model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	c := m.confirm

	switch msg.String() {
	case keyEscape, keyCtrlC:
		m.cancelConfirm()
		return m, nil

	case keyLeft, "right", "tab", "h", "l":
		// Focus cannot move onto the destructive button while the typed
		// confirmation is still incomplete: there would be nothing left to stop an
		// Enter from firing it.
		if !c.requireTyping || c.typedFully() {
			c.confirmFocused = !c.confirmFocused
		}
		return m, nil

	case keyEnter:
		if !c.confirmFocused {
			m.cancelConfirm()
			return m, nil
		}
		if c.requireTyping && !c.typedFully() {
			return m, nil
		}
		m.scr = m.confirmFrom
		act, batch := c.act, c.batch
		m.confirm = nil
		if len(batch) > 0 {
			cmd := m.startBatchDelete(batch, act)
			return m, cmd
		}
		m.pending = c.item
		m.status, m.statusIsError = c.inProgressLabel(), false
		// The tick is what makes the row spin. Without it the removal would run
		// invisibly and the row would sit there looking as though nothing happened.
		return m, tea.Batch(deleteCmd(c.parent, c.item, c.act), tickCmd())

	case keyBackspace:
		if c.requireTyping && c.typed != "" {
			c.typed = c.typed[:len(c.typed)-1]
		}
		return m, nil
	}

	// Anything else is a character for the type-to-confirm field, if there is one.
	if c.requireTyping && len(msg.Runes) == 1 {
		c.typed += string(msg.Runes)
		if len(c.typed) > len(confirmWord) {
			c.typed = c.typed[:len(confirmWord)]
		}
	}
	return m, nil
}

func (m *model) cancelConfirm() {
	m.confirm = nil
	m.scr = m.confirmFrom
}

func (c *confirmState) typedFully() bool { return c.typed == confirmWord }

// startBatchDelete kicks off a batch removal. Items go one at a time — two at once
// would race to resize a shared parent — so this fires the first and applyDelete
// chains the rest. A trash starts a fresh undo history; the marks are consumed now,
// the confirm having already happened.
func (m *model) startBatchDelete(items []fs.Item, act action) tea.Cmd {
	if act == actionTrash {
		m.lastTrashed = nil
	}
	m.deleteAct = act
	m.deleteRemaining = items[1:]
	m.batchTotal = len(items)
	m.batchFail = 0
	m.pending = items[0]
	m.clearMarks()
	m.status, m.statusIsError = fmt.Sprintf("%s %d items…", actionGerund(act), len(items)), false
	return tea.Batch(deleteCmd(items[0].GetParent(), items[0], act), tickCmd())
}

// applyDelete performs the tree half of a removal, on the render loop, once the
// filesystem half has come back. When a batch is running it chains the next item
// before settling anything, and one item it cannot remove never strands the rest.
func (m *model) applyDelete(msg deleteDoneMsg) tea.Cmd {
	batch := m.batchTotal > 0

	if msg.err != nil {
		m.batchFail++
		if !batch {
			m.pending = nil
			m.status, m.statusIsError = deleteErrorText(msg), true
			return nil
		}
	} else {
		switch msg.act {
		case actionTrash, actionDelete:
			// RemoveFile is the engine's own: it updates the size and item count all the
			// way up the tree, which is why the tree half is not reimplemented here.
			msg.parent.RemoveFile(msg.item)
			m.dropRow(msg.item)
			m.dropTopFile(msg.item)
			m.dropDuplicate(msg.item)
			m.dropFindResult(msg.item)
		case actionEmpty:
			msg.parent.RemoveFile(msg.item)
			msg.parent.AddFile(emptiedFile(msg.item, msg.parent))
			m.reloadRows()
		}
		// Only a trashed item can come back, so this is the one path that arms undo.
		// A batch appends each item, so undo can put the whole run back.
		if msg.act == actionTrash {
			m.lastTrashed = append(m.lastTrashed, &trashed{entry: msg.entry, item: msg.item, parent: msg.parent})
		}
	}

	// The run is not over: fire the next item before touching the status line, so
	// the interface never flashes "done" mid-batch.
	if len(m.deleteRemaining) > 0 {
		next := m.deleteRemaining[0]
		m.deleteRemaining = m.deleteRemaining[1:]
		m.pending = next
		return tea.Batch(deleteCmd(next.GetParent(), next, m.deleteAct), tickCmd())
	}

	m.pending = nil
	if batch {
		m.status, m.statusIsError = m.batchDoneLabel()
		m.batchTotal, m.batchFail = 0, 0
		// A batch run from the queue screen leaves it stale — its items are gone — so
		// with the marks consumed, land in the browser rather than an empty queue.
		if m.scr == screenQueue {
			m.scr = screenBrowse
		}
	} else {
		m.status, m.statusIsError = m.doneLabel(msg), false
	}

	// The header's disk gauge is now stale — it was read once, at startup. Re-reading
	// it is what makes the trash's central caveat visible rather than merely stated:
	// after a permanent delete the gauge drops, and after a trash it does not move,
	// because the item never left the volume.
	return deviceCmd(m.ui)
}

// batchDoneLabel reports a finished batch: how many went, in what way, and how many
// could not — a run that half-failed must say so, not claim it all worked.
func (m *model) batchDoneLabel() (string, bool) {
	ok := m.batchTotal - m.batchFail
	noun := itemNoun(ok)

	var done string
	switch m.deleteAct {
	case actionTrash:
		done = "moved to the trash"
	case actionDelete:
		done = "deleted"
	case actionEmpty:
		done = "emptied"
	}

	if m.batchFail > 0 {
		return fmt.Sprintf("%d %s %s · %d could not be removed", ok, noun, done, m.batchFail), true
	}
	label := fmt.Sprintf("%d %s %s", ok, noun, done)
	if m.deleteAct == actionTrash && trash.RestoreSupported() {
		label += " · u to undo"
	}
	return label, false
}

// dropRow removes one entry from the list without rebuilding it, so the cursor
// stays where the user left it rather than jumping back to the top. It removes
// from both the full list and the filtered view: the item is gone from disk, and
// a filter that went on listing it would be lying.
func (m *model) dropRow(item fs.Item) {
	m.rows = removeItem(m.rows, item)
	if m.filtered != nil {
		m.filtered = removeItem(m.filtered, item)
	}
	m.clampCursor()
}

func removeItem(items []fs.Item, item fs.Item) []fs.Item {
	for i, row := range items {
		if row == item {
			return append(items[:i], items[i+1:]...)
		}
	}
	return items
}

// reloadRows re-reads the current directory, keeping the selection on the same
// *item* rather than the same row number. Re-sorting moves every row, and a cursor
// that stayed at index 4 would silently be pointing at something else — which
// matters rather a lot when the next key might be D.
//
// A live filter survives: enterDir drops it, but re-sorting a filtered list should
// reorder the matches, not throw the filter away. So it is re-applied and the
// selection is found in the filtered view.
func (m *model) reloadRows() {
	selected := m.selected()
	filtering, filter := m.filtering, m.filter

	m.enterDir(m.currentDir)

	m.filtering, m.filter = filtering, filter
	m.applyFilter()

	if selected != nil {
		for i, row := range m.items() {
			if row == selected {
				m.cursor = i
				break
			}
		}
	}
	m.clampCursor()
}

func deleteErrorText(msg deleteDoneMsg) string {
	if errors.Is(msg.err, trash.ErrCrossVolume) {
		return fmt.Sprintf("%s is on another volume; its trash cannot take it — D deletes permanently",
			msg.item.GetName())
	}
	return "could not delete " + msg.item.GetName() + ": " + msg.err.Error()
}

// actionGerund names an action for a sentence — "emptying can act on". It is only
// used in the one status line that explains why a batch had nothing to do.
func actionGerund(act action) string {
	switch act {
	case actionTrash:
		return "trashing"
	case actionDelete:
		return "deleting"
	case actionEmpty:
		return "emptying"
	}
	return ""
}

func (c *confirmState) inProgressLabel() string {
	switch c.act {
	case actionTrash:
		return "moving " + c.item.GetName() + " to the trash…"
	case actionDelete:
		return "deleting " + c.item.GetName() + "…"
	case actionEmpty:
		return "emptying " + c.item.GetName() + "…"
	}
	return ""
}

func (m *model) doneLabel(msg deleteDoneMsg) string {
	name := msg.item.GetName()
	switch msg.act {
	case actionTrash:
		if trash.RestoreSupported() {
			return name + " moved to the trash · u to undo"
		}
		// Windows can recycle but cdu cannot take it back out, so no undo is
		// offered — and the line says where the item went, not that it can return.
		return name + " moved to the trash"
	case actionDelete:
		return name + " deleted permanently"
	case actionEmpty:
		return name + " emptied"
	}
	return ""
}

// viewConfirm draws the modal over the list. The header and footer stay: the
// question is about a thing on this screen, and replacing the whole screen would
// lose the context that makes it answerable.
func (m *model) viewConfirm() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	// padLines has the last word on height. A wrapped line inside the box could
	// still push it past what fitModal budgeted for, and the frame must not grow.
	parts = append(parts, padLines(m.viewModal(), m.visibleLines()))
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return strings.Join(parts, "\n")
}

func (m *model) viewModal() string {
	c := m.confirm
	width := m.modalWidth()

	// The consequence is a sentence, so it wraps rather than being truncated —
	// "→ goes to the trash · does not free disk…" would cut off the very fact the
	// line exists to state. The name and the path are identifiers, so they are cut.
	lines := []modalLine{
		{text: m.st.danger.Render(modalTitle(c)), dropAt: keepAlways},
		{text: "", dropAt: 1},
		{text: m.st.dirName.Render(runewidth.Truncate(m.modalSubject(c), width, "…")), dropAt: 3},
	}
	// A single item shows its full path; a batch spans directories and has none to
	// show, so its subject line already carries the count and size instead.
	if len(c.batch) == 0 {
		lines = append(lines,
			modalLine{text: m.st.dim.Render(middleTruncate(c.item.GetPath(), width)), dropAt: 2})
	}
	lines = append(lines,
		modalLine{text: "", dropAt: 1},
		modalLine{text: m.st.pct.Render(modalConsequence(c.act)), dropAt: 4},
	)

	if c.requireTyping {
		lines = append(lines,
			modalLine{text: "", dropAt: 1},
			modalLine{text: m.viewTypeToConfirm(), dropAt: keepAlways},
		)
	}
	lines = append(lines,
		modalLine{text: "", dropAt: 1},
		modalLine{text: m.viewButtons(), dropAt: keepAlways},
	)

	return m.centreInList(fitModal(lines, m.visibleLines()))
}

func modalTitle(c *confirmState) string {
	if len(c.batch) > 0 {
		n := len(c.batch)
		noun := itemNoun(n)
		switch c.act {
		case actionTrash:
			return fmt.Sprintf("Move %d %s to the trash?", n, noun)
		case actionDelete:
			return fmt.Sprintf("Delete %d %s permanently?", n, noun)
		case actionEmpty:
			return fmt.Sprintf("Empty %d %s?", n, noun)
		}
		return ""
	}
	switch c.act {
	case actionTrash:
		return "Move this item to the trash?"
	case actionDelete:
		return "Delete this item permanently?"
	case actionEmpty:
		return "Empty this file?"
	}
	return ""
}

// modalSubject names the thing and says how big it is — and, for a directory, how
// many items go with it, which is the number people most often turn out not to
// have expected. A batch is summed instead: how many folders and files, and the
// total that would come back.
func (m *model) modalSubject(c *confirmState) string {
	if len(c.batch) > 0 {
		return m.batchSubject(c.batch)
	}
	name := c.item.GetName()
	size := m.ui.formatSize(m.itemSize(c.item))
	if c.item.IsDir() {
		return fmt.Sprintf("%s/  —  %s, %s items", name, size, humanCount(c.item.GetItemCount()))
	}
	return fmt.Sprintf("%s  —  %s", name, size)
}

// batchSubject counts the set by kind and sums its size, so the modal says exactly
// what a set of many things adds up to — the number nobody has in their head.
func (m *model) batchSubject(items []fs.Item) string {
	var dirs, files int
	var total int64
	for _, it := range items {
		if it.IsDir() {
			dirs++
		} else {
			files++
		}
		total += m.itemSize(it)
	}

	parts := make([]string, 0, 2)
	if dirs > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", dirs, plural(dirs, "folder", "folders")))
	}
	if files > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", files, plural(files, "file", "files")))
	}
	return strings.Join(parts, ", ") + "  —  " + m.ui.formatSize(total)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// modalConsequence is the whole point of the modal: not "are you sure", but what
// will actually be true afterwards. The trash not freeing disk space is the fact
// most likely to catch someone out, so it is said every time.
func modalConsequence(act action) string {
	switch act {
	case actionTrash:
		if trash.RestoreSupported() {
			return "→ goes to the trash · does not free disk space · u undoes it"
		}
		return "→ goes to the trash · does not free disk space · cdu cannot undo it here"
	case actionDelete:
		return "→ gone for good · frees the space · this cannot be undone"
	case actionEmpty:
		return "→ truncated to 0 bytes · this cannot be undone"
	}
	return ""
}
