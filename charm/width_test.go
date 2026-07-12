package charm

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// TestRowWidthsUnderTruecolor guards against measuring or truncating a string
// that already carries escape sequences. A rune counter sees those bytes as
// visible columns, so a styled row cut to the terminal width loses most of its
// content — the selected row once rendered 41 columns wide in a 100-column
// terminal, with the size and percentage columns chopped off entirely.
//
// The test process has no TTY, so Lipgloss would fall back to a plain-ASCII
// profile and emit no escapes at all, hiding exactly the bug we care about.
// Force truecolor, as in a real terminal.
func TestRowWidthsUnderTruecolor(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	for _, width := range []int{40, 60, 80, 100, 200} {
		m := benchModel(50)
		m.width, m.height = width, 30
		m.haveSize = true
		m.st = newStyles(charmPalette(), true)

		total := m.itemSize(m.currentDir)

		selected := m.viewRow(m.rows[0], true, total)
		unselected := m.viewRow(m.rows[1], false, total)

		assert.Equal(t, width, lipgloss.Width(selected),
			"selected row must fill exactly %d columns", width)
		assert.Equal(t, width, lipgloss.Width(unselected),
			"unselected row must fill exactly %d columns", width)

		assert.LessOrEqual(t, lipgloss.Width(m.viewFooter()), width, "footer overflows")
		assert.LessOrEqual(t, lipgloss.Width(m.viewHeader()), width*2, "header overflows")
	}
}
