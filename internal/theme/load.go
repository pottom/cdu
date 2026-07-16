package theme

import (
	"embed"
	"fmt"
	"path"
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

// Preset returns a theme by name. The bool is false for an unknown name; the
// caller warns and falls back rather than exiting, because a typo in a config
// should not stop a disk usage tool from opening.
func Preset(name string) (Theme, bool) {
	th, ok := bundled()[name]
	return th, ok
}

// Names lists every theme, sorted, for `cdu themes` and for the error message on
// an unknown name.
func Names() []string {
	set := bundled()
	names := make([]string, 0, len(set))
	for name := range set {
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
