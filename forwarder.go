package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

func forwardRequestToDestinations(r *http.Request, destinations []Destination, defaultDest Destination) (*http.Response, []byte, error) {
	var mu sync.Mutex
	// Read the body once and allow it to be reused
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading request body: %v", err)
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(body)) // Reset the body for reuse

	log.Printf("Original request: Method: %s, URL: %s, Headers: %+v", r.Method, r.URL.String(), r.Header)

	// Use a WaitGroup to synchronize all goroutines
	var wg sync.WaitGroup
	var defaultResponseBody []byte
	var defaultResponse *http.Response

	for _, dest := range destinations {
		wg.Add(1) // Increment the WaitGroup counter for each destination
		go func(destination Destination) {
			defer wg.Done() // Mark this goroutine as done when finished

			// Parse the destination URL
			destURL, err := url.Parse(destination.URL)
			if err != nil {
				log.Printf("Error parsing destination URL %s: %v", destination.URL, err)
				return
			}

			// Construct the forward URL correctly
			forwardURL := *destURL
			if !strings.HasSuffix(forwardURL.Path, "/") && !strings.HasPrefix(r.URL.Path, "/") {
				forwardURL.Path += "/"
			}
			forwardURL.Path = strings.TrimRight(forwardURL.Path, "/") + r.URL.Path // Avoid double slashes
			forwardURL.RawQuery = r.URL.RawQuery

			log.Printf("Original request path: %s", r.URL.Path)
			log.Printf("Destination URL: %s", destURL.String())
			log.Printf("Forwarding to URL: %s", forwardURL.String())

			req, err := http.NewRequest(r.Method, forwardURL.String(), bytes.NewReader(body))
			if err != nil {
				log.Printf("Error creating request for destination %s: %v", destination.URL, err)
				return
			}

			// Copy the headers from the original request
			req.Header = r.Header.Clone()

			// Log the request being forwarded
			log.Printf("Forwarding request to: %s\n", req.URL.String())

			// Forward the request to the destination
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				// Log and broadcast if the destination is unavailable
				log.Printf("Error forwarding to %s: %v", req.URL.String(), err)
				BroadcastTraffic(fmt.Sprintf("Error forwarding to %s: %v", req.URL.String(), err)) // Broadcast error message
				return
			}
			defer resp.Body.Close()

			// If this is the default destination, save the response
			if destination.URL == defaultDest.URL {
				mu.Lock()
				defer mu.Unlock()
				defaultResponseBody, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Printf("Error reading response body from default destination: %v", err)
					return
				}
				defaultResponse = &http.Response{
					Status:        resp.Status,
					StatusCode:    resp.StatusCode,
					Proto:         resp.Proto,
					ProtoMajor:    resp.ProtoMajor,
					ProtoMinor:    resp.ProtoMinor,
					Header:        resp.Header.Clone(),
					Body:          ioutil.NopCloser(bytes.NewReader(defaultResponseBody)),
					ContentLength: int64(len(defaultResponseBody)),
					Request:       resp.Request,
				}
				log.Printf("Response from default destination (%s): Status: %s, Headers: %+v", forwardURL.String(), defaultResponse.Status, defaultResponse.Header)
				log.Printf("Response body from default destination: %s", string(defaultResponseBody))
			}

			// Log and broadcast the forwarded request and response status
			message := fmt.Sprintf("Request forwarded to %s with status: %s", req.URL.String(), resp.Status)
			BroadcastTraffic(message) // Broadcast success message
			log.Println(message)      // Log to console
		}(dest)
	}

	// Wait for all goroutines to finish
	wg.Wait()

	if defaultResponse == nil {
		return nil, nil, fmt.Errorf("no response received from default destination")
	}
	return defaultResponse, defaultResponseBody, nil
}
