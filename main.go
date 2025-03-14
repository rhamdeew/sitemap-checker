package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"net/url"
)

// SitemapIndex represents a sitemap index file
type SitemapIndex struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

// Sitemap represents a sitemap entry in a sitemap index file
type Sitemap struct {
	Loc string `xml:"loc"`
}

// URLSet represents a sitemap file
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []URL    `xml:"url"`
}

// URL represents a URL entry in a sitemap file
type URL struct {
	Loc string `xml:"loc"`
}

// Result represents the result of checking a URL
type Result struct {
	URL            string
	Status         int
	Error          error
	RedirectURL    string
	IsRedirect     bool
}

// Logger represents a simple logger for writing to a file
type Logger struct {
	file    *os.File
	mu      sync.Mutex
}

// ProgressBar represents a simple progress bar
type ProgressBar struct {
	total      int
	current    int
	mu         sync.Mutex
	lastUpdate time.Time
}

// NewProgressBar creates a new progress bar
func NewProgressBar(total int) *ProgressBar {
	return &ProgressBar{
		total:      total,
		current:    0,
		lastUpdate: time.Now(),
	}
}

// NewLogger creates a new logger with the specified file
func NewLogger(filename string) (*Logger, error) {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(filename)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}
	
	// Open the log file for writing
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	
	return &Logger{file: file}, nil
}

// Log writes a message to the log file
func (l *Logger) Log(message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := fmt.Fprintln(l.file, message)
	return err
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// createLogFilename generates a log filename based on target hostname, date and time
func createLogFilename(sitemapURL string) (string, error) {
	// Get hostname from the sitemap URL
	parsedURL, err := url.Parse(sitemapURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse sitemap URL: %w", err)
	}
	
	// Extract host
	hostname := parsedURL.Host
	
	// Strip port number if present
	if colonIndex := indexOf(hostname, ":"); colonIndex != -1 {
		hostname = hostname[:colonIndex]
	}
	
	// Replace any dots with dashes for a cleaner filename
	hostname = strings.ReplaceAll(hostname, ".", "-")
	
	// Format current time
	now := time.Now()
	dateStr := now.Format("2006-01-02")
	timeStr := now.Format("15-04-05")
	
	// Create filename
	filename := fmt.Sprintf("%s-%s-%s.log", hostname, dateStr, timeStr)
	return filename, nil
}

// indexOf returns the index of the first instance of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// Increment increases the progress by one and updates the display if needed
func (pb *ProgressBar) Increment() {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.current++
	
	// Only update the progress bar every 100ms to avoid flooding the terminal
	if time.Since(pb.lastUpdate) > 100*time.Millisecond || pb.current == pb.total {
		pb.update()
		pb.lastUpdate = time.Now()
	}
}

// update displays the current progress
func (pb *ProgressBar) update() {
	width := 50
	percentage := float64(pb.current) / float64(pb.total)
	completed := int(float64(width) * percentage)
	
	fmt.Printf("\r[")
	for i := 0; i < width; i++ {
		if i < completed {
			fmt.Print("=")
		} else if i == completed {
			fmt.Print(">")
		} else {
			fmt.Print(" ")
		}
	}
	
	fmt.Printf("] %d/%d (%d%%)", pb.current, pb.total, int(percentage*100))
	
	// Print newline when complete
	if pb.current == pb.total {
		fmt.Println()
	}
}

func main() {
	// Define command-line flags
	sitemapURL := flag.String("u", "", "URL of the sitemap.xml file (required)")
	timeout := flag.Int("t", 1000, "Timeout in milliseconds between check requests")
	logDir := flag.String("logdir", "", "Directory to store log files (default: current directory)")
	
	flag.Parse()
	
	// Check if sitemap URL is provided
	if *sitemapURL == "" {
		fmt.Println("Error: Sitemap URL is required. Use -u flag to specify the URL.")
		flag.Usage()
		os.Exit(1)
	}
	
	// Create log filename with format %hostname%-%date%-%time%.log
	logFilename, err := createLogFilename(*sitemapURL)
	if err != nil {
		fmt.Printf("Warning: Failed to create log filename: %v. Using default filename.\n", err)
		logFilename = "sitemap-check.log"
	}
	
	// If logdir is specified, prepend it to the filename
	if *logDir != "" {
		logFilename = filepath.Join(*logDir, logFilename)
	}
	
	// Create logger
	logger, err := NewLogger(logFilename)
	if err != nil {
		fmt.Printf("Warning: Failed to create logger: %v. Proceeding without logging.\n", err)
	} else {
		defer logger.Close()
		fmt.Printf("Logging to: %s\n", logFilename)
		
		// Write header to log file
		parsedURL, err := url.Parse(*sitemapURL)
		if err == nil {
			logger.Log(fmt.Sprintf("Sitemap check for: %s", parsedURL.Host))
		}
		logger.Log(fmt.Sprintf("Started at: %s", time.Now().Format(time.RFC3339)))
		logger.Log("-------------------------------------------")
	}
	
	// Create HTTP client with CheckRedirect to prevent following redirects
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects - instead return an error to capture the redirect
			return http.ErrUseLastResponse
		},
	}
	
	// Retrieve and process the sitemap
	fmt.Println("Retrieving URLs from sitemap...")
	allURLs, err := retrieveAllURLs(client, *sitemapURL)
	if err != nil {
		fmt.Printf("Error retrieving URLs: %v\n", err)
		if logger != nil {
			logger.Log(fmt.Sprintf("Error retrieving URLs: %v", err))
		}
		os.Exit(1)
	}
	
	fmt.Printf("Found %d URLs to check\n", len(allURLs))
	if logger != nil {
		logger.Log(fmt.Sprintf("Found %d URLs to check", len(allURLs)))
	}
	
	fmt.Println("Checking URLs...")
	
	// Check all URLs with progress bar and logger
	results := checkURLs(client, allURLs, *timeout, logger)
	
	// Print problematic URLs
	problematicCount := 0
	redirectCount := 0
	
	for _, result := range results {
		if result.Error != nil || result.Status < 200 || result.Status >= 300 {
			problematicCount++
			
			if result.IsRedirect {
				redirectCount++
				fmt.Printf("REDIRECT: %s -> %s (Status: %d)\n", result.URL, result.RedirectURL, result.Status)
			} else if result.Error != nil {
				fmt.Printf("ERROR: %s - %v\n", result.URL, result.Error)
			} else {
				fmt.Printf("INVALID STATUS: %s - %d\n", result.URL, result.Status)
			}
		}
	}
	
	// Log and print summary
	summaryMsg := fmt.Sprintf("\nSummary: Found %d problematic URLs out of %d total URLs", problematicCount, len(results))
	redirectMsg := fmt.Sprintf("Redirects: %d URLs", redirectCount)
	
	fmt.Println(summaryMsg)
	fmt.Println(redirectMsg)
	
	if logger != nil {
		logger.Log("-------------------------------------------")
		logger.Log(summaryMsg)
		logger.Log(redirectMsg)
		logger.Log(fmt.Sprintf("Finished at: %s", time.Now().Format(time.RFC3339)))
	}
}

