//go:build unix

package trash

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests point the trash at a temporary HOME rather than the real one. A test
// suite that fills the developer's actual trash is a test suite nobody runs twice.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return home
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestMoveToTrashAndRestore(t *testing.T) {
	home := withTempHome(t)

	original := filepath.Join(home, "work", "notes.txt")
	writeFile(t, original, "important")

	entry, err := MoveToTrash(original)
	require.NoError(t, err)

	assert.NoFileExists(t, original, "the item must be gone from where it was")
	assert.FileExists(t, entry.TrashPath, "the item must exist in the trash")
	assert.Equal(t, original, entry.OriginalPath)

	require.NoError(t, Restore(entry))

	assert.FileExists(t, original, "restore must put the item back where it came from")
	assert.NoFileExists(t, entry.TrashPath, "restore must take the item out of the trash")

	content, err := os.ReadFile(original)
	require.NoError(t, err)
	assert.Equal(t, "important", string(content), "the content must survive the round trip")
}

func TestMoveToTrashHandlesDirectories(t *testing.T) {
	home := withTempHome(t)

	dir := filepath.Join(home, "work", "node_modules")
	writeFile(t, filepath.Join(dir, "deep", "nested", "file.js"), "x")

	entry, err := MoveToTrash(dir)
	require.NoError(t, err)
	assert.NoDirExists(t, dir)

	require.NoError(t, Restore(entry))
	assert.FileExists(t, filepath.Join(dir, "deep", "nested", "file.js"),
		"a directory must come back with its contents")
}

// Two files deleted from different directories routinely share a name. The second
// must not silently destroy the first inside the trash.
func TestTrashDoesNotOverwriteAnItemAlreadyInIt(t *testing.T) {
	home := withTempHome(t)

	first := filepath.Join(home, "a", "config.yaml")
	second := filepath.Join(home, "b", "config.yaml")
	writeFile(t, first, "first")
	writeFile(t, second, "second")

	firstEntry, err := MoveToTrash(first)
	require.NoError(t, err)
	secondEntry, err := MoveToTrash(second)
	require.NoError(t, err)

	require.NotEqual(t, firstEntry.TrashPath, secondEntry.TrashPath,
		"the second item must get its own name in the trash")

	for path, want := range map[string]string{
		firstEntry.TrashPath:  "first",
		secondEntry.TrashPath: "second",
	} {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, want, string(content), "%s holds the wrong file", path)
	}
}

// An undo that destroys data is not an undo. If something has taken the name back
// in the meantime, restoring must refuse rather than overwrite it.
func TestRestoreRefusesToOverwrite(t *testing.T) {
	home := withTempHome(t)

	original := filepath.Join(home, "work", "notes.txt")
	writeFile(t, original, "old")

	entry, err := MoveToTrash(original)
	require.NoError(t, err)

	writeFile(t, original, "new")

	require.Error(t, Restore(entry), "restore must not overwrite what is there now")

	content, err := os.ReadFile(original)
	require.NoError(t, err)
	assert.Equal(t, "new", string(content), "the newer file must survive")
}

func TestMoveToTrashReportsAMissingItem(t *testing.T) {
	home := withTempHome(t)
	_, err := MoveToTrash(filepath.Join(home, "does-not-exist"))
	assert.Error(t, err)
}
