package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pottom/cdu/charm"
)

// This file is separate from app_test.go on purpose: app_test.go is upstream's
// and is the merge conflict surface, and a new file in the same package never
// conflicts.

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	return home
}

func TestSaveViewWritesTheViewToCdusConfig(t *testing.T) {
	home := withHome(t)
	app := &App{Flags: &Flags{}}

	path, err := app.saveView(charm.ViewSettings{
		ShowApparentSize: true,
		ShowItemCount:    true,
		SortBy:           "mtime",
		SortOrder:        "asc",
	})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu", "cdu.yaml"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var got Flags
	require.NoError(t, yaml.Unmarshal(data, &got))
	assert.True(t, got.ShowApparentSize)
	assert.True(t, got.ShowItemCount)
	assert.False(t, got.ShowMTime)
	assert.False(t, got.ShowRelativeSize)
	assert.Equal(t, "mtime", got.Sorting.By)
	assert.Equal(t, "asc", got.Sorting.Order)
}

// The view is six fields; a config is far more than that. charm cannot see Flags
// and would drop everything it does not know, which is exactly why the write is
// folded in here instead.
func TestSaveViewKeepsTheRestOfTheConfig(t *testing.T) {
	withHome(t)
	app := &App{Flags: &Flags{
		NoHidden:          true,
		IgnoreDirs:        []string{"/proc", "/custom/path"},
		LogFile:           "/tmp/cdu.log",
		MaxCores:          4,
		IgnoreDirPatterns: []string{"^/tmp"},
	}}

	path, err := app.saveView(charm.ViewSettings{ShowMTime: true, SortBy: "size", SortOrder: "desc"})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var got Flags
	require.NoError(t, yaml.Unmarshal(data, &got))

	assert.True(t, got.NoHidden, "a setting charm knows nothing about must survive the save")
	assert.Equal(t, []string{"/proc", "/custom/path"}, got.IgnoreDirs)
	assert.Equal(t, []string{"^/tmp"}, got.IgnoreDirPatterns)
	assert.Equal(t, "/tmp/cdu.log", got.LogFile)
	assert.Equal(t, 4, got.MaxCores)
	assert.True(t, got.ShowMTime, "and the view is still written")
}

// Reading a gdu config is the point of the fallback; writing back over it is
// not. The save goes to cdu's own path, exactly as --write-config does.
func TestSaveViewNeverWritesToTheGduConfig(t *testing.T) {
	home := withHome(t)
	gdu := filepath.Join(home, ".config", "gdu", "gdu.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(gdu), 0o700))
	require.NoError(t, os.WriteFile(gdu, []byte("no-hidden: true\n"), 0o600))

	// CfgFile is what was read — here, gdu's.
	app := &App{Flags: &Flags{CfgFile: gdu, NoHidden: true}}

	path, err := app.saveView(charm.ViewSettings{ShowItemCount: true})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "cdu", "cdu.yaml"), path)

	untouched, err := os.ReadFile(gdu)
	require.NoError(t, err)
	assert.Equal(t, "no-hidden: true\n", string(untouched), "gdu's config must not be rewritten")
}

// The config directory usually does not exist the first time.
func TestSaveViewCreatesTheConfigDirectory(t *testing.T) {
	home := withHome(t)
	app := &App{Flags: &Flags{}}

	_, err := app.saveView(charm.ViewSettings{SortBy: "name", SortOrder: "asc"})
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(home, ".config", "cdu"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
