//go:build windows

package trash

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

// Windows has no trash directory to rename into: the Recycle Bin is a shell
// concept, and an item only lands in it — with the metadata that lets Windows put
// it back — if the shell moves it. SHFileOperation with FOF_ALLOWUNDO is the
// CGO-free way to ask the shell to do that.
//
// The reverse does not exist without COM: restoring means talking to the Recycle
// Bin folder object. Rather than fake an undo that would silently do nothing,
// RestoreSupported reports false and the interface does not offer the key.

const (
	foDelete           = 3
	fofSilent          = 0x0004
	fofNoConfirmation  = 0x0010
	fofAllowUndo       = 0x0040
	fofNoErrorUI       = 0x0400
	fofNoConfirmMkdir  = 0x0200
	fofWantNukeWarning = 0x4000
)

type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	shFileOperationW = shell32.NewProc("SHFileOperationW")
)

// Supported reports whether items can be trashed at all.
func Supported() bool { return true }

// RestoreSupported reports whether Restore works. It does not on Windows: taking
// an item back out of the Recycle Bin needs the shell's COM interface.
func RestoreSupported() bool { return false }

// MoveToTrash asks the shell to recycle an item.
func MoveToTrash(path string) (*Entry, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	// pFrom is a double-null-terminated list, not a string. A single trailing NUL
	// would run the shell off the end of the buffer.
	from, err := syscall.UTF16FromString(path)
	if err != nil {
		return nil, err
	}
	from = append(from, 0)

	op := shFileOpStruct{
		wFunc: foDelete,
		pFrom: &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofNoConfirmMkdir |
			fofSilent | fofNoErrorUI | fofWantNukeWarning,
	}

	// The shell reports failure through the return value, not through GetLastError.
	ret, _, _ := shFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return nil, fmt.Errorf("the shell refused to recycle %s (code %d)", path, ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return nil, fmt.Errorf("recycling %s was aborted", path)
	}

	// There is no path to record: the item is inside the Recycle Bin under a name
	// only the shell knows. TrashPath stays empty, and Restore refuses.
	return &Entry{OriginalPath: path}, nil
}

// Restore always fails on Windows. Callers must check RestoreSupported first and
// not offer an undo they cannot honour.
func Restore(_ *Entry) error {
	return ErrRestoreUnsupported
}
