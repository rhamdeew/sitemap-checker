package main

import (
	"encoding/xml"
	"reflect"
	"testing"
)

// Test for parsing sitemap XML
func TestParseSitemapXML(t *testing.T) {
	// Test parsing a regular sitemap
	regularSitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/page1</loc>
  </url>
  <url>
    <loc>https://example.com/page2</loc>
  </url>
</urlset>`

	var urlSet URLSet
	if err := xml.Unmarshal([]byte(regularSitemapXML), &urlSet); err != nil {
		t.Fatalf("Failed to parse regular sitemap: %v", err)
	}

	expectedURLs := []URL{
		{Loc: "https://example.com/page1"},
		{Loc: "https://example.com/page2"},
	}

	if !reflect.DeepEqual(urlSet.URLs, expectedURLs) {
		t.Errorf("Parsed URLs = %+v, want %+v", urlSet.URLs, expectedURLs)
	}

	// Test parsing a sitemap index
	sitemapIndexXML := `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://example.com/sitemap1.xml</loc>
  </sitemap>
  <sitemap>
    <loc>https://example.com/sitemap2.xml</loc>
  </sitemap>
</sitemapindex>`

	var sitemapIndex SitemapIndex
	if err := xml.Unmarshal([]byte(sitemapIndexXML), &sitemapIndex); err != nil {
		t.Fatalf("Failed to parse sitemap index: %v", err)
	}

	expectedSitemaps := []Sitemap{
		{Loc: "https://example.com/sitemap1.xml"},
		{Loc: "https://example.com/sitemap2.xml"},
	}

	if !reflect.DeepEqual(sitemapIndex.Sitemaps, expectedSitemaps) {
		t.Errorf("Parsed sitemaps = %+v, want %+v", sitemapIndex.Sitemaps, expectedSitemaps)
	}
}

// Test handling invalid XML
func TestInvalidXML(t *testing.T) {
	invalidXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/page1</loc>
  </url>
  <url>
    <loc>https://example.com/page2
  </url>
</urlset>`

	var urlSet URLSet
	if err := xml.Unmarshal([]byte(invalidXML), &urlSet); err == nil {
		t.Errorf("Expected error when parsing invalid XML, got nil")
	}
}

// Test empty XML
func TestEmptyXML(t *testing.T) {
	emptyXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
</urlset>`

	var urlSet URLSet
	if err := xml.Unmarshal([]byte(emptyXML), &urlSet); err != nil {
		t.Fatalf("Failed to parse empty sitemap: %v", err)
	}

	if len(urlSet.URLs) != 0 {
		t.Errorf("Expected 0 URLs in empty sitemap, got %d", len(urlSet.URLs))
	}
}
