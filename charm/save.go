package charm

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/pkg/fs"
)

// Saving the view is `s` inside the `t` menu, and it is explicit on purpose.
//
// Saving on exit is the obvious alternative and it is worse: someone who turns
// the mtime column on to answer one question would find it on forever, and would
// have no idea what turned it on. A view is a thing you try; a config is a thing
// you decide. This is the key that turns one into the other.
//
// It is `s` inside `t`, not at the top level — `s` there is sort, and the column
// menu is where the settings being saved live.

// ViewSettings is what gets written: the toggles and the sort, which are the
// only things the interface can change that the config already has names for.
//
// It is a struct of plain types rather than the Flags struct because charm
// cannot see that — cmd/cdu/app imports charm, not the other way round. That is
// also why the write goes through a callback: charm does not know the rest of
// the config, and a writer that only knew the fields it owns would silently drop
// everything else in the file.
type ViewSettings struct {
	ShowApparentSize bool
	ShowRelativeSize bool
	ShowItemCount    bool
	ShowMTime        bool
	FoldersFirst     bool
	InfoPane         bool
	// ThemeName is the picked preset, written to the config's theme.preset. Empty
	// leaves whatever was there — an interface that never opened the picker must not
	// overwrite a theme the user set by hand.
	ThemeName string
	// SortBy and SortOrder are the yaml spellings, the same ones fs.ParseSortBy
	// and fs.ParseSortOrder read back.
	SortBy    string
	SortOrder string
}

// WithConfigSaver supplies the function that writes the view to the config. It
// returns the path written, which the status line reports — a save that does not
// say where it went is a save you have to go and verify.
//
// Without it, `s` says saving is unavailable rather than doing nothing.
func WithConfigSaver(save func(ViewSettings) (string, error)) Option {
	return func(ui *UI) { ui.save = save }
}

// viewSettings is the current view, in the config's vocabulary.
func (ui *UI) viewSettings() ViewSettings {
	return ViewSettings{
		ShowApparentSize: ui.ShowApparentSize,
		ShowRelativeSize: ui.ShowRelativeSize,
		ShowItemCount:    ui.showItemCount,
		ShowMTime:        ui.showMtime,
		FoldersFirst:     ui.foldersFirst,
		InfoPane:         ui.infoOpen,
		ThemeName:        ui.theme.Name,
		SortBy:           sortByYAML(ui.sortBy),
		SortOrder:        sortOrderYAML(ui.sortOrder),
	}
}

// sortByYAML is the config spelling of a sort field, from the set ParseSortBy
// reads back.
//
// SortByApparentSize is written as "size". ParseSortBy has no name for it, and
// it needs none: the field is implied by show-apparent-size, which is saved
// beside it, and CreateUI re-derives the pair on the way back in. Inventing a
// name here would produce a config that round-trips to the wrong field.
// The config's vocabulary for sorting. yamlSortSize doubles as the fallback for
// anything unrecognised, matching fs.ParseSortBy, which also defaults to size.
const (
	yamlSortSize = "size"
	yamlAsc      = "asc"
	yamlDesc     = "desc"
)

func sortByYAML(by fs.SortBy) string {
	switch by {
	case fs.SortByName:
		return "name"
	case fs.SortByItemCount:
		return "itemCount"
	case fs.SortByMtime:
		return "mtime"
	case fs.SortBySize, fs.SortByApparentSize:
		return yamlSortSize
	}
	return yamlSortSize
}

func sortOrderYAML(order fs.SortOrder) string {
	if order == fs.SortAsc {
		return yamlAsc
	}
	return yamlDesc
}

type viewSavedMsg struct {
	path string
	err  error
}

// saveViewCmd writes off the render loop. A config write is small, but $HOME can
// be a network mount, and the rule here is that anything which can block is a
// command rather than a keystroke that hangs the interface.
func saveViewCmd(save func(ViewSettings) (string, error), v ViewSettings) tea.Cmd {
	return func() tea.Msg {
		path, err := save(v)
		return viewSavedMsg{path: path, err: err}
	}
}

func (m *model) saveView() tea.Cmd {
	if m.ui.save == nil {
		m.status, m.statusIsError = "saving is not available in this build", true
		return nil
	}
	m.status, m.statusIsError = "saving…", false
	return saveViewCmd(m.ui.save, m.ui.viewSettings())
}

func (m *model) applyViewSaved(msg viewSavedMsg) {
	if msg.err != nil {
		m.status, m.statusIsError = "could not save: "+msg.err.Error(), true
		return
	}
	m.status, m.statusIsError = fmt.Sprintf("view saved to %s", msg.path), false
}
