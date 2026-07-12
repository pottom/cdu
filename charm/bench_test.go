package charm

import (
	"io"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// benchDir builds a synthetic directory of n entries.
func benchDir(n int) *analyze.Dir {
	dir := &analyze.Dir{
		File:      &analyze.File{Name: "bench", Usage: int64(n) * 4096},
		BasePath:  "/",
		ItemCount: int64(n),
	}
	for i := range n {
		dir.AddFile(&analyze.File{
			Name:   "some-reasonably-long-file-name-" + string(rune('a'+i%26)),
			Size:   int64(i) * 1024,
			Usage:  int64(i) * 4096,
			Parent: dir,
		})
	}
	return dir
}

// benchModel builds a browse-screen model holding n synthetic rows.
func benchModel(n int) *model {
	ui := CreateUI(io.Discard, true, false, false, false)
	dir := benchDir(n)

	m := newModel(ui)
	m.topDir = dir
	m.enterDir(dir)
	m.scr = screenBrowse
	m.width, m.height = 120, 40
	m.haveSize = true
	return m
}

// BenchmarkView is the render hot path: it must not scale with directory size.
func BenchmarkView(b *testing.B) {
	for _, n := range []int{100, 10000} {
		b.Run(itoa(n), func(b *testing.B) {
			m := benchModel(n)
			b.ResetTimer()
			for range b.N {
				_ = m.View()
			}
		})
	}
}

// BenchmarkKeyDown is what the user actually feels: one keystroke, one redraw.
func BenchmarkKeyDown(b *testing.B) {
	for _, n := range []int{100, 10000} {
		b.Run(itoa(n), func(b *testing.B) {
			m := benchModel(n)
			key := tea.KeyMsg{Type: tea.KeyDown}
			b.ResetTimer()
			for range b.N {
				next, _ := m.Update(key)
				m = next.(*model)
				_ = m.View()
			}
		})
	}
}

var _ fs.Item = (*analyze.File)(nil)

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
