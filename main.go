package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
    "net/http/httputil" // Import for DumpRequest
    "os"
    "path/filepath"
    "strings"
    "bufio"
    "strconv"
)

// handler function processes incoming requests
func handler(w http.ResponseWriter, r *http.Request) {
    // Print the path requested by the client to the server's console
    fmt.Printf("Received request for path: %s\n", r.URL.Path)

    // Dump the entire request
    requestDump, err := httputil.DumpRequest(r, true) // true for body
    if err != nil {
	fmt.Printf("Error dumping request: %v\n", err)
    }
    fmt.Printf("Request:\n%s\n", requestDump)

    // Build a string containing all the headers
    var headerString strings.Builder
    for key, values := range r.Header {
	headerString.WriteString(fmt.Sprintf("%s: %s\n", key, strings.Join(values, ", ")))
    }

    // Write the headers back to console
    fmt.Printf( "Request Headers:\n%s", headerString.String())


    // List of file extensions to serve directly
    extensions := []string{".jpg", ".html", ".gif", ".htm", ".jpeg"}

    // Check if the requested path has a matching extension
    for _, ext := range extensions {
	if strings.HasSuffix(r.URL.Path, ext) {
	    // Construct the file path
	    filePath := filepath.Join("public_html", r.URL.Path)

	    // Check if the file exists
	    file, err := os.Open(filePath)
	    if err != nil {
		if os.IsNotExist(err) {
		    // File not found, send a 404 response
		    http.NotFound(w, r)
		    fmt.Printf("File not found: %s\n", filePath)
		    return
		}
		// Other error opening file, send a 500 response
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Printf("Error opening file: %s, error: %v\n", filePath, err)
		return
	    }
	    defer file.Close()

	    // Set the content type based on the file extension.  This is crucial!
	    contentType := getContentType(ext)
	    w.Header().Set("Content-Type", contentType)

	    // Copy the file content to the response writer
	    _, err = io.Copy(w, file)
	    if err != nil {
		// Error copying file, send a 500 response
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Printf("Error copying file: %s, error: %v\n", filePath, err)
		return
	    }
	    // File served successfully, return from the handler.
	    return
	}
    }

    // If the request is not for a specific file, return a default message.
    fmt.Fprintf(w, "Hello, World!  This is the default response.\n")

}

// getContentType returns the correct content type for the given file extension
func getContentType(ext string) string {
    switch ext {
    case ".html", ".htm":
	return "text/html; charset=utf-8"
    case ".jpg", ".jpeg":
	return "image/jpeg"
    case ".gif":
	return "image/gif"
    default:
	return "application/octet-stream" // Default content type
    }
}

// readPortFromFile reads the port number from the httpd.conf file.
func readPortFromFile(filename string) (string, error) {
    file, err := os.Open(filename)
    if err != nil {
	return "", err
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
	line := scanner.Text()
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "Listen") {
	    parts := strings.Split(line, " ")
	    if len(parts) > 1 {
		portStr := strings.TrimSpace(parts[1])
		_, err := strconv.Atoi(portStr)
		if err == nil {
		    return ":" + portStr, nil
		} else {
		    return "", fmt.Errorf("invalid port format in config file: %s", portStr)
		}
	    }
	}
    }

    if err := scanner.Err(); err != nil {
	return "", err
    }

    return "", fmt.Errorf("port not found in config file")
}

func main() {
    // Define the default port
    defaultPort := ":8080"
    port := defaultPort

    // Read the port number from the configuration file
    filename := "httpd.conf"
    portFromFile, err := readPortFromFile(filename)
    if err != nil {
	// Log the error and use the default port
	log.Printf("Error reading port from %s: %v, using default port %s\n", filename, err, defaultPort)
    } else {
	port = portFromFile
	log.Printf("Using port from %s: %s\n", filename, port)
    }

    // Register the handler function for the root URL path "/"
    http.HandleFunc("/", handler)

    fmt.Printf("Starting server on http://localhost%s\n", port)

    // Start the HTTP server on the specified port
    err = http.ListenAndServe(port, nil)
    if err != nil {
	log.Fatal("ListenAndServe: ", err)
    }
}
