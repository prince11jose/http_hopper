package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func forwardRequestToDestinations(r *http.Request, destinations []Destination) {
	// Read the body once and allow it to be reused
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading request body:", err)
		return
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(body)) // Reset the body for reuse

	for _, dest := range destinations {
		// Run each destination forward in a goroutine
		go func(destination string) {
			// Create a new request with the same method, URL, headers, and body
			req, err := http.NewRequest(r.Method, destination, bytes.NewReader(body))
			if err != nil {
				log.Println("Error creating request:", err)
				return
			}
			// Copy the headers from the original request
			req.Header = r.Header

			// Log the request being forwarded
			log.Printf("Forwarding request to: %s\n", destination)

			// Forward the request to the destination
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Println("Error forwarding to:", destination, err)
				return
			}
			defer resp.Body.Close()

			// Log and broadcast the forwarded request and response status
			message := fmt.Sprintf("Request forwarded to %s with status: %s", destination, resp.Status)
			log.Println(message) // Logs to the file or console
			broadcastTraffic(message)
		}(dest.URL)
	}
}
