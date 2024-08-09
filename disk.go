package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

type DiskInfo struct {
	Name       string
	Path       string
	Temp       int64
	TotalSize  int64
	UsedSize   int64
	ReadCount  int64
	WriteCount int64
}

type MountOptions []string

func (opts MountOptions) Mode() string {
	if opts.exists("rw") {
		return "rw"
	} else if opts.exists("ro") {
		return "ro"
	}
	return "unknown"
}

func (opts MountOptions) exists(opt string) bool {
	for _, o := range opts {
		if o == opt {
			return true
		}
	}
	return false
}

type set struct {
	m map[string]struct{}
}

func (s *set) empty() bool {
	return len(s.m) == 0
}

func (s *set) add(key string) {
	s.m[key] = struct{}{}
}

func (s *set) has(key string) bool {
	var ok bool
	_, ok = s.m[key]
	return ok
}
func newSet() *set {
	s := &set{
		m: make(map[string]struct{}),
	}
	return s
}

func GetDiskInfos(mountPoints []string, ignoreMountOpts []string, ignoreFS []string) ([]DiskInfo, error) {
	diskInfos := []DiskInfo{}

	out, err := Exec("lsblk", "-P", "-b", "-o", "NAME,PATH,TYPE,SIZE")
	if err != nil {
		return nil, fmt.Errorf("failed to run command lsblk: %w - %s", err, string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		diskInfo := DiskInfo{}
		pairs := strings.Split(strings.TrimSpace(string(line)), " ")
		diskName := ""
		diskType := ""
		diskPath := ""
		diskSize := 0
		for _, pair := range pairs {
			kv := strings.Split(strings.TrimSpace(string(pair)), "=")
			if kv[0] == "NAME" {
				diskName = strings.Trim(kv[1], "\"")
			}
			if kv[0] == "TYPE" {
				diskType = strings.Trim(kv[1], "\"")
			}
			if kv[0] == "PATH" {
				diskPath = strings.Trim(kv[1], "\"")
			}
			if kv[0] == "SIZE" {
				diskSize, _ = strconv.Atoi(strings.Trim(kv[1], "\""))
			}
		}
		if diskType == "disk" {
			diskInfo.Name = diskName
			diskInfo.Path = diskPath
			diskInfo.TotalSize = int64(diskSize)
			counterStat, _ := disk.IOCounters(diskName)
			diskInfo.ReadCount = int64(counterStat[diskName].MergedReadCount)
			diskInfo.WriteCount = int64(counterStat[diskName].MergedWriteCount)
			out, _ := Exec("hddtemp", "-n", filepath.Join("/dev", diskName))
			temp, _ := strconv.Atoi(strings.Trim(string(out), "\n"))
			diskInfo.Temp = int64(temp)
			diskInfos = append(diskInfos, diskInfo)
		}
	}
	return diskInfos, nil
}

func DiskUsage(
	mountPointFilter []string,
	mountOptsExclude []string,
	fstypeExclude []string,
) ([]*disk.UsageStat, []*disk.PartitionStat, error) {
	parts, err := disk.Partitions(true)
	if err != nil {
		return nil, nil, err
	}

	mountPointFilterSet := newSet()
	for _, filter := range mountPointFilter {
		mountPointFilterSet.add(filter)
	}
	mountOptFilterSet := newSet()
	for _, filter := range mountOptsExclude {
		mountOptFilterSet.add(filter)
	}
	fstypeExcludeSet := newSet()
	for _, filter := range fstypeExclude {
		fstypeExcludeSet.add(filter)
	}
	paths := newSet()
	for _, part := range parts {
		paths.add(part.Mountpoint)
	}

	// Autofs mounts indicate a potential mount, the partition will also be
	// listed with the actual filesystem when mounted.  Ignore the autofs
	// partition to avoid triggering a mount.
	fstypeExcludeSet.add("autofs")

	var usage []*disk.UsageStat
	var partitions []*disk.PartitionStat
	hostMountPrefix := os.Getenv("HOST_MOUNT_PREFIX")

partitionRange:
	for i := range parts {
		p := parts[i]

		for _, o := range p.Opts {
			if !mountOptFilterSet.empty() && mountOptFilterSet.has(o) {
				continue partitionRange
			}
		}
		// If there is a filter set and if the mount point is not a
		// member of the filter set, don't gather info on it.
		if !mountPointFilterSet.empty() && !mountPointFilterSet.has(p.Mountpoint) {
			continue
		}

		// If the mount point is a member of the exclude set,
		// don't gather info on it.
		if fstypeExcludeSet.has(p.Fstype) {
			continue
		}

		// If there's a host mount prefix use it as newer gopsutil version check for
		// the init's mountpoints usually pointing to the host-mountpoint but in the
		// container. This won't work for checking the disk-usage as the disks are
		// mounted at HOST_MOUNT_PREFIX...
		mountpoint := p.Mountpoint
		if hostMountPrefix != "" && !strings.HasPrefix(p.Mountpoint, hostMountPrefix) {
			mountpoint = filepath.Join(hostMountPrefix, p.Mountpoint)
			// Exclude conflicting paths
			if paths.has(mountpoint) {

				continue
			}
		}

		du, err := disk.Usage(mountpoint)
		if err != nil {

			continue
		}

		du.Path = filepath.Join(string(os.PathSeparator), strings.TrimPrefix(p.Mountpoint, hostMountPrefix))
		du.Fstype = p.Fstype
		usage = append(usage, du)
		partitions = append(partitions, &p)
	}

	return usage, partitions, nil
}
