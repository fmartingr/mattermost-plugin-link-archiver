package main

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// URLMetadata contains metadata about a URL including ETag and content hash
type URLMetadata struct {
	MimeType string
	ETag     string
	Size     int64
}

// ContentDetector detects MIME types of URLs
type ContentDetector struct {
	client  *http.Client
	timeout time.Duration
}

// NewContentDetector creates a new content detector
func NewContentDetector(timeout time.Duration) *ContentDetector {
	return &ContentDetector{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Follow redirects
				return nil
			},
		},
		timeout: timeout,
	}
}

// DetectMimeType detects the MIME type of a URL
// First tries HEAD request, falls back to GET if HEAD is not supported
func (d *ContentDetector) DetectMimeType(url string) (string, error) {
	// Try HEAD request first
	mimeType, err := d.detectWithHEAD(url)
	if err == nil && mimeType != "" {
		return mimeType, nil
	}

	// Fallback to GET request
	mimeType, err = d.detectWithGET(url)
	if err != nil {
		return "", errors.Wrapf(err, "failed to detect MIME type for URL: %s", url)
	}

	return mimeType, nil
}

// GetURLMetadata retrieves metadata about a URL including ETag and size
func (d *ContentDetector) GetURLMetadata(url string) (*URLMetadata, error) {
	req, err := http.NewRequest("HEAD", url, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HEAD request")
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Mattermost-Link-Archiver-Plugin/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		// Fallback to GET if HEAD fails
		return d.getMetadataWithGET(url)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Fallback to GET if HEAD returns error
		return d.getMetadataWithGET(url)
	}

	contentType := resp.Header.Get("Content-Type")
	mimeType := ""
	if contentType != "" {
		parts := strings.Split(contentType, ";")
		mimeType = strings.TrimSpace(parts[0])
	}

	etag := resp.Header.Get("ETag")
	// Remove quotes from ETag if present
	etag = strings.Trim(etag, "\"")

	return &URLMetadata{
		MimeType: mimeType,
		ETag:     etag,
		Size:     resp.ContentLength,
	}, nil
}

// getMetadataWithGET retrieves metadata using GET request
func (d *ContentDetector) getMetadataWithGET(url string) (*URLMetadata, error) {
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create GET request")
	}

	req.Header.Set("User-Agent", "Mattermost-Link-Archiver-Plugin/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "GET request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, errors.Errorf("GET request returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	mimeType := ""
	if contentType != "" {
		parts := strings.Split(contentType, ";")
		mimeType = strings.TrimSpace(parts[0])
	}

	etag := resp.Header.Get("ETag")
	etag = strings.Trim(etag, "\"")

	return &URLMetadata{
		MimeType: mimeType,
		ETag:     etag,
		Size:     resp.ContentLength,
	}, nil
}

// detectWithHEAD tries to detect MIME type using HEAD request
func (d *ContentDetector) detectWithHEAD(url string) (string, error) {
	req, err := http.NewRequest("HEAD", url, http.NoBody)
	if err != nil {
		return "", errors.Wrap(err, "failed to create HEAD request")
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Mattermost-Link-Archiver-Plugin/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "HEAD request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", errors.Errorf("HEAD request returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		// Extract MIME type (remove charset and other parameters)
		mimeType := strings.Split(contentType, ";")[0]
		return strings.TrimSpace(mimeType), nil
	}

	return "", errors.New("no Content-Type header in response")
}

// detectWithGET tries to detect MIME type using GET request (only reads headers, not body)
func (d *ContentDetector) detectWithGET(url string) (string, error) {
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return "", errors.Wrap(err, "failed to create GET request")
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Mattermost-Link-Archiver-Plugin/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "GET request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", errors.Errorf("GET request returned status %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		// Extract MIME type (remove charset and other parameters)
		mimeType := strings.Split(contentType, ";")[0]
		return strings.TrimSpace(mimeType), nil
	}

	// Try to detect from first few bytes if Content-Type is missing
	// Read only a small chunk to detect file type
	buffer := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, buffer)
	if n > 0 {
		detectedType := http.DetectContentType(buffer[:n])
		return detectedType, nil
	}

	return "", errors.New("no Content-Type header and unable to detect from content")
}
