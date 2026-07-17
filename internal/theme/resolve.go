package theme

import (
	"errors"
	"fmt"
	"strings"
)

// Config is the `theme:` block of cdu's config file, and the shape --theme
// writes into. The tokens are inlined, so a user names a preset and then
// overrides the two colours they disagree with:
//
//	theme:
//	  preset: midnight
//	  accent: "#ff5fd1"
type Config struct {
	Preset string `yaml:"preset,omitempty"`
	Theme  `yaml:",inline"`
}

// Resolve produces the theme to render with. It always produces one.
//
// A bad preset name or a malformed hex is reported and the offending part falls
// back — never an exit. Someone who typo'd a colour in a config file opened cdu
// because a disk is full; refusing to start would be answering a question they
// did not ask. The caller prints the error to stderr and renders anyway.
//
// --no-color and NO_COLOR are not handled here: they already drive the plain
// render path, which is precisely what mono is, so forcing the theme as well
// would be a second way of saying the same thing.
func Resolve(cfg *Config, flag string) (Theme, error) {
	var problems []error

	name := cfg.Preset
	if flag != "" {
		// The flag chooses the preset only. Token overrides in the config are the
		// user's own explicit decisions, and survive a --theme on the command line.
		name = flag
	}
	if name == "" {
		name = Default
	}

	th, ok := Preset(name)
	if !ok {
		problems = append(problems, unknownPreset(name))
		th, _ = Preset(Default)
	}

	overrides := cfg.Theme
	if err := overrides.Validate(); err != nil {
		problems = append(problems, err)
		// Drop only the tokens that are wrong. The rest of the user's theme is
		// perfectly good and there is no reason to punish it for a neighbour.
		overrides.dropInvalid()
	}

	if th.Plain && len(overrides.Missing()) < len(TokenNames()) {
		problems = append(problems, fmt.Errorf(
			"theme %q uses no colour, so its colour tokens are ignored", th.Name))
	}

	th.Overlay(&overrides)
	return th, errors.Join(problems...)
}

// dropInvalid blanks every token that is set but malformed, so it inherits the
// preset's value instead of handing garbage to the terminal.
func (t *Theme) dropInvalid() {
	for _, c := range t.tokens() {
		if *c != "" && !c.Valid() {
			*c = ""
		}
	}
}

// unknownPreset names the alternatives. A user who typed `--theme midnite` wants
// to be shown `midnight`, not left guessing why nothing changed.
func unknownPreset(name string) error {
	return fmt.Errorf("unknown theme %q; available: %s", name, strings.Join(Names(), ", "))
}
