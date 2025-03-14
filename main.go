package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// SitemapIndex represents a sitemap index file
type SitemapIndex struct {
	XMLName xml.Name `xml:"sitemapindex"`
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
	URL    string
	Status int
	Error  error
}

func main() {
	// Define command-line flags
	sitemapURL := flag.String("u", "", "URL of the sitemap.xml file (required)")
	timeout := flag.Int("t", 1000, "Timeout in milliseconds between check requests")
	flag.Parse()

	// Check if sitemap URL is provided
	if *sitemapURL == "" {
		fmt.Println("Error: Sitemap URL is required. Use -u flag to specify the URL.")
		flag.Usage()
		os.Exit(1)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Retrieve and process the sitemap
	allURLs, err := retrieveAllURLs(client, *sitemapURL)
	if err != nil {
		fmt.Printf("Error retrieving URLs: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d URLs to check\n", len(allURLs))

	// Check all URLs
	results := checkURLs(client, allURLs, *timeout)

	// Print problematic URLs
	problematicCount := 0
	for _, result := range results {
		if result.Error != nil || result.Status < 200 || result.Status >= 300 {
			problematicCount++
			if result.Error != nil {
				fmt.Printf("ERROR: %s - %v\n", result.URL, result.Error)
			} else {
				fmt.Printf("INVALID STATUS: %s - %d\n", result.URL, result.Status)
			}
		}
	}

	fmt.Printf("\nSummary: Found %d problematic URLs out of %d total URLs\n", problematicCount, len(results))
}

// retrieveAllURLs retrieves all URLs from a sitemap, including referenced sitemaps
func retrieveAllURLs(client *http.Client, sitemapURL string) ([]string, error) {
	body, err := fetchURL(client, sitemapURL)
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
func checkURLs(client *http.Client, urls []string, timeoutMs int) []Result {
	results := make([]Result, 0, len(urls))
	resultsChan := make(chan Result, len(urls))
	var wg sync.WaitGroup

	// Process URLs with rate limiting
	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			
			// Create a request to check headers only
			req, err := http.NewRequest("HEAD", url, nil)
			if err != nil {
				resultsChan <- Result{URL: url, Error: err}
				return
			}
			
			// Set a user agent to avoid being blocked
			req.Header.Set("User-Agent", "SitemapChecker/1.0")
			
			resp, err := client.Do(req)
			if err != nil {
				resultsChan <- Result{URL: url, Error: err}
				return
			}
			defer resp.Body.Close()
			
			resultsChan <- Result{URL: url, Status: resp.StatusCode}
			
			// If HEAD request returned 405 Method Not Allowed, try GET instead
			if resp.StatusCode == http.StatusMethodNotAllowed {
				time.Sleep(time.Duration(timeoutMs) * time.Millisecond)
				
				getReq, err := http.NewRequest("GET", url, nil)
				if err != nil {
					return
				}
				getReq.Header.Set("User-Agent", "SitemapChecker/1.0")
				
				getResp, err := client.Do(getReq)
				if err != nil {
					return
				}
				defer getResp.Body.Close()
				
				resultsChan <- Result{URL: url, Status: getResp.StatusCode}
			}
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
