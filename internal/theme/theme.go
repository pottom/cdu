// Package theme holds cdu's colour tokens: the Theme struct every style in the
// interface is built from, and the presets bundled into the binary.
//
// A token names a role, not a hue. The charm theme's accent happens to be pink,
// but catppuccin-latte's is mauve and daylight's is not pink at all — a renderer
// reaching for `pink` would be telling the truth in exactly one theme. So the
// tokens are Accent, Danger, Size and so on, and the render path never sees a
// colour literal.
package theme

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// Color is a #rrggbb hex string.
//
// Hex only, deliberately. Lipgloss would also accept an ANSI index like "5", but
// the usage bar blends its endpoints in Luv, and an ANSI index carries no value
// to blend — it would come out black, on one theme, in one place. Constraining
// the token is cheaper than debugging that.
type Color string

var hexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Valid reports whether c is a well-formed hex colour. The empty string is not
// valid but is not an error either: it means "not set", which is what lets a
// user override two tokens of a preset without restating the other nine.
func (c Color) Valid() bool { return hexRe.MatchString(string(c)) }

// Theme is the complete set of colours the interface may use. A bundled preset
// sets every token; a user's config may set any subset, and the rest are
// inherited from the preset it names.
type Theme struct {
	// Name is the preset this came from, for `cdu themes` and error messages. It
	// is not a colour and is never read from the config.
	Name string `yaml:"-"`

	// Light marks a theme drawn for a light terminal.
	//
	// cdu never paints the field: there is no background token, so the terminal's
	// own background shows through, which is what keeps transparency and blur
	// working for the people most likely to care about themes at all. The price is
	// that a light theme on a dark terminal is unreadable, so `cdu themes` says
	// which is which.
	Light bool `yaml:"-"`

	// Plain means the theme uses no colour, rendering through the same
	// bold/reverse/underline path as --no-color.
	//
	// `mono` is defined this way rather than as a set of greys because no fixed
	// grey is legible on both a light and a dark terminal, while the no-colour
	// path is — it conveys state through attributes instead, and it is the path
	// nocolor_test.go already audits. A Plain theme therefore has no tokens, and
	// Missing does not apply to it.
	Plain bool `yaml:"-"`

	// Panel backs the modal and the cursor row.
	Panel Color `yaml:"panel,omitempty"`
	// Text is an ordinary file name and the modal's body.
	Text Color `yaml:"text,omitempty"`
	// Dir is a directory name, which reads brighter than a file.
	Dir Color `yaml:"dir,omitempty"`
	// Selected is the foreground on top of Panel and on filled buttons. It is a
	// token of its own rather than "white" because on a light theme it is dark.
	Selected Color `yaml:"selected,omitempty"`
	// Dim is percentages, key hints, and disabled buttons.
	Dim Color `yaml:"dim,omitempty"`
	// Accent is the cursor marker, the wordmark, the modal border and matched
	// filter runes.
	Accent Color `yaml:"accent,omitempty"`
	// Size is the size column.
	Size Color `yaml:"size,omitempty"`
	// Danger is anything destructive.
	Danger Color `yaml:"danger,omitempty"`

	// BarFrom and BarTo are the usage bar's gradient endpoints. BarFrom doubles as
	// the solid fill below truecolor, where the gradient is not drawn. They are
	// separate from Accent so a theme can run the bar through colours it would not
	// use for a marker.
	BarFrom Color `yaml:"bar-from,omitempty"`
	BarTo   Color `yaml:"bar-to,omitempty"`
	// BarTrack is the unlit part of the bar.
	BarTrack Color `yaml:"bar-track,omitempty"`
}

// tokens returns every colour token keyed by its yaml name, addressable so
// callers can write to it.
//
// Reflection rather than a hand-written list: validation, merging and
// `--write-config` all need to walk the tokens, and a hand-written list would
// let a token added later slip past all three silently. TestEveryTokenIsWalked
// holds the reflection honest.
func (t *Theme) tokens() map[string]*Color {
	out := make(map[string]*Color)
	v := reflect.ValueOf(t).Elem()
	ty := v.Type()
	for i := range ty.NumField() {
		if ty.Field(i).Type != reflect.TypeFor[Color]() {
			continue
		}
		key, _, _ := strings.Cut(ty.Field(i).Tag.Get("yaml"), ",")
		if key == "" || key == "-" {
			continue
		}
		ptr, ok := v.Field(i).Addr().Interface().(*Color)
		if !ok {
			continue
		}
		out[key] = ptr
	}
	return out
}

// TokenNames lists every yaml key a theme block accepts, sorted. `cdu themes`
// and the config writer use it, so the documentation cannot drift from the code.
func TokenNames() []string {
	var t Theme
	names := make([]string, 0, len(t.tokens()))
	for key := range t.tokens() {
		names = append(names, key)
	}
	sort.Strings(names)
	return names
}

// Validate reports every token that is set but malformed. An unset token is not
// an error — it inherits.
//
// The caller warns and falls back rather than exiting: someone with a typo in
// their config wants their disk usage tool to open, not to argue.
func (t *Theme) Validate() error {
	var bad []string
	for key, c := range t.tokens() {
		if *c != "" && !c.Valid() {
			bad = append(bad, fmt.Sprintf("%s: %q", key, *c))
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return fmt.Errorf("not a #rrggbb colour: %s", strings.Join(bad, ", "))
}

// Missing lists the tokens with no value, sorted. A preset with a missing token
// would render that element black-on-black, so the preset test uses this; the
// config loader uses it after merging to prove nothing was left unresolved.
func (t *Theme) Missing() []string {
	var missing []string
	for key, c := range t.tokens() {
		if *c == "" {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
}

// Overlay copies every token set on other over the receiver, leaving the rest
// alone. This is how a user's partial theme block lands on top of a preset.
func (t *Theme) Overlay(other *Theme) {
	dst := t.tokens()
	for key, src := range other.tokens() {
		if *src == "" {
			continue
		}
		if d, ok := dst[key]; ok {
			*d = *src
		}
	}
}
