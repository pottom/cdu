package charm

import (
	"fmt"
	"math"

	"github.com/charmbracelet/lipgloss"

	"github.com/pottom/cdu/internal/common"
	"github.com/pottom/cdu/internal/theme"
)

// lg turns a theme token into a Lipgloss colour. Every colour in the render path
// comes through here from the active theme — no style below holds a literal, so
// a theme can replace all of them.
func lg(c theme.Color) lipgloss.Color { return lipgloss.Color(string(c)) }

// styles are the resolved Lipgloss styles for one theme. When colour is off
// (--no-color, NO_COLOR, a dumb terminal, or the mono theme) every style
// degrades to plain text, which is why state is never conveyed by colour alone.
type styles struct {
	dirName  lipgloss.Style
	fileName lipgloss.Style
	selected lipgloss.Style
	// selectedMatch highlights filter matches on the cursor row. It carries the
	// selection's background so the row stays one continuous block; only the
	// foreground changes. Under no colour it underlines instead, so the match still
	// shows against the reversed selection.
	selectedMatch lipgloss.Style
	size          lipgloss.Style
	pct           lipgloss.Style
	dim           lipgloss.Style
	accent        lipgloss.Style
	danger        lipgloss.Style

	modal        lipgloss.Style
	button       lipgloss.Style
	buttonFocus  lipgloss.Style
	buttonDanger lipgloss.Style
}

func newStyles(t *theme.Theme, useColors bool) styles {
	if !useColors || t.Plain {
		plain := lipgloss.NewStyle()
		return styles{
			dirName:       plain.Bold(true),
			fileName:      plain,
			selected:      plain.Reverse(true),
			selectedMatch: plain.Reverse(true).Underline(true),
			size:          plain,
			pct:           plain,
			dim:           plain,
			accent:        plain.Bold(true),
			danger:        plain.Bold(true),

			modal: plain.Border(lipgloss.RoundedBorder()).Padding(0, modalPadding),
			// Without colour the focused button is told apart by its brackets and by
			// being reversed, never by hue alone.
			button:       plain,
			buttonFocus:  plain.Reverse(true).Bold(true),
			buttonDanger: plain.Reverse(true).Bold(true),
		}
	}
	return styles{
		dirName:  lipgloss.NewStyle().Foreground(lg(t.Dir)),
		fileName: lipgloss.NewStyle().Foreground(lg(t.Text)),
		selected: lipgloss.NewStyle().
			Foreground(lg(t.Selected)).
			Background(lg(t.Panel)).
			Bold(true),
		selectedMatch: lipgloss.NewStyle().
			Foreground(lg(t.Accent)).
			Background(lg(t.Panel)).
			Bold(true),
		size:   lipgloss.NewStyle().Foreground(lg(t.Size)),
		pct:    lipgloss.NewStyle().Foreground(lg(t.Dim)),
		dim:    lipgloss.NewStyle().Foreground(lg(t.Dim)),
		accent: lipgloss.NewStyle().Foreground(lg(t.Accent)).Bold(true),
		danger: lipgloss.NewStyle().Foreground(lg(t.Danger)).Bold(true),

		modal: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lg(t.Accent)).
			Background(lg(t.Panel)).
			Foreground(lg(t.Text)).
			Padding(0, modalPadding),
		button: lipgloss.NewStyle().
			Foreground(lg(t.Dim)).
			Background(lg(t.Panel)),
		buttonFocus: lipgloss.NewStyle().
			Foreground(lg(t.Ink)).
			Background(lg(t.Dim)).
			Bold(true),
		buttonDanger: lipgloss.NewStyle().
			Foreground(lg(t.Ink)).
			Background(lg(t.Danger)).
			Bold(true),
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
