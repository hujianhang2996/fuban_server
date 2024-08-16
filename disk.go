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

func HasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}

func GetDiskInfosUnraid() ([]DiskInfo, error) {
	diskInfos := []DiskInfo{}
	content, err := os.ReadFile("/proc/mdstat")
	if err != nil {
		return nil, fmt.Errorf("failed to read file /proc/mdstat")
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	mdInfos := [](map[string]string){}
	mdNumDisks := 0
	crrentIdx := -1
	started := false
	for _, line := range lines {
		if HasPrefix(line, "mdNumDisks") {
			mdNumDisks, _ = strconv.Atoi(strings.Split(strings.TrimSpace(line), "=")[1])
		}
		if HasPrefix(line, "diskNumber") {
			started = true
		}
		if !started {
			continue
		}
		keyNumAndValue := strings.Split(strings.TrimSpace(line), "=")
		keyNum := strings.Split(keyNumAndValue[0], ".")
		idx, _ := strconv.Atoi(keyNum[1])
		if idx >= mdNumDisks {
			break
		}
		if idx > crrentIdx {
			mdInfos = append(mdInfos, map[string]string{})
			crrentIdx = idx
		}
		mdInfos[crrentIdx][keyNum[0]] = keyNumAndValue[1]
	}

	out, err := Exec("lsblk", "-P", "-b", "-o", "NAME,PATH,TYPE,FSSIZE,FSUSED,MOUNTPOINT")
	if err != nil {
		return nil, fmt.Errorf("failed to run command lsblk: %w - %s", err, string(out))
	}

	blkInfos := map[string](map[string]string){}
	lines1 := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines1 {
		blkInfo := map[string]string{}
		pairs := strings.Split(strings.TrimSpace(string(line)), " ")
		name := ""
		for _, pair := range pairs {
			kv := strings.Split(strings.TrimSpace(string(pair)), "=")
			if kv[0] == "NAME" {
				name = strings.Trim(kv[1], "\"")
			} else {
				blkInfo[kv[0]] = strings.Trim(kv[1], "\"")
			}
		}
		blkInfos[name] = blkInfo
	}
	for _, mdInfo := range mdInfos {
		diskInfo := DiskInfo{}
		diskInfo.Name = mdInfo["diskId"]
		if mdInfo["diskName"] != "" {
			diskInfo.Path = blkInfos[mdInfo["diskName"]]["MOUNTPOINT"]
			totalSize, _ := strconv.Atoi(blkInfos[mdInfo["diskName"]]["FSSIZE"])
			diskInfo.TotalSize = int64(totalSize)
			usedSize, _ := strconv.Atoi(blkInfos[mdInfo["diskName"]]["FSUSED"])
			diskInfo.UsedSize = int64(usedSize)
		}
		rdevReads, _ := strconv.Atoi(mdInfo["rdevReads"])
		diskInfo.ReadCount = int64(rdevReads)
		rdevWrites, _ := strconv.Atoi(mdInfo["rdevWrites"])
		diskInfo.WriteCount = int64(rdevWrites)
		// path := blkInfos[mdInfo["rdevName"]]["PATH"]
		// tempOut, err := Exec("smartctl", "-j", "-a", path)
		// if err != nil {
		// 	diskInfos = append(diskInfos, diskInfo)
		// 	continue
		// }
		// var tempOutJson map[string]interface{}
		// json.Unmarshal(tempOut, &tempOutJson)
		// temperatureInter := tempOutJson["temperature"]
		// temperature := temperatureInter.(map[string]interface{})["current"].(float64)
		path := blkInfos[mdInfo["rdevName"]]["PATH"]
		// tempOut, err := Exec("hddtemp", path)
		// if err != nil {
		// 	diskInfos = append(diskInfos, diskInfo)
		// 	continue
		// }
		// temperature, err := strconv.Atoi(strings.Trim(string(tempOut), "\n"))
		// if err != nil {
		// 	diskInfos = append(diskInfos, diskInfo)
		// 	continue
		// }
		diskInfo.Temp = GoHddtemp(path)
		diskInfos = append(diskInfos, diskInfo)
	}
	// for _, line := range lines1 {
	// 	diskInfo := DiskInfo{}
	// 	pairs := strings.Split(strings.TrimSpace(string(line)), " ")
	// 	diskName := ""
	// 	diskType := ""
	// 	diskPath := ""
	// 	diskSize := 0
	// 	for _, pair := range pairs {
	// 		kv := strings.Split(strings.TrimSpace(string(pair)), "=")
	// 		if kv[0] == "NAME" {
	// 			diskName = strings.Trim(kv[1], "\"")
	// 		}
	// 		if kv[0] == "TYPE" {
	// 			diskType = strings.Trim(kv[1], "\"")
	// 		}
	// 		if kv[0] == "PATH" {
	// 			diskPath = strings.Trim(kv[1], "\"")
	// 		}
	// 		if kv[0] == "SIZE" {
	// 			diskSize, _ = strconv.Atoi(strings.Trim(kv[1], "\""))
	// 		}
	// 	}
	// 	if diskType == "disk" {
	// 		diskInfo.Name = diskName
	// 		diskInfo.Path = diskPath
	// 		diskInfo.TotalSize = int64(diskSize)
	// 		counterStat, _ := disk.IOCounters(diskName)
	// 		diskInfo.ReadCount = int64(counterStat[diskName].MergedReadCount)
	// 		diskInfo.WriteCount = int64(counterStat[diskName].MergedWriteCount)
	// 		out, _ := Exec("hddtemp", "-n", filepath.Join("/dev", diskName))
	// 		temp, _ := strconv.Atoi(strings.Trim(string(out), "\n"))
	// 		diskInfo.Temp = int64(temp)
	// 		diskInfos = append(diskInfos, diskInfo)
	// 	}
	// }
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
