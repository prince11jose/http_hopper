package main

import (
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/mux"
)

// URLNormalizationMiddleware normalizes the URL path
func URLNormalizationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originalPath := r.URL.Path
		r.URL.Path = path.Clean(r.URL.Path)
		r.URL.Path = strings.TrimLeft(r.URL.Path, "/")
		r.URL.Path = "/" + r.URL.Path
		log.Printf("Middleware - URL Normalization: Original: %s, Final: %s", originalPath, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// initializeRoutes sets up the HTTP routes for the application
func initializeRoutes(r *mux.Router) *mux.Router {
	r = r.SkipClean(true)

	// Apply the URL normalization middleware to all routes
	r.Use(URLNormalizationMiddleware)

	// Destination management routes
	r.HandleFunc("/destinations", GetDestinations).Methods("GET")
	r.HandleFunc("/destinations", AddDestination).Methods("POST")
	r.HandleFunc("/destinations/{id}", UpdateDestination).Methods("PUT")
	r.HandleFunc("/destinations/{id}", DeleteDestination).Methods("DELETE")

	// WebSocket traffic monitoring endpoint
	r.HandleFunc("/traffic", StreamTraffic).Methods("GET")

	// Catch-all route for forwarding any request (handles any path, method, etc.)
	r.PathPrefix("/").HandlerFunc(ForwardRequest)

	return r
}
