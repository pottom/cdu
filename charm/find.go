package charm

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/pottom/cdu/pkg/fs"
)

// `f` finds files by name anywhere in the current subtree and lists the matches,
// so `*.mkv` turns up every film wherever it sits.
//
// It is the tree-wide counterpart to `/`. The two are deliberately different
// tools with different names: `/` filters — a fuzzy, live narrowing of the one
// directory you are looking at — while `f` finds — an exact match over the
// directory and everything under it, opening a results list. `/` finding nothing
// for `*.mkv` was the whole reason this exists: it only ever saw a directory's
// direct children, and treated `*` as a literal.
//
// The walk reads only names, never contents, so it runs on the render loop like
// T rather than in a goroutine like the duplicate search.

// matchName reports whether a filename matches the find pattern.
//
// A pattern with a wildcard is a glob; anything else is a case-insensitive
// substring, so bare "mkv" finds every .mkv without the user having to spell out
// the glob. Both are matched case-insensitively — a search that missed IMG_1.JPG
// for "img" would feel broken.
func matchName(name, pattern string) bool {
	if pattern == "" {
		return false
	}
	name = strings.ToLower(name)
	pattern = strings.ToLower(pattern)
	if strings.ContainsAny(pattern, "*?[") {
		ok, err := filepath.Match(pattern, name)
		return err == nil && ok
	}
	return strings.Contains(name, pattern)
}

// findMatches walks the subtree for files whose name matches, biggest first —
// this is a disk usage tool, so the largest match is the one you most likely
// came for.
func findMatches(root fs.Item, pattern string) fs.Files {
	var out fs.Files
	var walk func(fs.Item)
	walk = func(item fs.Item) {
		if item.IsDir() {
			for child := range item.GetFiles(fs.SortByName, fs.SortAsc) {
				walk(child)
			}
			return
		}
		if matchName(item.GetName(), pattern) {
			out = append(out, item)
		}
	}
	walk(root)
	sort.Sort(sort.Reverse(fs.ByApparentSize(out)))
	return out
}

// openFind starts the f input. Unlike the filter it does not narrow anything as
// you type — the results are a whole subtree away, not the rows on screen — so it
// is a prompt that runs on enter.
func (m *model) openFind() {
	m.finding = true
	m.findQuery = ""
	m.status, m.statusIsError = "", false
}

func (m *model) closeFind() {
	m.finding = false
	m.findQuery = ""
}

// handleFindInputKey drives the f prompt. Like the filter and the menus it takes
// every key: while typing a pattern, q is a letter.
func (m *model) handleFindInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape:
		m.closeFind()
		return m, nil
	case keyEnter:
		return m.runFind()
	case keyBackspace:
		if m.findQuery != "" {
			m.findQuery = m.findQuery[:len(m.findQuery)-1]
		} else {
			m.closeFind()
		}
		return m, nil
	}
	if len(msg.Runes) == 1 {
		m.findQuery += string(msg.Runes)
	}
	return m, nil
}

// runFind performs the search and opens the results, or says nothing matched.
func (m *model) runFind() (tea.Model, tea.Cmd) {
	pattern := strings.TrimSpace(m.findQuery)
	root := m.searchRoot()
	m.finding = false
	if pattern == "" || root == nil {
		m.findQuery = ""
		return m, nil
	}

	m.findPattern = pattern
	m.findResults = findMatches(root, pattern)
	m.findQuery = ""
	m.findCursor, m.findOffset = 0, 0

	if len(m.findResults) == 0 {
		m.status, m.statusIsError = fmt.Sprintf("nothing matching %q%s", pattern, m.searchScopeWord()), false
		return m, nil
	}
	m.scr = screenFind
	return m, nil
}

// searchScopeWord names the subtree a find covered — the full path,
// home-shortened — for the header title and the "nothing matched" message.
func (m *model) searchScopeWord() string {
	root := m.searchRoot()
	if root == nil {
		return ""
	}
	return " under " + m.shortPath(root.GetPath())
}

func (m *model) selectedFind() fs.Item {
	if m.findCursor < 0 || m.findCursor >= len(m.findResults) {
		return nil
	}
	return m.findResults[m.findCursor]
}

func (m *model) handleFindKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case keyEscape, keyLeft, "h":
		m.scr = screenBrowse
		return m, nil
	case keyUp, "k":
		m.moveFindCursor(-1)
	case keyDown, "j":
		m.moveFindCursor(1)
	case keyHome, "g":
		m.moveFindCursor(-len(m.findResults))
	case keyEnd, "G":
		m.moveFindCursor(len(m.findResults))
	case keyPgUp:
		m.moveFindCursor(-m.visibleLines())
	case keyPgDown:
		m.moveFindCursor(m.visibleLines())
	case keyEnter, keyRight, "l":
		return m.revealFind()
	case "v":
		return m.openViewer()
	case "o":
		return m.openFile()
	case " ":
		m.markUnderCursor()
		m.moveFindCursor(1)
	case "M":
		return m.openQueue()
	case "d":
		m.askConfirm(actionTrash)
	case "D":
		m.askConfirm(actionDelete)
	}
	return m, nil
}

