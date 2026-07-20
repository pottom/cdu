//go:build unix

package charm

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

// statOwner reads the numeric owner and group off a Unix stat, and resolves their
// names. The lookups are safe here: cdu is built CGO_ENABLED=0, so os/user uses the
// pure-Go path that reads /etc/passwd and /etc/group directly — a local file, never a
// networked directory service that could hang. An id with no entry keeps its number
// and an empty name.
func statOwner(fi os.FileInfo) (uid, gid, uname, gname string) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", "", ""
	}
	uid = strconv.FormatUint(uint64(st.Uid), 10)
	gid = strconv.FormatUint(uint64(st.Gid), 10)
	if u, err := user.LookupId(uid); err == nil {
		uname = u.Username
	}
	if g, err := user.LookupGroupId(gid); err == nil {
		gname = g.Name
	}
	return uid, gid, uname, gname
}
