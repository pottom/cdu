package theme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestResolveDefaultsToCharm(t *testing.T) {
	th, err := Resolve(&Config{}, "")
	require.NoError(t, err)
	assert.Equal(t, Charm(), th)
}

func TestResolvePreferstheFlagOverTheConfigPreset(t *testing.T) {
	th, err := Resolve(&Config{Preset: "ember"}, "catppuccin-latte")
	require.NoError(t, err)
	assert.Equal(t, "catppuccin-latte", th.Name)
}

// The flag chooses a preset; it does not throw away the tokens the user pinned
// by hand, which are their own explicit decisions.
func TestFlagKeepsTheConfigsTokenOverrides(t *testing.T) {
	cfg := Config{Preset: "charm", Theme: Theme{Accent: "#00ff00"}}
	th, err := Resolve(&cfg, "ember")
	require.NoError(t, err)

	assert.Equal(t, "ember", th.Name)
	assert.Equal(t, Color("#00ff00"), th.Accent, "an explicit token survives --theme")
	assert.Equal(t, Ember().Size, th.Size, "the rest comes from the named preset")
}

// A typo must not stop cdu opening — the disk is still full either way. It is
// reported, and the theme falls back.
func TestUnknownThemeFallsBackAndSaysSo(t *testing.T) {
	th, err := Resolve(&Config{}, "mocha")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown theme "mocha"`)
	assert.Contains(t, err.Error(), "catppuccin-mocha", "the error must list what to type instead")
	assert.Equal(t, Charm(), th, "an unusable name falls back to the default, still rendering")
}

// One bad hex must not cost the user the rest of their theme.
func TestOnlyTheMalformedTokenIsDropped(t *testing.T) {
	cfg := Config{Preset: "charm", Theme: Theme{Accent: "not-a-colour", Size: "#00ff00"}}
	th, err := Resolve(&cfg, "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), `accent: "not-a-colour"`)
	assert.Equal(t, Charm().Accent, th.Accent, "the bad token falls back to the preset's")
	assert.Equal(t, Color("#00ff00"), th.Size, "the good token is kept")
	assert.Empty(t, th.Missing(), "a dropped token must inherit, never stay blank")
}

// mono renders through the no-colour path, so tokens set against it do nothing.
// Saying so beats leaving the user to wonder why their accent was ignored.
func TestColoursOnMonoAreReported(t *testing.T) {
	th, err := Resolve(&Config{Preset: "mono", Theme: Theme{Accent: "#00ff00"}}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uses no colour")
	assert.True(t, th.Plain, "mono stays plain regardless")

	// Naming mono without colouring it is not a complaint.
	_, err = Resolve(&Config{Preset: "mono"}, "")
	assert.NoError(t, err)
}

// The config block is what a user actually types, so it is parsed here rather
// than assumed: `preset` plus inlined tokens, in one flat map.
func TestConfigBlockParsesFromYAML(t *testing.T) {
	var cfg Config
	require.NoError(t, yaml.Unmarshal([]byte(
		"preset: catppuccin-mocha\naccent: \"#ff0000\"\nbar-track: \"#111111\"\n"), &cfg))

	assert.Equal(t, "catppuccin-mocha", cfg.Preset)
	assert.Equal(t, Color("#ff0000"), cfg.Accent)
	assert.Equal(t, Color("#111111"), cfg.BarTrack)

	th, err := Resolve(&cfg, "")
	require.NoError(t, err)
	assert.Equal(t, Color("#ff0000"), th.Accent)
	assert.Equal(t, CatppuccinMocha().Text, th.Text)
}

// --write-config marshals the resolved config back out. Name/Light/Plain are not
// colours and must never appear as keys a user could set.
func TestConfigBlockDoesNotWriteMetadata(t *testing.T) {
	out, err := yaml.Marshal(Config{Preset: "ember", Theme: Theme{Accent: "#ff0000"}})
	require.NoError(t, err)

	assert.Contains(t, string(out), "preset: ember")
	// yaml.v3 quotes the hex because # would otherwise start a comment. Which
	// quote it picks is its business; that the value round-trips is ours.
	assert.Contains(t, string(out), "accent: '#ff0000'")
	assert.NotContains(t, string(out), "name:")
	assert.NotContains(t, string(out), "light:")
	assert.NotContains(t, string(out), "plain:")
	assert.NotContains(t, string(out), "panel:", "an unset token must not be written as empty")
}
