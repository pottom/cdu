package charm

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/elevate"
	"github.com/pottom/cdu/pkg/fs"
)

// Elevated delete is the second offer, made only after a removal is refused for want
// of permission. cdu never takes the password: on Unix the terminal is handed to the
// real sudo (tea.ExecProcess), so the prompt, the masking and the policy are sudo's;
// on Windows, where UAC cannot be fed a password by a program, cdu says to relaunch
// as administrator instead. It is always permanent — the trash is per-user, and a
// root-owned file has no place in it — and always types to confirm.
//
// It works for a single delete and for a batch: a batch collects the items it could
// not remove and offers them all to one sudo, so a marked set of root-owned files
// costs a single prompt.

type elevatedDoneMsg struct {
	items []fs.Item
	err   error
}

// offerElevation opens the elevated-removal modal for a set of items, or — where
// elevation is impossible — says what to do instead rather than leaving the wall
// with no way over it. The set is never empty when this is reached.
func (m *model) offerElevation(items []fs.Item) tea.Cmd {
	if !elevate.Available() {
		m.status, m.statusIsError = elevate.Reason(), true
		return nil
	}
	m.confirm = &confirmState{
		elevateItems:  items,
		act:           actionDelete,
		elevated:      true,
		requireTyping: true,
	}
	m.scr = screenConfirm
	return nil
}

// runElevatedDelete hands the terminal to sudo to remove the set in one invocation,
// and never sees the password. A silent check first — is sudo already unlocked? —
// decides whether a prompt shows at all; when it will, a line on the terminal says
// so, so the interface stepping aside reads as intentional and not as a glitch.
func (m *model) runElevatedDelete(items []fs.Item) tea.Cmd {
	paths := make([]string, len(items))
	for i, it := range items {
		paths[i] = it.GetPath()
	}

	notice := ""
	if !elevate.Cached() {
		notice = "cdu is removing " + elevatedLabel(items) + " with elevated privileges — sudo will ask for your password:"
	}

	cmd := elevate.RemoveCmd(paths, notice)
	if cmd == nil {
		m.status, m.statusIsError = "elevated delete is not available here", true
		return nil
	}
	m.status, m.statusIsError = "removing "+elevatedLabel(items)+" with elevated privileges…", false
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return elevatedDoneMsg{items: items, err: err}
	})
}

// elevatedLabel names the target of an elevated removal for a status line: the one
// path, or a count when it is a set.
func elevatedLabel(items []fs.Item) string {
	if len(items) == 1 {
		return items[0].GetPath()
	}
	return fmt.Sprintf("%d %s", len(items), itemNoun(len(items)))
}

// applyElevatedDelete finishes the tree half once sudo has returned. sudo rm exits
// nonzero if it could not remove everything — a wrong password, a cancelled prompt,
// or a flag even root cannot get past, like an immutable file — and that is reported,
// not a success invented for rows still on disk.
func (m *model) applyElevatedDelete(msg elevatedDoneMsg) tea.Cmd {
	if msg.err != nil {
		m.status, m.statusIsError = "elevated delete failed: "+msg.err.Error(), true
		return nil
	}
	for _, item := range msg.items {
		if p := item.GetParent(); p != nil {
			p.RemoveFile(item)
		}
		m.dropRow(item)
		m.dropTopFile(item)
		m.dropDuplicate(item)
		m.dropFindResult(item)
	}
	m.status, m.statusIsError = fmt.Sprintf("%d %s removed with elevated privileges",
		len(msg.items), itemNoun(len(msg.items))), false
	// A permanent delete frees space, so the header's disk gauge is now stale.
	return deviceCmd(m.ui)
}
