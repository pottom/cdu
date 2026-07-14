//go:build unix && !darwin

package trash

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// The freedesktop.org Trash specification. An item does not merely move: it moves
// into <trash>/files/, and a matching <trash>/info/<name>.trashinfo records where
// it came from. Without that sidecar the desktop's trash shows an item it cannot
// put back, so the two are written and removed together.
//
// Each volume has its own trash — <mount>/.Trash-<uid> — because the spec, like
// this package, will not copy an item across a boundary just to delete it.

// Supported reports whether items can be trashed at all.
func Supported() bool { return true }

// RestoreSupported reports whether Restore works. Under the spec an item goes
// back by rename, so undo is real.
func RestoreSupported() bool { return true }

// MoveToTrash moves an item to the trash of the volume it lives on, and writes
// the .trashinfo that makes it restorable — by cdu and by the desktop alike.
func MoveToTrash(path string) (*Entry, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	trashDir, relativeTo, err := trashDirFor(path)
	if err != nil {
		return nil, err
	}

	filesDir := filepath.Join(trashDir, "files")
	infoDir := filepath.Join(trashDir, "info")
	if err := os.MkdirAll(infoDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", infoDir, err)
	}

	target, err := moveInto(path, filesDir)
	if err != nil {
		return nil, err
	}

	infoPath := filepath.Join(infoDir, filepath.Base(target)+".trashinfo")
	if err := writeInfo(infoPath, path, relativeTo); err != nil {
		// The move already happened. Rather than leave an item in the trash that
		// nothing can restore, put it back and report the failure.
		if restoreErr := os.Rename(target, path); restoreErr != nil {
			// Both halves are wrapped: the caller needs to know that the item is now
			// stranded in the trash without a sidecar, not merely that a write failed.
			return nil, fmt.Errorf("%w (and %s could not be moved back: %w)", err, target, restoreErr)
		}
		return nil, err
	}

	return &Entry{OriginalPath: path, TrashPath: target, metaPath: infoPath}, nil
}

// Restore puts a trashed item back and removes its sidecar, so the desktop's
// trash does not go on listing an item that is no longer in it.
func Restore(entry *Entry) error {
	return restoreEntry(entry)
}

// trashDirFor picks the trash on the item's own volume, and reports the path that
// the .trashinfo's Path is written relative to: absolute for the home trash, and
// relative to the mount point for a volume trash, as the spec requires.
func trashDirFor(path string) (trashDir, relativeTo string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	homeTrash := filepath.Join(dataHome, "Trash")

	itemDev, err := deviceOf(filepath.Dir(path))
	if err != nil {
		return "", "", err
	}
	// The home trash may not exist yet, so the device is taken from the directory
	// that will hold it.
	homeDev, homeErr := deviceOf(filepath.Dir(homeTrash))
	if homeErr != nil {
		homeDev, homeErr = deviceOf(home)
	}
	if homeErr == nil && itemDev == homeDev {
		return homeTrash, "", nil
	}

	root, err := mountRoot(filepath.Dir(path))
	if err != nil {
		return "", "", err
	}
	return filepath.Join(root, ".Trash-"+strconv.Itoa(os.Getuid())), root, nil
}

// writeInfo writes the .trashinfo sidecar. The Path is URL-encoded per the spec —
// a filename containing a newline or a percent sign would otherwise produce a file
// that parses as something else entirely.
func writeInfo(infoPath, originalPath, relativeTo string) error {
	recorded := originalPath
	if relativeTo != "" {
		rel, err := filepath.Rel(relativeTo, originalPath)
		if err != nil {
			return err
		}
		recorded = rel
	}

	info := fmt.Sprintf(
		"[Trash Info]\nPath=%s\nDeletionDate=%s\n",
		(&url.URL{Path: recorded}).EscapedPath(),
		time.Now().Format("2006-01-02T15:04:05"),
	)
	return os.WriteFile(infoPath, []byte(info), 0o600)
}
