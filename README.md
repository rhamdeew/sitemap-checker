# Sitemap Checker

![CI/CD Status](https://github.com/rhamdeew/sitemap-checker/actions/workflows/release.yml/badge.svg)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/rhamdeew/sitemap-checker)](https://github.com/rhamdeew/sitemap-checker/releases/latest)
[![GitHub license](https://img.shields.io/github/license/rhamdeew/sitemap-checker)](https://github.com/rhamdeew/sitemap-checker/blob/main/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/rhamdeew/sitemap-checker)](https://goreportcard.com/report/github.com/rhamdeew/sitemap-checker)
[![GitHub stars](https://img.shields.io/github/stars/rhamdeew/sitemap-checker)](https://github.com/rhamdeew/sitemap-checker/stargazers)

A high-performance Go tool to validate all URLs in a website's sitemap.xml, with comprehensive error detection and redirect validation.

## Features

- **Complete sitemap validation**: Process both sitemap indexes and individual sitemaps
- **Recursive processing**: Handles nested sitemaps within sitemap indexes
- **Redirect detection**: Identifies and logs all redirects, capturing the redirect destination
- **Parallel processing**: Efficiently checks multiple URLs concurrently with configurable parallelism
- **Rate limiting**: Configurable delays between requests to avoid overwhelming servers
- **Detailed logging**: Comprehensive logs with timestamps, status codes, and errors
- **Progress visualization**: Real-time progress bar to monitor validation status
- **HEAD request optimization**: Uses HEAD requests by default, with fallback to GET for URLs that don't support HEAD

## Installation

### Prerequisites

- Go 1.18 or higher

### Installation Steps

```bash
# Clone the repository
git clone https://github.com/yourusername/sitemap_checker.git
cd sitemap_checker

# Build the binary
go build -o sitemap_checker

# Optional: Install system-wide
go install
```

## Usage

```bash
# Basic usage
./sitemap_checker -u https://example.com/sitemap.xml

# With custom timeout between requests (default: 1000ms)
./sitemap_checker -u https://example.com/sitemap.xml -t 500

# Specify a custom directory for log files
./sitemap_checker -u https://example.com/sitemap.xml -logdir ./logs

# Run with 10 parallel requests
./sitemap_checker -u https://example.com/sitemap.xml -c 10

# Skip SSL certificate validation
./sitemap_checker -u https://example.com/sitemap.xml -k

# Combine options
./sitemap_checker -u https://example.com/sitemap.xml -t 200 -c 5 -logdir ./logs -k
```

### Command Line Options

| Flag     | Description                                    | Default              |
|----------|------------------------------------------------|----------------------|
| `-u`     | URL of the sitemap.xml file (required)         | None (Required)      |
| `-t`     | Timeout in milliseconds between check requests | 1000 (1 second)      |
| `-logdir`| Directory to store log files                   | Current directory    |
| `-c`     | Number of parallel requests to execute         | 1 (Sequential)       |
| `-k`     | Skip SSL certificate validation                | false                |

## Log Files

Log files are automatically created with a naming format of:
```
hostname-YYYY-MM-DD-HH-MM-SS.log
```

Example: `example-com-2025-03-14-14-30-45.log`

### Log File Contents

Each log file contains:

1. Header with sitemap URL and start time
2. Concurrency configuration (number of parallel requests)
3. Full list of problematic URLs with details:
   - Invalid status codes (non-2xx)
   - Connection errors
   - **Redirects with their destination URLs**
4. Summary statistics
5. End timestamp

## How It Works

1. **Sitemap Retrieval**: The tool fetches and parses the provided sitemap URL
2. **Recursive Processing**: For sitemap indexes, it processes all child sitemaps
3. **URL Extraction**: All URLs are extracted from the sitemap(s)
4. **Parallel Validation Process**:
   - Makes a HEAD request for each URL (more efficient)
   - Falls back to GET if HEAD is not supported (status 405)
   - Records status codes, errors, and redirect locations
   - **Does not follow redirects** - instead flags them as issues
   - Controls concurrency using a semaphore pattern
5. **Reporting**: Provides a detailed summary of problematic URLs

## Redirect Handling

This tool specifically detects and flags redirects (status codes 3xx) without following them. For each redirect, it:

1. Identifies the redirect status code (301, 302, 303, 307, 308)
2. Captures the destination URL from the `Location` header
3. Marks the URL as problematic in reports
4. Logs the full redirect chain information

## Example Output

```
Retrieving URLs from sitemap...
Found sitemap index with 3 sitemaps
Processing referenced sitemap: https://example.com/post-sitemap.xml
Processing referenced sitemap: https://example.com/page-sitemap.xml
Processing referenced sitemap: https://example.com/category-sitemap.xml
Found 845 URLs to check
Checking URLs...
[==================================================>] 845/845 (100%)

REDIRECT: https://example.com/old-page/ -> https://example.com/new-page/ (Status: 301)
INVALID STATUS: https://example.com/missing-page/ - 404
ERROR: https://example.com/timeout-page/ - Get "https://example.com/timeout-page/": context deadline exceeded (Client.Timeout exceeded while awaiting headers)

Summary: Found 37 problematic URLs out of 845 total URLs
Redirects: 12 URLs
```

## Performance Tuning

- The default timeout between requests is 1000ms (1 second)
- The default concurrency is 1 (sequential requests)
- For optimal performance:
  - Increase concurrency (`-c` flag) to check multiple URLs in parallel
  - Adjust the timeout (`-t` flag) based on the server's capacity
- Recommended starting values:
  - Small sites: `-c 5 -t 500`
  - Medium sites: `-c 10 -t 1000`
  - Large sites: `-c 20 -t 2000`

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
