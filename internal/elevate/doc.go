// Package elevate removes a file with elevated privileges when an ordinary delete
// is refused for lack of permission — and never handles the password itself.
//
// On Unix it hands the terminal to the real sudo (through the caller's
// tea.ExecProcess), so the prompt, the input masking and the sudoers policy are
// sudo's, not cdu's: the password never enters this process. On Windows, where UAC
// cannot be fed a password by an application (the consent prompt is on the secure
// desktop, by design), it declines and asks the user to relaunch as administrator.
//
// The two implementations are split by build tag, the same way internal/trash is.
package elevate
