package main

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// IOCounters(pernic bool)
var addr = flag.String("addr", "0.0.0.0:8080", "http service address")

type SystemInfo struct {
	CpuInfos []cpu.InfoStat
	HostInfo host.InfoStat
}

type NetInfo struct {
	UpSpeed   int64
	DownSpeed int64
}

type ContainerInfo struct {
	Name    string
	Status  int64
	HostNet bool
	Port    string
	Volume  string
	CpuLoad float64
	MemLoad float64
}

type Catch1s struct {
	Type         uint
	CpuLoad      int64
	CpuCoreLoads []int64
	MemTotal     int64
	MemUsed      int64
	NetInfos     map[string]NetInfo
	Uptime       int64
}

type Catch5s struct {
	Type       uint
	Sensors    []SensorInfo
	Disks      []DiskInfo
	Containers []ContainerInfo
	Temps      map[string]TemperatureStat
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

func catch_1s(oldNetInfos *map[string]NetInfo) Catch1s {
	percent, _ := cpu.Percent(0, true)
	t := 0.0
	n := 0
	percentInt := []int64{}
	for _, p := range percent {
		t += p
		n += 1
		percentInt = append(percentInt, int64(math.Round(p)))
	}
	ioCounters, _ := net.IOCounters(true)
	netInfos := make(map[string]NetInfo)
	for _, i := range ioCounters {
		v, ok := (*oldNetInfos)[i.Name]
		if ok {
			netInfos[i.Name] = NetInfo{int64(i.BytesSent) - v.UpSpeed, int64(i.BytesRecv) - v.DownSpeed}
		} else {
			netInfos[i.Name] = NetInfo{-1, -1}
		}
		(*oldNetInfos)[i.Name] = NetInfo{int64(i.BytesSent), int64(i.BytesRecv)}
	}

	memStat, _ := mem.VirtualMemory()
	uptime, _ := host.Uptime()
	return Catch1s{1, int64(math.Round(t / float64(n))), percentInt,
		int64(memStat.Total), int64(memStat.Total - memStat.Available), netInfos, int64(uptime)}
}
func catch_5s() Catch5s {
	// diskInfos := []DiskInfo{}
	// partitions, _ := disk.Partitions(false)
	// counterStat, _ := disk.IOCounters("")
	// for _, partition := range partitions {
	// 	usageStat, _ := disk.Usage(partition.Mountpoint)
	// 	diskInfos = append(diskInfos, DiskInfo{partition.Device, -1, int64(usageStat.Total), int64(usageStat.Used),
	// 		int64(counterStat[partition.Device].ReadCount), int64(counterStat[partition.Device].WriteCount)})
	// }
	diskInfos, _ := GetDiskInfos([]string{}, []string{}, []string{"tmpfs", "devtmpfs", "devfs", "iso9660", "overlay", "aufs", "squashfs"})
	temps, _ := GetTemps()
	sensorInfos, _ := GetSensorInfos()
	return Catch5s{5,
		sensorInfos,
		// []DiskInfo{
		// 	DiskInfo{"WDC_WD40EZRZ-00GXCB0_PL1331LAH3832H", 38 + int64(rand.Intn(5)), 3905110812000, 1634428668000},
		// 	DiskInfo{"WDC_WD40EZRZ-00GXCB0_PL2331LAH0NP5J", 38 + int64(rand.Intn(5)), 3905110812000, 1134428668000},
		// 	DiskInfo{"ST4000VX007_ZA4M1D1O", 38 + int64(rand.Intn(5)), 3905110812000, 634428668000},
		// 	DiskInfo{"ST1000VM002-1ET162_S5131DBZ", 38 + int64(rand.Intn(5)), 3905110812000, 2534428668000},
		// },
		diskInfos,
		[]ContainerInfo{
			ContainerInfo{"emby", 1, true, "8096:8096", "/mnt:/olympos", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"nas-tools", 1, false, "3000:3000", "/config:/mnt/user/appdata/nas-tools", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"GoStatic", 0, false, "8043:8043", "/olympos:/mnt", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"aria2-pro", 1, false, "6800:6800,6888:6888", "/downloads:/mnt/user/downloads", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"prowlarr", 1, false, "9696:9696", "/config:/mnt/user/appdata/prowlarr", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
		}, temps}
}

func start(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(r.RemoteAddr, "|websocket upgrade error:", err)
		return
	}
	c.SetCloseHandler(func(code int, text string) error {
		log.Printf("%s|websocket close: code %d, %s", r.RemoteAddr, code, text)
		c.Close()
		return nil
	})
	go func() {
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	count_5s := 0
	oldNetInfos := make(map[string]NetInfo)
	for t := range ticker.C {
		err = c.WriteJSON(catch_1s(&oldNetInfos))
		if err != nil {
			log.Println(r.RemoteAddr, "|websocket write error:", err)
			break
		}
		if count_5s%5 == 0 {
			log.Printf("%s | %v\n", r.RemoteAddr, t.UTC().Local().Format("2006-01-02-15:04:05"))
			count_5s = 0
			err = c.WriteJSON(catch_5s())
			if err != nil {
				log.Println(r.RemoteAddr, "|websocket write error:", err)
				break
			}
		}
		count_5s++
	}
	log.Println(r.RemoteAddr, "|websocket exit")
}

func system_info(w http.ResponseWriter, r *http.Request) {
	cpuInfos, _ := cpu.Info()
	hostInfo, _ := host.Info()
	systemInfoJson, _ := json.Marshal(SystemInfo{cpuInfos, *hostInfo})
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.Write(systemInfoJson)
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/start", start)
	http.HandleFunc("/system_info", system_info)
	http.Handle("/", http.FileServer(http.Dir("dist")))
	log.Fatal(http.ListenAndServe(*addr, nil))
}
