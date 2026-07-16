package theme

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Default is the theme cdu opens with when nothing says otherwise.
const Default = "charm"

// The bundled themes are YAML, embedded rather than read from disk: the brief
// asks that they "ship inside the binary and work immediately after install",
// and cdu is a single static binary with no data directory to install into.
//
// They are data rather than Go literals so that a theme can be read, copied and
// diffed by anyone who wants to make their own — which is the whole point of
// shipping them. Copy one to ~/.config/cdu/themes/ and edit it.
//
//go:embed themes/*.yaml
var bundledFS embed.FS

// file is a theme on disk: the colour tokens, plus the two facts about a theme
// that are not colours.
//
// It is separate from Theme rather than being Theme with more yaml tags, because
// these two keys belong in a *theme file* and not in the `theme:` block of a
// config — where they would parse happily and then do nothing.
type file struct {
	// Light marks a theme drawn for a light terminal. cdu never paints the
	// background, so this is a fact about the reader's terminal, not a preference.
	Light bool `yaml:"light,omitempty"`
	// Plain means the theme uses no colour and renders through the --no-color
	// path. See themes/mono.yaml for why that is a theme rather than an absence.
	Plain bool `yaml:"plain,omitempty"`

	Tokens Theme `yaml:",inline"`
}

// bundled is parsed once, on first use rather than at init, so a program that
// never draws anything never pays for it.
var bundled = sync.OnceValue(loadBundled)

// loadBundled parses the embedded themes.
//
// It panics on a malformed one, which is the one place in this package that
// does. A broken embedded theme is not user input — it is a corrupt build, the
// same class of thing as a bad regexp constant, and there is no sensible way to
// carry on without the theme the interface is about to be drawn in.
// TestEveryBundledThemeParses is what makes sure it cannot fire. User themes,
// which are input, never panic: they warn and are skipped.
func loadBundled() map[string]Theme {
	entries, err := bundledFS.ReadDir("themes")
	if err != nil {
		panic(fmt.Sprintf("theme: the bundled themes are missing from the binary: %v", err))
	}

	out := make(map[string]Theme, len(entries))
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".yaml")
		data, err := bundledFS.ReadFile(path.Join("themes", entry.Name()))
		if err != nil {
			panic(fmt.Sprintf("theme: cannot read bundled theme %s: %v", entry.Name(), err))
		}
		th, err := parse(name, data)
		if err != nil {
			panic(fmt.Sprintf("theme: bundled theme %s is broken: %v", entry.Name(), err))
		}
		out[name] = th
	}

	if _, ok := out[Default]; !ok {
		panic("theme: the default theme is missing from the bundle")
	}
	return out
}

// parse turns a theme file into a Theme. The name comes from the caller — for
// both bundled and user themes that is the filename, so there is one source of
// truth for what a theme is called and a file cannot disagree with itself.
func parse(name string, data []byte) (Theme, error) {
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return Theme{}, err
	}

	th := f.Tokens
	th.Name = name
	th.Light = f.Light
	th.Plain = f.Plain

	if err := th.Validate(); err != nil {
		return Theme{}, err
	}
	if th.Plain {
		if missing := th.Missing(); len(missing) < len(TokenNames()) {
			return Theme{}, fmt.Errorf("a plain theme uses no colour, so it must set no colour tokens")
		}
		return th, nil
	}
	if missing := th.Missing(); len(missing) > 0 {
		return Theme{}, fmt.Errorf("missing tokens: %s", strings.Join(missing, ", "))
	}
	return th, nil
}

// userThemes are themes loaded from the user's theme directory. It is written
// once by LoadUserThemes during startup, before anything renders, and read-only
// after — the same shape as the rest of this program's configuration.
var userThemes = map[string]Theme{}

// LoadUserThemes reads themes from dir, where a .yaml (or .yml) file is a theme
// named after itself. A theme of yours with a bundled name replaces it, so you
// can keep `charm` and mean your charm.
//
// Every error is returned and that one file is skipped. This is the opposite of
// the bundled loader's panic, and deliberately: a bundled theme is part of the
// binary and a broken one is a corrupt build, while these are somebody's
// half-finished theme — not a reason to refuse to show them their disk.
//
// A missing directory is not an error. Most people will never make one.
func LoadUserThemes(dir string) []error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return []error{fmt.Errorf("reading %s: %w", dir, err)}
	}

	var problems []error
	for _, entry := range entries {
		name, ok := themeFileName(entry)
		if !ok {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		th, err := parse(name, data)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		th.User = true
		th.source = filepath.Join(dir, entry.Name())
		userThemes[name] = th
	}
	return problems
}

// Dump returns a theme's file, comments and all.
//
// This is what makes shipping themes as files mean anything to someone holding
// only the binary: without it, "copy one and edit it" would mean going to
// GitHub. The comments are most of the value — they say what each token paints
// and why the palette is what it is.
func Dump(name string) ([]byte, error) {
	if th, ok := userThemes[name]; ok {
		// Yours is already a file; hand back what is actually on disk rather than a
		// re-serialisation of it, which would lose the comments you wrote.
		return os.ReadFile(th.source)
	}
	if _, ok := bundled()[name]; !ok {
		return nil, unknownPreset(name)
	}
	return bundledFS.ReadFile(path.Join("themes", name+".yaml"))
}

// themeFileName reports the theme name for a directory entry, and whether it is
// a theme file at all. Both .yaml and .yml are taken: half the world writes one
// and half the other, and a theme that silently never appeared would be a
// miserable thing to debug.
func themeFileName(entry fs.DirEntry) (string, bool) {
	if entry.IsDir() {
		return "", false
	}
	for _, ext := range []string{".yaml", ".yml"} {
		if name, ok := strings.CutSuffix(entry.Name(), ext); ok {
			return name, true
		}
	}
	return "", false
}

// Preset returns a theme by name. The bool is false for an unknown name; the
// caller warns and falls back rather than exiting, because a typo in a config
// should not stop a disk usage tool from opening.
func Preset(name string) (Theme, bool) {
	if th, ok := userThemes[name]; ok {
		return th, true
	}
	th, ok := bundled()[name]
	return th, ok
}

// Names lists every theme, yours included, sorted — for `cdu themes` and for the
// error message on an unknown name.
func Names() []string {
	seen := make(map[string]struct{}, len(bundled())+len(userThemes))
	for name := range bundled() {
		seen[name] = struct{}{}
	}
	for name := range userThemes {
		seen[name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Charm is the default theme. It is the one theme the rest of the program may
// ask for without handling "not found" — loadBundled guarantees it exists.
func Charm() Theme {
	th, _ := Preset(Default)
	return th
}
