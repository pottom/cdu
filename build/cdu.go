package build

// GduVersion is the upstream gdu release this cdu is built on.
//
// cdu carries its own version in Version (a fork with its own UI and features keeps
// its own version, not the one it forked from). This records which gdu the engine is
// synced to — build metadata, the "+gduA.B.C" of a "cdu vX.Y.Z+gduA.B.C" — and is
// bumped here on each upstream sync. A release can override it with
// -ldflags "-X github.com/pottom/cdu/build.GduVersion=A.B.C".
//
// It lives in its own file so build/build.go, which is upstream's and on the merge
// conflict surface, is never edited.
var GduVersion = "5.36.1"
