//go:build unix && !darwin

package trash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The sidecar is not bookkeeping for us — it is what lets the *desktop's* trash
// put the item back. Without it the user sees a file they cannot restore, which
// is precisely the outcome this package exists to prevent.
func TestTrashWritesARestorableSidecar(t *testing.T) {
	home := withTempHome(t)

	original := filepath.Join(home, "work", "notes.txt")
	writeFile(t, original, "x")

	entry, err := MoveToTrash(original)
	require.NoError(t, err)

	require.NotEmpty(t, entry.metaPath, "the spec requires a .trashinfo")
	assert.Equal(t, filepath.Base(entry.TrashPath)+".trashinfo", filepath.Base(entry.metaPath))

	raw, err := os.ReadFile(entry.metaPath)
	require.NoError(t, err)

	info := string(raw)
	assert.True(t, strings.HasPrefix(info, "[Trash Info]\n"))
	assert.Contains(t, info, "Path="+original)
	assert.Contains(t, info, "DeletionDate=")

	// Restoring must take the sidecar with it, or the desktop goes on listing an
	// item that is no longer in the trash.
	require.NoError(t, Restore(entry))
	assert.NoFileExists(t, entry.metaPath)
}

// A name with a space or a percent sign must not produce a sidecar that parses as
// something else.
func TestTrashInfoEscapesThePath(t *testing.T) {
	home := withTempHome(t)

	original := filepath.Join(home, "work", "a b%c.txt")
	writeFile(t, original, "x")

	entry, err := MoveToTrash(original)
	require.NoError(t, err)

	raw, err := os.ReadFile(entry.metaPath)
	require.NoError(t, err)

	assert.Contains(t, string(raw), "a%20b%25c.txt")
}
