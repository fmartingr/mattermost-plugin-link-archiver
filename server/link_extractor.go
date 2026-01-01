package main

import (
	"net/url"
	"regexp"
	"strings"
)

// LinkExtractor extracts URLs from post messages
type LinkExtractor struct{}

// NewLinkExtractor creates a new link extractor
func NewLinkExtractor() *LinkExtractor {
	return &LinkExtractor{}
}

// ExtractURLs extracts all URLs from a post message text
// Handles various formats: plain URLs, markdown links, etc.
func (e *LinkExtractor) ExtractURLs(message string) []string {
	var urls []string
	seen := make(map[string]bool)

	// Pattern to match URLs (http, https, and other common protocols)
	urlPattern := regexp.MustCompile(`(?i)(https?://[^\s<>"{}|\\^` + "`" + `\[\]]+)`)

	// First, extract plain URLs
	matches := urlPattern.FindAllString(message, -1)
	for _, match := range matches {
		match = strings.Trim(match, ".,;:!?)")
		if isValidURL(match) && !seen[match] {
			urls = append(urls, match)
			seen[match] = true
		}
	}

	// Also handle markdown links: [text](url)
	markdownPattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	markdownMatches := markdownPattern.FindAllStringSubmatch(message, -1)
	for _, match := range markdownMatches {
		if len(match) >= 3 {
			linkURL := match[2]
			if isValidURL(linkURL) && !seen[linkURL] {
				urls = append(urls, linkURL)
				seen[linkURL] = true
			}
		}
	}

	return urls
}

// isValidURL checks if a string is a valid URL
func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}
