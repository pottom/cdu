package charm

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/internal/common"
)

func padLeft(s string, width int) string {
	if runewidth.StringWidth(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-runewidth.StringWidth(s)) + s
}

// padLines fits a block to exactly n lines and no trailing newline: it pads a
// short one, so the footer never floats up into the middle of a short list, and
// clips a long one, so a list whose height is not a whole number of entries
// cannot push the footer off the bottom of the screen. Getting this wrong by one
// line makes the whole frame scroll on every render.
func padLines(s string, n int) string {
	if n < 1 {
		return ""
	}

	lines := strings.Split(s, "\n")
	if len(lines) > n {
		return strings.Join(lines[:n], "\n")
	}
	return s + strings.Repeat("\n", n-len(lines))
}

// joinLines is strings.Join with a newline, named for what it is doing so a
// caller building a screen reads as building a screen.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// clipTo fits a plain string to exactly width columns: truncating a long one and
// padding a short one.
//
// "Exactly" is the point. Truncate alone can come back a column short, because it
// will not cut a wide rune in half, and a row one column short is as wrong as one
// column long once something is drawn to its right.
//
// Plain text only. runewidth counts an escape sequence's bytes as visible
// columns, so clipping a styled string throws away most of its content and can
// leave a background escape unterminated. Clip first, style after.
func clipTo(s string, width int) string {
	if width < 1 {
		return ""
	}
	return runewidth.FillRight(runewidth.Truncate(s, width, ""), width)
}

// middleTruncate keeps both ends of a path readable, which matters more than
// the middle when you are looking at a breadcrumb.
func middleTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= width {
		return s
	}
	if width <= 1 {
		return "…"
	}

	half := (width - 1) / 2
	left := runewidth.Truncate(s, half, "")

	r := []rune(s)
	right := ""
	for i := len(r) - 1; i >= 0; i-- {
		candidate := string(r[i:])
		if runewidth.StringWidth(candidate) > width-1-runewidth.StringWidth(left) {
			break
		}
		right = candidate
	}
	return left + "…" + right
}

func formatPct(size, total int64) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", float64(size)/float64(total)*100)
}

func humanCount(n int64) string {
	return common.FormatNumber(n)
}
