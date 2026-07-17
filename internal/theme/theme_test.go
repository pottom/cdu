package theme

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tokens() walks the struct by reflection so that validation, merging and the
// config writer all pick up a new token for free. The cost of that convenience
// is this test: a token added without a yaml tag would be skipped by all three
// silently, and the only symptom would be a colour a user cannot override.
func TestEveryColourFieldIsAToken(t *testing.T) {
	ty := reflect.TypeFor[Theme]()

	var want int
	for i := range ty.NumField() {
		f := ty.Field(i)
		if f.Type != reflect.TypeFor[Color]() {
			continue
		}
		key, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		assert.NotEmpty(t, key, "%s is a Color but has no yaml key, so it can never be configured", f.Name)
		assert.NotEqual(t, "-", key, "%s is a Color but is excluded from yaml", f.Name)
		want++
	}

	assert.Len(t, TokenNames(), want, "every Color field must be reachable as a token")
	assert.Positive(t, want, "reflection found no tokens at all, which means it is broken")
}

func TestColorAcceptsOnlySixDigitHex(t *testing.T) {
	for _, good := range []Color{"#ff5fd1", "#FF5FD1", "#000000"} {
		assert.True(t, good.Valid(), "%q is a hex colour", good)
	}
	// "5" is the one that matters: Lipgloss would take it as an ANSI index, but
	// the bar cannot blend it, so it is refused at the door rather than coming
	// out black halfway down the gradient.
	for _, bad := range []Color{"5", "#fff", "ff5fd1", "#ff5fd", "#gggggg", "red", ""} {
		assert.False(t, bad.Valid(), "%q must not pass as a hex colour", bad)
	}
}

// A typo in a config must be reported, not rendered. The error names the key and
// the value, because "invalid theme" would send the user hunting through eleven
// tokens.
func TestValidateNamesTheBadTokens(t *testing.T) {
	th := Charm()
	require.NoError(t, th.Validate(), "the bundled preset must be valid")

	th.Accent = "nope"
	th.BarTo = "#ff5"
	err := th.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `accent: "nope"`)
	assert.Contains(t, err.Error(), `bar-to: "#ff5"`)

	// An unset token is not an error — it inherits from the preset.
	th = Theme{Accent: "#ff5fd1"}
	assert.NoError(t, th.Validate())
}

// A preset with an unset token would paint that element black on black. Missing
// is what the preset test and the config loader both check.
func TestMissingReportsUnsetTokens(t *testing.T) {
	th := Charm()
	assert.Empty(t, th.Missing(), "the bundled preset must set every token")

	var empty Theme
	assert.Len(t, empty.Missing(), len(TokenNames()), "a zero Theme is missing everything")

	th.Accent = ""
	th.Dim = ""
	assert.Equal(t, []string{"accent", "dim"}, th.Missing())
}

// Overlay is how a user's partial theme block lands on a preset: the two keys
// they set win, the other nine survive.
func TestOverlayOnlyTakesTokensThatAreSet(t *testing.T) {
	base := Charm()
	base.Overlay(&Theme{Accent: "#00ff00", BarTo: "#0000ff"})

	assert.Equal(t, Color("#00ff00"), base.Accent, "a set token overrides")
	assert.Equal(t, Color("#0000ff"), base.BarTo)
	assert.Equal(t, Charm().Panel, base.Panel, "an unset token must not blank the preset's")
	assert.Equal(t, Charm().Size, base.Size)
	assert.Empty(t, base.Missing(), "overlaying a partial theme must not leave holes")
}

// Name is not a colour and must never be settable from the theme block — it
// reports which preset the colours came from.
func TestOverlayDoesNotTouchName(t *testing.T) {
	base := Charm()
	base.Overlay(&Theme{Name: "impostor", Accent: "#00ff00"})
	assert.Equal(t, "charm", base.Name)
}
