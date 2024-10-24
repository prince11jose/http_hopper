package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Destination struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	URL       string             `bson:"url" json:"url"`
	Method    string             `bson:"method,omitempty" json:"method,omitempty"`
	IsActive  bool               `bson:"isActive" json:"isActive"`
	IsDefault bool               `bson:"isDefault" json:"isDefault"`
}

// WebSocket clients and related variables
var clients = make(map[*websocket.Conn]bool)
var mu sync.Mutex

// Upgrader for WebSocket connections
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for testing; adjust for production
	},
}

// Get all destinations from the database
func GetDestinations(w http.ResponseWriter, r *http.Request) {
	destinations, err := getAllDestinationsFromDB()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting destinations: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(destinations); err != nil {
		http.Error(w, fmt.Sprintf("Error encoding destinations: %v", err), http.StatusInternalServerError)
		return
	}
}

// Add a new destination to the database
func AddDestination(w http.ResponseWriter, r *http.Request) {
	var destination Destination
	if err := json.NewDecoder(r.Body).Decode(&destination); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	addDestinationToDB(destination)
	w.WriteHeader(http.StatusCreated)
}

// Update an existing destination in the database
func UpdateDestination(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var updatedDestination Destination
	if err := json.NewDecoder(r.Body).Decode(&updatedDestination); err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("Updating destination with ID: %s", params["id"])
	updateDestinationInDB(params["id"], updatedDestination)

	if updatedDestination.IsDefault {
		log.Printf("Setting destination %s as default", params["id"])
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Destination updated successfully"})
}

// Delete a destination from the database
func DeleteDestination(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deleteDestinationFromDB(params["id"])
	w.WriteHeader(http.StatusOK)
}

// StreamTraffic handles WebSocket connections for viewing traffic
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
		log.Println("WebSocket client disconnected")
	}()

	// Add the new client to the clients map
	mu.Lock()
	clients[conn] = true
	mu.Unlock()

	log.Println("New WebSocket client connected")

	// Keep reading from the WebSocket to prevent disconnection
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket error: %v", err)
			break // Exit the loop and close the connection on error
		}
	}
}

// BroadcastTraffic sends the traffic information to all connected WebSocket clients
func BroadcastTraffic(message string) {
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

// Forward incoming requests to multiple destinations
func ForwardRequest(w http.ResponseWriter, r *http.Request) {
	log.Printf("ForwardRequest called with: Method: %s, URL: %s, Headers: %+v", r.Method, r.URL.String(), r.Header)

	// Read and log the request body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()                                             // Close the original body
	r.Body = ioutil.NopCloser(strings.NewReader(string(body))) // Recreate the body

	// Log the incoming traffic
	logMessage := fmt.Sprintf("Incoming Request: Method: %s, URL: %s, Body: %s, Headers: %+v",
		r.Method, r.URL.String(), string(body), r.Header)
	log.Println(logMessage)

	// Broadcast the traffic information to WebSocket clients
	BroadcastTraffic(logMessage)

	// Fetch destinations from the database
	destinations, err := getAllDestinationsFromDB()
	if err != nil {
		log.Printf("Error getting destinations: %v", err)
		http.Error(w, fmt.Sprintf("Error getting destinations: %v", err), http.StatusInternalServerError)
		return
	}

	activeDestinations := []Destination{}
	var defaultDestination *Destination
	for _, dest := range destinations {
		log.Printf("Checking destination: %+v", dest)
		if dest.IsActive {
			log.Printf("Destination is active")
			// If a method is specified, only forward if it matches the incoming request's method
			if dest.Method == "" || dest.Method == r.Method {
				log.Printf("Adding destination to active destinations")
				activeDestinations = append(activeDestinations, dest)
				if dest.IsDefault {
					defaultDestination = &dest
					log.Printf("Default destination set: %+v", *defaultDestination)
				}
			} else {
				log.Printf("Destination method does not match request method")
			}
		} else {
			log.Printf("Destination is not active")
		}
	}

	log.Printf("Active destinations: %+v", activeDestinations)
	log.Printf("Default destination: %+v", defaultDestination)

	if len(activeDestinations) == 0 {
		log.Println("No active destinations available for forwarding")
		http.Error(w, "No active destinations available", http.StatusBadGateway)
		return
	}

	if defaultDestination == nil {
		log.Println("No default destination specified")
		http.Error(w, "No default destination specified", http.StatusInternalServerError)
		return
	}

	if defaultDestination.URL == "" {
		log.Println("Default destination URL is empty")
		http.Error(w, "Default destination URL is empty", http.StatusInternalServerError)
		return
	}

	// Construct the full URL for logging
	destURL, err := url.Parse(defaultDestination.URL)
	if err != nil {
		log.Printf("Error parsing default destination URL: %v", err)
		http.Error(w, "Error parsing default destination URL", http.StatusInternalServerError)
		return
	}
	fullURL := *destURL
	if strings.HasSuffix(fullURL.Path, "/") {
		fullURL.Path = fullURL.Path[:len(fullURL.Path)-1]
	}
	fullURL.Path += r.URL.Path
	fullURL.RawQuery = r.URL.RawQuery

	log.Printf("Original request path: %s", r.URL.Path)
	log.Printf("Default destination URL: %s", defaultDestination.URL)
	log.Printf("Constructed full URL: %s", fullURL.String())

	log.Printf("Forwarding request to destinations. Default destination: %+v", *defaultDestination)

	// Call the forwarding logic and get the response from the default destination
	defaultResponse, responseBody, err := forwardRequestToDestinations(r, activeDestinations, *defaultDestination)
	if err != nil {
		log.Printf("Error forwarding request: %v", err)
		http.Error(w, fmt.Sprintf("Error forwarding request: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Response received from forwardRequestToDestinations")

	if defaultResponse.StatusCode == 404 {
		log.Printf("Default destination returned 404. URL: %s, Response: %s", fullURL.String(), string(responseBody))
	}

	log.Printf("Received response from default destination: Status %d, Headers: %+v, Body length %d", defaultResponse.StatusCode, defaultResponse.Header, len(responseBody))
	log.Printf("Response body: %s", string(responseBody))

	// Copy the response from the default destination to the client
	for k, v := range defaultResponse.Header {
		w.Header()[k] = v
		log.Printf("Setting header: %s: %v", k, v)
	}
	w.WriteHeader(defaultResponse.StatusCode)
	_, err = w.Write(responseBody)
	if err != nil {
		log.Printf("Error writing response: %v", err)
	}

	log.Printf("Response sent to client: Status %d, Body length %d", defaultResponse.StatusCode, len(responseBody))
}
