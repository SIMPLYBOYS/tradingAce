package websocket

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockWebSocketManager struct {
	clients             map[string]bool
	register            chan string
	unregister          chan string
	broadcast           chan []byte
	RegisteredClients   map[string]bool
	BroadcastedMessages [][]byte
	mutex               sync.Mutex
}

func NewMockWebSocketManager() *MockWebSocketManager {
	return &MockWebSocketManager{
		clients:             make(map[string]bool),
		register:            make(chan string),
		unregister:          make(chan string),
		broadcast:           make(chan []byte),
		RegisteredClients:   make(map[string]bool),
		BroadcastedMessages: [][]byte{},
	}
}

func (m *MockWebSocketManager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mutex.Lock()
			m.clients[client] = true
			m.RegisteredClients[client] = true
			m.mutex.Unlock()
		case client := <-m.unregister:
			m.mutex.Lock()
			delete(m.clients, client)
			delete(m.RegisteredClients, client)
			m.mutex.Unlock()
		case message := <-m.broadcast:
			m.mutex.Lock()
			m.BroadcastedMessages = append(m.BroadcastedMessages, message)
			m.mutex.Unlock()
		}
	}
}

func (m *MockWebSocketManager) BroadcastSwapEvent(event *types.SwapEvent) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":  "swap_event",
		"event": event,
	})
	if err != nil {
		return err
	}
	m.mutex.Lock()
	m.BroadcastedMessages = append(m.BroadcastedMessages, data)
	m.mutex.Unlock()
	return nil
}

func (m *MockWebSocketManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, we would upgrade the connection here
	// For our mock, we'll just generate a unique identifier
	clientID := generateUniqueID()
	m.register <- clientID
}

func (m *MockWebSocketManager) BroadcastLeaderboardUpdate(leaderboard []map[string]interface{}) error {
	data, err := json.Marshal(map[string]interface{}{
		"type":        "leaderboard_update",
		"leaderboard": leaderboard,
	})
	if err != nil {
		return err
	}
	m.mutex.Lock()
	m.BroadcastedMessages = append(m.BroadcastedMessages, data)
	m.mutex.Unlock()
	return nil
}

// Helper function to generate a unique ID (you can implement this as needed)
func generateUniqueID() string {
	return fmt.Sprintf("client-%d", time.Now().UnixNano())
}

func (m *MockWebSocketManager) GetBroadcastedMessages() [][]byte {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.BroadcastedMessages
}

func TestWebSocketManager_Run(t *testing.T) {
	manager := NewMockWebSocketManager()
	go manager.Run()

	// Allow some time for the manager to start
	time.Sleep(100 * time.Millisecond)

	// Test registering a client
	clientID := "testClient"
	manager.register <- clientID

	// Test broadcasting a message
	message := []byte("test message")
	manager.broadcast <- message

	// Test unregistering a client
	manager.unregister <- clientID

	// Allow some time for the operations to complete
	time.Sleep(100 * time.Millisecond)

	// Assert that the client was added and then removed
	manager.mutex.Lock()
	assert.NotContains(t, manager.RegisteredClients, clientID)
	manager.mutex.Unlock()

	// Assert that the message was broadcasted
	assert.Contains(t, manager.GetBroadcastedMessages(), message)
}

func TestWebSocketManager_HandleWebSocket(t *testing.T) {
	manager := NewMockWebSocketManager()
	go manager.Run()

	// Allow some time for the manager to start
	time.Sleep(100 * time.Millisecond)

	// Create a mock http.ResponseWriter and *http.Request
	w := httptest.NewRecorder()
	r, err := http.NewRequest("GET", "/ws", nil)
	require.NoError(t, err)

	// Call HandleWebSocket directly
	manager.HandleWebSocket(w, r)

	// Allow some time for the client to be registered
	time.Sleep(100 * time.Millisecond)

	// Assert that a client was added
	manager.mutex.Lock()
	clientCount := len(manager.RegisteredClients)
	manager.mutex.Unlock()

	assert.Equal(t, 1, clientCount, "Expected one client to be registered")
}

