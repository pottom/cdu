package charm

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"

	"github.com/pottom/cdu/internal/theme"
)

// The design draws the usage bar as a CSS linear-gradient. A terminal cannot
// fill a cell with a gradient, so the closest honest translation is to colour
// every cell of the bar separately, interpolating between the two endpoints
// across the whole track width. A cell's colour is fixed by its position in the
// bar, so the gradient's dark end belongs to a full bar and a short bar is just
// the light beginning of it — the tip colour then reads as the row's size, and
// no bar has the whole gradient crushed into a handful of cells.
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

// barPaint is one bar's worth of pre-rendered cells, for one background.
//
// There are two of them because the bar has to be drawn on two backgrounds: the
// terminal's own, and the cursor row's panel. Painting the cursor row's bar in
// the row's own foreground — which is what happens if it is simply included in
// the text — turns a gradient into a solid white smear; leaving it on the
// terminal background instead punches a strip of the terminal through the middle
// of what is supposed to be one block.
//
// Both are built once, at model construction. Building a Lipgloss style per cell
// per row per frame cost 4.1 ms against 0.28 ms for a ramp, and the whole reason
// this type exists is not to do that.
type barPaint struct {
	// ramp holds the filled cell pre-rendered at each step of the gradient.
	ramp  []string
	solid lipgloss.Style
	track lipgloss.Style
}

// barRenderer draws one usage bar. It is built once, at model construction, so
// the colour profile is probed once rather than once per frame.
type barRenderer struct {
	mode  barMode
	chars barChars

	// Gradient endpoints, kept as colorful.Color so they can be blended.
	from, to colorful.Color

	// field is the bar on the terminal's own background; panel is the same bar on
	// the cursor row.
	field barPaint
	panel barPaint
}

func newBarRenderer(t *theme.Theme, useColors, noUnicode bool) barRenderer {
	b := barRenderer{
		chars: unicodeBarChars,
		mode:  barPlain,
	}
	if noUnicode {
		b.chars = asciiBarChars
	}
	if !useColors || t.Plain {
		return b
	}

	b.from = mustColor(string(t.BarFrom))
	b.to = mustColor(string(t.BarTo))

	b.mode = barSolid
	if lipgloss.ColorProfile() == termenv.TrueColor {
		b.mode = barGradient
	}

	b.field = b.paintFor(t, nil)
	b.panel = b.paintFor(t, lg(t.Panel))
	return b
}

// paintFor builds the cells for one background. bg nil means the terminal's own.
func (b *barRenderer) paintFor(t *theme.Theme, bg lipgloss.TerminalColor) barPaint {
	on := func(s lipgloss.Style) lipgloss.Style {
		if bg == nil {
			return s
		}
		return s.Background(bg)
	}

	// Below truecolor the bar is a solid fill of the gradient's own starting
	// colour, so the two paths are recognisably the same element.
	p := barPaint{
		solid: on(lipgloss.NewStyle().Foreground(lg(t.BarFrom))),
		track: on(lipgloss.NewStyle().Foreground(lg(t.BarTrack))),
	}
	if b.mode != barGradient {
		return p
	}
	p.ramp = make([]string, gradientSteps)
	for i := range p.ramp {
		p.ramp[i] = on(b.cellStyle(i, gradientSteps)).Render(b.chars.full)
	}
	return p
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

// render returns a bar of exactly width cells, of which frac (0..1) are filled,
// drawn on the terminal's own background. It is safe for any width and any frac,
// including nonsense ones.
func (b *barRenderer) render(frac float64, width int) string {
	return b.renderOn(&b.field, frac, width)
}

// renderSelected is the same bar on the cursor row, carrying that row's
// background so the row stays one block.
func (b *barRenderer) renderSelected(frac float64, width int) string {
	return b.renderOn(&b.panel, frac, width)
}

func (b *barRenderer) renderOn(p *barPaint, frac float64, width int) string {
	if width < 1 {
		return ""
	}
	filled := b.filledCells(frac, width)
	empty := width - filled

	switch b.mode {
	case barPlain:
		return strings.Repeat(b.chars.full, filled) + strings.Repeat(b.chars.empty, empty)

	case barSolid:
		return b.paint(&p.solid, b.chars.full, filled) + b.paint(&p.track, b.chars.empty, empty)

	case barGradient:
		var sb strings.Builder
		for i := range filled {
			sb.WriteString(p.ramp[rampIndex(i, width)])
		}
		sb.WriteString(b.paint(&p.track, b.chars.empty, empty))
		return sb.String()
	}
	return ""
}

// plainCells is the bar as bare characters, for a row that will be styled whole
// afterwards — the cursor row, which carries one background across everything on
// it and so cannot hold a bar coloured cell by cell.
func (b *barRenderer) plainCells(frac float64, width int) string {
	if width < 1 {
		return ""
	}
	filled := b.filledCells(frac, width)
	return strings.Repeat(b.chars.full, filled) + strings.Repeat(b.chars.empty, width-filled)
}

// rampIndex maps cell i of a bar span onto the precomputed ramp. The span is the
// whole track width, not the filled run — so a cell's colour is fixed by where it
// sits in the bar, and the gradient's dark end is reached only by a full bar. A
// short bar is the light beginning of the gradient rather than the whole ramp
// compressed into a few cells, and its tip colour therefore reads as its size.
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

// cellStyle is the colour of cell i of a span of n cells. It builds the ramp the
// renderer indexes into with rampIndex; the span there is the whole track width,
// so the endpoints are the colours of an empty and a completely full bar.
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
