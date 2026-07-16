package theme

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// swatchCells is how much of the usage bar's gradient the listing shows. Enough
// to see where it starts and ends, short enough to keep the table aligned on an
// 80-column terminal.
const swatchCells = 10

// List writes the bundled themes with a preview of each. The preview is the
// point: a name is not a colour, and the whole reason to run this is to see
// them.
//
// It degrades with the terminal exactly as the interface does — on a dumb
// terminal or under --no-color, Lipgloss emits nothing and the listing is a
// plain table, which is still useful, because it still says what to type.
func List(w io.Writer, current string) error {
	var b strings.Builder

	var anyLight bool

	b.WriteString("Bundled themes:\n\n")
	for _, name := range Names() {
		th, ok := Preset(name)
		if !ok {
			continue
		}
		anyLight = anyLight || th.Light
		marker := "  "
		if name == current {
			marker = "* "
		}
		fmt.Fprintf(&b, "%s%-10s %-6s %s\n", marker, name, kind(&th), swatch(&th))
	}

	b.WriteString("\nA * marks the theme in use. Pick one with --theme NAME, or in your config:\n\n")
	b.WriteString("  theme:\n    preset: midnight\n    accent: \"#ff5fd1\"\n")
	b.WriteString("\nOverridable tokens:\n")
	fmt.Fprintf(&b, "  %s\n", strings.Join(TokenNames(), ", "))
	b.WriteString("Colours are #rrggbb only, because the usage bar blends them.\n")

	// Only say this when something above it is light. No bundled theme is, since
	// daylight was dropped, but a theme of your own can be.
	if anyLight {
		b.WriteString("\nA light theme needs a light terminal: cdu never paints the background, so\n" +
			"your terminal's own shows through — which is what keeps transparency working.\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func kind(t *Theme) string {
	switch {
	case t.Plain:
		return "any"
	case t.Light:
		return "light"
	default:
		return "dark"
	}
}

// swatch previews a theme as the usage bar's gradient followed by the three
// colours that carry the most meaning in the interface.
func swatch(t *Theme) string {
	if t.Plain {
		return "(no colour)"
	}

	from, errFrom := colorful.Hex(string(t.BarFrom))
	to, errTo := colorful.Hex(string(t.BarTo))
	if errFrom != nil || errTo != nil {
		return ""
	}

	var b strings.Builder
	for i := range swatchCells {
		p := 0.0
		if swatchCells > 1 {
			p = float64(i) / float64(swatchCells-1)
		}
		c := from.BlendLuv(to, p).Clamped()
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex())).Render("█"))
	}
	b.WriteString(" ")
	for _, c := range []Color{t.Accent, t.Size, t.Danger} {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(string(c))).Render("██"))
	}
	return b.String()
}
