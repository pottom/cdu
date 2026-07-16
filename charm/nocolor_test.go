package charm

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/theme"
	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/device"
)

// colourEscape matches an SGR sequence that sets a colour: the 38/48 extended
// forms and the 30–37 / 40–47 / 90–97 / 100–107 basic ones. It deliberately does
// not match bold (1), underline (4) or reverse (7) — those are how the interface
// conveys state without colour, so they are meant to survive --no-color.
var colourEscape = regexp.MustCompile(`\x1b\[[0-9;]*(38|48|3[0-7]|4[0-7]|9[0-7]|10[0-7])[;m]`)

var anyEscape = regexp.MustCompile("\x1b")

// auditModel builds a model on a directory with a few entries, ready to render
// any screen.
func auditModel(t *testing.T, useColors, noUnicode bool) *model {
	t.Helper()
	return auditModelWith(t, theme.Charm(), useColors, noUnicode)
}

func auditModelWith(t *testing.T, th theme.Theme, useColors, noUnicode bool) *model {
	t.Helper()
	ui := CreateUI(nil, useColors, false, false, false)
	ui.noUnicode = noUnicode
	ui.theme = th

	m := newModel(ui)
	m.width, m.height, m.haveSize = 90, 16, true

	dir := &analyze.Dir{File: &analyze.File{Name: "root"}, BasePath: "/"}
	for _, n := range []string{"a-file.txt", "subdir", "b.bin"} {
		dir.AddFile(&analyze.File{Name: n, Size: 4096, Usage: 8192, Parent: dir})
	}
	m.topDir = dir
	m.enterDir(dir)
	m.dev = &device.Device{Name: "Disk", MountPoint: "/", Size: 1 << 40, Free: 1 << 39}
	return m
}

func renderEachScreen(m *model, visit func(name, out string)) {
	screens := []struct {
		name  string
		setup func()
	}{
		{"browse", func() { m.scr = screenBrowse }},
		{"scan", func() { m.scr = screenScanning; m.progress.ItemCount = 5 }},
		{"confirm", func() {
			m.scr = screenConfirm
			m.confirm = &confirmState{item: m.rows[0], parent: m.currentDir, act: actionTrash}
		}},
		{"filter", func() { m.scr = screenBrowse; m.filtering = true; m.filter = "a"; m.applyFilter() }},
		{"viewer", func() {
			m.scr = screenViewer
			m.viewer = &viewerState{path: "/x", lines: []string{"hello", "world"}}
		}},
	}
	for _, s := range screens {
		s.setup()
		visit(s.name, m.View())
		m.filtering, m.filter, m.filtered = false, "", nil
		m.confirm, m.viewer = nil, nil
	}
}

// Under a colour-capable terminal but --no-color, no screen may emit a colour
// escape. Bold, underline and reverse are allowed and expected — they are the
// state cues that replace colour.
func TestNoColorEmitsNoColourEscapes(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)

	for _, prof := range []termenv.Profile{termenv.ANSI256, termenv.TrueColor} {
		lipgloss.SetColorProfile(prof)
		for _, noUni := range []bool{false, true} {
			m := auditModel(t, false /* useColors */, noUni)
			renderEachScreen(m, func(name, out string) {
				if esc := colourEscape.FindString(out); esc != "" {
					t.Errorf("prof=%v noUni=%v screen=%s: colour escape %q leaked under --no-color",
						prof, noUni, name, esc)
				}
			})
		}
	}
}

// Under the Ascii profile — a dumb terminal — Lipgloss strips every attribute, so
// there must be no escape sequence of any kind on any screen.
func TestAsciiProfileEmitsNoEscapes(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)
	lipgloss.SetColorProfile(termenv.Ascii)

	for _, useColors := range []bool{true, false} {
		for _, noUni := range []bool{true, false} {
			m := auditModel(t, useColors, noUni)
			renderEachScreen(m, func(name, out string) {
				if anyEscape.MatchString(out) {
					t.Errorf("useColors=%v noUni=%v screen=%s: escape reached a dumb terminal",
						useColors, noUni, name)
				}
			})
		}
	}
}

