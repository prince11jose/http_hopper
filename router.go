package main

import (
	"github.com/gorilla/mux"
)

func initializeRoutes(r *mux.Router) {
	// Destination management
	r.HandleFunc("/destinations", GetDestinations).Methods("GET")
	r.HandleFunc("/destinations", AddDestination).Methods("POST")
	r.HandleFunc("/destinations/{id}", UpdateDestination).Methods("PUT")
	r.HandleFunc("/destinations/{id}", DeleteDestination).Methods("DELETE")

	// Forwarding requests - accepts any HTTP method
	r.HandleFunc("/", ForwardRequest)

	// WebSocket traffic monitoring
	r.HandleFunc("/traffic", StreamTraffic).Methods("GET")
}
