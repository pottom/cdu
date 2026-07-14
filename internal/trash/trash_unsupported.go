//go:build !unix && !windows

package trash

// Plan 9 and anything else gdu builds for but that has no trash cdu knows how to
// drive. Supported reports false so the interface never offers a recoverable
// delete it cannot perform — a "moved to trash" that quietly deleted the file
// would be the worst failure this package could have.

// Supported reports whether items can be trashed at all.
func Supported() bool { return false }

// RestoreSupported reports whether Restore works.
func RestoreSupported() bool { return false }

// MoveToTrash always fails here.
func MoveToTrash(_ string) (*Entry, error) {
	return nil, ErrUnsupported
}

// Restore always fails here.
func Restore(_ *Entry) error {
	return ErrUnsupported
}
