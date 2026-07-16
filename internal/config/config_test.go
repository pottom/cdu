package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withHome points both the home directory and XDG at a temporary tree, so no
// test can read or write the real one.
func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

func write(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func TestDirIsUnderDotConfigOnEveryPlatform(t *testing.T) {
	home := withHome(t)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu"), dir,
		"a terminal tool lives in ~/.config, not in os.UserConfigDir")

	path, err := Path()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu", "cdu.yaml"), path)

	themes, err := ThemeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu", "themes"), themes)
}

func TestXdgConfigHomeWins(t *testing.T) {
	withHome(t)
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)

	dir, err := Dir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(xdg, "cdu"), dir)
}

func TestResolvePrefersCdusOwnConfig(t *testing.T) {
	home := withHome(t)
	own := filepath.Join(home, ".config", "cdu", "cdu.yaml")
	write(t, own, "no-hidden: true\n")
	write(t, filepath.Join(home, ".config", "gdu", "gdu.yaml"), "no-hidden: false\n")

	path, notice, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, own, path)
	assert.Empty(t, notice, "there is nothing to remark on when cdu has its own config")
}

// A fork that ignored the config of the tool it forked would silently drop
// someone's ignore patterns on first run, and it would look like cdu being
// broken rather than like cdu looking elsewhere.
func TestResolveFallsBackToGdusConfigAndSaysSo(t *testing.T) {
	home := withHome(t)
	gdu := filepath.Join(home, ".config", "gdu", "gdu.yaml")
	write(t, gdu, "no-hidden: true\n")

	path, notice, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, gdu, path)
	assert.Contains(t, notice, gdu, "the notice must name the file being read")
	assert.Contains(t, notice, "--write-config", "and how to take it over")
}

// gdu's own second location. It is checked after the first, as gdu checks it.
func TestResolveFindsTheGduDotfile(t *testing.T) {
	home := withHome(t)
	dotfile := filepath.Join(home, ".gdu.yaml")
	write(t, dotfile, "no-hidden: true\n")

	path, notice, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, dotfile, path)
	assert.NotEmpty(t, notice)

	// The XDG-style location wins when both exist, matching gdu's search order.
	preferred := filepath.Join(home, ".config", "gdu", "gdu.yaml")
	write(t, preferred, "no-hidden: true\n")
	path, _, err = Resolve()
	require.NoError(t, err)
	assert.Equal(t, preferred, path)
}

// gdu hardcodes ~/.config/gdu and never consults XDG_CONFIG_HOME. Following XDG
// here would make cdu look for gdu's config somewhere gdu would never have
// written it, and the fallback would silently never fire.
func TestGduConfigIsFoundWhereGduActuallyPutsIt(t *testing.T) {
	home := withHome(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	gdu := filepath.Join(home, ".config", "gdu", "gdu.yaml")
	write(t, gdu, "no-hidden: true\n")

	path, notice, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, gdu, path)
	assert.NotEmpty(t, notice)
}

// With no config anywhere, Resolve still names cdu's own path: nothing reads it,
// and --write-config needs somewhere to go.
func TestResolveNamesCdusPathWhenThereIsNoConfigAtAll(t *testing.T) {
	home := withHome(t)

	path, notice, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu", "cdu.yaml"), path)
	assert.Empty(t, notice)
}

// A directory named cdu.yaml is not a config. Stat alone would have said yes.
func TestADirectoryIsNotAConfigFile(t *testing.T) {
	home := withHome(t)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "cdu", "cdu.yaml"), 0o700))

	path, _, err := Resolve()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu", "cdu.yaml"), path,
		"it falls through to the write path rather than treating a directory as a config")
}

// gdu wrote to a dotfile in $HOME, which always exists. cdu writes into a
// directory that may not, and --write-config failing with "no such file or
// directory" would be a poor first impression.
func TestWriteFileCreatesTheDirectory(t *testing.T) {
	home := withHome(t)
	path := filepath.Join(home, ".config", "cdu", "cdu.yaml")

	require.NoError(t, WriteFile(path, []byte("no-hidden: true\n")))

	body, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "no-hidden: true\n", string(body))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "a config may hold paths worth not sharing")

	// Writing twice replaces rather than appends.
	require.NoError(t, WriteFile(path, []byte("no-hidden: false\n")))
	body, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "no-hidden: false\n", string(body))
}
