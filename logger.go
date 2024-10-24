package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for testing; adjust for production
	},
}

var clients = make(map[*websocket.Conn]bool)
var mu sync.Mutex

// Broadcast traffic to all connected WebSocket clients
func broadcastTraffic(message string) {
	mu.Lock()
	defer mu.Unlock()
	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			log.Printf("WebSocket error: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// WebSocket handler to stream traffic to clients
func StreamTraffic(w http.ResponseWriter, r *http.Request) {
	// Upgrade the connection from HTTP to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	// Ensure the connection is closed when the function exits
	defer func() {
		conn.Close()
		mu.Lock()
		delete(clients, conn) // Remove the client from the map
		mu.Unlock()
		log.Println("WebSocket client disconnected") // Log client disconnection
	}()

	// Add the new client to the clients map
	mu.Lock()
	clients[conn] = true
	mu.Unlock()

	log.Println("New WebSocket client connected") // Log new connection

	// Read messages from the WebSocket connection (this can be expanded if needed)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error reading message:", err) // Log read errors
			break                                      // Exit the loop on error
		}
	}
}
