package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListNamesEveryThemeAndMarksTheCurrent(t *testing.T) {
	var b strings.Builder
	require.NoError(t, List(&b, "ember"))
	out := b.String()

	for _, name := range Names() {
		assert.Contains(t, out, name, "every bundled theme must be listed")
	}
	assert.Contains(t, out, "* ember", "the theme in use must be marked")
	assert.NotContains(t, out, "* charm", "only one theme is in use")
}

// mono has no colours to preview, and saying so is more useful than an empty gap.
func TestListSaysMonoHasNoColour(t *testing.T) {
	var b strings.Builder
	require.NoError(t, List(&b, ""))
	assert.Regexp(t, `mono\s+any\s+\(no colour\)`, b.String())
}

// The listing is the answer to "what can I type", so it has to say how to type
// it and what each theme expects of the terminal.
func TestListSaysWhatEachThemeExpects(t *testing.T) {
	var b strings.Builder
	require.NoError(t, List(&b, ""))
	out := b.String()

	assert.Regexp(t, `midnight\s+dark`, out)
	assert.Regexp(t, `phosphor\s+dark`, out)
	assert.Contains(t, out, "--theme NAME", "it must say how to use one")

	// The background rule only applies to a light theme, and none is bundled. It
	// would be noise on a listing where every row says "dark".
	assert.NotContains(t, out, "light theme needs a light terminal",
		"do not explain light themes when none is listed")
}

// Piped into a file or run on a dumb terminal, the listing still has to be
// readable — it is the one command whose whole output is colour.
func TestListStaysUsableWithoutColour(t *testing.T) {
	original := lipgloss.ColorProfile()
	defer lipgloss.SetColorProfile(original)
	lipgloss.SetColorProfile(termenv.Ascii)

	var b strings.Builder
	require.NoError(t, List(&b, "charm"))
	out := b.String()

	assert.NotContains(t, out, "\x1b", "no escape may reach a dumb terminal")
	assert.Contains(t, out, "charm", "the names still have to be there")
	assert.Contains(t, out, "* charm")
}

// The listing enumerates the tokens from the struct, so it cannot drift from
// what the config actually accepts.
func TestListDocumentsTheRealTokens(t *testing.T) {
	var b strings.Builder
	require.NoError(t, List(&b, ""))

	for _, token := range TokenNames() {
		assert.Contains(t, b.String(), token)
	}
}
