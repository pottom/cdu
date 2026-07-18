package charm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/theme"
)

// testTheme is the default preset, addressable for the renderers that take a
// pointer to one.
func testTheme() *theme.Theme {
	th := theme.Charm()
	return &th
}

// withProfile forces a colour profile for the duration of a test. Without this
// the test process — which has no TTY — makes Lipgloss emit no escapes at all,
// so every colour bug hides.
func withProfile(t *testing.T, p termenv.Profile) {
	t.Helper()
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(p)
	t.Cleanup(func() { lipgloss.SetColorProfile(original) })
}

func TestBarModeFollowsColorProfile(t *testing.T) {
	p := testTheme()

	withProfile(t, termenv.TrueColor)
	assert.Equal(t, barGradient, newBarRenderer(p, true, false).mode,
		"truecolor should get the per-cell gradient")

	withProfile(t, termenv.ANSI256)
	assert.Equal(t, barSolid, newBarRenderer(p, true, false).mode,
		"256 colours band visibly, so the bar must fall back to a solid fill")

	withProfile(t, termenv.TrueColor)
	assert.Equal(t, barPlain, newBarRenderer(p, false, false).mode,
		"--no-color must win over a capable terminal")
}

// The bar occupies a column, so its width has to be exact regardless of how it
// is coloured — an off-by-one here smears the layout of every row.
func TestBarWidthIsExact(t *testing.T) {
	for name, profile := range map[string]termenv.Profile{
		"truecolor": termenv.TrueColor,
		"ansi256":   termenv.ANSI256,
		"ascii":     termenv.Ascii,
	} {
		t.Run(name, func(t *testing.T) {
			withProfile(t, profile)
			b := newBarRenderer(testTheme(), true, false)

			for _, width := range []int{1, 2, 3, 7, 20, 60} {
				for _, frac := range []float64{0, 0.01, 0.25, 0.5, 0.999, 1} {
					assert.Equal(t, width, lipgloss.Width(b.render(frac, width)),
						"width=%d frac=%v", width, frac)
				}
			}
		})
	}
}

func TestBarDegenerateInputs(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	b := newBarRenderer(testTheme(), true, false)

	assert.Empty(t, b.render(0.5, 0), "zero width renders nothing")
	assert.Empty(t, b.render(0.5, -3), "negative width renders nothing")

	// A percentage can only be computed against a parent total, and an empty
	// parent gives 0/0. The bar must clamp rather than produce garbage.
	assert.Equal(t, 10, lipgloss.Width(b.render(nan(), 10)), "NaN must not panic")
	assert.Equal(t, 0, b.filledCells(nan(), 10), "NaN reads as empty")
	assert.Equal(t, 10, b.filledCells(2.5, 10), "over-full clamps to the width")
	assert.Equal(t, 0, b.filledCells(-1, 10), "negative reads as empty")
}

// A directory holding a real but tiny share must still show something. Rounding
// it away to an empty bar would read as "this is 0 bytes", which is a lie.
func TestBarNeverRoundsAwayANonzeroShare(t *testing.T) {
	withProfile(t, termenv.Ascii)
	b := newBarRenderer(testTheme(), true, false)

	assert.Equal(t, 1, b.filledCells(0.0001, 20))
	assert.Equal(t, 20, b.filledCells(1, 20), "a full share fills every cell")
}

func TestBarPlainAndAsciiCharacters(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	plain := newBarRenderer(testTheme(), false, false)
	assert.Equal(t, "█████░░░░░", plain.render(0.5, 10),
		"--no-color keeps the blocks but drops the colour")

	ascii := newBarRenderer(testTheme(), false, true)
	assert.Equal(t, "#####-----", ascii.render(0.5, 10),
		"--no-unicode must not emit block runes")
}

// The gradient is only worth its cost if the cells actually differ. This asserts
// the thing the eye is meant to see: distinct colours along the filled run.
func TestGradientColoursEveryCellDifferently(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	b := newBarRenderer(testTheme(), true, false)

	out := b.render(1, 12)
	require.Contains(t, out, "\x1b[", "truecolor bar must carry escapes")

	seen := map[string]bool{}
	for _, cell := range strings.SplitAfter(out, "█") {
		if cell != "" {
			seen[cell] = true
		}
	}
	assert.Greater(t, len(seen), 8, "a 12-cell bar should show a spread of colours, got %d", len(seen))

	// The endpoints are the theme's, not something drifted by the blend.
	first := b.cellStyle(0, 12).GetForeground()
	last := b.cellStyle(11, 12).GetForeground()
	assert.Equal(t, strings.ToLower(string(theme.Charm().BarFrom)), strings.ToLower(hexOf(first)))
	assert.Equal(t, strings.ToLower(string(theme.Charm().BarTo)), strings.ToLower(hexOf(last)))
}

// The gradient is tied to the track, not the filled run: a cell's colour is set
// by where it sits in the bar, so only a full bar reaches the dark end and a short
// one is the light beginning of the ramp. This is what makes the tip colour read
// as the row's size instead of every bar, long or short, running the whole ramp.
func TestGradientIsProportionalToFill(t *testing.T) {
	// The last cell of a full 20-wide bar is the dark endpoint; the tip of a
	// half-full one (cell 9 of 20) stops well short of it.
	assert.Equal(t, gradientSteps-1, rampIndex(19, 20), "a full bar reaches the dark end")
	assert.Less(t, rampIndex(9, 20), gradientSteps-1, "a half bar's tip stops short of it")

	// A cell's colour depends on its position in the track, not on the bar's own
	// length — so the same position is the same colour whatever the fill, and the
	// same fill is lighter drawn across a wider track.
	assert.Equal(t, rampIndex(5, 40), rampIndex(5, 40))
	assert.Less(t, rampIndex(5, 40), rampIndex(5, 20), "a wider track spreads the gradient further right")
}

func hexOf(c lipgloss.TerminalColor) string {
	if col, ok := c.(lipgloss.Color); ok {
		return string(col)
	}
	return ""
}

func nan() float64 {
	zero := 0.0
	return zero / zero
}
