package app

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/pottom/cdu/charm"
	"github.com/pottom/cdu/internal/config"
)

// saveView writes the interface's current view to cdu's config, for `t` then `s`.
//
// It lives in its own file rather than in app.go because app.go is upstream's
// and is the merge conflict surface; a new file in the same package never
// conflicts.
//
// It folds the view into the whole Flags struct and writes that, so everything
// else in the config survives. charm cannot do this itself — it cannot see
// Flags, and a writer that knew only the six fields it owns would quietly drop
// the rest of the file.
func (a *App) saveView(v charm.ViewSettings) (string, error) {
	a.Flags.ShowApparentSize = v.ShowApparentSize
	a.Flags.ShowRelativeSize = v.ShowRelativeSize
	a.Flags.ShowItemCount = v.ShowItemCount
	a.Flags.ShowMTime = v.ShowMTime
	a.Flags.FoldersFirst = v.FoldersFirst
	if v.ThemeName != "" {
		// The preset only — any token overrides the user hand-wrote stay, the way
		// --theme leaves them, so saving a picked theme keeps their tweaks.
		a.Flags.Theme.Preset = v.ThemeName
	}
	a.Flags.Sorting.By = v.SortBy
	a.Flags.Sorting.Order = v.SortOrder

	// Always cdu's own path, never the file that was read: on a gdu config,
	// reading it is the point and writing back over it is not. This is the same
	// split --write-config makes.
	path, err := config.Path()
	if err != nil {
		return "", err
	}

	data, err := yaml.Marshal(a.Flags)
	if err != nil {
		return "", fmt.Errorf("building the config: %w", err)
	}
	if err := config.WriteFile(path, data); err != nil {
		return "", err
	}
	return path, nil
}

// saveInfo persists just the item-info pane's on/off, for i.
//
// Unlike saveView it does not fold a whole ViewSettings in: it sets Info alone and
// writes the Flags as they stand, so toggling the pane never commits an uncommitted
// column try. Same cdu-owned path and whole-file write as saveView, so nothing else in
// the config is lost.
func (a *App) saveInfo(on bool) (string, error) {
	a.Flags.Info = on

	path, err := config.Path()
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(a.Flags)
	if err != nil {
		return "", fmt.Errorf("building the config: %w", err)
	}
	if err := config.WriteFile(path, data); err != nil {
		return "", err
	}
	return path, nil
}
