package charm

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/fs"
)

// saveModel wires a recording saver in place of the real config writer.
func saveModel(t *testing.T, save func(ViewSettings) (string, error)) *model {
	t.Helper()
	ui := CreateUI(nil, true, false, false, false)
	if save != nil {
		WithConfigSaver(save)(ui)
	}
	m := benchModel(3)
	m.ui.save = ui.save
	return m
}

// The whole feature: t, then s, writes what is on screen.
func TestSavingTheViewFromTheColumnMenu(t *testing.T) {
	var got ViewSettings
	var calls int
	m := saveModel(t, func(v ViewSettings) (string, error) {
		calls, got = calls+1, v
		return "/home/x/.config/cdu/cdu.yaml", nil
	})

	m.ui.ShowApparentSize = true
	m.ui.ShowRelativeSize = true
	m.ui.showItemCount = true
	m.ui.showMtime = false
	m.ui.sortBy, m.ui.sortOrder = fs.SortByMtime, fs.SortAsc

	m.colPending = true
	cmd := m.handleColumnKey("s")
	require.NotNil(t, cmd, "saving must return a command — a config write can block")
	assert.False(t, m.colPending, "the menu closes")

	msg := cmd().(viewSavedMsg)
	assert.Equal(t, 1, calls)
	assert.Equal(t, ViewSettings{
		ShowApparentSize: true,
		ShowRelativeSize: true,
		ShowItemCount:    true,
		ShowMTime:        false,
		InfoPane:         true,
		ThemeName:        "charm",
		SortBy:           "mtime",
		SortOrder:        "asc",
	}, got, "what is written is what is on screen")

	m.applyViewSaved(msg)
	assert.False(t, m.statusIsError)
	assert.Contains(t, m.status, "/home/x/.config/cdu/cdu.yaml",
		"a save that does not say where it went is a save you have to go and verify")
}

// A write can block — $HOME can be a network mount — so it is a command, not a
// keystroke that freezes the interface.
func TestSavingRunsOffTheRenderLoop(t *testing.T) {
	m := saveModel(t, func(ViewSettings) (string, error) { return "/tmp/cdu.yaml", nil })
	m.colPending = true

	_, cmd := m.Update(key("s"))
	require.NotNil(t, cmd)
	assert.IsType(t, viewSavedMsg{}, cmd(), "the result must arrive as a message")
}

func TestAFailedSaveIsReportedNotSwallowed(t *testing.T) {
	m := saveModel(t, func(ViewSettings) (string, error) {
		return "", errors.New("read-only file system")
	})
	m.colPending = true

	cmd := m.handleColumnKey("s")
	require.NotNil(t, cmd)
	m.applyViewSaved(cmd().(viewSavedMsg))

	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "read-only file system")
}

// A key that silently does nothing reads as a broken interface — the same rule
// --no-delete follows.
func TestSavingWithoutASaverSaysSo(t *testing.T) {
	m := saveModel(t, nil)
	m.colPending = true

	cmd := m.handleColumnKey("s")
	assert.Nil(t, cmd)
	assert.True(t, m.statusIsError)
	assert.Contains(t, m.status, "not available")
}

// s is the save; it must not also be read as a column toggle, and the other keys
// must keep working.
func TestSaveDoesNotDisturbTheColumnKeys(t *testing.T) {
	var saved bool
	m := saveModel(t, func(ViewSettings) (string, error) { saved = true; return "/tmp/x", nil })

	before := m.ui.showMtime
	m.colPending = true
	assert.Nil(t, m.handleColumnKey("m"), "m still toggles mtime")
	assert.Equal(t, !before, m.ui.showMtime)
	assert.False(t, saved, "toggling must not write anything")

	m.colPending = true
	cmd := m.handleColumnKey("s")
	require.NotNil(t, cmd)
	assert.False(t, saved, "the key returns the write; it does not perform it")
	cmd()
	assert.True(t, saved)
}

// The sort field has to round-trip through the config's vocabulary, or the view
// saved is not the view restored.
func TestSortNamesRoundTripThroughTheConfig(t *testing.T) {
	for _, tc := range []struct {
		by   fs.SortBy
		want string
	}{
		{fs.SortByName, "name"},
		{fs.SortBySize, "size"},
		{fs.SortByItemCount, "itemCount"},
		{fs.SortByMtime, "mtime"},
	} {
		assert.Equal(t, tc.want, sortByYAML(tc.by))
		assert.Equal(t, tc.by, fs.ParseSortBy(tc.want), "%s must parse back to itself", tc.want)
	}

	// Apparent size has no name of its own in the config, and needs none: it is
	// implied by show-apparent-size, which is saved beside it. Writing anything
	// else would round-trip to the wrong field.
	assert.Equal(t, "size", sortByYAML(fs.SortByApparentSize))

	assert.Equal(t, "asc", sortOrderYAML(fs.SortAsc))
	assert.Equal(t, "desc", sortOrderYAML(fs.SortDesc))
	assert.Equal(t, fs.SortAsc, fs.ParseSortOrder("asc"))
	assert.Equal(t, fs.SortDesc, fs.ParseSortOrder("desc"))
}

// The pair a saved config restores must be the pair handleToggle would have
// produced: sorting by size means the size on screen. Without this, a config
// with show-apparent-size and sorting.by: size opens ordered by disk usage while
// showing apparent size — the inconsistency the toggle carefully avoids,
// arriving through the front door.
func TestApparentSizeAndSizeSortAreReconciledAtStartup(t *testing.T) {
	ui := CreateUI(nil, true, true /* showApparentSize */, false, false,
		func(ui *UI) { ui.SetDefaultSorting("size", "desc") })
	assert.Equal(t, fs.SortByApparentSize, ui.sortBy,
		"a size sort must follow the column that is actually shown")

	// And it must not reach for apparent size when the column is off.
	ui = CreateUI(nil, true, false, false, false,
		func(ui *UI) { ui.SetDefaultSorting("size", "desc") })
	assert.Equal(t, fs.SortBySize, ui.sortBy)

	// Other fields are left alone.
	ui = CreateUI(nil, true, true, false, false,
		func(ui *UI) { ui.SetDefaultSorting("name", "asc") })
	assert.Equal(t, fs.SortByName, ui.sortBy)
}

// The save has to be visible in the menu that offers it.
func TestTheColumnMenuAdvertisesTheSave(t *testing.T) {
	m := benchModel(3)
	m.width, m.height, m.haveSize = 120, 20, true
	m.colPending = true

	assert.Contains(t, m.viewFooter(), "save view")
}

var _ = tea.Cmd(nil)
