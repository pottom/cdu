package charm

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

const (
	// modalMaxWidth keeps the box readable on a wide terminal. A question spread
	// across 200 columns is a question nobody reads.
	modalMaxWidth = 64
	modalPadding  = 2
	modalBorder   = 2
	// modalMargin is the gap between the box and the edges of the screen. It is
	// the first thing given up when the terminal is small — it is only air.
	modalMargin = 4
)

// modalWidth is the room the text has. Lipgloss counts padding inside Width, so
// the two are kept apart here: get this wrong and every line wraps one word early.
// It never goes below one column, so a hostile terminal size clamps rather than
// panicking on a negative repeat count.
func (m *model) modalWidth() int {
	return max(m.boxWidth()-m.modalPad()*2, 1)
}

// modalPad is the breathing room inside the box, and the second thing given up
// after the margin. Before the border, which is what makes the box legible as a
// box at all.
func (m *model) modalPad() int {
	return min(modalPadding, max((m.boxWidth()-1)/2, 0))
}

// boxWidth is what Lipgloss is told, and excludes only the border it draws around
// the outside.
//
// The hard limit is the terminal itself: a box wider than the screen is wrapped
// by the terminal, which pushes the frame down on every render — the horizontal
// form of the bug padLines exists for. So the margin goes first, then the
// padding, and the box still fits.
func (m *model) boxWidth() int {
	roomy := min(m.width-modalMargin, modalMaxWidth) - modalBorder
	return max(min(roomy, m.width-modalBorder), 1)
}

// centreInList lays the modal over the list it was opened from, so the directory
// you are acting on stays visible above and below the question rather than the whole
// list blanking out. Only the band the box covers is replaced; the header and footer
// are added by the caller.
func (m *model) centreInList(content string) string {
	height := max(m.visibleLines(), 1)

	// A border costs two columns and the content needs at least one. With less than
	// that there is no box to draw, and drawing one anyway would overflow — the list
	// alone is the honest answer for a two-column terminal.
	if m.width < modalBorder+1 {
		return padLines(m.listBody(), height)
	}

	box := m.st.modal.Padding(0, m.modalPad()).Width(m.boxWidth()).Render(content)
	boxLines := strings.Split(box, "\n")

	lines := strings.Split(padLines(m.listBody(), height), "\n")
	boxW := lipgloss.Width(boxLines[0])
	left := max((m.width-boxW)/2, 0)
	top := max((len(lines)-len(boxLines))/2, 0)

	// Each box row replaces one list row, centred on blank: the sides are cleared
	// rather than composited, since slicing the list line around the box would mean
	// cutting a styled string mid-escape. The rows the box does not reach keep the
	// list, which is the point.
	for i, bl := range boxLines {
		row := top + i
		if row >= len(lines) {
			break
		}
		pad := max(m.width-left-lipgloss.Width(bl), 0)
		lines[row] = strings.Repeat(" ", left) + bl + strings.Repeat(" ", pad)
	}
	return strings.Join(lines, "\n")
}

// listBody is the list behind the modal: the body of whichever screen the modal was
// opened from, so the backdrop is the directory or the list you were acting on.
func (m *model) listBody() string {
	//nolint:exhaustive // every other origin falls through to the browser's list
	switch m.confirmFrom {
	case screenTop:
		return m.viewTopList()
	case screenDup:
		return m.viewDupList()
	case screenFind:
		return m.viewFindList()
	case screenQueue:
		return m.viewQueueList()
	default:
		return m.viewList()
	}
}

// modalLine is one line of the box, and how readily it can be given up when the
// terminal is too short to hold all of them. A modal that overflowed the list
// area would push the footer off the screen and make the frame scroll.
type modalLine struct {
	text string
	// dropAt is the order lines are shed in: the lowest goes first. A line with
	// dropAt 0 is never dropped — the question, the guard, and the buttons.
	dropAt int
}

const keepAlways = 0

// fitModal sheds the least essential lines until the box fits the space it has.
// It is deliberately not a scroll: a confirmation you have to scroll to read is a
// confirmation people click through.
func fitModal(lines []modalLine, maxLines int) string {
	// Two lines go to the border Lipgloss draws.
	budget := maxLines - modalBorder

	for level := 1; len(lines) > budget && level <= 4; level++ {
		kept := lines[:0]
		for _, line := range lines {
			if line.dropAt == keepAlways || line.dropAt > level {
				kept = append(kept, line)
			}
		}
		lines = kept
	}

	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, line.text)
	}
	// If even the essentials do not fit, the frame's height still wins: the outer
	// padLines clips. Nothing here may make View return more lines than it has.
	return strings.Join(texts, "\n")
}

// viewButtons renders Cancel and the destructive action. Cancel holds the focus
// on entry, so a reflexive Enter cancels — the destructive button has to be
// chosen, not merely arrived at.
func (m *model) viewButtons() string {
	c := m.confirm

	confirmLabel := map[action]string{
		actionTrash:  "Move to Trash",
		actionDelete: "Delete Permanently",
		actionEmpty:  "Empty File",
	}[c.act]

	cancel := m.st.button.Render("  Cancel  ")
	confirm := m.st.button.Render("  " + confirmLabel + "  ")

	switch {
	case !c.confirmFocused:
		cancel = m.st.buttonFocus.Render("  Cancel  ")
	case c.requireTyping && !c.typedFully():
		// Unreachable by design — focus cannot move here until the word is typed —
		// but if it ever were, the button must not look ready to fire.
		confirm = m.st.button.Render("  " + confirmLabel + "  ")
	default:
		confirm = m.st.buttonDanger.Render("  " + confirmLabel + "  ")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cancel, "  ", confirm)
}

// viewTypeToConfirm is the guard on a protected path: somewhere you can delete by
// reflex is somewhere a single keypress must not be enough.
func (m *model) viewTypeToConfirm() string {
	c := m.confirm

	typed := c.typed
	if remaining := len(confirmWord) - len(typed); remaining > 0 {
		typed += strings.Repeat("_", remaining)
	}

	// The reason to slow down differs: a protected path is one you might have reached
	// by accident, an elevated delete is one root itself will carry out. Both ask for
	// the word; each says why.
	reason := "this path is protected"
	if c.elevated {
		reason = "this runs as root and cannot be undone"
	}
	label := m.st.danger.Render(reason + " — type " + confirmWord + " to confirm")
	field := m.st.dirName.Render(typed)
	return label + "\n" + field
}

// emptiedFile is the zero-sized file that replaces a truncated one, so the tree
// shows what is now on disk rather than what used to be.
func emptiedFile(old, parent fs.Item) *analyze.File {
	return &analyze.File{
		Name:   old.GetName(),
		Flag:   old.GetFlag(),
		Size:   0,
		Usage:  0,
		Mtime:  old.GetMtime(),
		Parent: parent,
	}
}
