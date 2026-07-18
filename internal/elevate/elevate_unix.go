//go:build !windows

package elevate

import "os/exec"

// Available reports whether an elevated delete can be attempted here: on Unix, when
// there is a sudo on PATH to hand the terminal to.
func Available() bool {
	_, err := exec.LookPath("sudo")
	return err == nil
}

// Cached reports whether sudo would run without prompting — its credentials are
// still in the timestamp cache. It is a silent, side-effect-free check (sudo -n
// true), used only to decide whether the handoff will show a prompt.
func Cached() bool {
	return exec.Command("sudo", "-n", "true").Run() == nil
}

// RemoveCmd is the command that removes path with elevated privileges, ready to be
// run by tea.ExecProcess. It is `sudo rm -rf`, which covers a file or a whole
// directory.
//
// When notice is non-empty it is printed on the terminal just before sudo runs, so
// the handoff — the moment the TUI steps aside — reads as intentional rather than as
// the interface vanishing. The path is always a positional argument, never spliced
// into the little shell script, so no filename can break out of it.
func RemoveCmd(path, notice string) *exec.Cmd {
	if notice == "" {
		return exec.Command("sudo", "rm", "-rf", "--", path)
	}
	return exec.Command("sh", "-c", `printf '%s\n' "$1"; exec sudo rm -rf -- "$2"`, "sh", notice, path)
}

// Reason explains why an elevated delete is unavailable. On Unix that only happens
// when sudo is missing, which is rare, but the message is here so the caller has one
// shape to code against on every platform.
func Reason() string {
	return "no sudo found on PATH; cdu cannot elevate here — remove it from a shell instead"
}
