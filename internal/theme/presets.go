package theme

import "sort"

// Default is the theme cdu opens with when nothing says otherwise.
const Default = "charm"

// presets are the themes built into the binary. Aliases live here too, so
// `--theme catppuccin` resolves without a second lookup table.
var presets = map[string]func() Theme{
	"charm": Charm,

	"catppuccin-latte":     CatppuccinLatte,
	"catppuccin-frappe":    CatppuccinFrappe,
	"catppuccin-macchiato": CatppuccinMacchiato,
	"catppuccin-mocha":     CatppuccinMocha,
	// Mocha is the flavour people mean when they say Catppuccin.
	"catppuccin": CatppuccinMocha,

	"daylight": Daylight,
	"ember":    Ember,
	"mono":     Mono,
}

// Preset returns a bundled theme by name. The bool is false for an unknown name;
// the caller warns and falls back rather than exiting, because a typo in a
// config should not stop a disk usage tool from opening.
func Preset(name string) (Theme, bool) {
	build, ok := presets[name]
	if !ok {
		return Theme{}, false
	}
	return build(), true
}

// Names lists every bundled theme, sorted, for `cdu themes` and for the error
// message on an unknown name.
func Names() []string {
	names := make([]string, 0, len(presets))
	for name := range presets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Charm is the design spec's palette, and the reference every other preset is
// measured against: it is the one the mocks in docs/design were drawn in.
func Charm() Theme {
	return Theme{
		Name:     "charm",
		Panel:    "#241c34",
		Text:     "#cfc6ef",
		Dir:      "#e9e3ff",
		Selected: "#ffffff",
		Dim:      "#7d739e",
		Accent:   "#ff5fd1",
		Size:     "#4ff0c0",
		Danger:   "#ff2fb3",
		BarFrom:  "#ff5fd1",
		BarTo:    "#8b6dff",
		BarTrack: "#2a2140",
	}
}

// The Catppuccin flavours. The hex values are from the project's own
// palette.json (v1.8.0, MIT — see NOTICE); none of them are typed from memory.
//
// The mapping from cdu's tokens onto Catppuccin's roles is the same for all four
// flavours, which is the point of the palette's design — the flavours share role
// names, and the ordering within a ramp is already flipped for the light one.
// So:
//
//	Panel    surface0  — a raised surface above the terminal's base
//	Text     text      — the main foreground
//	Dir      blue      — every catppuccin file listing colours directories blue
//	Selected text      — the cursor row is told apart by its panel, marker and
//	                     weight; there is nothing brighter than `text` to reach
//	                     for that is not also pink, which the accent already is
//	Dim      overlay2  — the muted role with the most contrast against base, in
//	                     both the light and the dark flavours
//	Accent   mauve     — Catppuccin's own default accent
//	Size     green
//	Danger   red
//	Bar      pink → mauve, over a surface1 track

func CatppuccinLatte() Theme {
	return Theme{
		Name:     "catppuccin-latte",
		Light:    true,
		Panel:    "#ccd0da", // surface0
		Text:     "#4c4f69", // text
		Dir:      "#1e66f5", // blue
		Selected: "#4c4f69", // text
		Dim:      "#7c7f93", // overlay2
		Accent:   "#8839ef", // mauve
		Size:     "#40a02b", // green
		Danger:   "#d20f39", // red
		BarFrom:  "#ea76cb", // pink
		BarTo:    "#8839ef", // mauve
		BarTrack: "#bcc0cc", // surface1
	}
}

func CatppuccinFrappe() Theme {
	return Theme{
		Name:     "catppuccin-frappe",
		Panel:    "#414559", // surface0
		Text:     "#c6d0f5", // text
		Dir:      "#8caaee", // blue
		Selected: "#c6d0f5", // text
		Dim:      "#949cbb", // overlay2
		Accent:   "#ca9ee6", // mauve
		Size:     "#a6d189", // green
		Danger:   "#e78284", // red
		BarFrom:  "#f4b8e4", // pink
		BarTo:    "#ca9ee6", // mauve
		BarTrack: "#51576d", // surface1
	}
}

func CatppuccinMacchiato() Theme {
	return Theme{
		Name:     "catppuccin-macchiato",
		Panel:    "#363a4f", // surface0
		Text:     "#cad3f5", // text
		Dir:      "#8aadf4", // blue
		Selected: "#cad3f5", // text
		Dim:      "#939ab7", // overlay2
		Accent:   "#c6a0f6", // mauve
		Size:     "#a6da95", // green
		Danger:   "#ed8796", // red
		BarFrom:  "#f5bde6", // pink
		BarTo:    "#c6a0f6", // mauve
		BarTrack: "#494d64", // surface1
	}
}

func CatppuccinMocha() Theme {
	return Theme{
		Name:     "catppuccin-mocha",
		Panel:    "#313244", // surface0
		Text:     "#cdd6f4", // text
		Dir:      "#89b4fa", // blue
		Selected: "#cdd6f4", // text
		Dim:      "#9399b2", // overlay2
		Accent:   "#cba6f7", // mauve
		Size:     "#a6e3a1", // green
		Danger:   "#f38ba8", // red
		BarFrom:  "#f5c2e7", // pink
		BarTo:    "#cba6f7", // mauve
		BarTrack: "#45475a", // surface1
	}
}

// Daylight is charm's identity on paper: the same pink-to-purple gradient, with
// every token darkened until it carries on a white terminal. The accents are not
// charm's own — #ff5fd1 on white is roughly 2:1 and unreadable — so they are the
// same hues taken to a legible lightness.
func Daylight() Theme {
	return Theme{
		Name:     "daylight",
		Light:    true,
		Panel:    "#ece6f8",
		Text:     "#3b3450",
		Dir:      "#1e1830",
		Selected: "#14101f",
		Dim:      "#7a7191",
		Accent:   "#c0179c",
		Size:     "#0f7a63",
		Danger:   "#c01d5e",
		BarFrom:  "#d63bb0",
		BarTo:    "#6b4fe0",
		BarTrack: "#ded6f0",
	}
}

// Ember is the warm one: the bar burns amber to red, which is the one gradient
// where the colour happens to agree with the meaning — a bar that is nearly full
// ends up red. That agreement is a coincidence and is not relied on. The
// percentage column still carries the number, exactly as under every other
// theme, because on a 256-colour terminal this gradient is a solid amber fill.
func Ember() Theme {
	return Theme{
		Name:     "ember",
		Panel:    "#2b1d16",
		Text:     "#f0d8c4",
		Dir:      "#fff0e2",
		Selected: "#fffaf5",
		Dim:      "#a08472",
		Accent:   "#ff8c42",
		Size:     "#ffd479",
		Danger:   "#ff4d4d",
		BarFrom:  "#ffb347",
		BarTo:    "#e03a3a",
		BarTrack: "#3a251c",
	}
}

// Mono has no tokens on purpose — see Theme.Plain. It renders through the
// no-colour path, so it is the one theme that is legible on any terminal, at any
// colour depth, to any reader.
func Mono() Theme {
	return Theme{Name: "mono", Plain: true}
}
