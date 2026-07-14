package charm

import (
	"strings"
	"testing"

	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
)

// The cursor blinks off one clock: the same progress tick that samples the scan.
// Two clocks would beat against each other and make the line stutter.
func TestScanCursorBlinksOffTheProgressTick(t *testing.T) {
	m := benchModel(0)
	m.scr = screenScanning

	seen := map[bool]bool{}
	for range blinkTicks * 4 {
		next, _ := m.Update(tickMsg{})
		m = next.(*model)
		seen[m.blinkOn] = true
	}
	assert.True(t, seen[true] && seen[false], "the cursor must both appear and disappear")
}

// The analyzer cannot know how much tree is left, so the scan line reports what
// it does know. A percentage here would be fabricated, and the mock's is.
func TestScanLineReportsCountsNotAPercentage(t *testing.T) {
	withProfile(t, termenv.Ascii)

	m := benchModel(0)
	m.scr = screenScanning
	m.width, m.height = 80, 24
	m.haveSize = true
	m.progress.ItemCount = 12345
	m.progress.TotalUsage = 8 << 30

	body := m.viewScanBody()
	assert.Contains(t, body, "items")
	assert.NotContains(t, body, "%", "the scan line must not invent a percentage")

	// The footer must not offer keys that do nothing while the scan is running.
	footer := m.viewFooter()
	assert.Contains(t, footer, "quit")
	assert.NotContains(t, footer, "open")
	assert.NotContains(t, footer, "sorted by", "there is no list to be sorted yet")

	// The header keeps the same breadcrumb the browser will show, so the scan
	// reads as this screen filling up rather than a different screen.
	assert.True(t, strings.Contains(m.viewHeader(), "scanning"), "header must name the root under way")
}
