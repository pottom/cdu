//go:build windows

package elevate

import "os/exec"

// Available is false on Windows. UAC elevation cannot be handed a password by an
// application — the consent prompt lives on the secure desktop, by design — so
// there is no in-app path to it, and cdu offers a message instead of a password box
// that could not work.
func Available() bool { return false }

// Cached is always false on Windows, since Available is false and nothing is ever
// run.
func Cached() bool { return false }

// RemoveCmd is never called on Windows, because Available is false; it returns nil
// so a caller that ignored that would fail loudly rather than run something wrong.
func RemoveCmd(_, _ string) *exec.Cmd { return nil }

// Reason is what to tell the user instead: elevation here means relaunching the
// whole program as administrator, which cdu cannot do to itself mid-run.
func Reason() string {
	return "an elevated delete is not available on Windows; restart cdu as administrator to remove this"
}
