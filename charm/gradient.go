package charm

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
)

// The design draws the usage bar as a CSS linear-gradient. A terminal cannot
// fill a cell with a gradient, so the closest honest translation is to colour
// every cell of the bar separately, interpolating between the two endpoints
// across the filled part.
//
// That only reads as a gradient with 24-bit colour. On a 256-colour terminal the
// interpolation quantises into visible bands, which looks like a rendering fault
// rather than a design, so below truecolor the bar degrades to a solid accent
// fill — and without colour at all, to plain characters. The bar therefore never
// carries information that the row does not also carry as text: it is decoration
// for the percentage column, not a substitute for it.

type barMode int

const (
	// barPlain uses characters alone. --no-color, NO_COLOR, dumb terminals. It is
	// first so that it is the zero value: a barRenderer nobody remembered to build
	// then degrades to plain text rather than painting a black gradient.
	barPlain barMode = iota
	// barSolid fills with one accent colour. 256- and 16-colour terminals.
	barSolid
	// barGradient interpolates per cell. Truecolor only.
	barGradient
)

// barChars are the runes the bar is drawn with. Unicode blocks by default, ASCII
// under --no-unicode.
type barChars struct {
	full  string
	empty string
}

var (
	unicodeBarChars = barChars{full: "█", empty: "░"}
	asciiBarChars   = barChars{full: "#", empty: "-"}
)

// gradientSteps is how finely the ramp is precomputed. A bar is at most a couple
// of hundred cells wide, and the eye cannot separate neighbouring steps at this
// resolution, so quantising here is free visually and saves building a Lipgloss
// style — 552 bytes of it — for every cell of every row on every frame.
const gradientSteps = 64

// barRenderer draws one usage bar. It is built once, at model construction, so
// the colour profile is probed once rather than once per frame.
type barRenderer struct {
	mode  barMode
	chars barChars

	// Gradient endpoints, kept as colorful.Color so they can be blended.
	from, to colorful.Color

	// ramp holds the filled cell pre-rendered at each step of the gradient.
	ramp []string

	solid lipgloss.Style
	track lipgloss.Style
}

func newBarRenderer(p *palette, useColors, noUnicode bool) barRenderer {
	b := barRenderer{
		chars: unicodeBarChars,
		mode:  barPlain,
	}
	if noUnicode {
		b.chars = asciiBarChars
	}
	if !useColors {
		return b
	}

	b.from = mustColor(string(p.pink))
	b.to = mustColor(string(p.purple))
	b.solid = lipgloss.NewStyle().Foreground(p.pink)
	b.track = lipgloss.NewStyle().Foreground(p.barTrack)

	b.mode = barSolid
	if lipgloss.ColorProfile() == termenv.TrueColor {
		b.mode = barGradient
		b.ramp = make([]string, gradientSteps)
		for i := range b.ramp {
			style := b.cellStyle(i, gradientSteps)
			b.ramp[i] = style.Render(b.chars.full)
		}
	}
	return b
}

// mustColor parses a token from the palette. The palette is ours, not the user's
// — a bad hex here is a bug in cdu, not bad input — so it falls back silently to
// black rather than growing an error path the caller cannot act on. Once themes
// are user-supplied, parsing moves to load time, where a warning belongs.
func mustColor(hex string) colorful.Color {
	c, err := colorful.Hex(hex)
	if err != nil {
		return colorful.Color{}
	}
	return c
}

// render returns a bar of exactly width cells, of which frac (0..1) are filled.
// It is safe for any width and any frac, including nonsense ones.
func (b *barRenderer) render(frac float64, width int) string {
	if width < 1 {
		return ""
	}
	filled := b.filledCells(frac, width)
	empty := width - filled

	switch b.mode {
	case barPlain:
		return strings.Repeat(b.chars.full, filled) + strings.Repeat(b.chars.empty, empty)

	case barSolid:
		return b.paint(&b.solid, b.chars.full, filled) + b.paint(&b.track, b.chars.empty, empty)

	case barGradient:
		var sb strings.Builder
		for i := range filled {
			sb.WriteString(b.ramp[rampIndex(i, filled)])
		}
		sb.WriteString(b.paint(&b.track, b.chars.empty, empty))
		return sb.String()
	}
	return ""
}

// rampIndex maps cell i of a filled run of n onto the precomputed ramp. A run of
// one sits at the start of the gradient rather than in the middle of it, so a
// sliver of a bar is pink — the same colour a long bar starts with.
func rampIndex(i, n int) int {
	if n <= 1 {
		return 0
	}
	idx := int(math.Round(float64(i) / float64(n-1) * float64(gradientSteps-1)))
	return min(max(idx, 0), gradientSteps-1)
}

// paint renders n copies of a cell, and nothing at all for n == 0 — a styled
// empty string is still a pair of escape sequences on the wire.
func (b *barRenderer) paint(style *lipgloss.Style, cell string, n int) string {
	if n < 1 {
		return ""
	}
	return style.Render(strings.Repeat(cell, n))
}

// filledCells rounds the fraction to whole cells. A directory with a nonzero
// share never rounds down to an empty bar, and a full one never rounds up past
// the width.
func (b *barRenderer) filledCells(frac float64, width int) int {
	if math.IsNaN(frac) || frac <= 0 {
		return 0
	}
	if frac >= 1 {
		return width
	}
	n := int(math.Round(frac * float64(width)))
	return min(max(n, 1), width)
}

// cellStyle is the colour of cell i of a filled run of n cells. The gradient
// spans the filled part, exactly as the mock's `.barfill` carries the gradient
// rather than the track behind it — so a short bar is pink-to-purple in
// miniature, not a pink stub.
func (b *barRenderer) cellStyle(i, n int) lipgloss.Style {
	t := 0.0
	if n > 1 {
		t = float64(i) / float64(n-1)
	}
	// Luv blending keeps the midpoint from going muddy, which a naive RGB lerp
	// between pink and purple does.
	c := b.from.BlendLuv(b.to, t).Clamped()
	return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
}