func (m *model) moveFindCursor(delta int) {
	if len(m.findResults) == 0 {
		return
	}
	m.findCursor = min(max(m.findCursor+delta, 0), len(m.findResults)-1)

	height := max(m.visibleLines(), 1)
	m.findOffset = min(m.findOffset, m.findCursor)
	if m.findCursor >= m.findOffset+height {
		m.findOffset = m.findCursor - height + 1
	}
	m.findOffset = min(max(m.findOffset, 0), max(len(m.findResults)-height, 0))
}

// revealFind opens the match's directory with the cursor on it, the same move as
// on the largest-files and duplicate screens.
func (m *model) revealFind() (tea.Model, tea.Cmd) {
	item := m.selectedFind()
	if item == nil {
		return m, nil
	}
	parent := item.GetParent()
	if parent == nil {
		return m, nil
	}
	m.enterDir(parent)
	for i, r := range m.rows {
		if r == item {
			m.cursor = i
			break
		}
	}
	m.clampCursor()
	m.scr = screenBrowse
	return m, nil
}

// dropFindResult takes a deleted file out of the results.
func (m *model) dropFindResult(item fs.Item) {
	m.findResults = removeItem(m.findResults, item)
	m.findCursor = min(m.findCursor, max(len(m.findResults)-1, 0))
	m.moveFindCursor(0)
	if m.scr == screenFind && len(m.findResults) == 0 {
		m.scr = screenBrowse
		m.status, m.statusIsError = "no matches left", false
	}
}

// viewFindFooter is the prompt while typing a pattern. Composed left-to-right and
// only cut, escapes and all, when the terminal is too narrow to hold it whole —
// the same concession the filter footer makes.
func (m *model) viewFindFooter() string {
	if m.width < 1 {
		return ""
	}
	prompt := m.st.accent.Render("find ") + m.st.dirName.Render(m.findQuery) + m.st.accent.Render("▏")
	hint := m.st.dim.Render("*.mkv or a name")

	gap := m.width - lipgloss.Width(prompt) - lipgloss.Width(hint)
	if gap < 1 {
		return runewidth.Truncate(prompt, m.width, "")
	}
	return prompt + strings.Repeat(" ", gap) + hint
}

func (m *model) viewFindList() string {
	lines := m.visibleLines()
	if len(m.findResults) == 0 {
		return padLines(m.st.dim.Render(clipTo("  no matches", m.width)), lines)
	}
	end := min(m.findOffset+lines, len(m.findResults))
	rows := make([]string, 0, lines)
	for i := m.findOffset; i < end; i++ {
		rows = append(rows, m.viewFindRow(m.findResults[i], i == m.findCursor))
	}
	return padLines(joinLines(rows), lines)
}

// viewFindRow is size, then the path — the same shape as a largest-files row,
// composed as plain text at an exact width and styled after.
func (m *model) viewFindRow(item fs.Item, selected bool) string {
	if m.width < 1 {
		return ""
	}
	icon := m.rowIcon(item)
	sizeText := cellRight(m.ui.formatSize(m.itemSize(item)), sizeColWidth)
	path := item.GetPath()

	rest := m.width - 1 - runewidth.StringWidth(icon) - sizeColWidth - 1
	pathText := cellPath(path, max(rest, 0))

	plain := icon + sizeText + " " + pathText
	if selected {
		bar := m.st.accent.Render("▌")
		if m.width < 2 {
			return bar
		}
		if m.markOverlay(item) {
			return bar + m.markableGlyph(item, icon, &m.st.selected) +
				m.st.selected.Render(sizeText+" ") +
				m.renderMarkedName(pathText, &m.st.selected)
		}
		return bar + m.st.selected.Render(clipTo(plain, m.width-1))
	}
	if 1+runewidth.StringWidth(plain) > m.width {
		return m.st.fileName.Render(clipTo(" "+plain, m.width))
	}
	nameRender := m.st.fileName.Render(pathText)
	if m.markOverlay(item) {
		nameRender = m.renderMarkedName(pathText, &m.st.fileName)
	}
	return " " + m.markableGlyph(item, icon, &m.st.dim) +
		m.st.size.Render(sizeText) + " " +
		nameRender
}

func (m *model) viewFind() string {
	parts := make([]string, 0, 3)
	if m.headerHeight() > 0 {
		parts = append(parts, m.viewHeader())
	}
	parts = append(parts, m.viewFindList())
	if m.footerHeight() > 0 {
		parts = append(parts, m.viewFooter())
	}
	return joinLines(parts)
}
