package main

import (
	"flag"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// IOCounters(pernic bool)
var wsConnectionCount = 0

type SystemInfo struct {
	CpuInfos []cpu.InfoStat
	HostInfo host.InfoStat
}

type NetInfo struct {
	UpSpeed   int64
	DownSpeed int64
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
	containerInfos, _ := getContainers()
	rwMutex5s.Lock()
	defer rwMutex5s.Unlock()
	*data = Catch5s{5, sensorInfos, diskInfos, containerInfos, temps}
}

func start(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("%s|websocket upgrade error:%s\n", c.Request.RemoteAddr, err)
		return
	}
	if wsConnectionCount < 0 {
		wsConnectionCount = 1
	} else {
		wsConnectionCount++
	}
	conn.SetCloseHandler(func(code int, text string) error {
		wsConnectionCount--
		log.Printf("%s|websocket close: code %d, %s\n", c.Request.RemoteAddr, code, text)
		conn.Close()
		return nil
	})
	go func() {
		for {
			_, _, err := conn.ReadMessage()
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
		err = conn.WriteJSON(catch_1sData)
		rwMutex1s.RUnlock()
		if err != nil {
			log.Printf("%s|websocket write error:%s\n", c.Request.RemoteAddr, err)
			break
		}
		if count_5s%5 == 0 {
			count_5s = 0
			log.Printf("%s|websocket sending\n", c.Request.RemoteAddr)
			rwMutex5s.RLock()
			err = conn.WriteJSON(catch_5sData)
			rwMutex5s.RUnlock()
			if err != nil {
				log.Printf("%s|websocket write error:%s", c.Request.RemoteAddr, err)
				break
			}
		}
		count_5s++
	}
	log.Printf("%s|websocket exit", c.Request.RemoteAddr)
}

func system_info(c *gin.Context) {
	cpuInfos, _ := cpu.Info()
	hostInfo, _ := host.Info()
	c.JSON(http.StatusOK, gin.H{"CpuInfos": cpuInfos, "HostInfo": *hostInfo})
}

func gather1s(oldNetInfos map[string]NetInfo) {
	ticker := time.NewTicker(time.Second)
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

var do = init_dataopt()
var RELEASE = false

func main() {
	var release = flag.Bool("release", false, "release mode")
	flag.Parse()
	RELEASE = *release
	oldNetInfos := make(map[string]NetInfo)
	catch_1s(&oldNetInfos, &catch_1sData)
	catch_5s(&catch_5sData)
	go gather1s(oldNetInfos)
	go gather5s()

	genRsaKey()
	if RELEASE {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	if !RELEASE {
		r.Use(cors.New(cors.Config{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{"*"},
			AllowHeaders: []string{"Authorization", "Content-Type"},
		}))
	}
	//=============================静态文件=============================
	r.Static("/main", "../dist")
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/main")
	})
	//==========================登录和JWT验证===========================
	r.POST("/public_key", getPublicKey)
	r.POST("/login", login)
	r.POST("/auth", jwtAuth)

	//==============================API================================
	api := r.Group("/api")
	api.Use(jwtParseMiddleWare)
	api.GET("/ws", start)
	//======================GET======================
	api.POST("/system_info", system_info)
	api.POST("/navs", do.navs)
	api.POST("/unselected_nets", do.unselected_nets)
	api.POST("/selected_sensors", do.selected_sensors)
	api.POST("/username", do.username)
	api.POST("/cpu_and_mem_temp", do.cpu_and_mem_temp)
	//======================SET======================
	api.POST("/set/nav", do.change_nav)
	api.POST("/set/selected_sensor", do.change_selected_sensor)
	api.POST("/set/unselected_nets", do.change_unselected_nets)
	api.POST("/set/username", do.change_username)
	api.POST("/set/password", do.change_password)
	api.POST("/set/cpu_and_mb_temp", do.change_cpu_and_mb_temp)
	api.POST("/set/switch_nav", do.switch_nav)
	api.POST("/control-container", controlContainer)
	//====================DELETE=====================
	api.POST("/delete/nav", do.delete_nav)
	api.POST("/delete/selected_sensor", do.delete_selected_sensor)
	api.POST("/delete/unselected_net", do.delete_unselected_net)
	//======================ADD======================
	api.POST("/add/nav", do.add_nav)
	api.POST("/add/selected_sensor", do.add_selected_sensor)
	api.POST("/add/unselected_net", do.add_unselected_net)
	api.POST("/add/container", addContainer)

	r.Run("0.0.0.0:8080")
}
