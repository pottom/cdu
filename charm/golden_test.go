package charm

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/internal/theme"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate the golden screen snapshots")

// Golden snapshots are the exact rendered output of a screen, byte for byte, in
// testdata/*.golden. They catch the regression the property tests cannot: the frame is
// still the right height and no line is too wide, but a colour shifted, a column moved,
// or an element vanished — it no longer looks right.
//
// Regenerate them deliberately: `go test ./charm/ -run TestScreenGolden -update-golden`,
// then read the diff before committing. A golden that changed without you meaning it to
// is the bug the test exists to surface.
//
// The item-info pane is off for these: on it would make the snapshot depend on an
// os.Lstat of a synthetic path. Its rendering is covered by info_test.go instead.
func TestScreenGolden(t *testing.T) {
	// Force truecolor. Without a TTY, Lipgloss emits no escapes and every theme renders
	// identically, so a colour regression would slip through; this bakes the real,
	// deterministic escapes into the snapshot.
	original := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(original) })

	cases := []struct {
		name  string
		theme string
		w, h  int
		setup func(*model)
	}{
		{name: "browse-charm", theme: "charm", w: 120, h: 40},
		{name: "browse-midnight", theme: "midnight", w: 120, h: 40},
		{name: "browse-ember", theme: "ember", w: 120, h: 40},
		{name: "browse-phosphor", theme: "phosphor", w: 120, h: 40},
		{name: "browse-mono", theme: "mono", w: 120, h: 40},
		// The narrow terminal: the layout sheds its columns rather than wrapping.
		{name: "browse-tiny", theme: "charm", w: 44, h: 14},
		// The help screen: two columns of bindings and the detail pane.
		{name: "help", theme: "charm", w: 120, h: 40, setup: func(m *model) { m.scr = screenHelp }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := benchModel(8)
			m.ui.infoOpen = false
			if th, ok := theme.Preset(tc.theme); ok {
				m.applyTheme(&th)
			}
			m.width, m.height = tc.w, tc.h
			if tc.setup != nil {
				tc.setup(m)
			}
			assertGolden(t, tc.name, m.View())
		})
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden for %s — run with -update-golden", name)
	require.Equal(t, string(want), got, "%s render changed; if intended, rerun with -update-golden", name)
}
