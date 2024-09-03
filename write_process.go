package main

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	}}

// Hub 用于管理所有WebSocket连接
type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *Hub) run() {
	for {
		select {
		case conn := <-h.register:
			h.clients[conn] = true
		case conn := <-h.unregister:
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
			}
		case message := <-h.broadcast:
			for conn := range h.clients {
				if conn != nil {
					err := conn.WriteMessage(websocket.TextMessage, message)
					if err != nil {
						conn.Close()
						delete(h.clients, conn)
					}
				}
			}
		}
	}
}

func test1() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.register <- conn

		defer func() {
			hub.unregister <- conn
			conn.Close()
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	})
	http.Handle("/", http.FileServer(http.Dir("dist")))
	http.ListenAndServe(":8080", nil)
}
