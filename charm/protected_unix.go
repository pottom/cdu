//go:build unix

package charm

import (
	"path/filepath"
	"syscall"
)

// isMountPoint asks the filesystem rather than the mount table: a directory is a
// mount point when it sits on a different device from its parent. That answer
// stays right for bind mounts, network shares and anything mounted after the scan
// started, none of which the cached device list would know about.
func isMountPoint(path string) bool {
	parent := filepath.Dir(path)
	if parent == path {
		return true // the root itself
	}

	var here, above syscall.Stat_t
	if err := syscall.Lstat(path, &here); err != nil {
		return false
	}
	if err := syscall.Lstat(parent, &above); err != nil {
		return false
	}
	return here.Dev != above.Dev
}
