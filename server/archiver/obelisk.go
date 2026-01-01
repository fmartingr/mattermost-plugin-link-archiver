package archiver

import (
	"context"
	"strings"
	"time"

	"github.com/go-shiori/obelisk"
	"github.com/pkg/errors"
)

const (
	// ObeliskToolName is the name of the obelisk archival tool
	ObeliskToolName = "obelisk"
	// ObeliskDefaultTimeout is the default timeout for obelisk archival
	ObeliskDefaultTimeout = 60 * time.Second
	// ObeliskMaxFileSize is the maximum file size for archived HTML (50MB)
	ObeliskMaxFileSize = 50 * 1024 * 1024
)

// Obelisk implements the ArchivalTool interface for archiving HTML pages
type Obelisk struct {
	timeout time.Duration
}

// NewObelisk creates a new obelisk archival tool
func NewObelisk(timeout time.Duration) *Obelisk {
	if timeout == 0 {
		timeout = ObeliskDefaultTimeout
	}

	return &Obelisk{
		timeout: timeout,
	}
}

// Name returns the name of this archival tool
func (o *Obelisk) Name() string {
	return ObeliskToolName
}

// Archive archives an HTML page from the given URL using obelisk
func (o *Obelisk) Archive(url, mimeType string) (*ArchivedFile, error) {
	// Create a new archiver instance
	archiver := &obelisk.Archiver{
		RequestTimeout:        o.timeout,
		MaxConcurrentDownload: 5,
		DisableJS:             false,
		DisableCSS:            false,
		DisableEmbeds:         false,
		DisableMedias:         false,
		SkipResourceURLError:  true, // Ignore DNS errors and other resource URL errors
	}

	// Validate the archiver configuration
	archiver.Validate()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), o.timeout)
	defer cancel()

	// Create request
	req := obelisk.Request{
		URL: url,
	}

	// Archive the page
	data, contentType, err := archiver.Archive(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to archive page with obelisk")
	}

	// Check if data is empty
	if len(data) == 0 {
		return nil, errors.New("obelisk returned empty content")
	}

	// Check file size
	if int64(len(data)) > ObeliskMaxFileSize {
		return nil, errors.Errorf("archived page size %d exceeds maximum allowed size %d", len(data), ObeliskMaxFileSize)
	}

	// Generate filename from URL
	filename := o.extractFilename(url)
	if filename == "" {
		filename = "archived_page.obelisk.html"
	} else {
		// Remove existing .html or .htm extension if present
		if hasExtension(filename, ".html") {
			filename = filename[:len(filename)-5]
		} else if hasExtension(filename, ".htm") {
			filename = filename[:len(filename)-4]
		}
		// Add .obelisk.html extension
		filename += ".obelisk.html"
	}

	// Use content type from obelisk if available, otherwise default to text/html
	resultMimeType := "text/html"
	if contentType != "" {
		resultMimeType = contentType
	}

	return &ArchivedFile{
		Filename: filename,
		Data:     data,
		MimeType: resultMimeType,
		Size:     int64(len(data)),
	}, nil
}

// extractFilename extracts filename from URL
func (o *Obelisk) extractFilename(url string) string {
	// Simple extraction: get the last path segment from URL
	// Remove protocol
	urlPart := url
	if idx := strings.Index(url, "://"); idx != -1 {
		urlPart = url[idx+3:]
	}
	// Remove domain and get path
	if idx := strings.Index(urlPart, "/"); idx != -1 {
		path := urlPart[idx+1:]
		if path == "" {
			return "index.html"
		}
		// Get last segment
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Remove query parameters
			if idx := strings.Index(lastPart, "?"); idx != -1 {
				lastPart = lastPart[:idx]
			}
			// Remove fragment
			if idx := strings.Index(lastPart, "#"); idx != -1 {
				lastPart = lastPart[:idx]
			}
			if lastPart != "" {
				return lastPart
			}
		}
	}

	// Default to index.html if we can't extract a good name
	return "index.html"
}

// hasExtension checks if a filename has a specific extension
func hasExtension(filename, ext string) bool {
	return len(filename) >= len(ext) && filename[len(filename)-len(ext):] == ext
}
