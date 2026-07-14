//go:build unix

package trash

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// deviceOf is the volume a path lives on. Whether two paths share a volume is
// the only thing that decides whether the trash can accept an item, so it is
// asked directly rather than inferred from the path text.
func deviceOf(path string) (uint64, error) {
	var st syscall.Stat_t
	if err := syscall.Lstat(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Dev), nil
}

// mountRoot walks up from a path until the volume changes. The directory on the
// far side of that change is the mount point, which is where a volume's own
// trash lives.
func mountRoot(path string) (string, error) {
	dev, err := deviceOf(path)
	if err != nil {
		return "", err
	}

	current := path
	for {
		parent := filepath.Dir(current)
		if parent == current {
			return current, nil // reached "/"
		}
		parentDev, err := deviceOf(parent)
		if err != nil || parentDev != dev {
			return current, nil
		}
		current = parent
	}
}

// moveInto renames an item into a trash directory, refusing rather than copying
// when the two are on different volumes. Copying could mean gigabytes of I/O in
// response to a single keypress, and would not free the space the user is trying
// to reclaim anyway.
func moveInto(path, trashDir string) (string, error) {
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		return "", fmt.Errorf("creating %s: %w", trashDir, err)
	}

	target, err := uniqueTarget(trashDir, filepath.Base(path))
	if err != nil {
		return "", err
	}

	if err := os.Rename(path, target); err != nil {
		if isCrossDevice(err) {
			return "", ErrCrossVolume
		}
		return "", err
	}
	return target, nil
}

func isCrossDevice(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EXDEV
	}
	return false
}

// restoreEntry puts an item back where it came from and clears up whatever the
// platform's trash keeps alongside it. It refuses to overwrite: in the time since
// the delete, something else may have taken the name, and an undo that destroys
// data is not an undo.
func restoreEntry(entry *Entry) error {
	if _, err := os.Lstat(entry.OriginalPath); err == nil {
		return fmt.Errorf("%s already exists again; not overwriting it", entry.OriginalPath)
	}

	if err := os.MkdirAll(filepath.Dir(entry.OriginalPath), 0o700); err != nil {
		return err
	}
	if err := os.Rename(entry.TrashPath, entry.OriginalPath); err != nil {
		return err
	}

	// The sidecar goes last: while it exists, the desktop's trash still believes
	// the item is in there.
	if entry.metaPath == "" {
		return nil
	}
	if err := os.Remove(entry.metaPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
