// Package dup finds byte-identical files in a scanned tree.
//
// It is the one thing in cdu that reads file contents. Everything else only
// stats — that is the whole reason cdu is fast — so this is opt-in, runs off the
// render loop, and can be cancelled. Honest duplicate detection cannot be done
// from stat alone: two different files can share a size to the byte, and the key
// next to the result is `d`.
package dup

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"sort"

	"github.com/pottom/cdu/pkg/fs"
)

// ErrCancelled is returned when the caller's cancel function asks to stop.
var ErrCancelled = errors.New("duplicate search cancelled")

// Group is a set of files with identical content, on two or more distinct
// inodes. Hard-linked copies of one file are one member, not several: they share
// their bytes on disk, so deleting one frees nothing.
type Group struct {
	Size  int64
	Files fs.Files
}

// Reclaimable is the space freed by keeping a single copy: every file past the
// first is redundant.
func (g Group) Reclaimable() int64 { return int64(len(g.Files)-1) * g.Size }

// Find returns the duplicate groups in the tree under root, most reclaimable
// first.
//
// The work is proportional to the *candidates*, not the tree. Files are bucketed
// by size first — free, the sizes are already in memory — and only sizes shared
// by two or more files are ever read. Most files have a size nobody else does
// and are never opened.
//
// cancel is checked between reads. It cannot interrupt a single file mid-hash,
// but the granularity is one file, which is enough: a cancel lands within one
// file's read.
//
// An unreadable file is skipped, not guessed at. A permission error must never
// make cdu claim two files are the same when it could not check.
func Find(root fs.Item, cancel func() bool) ([]Group, error) {
	bySize := map[int64]fs.Files{}
	collectBySize(root, bySize)

	var groups []Group
	for size, items := range bySize {
		if len(items) < 2 {
			continue // a size nobody shares cannot be duplicated
		}
		candidates := dedupeInodes(items)
		if len(candidates) < 2 {
			continue // all one file under different names — hard links, not copies
		}

		byHash := map[[sha256.Size]byte]fs.Files{}
		for _, item := range candidates {
			if cancel() {
				return nil, ErrCancelled
			}
			sum, err := hashFile(item.GetPath())
			if err != nil {
				continue
			}
			byHash[sum] = append(byHash[sum], item)
		}

		for _, matched := range byHash {
			if len(matched) >= 2 {
				sortByPath(matched)
				groups = append(groups, Group{Size: size, Files: matched})
			}
		}
	}

	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].Reclaimable() != groups[j].Reclaimable() {
			return groups[i].Reclaimable() > groups[j].Reclaimable()
		}
		// A stable tiebreak so the list does not reshuffle between runs.
		return groups[i].Files[0].GetPath() < groups[j].Files[0].GetPath()
	})
	return groups, nil
}

// collectBySize buckets every file (not directory) by its size. Zero-length
// files are left out: they are all trivially identical, in their hundreds, and
// deleting one frees nothing — noise, not a finding.
func collectBySize(item fs.Item, bySize map[int64]fs.Files) {
	if item.IsDir() {
		for child := range item.GetFiles(fs.SortByName, fs.SortAsc) {
			collectBySize(child, bySize)
		}
		return
	}
	if size := item.GetSize(); size > 0 {
		bySize[size] = append(bySize[size], item)
	}
}

// dedupeInodes keeps one file per hard-linked inode. Two names for one inode are
// one file's worth of bytes on disk; counting both would report space that
// deleting a name would not free. A file that is not hard-linked has inode 0
// here and is always kept.
func dedupeInodes(items fs.Files) fs.Files {
	seen := map[uint64]struct{}{}
	out := make(fs.Files, 0, len(items))
	for _, item := range items {
		if inode := item.GetMultiLinkedInode(); inode != 0 {
			if _, ok := seen[inode]; ok {
				continue
			}
			seen[inode] = struct{}{}
		}
		out = append(out, item)
	}
	return out
}

// hashFile is the SHA-256 of a file's contents. SHA-256 rather than a faster
// non-cryptographic hash because the answer feeds a delete: a collision here
// would tell someone two different files are the same, and the odds of a
// SHA-256 collision are not odds worth a `d` keypress.
func hashFile(path string) ([sha256.Size]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	var sum [sha256.Size]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func sortByPath(items fs.Files) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].GetPath() < items[j].GetPath()
	})
}
