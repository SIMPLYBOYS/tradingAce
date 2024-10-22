package websocket

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
	"github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/SIMPLYBOYS/trading_ace/pkg/logger"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Note: Adjust this for production!
	},
}

type WebSocketManager struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mutex      sync.Mutex
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (manager *WebSocketManager) Run() {
	for {
		select {
		case client := <-manager.register:
			manager.mutex.Lock()
			manager.clients[client] = true
			manager.mutex.Unlock()
		case client := <-manager.unregister:
			manager.mutex.Lock()
			if _, ok := manager.clients[client]; ok {
				delete(manager.clients, client)
				client.Close()
			}
			manager.mutex.Unlock()
		case message := <-manager.broadcast:
			manager.mutex.Lock()
			for client := range manager.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					logger.Error("Error broadcasting message: %v", err)
					client.Close()
					delete(manager.clients, client)
				}
			}
			manager.mutex.Unlock()
		}
	}
}

func (manager *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("Failed to upgrade connection: %v", err)
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	manager.register <- conn

	go manager.readPump(conn)
	go manager.writePump(conn)
}

func (manager *WebSocketManager) readPump(conn *websocket.Conn) {
	defer func() {
		manager.unregister <- conn
		conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error { conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error("Unexpected close error: %v", err)
			}
			break
		}
		// Process the message if needed
	}
}

func (manager *WebSocketManager) writePump(conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (manager *WebSocketManager) BroadcastSwapEvent(event *types.SwapEvent) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":  "swap_event",
		"event": event,
	})
	if err != nil {
		return &errors.WebSocketError{Operation: "marshal swap event", Err: err}
	}

	manager.broadcast <- data
	return nil
}

func (manager *WebSocketManager) BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":        "leaderboard_update",
		"leaderboard": leaderboard,
	})
	if err != nil {
		return &errors.WebSocketError{Operation: "marshal leaderboard update", Err: err}
	}

	manager.broadcast <- data
	return nil
}

func (manager *WebSocketManager) BroadcastUserPointsUpdate(address string, points int64) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":    "user_points_update",
		"address": address,
		"points":  points,
	})
	if err != nil {
		return &errors.WebSocketError{Operation: "marshal user points update", Err: err}
	}

	manager.broadcast <- data
	return nil
}
