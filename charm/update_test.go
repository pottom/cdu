package charm

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The fake-latest env var drives the check without a real release to point at, so a
// demo (or a test) can see the mark.
func TestCheckUpdateWithFakeLatest(t *testing.T) {
	t.Setenv("CDU_NO_UPDATE_CHECK", "")
	t.Setenv("CDU_FAKE_LATEST_VERSION", "v9.9.9")

	m := benchModel(1)
	m.ui.version = "v1.0.0"

	cmd := m.checkUpdate()
	require.NotNil(t, cmd)
	msg, ok := cmd().(updateAvailableMsg)
	require.True(t, ok, "a newer fake version reports an update")
	assert.Equal(t, "v9.9.9", msg.tag)
}

// A fake that is not newer reports nothing, so the header stays clean.
func TestCheckUpdateFakeNotNewer(t *testing.T) {
	t.Setenv("CDU_NO_UPDATE_CHECK", "")
	t.Setenv("CDU_FAKE_LATEST_VERSION", "v1.0.0")

	m := benchModel(1)
	m.ui.version = "v1.0.0"

	assert.Nil(t, m.checkUpdate()(), "same version, no message")
}

// The opt-out env var skips the check entirely — no command, no network.
func TestCheckUpdateDisabled(t *testing.T) {
	t.Setenv("CDU_NO_UPDATE_CHECK", "1")

	m := benchModel(1)
	m.ui.version = "v1.0.0"
	assert.Nil(t, m.checkUpdate(), "disabled means no command at all")
}

// The message from the check lands the newer version on the model, which the header
// then shows.
func TestUpdateMessageSetsLatest(t *testing.T) {
	m := benchModel(1)
	next, _ := m.Update(updateAvailableMsg{tag: "v2.0.0"})
	assert.Equal(t, "v2.0.0", next.(*model).latestVersion)
}

// The header carries the build's version, and — once an update is known — the ↯ and
// the newer version beside it.
func TestHeaderShowsVersionAndUpdateMark(t *testing.T) {
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(original)

	m := benchModel(1)
	m.ui.version = "v1.2.3"
	m.scr = screenBrowse

	assert.Contains(t, m.viewHeader(), "v1.2.3", "the version is shown")
	assert.NotContains(t, m.viewHeader(), "↯", "no mark until an update is found")

	m.latestVersion = "v1.3.0"
	assert.Contains(t, m.viewHeader(), "↯v1.3.0", "the newer version is marked")
}