// retrieveAllURLs retrieves all URLs from a sitemap, including referenced sitemaps
func retrieveAllURLs(client *http.Client, sitemapURL string) ([]string, error) {
	// Create a temporary client that follows redirects for sitemap retrieval
	tempClient := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	body, err := fetchURL(tempClient, sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching sitemap: %w", err)
	}
	
	// Try to parse as a sitemap index first
	var sitemapIndex SitemapIndex
	if err := xml.Unmarshal(body, &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		fmt.Printf("Found sitemap index with %d sitemaps\n", len(sitemapIndex.Sitemaps))
		
		var allURLs []string
		for _, sitemap := range sitemapIndex.Sitemaps {
			fmt.Printf("Processing referenced sitemap: %s\n", sitemap.Loc)
			urls, err := retrieveAllURLs(client, sitemap.Loc)
			if err != nil {
				fmt.Printf("Warning: Error processing referenced sitemap %s: %v\n", sitemap.Loc, err)
				continue
			}
			allURLs = append(allURLs, urls...)
		}
		
		return allURLs, nil
	}
	
	// If not a sitemap index, try to parse as a regular sitemap
	var urlSet URLSet
	if err := xml.Unmarshal(body, &urlSet); err != nil {
		return nil, fmt.Errorf("error parsing sitemap: %w", err)
	}
	
	var urls []string
	for _, u := range urlSet.URLs {
		urls = append(urls, u.Loc)
	}
	
	return urls, nil
}

// fetchURL fetches the content of a URL
func fetchURL(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}
	
	return io.ReadAll(resp.Body)
}

