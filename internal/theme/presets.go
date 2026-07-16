package theme

// Default is the theme cdu opens with when nothing says otherwise.
const Default = "charm"

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
