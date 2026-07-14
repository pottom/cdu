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
)

// modalWidth is the room the text has. Lipgloss counts padding inside Width, so
// the two are kept apart here: get this wrong and every line wraps one word early.
// It never goes below one column, so a hostile terminal size clamps rather than
// panicking on a negative repeat count.
func (m *model) modalWidth() int {
	return max(m.boxWidth()-modalPadding*2, 1)
}

// boxWidth is what Lipgloss is told, and excludes only the border it draws around
// the outside.
func (m *model) boxWidth() int {
	return max(min(m.width-4, modalMaxWidth)-modalBorder, 1+modalPadding*2)
}

// centreInList places the modal in the space the list occupies, so the header and
// footer stay put. lipgloss.Place pads to exactly these dimensions, which is what
// keeps the frame the right height.
func (m *model) centreInList(content string) string {
	box := m.st.modal.Width(m.boxWidth()).Render(content)

	return lipgloss.Place(
		max(m.width, 1), max(m.visibleLines(), 1),
		lipgloss.Center, lipgloss.Center,
		box,
	)
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

	label := m.st.danger.Render("this path is protected — type " + confirmWord + " to confirm")
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
