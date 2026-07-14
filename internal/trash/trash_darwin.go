//go:build darwin

package trash

import (
	"os"
	"path/filepath"
	"strconv"
)

// macOS keeps one trash per volume: ~/.Trash on the boot volume, and
// /Volumes/<name>/.Trashes/<uid> on the others. Finder shows them as one place,
// but they are separate directories, and an item can only be renamed into the one
// on its own volume.

// Supported reports whether items can be trashed at all.
func Supported() bool { return true }

// RestoreSupported reports whether Restore works. On macOS an item goes back by
// rename, so undo is real.
func RestoreSupported() bool { return true }

// MoveToTrash moves an item to the trash of the volume it lives on.
func MoveToTrash(path string) (*Entry, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	dir, err := trashDirFor(path)
	if err != nil {
		return nil, err
	}

	target, err := moveInto(path, dir)
	if err != nil {
		return nil, err
	}
	return &Entry{OriginalPath: path, TrashPath: target}, nil
}

// Restore puts a trashed item back.
func Restore(entry *Entry) error {
	return restoreEntry(entry)
}

// trashDirFor picks the trash on the item's own volume. Nothing is copied across
// a boundary, so an item on an external disk goes to that disk's trash.
func trashDirFor(path string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	homeTrash := filepath.Join(home, ".Trash")

	itemDev, err := deviceOf(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	homeDev, err := deviceOf(home)
	if err == nil && itemDev == homeDev {
		return homeTrash, nil
	}

	// A different volume: use its own .Trashes/<uid>, which is where Finder puts
	// deletions from external disks.
	root, err := mountRoot(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(root, ".Trashes", strconv.Itoa(os.Getuid())), nil
}
