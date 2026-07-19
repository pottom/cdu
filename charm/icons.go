package charm

import (
	"path/filepath"
	"strings"

	"github.com/pottom/cdu/pkg/fs"
)

// Icons are off unless --icons, and that default is not timidity.
//
// The glyphs live in the Nerd Fonts private use area. A terminal without a
// patched font draws them as boxes, and cdu cannot tell — the program has no way
// to ask what font is loaded. So the person who has one opts in; the person who
// does not gets the markers below and never sees a row of tofu.
//
// Width is the other half of the same problem. runewidth measures a PUA
// codepoint as one cell, but a Nerd Font's non-Mono variants draw the icons
// across two. That would shift every column by one — except that the icon cell
// is always glyph plus a space, so a double-width glyph spills into the space
// and the next column stays put. It is the same trick exa uses, and it is why
// iconWidth is 2 rather than 1.
const (
	// iconFolder and iconFile are the fallbacks: exa's own, for anything the
	// tables do not name.
	iconFolder = ""
	iconFile   = ""

	// Directories worth telling apart at a glance. A .git is not a folder you
	// meant to fill up, and bin is not one you meant to keep.
	iconGit = ""
	iconBin = ""
)

// markerDir and markerFile are what the icon cell holds without --icons: the
// brief's own markers, which any terminal can draw.
const (
	markerDir  = "▸"
	markerFile = "·"
	// markerBack is the ../ row's icon: a left arrow, because the row is the way
	// back out of the directory, the same direction ← / h take.
	markerBack = "←"
	// Under --no-unicode even those go, matching gdu's scope for the flag.
	asciiMarkerDir  = ">"
	asciiMarkerFile = " "
	asciiMarkerBack = "<"
)

// The device list's own icon cell. A physical disk and a volume on it are
// different kinds of thing — you can analyze a volume and you cannot analyze a
// container — so the tree says it by shape and these say it again, for the same
// reason the cursor row carries a marker as well as a colour.
const (
	iconDisk   = "\U000f02ca" // nf-md-harddisk
	iconVolume = ""          // nf-fae-database
	// A mount with no disk behind it: tmpfs, devfs, an automount map.
	iconPseudo = "" // nf-fa-cubes

	markerDisk        = "■"
	markerVolume      = "▸"
	asciiMarkerDisk   = "#"
	asciiMarkerVolume = ">"
)

// diskIcon is the icon cell for a device-list row: exactly iconWidth columns, or
// nothing when the terminal cannot hold one.
func (m *model) diskIcon(r *diskRow) string {
	if m.width < minWidthForIcon {
		return ""
	}
	switch {
	case m.ui.noUnicode:
		if r.isHeader() {
			return asciiMarkerDisk + " "
		}
		return asciiMarkerVolume + " "
	case m.ui.icons:
		switch {
		case r.isHeader():
			return iconDisk + " "
		case r.depth == 0:
			return iconPseudo + " " // not on a disk at all
		}
		return iconVolume + " "
	case r.isHeader():
		return markerDisk + " "
	}
	return markerVolume + " "
}

// iconFor returns the glyph for an item, without its trailing space.
//
// The order is exa's: an exact filename first, then the fact that it is a
// directory, then the extension. A name beats an extension because `Dockerfile`
// and `.gitignore` have no useful extension between them, and a directory beats
// an extension because `node_modules.bak` is a directory, not a backup file.
func iconFor(item fs.Item) string {
	name := item.GetName()

	if icon, ok := iconByName[name]; ok {
		return icon
	}
	if item.IsDir() {
		switch name {
		case ".git":
			return iconGit
		case "bin":
			return iconBin
		}
		return iconFolder
	}

	// filepath.Ext keeps the dot and the case; the table has neither. A dotfile
	// with no extension ("..bashrc") comes out empty and falls through, which is
	// right — it was already looked up by name.
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if icon, ok := iconByExt[ext]; ok {
		return icon
	}
	return iconFile
}

// rowIcon is the icon cell: exactly iconWidth columns, or nothing at all when
// the terminal is too narrow to hold it.
func (m *model) rowIcon(item fs.Item) string {
	if m.width < minWidthForIcon {
		return ""
	}
	switch {
	case item == m.pending:
		// The removal is happening off the render loop and can take seconds. The
		// row spins so that the wait is visible rather than looking like a key that
		// never registered.
		return m.tickFrame() + " "
	case m.ui.noUnicode:
		if item.IsDir() {
			return asciiMarkerDir + " "
		}
		return asciiMarkerFile + " "
	case m.ui.icons:
		return iconFor(item) + " "
	case item.IsDir():
		return markerDir + " "
	}
	return markerFile + " "
}
