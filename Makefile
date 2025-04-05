# Makefile for building the Go web server

# The name of the final executable
BINARY_NAME := gopache

# Go source files to include in the build
GO_FILES := main.go

# Build the binary
build:
	go build -o $(BINARY_NAME) $(GO_FILES)

# Run the binary (assumes you have a httpd.conf and public_html directory set up)
run: build
	./$(BINARY_NAME)

# Clean up the binary
clean:
	rm -f $(BINARY_NAME)

# Format the Go code (optional)
fmt:
	go fmt $(GO_FILES)

# Run tests (if you have any)
test:
	go test ./...

# Default target (build the binary)
.PHONY: all build run clean fmt test
all: build

