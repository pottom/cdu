package charm

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

func iconModel(t *testing.T, icons, noUnicode bool, names ...string) *model {
	t.Helper()
	ui := CreateUI(nil, true, false, false, false)
	ui.icons, ui.noUnicode = icons, noUnicode

	m := newModel(ui)
	m.width, m.height, m.haveSize = 90, 20, true

	dir := &analyze.Dir{File: &analyze.File{Name: "root"}, BasePath: "/"}
	for _, n := range names {
		dir.AddFile(&analyze.File{Name: n, Size: 4096, Usage: 4096, Parent: dir})
	}
	m.topDir = dir
	m.enterDir(dir)
	return m
}

// The whole hazard of this feature: a Nerd Font glyph is one cell to runewidth
// and two to a non-Mono font, so the icon cell is glyph-plus-space and the
// double-width draw spills into the space rather than into the size column.
// If the cell is ever not exactly iconWidth, every column after it shifts.
func TestTheIconCellIsAlwaysExactlyTwoColumns(t *testing.T) {
	m := iconModel(t, true, false,
		"photo.jpg", "clip.mp4", "notes.md", "main.go", "Dockerfile", ".gitignore",
		"archive.tar.gz", "no-extension", "weird.unknown-ext", "UPPER.PNG")
	sub := &analyze.Dir{File: &analyze.File{Name: "src", Parent: m.currentDir.(*analyze.Dir)}}
	m.currentDir.(*analyze.Dir).AddFile(sub)
	m.reloadRows()

	for _, r := range m.rows {
		cell := m.rowIcon(r)
		assert.Equal(t, iconWidth, runewidth.StringWidth(cell),
			"the icon cell for %q must be exactly %d columns, got %q", r.GetName(), iconWidth, cell)
	}
}

// Rows must still measure exactly the terminal width with icons on. This is the
// same guarantee width_test.go makes for the default markers, and it is the one
// that would break first.
func TestRowsKeepTheirWidthWithIcons(t *testing.T) {
	withProfile(t, termenv.TrueColor)

	for _, icons := range []bool{true, false} {
		for _, width := range []int{44, 60, 90, 200} {
			m := iconModel(t, icons, false, "photo.jpg", "main.go", "archive.zip")
			m.width, m.height = width, 20
			total := m.itemSize(m.currentDir)

			for i, r := range m.rows {
				got := lipgloss.Width(m.viewRow(r, i == 0, total))
				assert.Equal(t, width, got,
					"icons=%v width=%d: row %q measured %d", icons, width, r.GetName(), got)
			}
		}
	}
}

// Below minWidthForIcon there is no icon cell at all, with or without --icons.
func TestNoIconCellOnANarrowTerminal(t *testing.T) {
	m := iconModel(t, true, false, "photo.jpg")
	m.width = minWidthForIcon - 1
	assert.Empty(t, m.rowIcon(m.rows[0]))

	m.width = minWidthForIcon
	assert.Equal(t, iconWidth, runewidth.StringWidth(m.rowIcon(m.rows[0])))
}

// The lookup order is exa's, and each step of it earns its place.
func TestIconLookupOrder(t *testing.T) {
	dir := &analyze.Dir{File: &analyze.File{Name: "root"}, BasePath: "/"}
	file := func(name string) fs.Item {
		return &analyze.File{Name: name, Parent: dir}
	}
	subdir := func(name string) fs.Item {
		return &analyze.Dir{File: &analyze.File{Name: name, Parent: dir}}
	}

	// A whole filename beats an extension: these have no useful extension.
	assert.Equal(t, iconByName["Dockerfile"], iconFor(file("Dockerfile")))
	assert.Equal(t, iconByName[".gitignore"], iconFor(file(".gitignore")))

	// A directory beats an extension — node_modules.bak is a directory, not a
	// backup file.
	assert.Equal(t, iconFolder, iconFor(subdir("node_modules.bak")))
	assert.Equal(t, iconGit, iconFor(subdir(".git")))
	assert.Equal(t, iconBin, iconFor(subdir("bin")))

	// Then the extension, case-insensitively: the table is lowercase and the
	// filesystem is not.
	assert.Equal(t, iconByExt["jpg"], iconFor(file("photo.jpg")))
	assert.Equal(t, iconByExt["jpg"], iconFor(file("PHOTO.JPG")))
	assert.Equal(t, iconByExt["gz"], iconFor(file("archive.tar.gz")), "the last extension wins")

	// And a fallback, so nothing is ever blank.
	assert.Equal(t, iconFile, iconFor(file("no-extension")))
	assert.Equal(t, iconFile, iconFor(file("mystery.qqq")))
	assert.Equal(t, iconFile, iconFor(file("")))
}

// --icons is opt-in, and off means the markers that any terminal can draw.
func TestIconsAreOffUnlessAskedFor(t *testing.T) {
	off := iconModel(t, false, false, "photo.jpg")
	assert.Equal(t, markerFile+" ", off.rowIcon(off.rows[0]),
		"without --icons no Nerd Font glyph may appear — it would be a box on most terminals")

	on := iconModel(t, true, false, "photo.jpg")
	assert.Equal(t, iconByExt["jpg"]+" ", on.rowIcon(on.rows[0]))
}

// --no-unicode wins over --icons. Asking for icons on a terminal that has just
// been told it cannot draw unicode is a contradiction, and the flag that says
// "you cannot" is the one to believe.
func TestNoUnicodeBeatsIcons(t *testing.T) {
	m := iconModel(t, true, true, "photo.jpg")
	sub := &analyze.Dir{File: &analyze.File{Name: "src", Parent: m.currentDir.(*analyze.Dir)}}
	m.currentDir.(*analyze.Dir).AddFile(sub)
	m.reloadRows()

	for _, r := range m.rows {
		cell := m.rowIcon(r)
		for _, ru := range cell {
			require.Less(t, ru, rune(0x80), "%q is not ASCII, under --no-unicode", cell)
		}
		assert.Equal(t, iconWidth, runewidth.StringWidth(cell))
	}
}

// Every glyph in the tables must be a single-cell private-use codepoint. A
// two-cell entry would silently widen the icon cell and shift the layout on the
// one file type that has it.
func TestEveryGlyphIsOneCellAndInThePrivateUseArea(t *testing.T) {
	check := func(what, key, glyph string) {
		t.Helper()
		runes := []rune(glyph)
		require.Len(t, runes, 1, "%s %q maps to %d runes, want 1", what, key, len(runes))
		assert.Equal(t, 1, runewidth.RuneWidth(runes[0]),
			"%s %q is %d cells wide; the icon cell has room for one glyph and a space",
			what, key, runewidth.RuneWidth(runes[0]))
		inPUA := (runes[0] >= 0xe000 && runes[0] <= 0xf8ff) || (runes[0] >= 0xf0000 && runes[0] <= 0xffffd)
		assert.True(t, inPUA, "%s %q is U+%04X, outside the Nerd Font private use area", what, key, runes[0])
	}

	require.NotEmpty(t, iconByName)
	require.NotEmpty(t, iconByExt)
	for k, v := range iconByName {
		check("iconByName", k, v)
	}
	for k, v := range iconByExt {
		check("iconByExt", k, v)
	}
	for _, k := range []string{iconFolder, iconFile, iconGit, iconBin} {
		check("fallback", k, k)
	}
}
