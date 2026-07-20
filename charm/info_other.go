//go:build !unix

package charm

import "os"

// statOwner has no owner to give off a non-Unix stat (Windows has no uid/gid), so the
// info pane omits it.
func statOwner(_ os.FileInfo) (uid, gid, uname, gname string) {
	return "", "", "", ""
}
