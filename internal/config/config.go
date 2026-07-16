// Package config locates cdu's configuration on disk.
//
// It is a package rather than code in cmd/cdu/main.go because main.go is
// upstream's file and our merge conflict surface: every line added there is
// re-merged on every gdu release. main.go calls Resolve and WriteFile, and that
// is the whole of it.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the config cdu reads, and what --write-config writes.
const FileName = "cdu.yaml"

const dirName = "cdu"

// Dir is cdu's configuration directory: $XDG_CONFIG_HOME/cdu, else
// ~/.config/cdu — on every platform.
//
// Deliberately not os.UserConfigDir, which would put it in
// ~/Library/Application Support on macOS and %AppData% on Windows. No terminal
// tool lives there: nvim, gh, bat, ripgrep, starship and gdu itself all use
// ~/.config everywhere, which is where someone will look for themes/, and it
// keeps cdu's config next door to the gdu config it can inherit.
func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, dirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating the home directory: %w", err)
	}
	return filepath.Join(home, ".config", dirName), nil
}

// Path is cdu's own config file.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// ThemeDir is where themes of your own live. A .yaml dropped in here is a theme,
// named after its file.
func ThemeDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "themes"), nil
}

// Resolve returns the config file to read, and a notice to show the user if
// something about the choice is worth knowing.
//
// cdu's own config wins. Failing that it falls back to a gdu config, because a
// fork that ignored the config of the tool it forked would silently drop
// someone's ignore patterns and sorting on first run — and the failure would
// look like cdu being broken rather than like cdu looking elsewhere. The
// fallback is read-only and announced: --write-config is what makes it cdu's,
// and the notice says so.
//
// When neither exists, cdu's own path comes back anyway: nothing reads it, and
// --write-config has somewhere to go.
func Resolve() (path, notice string, err error) {
	own, err := Path()
	if err != nil {
		return "", "", err
	}
	if isFile(own) {
		return own, "", nil
	}

	if legacy, ok := gduPath(); ok {
		return legacy, fmt.Sprintf(
			"reading gdu's config at %s — run `cdu --write-config` to make it cdu's own", legacy), nil
	}
	return own, "", nil
}

// gduPath finds an existing gdu config, in gdu's own search order.
//
// The paths are hardcoded exactly as gdu hardcodes them — gdu does not consult
// XDG_CONFIG_HOME, so neither may this, or cdu would look for gdu's config
// somewhere gdu would never have written it.
func gduPath() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	for _, p := range []string{
		filepath.Join(home, ".config", "gdu", "gdu.yaml"),
		filepath.Join(home, ".gdu.yaml"),
	} {
		if isFile(p) {
			return p, true
		}
	}
	return "", false
}

// WriteFile writes the config, creating its directory.
//
// gdu's --write-config wrote to a dotfile in $HOME, which always existed. cdu's
// path is a directory that may not, and a --write-config that failed with
// "no such file or directory" would be a poor first impression.
func WriteFile(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return os.WriteFile(path, data, 0o600)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
