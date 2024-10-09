package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestWebSocketConnection(t *testing.T) {
	manager := NewWebSocketManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager.Run(ctx)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.HandleWebSocket(w, r)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect to the server
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	defer ws.Close()

	// Test subscription
	err = ws.WriteJSON(map[string]string{"action": "subscribe", "topic": "leaderboard"})
	assert.NoError(t, err)

	// Wait for subscription to be processed
	time.Sleep(100 * time.Millisecond)

	// Broadcast a leaderboard update
	leaderboard := []map[string]interface{}{
		{"address": "0x1234", "points": 100},
		{"address": "0x5678", "points": 200},
	}
	manager.BroadcastLeaderboardUpdate(leaderboard)

	// Read the message from the WebSocket with retries
	var received map[string]interface{}
	for i := 0; i < 5; i++ { // Try up to 5 times
		ws.SetReadDeadline(time.Now().Add(time.Second))
		_, message, err := ws.ReadMessage()
		if err != nil {
			t.Logf("Read attempt %d failed: %v", i+1, err)
			time.Sleep(time.Millisecond * 100) // Wait a bit before retrying
			continue
		}

		err = json.Unmarshal(message, &received)
		if err != nil {
			t.Logf("Unmarshal attempt %d failed: %v", i+1, err)
			continue
		}

		// If we successfully read and unmarshaled the message, break the loop
		break
	}

	// Assert the received message
	assert.NotNil(t, received, "No message received after multiple attempts")
	if received != nil {
		assert.Equal(t, "leaderboard_update", received["type"], "Unexpected message type")

		receivedLeaderboard, ok := received["leaderboard"].([]interface{})
		assert.True(t, ok, "Leaderboard is not of type []interface{}")

		assert.Equal(t, len(leaderboard), len(receivedLeaderboard), "Leaderboard length mismatch")

		for i, item := range receivedLeaderboard {
			receivedItem, ok := item.(map[string]interface{})
			assert.True(t, ok, "Leaderboard item is not of type map[string]interface{}")
			assert.Equal(t, leaderboard[i]["address"], receivedItem["address"], "Address mismatch")

			expectedPoints, ok := leaderboard[i]["points"].(int)
			assert.True(t, ok, "Expected points is not of type int")
			receivedPoints, ok := receivedItem["points"].(float64)
			assert.True(t, ok, "Received points is not of type float64")
			assert.InDelta(t, float64(expectedPoints), receivedPoints, 0.001, "Points mismatch")
		}
	}

	// Clean up
	cancel()
	wg.Wait()
}

