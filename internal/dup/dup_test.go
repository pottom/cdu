package dup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/analyze"
	"github.com/pottom/cdu/pkg/fs"
)

// scanTree writes a set of files and returns the analyzed tree, so the test
// exercises the real GetSize / GetMultiLinkedInode / GetPath the finder relies
// on rather than a mock of them.
func scanTree(t *testing.T, files map[string]string) fs.Item {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		full := filepath.Join(root, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o700))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}
	analyzer := analyze.CreateAnalyzer()
	dir := analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, func(string) bool { return false })
	dir.UpdateStats(make(fs.HardLinkedItems))
	return dir
}

func never() bool { return false }

func names(g Group) []string {
	var out []string
	for _, f := range g.Files {
		out = append(out, f.GetName())
	}
	return out
}

// The whole point: two files with the same bytes are a duplicate; a third with
// the same *size* but different bytes is not.
func TestFindsByteIdenticalFilesAndNotSizeAlikes(t *testing.T) {
	root := scanTree(t, map[string]string{
		"a/report.txt": "the quick brown fox",
		"b/copy.txt":   "the quick brown fox", // identical to a/report.txt
		"c/decoy.txt":  "the lazy brown dogs", // same length, different bytes
		"d/unique.dat": "something else again of another length entirely",
	})

	groups, err := Find(root, never)
	require.NoError(t, err)
	require.Len(t, groups, 1, "exactly one duplicate group")

	assert.ElementsMatch(t, []string{"report.txt", "copy.txt"}, names(groups[0]))
	assert.NotContains(t, names(groups[0]), "decoy.txt", "same size is not same content")
}

// The reason the size bucket comes first: a file whose size nobody shares is
// never opened. This checks the outcome, which is that nothing but the real
// candidates ends up compared.
func TestAUniqueSizeIsNeverADuplicate(t *testing.T) {
	root := scanTree(t, map[string]string{
		"one.txt": "a",
		"two.txt": "bb",
		"three":   "ccc",
	})
	groups, err := Find(root, never)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

// Hard links are not duplicates. Two names for one inode share their bytes on
// disk, so deleting one frees nothing — reporting them would be a lie about
// reclaimable space.
func TestHardLinksAreNotDuplicates(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.bin")
	require.NoError(t, os.WriteFile(original, []byte("shared bytes on one inode"), 0o600))
	if err := os.Link(original, filepath.Join(dir, "hardlink.bin")); err != nil {
		t.Skipf("hard links unsupported here: %v", err)
	}

	analyzer := analyze.CreateAnalyzer()
	root := analyzer.AnalyzeDir(dir, func(_, _ string) bool { return false }, func(string) bool { return false })
	root.UpdateStats(make(fs.HardLinkedItems))

	groups, err := Find(root, never)
	require.NoError(t, err)
	assert.Empty(t, groups, "two names for one inode are one file, not a duplicate")
}

// A real copy alongside a hard link: the copy counts, the hard-linked twin does
// not, so the group has two members and not three.
func TestACopyCountsButItsHardLinkedTwinDoesNot(t *testing.T) {
	dir := t.TempDir()
	body := []byte("content that exists three times by name, twice by inode")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.bin"), body, 0o600))
	if err := os.Link(filepath.Join(dir, "a.bin"), filepath.Join(dir, "a-link.bin")); err != nil {
		t.Skipf("hard links unsupported here: %v", err)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b-copy.bin"), body, 0o600)) // a genuine second copy

	analyzer := analyze.CreateAnalyzer()
	root := analyzer.AnalyzeDir(dir, func(_, _ string) bool { return false }, func(string) bool { return false })
	root.UpdateStats(make(fs.HardLinkedItems))

	groups, err := Find(root, never)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0].Files, 2, "the hard-linked twin is one file with a.bin, not a third copy")
	assert.Equal(t, int64(len(body)), groups[0].Reclaimable(), "deleting the one real copy frees its size, once")
}

// Empty files are all trivially identical and deleting them frees nothing. They
// are noise, and left out.
func TestEmptyFilesAreIgnored(t *testing.T) {
	root := scanTree(t, map[string]string{
		"empty1": "",
		"empty2": "",
		"empty3": "",
	})
	groups, err := Find(root, never)
	require.NoError(t, err)
	assert.Empty(t, groups)
}

// Groups come back most-reclaimable first: three copies of a big file outrank
// two copies of a small one, because that is the order you would act in.
func TestGroupsAreSortedByReclaimableSpace(t *testing.T) {
	big := string(make([]byte, 4096))
	small := string(make([]byte, 100)) + "x" // avoid the all-zero big file's prefix
	root := scanTree(t, map[string]string{
		"big/1":   big,
		"big/2":   big,
		"small/1": small,
		"small/2": small,
		"small/3": small,
	})

	groups, err := Find(root, never)
	require.NoError(t, err)
	require.Len(t, groups, 2)

	assert.Equal(t, int64(4096), groups[0].Size, "the big group reclaims more, so it leads")
	assert.Equal(t, int64(4096), groups[0].Reclaimable(), "one redundant 4096-byte copy")
	assert.Equal(t, int64(2*101), groups[1].Reclaimable(), "two redundant copies of the small file")
}

// An unreadable file is skipped, never guessed. cdu must not claim two files
// match when it could not read one of them — the answer feeds a delete.
func TestUnreadableFilesAreSkippedNotAssumedEqual(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads anything; the permission cannot be tested")
	}
	dir := t.TempDir()
	body := "identical everywhere"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readable.txt"), []byte(body), 0o600))
	locked := filepath.Join(dir, "locked.txt")
	require.NoError(t, os.WriteFile(locked, []byte(body), 0o600))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })

	analyzer := analyze.CreateAnalyzer()
	root := analyzer.AnalyzeDir(dir, func(_, _ string) bool { return false }, func(string) bool { return false })
	root.UpdateStats(make(fs.HardLinkedItems))

	groups, err := Find(root, never)
	require.NoError(t, err)
	assert.Empty(t, groups, "one of the pair could not be read, so no match may be claimed")
}

// Cancel stops the search. It is checked between files, so it lands promptly on a
// tree of many candidates.
func TestCancelStopsTheSearch(t *testing.T) {
	files := map[string]string{}
	for i := range 50 {
		// Same size, same content: all candidates, so all would be hashed.
		files[filepath.Join("d", string(rune('a'+i%26))+string(rune('a'+i/26)))] = "identical body of some length"
	}
	root := scanTree(t, files)

	_, err := Find(root, func() bool { return true }) // cancelled from the first check
	require.ErrorIs(t, err, ErrCancelled)
}

// Three copies in one group, and the reclaimable is two of them.
func TestThreeCopiesReclaimTwo(t *testing.T) {
	root := scanTree(t, map[string]string{
		"x/1.iso": "a whole disk image, pretend",
		"y/2.iso": "a whole disk image, pretend",
		"z/3.iso": "a whole disk image, pretend",
	})
	groups, err := Find(root, never)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Len(t, groups[0].Files, 3)
	assert.Equal(t, int64(2*len("a whole disk image, pretend")), groups[0].Reclaimable())
}
