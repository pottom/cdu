package charm

import (
	"fmt"
	"math"

	"github.com/charmbracelet/lipgloss"

	"github.com/pottom/cdu/internal/common"
)

// palette holds every colour the renderer is allowed to use. It exists as a
// struct rather than a set of constants so the theme system can replace it
// wholesale later — nothing below reaches for a colour literal directly.
//
// These are the `charm` theme's tokens from the design spec.
type palette struct {
	panel  lipgloss.Color
	pink   lipgloss.Color
	text   lipgloss.Color
	dim    lipgloss.Color
	mint   lipgloss.Color
	danger lipgloss.Color
}

func charmPalette() *palette {
	return &palette{
		panel:  lipgloss.Color("#241c34"),
		pink:   lipgloss.Color("#ff5fd1"),
		text:   lipgloss.Color("#cfc6ef"),
		dim:    lipgloss.Color("#7d739e"),
		mint:   lipgloss.Color("#4ff0c0"),
		danger: lipgloss.Color("#ff2fb3"),
	}
}

// styles are the resolved Lipgloss styles for one palette. When colour is off
// (--no-color, NO_COLOR, or a dumb terminal) every style degrades to plain text,
// which is why state is never conveyed by colour alone.
type styles struct {
	dirName  lipgloss.Style
	fileName lipgloss.Style
	selected lipgloss.Style
	size     lipgloss.Style
	pct      lipgloss.Style
	dim      lipgloss.Style
	accent   lipgloss.Style
	danger   lipgloss.Style
}

func newStyles(p *palette, useColors bool) styles {
	if !useColors {
		plain := lipgloss.NewStyle()
		return styles{
			dirName:  plain.Bold(true),
			fileName: plain,
			selected: plain.Reverse(true),
			size:     plain,
			pct:      plain,
			dim:      plain,
			accent:   plain.Bold(true),
			danger:   plain.Bold(true),
		}
	}
	return styles{
		dirName:  lipgloss.NewStyle().Foreground(lipgloss.Color("#e9e3ff")),
		fileName: lipgloss.NewStyle().Foreground(p.text),
		selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(p.panel).
			Bold(true),
		size:   lipgloss.NewStyle().Foreground(p.mint),
		pct:    lipgloss.NewStyle().Foreground(p.dim),
		dim:    lipgloss.NewStyle().Foreground(p.dim),
		accent: lipgloss.NewStyle().Foreground(p.pink).Bold(true),
		danger: lipgloss.NewStyle().Foreground(p.danger).Bold(true),
	}
}

type sizeUnit struct {
	limit float64
	unit  string
}

var (
	binaryUnits = []sizeUnit{
		{common.Ei, "EiB"}, {common.Pi, "PiB"}, {common.Ti, "TiB"},
		{common.Gi, "GiB"}, {common.Mi, "MiB"}, {common.Ki, "KiB"},
	}
	siUnits = []sizeUnit{
		{common.E, "EB"}, {common.P, "PB"}, {common.T, "TB"},
		{common.G, "GB"}, {common.M, "MB"}, {common.K, "kB"},
	}
)

// formatSize renders a byte count, honouring --si.
func (ui *UI) formatSize(size int64) string {
	units := binaryUnits
	if ui.UseSIPrefix {
		units = siUnits
	}

	abs := math.Abs(float64(size))
	for _, u := range units {
		if abs >= u.limit {
			return fmt.Sprintf("%.1f %s", float64(size)/u.limit, u.unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}
