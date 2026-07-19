package charm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pottom/cdu/pkg/fs"
)

// Marking is space, and it queues an item for a batch delete rather than acting
// at once. The set is keyed by fs.Item, not by row, so a mark holds while you move
// between directories and while the list re-sorts under it — gdu keys marks by row
// number and loses them the instant either happens.
//
// The destructive keys then act on the whole set when it is non-empty, and on the
// cursor row alone when it is empty. That keeps the one-key delete anyone already
// knows, and adds the batch without a mode to enter or leave.

func (m *model) isMarked(item fs.Item) bool {
	return item != nil && m.marked[item]
}

// toggleMark adds or removes the cursor row, and advances the cursor so a run can
// be marked with a held space — the one affordance that makes marking many rows
// bearable, and what gdu's space does too.
func (m *model) toggleMark(item fs.Item) {
	if item == nil {
		return
	}
	if m.marked[item] {
		delete(m.marked, item)
	} else {
		m.marked[item] = true
	}
	m.status, m.statusIsError = "", false
}

func (m *model) clearMarks() {
	m.marked = make(map[fs.Item]bool)
}

// unmarkAll drops the whole selection at once and says it did — the "unmark all"
// to space's mark-one. Silent when there was nothing marked, so a stray esc on an
// unmarked list does not flash a message about a set that was never there.
func (m *model) unmarkAll() {
	if m.markedCount() == 0 {
		return
	}
	m.clearMarks()
	m.status, m.statusIsError = "marks cleared", false
}

func (m *model) markedCount() int { return len(m.marked) }

// markUnderCursor toggles the item the cursor is on, on whichever list is showing —
// the browser, the largest-files or duplicate screens, or find. It reads the cursor
// through target(), so one helper serves every screen; advancing the cursor is the
// caller's job, since each list moves its own.
func (m *model) markUnderCursor() {
	item, _ := m.target()
	if item == nil || m.isParentRow(item) {
		return
	}
	m.toggleMark(item)
}

// marksActOnSet reports whether the destructive keys should act on the marked set
// rather than the cursor row: on every list that can mark, which is all of them
// except the modes that have no list.
func (m *model) marksActOnSet() bool {
	//nolint:exhaustive // the default covers every screen that has no list to mark
	switch m.scr {
	case screenBrowse, screenQueue, screenTop, screenDup, screenFind:
		return true
	default:
		return false
	}
}

// markOverlay reports whether a row should wear the mark — the ✗ and the struck
// name. It is drawn on the browsing lists but not on the queue, where every row is
// marked by definition and marking them all again would just be noise.
func (m *model) markOverlay(item fs.Item) bool {
	return m.scr != screenQueue && m.isMarked(item)
}

// renderMarkedName renders a row's name column when the row is marked: the visible
// name struck through and in the danger colour, then its trailing padding left plain,
// so the strike covers the text and not the empty rest of the column. base carries
// whatever the row already has — the file colour, or the selection background on the
// cursor row — so the padding still belongs to the row.
func (m *model) renderMarkedName(nameText string, base *lipgloss.Style) string {
	name := strings.TrimRight(nameText, " ")
	pad := nameText[len(name):]
	return m.markedNameStyle(base).Render(name) + base.Render(pad)
}

// markedNameStyle is the style of a marked name: struck through and in the danger
// colour, so it reads as bound for deletion both by the line through it and by a
// colour unlike an unmarked row's. Kept in one place so the cursor row (over the
// selection background) and the plain rows agree; under no colour the strike alone
// carries it.
func (m *model) markedNameStyle(base *lipgloss.Style) lipgloss.Style {
	return base.Strikethrough(true).Foreground(lg(m.ui.theme.Danger))
}

// isAncestorMarked reports whether a marked directory contains this item. Such an
// item is already covered — deleting the ancestor takes it too — so it must not be
// counted a second time in the reclaimable total, nor deleted on its own.
func (m *model) isAncestorMarked(item fs.Item) bool {
	for p := item.GetParent(); p != nil; p = p.GetParent() {
		if m.marked[p] {
			return true
		}
	}
	return false
}

// effectiveMarks is the set a batch action really operates on: the marked items,
// minus any that sit under another marked item, biggest first. For emptying, which
// only a file can undergo, directories drop out too.
func (m *model) effectiveMarks(act action) []fs.Item {
	out := make([]fs.Item, 0, len(m.marked))
	for item := range m.marked {
		if m.isAncestorMarked(item) {
			continue
		}
		if act == actionEmpty && item.IsDir() {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return m.itemSize(out[i]) > m.itemSize(out[j])
	})
	return out
}

// markedReclaimable is the space a permanent delete of the set would free — the
// deduped sizes summed, so a marked file inside a marked folder is not added twice.
func (m *model) markedReclaimable() int64 {
	var total int64
	for _, item := range m.effectiveMarks(actionDelete) {
		total += m.itemSize(item)
	}
	return total
}

// markTally is the running "✓ N marked · <size>" the header shows while anything
// is marked, so the queue building up is never invisible. Empty when nothing is.
func (m *model) markTally() string {
	n := m.markedCount()
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d marked · %s", n, m.ui.formatSize(m.markedReclaimable()))
}

// itemNoun is "item" or "items" — the count-dependent word the tally, the queue
// header and the batch labels all need, kept in one place.
func itemNoun(n int) string {
	if n == 1 {
		return "item"
	}
	return "items"
}
