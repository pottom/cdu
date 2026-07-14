// Package trash moves files to the operating system's trash, and puts them back.
//
// It exists because gdu has no trash: pkg/remove calls os.RemoveAll, which is
// final. A disk usage tool exists to be pointed at things and told to delete
// them, which is exactly the situation in which a mistake is easiest to make and
// most expensive to have made, so the default delete must be recoverable.
//
// The two costs of that are stated plainly rather than hidden, because both
// surprise people:
//
//  1. Trashing does not free disk space. The item stays on the same volume.
//     Someone deleting because a disk is full needs a permanent delete.
//  2. Trashing cannot cross a volume boundary. A rename cannot, and copying
//     gigabytes because the user pressed one key would be a worse surprise than
//     refusing, so this package refuses with ErrCrossVolume and lets the caller
//     offer the permanent delete instead.
//
// Every implementation is CGO-free, which is what rules out the usual libraries
// and is why this is written by hand per platform.
package trash

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

var (
	// ErrUnsupported means this platform has no trash cdu knows how to use.
	ErrUnsupported = errors.New("this platform has no trash cdu can use")

	// ErrCrossVolume means the item is not on the volume its trash lives on.
	// Trashing it would mean copying it, which is not what the user asked for.
	ErrCrossVolume = errors.New("the item is on a different volume from its trash")

	// ErrRestoreUnsupported means items can be trashed on this platform but not
	// put back by cdu. Callers must not offer an undo that cannot work.
	ErrRestoreUnsupported = errors.New("cdu cannot restore from this platform's trash")
)

// Entry is where an item went, and where it came from. It is what makes undo
// possible: the trash renames the item, so nothing else knows both halves.
type Entry struct {
	// OriginalPath is where the item was, and where Restore puts it back.
	OriginalPath string
	// TrashPath is where the item is now.
	TrashPath string
	// metaPath is a sidecar the platform's trash requires, and Restore removes.
	// Empty where the platform has none.
	metaPath string
}

// uniqueTarget finds a free name in dir for an item called name. Two files
// deleted from different directories on the same day routinely share a name, and
// the second must not silently destroy the first inside the trash.
func uniqueTarget(dir, name string) (string, error) {
	candidate := filepath.Join(dir, name)
	if _, err := os.Lstat(candidate); os.IsNotExist(err) {
		return candidate, nil
	}

	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 2; i < 10000; i++ {
		candidate = filepath.Join(dir, base+"."+strconv.Itoa(i)+ext)
		if _, err := os.Lstat(candidate); os.IsNotExist(err) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free name for %q in %s", name, dir)
}
