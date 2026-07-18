//go:build !windows

package elevate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The plain command is one sudo rm -rf over every path, with -- before them so no
// name that looks like a flag is ever taken as one.
func TestRemoveCmdPlain(t *testing.T) {
	cmd := RemoveCmd([]string{"/tmp/-rf-trick", "/tmp/two"}, "")
	assert.Equal(t, []string{"sudo", "rm", "-rf", "--", "/tmp/-rf-trick", "/tmp/two"}, cmd.Args)
}

// With a notice the command becomes a tiny shell that prints the line and then execs
// one sudo over all the paths — the notice and the paths as positional arguments,
// never spliced into the script, so none can break out of it.
func TestRemoveCmdWithNotice(t *testing.T) {
	cmd := RemoveCmd([]string{"/tmp/a b; rm -rf ~"}, "heads up")
	assert.Equal(t, "sh", cmd.Args[0])
	assert.Contains(t, cmd.Args, "heads up")
	assert.Contains(t, cmd.Args, "/tmp/a b; rm -rf ~", "the path is an argument, not part of the script")
	// The script itself must carry neither the path nor the notice interpolated in.
	assert.NotContains(t, cmd.Args[2], "heads up")
	assert.NotContains(t, cmd.Args[2], "rm -rf ~")
}
