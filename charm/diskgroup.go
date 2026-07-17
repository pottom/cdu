package charm

import (
	"regexp"
	"sort"
	"strings"

	"github.com/pottom/cdu/pkg/device"
)

// Mounts are grouped by the physical disk behind them, because a flat table
// lies about what you are looking at.
//
// On this machine `cdu -d` listed six volumes each reporting 460.4 GiB, 75%
// used — identical, six times over. They are not six disks: they are six APFS
// volumes sharing one container, so each of them honestly reports the
// container's space. Flat, that reads as half a terabyte of duplication.
// Grouped, it reads as what it is.

// diskPatterns match a *partition or volume* name and capture the disk it lives
// on. Each one requires the partition suffix, so a whole-disk mount falls
// through and becomes its own group rather than being grouped under a truncation
// of itself.
//
// They are spelled out rather than inferred. "Strip the trailing digits" turns
// nvme0n1 into nvme0n and groups loop0 with loop1, and neither failure is
// visible — the table simply shows the wrong tree, confidently.
var diskPatterns = []*regexp.Regexp{
	// macOS: disk3s1, and disk3s1s2 for a snapshot of a volume.
	regexp.MustCompile(`^(disk\d+)s\d+`),
	// Linux NVMe: nvme0n1p1. The p is what separates the namespace from the
	// partition, which is why nvme0n1 itself must not match.
	regexp.MustCompile(`^(nvme\d+n\d+)p\d+$`),
	// Linux eMMC and SD: mmcblk0p1.
	regexp.MustCompile(`^(mmcblk\d+)p\d+$`),
	// Linux SCSI/SATA/USB, virtio, Xen, and old IDE.
	regexp.MustCompile(`^(sd[a-z]+)\d+$`),
	regexp.MustCompile(`^(vd[a-z]+)\d+$`),
	regexp.MustCompile(`^(xvd[a-z]+)\d+$`),
	regexp.MustCompile(`^(hd[a-z]+)\d+$`),
}

// physicalDisk names the disk a device lives on, and reports whether the device
// is a partition of it rather than the disk itself.
//
// It comes back empty for anything that is not a disk at all: tmpfs, devfs,
// overlay, `map auto_home`. Those have no disk to group under and are not
// pretending to.
//
// /dev/mapper and /dev/dm-N are deliberately left ungrouped. An LVM volume can
// span several disks — that is what LVM is for — so there is no single parent to
// claim, and inventing one would be the same lie in a new place.
func physicalDisk(name string) (disk string, isPartition bool) {
	base, ok := strings.CutPrefix(name, "/dev/")
	if !ok || base == "" || strings.Contains(base, "/") {
		return "", false
	}
	for _, re := range diskPatterns {
		if m := re.FindStringSubmatch(base); m != nil {
			return m[1], true
		}
	}
	// A device node with no partition suffix: a whole disk, mounted directly.
	return base, false
}

// diskRow is one line of the table: a physical disk, or a device on one.
type diskRow struct {
	// dev is nil on a disk header — a physical disk is not in the mount table and
	// has no usage of its own to report, only its volumes' .
	dev  *device.Device
	disk string

	// volumes and shared describe a header. shared means every volume under it
	// reports the same size and the same free space, which is what an APFS
	// container or a btrfs pool looks like from here: one pool of space, seen
	// once per volume. It is the whole reason the numbers repeat.
	volumes int
	shared  bool

	// depth is 1 for a device under a header, 0 for a header and for a mount with
	// no disk behind it. last marks the final device in its group, for the glyph.
	depth int
	last  bool
}

func (r *diskRow) isHeader() bool { return r.dev == nil }

// groupDisks turns the mount table into the rows of the tree.
//
// Order is by usage, biggest first, at both levels: the group is ranked by its
// fullest device, and the devices within it by their own. The mount table's own
// order is whatever the kernel holds, which means nothing to someone looking for
// the disk that is full.
func groupDisks(devices device.Devices) []diskRow {
	type group struct {
		disk    string
		devs    []*device.Device
		maxUsed int64
	}

	var (
		groups   []*group
		byName   = map[string]*group{}
		ungroupd []*device.Device
	)

	for i := range devices {
		dev := devices[i]
		disk, _ := physicalDisk(dev.Name)
		if disk == "" {
			ungroupd = append(ungroupd, dev)
			continue
		}
		g, ok := byName[disk]
		if !ok {
			g = &group{disk: disk}
			byName[disk] = g
			groups = append(groups, g)
		}
		g.devs = append(g.devs, dev)
		g.maxUsed = max(g.maxUsed, dev.GetUsage())
	}

	sort.SliceStable(groups, func(i, j int) bool { return groups[i].maxUsed > groups[j].maxUsed })
	sort.SliceStable(ungroupd, func(i, j int) bool { return ungroupd[i].GetUsage() > ungroupd[j].GetUsage() })

	var rows []diskRow
	for _, g := range groups {
		sort.SliceStable(g.devs, func(i, j int) bool { return g.devs[i].GetUsage() > g.devs[j].GetUsage() })

		rows = append(rows, diskRow{
			disk:    g.disk,
			volumes: len(g.devs),
			shared:  sharesOnePool(g.devs),
		})
		for i, dev := range g.devs {
			rows = append(rows, diskRow{dev: dev, disk: g.disk, depth: 1, last: i == len(g.devs)-1})
		}
	}
	// Whatever is not on a disk goes last and unindented: tmpfs and friends are
	// not volumes of anything, and pretending otherwise would be the lie again.
	for _, dev := range ungroupd {
		rows = append(rows, diskRow{dev: dev})
	}
	return rows
}

// sharesOnePool reports whether every device here reports the same space, which
// is what one APFS container — or one btrfs pool — looks like through statfs.
//
// It is a guess from the outside, and the only one available: nothing in the
// mount table says "these share". But two volumes agreeing to the byte on both
// total and free is not a coincidence, and saying "one pool" is what makes six
// identical rows read as six views of one thing rather than as six disks.
func sharesOnePool(devs []*device.Device) bool {
	if len(devs) < 2 {
		return false
	}
	for _, d := range devs[1:] {
		if d.Size != devs[0].Size || d.Free != devs[0].Free {
			return false
		}
	}
	return true
}
