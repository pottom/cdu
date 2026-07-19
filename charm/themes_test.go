package charm

import (
	"strings"
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

// The picker is a box floating over the browser: the directory you were in stays
// on screen behind it — the whole point of previewing a theme on your own content —
// and every line still fits the terminal's width exactly.
func TestThemePickerFloatsOverTheList(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(8)
	m.width, m.height, m.haveSize = 80, 24, true
	rowName := m.rows[0].GetName()

	m.openThemePicker()
	view := m.View()

	assert.Contains(t, view, "Theme", "the picker box is shown")
	assert.Contains(t, view, rowName, "and the directory behind it stays visible")
	for i, line := range strings.Split(view, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 80, "line %d overflows", i)
	}
}