// --no-unicode is scoped, as in gdu, to the size bar: it must not draw the block
// runes. Other chrome (the marker, the rule) is out of scope, matching gdu.
func TestNoUnicodeKeepsTheBarAscii(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)
	lipgloss.SetColorProfile(termenv.TrueColor)

	m := auditModel(t, true, true /* noUnicode */)
	renderEachScreen(m, func(name, out string) {
		if strings.ContainsAny(out, "█░") {
			t.Errorf("screen=%s: block runes drawn under --no-unicode", name)
		}
	})
}

// mono is a theme with no tokens: it is meant to render through the same path as
// --no-color, on a colour-capable terminal, with colour switched on everywhere
// else. If that wiring is ever lost, mono silently becomes black-on-black rather
// than colourless, so it is asserted rather than assumed.
func TestMonoThemeIsColourlessOnACapableTerminal(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)

	for _, prof := range []termenv.Profile{termenv.ANSI256, termenv.TrueColor} {
		lipgloss.SetColorProfile(prof)
		m := auditModelWith(t, theme.Mono(), true /* useColors */, false)
		renderEachScreen(m, func(name, out string) {
			if esc := colourEscape.FindString(out); esc != "" {
				t.Errorf("prof=%v screen=%s: mono emitted a colour escape %q", prof, name, esc)
			}
		})
	}
}

// A theme may change what a cell looks like, never how wide it is. Colour is not
// a layout input, so every preset must lay out identically to the default, down
// to the column — including mono, which renders through a different path
// entirely and so is the one most able to drift.
//
// Comparing against charm rather than against the terminal width is deliberate:
// it asks the question this test is for, and does not quietly re-litigate what
// the layout should do in a terminal too narrow to hold its own floors.
func TestEveryPresetLaysOutIdenticallyToTheDefault(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)
	lipgloss.SetColorProfile(termenv.TrueColor)

	sizes := []struct{ w, h int }{{40, 3}, {40, 24}, {90, 24}, {200, 24}}

	for _, name := range theme.Names() {
		th, ok := theme.Preset(name)
		require.True(t, ok, "Names() offered %q but Preset() does not know it", name)

		t.Run(name, func(t *testing.T) {
			for _, size := range sizes {
				ref := auditModelWith(t, theme.Charm(), true, false)
				ref.width, ref.height = size.w, size.h
				got := auditModelWith(t, th, true, false)
				got.width, got.height = size.w, size.h

				want := map[string][]int{}
				renderEachScreen(ref, func(screen, out string) { want[screen] = lineWidths(out) })
				renderEachScreen(got, func(screen, out string) {
					assert.Equal(t, want[screen], lineWidths(out),
						"screen=%s at %dx%d: %s lays out differently to charm", screen, size.w, size.h, name)
				})
			}
		})
	}
}

func lineWidths(out string) []int {
	var widths []int
	for line := range strings.SplitSeq(out, "\n") {
		widths = append(widths, lipgloss.Width(line))
	}
	return widths
}

// The whole point of the exercise: no combination of profile and flags panics.
func TestNoScreenPanicsAcrossProfiles(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)

	for _, prof := range []termenv.Profile{termenv.Ascii, termenv.ANSI256, termenv.TrueColor} {
		lipgloss.SetColorProfile(prof)
		for _, useColors := range []bool{true, false} {
			for _, noUni := range []bool{true, false} {
				m := auditModel(t, useColors, noUni)
				for _, w := range []int{1, 20, 80, 200} {
					for _, h := range []int{1, 3, 24} {
						m.width, m.height = w, h
						renderEachScreen(m, func(_, out string) { _ = out })
					}
				}
			}
		}
	}
}
