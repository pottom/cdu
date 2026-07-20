package charm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pottom/cdu/pkg/fs"
)

// `i` toggles a live item-info pane docked at the foot of the list, under a rule and
// above the footer. The list shrinks to make room; the pane follows the cursor,
// showing the selected item's metadata; `i` again closes it and the list grows back.
// It is not a modal — you keep moving through the list with it open, and each move
// updates what it shows.

// infoPaneLines is the pane's fixed height: a rule, three lines of fields, and a rule,
// so the block is fenced off from the list above and the footer below, and the list
// does not reflow as the cursor moves between items.
const infoPaneLines = 5

// itemStat caches the parts of an item the engine does not carry — the mode and owner,
// which need an os.Lstat. It is cached so View does no I/O: it is refreshed when the
// selection changes (syncInfoStat), not per frame.
type itemStat struct {
	item  fs.Item
	mode  string
	uid   string
	gid   string
	uname string // resolved user name, empty if the id is not in /etc/passwd
	gname string // resolved group name, empty if the id is not in /etc/group
	ok    bool
}

// WithInfoPane sets whether the item-info pane starts open, from the config's `info`
// key (default true). i toggles it at runtime.
func WithInfoPane(on bool) Option {
	return func(ui *UI) { ui.infoOpen = on }
}

// WithInfoSaver supplies the function that persists the pane's on/off to the config.
// The pane is a plain preference, not a per-directory view you try — so unlike the
// columns (t then s), i saves it on the spot, writing only the info key and leaving the
// rest of the view alone.
func WithInfoSaver(save func(on bool) (string, error)) Option {
	return func(ui *UI) { ui.saveInfo = save }
}

// infoSavedMsg is the result of persisting the pane setting; a failure is worth a word,
// a success is silent — flashing "saved" on every toggle would be noise.
type infoSavedMsg struct{ err error }

// infoScreen reports whether the current screen is a list the info pane can attach to.
func (m *model) infoScreen() bool {
	//nolint:exhaustive // the info pane attaches to the lists; every other screen is false
	switch m.scr {
	case screenBrowse, screenTop, screenDup, screenFind, screenQueue:
		return true
	default:
		return false
	}
}

// infoTarget is the item the pane describes: the cursor row on whichever list is
// showing, or nil on the ../ row or an empty list.
func (m *model) infoTarget() fs.Item {
	var it fs.Item
	//nolint:exhaustive // one selector per list screen; every other screen has no target
	switch m.scr {
	case screenTop:
		it = m.selectedTop()
	case screenDup:
		it = m.selectedDup()
	case screenFind:
		it = m.selectedFind()
	case screenQueue:
		it = m.selectedQueue()
	case screenBrowse:
		it = m.selected()
	default:
		return nil
	}
	if it == nil || m.isParentRow(it) {
		return nil
	}
	return it
}

// toggleInfo opens or closes the pane. It is inert where there is nothing to describe,
// so `i` on an empty list or the ../ row does not open a blank pane.
func (m *model) toggleInfo() tea.Cmd {
	if !m.ui.infoOpen && m.infoTarget() == nil {
		return nil
	}
	m.ui.infoOpen = !m.ui.infoOpen
	m.syncInfoStat()
	if m.ui.saveInfo == nil {
		return nil
	}
	// Persist off the render loop: a config write is small, but $HOME can be a network
	// mount — the same rule the view save follows.
	on := m.ui.infoOpen
	save := m.ui.saveInfo
	return func() tea.Msg {
		_, err := save(on)
		return infoSavedMsg{err: err}
	}
}

// ownerField is a numeric id with its name in parentheses, or the number alone when
// the id has no entry in /etc/passwd or /etc/group.
func ownerField(id, name string) string {
	if name == "" {
		return id
	}
	return id + " (" + name + ")"
}

// afterInput runs after a key or click is handled: it refreshes the info pane's cached
// stat so the pane tracks the cursor, then passes the handler's result through.
func (m *model) afterInput(next tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m.syncInfoStat()
	return next, cmd
}

