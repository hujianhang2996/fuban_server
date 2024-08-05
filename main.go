package main

import (
	"flag"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", "0.0.0.0:8080", "http service address")

type FanInfo struct {
	Name  string
	Speed int64
}

type DiskInfo struct {
	Name      string
	Temp      int64
	TotalSize int64
	UsedSize  int64
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
	MemLoad      int64
	NetUpSpeed   int64
	NetDownSpeed int64
}

type Catch5s struct {
	Type       uint
	CpuTemp    int64
	MbTemp     int64
	Fans       []FanInfo
	Disks      []DiskInfo
	Containers []ContainerInfo
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

func catch_1s() Catch1s {
	return Catch1s{1, 1 + int64(rand.Intn(70)), 18 + int64(rand.Intn(6)), 2097152 + int64(rand.Intn(102400)), 1048576 + int64(rand.Intn(102400))}
}
func catch_5s() Catch5s {
	return Catch5s{5, 35 + int64(rand.Intn(5)), 32 + int64(rand.Intn(5)),
		[]FanInfo{FanInfo{"fan1", 1500 + int64(rand.Intn(300))}, FanInfo{"fan2", 1400 + int64(rand.Intn(300))}},
		[]DiskInfo{
			DiskInfo{"WDC_WD40EZRZ-00GXCB0_PL1331LAH3832H", 38 + int64(rand.Intn(5)), 3905110812000, 1634428668000},
			DiskInfo{"WDC_WD40EZRZ-00GXCB0_PL2331LAH0NP5J", 38 + int64(rand.Intn(5)), 3905110812000, 1134428668000},
			DiskInfo{"ST4000VX007_ZA4M1D1O", 38 + int64(rand.Intn(5)), 3905110812000, 634428668000},
			DiskInfo{"ST1000VM002-1ET162_S5131DBZ", 38 + int64(rand.Intn(5)), 3905110812000, 2534428668000},
		},
		[]ContainerInfo{
			ContainerInfo{"emby", 1, true, "8096:8096", "/mnt:/olympos", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"nas-tools", 1, false, "3000:3000", "/config:/mnt/user/appdata/nas-tools", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"GoStatic", 0, false, "8043:8043", "/olympos:/mnt", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"aria2-pro", 1, false, "6800:6800,6888:6888", "/downloads:/mnt/user/downloads", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
			ContainerInfo{"prowlarr", 1, false, "9696:9696", "/config:/mnt/user/appdata/prowlarr", 0.1 + rand.Float64()*0.02, 3 + rand.Float64()},
		}}
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
	for t := range ticker.C {
		log.Printf("%s|%v\n", r.RemoteAddr, t.UTC().Local().Format("2006-01-02-15:04:05"))
		// err = c.WriteMessage(1, []byte("1s msg: "+t.UTC().Local().Format("2006-01-02-15:04:05")))
		err = c.WriteJSON(catch_1s())
		if err != nil {
			log.Println(r.RemoteAddr, "|websocket write error:", err)
			break
		}
		if count_5s%5 == 0 {
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

func home(w http.ResponseWriter, r *http.Request) {
	homeTemplate.Execute(w, "ws://"+r.Host+"/start")
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/start", start)
	http.Handle("/", http.FileServer(http.Dir("dist")))
	// http.HandleFunc("/", home)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>  
window.addEventListener("load", function(evt) {

    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;

    var print = function(message) {
        var d = document.createElement("div");
        d.textContent = message;
        output.appendChild(d);
        output.scroll(0, output.scrollHeight);
    };

    document.getElementById("open").onclick = function(evt) {
        if (ws) {
            return false;
        }
        ws = new WebSocket("{{.}}");
        ws.onopen = function(evt) {
            print("OPEN");
        }
        ws.onclose = function(evt) {
            print("CLOSE");
            ws = null;
        }
        ws.onmessage = function(evt) {
            print(evt.data);
        }
        ws.onerror = function(evt) {
            print("ERROR: " + evt.data);
        }
        return false;
    };

    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        print("SEND: " + input.value);
        ws.send(input.value);
        return false;
    };

    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close(1000, "close");
        return false;
    };

});
</script>
</head>
<body>
<form style="margin-bottom: 2px">
<button id="open">Open</button>
<button id="close">Close</button>
<input id="input" type="text" value="Hello world!">
<button id="send">Send</button>
</form>
<div id="output" style="max-height: 93vh;overflow-y: scroll"></div>
</body>
</html>
`))
