package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A preset with a malformed or unset token would paint that element black on
// black, and only on that one theme — the kind of thing nobody notices until a
// user reports it. Every bundled theme is checked, so adding one cannot skip it.
func TestEveryPresetIsCompleteAndValid(t *testing.T) {
	require.NotEmpty(t, Names())

	for _, name := range Names() {
		t.Run(name, func(t *testing.T) {
			th, ok := Preset(name)
			require.True(t, ok, "Names() offered %q but Preset() does not know it", name)
			assert.NotEmpty(t, th.Name, "a preset must name itself, for `cdu themes` and errors")
			require.NoError(t, th.Validate())

			if th.Plain {
				// mono is defined as the absence of colour; tokens would be dead weight.
				assert.Len(t, th.Missing(), len(TokenNames()), "a plain theme must carry no tokens")
				return
			}
			assert.Empty(t, th.Missing(), "every token must be set, or it renders as black")
		})
	}
}

func TestDefaultThemeIsBundled(t *testing.T) {
	th, ok := Preset(Default)
	require.True(t, ok, "the default theme must exist")
	assert.Equal(t, "charm", th.Name)
}

// Bare `catppuccin` is the flavour people mean when they say the name.
func TestCatppuccinResolvesToMocha(t *testing.T) {
	alias, ok := Preset("catppuccin")
	require.True(t, ok)
	assert.Equal(t, CatppuccinMocha(), alias)
	assert.Equal(t, "catppuccin-mocha", alias.Name, "the alias must report the flavour it really is")
}

// All four flavours are bundled, and the light one is flagged: cdu never paints
// the field, so a light theme on a dark terminal is unreadable and `cdu themes`
// has to be able to say which is which.
func TestCatppuccinFlavoursAndLightFlags(t *testing.T) {
	for _, name := range []string{"catppuccin-latte", "catppuccin-frappe", "catppuccin-macchiato", "catppuccin-mocha"} {
		_, ok := Preset(name)
		assert.True(t, ok, "%s must be bundled", name)
	}

	light := map[string]bool{"catppuccin-latte": true, "daylight": true}
	for _, name := range Names() {
		th, _ := Preset(name)
		assert.Equal(t, light[name], th.Light, "%s: Light flag is wrong", name)
	}
}

// An unknown name is reported, never silently swapped for the default: someone
// who typed `--theme mocha` wants to be told it is `catppuccin-mocha`, not to
// wonder why their theme did nothing.
func TestUnknownPresetIsReported(t *testing.T) {
	_, ok := Preset("mocha")
	assert.False(t, ok)
	_, ok = Preset("")
	assert.False(t, ok)
	_, ok = Preset("midnight") // dropped in favour of the catppuccin flavours
	assert.False(t, ok)
}

// The tokens are transcribed from catppuccin's palette.json by hand, so a
// handful are pinned against the published values. A typo here is invisible —
// it just renders as a slightly different purple — which is exactly why it is
// worth a test.
func TestCatppuccinTokensMatchTheUpstreamPalette(t *testing.T) {
	mocha := CatppuccinMocha()
	assert.Equal(t, Color("#313244"), mocha.Panel, "surface0")
	assert.Equal(t, Color("#cdd6f4"), mocha.Text, "text")
	assert.Equal(t, Color("#cba6f7"), mocha.Accent, "mauve")
	assert.Equal(t, Color("#f38ba8"), mocha.Danger, "red")

	latte := CatppuccinLatte()
	assert.Equal(t, Color("#ccd0da"), latte.Panel, "surface0")
	assert.Equal(t, Color("#4c4f69"), latte.Text, "text")
	assert.Equal(t, Color("#8839ef"), latte.Accent, "mauve")

	// overlay2 is the muted role with the most contrast against base in both
	// directions — the light flavour's ramp is already flipped upstream, which is
	// why one token name works for all four.
	assert.Equal(t, Color("#7c7f93"), latte.Dim, "latte overlay2")
	assert.Equal(t, Color("#9399b2"), mocha.Dim, "mocha overlay2")
}
