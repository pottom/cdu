package theme

import (
	"testing"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/stretchr/testify/require"
)

// The bug this file exists for: Ink used to be Selected, on the grounds that
// both are "the bright one". In charm they are the same white and the mistake
// costs nothing — which is why it survived. It only surfaces on a theme whose
// danger colour is light, where that same white landed on it at 1.3:1 and the
// delete button became unreadable, in a theme that is otherwise dark.
//
// Colour pairings are not checkable by eye across a set of themes, and this one
// only shows up in a modal nobody opens by accident. So they are checked here.
//
// The bar is WCAG AA for large or bold text, 3:1, which is what every one of
// these is: the buttons are bold, the cursor row is bold, the size column is a
// short bold-weight figure. Ordinary body text would want 4.5:1, but no theme
// here has body text on a coloured chip.
const minContrast = 3.0

// contrast is the WCAG relative-luminance ratio between two colours.
func contrast(t *testing.T, a, b Color) float64 {
	t.Helper()
	ca, err := colorful.Hex(string(a))
	require.NoError(t, err, "%q", a)
	cb, err := colorful.Hex(string(b))
	require.NoError(t, err, "%q", b)
	return contrastOf(ca, cb)
}

func contrastOf(a, b colorful.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c colorful.Color) float64 {
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// Every foreground must be legible on the surface it is actually drawn on. The
// pairs are taken from newStyles in charm/style.go — if a style there starts
// combining two tokens in a new way, it belongs in this list.
func TestEveryPresetIsLegible(t *testing.T) {
	for _, name := range Names() {
		th, ok := Preset(name)
		require.True(t, ok)
		if th.Plain {
			continue // no colour, nothing to contrast
		}

		t.Run(name, func(t *testing.T) {
			pairs := []struct {
				what     string
				fg, bg   Color
				drawnOn  string
				minRatio float64
			}{
				{"the cursor row's name", th.Selected, th.Panel, "panel", minContrast},
				{"the modal's body text", th.Text, th.Panel, "panel", minContrast},
				{"an unfocused button", th.Dim, th.Panel, "panel", minContrast},
				{"the focused button", th.Ink, th.Dim, "dim", minContrast},
				{"the destructive button", th.Ink, th.Danger, "danger", minContrast},
				{"a filter match on the cursor row", th.Accent, th.Panel, "panel", minContrast},
			}
			for _, p := range pairs {
				got := contrast(t, p.fg, p.bg)
				if got < p.minRatio {
					t.Errorf("%s: %s (%s) on %s (%s) is %.2f:1, want %.1f:1",
						p.what, p.fg, colourName(&th, p.fg), p.bg, p.drawnOn, got, p.minRatio)
				}
			}
		})
	}
}

// The cursor row has to lift off the panel rather than merely sit on it, which
// means Selected cannot be the same colour as an ordinary file name. A theme
// where the two are equal looks flat, and the reason is hard to spot: everything
// is present, the row just does not read as selected.
func TestSelectedLiftsOffTheOrdinaryText(t *testing.T) {
	for _, name := range Names() {
		th, ok := Preset(name)
		require.True(t, ok)
		if th.Plain || th.Light {
			// A light theme can genuinely run out of room: its `text` may already be
			// the darkest ink it has, leaving the cursor row to lean on its panel, its
			// marker and its weight. No bundled theme is light, but one of yours can be.
			continue
		}
		t.Run(name, func(t *testing.T) {
			require.NotEqual(t, th.Text, th.Selected,
				"the cursor row's name must differ from an ordinary one, or only the background marks it")
		})
	}
}

// The panel is the cursor row's bed. If it is barely a shade off the terminal's
// own background, the cursor row has no bed at all and the whole theme reads as
// flat. There is no background token to compare against — cdu does not paint the
// field — so this checks the panel against the darkest and lightest a terminal
// is likely to be.
func TestPanelSeparatesFromAPlausibleTerminalBackground(t *testing.T) {
	const minPanelContrast = 1.2

	for _, name := range Names() {
		th, ok := Preset(name)
		require.True(t, ok)
		if th.Plain {
			continue
		}
		t.Run(name, func(t *testing.T) {
			bg := Color("#000000")
			if th.Light {
				bg = "#ffffff"
			}
			got := contrast(t, th.Panel, bg)
			if got < minPanelContrast {
				t.Errorf("panel %s against a %s terminal is %.2f:1, want %.1f:1 — "+
					"the cursor row would have no visible bed", th.Panel, bg, got, minPanelContrast)
			}
		})
	}
}

// colourName reports which token a colour is, for a readable failure.
func colourName(t *Theme, c Color) string {
	for key, tok := range t.tokens() {
		if *tok == c {
			return key
		}
	}
	return "?"
}
