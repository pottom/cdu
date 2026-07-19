package charm

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The picker previews live: moving the cursor swaps the whole interface's theme, so
// what you see is the theme, not a swatch of it.
func TestThemePickerPreviewsLive(t *testing.T) {
	m := benchModel(4)
	original := m.ui.theme.Name

	m.openThemePicker()
	require.Equal(t, screenThemes, m.scr)

	m = press(t, m, "down")
	assert.NotEqual(t, original, m.ui.theme.Name, "moving the cursor applies a different theme")
}

// Esc puts back the theme you opened on, so a preview you did not want costs nothing.
func TestThemePickerEscReverts(t *testing.T) {
	m := benchModel(4)
	original := m.ui.theme.Name

	m.openThemePicker()
	m = press(t, m, "down", "down", "esc")

	assert.Equal(t, original, m.ui.theme.Name, "esc restores the original theme")
	assert.Equal(t, screenBrowse, m.scr)
}

// Enter keeps the highlighted theme and writes it to the config — a theme is a
// decision, like a saved view, so it persists.
func TestKeepingAThemeSavesItsName(t *testing.T) {
	var saved ViewSettings
	var calls int
	m := saveModel(t, func(v ViewSettings) (string, error) {
		calls, saved = calls+1, v
		return "/home/x/.config/cdu/cdu.yaml", nil
	})
	original := m.ui.theme.Name

	m.openThemePicker()
	m.moveThemeCursor(1)
	picked := m.ui.theme.Name
	require.NotEqual(t, original, picked)

	// Enter keeps it and returns the save command; run the command to reach the saver.
	next, cmd := m.Update(key("enter"))
	m = next.(*model)
	require.NotNil(t, cmd)
	cmd()

	assert.Equal(t, screenBrowse, m.scr)
	assert.Equal(t, picked, m.ui.theme.Name, "the picked theme stays")
	require.Equal(t, 1, calls, "the config was written")
	assert.Equal(t, picked, saved.ThemeName, "and it is the picked theme's name that is written")
}

// Every theme name is drawn in its own accent, so the list is a set of swatches you
// can read without selecting each — and the row stays exactly the terminal's width.
func TestThemeRowsAreWidthExact(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(1)
	m.openThemePicker()
	for _, width := range []int{20, 40, 80} {
		m.width = width
		for i, name := range m.themeNames {
			row := m.viewThemeRow(name, i == m.themeCursor)
			assert.Equal(t, width, lipgloss.Width(row), "theme row %q at width %d", name, width)
		}
	}
}
