//go:build !unix

package charm

// On Windows a volume root is already caught by the VolumeName check in
// isProtected, and there is no cheap device-number test for anything else. The
// other protections — the filesystem root, the home directory, and everything
// directly inside it — still apply.
func isMountPoint(string) bool { return false }
