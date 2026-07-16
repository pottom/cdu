package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// completeTheme is a valid theme file, for tests that care about something other
// than the tokens.
const completeTheme = `panel: "#111111"
text: "#cccccc"
dir: "#ffffff"
selected: "#ffffff"
ink: "#000000"
dim: "#777777"
accent: "#00ff00"
size: "#00ffff"
danger: "#ff0000"
bar-from: "#00ff00"
bar-to: "#0000ff"
bar-track: "#222222"
`

// userThemes is package state written once at startup. A test that loads into it
// has to put it back, or the next test sees a theme it never asked for.
func withThemeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	t.Cleanup(func() { userThemes = map[string]Theme{} })
	return dir
}

func TestUserThemeIsLoadedAndMarked(t *testing.T) {
	dir := withThemeDir(t, map[string]string{"mine.yaml": completeTheme})

	require.Empty(t, LoadUserThemes(dir))

	th, ok := Preset("mine")
	require.True(t, ok, "a theme in the theme directory must be selectable")
	assert.Equal(t, "mine", th.Name, "the name is the filename")
	assert.Equal(t, Color("#00ff00"), th.Accent)
	assert.True(t, th.User, "it must know it is yours, so `cdu themes` can say so")
	assert.Contains(t, Names(), "mine")
}

// Half the world writes .yml. A theme that silently never appeared would be a
// miserable thing to debug.
func TestBothYamlAndYmlAreThemes(t *testing.T) {
	dir := withThemeDir(t, map[string]string{
		"long.yaml": completeTheme,
		"short.yml": completeTheme,
		"notes.txt": "not a theme",
		"README.md": "not a theme either",
		"noext":     completeTheme,
	})

	require.Empty(t, LoadUserThemes(dir))

	_, ok := Preset("long")
	assert.True(t, ok)
	_, ok = Preset("short")
	assert.True(t, ok)
	_, ok = Preset("notes")
	assert.False(t, ok, "a .txt is not a theme")
	_, ok = Preset("noext")
	assert.False(t, ok, "a file with no extension is not a theme")
}

// You can keep `charm` and mean your charm.
func TestAUserThemeReplacesABundledOne(t *testing.T) {
	dir := withThemeDir(t, map[string]string{"charm.yaml": completeTheme})

	require.Empty(t, LoadUserThemes(dir))

	th, ok := Preset("charm")
	require.True(t, ok)
	assert.Equal(t, Color("#00ff00"), th.Accent, "yours wins")
	assert.True(t, th.User)

	// And it is still one entry in the listing, not two.
	var count int
	for _, n := range Names() {
		if n == "charm" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

// The whole rule for user themes: broken input is reported and skipped. One bad
// file cannot stop cdu opening, and cannot hide the good ones next to it.
func TestABrokenUserThemeIsReportedAndSkipped(t *testing.T) {
	dir := withThemeDir(t, map[string]string{
		"good.yaml":    completeTheme,
		"partial.yaml": "accent: \"#ff0000\"\n",
		"badhex.yaml":  strings.Replace(completeTheme, "#00ff00", "not-a-colour", 1),
		"garbage.yaml": "\tnot: valid: yaml:\n",
	})

	problems := LoadUserThemes(dir)
	assert.Len(t, problems, 3, "each broken file is reported once")

	joined := strings.Join(errStrings(problems), "\n")
	assert.Contains(t, joined, "partial.yaml", "the report must name the file")
	assert.Contains(t, joined, "missing tokens")
	assert.Contains(t, joined, "badhex.yaml")
	assert.Contains(t, joined, "garbage.yaml")

	_, ok := Preset("good")
	assert.True(t, ok, "a good theme next to a broken one must still load")
	for _, name := range []string{"partial", "badhex", "garbage"} {
		_, ok := Preset(name)
		assert.False(t, ok, "%s must not be selectable", name)
	}
}

// Most people will never make a theme directory. Its absence is not a problem to
// report at them on every run.
func TestAMissingThemeDirectoryIsNotAProblem(t *testing.T) {
	t.Cleanup(func() { userThemes = map[string]Theme{} })
	assert.Empty(t, LoadUserThemes(filepath.Join(t.TempDir(), "nope")))
	assert.Equal(t, []string{"charm", "ember", "midnight", "mono", "phosphor"}, Names())
}

// A theme of your own goes through the same parser as a bundled one, so it can
// be plain or light too.
func TestAUserThemeCanBePlainOrLight(t *testing.T) {
	dir := withThemeDir(t, map[string]string{
		"quiet.yaml": "plain: true\n",
		"paper.yaml": "light: true\n" + completeTheme,
	})

	require.Empty(t, LoadUserThemes(dir))

	quiet, ok := Preset("quiet")
	require.True(t, ok)
	assert.True(t, quiet.Plain)

	paper, ok := Preset("paper")
	require.True(t, ok)
	assert.True(t, paper.Light)
}

// The listing is where you find out whether the file you just wrote loaded.
func TestListMarksYourThemesAndNamesTheDirectory(t *testing.T) {
	dir := withThemeDir(t, map[string]string{"mine.yaml": completeTheme})
	require.Empty(t, LoadUserThemes(dir))

	var b strings.Builder
	require.NoError(t, List(&b, "mine", dir))
	out := b.String()

	assert.Contains(t, out, "(yours)", "your theme must be marked as yours")
	assert.Contains(t, out, "* mine")
	assert.Contains(t, out, dir, "the listing must say where to put one")
	assert.NotContains(t, out, "There are none there yet")
}

func TestListSaysWhereToPutAThemeWhenYouHaveNone(t *testing.T) {
	var b strings.Builder
	require.NoError(t, List(&b, "charm", "/somewhere/themes"))
	out := b.String()

	assert.Contains(t, out, "/somewhere/themes")
	assert.Contains(t, out, "There are none there yet")
	assert.NotContains(t, out, "(yours)")
}

func errStrings(errs []error) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, err.Error())
	}
	return out
}