// syncInfoStat refreshes the cached stat when the selected item has changed, so the
// pane follows the cursor. It runs after a key or click, never in View, so the one
// os.Lstat it does stays off the render path.
func (m *model) syncInfoStat() {
	if !m.ui.infoOpen {
		return
	}
	it := m.infoTarget()
	switch {
	case it == nil:
		m.infoStat = itemStat{}
	case it != m.infoStat.item:
		m.infoStat = statItem(it)
	}
}

// statItem gathers the mode and owner an os.Lstat gives, to sit over the engine's own
// size/usage/mtime. A failed stat is not fatal — the pane shows what the engine has and
// marks the rest unavailable.
func statItem(it fs.Item) itemStat {
	s := statPath(it.GetPath())
	s.item = it
	return s
}

// statPath is statItem's I/O half: the mode and, on Unix, the owner of a path. An
// unstatted path comes back with ok false.
func statPath(path string) itemStat {
	fi, err := os.Lstat(path)
	if err != nil {
		return itemStat{}
	}
	s := itemStat{mode: fi.Mode().String(), ok: true}
	s.uid, s.gid, s.uname, s.gname = statOwner(fi)
	return s
}

// infoPaneHeight is the pane's height when it is open on a list with room for it, zero
// otherwise. It never leaves the list fewer than one row.
func (m *model) infoPaneHeight() int {
	if !m.ui.infoOpen || !m.infoScreen() || m.infoTarget() == nil {
		return 0
	}
	// Room is computed from height and chrome directly: visibleLines() subtracts this,
	// so it must not be reached from here.
	if m.height-m.headerHeight()-m.footerHeight() < infoPaneLines+1 {
		return 0
	}
	return infoPaneLines
}

// infoSeg is one styled piece of a pane line; the plain text is measured, the styled
// text is drawn — the same split rows and the footer use, so a line is never cut mid
// escape.
type infoSeg struct {
	text  string
	style lipgloss.Style
}

// infoLine composes segments to exactly width columns: styled when they fit, clipped as
// plain dim text when they do not.
func (m *model) infoLine(segs []infoSeg, width int) string {
	var plain, styled strings.Builder
	for i := range segs {
		plain.WriteString(segs[i].text)
		styled.WriteString(segs[i].style.Render(segs[i].text))
	}
	p := plain.String()
	if lineWidth(p) > width {
		return m.st.dim.Render(clipTo(p, width))
	}
	return styled.String() + spaces(width-lineWidth(p))
}

// infoPane renders the docked pane: a rule, then the selected item's metadata, exactly
// infoPaneHeight lines.
func (m *model) infoPane() string {
	h := m.infoPaneHeight()
	if h < 1 {
		return ""
	}
	w := max(m.width, 1)
	rule := m.st.dim.Render(strings.Repeat("─", w))

	it := m.infoTarget()
	if it == nil {
		return padLines(rule, h)
	}

	dim, val, name := m.st.dim, m.st.size, m.st.accent

	nameText := it.GetName()
	if it.IsDir() {
		nameText += "/"
	}

	const unknown = "—" // stat failed, or no owner on this platform
	mode, owner := unknown, unknown
	if m.infoStat.item == it && m.infoStat.ok {
		mode = m.infoStat.mode
		if m.infoStat.uid != "" {
			owner = "uid " + ownerField(m.infoStat.uid, m.infoStat.uname) +
				"  gid " + ownerField(m.infoStat.gid, m.infoStat.gname)
		}
	}

	usage, size := it.GetUsage(), it.GetSize()

	lines := []string{
		rule,
		m.infoLine([]infoSeg{
			{" " + nameText, name},
			{"   " + filepath.Dir(it.GetPath()), dim},
		}, w),
		m.infoLine([]infoSeg{
			{" mode ", dim}, {mode, val},
			{"   " + owner, dim},
			{"   modified ", dim}, {it.GetMtime().Format("2006-01-02 15:04:05"), val},
		}, w),
		m.infoLine([]infoSeg{
			{" disk ", dim}, {m.ui.formatSize(usage), val}, {fmt.Sprintf(" (%s B)", humanCount(usage)), dim},
			{"   apparent ", dim}, {m.ui.formatSize(size), val}, {fmt.Sprintf(" (%s B)", humanCount(size)), dim},
		}, w),
		rule,
	}
	return padLines(joinLines(lines), h)
}
