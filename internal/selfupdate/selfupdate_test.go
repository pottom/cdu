package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetName(t *testing.T) {
	got, err := assetName("0.1.0", "linux", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "cdu_0.1.0_linux_amd64.tar.gz", got)

	got, err = assetName("1.2.3", "darwin", "arm64")
	require.NoError(t, err)
	assert.Equal(t, "cdu_1.2.3_darwin_arm64.tar.gz", got)

	// Windows archives are zips.
	got, err = assetName("0.1.0", "windows", "amd64")
	require.NoError(t, err)
	assert.Equal(t, "cdu_0.1.0_windows_amd64.zip", got)

	// 32-bit arm cannot be named at runtime — its GOARM (v5/v6/v7) is not exposed, and
	// the archive name encodes it — so self-update declines rather than fetch the wrong
	// build.
	_, err = assetName("0.1.0", "linux", "arm")
	assert.ErrorIs(t, err, ErrUnsupported)
}

func TestBinaryName(t *testing.T) {
	assert.Equal(t, "cdu", binaryName("linux"))
	assert.Equal(t, "cdu.exe", binaryName("windows"))
}

func TestVerifyChecksum(t *testing.T) {
	archive := []byte("pretend this is a release archive")
	sum := sha256.Sum256(archive)
	hexSum := hex.EncodeToString(sum[:])

	// GoReleaser's format: "<hex>  <filename>", two spaces, several lines.
	sums := "deadbeef  cdu_0.1.0_darwin_arm64.tar.gz\n" +
		hexSum + "  cdu_0.1.0_linux_amd64.tar.gz\n"

	assert.NoError(t, verifyChecksum(archive, sums, "cdu_0.1.0_linux_amd64.tar.gz"),
		"the matching line's hash verifies")

	err := verifyChecksum([]byte("tampered"), sums, "cdu_0.1.0_linux_amd64.tar.gz")
	assert.Error(t, err, "a different archive fails")
	assert.Contains(t, err.Error(), "mismatch")

	err = verifyChecksum(archive, sums, "cdu_0.1.0_freebsd_amd64.tar.gz")
	assert.Error(t, err, "an asset with no line fails")
	assert.Contains(t, err.Error(), "not listed")
}

func TestExtractTarGz(t *testing.T) {
	want := []byte("\x7fELF-ish cdu binary")
	archive := makeTarGz(t, map[string][]byte{
		// GoReleaser also puts LICENSE etc. beside the binary; extraction must pick the
		// binary out of the rest.
		"LICENSE.md": []byte("license"),
		"cdu":        want,
	})

	got, err := extractBinary(archive, false, "cdu")
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = extractBinary(archive, false, "cdu.exe")
	assert.Error(t, err, "a name not in the archive is an error")
}

func TestExtractZip(t *testing.T) {
	want := []byte("MZ-ish cdu.exe binary")
	archive := makeZip(t, map[string][]byte{
		"README.md": []byte("readme"),
		"cdu.exe":   want,
	})

	got, err := extractBinary(archive, true, "cdu.exe")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// extractBinary matches on the base name, so a binary under a directory prefix is
// still found — some archives nest their contents.
func TestExtractFindsBinaryUnderAPrefix(t *testing.T) {
	want := []byte("nested binary")
	archive := makeTarGz(t, map[string][]byte{"cdu_0.1.0/cdu": want})

	got, err := extractBinary(archive, false, "cdu")
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestErrUpToDateIsDistinct(t *testing.T) {
	assert.False(t, errors.Is(ErrUpToDate, ErrUnsupported))
}

func makeTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(data)),
		}))
		_, err := tw.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func makeZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}
