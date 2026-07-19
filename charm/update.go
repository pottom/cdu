package charm

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/internal/updater"
)

// The header shows this build's version, and — once a background check has found one
// — a yellow ↯ and the newer version beside it. The check is one GET to GitHub's
// public releases API at startup: it sends nothing about the user or the machine,
// runs off the render loop, and stays silent on any failure, so a machine with no
// network simply never shows the mark. CDU_NO_UPDATE_CHECK turns it off entirely;
// CDU_FAKE_LATEST_VERSION drives it for a demo without a real release to point at.

type updateAvailableMsg struct{ tag string }

// checkUpdate looks for a newer release off the render loop. It returns a message
// only when there is one to show — nil otherwise, so nothing redraws for a check
// that found the build current, could not run, or was switched off.
func (m *model) checkUpdate() tea.Cmd {
	if os.Getenv("CDU_NO_UPDATE_CHECK") != "" {
		return nil
	}
	current := m.ui.version

	if fake := os.Getenv("CDU_FAKE_LATEST_VERSION"); fake != "" {
		return func() tea.Msg {
			if updater.IsNewer(current, fake) {
				return updateAvailableMsg{tag: fake}
			}
			return nil
		}
	}

	return func() tea.Msg {
		tag, err := updater.LatestTag()
		if err != nil || !updater.IsNewer(current, tag) {
			return nil
		}
		return updateAvailableMsg{tag: tag}
	}
}
