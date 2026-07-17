package charm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pottom/cdu/pkg/device"
)

// The grouping is a heuristic on device names, and a wrong one is invisible: the
// table just shows a confidently wrong tree. So every form it claims to know is
// pinned here.
func TestPhysicalDiskNaming(t *testing.T) {
	for _, tc := range []struct {
		name      string
		disk      string
		partition bool
		why       string
	}{
		// macOS.
		{"/dev/disk3s1", "disk3", true, "an APFS volume"},
		{"/dev/disk3s1s2", "disk3", true, "a snapshot of a volume is still on disk3"},
		{"/dev/disk11s5", "disk11", true, "two-digit disks are not disk1"},
		{"/dev/disk3", "disk3", false, "a whole disk, mounted directly"},

		// Linux.
		{"/dev/sda1", "sda", true, ""},
		{"/dev/sda", "sda", false, "no partition suffix: the disk itself"},
		{"/dev/sdab3", "sdab", true, "past sdz the names keep going"},
		{"/dev/vda2", "vda", true, "virtio"},
		{"/dev/xvda1", "xvda", true, "xen"},
		{"/dev/hda1", "hda", true, "ide, still out there"},
		{"/dev/nvme0n1p3", "nvme0n1", true, "the p separates namespace from partition"},
		{"/dev/nvme0n1", "nvme0n1", false, "a namespace is not a partition of nvme0n"},
		{"/dev/mmcblk0p1", "mmcblk0", true, "sd cards and emmc"},
		{"/dev/mmcblk0", "mmcblk0", false, ""},

		// Its own disk, not a partition of a truncation of itself. "Strip the
		// trailing digits" would file loop0 and loop1 under "loop".
		{"/dev/loop0", "loop0", false, "each loop device is its own disk"},
		{"/dev/loop1", "loop1", false, ""},
		{"/dev/sr0", "sr0", false, "optical"},

		// Not disks at all.
		{"tmpfs", "", false, "not a device node"},
		{"devfs", "", false, ""},
		{"overlay", "", false, ""},
		{"map auto_home", "", false, "macOS autofs"},
		{"", "", false, ""},

		// LVM and device-mapper span disks by design; there is no one parent.
		{"/dev/mapper/vg0-root", "", false, "an LVM volume can span several disks"},
	} {
		disk, isPart := physicalDisk(tc.name)
		assert.Equal(t, tc.disk, disk, "%s: %s", tc.name, tc.why)
		assert.Equal(t, tc.partition, isPart, "%s: partition?", tc.name)
	}
}

// loop0 and loop1 are two disks. The obvious implementation files them together.
func TestLoopDevicesAreNotSiblings(t *testing.T) {
	a, _ := physicalDisk("/dev/loop0")
	b, _ := physicalDisk("/dev/loop1")
	assert.NotEqual(t, a, b, "loop0 and loop1 are not partitions of a disk called loop")
}

func TestGroupingBuildsATree(t *testing.T) {
	rows := groupDisks(device.Devices{
		{Name: "/dev/disk3s1", MountPoint: "/", Size: 994 << 30, Free: 300 << 30},
		{Name: "/dev/disk1s1", MountPoint: "/x", Size: 500 << 20, Free: 400 << 20},
		{Name: "/dev/disk3s5", MountPoint: "/System/Volumes/Data", Size: 994 << 30, Free: 300 << 30},
		{Name: "tmpfs", MountPoint: "/tmp", Size: 1 << 30, Free: 1 << 29},
	})

	// disk3 first: its group is the fullest. Header, then its two volumes.
	require.Len(t, rows, 2+1+1+1+1, "two headers, three volumes, one loner")
	assert.True(t, rows[0].isHeader())
	assert.Equal(t, "disk3", rows[0].disk)
	assert.Equal(t, 2, rows[0].volumes)
	assert.True(t, rows[0].shared, "two volumes reporting the same space are one container")

	assert.Equal(t, 1, rows[1].depth)
	assert.False(t, rows[1].last)
	assert.Equal(t, 1, rows[2].depth)
	assert.True(t, rows[2].last, "the last volume in a group is marked, for the glyph")

	assert.True(t, rows[3].isHeader())
	assert.Equal(t, "disk1", rows[3].disk)
	assert.False(t, rows[3].shared, "one volume is not a shared pool")

	// tmpfs is not on a disk. It goes last, unindented, with no header.
	last := rows[len(rows)-1]
	assert.False(t, last.isHeader())
	assert.Equal(t, 0, last.depth, "a mount with no disk is not indented under one")
	assert.Equal(t, "tmpfs", last.dev.Name)
}

// The reason the tree exists. Six APFS volumes each report the container's
// space, so a flat table shows 460 GiB six times and reads as six disks.
func TestVolumesSharingAContainerAreMarkedShared(t *testing.T) {
	// This machine's actual table: four volumes of one container, each reporting
	// the container's space, to the byte.
	same := device.Devices{
		{Name: "/dev/disk3s1s1", MountPoint: "/", Size: 994 << 30, Free: 367 << 30},
		{Name: "/dev/disk3s5", MountPoint: "/System/Volumes/Data", Size: 994 << 30, Free: 367 << 30},
		{Name: "/dev/disk3s6", MountPoint: "/System/Volumes/VM", Size: 994 << 30, Free: 367 << 30},
		{Name: "/dev/disk3s2", MountPoint: "/System/Volumes/Preboot", Size: 994 << 30, Free: 367 << 30},
	}
	rows := groupDisks(same)
	require.True(t, rows[0].isHeader())
	assert.True(t, rows[0].shared)
	assert.Equal(t, 4, rows[0].volumes)

	// A classic partition table is the opposite: each partition has its own space.
	rows = groupDisks(device.Devices{
		{Name: "/dev/sda1", MountPoint: "/boot", Size: 1 << 30, Free: 1 << 29},
		{Name: "/dev/sda2", MountPoint: "/", Size: 500 << 30, Free: 100 << 30},
	})
	require.True(t, rows[0].isHeader())
	assert.False(t, rows[0].shared, "partitions do not share a pool")
}

func TestGroupsAreSortedByTheirFullestDevice(t *testing.T) {
	rows := groupDisks(device.Devices{
		{Name: "/dev/sda1", MountPoint: "/small", Size: 10 << 30, Free: 9 << 30}, // 1 GiB used
		{Name: "/dev/sdb1", MountPoint: "/big", Size: 100 << 30, Free: 10 << 30}, // 90 GiB used
		{Name: "/dev/sda2", MountPoint: "/mid", Size: 100 << 30, Free: 50 << 30}, // 50 GiB used
	})

	assert.Equal(t, "sdb", rows[0].disk, "the group with the fullest device leads")
	assert.Equal(t, "sda", rows[2].disk)
	// And within a group, biggest first.
	assert.Equal(t, "/mid", rows[3].dev.MountPoint)
	assert.Equal(t, "/small", rows[4].dev.MountPoint)
}

func TestGroupingAnEmptyTable(t *testing.T) {
	assert.Empty(t, groupDisks(nil))
	assert.Empty(t, groupDisks(device.Devices{}))
}
