package archiver

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	// DirectDownloadToolName is the name of the direct download archival tool
	DirectDownloadToolName = "direct_download"
	// DefaultTimeout is the default timeout for downloads
	DefaultTimeout = 30 * time.Second
	// MaxFileSize is the maximum file size to download (100MB)
	MaxFileSize = 100 * 1024 * 1024
)

// DirectDownload implements the ArchivalTool interface for direct file downloads
type DirectDownload struct {
	client  *http.Client
	timeout time.Duration
}

// NewDirectDownload creates a new direct download archival tool
func NewDirectDownload(timeout time.Duration) *DirectDownload {
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	return &DirectDownload{
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

// Name returns the name of this archival tool
func (d *DirectDownload) Name() string {
	return DirectDownloadToolName
}

// Archive downloads a file from the given URL
func (d *DirectDownload) Archive(url, mimeType string) (*ArchivedFile, error) {
	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create GET request")
	}

	// Set a reasonable User-Agent
	req.Header.Set("User-Agent", "Mattermost-Link-Archiver-Plugin/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to download file")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Check Content-Length if available
	if resp.ContentLength > MaxFileSize {
		return nil, errors.Errorf("file size %d exceeds maximum allowed size %d", resp.ContentLength, MaxFileSize)
	}

	// Limit reader to prevent downloading files that are too large
	limitedReader := io.LimitReader(resp.Body, MaxFileSize+1)

	// Read the file data
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file data")
	}

	// Check if we hit the limit
	if int64(len(data)) > MaxFileSize {
		return nil, errors.Errorf("file size exceeds maximum allowed size %d", MaxFileSize)
	}

	// Determine filename from URL or Content-Disposition header
	filename := d.extractFilename(url, resp.Header.Get("Content-Disposition"))

	// Use MIME type from response if available, otherwise use the provided one
	if respMimeType := resp.Header.Get("Content-Type"); respMimeType != "" {
		// Remove charset and other parameters
		parts := strings.Split(respMimeType, ";")
		mimeType = strings.TrimSpace(parts[0])
	}

	return &ArchivedFile{
		Filename: filename,
		Data:     data,
		MimeType: mimeType,
		Size:     int64(len(data)),
	}, nil
}

// extractFilename extracts filename from URL or Content-Disposition header
func (d *DirectDownload) extractFilename(url, contentDisposition string) string {
	// Try Content-Disposition header first
	if contentDisposition != "" {
		// Parse "attachment; filename=example.pdf" or "attachment; filename*=UTF-8''example.pdf"
		parts := strings.Split(contentDisposition, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				filename := strings.TrimPrefix(part, "filename=")
				filename = strings.Trim(filename, `"`)
				if filename != "" {
					return filename
				}
			} else if strings.HasPrefix(part, "filename*=") {
				// Handle RFC 5987 encoded filenames: filename*=UTF-8''example.pdf
				filename := strings.TrimPrefix(part, "filename*=")
				parts := strings.SplitN(filename, "''", 2)
				if len(parts) == 2 {
					return parts[1]
				}
			}
		}
	}

	// Fallback to extracting from URL
	urlParts := strings.Split(url, "/")
	if len(urlParts) > 0 {
		lastPart := urlParts[len(urlParts)-1]
		// Remove query parameters
		if idx := strings.Index(lastPart, "?"); idx != -1 {
			lastPart = lastPart[:idx]
		}
		if lastPart != "" {
			return lastPart
		}
	}

	// Default filename based on extension or generic name
	return "downloaded_file"
}

// GetFileExtension returns the file extension for a given MIME type
func GetFileExtension(mimeType string) string {
	extensions := map[string]string{
		"application/pdf":              ".pdf",
		"image/jpeg":                   ".jpg",
		"image/png":                    ".png",
		"image/gif":                    ".gif",
		"image/webp":                   ".webp",
		"application/zip":              ".zip",
		"application/x-zip-compressed": ".zip",
		"application/x-rar-compressed": ".rar",
		"application/x-7z-compressed":  ".7z",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
		"application/msword": ".doc",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
		"application/vnd.ms-excel": ".xls",
		"text/plain":               ".txt",
		"text/html":                ".html",
		"text/css":                 ".css",
		"application/javascript":   ".js",
		"application/json":         ".json",
	}

	if ext, ok := extensions[mimeType]; ok {
		return ext
	}

	// Try to extract from MIME type pattern (e.g., "image/*" -> ".jpg")
	if strings.HasPrefix(mimeType, "image/") {
		return ".jpg"
	}

	return ""
}
