package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadBundled panics on a malformed theme, on the grounds that a corrupt build
// is not something to render around. This is the test that lets it: it proves
// the panic cannot fire, by parsing every embedded theme the same way the loader
// does — including the checks that a theme sets every token, or none at all.
func TestEveryBundledThemeParses(t *testing.T) {
	require.NotPanics(t, func() { _ = Names() })

	for _, name := range Names() {
		th, ok := Preset(name)
		require.True(t, ok, "Names() offered %q but Preset() does not know it", name)

		t.Run(name, func(t *testing.T) {
			assert.Equal(t, name, th.Name, "a theme's name is its filename; they cannot disagree")
			require.NoError(t, th.Validate())

			if th.Plain {
				assert.Len(t, th.Missing(), len(TokenNames()), "a plain theme must carry no tokens")
				return
			}
			assert.Empty(t, th.Missing(), "an unset token renders as black")
		})
	}
}

// The brief names exactly five, and the set drifted to nine once before. The
// count is not decoration: every one of them is a promise to keep working, and
// `cdu themes` is meant to be a list a person reads rather than scrolls.
func TestTheBundledSetIsExactlyTheBriefsFive(t *testing.T) {
	assert.Equal(t, []string{"charm", "ember", "midnight", "mono", "phosphor"}, Names())
}

func TestDefaultThemeIsBundled(t *testing.T) {
	th, ok := Preset(Default)
	require.True(t, ok, "the default theme must exist")
	assert.Equal(t, "charm", th.Name)
	assert.Equal(t, Charm(), th)
}

// An unknown name is reported, never silently swapped for the default.
//
// `catppuccin` and `glacier` are in the list because they briefly existed:
// catppuccin was replaced by the brief's own five, and glacier turned out to be
// midnight under another name. Anyone who tried them should be told they are
// gone rather than quietly given charm.
func TestUnknownPresetIsReported(t *testing.T) {
	for _, name := range []string{"nonsense", "", "catppuccin", "glacier", "daylight"} {
		_, ok := Preset(name)
		assert.False(t, ok, "%q must not resolve", name)
	}
}

// The parser is what user themes will go through too (task: ~/.config/cdu/themes),
// so its refusals are worth pinning: a theme that is neither complete nor plain
// would render something black, and a plain theme with colours in it is a
// contradiction rather than a preference.
func TestParseRefusesAHalfFinishedTheme(t *testing.T) {
	_, err := parse("partial", []byte("accent: \"#ff0000\"\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing tokens")
	assert.Contains(t, err.Error(), "panel", "the error must name what is missing")

	_, err = parse("bad-hex", []byte("accent: \"nope\"\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a #rrggbb colour")

	_, err = parse("confused", []byte("plain: true\naccent: \"#ff0000\"\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uses no colour")

	_, err = parse("garbage", []byte("\tthis: is: not: yaml\n"))
	assert.Error(t, err)
}

// The name is the filename, and the file has no say in it. A `name:` key would
// be a second source of truth and the two would eventually disagree.
func TestParseTakesTheNameFromTheCaller(t *testing.T) {
	th, err := parse("mine", []byte("plain: true\nname: something-else\n"))
	require.NoError(t, err)
	assert.Equal(t, "mine", th.Name)
}

// The metadata keys have to survive the round trip from file to Theme — light in
// particular, because it is the only thing that tells a reader a theme needs a
// light terminal.
//
// No bundled theme is light any more: daylight was dropped for phosphor, and the
// set is deliberately all dark. So `light` is exercised through the parser, which
// is the path a user's own theme takes.
func TestParseReadsTheMetadata(t *testing.T) {
	th, err := parse("paper", []byte(
		"light: true\npanel: \"#eeeeee\"\ntext: \"#222222\"\ndir: \"#000000\"\n"+
			"selected: \"#000000\"\nink: \"#ffffff\"\ndim: \"#666666\"\naccent: \"#aa0066\"\n"+
			"size: \"#006644\"\ndanger: \"#aa0022\"\nbar-from: \"#cc3399\"\nbar-to: \"#6644cc\"\n"+
			"bar-track: \"#dddddd\"\n"))
	require.NoError(t, err)
	assert.True(t, th.Light, "a theme that says it is light must come back light")
	assert.False(t, th.Plain)

	mono, ok := Preset("mono")
	require.True(t, ok)
	assert.True(t, mono.Plain)

	charm, ok := Preset("charm")
	require.True(t, ok)
	assert.False(t, charm.Light)
	assert.False(t, charm.Plain)
}

// The bundled set is all dark, and mono covers a light terminal by using no
// colour at all. That is a real gap — the brief asked for a light theme — so it
// is recorded here rather than left to be rediscovered.
func TestNoBundledThemeIsLight(t *testing.T) {
	for _, name := range Names() {
		th, _ := Preset(name)
		assert.False(t, th.Light,
			"%s is light: if a light theme comes back, `cdu themes` must explain the background rule again", name)
	}
}

// midnight is the brief's own second theme — cool dark, deep blue/cyan, calmer
// than charm — and it is the only coloured preset with no pink in it.
func TestMidnightIsTheCoolOne(t *testing.T) {
	th, ok := Preset("midnight")
	require.True(t, ok)
	assert.False(t, th.Light)
	assert.Equal(t, Color("#38bdf8"), th.Accent)
	assert.Equal(t, Color("#67e8f9"), th.BarFrom)
	assert.Equal(t, Color("#2563eb"), th.BarTo)
}
