package charm

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

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
	ui := CreateUI(nil, useColors, false, false, false)
	ui.noUnicode = noUnicode

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
				assert := colourEscape.FindString(out)
				if assert != "" {
					t.Errorf("prof=%v noUni=%v screen=%s: colour escape %q leaked under --no-color",
						prof, noUni, name, assert)
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
