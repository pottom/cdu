//go:build !windows

package elevate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The plain command is sudo rm -rf, with the path after -- so no name that looks
// like a flag is ever taken as one.
func TestRemoveCmdPlain(t *testing.T) {
	cmd := RemoveCmd("/tmp/-rf-trick", "")
	assert.Equal(t, []string{"sudo", "rm", "-rf", "--", "/tmp/-rf-trick"}, cmd.Args)
}

// With a notice the command becomes a tiny shell that prints the line and then execs
// sudo — with the notice and the path as positional arguments, never spliced into
// the script, so neither can break out of it.
func TestRemoveCmdWithNotice(t *testing.T) {
	cmd := RemoveCmd("/tmp/a b; rm -rf ~", "heads up")
	assert.Equal(t, "sh", cmd.Args[0])
	assert.Contains(t, cmd.Args, "heads up")
	assert.Contains(t, cmd.Args, "/tmp/a b; rm -rf ~", "the path is an argument, not part of the script")
	// The script itself must not contain the path or the notice interpolated in.
	assert.NotContains(t, cmd.Args[2], "heads up")
	assert.NotContains(t, cmd.Args[2], "rm -rf ~")
}
