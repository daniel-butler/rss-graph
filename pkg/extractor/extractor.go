// Package extractor provides functions for extracting links from HTML content.
package extractor

import (
	"regexp"
	"strings"
)

// Link represents an extracted hyperlink.
type Link struct {
	URL  string
	Text string
}

// hrefRegex matches href attributes in anchor tags
var hrefRegex = regexp.MustCompile(`<a[^>]+href=["']([^"']+)["'][^>]*>([^<]*)</a>`)

// ExtractLinks extracts all http/https links from HTML content.
// It ignores anchors (#), javascript:, and mailto: links.
func ExtractLinks(html string) []Link {
	if html == "" {
		return []Link{}
	}

	matches := hrefRegex.FindAllStringSubmatch(html, -1)
	if matches == nil {
		return []Link{}
	}

	seen := make(map[string]bool)
	var links []Link

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		url := strings.TrimSpace(match[1])
		text := strings.TrimSpace(match[2])

		// Skip unwanted URL schemes
		if strings.HasPrefix(url, "#") ||
			strings.HasPrefix(url, "javascript:") ||
			strings.HasPrefix(url, "mailto:") {
			continue
		}

		// Normalize URL for deduplication (remove trailing slash)
		normalizedURL := strings.TrimSuffix(url, "/")
		if seen[normalizedURL] {
			continue
		}
		seen[normalizedURL] = true

		links = append(links, Link{
			URL:  url,
			Text: text,
		})
	}

	return links
}
