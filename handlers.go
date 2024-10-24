package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Destination struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	URL      string             `bson:"url" json:"url"`
	Method   string             `bson:"method,omitempty" json:"method,omitempty"`
	IsActive bool               `bson:"isActive" json:"isActive"`
}

// Get all destinations
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

// Add a new destination
func AddDestination(w http.ResponseWriter, r *http.Request) {
	var destination Destination
	_ = json.NewDecoder(r.Body).Decode(&destination)
	addDestinationToDB(destination)
	w.WriteHeader(http.StatusCreated)
}

// Update an existing destination
func UpdateDestination(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var updatedDestination Destination
	err := json.NewDecoder(r.Body).Decode(&updatedDestination)
	if err != nil {
		log.Printf("Error decoding request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Updating destination with ID: %s", params["id"])
	updateDestinationInDB(params["id"], updatedDestination)
	w.WriteHeader(http.StatusOK)
}

// Delete a destination
func DeleteDestination(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	deleteDestinationFromDB(params["id"])
	w.WriteHeader(http.StatusOK)
}

// Forward incoming requests to multiple destinations
func ForwardRequest(w http.ResponseWriter, r *http.Request) {
	destinations, err := getAllDestinationsFromDB()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting destinations: %v", err), http.StatusInternalServerError)
		return
	}

	activeDestinations := []Destination{}
	for _, dest := range destinations {
		if dest.IsActive {
			// If a method is specified, only forward if it matches the incoming request's method
			if dest.Method == "" || dest.Method == r.Method {
				activeDestinations = append(activeDestinations, dest)
			}
		}
	}

	// Forward the request to the filtered (active) destinations
	for _, dest := range activeDestinations {
		// Log the forwarding action
		logMessage := fmt.Sprintf("Forwarding request to %s with method %s", dest.URL, r.Method)
		log.Println(logMessage)

		// Broadcast to WebSocket clients
		broadcastTraffic(logMessage)

		// Actual forwarding logic here...
	}

	w.WriteHeader(http.StatusOK)
}
