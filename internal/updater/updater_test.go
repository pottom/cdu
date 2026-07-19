package updater

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNewer(t *testing.T) {
	assert.True(t, IsNewer("v1.2.3", "v1.2.4"), "a higher patch is newer")
	assert.True(t, IsNewer("v1.9.0", "v1.10.0"), "10 beats 9 — numeric, not lexical")
	assert.False(t, IsNewer("v1.2.3", "v1.2.3"), "the same version is not newer")
	assert.False(t, IsNewer("v1.2.3", "v1.2.2"), "an older version is not newer")
	assert.True(t, IsNewer("v1.0.0", "v1.0.1"), "with or without the v")
	assert.True(t, IsNewer("1.0.0", "v2.0.0"))

	// A build with no real version is below everything, so a dev binary is never
	// told it is current.
	assert.True(t, IsNewer("development", "v0.0.1"))
	assert.False(t, IsNewer("v1.0.0", "not-a-version"), "an unparseable tag is never newer")

	// Build metadata and pre-release suffixes are dropped before comparing.
	assert.True(t, IsNewer("v1.2.3+gdu5.0.0", "v1.3.0+gdu5.0.0"))
	assert.False(t, IsNewer("v1.3.0+gdu5.0.0", "v1.3.0+gdu5.1.0"), "only the release version counts")
}