func TestWebSocketManager_BroadcastSwapEvent(t *testing.T) {
	manager := NewMockWebSocketManager()
	go manager.Run()

	// Allow some time for the manager to start
	time.Sleep(100 * time.Millisecond)

	// Simulate a client connection
	clientID := "testClient"
	manager.register <- clientID

	// Allow some time for the client to be registered
	time.Sleep(100 * time.Millisecond)

	// Create a swap event
	event := &types.SwapEvent{
		TxHash:   common.HexToHash("0x123"),
		Sender:   common.HexToAddress("0xabc"),
		USDValue: big.NewFloat(1000.0),
	}

	// Broadcast the event
	err := manager.BroadcastSwapEvent(event)
	require.NoError(t, err)

	// Allow some time for the message to be broadcasted
	time.Sleep(100 * time.Millisecond)

	// Get the broadcasted messages
	messages := manager.GetBroadcastedMessages()

	// Assert that a message was broadcasted
	require.Len(t, messages, 1)

	// Parse the message
	var received map[string]interface{}
	err = json.Unmarshal(messages[0], &received)
	require.NoError(t, err)

	// Assert the message content
	assert.Equal(t, "swap_event", received["type"])
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000123", received["event"].(map[string]interface{})["TxHash"])
	assert.Equal(t, "0x0000000000000000000000000000000000000abc", received["event"].(map[string]interface{})["Sender"])
	assert.Equal(t, "1000", received["event"].(map[string]interface{})["USDValue"])
}

func TestWebSocketManager_BroadcastLeaderboardUpdate(t *testing.T) {
	manager := NewMockWebSocketManager()
	go manager.Run()

	// Allow some time for the manager to start
	time.Sleep(100 * time.Millisecond)

	// Simulate a client connection
	clientID := "testClient"
	manager.register <- clientID

	// Allow some time for the client to be registered
	time.Sleep(100 * time.Millisecond)

	// Create a leaderboard update
	leaderboard := []map[string]interface{}{
		{"address": "0x123", "points": 100},
		{"address": "0x456", "points": 200},
	}

	// Broadcast the leaderboard update
	err := manager.BroadcastLeaderboardUpdate(leaderboard)
	require.NoError(t, err)

	// Allow some time for the message to be broadcasted
	time.Sleep(100 * time.Millisecond)

	// Get the broadcasted messages
	messages := manager.GetBroadcastedMessages()

	// Assert that a message was broadcasted
	require.Len(t, messages, 1)

	// Parse the message
	var received map[string]interface{}
	err = json.Unmarshal(messages[0], &received)
	require.NoError(t, err)

	// Assert the message type
	assert.Equal(t, "leaderboard_update", received["type"])

	// Assert the leaderboard content
	actualLeaderboard, ok := received["leaderboard"].([]interface{})
	require.True(t, ok, "Leaderboard should be a slice")
	require.Len(t, actualLeaderboard, 2, "Leaderboard should have 2 entries")

	for i, expectedEntry := range leaderboard {
		actualEntry, ok := actualLeaderboard[i].(map[string]interface{})
		require.True(t, ok, "Leaderboard entry should be a map")
		assert.Equal(t, expectedEntry["address"], actualEntry["address"])
		assert.Equal(t, float64(expectedEntry["points"].(int)), actualEntry["points"])
	}
}

func TestWebSocketManager_BroadcastUserPointsUpdate(t *testing.T) {
	manager := NewWebSocketManager()
	go manager.Run()

	// Create a test server and client
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.HandleWebSocket(w, r)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Allow some time for the connection to be registered
	time.Sleep(100 * time.Millisecond)

	// Broadcast a user points update
	err = manager.BroadcastUserPointsUpdate("0x123", 100)
	require.NoError(t, err)

	// Read the message from the client
	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	// Parse the message
	var received map[string]interface{}
	err = json.Unmarshal(message, &received)
	require.NoError(t, err)

	// Assert the message content
	assert.Equal(t, "user_points_update", received["type"])
	assert.Equal(t, "0x123", received["address"])
	assert.Equal(t, float64(100), received["points"])
}