// checkURLs checks all URLs and returns their status
func checkURLs(client *http.Client, urls []string, timeoutMs int, logger *Logger) []Result {
	results := make([]Result, 0, len(urls))
	resultsChan := make(chan Result, len(urls))
	var wg sync.WaitGroup
	
	// Create progress bar
	progressBar := NewProgressBar(len(urls))
	
	// Process URLs with rate limiting
	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			
			// Create a request to check headers only
			req, err := http.NewRequest("HEAD", url, nil)
			if err != nil {
				result := Result{URL: url, Error: err}
				resultsChan <- result
				
				// Log error immediately
				if logger != nil {
					logger.Log(fmt.Sprintf("ERROR: %s - %v", url, err))
				}
				
				progressBar.Increment()
				return
			}
			
			// Set a user agent to avoid being blocked
			req.Header.Set("User-Agent", "SitemapChecker/1.0")
			
			resp, err := client.Do(req)
			if err != nil {
				// Check if it's a redirect error
				if resp != nil && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
					// It's a redirect
					redirectURL := resp.Header.Get("Location")
					result := Result{
						URL:         url,
						Status:      resp.StatusCode,
						IsRedirect:  true,
						RedirectURL: redirectURL,
					}
					resultsChan <- result
					
					// Log redirect immediately
					if logger != nil {
						logger.Log(fmt.Sprintf("REDIRECT: %s -> %s (Status: %d)", url, redirectURL, resp.StatusCode))
					}
				} else {
					// It's another error
					result := Result{URL: url, Error: err}
					resultsChan <- result
					
					// Log error immediately
					if logger != nil {
						logger.Log(fmt.Sprintf("ERROR: %s - %v", url, err))
					}
				}
				
				progressBar.Increment()
				return
			}
			defer resp.Body.Close()
			
			result := Result{URL: url, Status: resp.StatusCode}
			
			// Check for redirects (status codes 301, 302, 303, 307, 308)
			if resp.StatusCode >= 300 && resp.StatusCode < 400 {
				redirectURL := resp.Header.Get("Location")
				result.IsRedirect = true
				result.RedirectURL = redirectURL
				
				// Log redirect immediately
				if logger != nil {
					logger.Log(fmt.Sprintf("REDIRECT: %s -> %s (Status: %d)", url, redirectURL, resp.StatusCode))
				}
			} else if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				// Log bad status immediately
				if logger != nil {
					logger.Log(fmt.Sprintf("INVALID STATUS: %s - %d", url, resp.StatusCode))
				}
			}
			
			resultsChan <- result
			
			// If HEAD request returned 405 Method Not Allowed, try GET instead
			if resp.StatusCode == http.StatusMethodNotAllowed {
				time.Sleep(time.Duration(timeoutMs) * time.Millisecond)
				
				getReq, err := http.NewRequest("GET", url, nil)
				if err != nil {
					progressBar.Increment()
					return
				}
				
				getReq.Header.Set("User-Agent", "SitemapChecker/1.0")
				
				getResp, err := client.Do(getReq)
				if err != nil {
					// Check if it's a redirect error
					if getResp != nil && (getResp.StatusCode >= 300 && getResp.StatusCode < 400) {
						// It's a redirect
						redirectURL := getResp.Header.Get("Location")
						getResult := Result{
							URL:         url,
							Status:      getResp.StatusCode,
							IsRedirect:  true,
							RedirectURL: redirectURL,
						}
						resultsChan <- getResult
						
						// Log redirect immediately
						if logger != nil {
							logger.Log(fmt.Sprintf("REDIRECT (GET after 405): %s -> %s (Status: %d)", 
								url, redirectURL, getResp.StatusCode))
						}
					} else {
						// It's another error
						if logger != nil {
							logger.Log(fmt.Sprintf("ERROR (GET after 405): %s - %v", url, err))
						}
					}
					
					progressBar.Increment()
					return
				}
				defer getResp.Body.Close()
				
				getResult := Result{URL: url, Status: getResp.StatusCode}
				
				// Check for redirects (status codes 301, 302, 303, 307, 308)
				if getResp.StatusCode >= 300 && getResp.StatusCode < 400 {
					redirectURL := getResp.Header.Get("Location")
					getResult.IsRedirect = true
					getResult.RedirectURL = redirectURL
					
					// Log redirect immediately
					if logger != nil {
						logger.Log(fmt.Sprintf("REDIRECT (GET after 405): %s -> %s (Status: %d)", 
							url, redirectURL, getResp.StatusCode))
					}
				} else if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
					// Log bad status immediately
					if logger != nil {
						logger.Log(fmt.Sprintf("INVALID STATUS (GET after 405): %s - %d", url, getResp.StatusCode))
					}
				}
				
				resultsChan <- getResult
			}
			
			progressBar.Increment()
		}(url)
		
		// Sleep to respect the timeout between requests
		time.Sleep(time.Duration(timeoutMs) * time.Millisecond)
	}
	
	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()
	
	// Collect results
	for result := range resultsChan {
		results = append(results, result)
	}
	
	return results
}
