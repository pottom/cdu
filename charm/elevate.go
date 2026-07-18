package charm

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/elevate"
	"github.com/pottom/cdu/pkg/fs"
)

// Elevated delete is the second offer, made only after an ordinary removal is
// refused for want of permission. cdu never takes the password: on Unix the terminal
// is handed to the real sudo (tea.ExecProcess), so the prompt, the masking and the
// policy are sudo's; on Windows, where UAC cannot be fed a password by a program,
// cdu says to relaunch as administrator instead. It is always permanent — the trash
// is per-user, and a root-owned file has no place in it — and it always asks you to
// type the word, being destructive and usually a system path.

type elevatedDoneMsg struct {
	item, parent fs.Item
	err          error
}

// offerElevation is reached when a delete came back with a permission error. Where
// elevation is possible it opens a second modal; where it is not, it says what to do
// instead rather than leaving the wall with no way over it.
func (m *model) offerElevation(item, parent fs.Item) tea.Cmd {
	if !elevate.Available() {
		m.status, m.statusIsError = elevate.Reason(), true
		return nil
	}
	m.confirm = &confirmState{
		item:          item,
		parent:        parent,
		act:           actionDelete,
		elevated:      true,
		requireTyping: true,
	}
	m.scr = screenConfirm
	return nil
}

// runElevatedDelete hands the terminal to sudo to remove the item, and never sees
// the password. A silent check first — is sudo already unlocked? — decides whether
// the handoff will show a prompt at all; when it will, a line on the terminal says
// so, so the interface stepping aside reads as intentional and not as a glitch.
func (m *model) runElevatedDelete(item, parent fs.Item) tea.Cmd {
	notice := ""
	if !elevate.Cached() {
		notice = "cdu is removing " + item.GetPath() + " with elevated privileges — sudo will ask for your password:"
	}
	cmd := elevate.RemoveCmd(item.GetPath(), notice)
	if cmd == nil {
		// Available() said yes but there is no command to run — a platform mismatch
		// that should never reach here. Fail loudly rather than pretend it worked.
		m.status, m.statusIsError = "elevated delete is not available here", true
		return nil
	}
	m.status, m.statusIsError = "removing "+item.GetName()+" with elevated privileges…", false
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return elevatedDoneMsg{item: item, parent: parent, err: err}
	})
}

// applyElevatedDelete finishes the tree half once sudo has returned. sudo rm exits
// nonzero if it could not remove the item — a wrong password, a cancelled prompt, or
// a flag even root cannot get past, like an immutable file — and that is reported,
// not a success invented for a row still on disk.
func (m *model) applyElevatedDelete(msg elevatedDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.status, m.statusIsError = "elevated delete failed: "+msg.err.Error(), true
		return nil
	}
	msg.parent.RemoveFile(msg.item)
	m.dropRow(msg.item)
	m.dropTopFile(msg.item)
	m.dropDuplicate(msg.item)
	m.dropFindResult(msg.item)
	m.status, m.statusIsError = msg.item.GetName()+" removed with elevated privileges", false
	// A permanent delete frees space, so the header's disk gauge is now stale.
	return deviceCmd(m.ui)
}