func TestWebSocketMultipleClients(t *testing.T) {
	manager := NewWebSocketManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager.Run(ctx)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.HandleWebSocket(w, r)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect two clients
	ws1, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	defer ws1.Close()

	ws2, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	defer ws2.Close()

	// Subscribe clients to different topics
	err = ws1.WriteJSON(map[string]string{"action": "subscribe", "topic": "leaderboard"})
	assert.NoError(t, err)

	err = ws2.WriteJSON(map[string]string{"action": "subscribe", "topic": "swap_events"})
	assert.NoError(t, err)

	// Wait for subscriptions to be processed
	time.Sleep(100 * time.Millisecond)

	// Broadcast messages
	leaderboard := []map[string]interface{}{{"address": "0x1234", "points": 100}}
	manager.BroadcastLeaderboardUpdate(leaderboard)

	swapEvent := &SwapEvent{
		Sender:     common.HexToAddress("0x5678"),
		Amount0In:  big.NewInt(100),
		Amount1In:  big.NewInt(0),
		Amount0Out: big.NewInt(0),
		Amount1Out: big.NewInt(200),
		To:         common.HexToAddress("0x9ABC"),
	}
	manager.BroadcastSwapEvent(swapEvent)

	// Function to read message with retry
	readMessageWithRetry := func(ws *websocket.Conn, expectedType string) (map[string]interface{}, error) {
		var received map[string]interface{}
		for i := 0; i < 5; i++ {
			ws.SetReadDeadline(time.Now().Add(time.Second))
			_, message, err := ws.ReadMessage()
			if err != nil {
				t.Logf("Read attempt %d failed: %v", i+1, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			err = json.Unmarshal(message, &received)
			if err != nil {
				t.Logf("Unmarshal attempt %d failed: %v", i+1, err)
				continue
			}

			if received["type"] == expectedType {
				return received, nil
			}
		}
		return nil, fmt.Errorf("failed to receive expected message type: %s", expectedType)
	}

	// Check that each client receives only the subscribed message
	received1, err := readMessageWithRetry(ws1, "leaderboard_update")
	assert.NoError(t, err)
	assert.Equal(t, "leaderboard_update", received1["type"])

	received2, err := readMessageWithRetry(ws2, "swap_event")
	assert.NoError(t, err)
	assert.Equal(t, "swap_event", received2["type"])

	// Clean up
	cancel()
	wg.Wait()
}

func TestWebSocketUnsubscribe(t *testing.T) {
	manager := NewWebSocketManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager.Run(ctx)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.HandleWebSocket(w, r)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	defer ws.Close()

	// Subscribe to a topic
	err = ws.WriteJSON(map[string]string{"action": "subscribe", "topic": "leaderboard"})
	assert.NoError(t, err)

	// Wait a bit to ensure the subscription is processed
	time.Sleep(100 * time.Millisecond)

	// Broadcast a message (should receive this)
	leaderboard1 := []map[string]interface{}{{"address": "0x1234", "points": 100}}
	manager.BroadcastLeaderboardUpdate(leaderboard1)

	// Read the message (should succeed)
	ws.SetReadDeadline(time.Now().Add(time.Second))
	_, message1, err := ws.ReadMessage()
	assert.NoError(t, err, "Expected to receive message while subscribed")
	assert.NotNil(t, message1, "Expected non-nil message while subscribed")

	// Unsubscribe from the topic
	err = ws.WriteJSON(map[string]string{"action": "unsubscribe", "topic": "leaderboard"})
	assert.NoError(t, err)

	// Wait a bit to ensure the unsubscribe action is processed
	time.Sleep(100 * time.Millisecond)

	// Broadcast another message (should not receive this)
	leaderboard2 := []map[string]interface{}{{"address": "0x5678", "points": 200}}
	manager.BroadcastLeaderboardUpdate(leaderboard2)

	// Try to read a message (should timeout)
	ws.SetReadDeadline(time.Now().Add(time.Second))
	_, message2, err := ws.ReadMessage()
	assert.Error(t, err, "Expected error (timeout) after unsubscribing")
	assert.Nil(t, message2, "Expected nil message after unsubscribing")

	// Clean up
	cancel()
	wg.Wait()
}

func TestWebSocketBroadcastCampaignUpdate(t *testing.T) {
	manager := NewWebSocketManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		manager.Run(ctx)
	}()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.HandleWebSocket(w, r)
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	assert.NoError(t, err)
	defer ws.Close()

	// Subscribe to campaign updates
	err = ws.WriteJSON(map[string]string{"action": "subscribe", "topic": "campaign"})
	assert.NoError(t, err)

	// Wait for subscription to be processed
	time.Sleep(100 * time.Millisecond)

	// Broadcast a campaign update
	startTime := time.Now().Unix()
	endTime := time.Now().Add(28 * 24 * time.Hour).Unix()
	campaignInfo := map[string]interface{}{
		"start_time": startTime,
		"end_time":   endTime,
		"is_active":  true,
	}
	manager.BroadcastCampaignUpdate(campaignInfo)

	// Function to read message with retry
	readMessageWithRetry := func(ws *websocket.Conn, expectedType string) (map[string]interface{}, error) {
		var received map[string]interface{}
		for i := 0; i < 5; i++ {
			ws.SetReadDeadline(time.Now().Add(time.Second))
			_, message, err := ws.ReadMessage()
			if err != nil {
				t.Logf("Read attempt %d failed: %v", i+1, err)
				time.Sleep(time.Millisecond * 100)
				continue
			}

			err = json.Unmarshal(message, &received)
			if err != nil {
				t.Logf("Unmarshal attempt %d failed: %v", i+1, err)
				continue
			}

			if received["type"] == expectedType {
				return received, nil
			}
		}
		return nil, fmt.Errorf("failed to receive expected message type: %s", expectedType)
	}

	// Read the message from the WebSocket
	received, err := readMessageWithRetry(ws, "campaign_update")
	assert.NoError(t, err)

	assert.Equal(t, "campaign_update", received["type"])

	receivedCampaign, ok := received["campaign"].(map[string]interface{})
	assert.True(t, ok, "Received campaign is not of type map[string]interface{}")

	assert.Equal(t, campaignInfo["is_active"], receivedCampaign["is_active"])

	// Check start_time and end_time with a small delta to account for float conversion
	assert.InDelta(t, float64(startTime), receivedCampaign["start_time"], 1.0, "start_time mismatch")
	assert.InDelta(t, float64(endTime), receivedCampaign["end_time"], 1.0, "end_time mismatch")

	// Clean up
	cancel()
	wg.Wait()
}
