package charm

import (
	"os"
	"path/filepath"
)

// confirmWord is what has to be typed out to delete a protected path. It is
// deliberately not a single key: the whole point is to interrupt a reflex.
const confirmWord = "DELETE"

// isProtected reports whether a path is one where a mistaken delete is
// catastrophic rather than merely annoying: the filesystem root, a home
// directory, or a mount point.
//
// The guard is on the path, so it holds for a permanent delete and for a trash
// alike — trashing $HOME would still take the whole home directory out from under
// the user, and on a full disk it would not even free anything.
func isProtected(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return true // if we cannot even resolve it, assume the worst
	}
	abs = filepath.Clean(abs)

	if abs == string(filepath.Separator) || abs == filepath.VolumeName(abs)+string(filepath.Separator) {
		return true
	}
	if home, err := os.UserHomeDir(); err == nil {
		if abs == filepath.Clean(home) {
			return true
		}
		// The directories directly under $HOME are where a scan usually starts, and
		// so are where a stray keypress most often lands.
		if parent := filepath.Dir(abs); parent == filepath.Clean(home) {
			return true
		}
	}
	return isMountPoint(abs)
}
