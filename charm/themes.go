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
	m.themeCursor = 0
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
// screen previews it at once. The box holds every theme at once, so there is nothing
// to scroll.
func (m *model) moveThemeCursor(delta int) {
	if len(m.themeNames) == 0 {
		return
	}
	m.themeCursor = min(max(m.themeCursor+delta, 0), len(m.themeNames)-1)
	if th, ok := theme.Preset(m.themeNames[m.themeCursor]); ok {
		m.applyTheme(&th)
	}
}

// viewThemes floats the picker as a box over the browser, so a theme is previewed
// on the real screen — your own directory, header and bars — not on a list of names.
// The header and footer are the browser's, re-themed live like everything behind the
// box.
func (m *model) viewThemes() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	box := m.st.modal.Padding(0, m.modalPad()).Render(m.themePickerContent())
	parts = append(parts, m.overlayBox(box, m.viewList()))
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}

// themePickerContent is the inside of the box: a heading, the themes each in their
// own accent so the list doubles as a set of swatches, and a one-line reminder of
// the keys. The cursor is a caret rather than a filled band, since the box already
// sits on the panel colour and a second fill would not show.
func (m *model) themePickerContent() string {
	lines := make([]string, 0, len(m.themeNames)+3)
	lines = append(lines, m.st.accent.Render("Theme"), "")
	for i, name := range m.themeNames {
		lines = append(lines, m.themeBoxRow(name, i == m.themeCursor))
	}
	lines = append(lines, "", m.st.dim.Render("↑↓ preview · ↵ keep · esc cancel"))
	return joinLines(lines)
}

func (m *model) themeBoxRow(name string, selected bool) string {
	th, ok := theme.Preset(name)
	style := lipgloss.NewStyle().Bold(true)
	if ok && !th.Plain {
		style = style.Foreground(lg(th.Accent))
	}

	marker := "  "
	if selected {
		marker = m.st.accent.Render("▸ ")
	}
	return marker + style.Render(name)
}
