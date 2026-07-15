package charm

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// Fuzzy matching here is a subsequence test: the query's runes must appear in the
// name, in order, but not necessarily together. So "nmd" finds "node_modules" and
// "dwn" finds "Downloads".
//
// It deliberately does not rank by match quality. A disk usage tool sorts by size
// for a reason — the biggest offender is the point — so the filter decides only
// what is shown, and the existing sort decides the order. A well-spelled small
// file must never float above the large one it matches less tidily.
//
// The candidate set is one directory's children, never the whole tree, so there
// is nothing here worth optimising beyond the obvious.

// fuzzyMatch reports whether query is a subsequence of name, and if so where each
// query rune landed. The indices are rune offsets into name, which is what the
// highlighter needs — byte offsets would split multi-byte runes.
//
// An empty query matches everything with no highlights, so an open-but-empty
// filter shows the directory unchanged rather than hiding all of it.
func fuzzyMatch(name, query string) (matched bool, positions []int) {
	if query == "" {
		return true, nil
	}

	q := []rune(query)
	qi := 0
	positions = make([]int, 0, len(q))

	for i, r := range []rune(name) {
		if foldEqual(r, q[qi]) {
			positions = append(positions, i)
			qi++
			if qi == len(q) {
				return true, positions
			}
		}
	}
	return false, nil
}

// foldEqual compares two runes case-insensitively without allocating. Typing a
// filter should not require getting the case right.
func foldEqual(a, b rune) bool {
	return a == b || unicode.ToLower(a) == unicode.ToLower(b)
}

// highlightMatch styles the query's matched runes within an already-laid-out name
// cell. It matches against the cell itself, not the original name, so the
// positions line up with exactly what is on screen even after truncation — a name
// cut by "…" simply highlights whatever of the query still shows.
//
// The cell keeps its width: styling changes no columns. Runs of same-styled runes
// are rendered together rather than one escape pair per rune.
func highlightMatch(cell, query string, base, hl *lipgloss.Style) string {
	ok, positions := fuzzyMatch(cell, query)
	if !ok || len(positions) == 0 {
		return base.Render(cell)
	}

	hits := make(map[int]bool, len(positions))
	for _, p := range positions {
		hits[p] = true
	}

	var out strings.Builder
	runes := []rune(cell)
	i := 0
	for i < len(runes) {
		on := hits[i]
		j := i
		for j < len(runes) && hits[j] == on {
			j++
		}
		segment := string(runes[i:j])
		if on {
			out.WriteString(hl.Render(segment))
		} else {
			out.WriteString(base.Render(segment))
		}
		i = j
	}
	return out.String()
}
