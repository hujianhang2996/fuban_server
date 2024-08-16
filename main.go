package main

/*
#include "hddtemp.h"
*/
import "C"

import (
	"encoding/json"
	"flag"
	"log"
	"math"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

func GoHddtemp(device string) int64 {
	temp := C.hddtemp(C.CString(device))
	return int64(temp)
}

// IOCounters(pernic bool)
var addr = flag.String("addr", "0.0.0.0:8080", "http service address")
var wsConnectionCount = 0

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

var catch_1sData Catch1s

type Catch5s struct {
	Type       uint
	Sensors    []SensorInfo
	Disks      []DiskInfo
	Containers []ContainerInfo
	Temps      map[string]TemperatureStat
}

var catch_5sData Catch5s

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

var rwMutex1s sync.RWMutex
var rwMutex5s sync.RWMutex

func catch_1s(oldNetInfos *map[string]NetInfo, data *Catch1s) {
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
	rwMutex1s.Lock()
	defer rwMutex1s.Unlock()
	*data = Catch1s{1, int64(math.Round(t / float64(n))), percentInt,
		int64(memStat.Total), int64(memStat.Total - memStat.Available), netInfos, int64(uptime)}
}
func catch_5s(data *Catch5s) {
	// diskInfos := []DiskInfo{}
	// partitions, _ := disk.Partitions(false)
	// counterStat, _ := disk.IOCounters("")
	// for _, partition := range partitions {
	// 	usageStat, _ := disk.Usage(partition.Mountpoint)
	// 	diskInfos = append(diskInfos, DiskInfo{partition.Device, -1, int64(usageStat.Total), int64(usageStat.Used),
	// 		int64(counterStat[partition.Device].ReadCount), int64(counterStat[partition.Device].WriteCount)})
	// }
	diskInfos, _ := GetDiskInfosUnraid()
	temps, _ := GetTemps()
	sensorInfos, _ := GetSensorInfos()
	rwMutex5s.Lock()
	defer rwMutex5s.Unlock()
	*data = Catch5s{5,
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
		log.Printf("%s|websocket upgrade error:%s\n", r.RemoteAddr, err)
		return
	}
	if wsConnectionCount < 0 {
		wsConnectionCount = 1
	} else {
		wsConnectionCount++
	}
	c.SetCloseHandler(func(code int, text string) error {
		wsConnectionCount--
		log.Printf("%s|websocket close: code %d, %s\n", r.RemoteAddr, code, text)
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
	// oldNetInfos := make(map[string]NetInfo)
	for range ticker.C {
		// err = c.WriteJSON(catch_1s(&oldNetInfos))
		rwMutex1s.RLock()
		err = c.WriteJSON(catch_1sData)
		rwMutex1s.RUnlock()
		if err != nil {
			log.Printf("%s|websocket write error:%s\n", r.RemoteAddr, err)
			break
		}
		if count_5s%5 == 0 {
			count_5s = 0
			log.Printf("%s|websocket sending\n", r.RemoteAddr)
			rwMutex5s.RLock()
			err = c.WriteJSON(catch_5sData)
			rwMutex5s.RUnlock()
			if err != nil {
				log.Printf("%s|websocket write error:%s", r.RemoteAddr, err)
				break
			}
		}
		count_5s++
	}
	log.Printf("%s|websocket exit", r.RemoteAddr)
}

func system_info(w http.ResponseWriter, r *http.Request) {
	cpuInfos, _ := cpu.Info()
	hostInfo, _ := host.Info()
	systemInfoJson, _ := json.Marshal(SystemInfo{cpuInfos, *hostInfo})
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	w.Write(systemInfoJson)
}

func gather1s() {
	ticker := time.NewTicker(time.Second)
	oldNetInfos := make(map[string]NetInfo)
	pause := false
	for range ticker.C {
		if wsConnectionCount <= 0 && !pause {
			pause = true
		}
		if wsConnectionCount > 0 && pause {
			pause = false
			oldNetInfos = make(map[string]NetInfo)
		}
		if pause {
			continue
		}
		catch_1s(&oldNetInfos, &catch_1sData)
	}
	log.Println("gather 1s exit")
}

func gather5s() {
	ticker := time.NewTicker(time.Second * 5)
	pause := false
	for range ticker.C {
		if wsConnectionCount <= 0 && !pause {
			pause = true
		}
		if wsConnectionCount > 0 && pause {
			pause = false
		}
		if pause {
			continue
		}
		catch_5s(&catch_5sData)
	}
	log.Println("gather 5s exit")
}

func main() {
	// GetDiskInfosUnraid()
	flag.Parse()
	http.HandleFunc("/start", start)
	http.HandleFunc("/system_info", system_info)
	http.Handle("/", http.FileServer(http.Dir("dist")))
	go gather1s()
	go gather5s()
	log.Fatal(http.ListenAndServe(*addr, nil))
	// log.Print(GoHddtemp("/dev/sdb"))
	// log.Print(GoHddtemp("/dev/sdc"))
	// log.Print(GoHddtemp("/dev/sdd"))
	// log.Print(GoHddtemp("/dev/sde"))
}
