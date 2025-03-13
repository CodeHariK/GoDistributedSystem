package k7

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Store active WebSocket connections
var (
	clients      = make(map[*websocket.Conn]bool)
	clientsMutex sync.Mutex
)

func (config BenchmarkConfig) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	// Register client
	clientsMutex.Lock()
	clients[conn] = true
	clientsMutex.Unlock()

	config.RunBenchmark()

	// Keep connection open
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			clientsMutex.Lock()
			delete(clients, conn) // Remove closed connection
			clientsMutex.Unlock()
			break
		}
	}
}

// Function to broadcast benchmark results to all WebSocket clients
func broadcastResults(data Bucket) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()

	jsonData, _ := json.Marshal(data)

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, jsonData)
		if err != nil {
			fmt.Println("WebSocket send error:", err)
			client.Close()
			delete(clients, client)
		}
	}
}
