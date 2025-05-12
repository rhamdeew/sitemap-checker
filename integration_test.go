package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMainIntegration tests the main functionality with a mock server
func TestMainIntegration(t *testing.T) {
	// Create a temp directory for test files
	tmpDir, err := os.MkdirTemp("", "integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a sitemap file
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>/page1</loc></url>
  <url><loc>/page2</loc></url>
  <url><loc>/redirect</loc></url>
  <url><loc>/not-found</loc></url>
</urlset>`

	// Set up a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, sitemapXML)
		case "/page1", "/page2":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Content of %s", r.URL.Path)
		case "/redirect":
			w.Header().Set("Location", "/page1")
			w.WriteHeader(http.StatusMovedPermanently)
		case "/not-found":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Replace the URL host in the sitemap with the test server's host
	sitemapXML = strings.Replace(sitemapXML, "<loc>/", fmt.Sprintf("<loc>%s/", server.URL), -1)

	// Create a sitemap file on the test server
	sitemapURL := fmt.Sprintf("%s/sitemap.xml", server.URL)

	// Set up command-line arguments for testing
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"sitemap_checker", "-u", sitemapURL, "-c", "2", "-t", "10", "-logdir", tmpDir}

	// Redirect stdout for testing
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Restore stdout when done
	defer func() { os.Stdout = oldStdout }()

	// Run the main function in a goroutine
	done := make(chan bool)
	go func() {
		main()
		done <- true
	}()

	// Wait for the main function to complete with a timeout
	select {
	case <-done:
		// Main function completed
	case <-time.After(5 * time.Second):
		// Timeout
		t.Log("Main function did not complete within timeout period")
	}

	// Close the pipe and read the output
	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if the output contains expected information
	if !strings.Contains(output, "Found") && !strings.Contains(output, "URLs to check") {
		t.Errorf("Output does not contain expected text: %s", output)
	}

	// Verify the log file exists
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	logFileFound := false
	for _, file := range files {
		if strings.Contains(file.Name(), ".log") {
			logFileFound = true
			break
		}
	}

	if !logFileFound {
		t.Errorf("Log file not created in directory: %s", tmpDir)
	}
}
