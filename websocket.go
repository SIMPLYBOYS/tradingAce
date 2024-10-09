package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline  = []byte{'\n'}
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now. In production, you might want to restrict this.
		},
	}
)

type WSClient struct {
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]bool
	mutex         sync.RWMutex
}

type WebSocketManager struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mutex      sync.RWMutex
	stop       chan struct{}
	stopOnce   sync.Once
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		stop:       make(chan struct{}),
	}
}

func (manager *WebSocketManager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-manager.register:
			manager.mutex.Lock()
			manager.clients[client] = true
			manager.mutex.Unlock()
			log.Printf("New client registered. Total clients: %d", len(manager.clients))
		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				manager.mutex.Lock()
				delete(manager.clients, client)
				close(client.send)
				manager.mutex.Unlock()
				log.Printf("Client unregistered. Total clients: %d", len(manager.clients))
			}
		case message := <-manager.broadcast:
			manager.broadcastToAll(message)
		}
	}
}

func (manager *WebSocketManager) Stop() {
	manager.stopOnce.Do(func() {
		close(manager.stop)
	})
}

func (manager *WebSocketManager) broadcastToAll(message []byte) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	for client := range manager.clients {
		select {
		case client.send <- message:
		default:
			close(client.send)
			delete(manager.clients, client)
		}
	}
}

func (manager *WebSocketManager) BroadcastToTopic(topic string, message []byte) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	count := 0
	for client := range manager.clients {
		if client.isSubscribed(topic) {
			select {
			case client.send <- message:
				count++
			default:
				go func(c *WSClient) {
					manager.unregister <- c
				}(client)
			}
		}
	}
	log.Printf("Broadcasted message to %d clients subscribed to topic %s", count, topic)
}

func (manager *WebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading to WebSocket:", err)
		return
	}

	client := &WSClient{
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	manager.register <- client

	go client.writePump(manager)
	client.readPump(manager)
}

func (c *WSClient) readPump(manager *WebSocketManager) {
	defer func() {
		manager.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var request map[string]string
		if err := json.Unmarshal(message, &request); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		switch request["action"] {
		case "subscribe":
			c.subscribe(request["topic"])
			log.Printf("Client subscribed to topic: %s", request["topic"])
		case "unsubscribe":
			c.unsubscribe(request["topic"])
			log.Printf("Client unsubscribed from topic: %s", request["topic"])
		default:
			log.Printf("Unknown action: %s", request["action"])
		}
	}
}

func (c *WSClient) writePump(manager *WebSocketManager) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The manager closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *WSClient) subscribe(topic string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.subscriptions[topic] = true
}

func (c *WSClient) unsubscribe(topic string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.subscriptions, topic)
}

func (c *WSClient) isSubscribed(topic string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.subscriptions[topic]
}

func (manager *WebSocketManager) BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) {
	message, err := json.Marshal(map[string]interface{}{
		"type":        "leaderboard_update",
		"leaderboard": leaderboard,
	})
	if err != nil {
		log.Println("Error marshaling leaderboard update:", err)
		return
	}
	log.Printf("Broadcasting leaderboard update to %d clients", len(manager.clients))
	manager.BroadcastToTopic("leaderboard", message)
}

func (manager *WebSocketManager) BroadcastSwapEvent(event *SwapEvent) {
	message, err := json.Marshal(map[string]interface{}{
		"type":  "swap_event",
		"event": event,
	})
	if err != nil {
		log.Println("Error marshaling swap event:", err)
		return
	}
	manager.BroadcastToTopic("swap_events", message)
}

func (manager *WebSocketManager) BroadcastCampaignUpdate(campaignInfo map[string]interface{}) {
	message, err := json.Marshal(map[string]interface{}{
		"type":     "campaign_update",
		"campaign": campaignInfo,
	})
	if err != nil {
		log.Println("Error marshaling campaign update:", err)
		return
	}
	log.Printf("Broadcasting campaign update to clients")
	manager.BroadcastToTopic("campaign", message)
}
