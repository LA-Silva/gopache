package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Global variable for the server, so we can shut it down.
var server *http.Server
var serverWaitGroup sync.WaitGroup
var isRunning bool // Track server state.  Good practice.
var documentRoot string												  

const webserver = "gopache"

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
	fmt.Printf("Request Headers:\n%s", headerString.String())

	// Check for the /stop command
	if r.URL.Path == "/stop" && r.Host == "localhost:8080" { // IMPORTANT: Check Host
		fmt.Println("Received /stop request. Shutting down server...")
		w.WriteHeader(http.StatusOK) // Send a 200 OK response before shutting down
		w.Write([]byte("Server is shutting down..."))
		// Use a goroutine to shut down the server gracefully.
		go func() {
			// Create a context with a timeout.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Call the server's Shutdown method with the context.
			if err := server.Shutdown(ctx); err != nil {
				log.Printf("Server shutdown error: %v\n", err)
			}
			// DO NOT CALL serverWaitGroup.Done() HERE.
			// The main server goroutine will handle signaling the WaitGroup.
			isRunning = false // Update server state
		}()
		return // IMPORTANT:  Return from the handler!  Don't try to write "Hello, World!"
	}

	// Check if the request is for a CGI script
	if strings.HasPrefix(r.URL.Path, "/cgi-bin/") {
		executeCGI(w, r)
		return // IMPORTANT: Return after CGI execution
	}

	// List of file extensions to serve directly
	extensions := []string{".jpg", ".html", ".gif", ".htm", ".jpeg", ".css", ".js", ".png"} // Add more extensions

	// Check if the requested path has a matching extension
	for _, ext := range extensions {
		if strings.HasSuffix(r.URL.Path, ext) {
			// Construct the file path using the documentRoot
			filePath := filepath.Join(documentRoot, r.URL.Path) // Use documentRoot

			// Check if the file exists
			file, err := os.Open(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					// File not found, send a 404 response
					http.NotFound(w, r)
					fmt.Printf("File not found: %s documentRoot: %s\n", filePath, documentRoot)
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
	case ".css": //added css
		return "text/css; charset=utf-8"
	case ".js": //added js
		return "application/javascript; charset=utf-8"
	case ".png":
		return "image/png"
	default:
		return "application/octet-stream" // Default content type
	}
}

// readConfigFile reads the port number and other settings from the httpd.conf file.
func readConfigFile(filename string) (string, string, string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	port := ""
	pidFile := ""
	lDocumentRoot := ""
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " ", 2) // Split only once
		if len(parts) < 2 {
			continue // Skip lines without a key and value
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Listen":
			_, err := strconv.Atoi(value)
			if err != nil {
				return "", "", "", fmt.Errorf("invalid port format in config file: %s", value)
			}
			port = ":" + value
		case "DocumentRoot":
			lDocumentRoot = value
			//add check for the validity of the path
			if _, err := os.Stat(lDocumentRoot); os.IsNotExist(err) {
				return "", "", "", fmt.Errorf("invalid document root in config file: %s", value)
			}
		case "PidFile":
			pidFile = value
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", "", err
	}

	return port, lDocumentRoot, pidFile, nil
}

// writePIDFile writes the current process's PID to a file.
func writePIDFile(pidFile string) error {
	pid := os.Getpid()
	pidString := strconv.Itoa(pid)
	err := ioutil.WriteFile(pidFile, []byte(pidString), 0644) // 0644: rw-r--r--
	if err != nil {
		return fmt.Errorf("failed to write PID to file %s: %v", pidFile, err)
	}
	return nil
}

// stopServer sends an HTTP request to localhost:8080/stop to shut down the server.
func stopServer() {
	fmt.Println("stopServer function called, sending /stop request")
	// Create an HTTP client.
	client := &http.Client{}

	// Create a new HTTP request.
	req, err := http.NewRequest("GET", "http://localhost:8080/stop", nil)
	if err != nil {
		log.Printf("Error creating /stop request: %v\n", err)
		return // IMPORTANT:  Return on error!
	}

	// Send the request.
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending /stop request: %v\n", err)
		return // IMPORTANT: Return on error!
	}
	defer resp.Body.Close() // Ensure the body is closed after the function finishes.

	// Read the response body (optional, but good for logging).
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading /stop response body: %v\n", err)
		return //  Return on error
	}

	// Log the response from the /stop request.
	fmt.Printf("Received response from /stop: %s\n", string(body))
}

