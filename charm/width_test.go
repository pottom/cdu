package charm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"

	"github.com/pottom/cdu/pkg/device"
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
	withProfile(t, termenv.TrueColor)

	for _, width := range []int{40, 60, 80, 100, 200} {
		m := benchModel(50)
		m.width, m.height = width, 30
		m.haveSize = true

		total := m.itemSize(m.currentDir)

		selected := m.viewRow(m.rows[0], true, total)
		unselected := m.viewRow(m.rows[1], false, total)

		assert.Equal(t, width, lipgloss.Width(selected),
			"selected row must fill exactly %d columns", width)
		assert.Equal(t, width, lipgloss.Width(unselected),
			"unselected row must fill exactly %d columns", width)

		// The bar line is drawn from styled cells too, so it is exposed to exactly
		// the same escape-blind truncation bug as the row above it.
		if m.linesPerEntry() > 1 {
			for _, sel := range []bool{true, false} {
				bar := m.viewBar(m.rows[0], sel, total)
				assert.Equal(t, width, lipgloss.Width(bar),
					"bar line must fill exactly %d columns (selected=%v)", width, sel)
			}
		}

		assert.LessOrEqual(t, lipgloss.Width(m.viewFooter()), width, "footer overflows")

		// The header is only exercised honestly once a disk has resolved: the disk
		// line is built from a bar and two padded cells, so it is the line most
		// likely to overflow.
		m.dev = &device.Device{Name: "Macintosh HD", MountPoint: "/", Size: 994 << 30, Free: 210 << 30}
		for _, line := range strings.Split(m.viewHeader(), "\n") {
			assert.LessOrEqual(t, lipgloss.Width(line), width, "header line overflows at width %d", width)
		}
		assert.Equal(t, m.headerHeight(), len(strings.Split(m.viewHeader(), "\n")),
			"headerHeight must match the lines viewHeader actually renders")
	}
}

// The frame must be exactly as tall as the terminal at every size, on every
// screen — including the sizes where the list height is not a whole number of
// two-line entries. One line too many and the terminal scrolls on every render.
func TestFrameHeight(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	for _, scr := range []screen{screenBrowse, screenScanning, screenConfirm} {
		for _, width := range []int{40, 79, 80, 120} {
			for _, height := range []int{1, 2, 3, 4, 5, 6, 11, 24, 50} {
				m := benchModel(50)
				m.width, m.height = width, height
				m.haveSize = true
				m.scr = scr
				m.progress.CurrentItemName = "/some/deeply/nested/path/being/walked/right/now"
				// The modal is the tallest thing the interface draws, so it is the one
				// most able to push the footer off the bottom of a short terminal.
				m.confirm = &confirmState{
					item: m.rows[0], parent: m.currentDir,
					act: actionDelete, requireTyping: true,
				}
				// With a disk the header is three lines, which is the case where the
				// list height stops being a whole number of two-line entries.
				m.dev = &device.Device{Name: "Macintosh HD", MountPoint: "/", Size: 994 << 30, Free: 210 << 30}

				lines := strings.Split(m.View(), "\n")
				assert.Len(t, lines, height,
					"frame must be exactly %d lines on screen %d at %dx%d", height, scr, width, height)
			}
		}
	}
}
