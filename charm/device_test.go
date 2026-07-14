package charm

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pottom/cdu/pkg/device"
)

func TestDeviceForPicksTheLongestMatchingMount(t *testing.T) {
	mounts := device.Devices{
		{Name: "root", MountPoint: "/"},
		{Name: "home", MountPoint: "/home"},
		{Name: "vault", MountPoint: "/home/me/vault"},
		{Name: "var", MountPoint: "/var"},
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/", "root"},
		{"/etc/hosts", "root"},
		{"/home", "home"},
		{"/home/me", "home"},
		// Mount points nest, so the longest prefix is the one the path lives on.
		{"/home/me/vault/x", "vault"},
		// A prefix match must land on a path component: /variable is not on /var.
		{"/variable/data", "root"},
	} {
		got := deviceFor(tc.path, mounts)
		if assert.NotNil(t, got, "no device for %s", tc.path) {
			assert.Equal(t, tc.want, got.Name, "path %s", tc.path)
		}
	}
}

// A path on no listed mount, or a machine that lists none, must simply produce
// no disk line rather than a wrong one.
func TestDeviceForReturnsNilWhenNothingMatches(t *testing.T) {
	assert.Nil(t, deviceFor("/home/me", device.Devices{}))
	assert.Nil(t, deviceFor("/home/me", device.Devices{
		{Name: "other", MountPoint: "/mnt/other"},
	}))
}