// executeCGI executes the CGI script and returns the output
func executeCGI(w http.ResponseWriter, r *http.Request) {
	// Extract the CGI script name from the URL
	cgiScriptName := strings.TrimPrefix(r.URL.Path, "/cgi-bin/")
	cgiScriptPath := filepath.Join("cgi-bin", cgiScriptName) // Ensure script is in cgi-bin directory

	// Check if the CGI script exists and is executable
	fileInfo, err := os.Stat(cgiScriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			fmt.Printf("CGI script not found: %s\n", cgiScriptPath)
		} else {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			fmt.Printf("Error stating CGI script: %s, error: %v\n", cgiScriptPath, err)
		}
		return
	}
	if fileInfo.Mode()&0100 == 0 { // Check if executable (Unix-like)
		http.Error(w, "Forbidden", http.StatusForbidden)
		fmt.Printf("CGI script is not executable: %s\n", cgiScriptPath)
		return
	}

	// Set up the command to execute the CGI script
	cmd := exec.Command(cgiScriptPath)

	// Set the request environment variables for the CGI script.  Crucial for CGI!
	cmd.Env = os.Environ() // Start with the current environment
	cmd.Env = append(cmd.Env,
		"REQUEST_METHOD="+r.Method,
		"QUERY_STRING="+r.URL.RawQuery,
		"CONTENT_LENGTH="+strconv.FormatInt(r.ContentLength, 10),
		"REQUEST_URI="+r.URL.RequestURI(), // Added Request URI
		"SCRIPT_NAME="+cgiScriptName, // Added SCRIPT_NAME
		"SCRIPT_FILENAME="+cgiScriptPath,
		"SERVER_PROTOCOL="+r.Proto,
		"SERVER_SOFTWARE=gopache/1.0", //added server software name
		"REMOTE_ADDR="+r.RemoteAddr,
	)
	if r.Method == "POST" {
		cmd.Stdin = r.Body
	}

	// Capture the output from the CGI script
	output, err := cmd.CombinedOutput() // Use CombinedOutput for both stdout and stderr
	if err != nil {
		// Log the error from the CGI script
		log.Printf("Error executing CGI script: %s, error: %v, output: %s\n", cgiScriptPath, err, output)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the output of the CGI script to the response
	w.Write(output)
	fmt.Printf("CGI script executed successfully: %s\n", cgiScriptPath)
}

func usage() {
	fmt.Printf("Usage: %s start|startnohup|stop\n", webserver)
	return
}

func main() {
	isRunning = false // Initialize server state

	// Check for the "stop" argument.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "start":
			break
		case "stop":
			stopServer()
			return // Exit the program after calling stopServer.
		case "startnohup":
			// Detach and start server in background using nohup
			fmt.Println("Starting server in background with nohup...")
			cmd := exec.Command("nohup", os.Args[0], "server") // Start the server, passing "server" argument
			cmd.Stdout = os.Stdout                                 // Or a file if you want to log output
			cmd.Stderr = os.Stderr
			err := cmd.Start()
			if err != nil {
				log.Fatalf("Failed to start server in background with nohup: %v", err)
			}
			fmt.Printf("Server started in background, PID: %d\n", cmd.Process.Pid)
			return
		default:
			usage()
			return
		}
	} else {
		usage()
		return
	}
	// Define the default port
	defaultPort := ":8080"
	port := defaultPort

	// Define the default pid file.
	defaultPidFile := fmt.Sprintf("/var/run/%s.pid", webserver) // Use the variable
	pidFile := defaultPidFile

	// Read the port number from the configuration file
	filename := "httpd.conf"
	serverPort, documentRoot, confPidFile, err := readConfigFile(filename)
	if err != nil {
		log.Fatalf("Error reading from %s: %v\n", filename, err)
	} else {
		port = serverPort // Use the port from the file if found
		if confPidFile != "" {
			pidFile = confPidFile
		}
	}
	log.Printf("Using port %s and pid file: %s, document root: %s\n", port, pidFile, documentRoot) // Include documentRoot in log

	// Write PID to file
	err = writePIDFile(pidFile)
	if err != nil {
		log.Fatalf("Error writing PID file: %v", err) // Use log.Fatalf to exit on error.
	}
	defer os.Remove(pidFile) // Ensure PID file is removed when the program exits.
	// Create a new HTTP server instance.  This is important for graceful shutdown.
	server = &http.Server{
		Addr:         port,                                     // Use the port we determined.
		Handler:      http.HandlerFunc(handler),                // Use the handler function.
		// Add timeouts for added robustness
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Register the handler function for the root URL path "/"
	//http.HandleFunc("/", handler) //  No longer used directly with server instance

	fmt.Printf("Starting server on http://localhost%s\n", port)

	isRunning = true

	// Start the HTTP server in a goroutine, so it doesn't block.
	serverWaitGroup.Add(1) // Increment the WaitGroup counter before starting the server.
	go func() {
		defer serverWaitGroup.Done() // Ensure Done is called when the goroutine exits
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe error: %v\n", err)
		}
		isRunning = false
		log.Println("Server goroutine exited") // Add this line
	}()

	// Set up a signal handler for graceful shutdown on Ctrl+C or SIGTERM.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Block until a signal is received.
	<-signalChan
	fmt.Println("Received shutdown signal.  Shutting down server...")

	// Create a context for shutdown with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Attempt to shut down the server gracefully.
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v\n", err)
	}

	// Wait for the server to completely stop.
	serverWaitGroup.Wait()
	fmt.Println("Server stopped. Exiting.")
}

