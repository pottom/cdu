package charm

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	p := charmPalette()

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
			b := newBarRenderer(charmPalette(), true, false)

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
	b := newBarRenderer(charmPalette(), true, false)

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
	b := newBarRenderer(charmPalette(), true, false)

	assert.Equal(t, 1, b.filledCells(0.0001, 20))
	assert.Equal(t, 20, b.filledCells(1, 20), "a full share fills every cell")
}

func TestBarPlainAndAsciiCharacters(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	plain := newBarRenderer(charmPalette(), false, false)
	assert.Equal(t, "█████░░░░░", plain.render(0.5, 10),
		"--no-color keeps the blocks but drops the colour")

	ascii := newBarRenderer(charmPalette(), false, true)
	assert.Equal(t, "#####-----", ascii.render(0.5, 10),
		"--no-unicode must not emit block runes")
}

// The gradient is only worth its cost if the cells actually differ. This asserts
// the thing the eye is meant to see: distinct colours along the filled run.
func TestGradientColoursEveryCellDifferently(t *testing.T) {
	withProfile(t, termenv.TrueColor)
	b := newBarRenderer(charmPalette(), true, false)

	out := b.render(1, 12)
	require.Contains(t, out, "\x1b[", "truecolor bar must carry escapes")

	seen := map[string]bool{}
	for _, cell := range strings.SplitAfter(out, "█") {
		if cell != "" {
			seen[cell] = true
		}
	}
	assert.Greater(t, len(seen), 8, "a 12-cell bar should show a spread of colours, got %d", len(seen))

	// The endpoints are the palette's, not something drifted by the blend.
	first := b.cellStyle(0, 12).GetForeground()
	last := b.cellStyle(11, 12).GetForeground()
	assert.Equal(t, strings.ToLower(string(charmPalette().pink)), strings.ToLower(hexOf(first)))
	assert.Equal(t, strings.ToLower(string(charmPalette().purple)), strings.ToLower(hexOf(last)))
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
