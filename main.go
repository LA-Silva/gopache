package main

import (
	"fmt"
	"log"
	"net/http"
)

// handler function processes incoming requests
func handler(w http.ResponseWriter, r *http.Request) {
	// Print the path requested by the client to the server's console
	fmt.Printf("Received request for path: %s\n", r.URL.Path)

	// Write the response "Hello, World!" back to the client
	// fmt.Fprintf is like fmt.Printf but writes to an io.Writer (w in this case)
	fmt.Fprintf(w, "Hello, World!")
}

func main() {
	// Register the handler function for the root URL path "/"
	// All requests to the root path will be handled by the 'handler' function
	http.HandleFunc("/", handler)

	// Define the port the server will listen on
	port := ":8080"
	fmt.Printf("Starting server on http://localhost%s\n", port)

	// Start the HTTP server on the specified port
	// ListenAndServe blocks until the server is stopped or an error occurs
	// If the second argument (handler) is nil, it uses http.DefaultServeMux,
	// which is where http.HandleFunc registers handlers.
	err := http.ListenAndServe(port, nil)
	if err != nil {
		// If the server fails to start (e.g., port is already in use), log the error and exit
		log.Fatal("ListenAndServe: ", err)
	}
}
