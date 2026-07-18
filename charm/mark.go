package charm

import (
	"fmt"
	"sort"

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

// markGlyph is the tick drawn in a marked row's gutter, or its ASCII stand-in when
// unicode is off. It measures one cell either way, so a marked row stays the exact
// width an unmarked one is.
func (m *model) markGlyph() string {
	if m.ui.noUnicode {
		return "*"
	}
	return "✓"
}

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

func (m *model) markedCount() int { return len(m.marked) }

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
	return fmt.Sprintf("%s %d %s · %s", m.markGlyph(), n, itemNoun(n), m.ui.formatSize(m.markedReclaimable()))
}

// itemNoun is "item" or "items" — the count-dependent word the tally, the queue
// header and the batch labels all need, kept in one place.
func itemNoun(n int) string {
	if n == 1 {
		return "item"
	}
	return "items"
}
