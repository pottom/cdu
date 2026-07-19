package charm

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pottom/cdu/internal/theme"
)

// `p` opens the theme picker — p for palette. The list re-themes the whole interface
// live as the cursor moves, so the picker is its own preview: you see a theme on the
// real screen, not on a swatch. Enter keeps the one you land on and writes it to the
// config, the way t→s saves the rest of the view — a theme is a thing you decide, not
// a thing you try and lose. Esc restores the theme you opened on.

// applyTheme swaps the theme and rebuilds everything painted from it — the styles,
// the bar's precomputed gradient, the spinner's colour. It is the whole of what a
// live re-theme costs.
func (m *model) applyTheme(th *theme.Theme) {
	m.ui.theme = *th
	m.st = newStyles(&m.ui.theme, m.ui.UseColors)
	m.bar = newBarRenderer(&m.ui.theme, m.ui.UseColors, m.ui.noUnicode)
	m.spinner.Style = m.st.accent
}

func (m *model) openThemePicker() (tea.Model, tea.Cmd) {
	m.themeNames = theme.Names()
	m.themeOriginal = m.ui.theme
	m.themeCursor, m.themeOffset = 0, 0
	for i, n := range m.themeNames {
		if n == m.ui.theme.Name {
			m.themeCursor = i
			break
		}
	}
	m.status, m.statusIsError = "", false
	m.scr = screenThemes
	return m, nil
}

func (m *model) handleThemeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape, keyLeft, "h", "p":
		// Cancel: put back the theme we opened on, overrides and all.
		m.applyTheme(&m.themeOriginal)
		m.scr = screenBrowse
		return m, nil
	case keyUp, "k":
		m.moveThemeCursor(-1)
	case keyDown, "j":
		m.moveThemeCursor(1)
	case keyHome, "g":
		m.moveThemeCursor(-len(m.themeNames))
	case keyEnd, "G":
		m.moveThemeCursor(len(m.themeNames))
	case keyPgUp:
		m.moveThemeCursor(-m.visibleLines())
	case keyPgDown:
		m.moveThemeCursor(m.visibleLines())
	case keyEnter, keyRight, "l":
		// Keep it: picking a theme is a decision, so it persists like a saved view.
		m.themeOriginal = m.ui.theme
		m.scr = screenBrowse
		cmd := m.saveView()
		return m, cmd
	}
	return m, nil
}

// moveThemeCursor moves the cursor and applies the theme it lands on, so the whole
// screen previews it at once.
func (m *model) moveThemeCursor(delta int) {
	if len(m.themeNames) == 0 {
		return
	}
	m.themeCursor = min(max(m.themeCursor+delta, 0), len(m.themeNames)-1)

	height := max(m.visibleLines(), 1)
	m.themeOffset = min(m.themeOffset, m.themeCursor)
	if m.themeCursor >= m.themeOffset+height {
		m.themeOffset = m.themeCursor - height + 1
	}
	m.themeOffset = min(max(m.themeOffset, 0), max(len(m.themeNames)-height, 0))

	if th, ok := theme.Preset(m.themeNames[m.themeCursor]); ok {
		m.applyTheme(&th)
	}
}

func (m *model) viewThemeList() string {
	lines := m.visibleLines()
	if len(m.themeNames) == 0 {
		return padLines(m.st.dim.Render(clipTo("  no themes", m.width)), lines)
	}

	end := min(m.themeOffset+lines, len(m.themeNames))
	rows := make([]string, 0, lines)
	for i := m.themeOffset; i < end; i++ {
		rows = append(rows, m.viewThemeRow(m.themeNames[i], i == m.themeCursor))
	}
	return padLines(joinLines(rows), lines)
}

// viewThemeRow shows a theme's name in its own accent — a swatch you can compare
// down the list without selecting each. The cursor row is the live preview: the
// whole screen is already wearing that theme, so it takes the selection band.
func (m *model) viewThemeRow(name string, selected bool) string {
	if m.width < 1 {
		return ""
	}
	if selected {
		return m.st.accent.Render("▌") + m.st.selected.Render(cell(" "+name, max(m.width-1, 1)))
	}

	th, ok := theme.Preset(name)
	style := lipgloss.NewStyle().Bold(true)
	if ok && !th.Plain {
		style = style.Foreground(lg(th.Accent))
	}
	return "  " + style.Render(cell(name, max(m.width-2, 1)))
}

func (m *model) viewThemes() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewThemeList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
