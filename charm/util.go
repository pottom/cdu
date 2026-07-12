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

// padLines pads a block out to exactly n lines, so the footer never floats up
// into the middle of a short list.
func padLines(s string, n int) string {
	lines := strings.Count(s, "\n") + 1
	if s == "" {
		lines = 0
	}
	if lines >= n {
		return s + "\n"
	}
	return s + strings.Repeat("\n", n-lines+1)
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
