package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/fmartingrmattermost-plugin-link-archiver/server/archiver"
)

// ArchiveProcessor orchestrates the archival workflow
type ArchiveProcessor struct {
	linkExtractor      *LinkExtractor
	contentDetector    *ContentDetector
	storageService     *StorageService
	threadReplyService *ThreadReplyService
	archivalTools      map[string]archiver.ArchivalTool
	api                plugin.API
}

// NewArchiveProcessor creates a new archive processor
func NewArchiveProcessor(
	api plugin.API,
	linkExtractor *LinkExtractor,
	contentDetector *ContentDetector,
	storageService *StorageService,
	threadReplyService *ThreadReplyService,
) *ArchiveProcessor {
	processor := &ArchiveProcessor{
		linkExtractor:      linkExtractor,
		contentDetector:    contentDetector,
		storageService:     storageService,
		threadReplyService: threadReplyService,
		archivalTools:      make(map[string]archiver.ArchivalTool),
		api:                api,
	}

	// Register default archival tools
	processor.registerDefaultTools()

	return processor
}

// registerDefaultTools registers the default archival tools
func (p *ArchiveProcessor) registerDefaultTools() {
	// Register direct download tool
	directDownload := archiver.NewDirectDownload(30 * time.Second)
	p.archivalTools[archiver.DirectDownloadToolName] = directDownload

	// Register obelisk tool for HTML pages
	obeliskTool := archiver.NewObelisk(60 * time.Second)
	p.archivalTools[archiver.ObeliskToolName] = obeliskTool
}

// GetAvailableArchivalTools returns a list of available archival tool names
func (p *ArchiveProcessor) GetAvailableArchivalTools() []string {
	tools := make([]string, 0, len(p.archivalTools))
	for name := range p.archivalTools {
		tools = append(tools, name)
	}
	// Sort tools for consistent ordering
	sort.Strings(tools)
	return tools
}

// ProcessPost processes a post to archive any URLs found in it
func (p *ArchiveProcessor) ProcessPost(postID, message string, config *configuration) error {
	// Extract URLs from the message
	urls := p.linkExtractor.ExtractURLs(message)
	if len(urls) == 0 {
		return nil
	}

	// Process each URL asynchronously
	for _, url := range urls {
		go p.processURL(postID, url, config)
	}

	return nil
}

// processURL processes a single URL for archival
func (p *ArchiveProcessor) processURL(postID, url string, config *configuration) {
	// Check if URL has already been archived for this post
	alreadyArchivedForPost, err := p.storageService.IsURLAlreadyArchived(postID, url)
	if err != nil {
		p.api.LogError("Failed to check if URL is already archived for post", "url", url, "error", err.Error())
		// Continue processing - better to archive twice than to skip
	} else if alreadyArchivedForPost {
		p.api.LogInfo("URL already archived for this post, skipping", "url", url, "postID", postID)
		return
	}

	// Get URL metadata (ETag, size, etc.) to check if content has changed
	urlMetadata, err := p.contentDetector.GetURLMetadata(url)
	if err != nil {
		p.api.LogWarn("Failed to get URL metadata, proceeding with download", "url", url, "error", err.Error())
		urlMetadata = nil
	}

	// Check if URL has been archived globally and if content matches
	existingArchive, err := p.storageService.GetExistingArchiveForURL(url)
	if err != nil {
		p.api.LogWarn("Failed to check existing archive, proceeding with download", "url", url, "error", err.Error())
		existingArchive = nil
	}

	// If we have existing archive and URL metadata, check if content matches
	if existingArchive != nil && urlMetadata != nil {
		// Check if ETag matches (if both exist)
		if existingArchive.ETag != "" && urlMetadata.ETag != "" {
			if existingArchive.ETag == urlMetadata.ETag {
				// Content hasn't changed, reuse existing file
				p.api.LogInfo("URL content unchanged (ETag match), reusing existing archive", "url", url, "fileID", existingArchive.FileID)
				metadata := p.storageService.CreateMetadataForExistingFile(postID, url, existingArchive)

				// Create thread reply with existing file (include original post ID)
				if err = p.threadReplyService.ReplyWithAttachment(
					postID,
					metadata.FileID,
					url,
					metadata.Filename,
					metadata.MimeType,
					metadata.Size,
					existingArchive.PostID, // Original post where file was first archived
				); err != nil {
					p.api.LogError("Failed to create thread reply with existing attachment", "url", url, "error", err.Error())
					return
				}

				// Store per-post metadata
				if err = p.storageService.StoreArchiveMetadata(metadata); err != nil {
					p.api.LogError("Failed to store archive metadata", "error", err.Error())
				}

				return
			}
		}

		// If ETags don't match or aren't available, we'll download and compare content hash
		// This will be done after download
	}

	// Detect MIME type
	mimeType := ""
	if urlMetadata != nil && urlMetadata.MimeType != "" {
		mimeType = urlMetadata.MimeType
	} else {
		// Fallback to full detection
		var detectedMimeType string
		detectedMimeType, err = p.contentDetector.DetectMimeType(url)
		if err != nil {
			p.api.LogError("Failed to detect MIME type", "url", url, "error", err.Error())
			// Reply with error in thread
			if replyErr := p.threadReplyService.ReplyWithError(postID, url, err); replyErr != nil {
				p.api.LogError("Failed to create error thread reply", "url", url, "error", replyErr.Error())
			}
			return
		}
		mimeType = detectedMimeType
	}

	// Find the appropriate archival tool
	toolName := p.findArchivalTool(url, mimeType, config)
	if toolName == "" {
		err = fmt.Errorf("no archival tool found for MIME type: %s", mimeType)
		p.api.LogWarn("No archival tool found for MIME type", "mimeType", mimeType, "url", url)
		// Reply with error in thread
		if replyErr := p.threadReplyService.ReplyWithError(postID, url, err); replyErr != nil {
			p.api.LogError("Failed to create error thread reply", "url", url, "error", replyErr.Error())
		}
		return
	}

	// If tool is "do_nothing", skip archiving
	if toolName == "do_nothing" {
		p.api.LogInfo("Archival tool is 'do_nothing', skipping archive", "url", url, "mimeType", mimeType)
		return
	}

	// Get the archival tool
	tool, ok := p.archivalTools[toolName]
	if !ok {
		err = fmt.Errorf("archival tool not found: %s", toolName)
		p.api.LogError("Archival tool not found", "toolName", toolName)
		// Reply with error in thread
		if replyErr := p.threadReplyService.ReplyWithError(postID, url, err); replyErr != nil {
			p.api.LogError("Failed to create error thread reply", "url", url, "error", replyErr.Error())
		}
		return
	}

	// Archive the URL
	archivedFile, err := tool.Archive(url, mimeType)
	if err != nil {
		p.api.LogError("Failed to archive URL", "url", url, "error", err.Error())
		// Reply with error in thread
		if replyErr := p.threadReplyService.ReplyWithError(postID, url, err); replyErr != nil {
			p.api.LogError("Failed to create error thread reply", "url", url, "error", replyErr.Error())
		}
		return
	}

	// Check if we have existing archive and compare content hash
	if existingArchive != nil && existingArchive.ContentHash != "" {
		// Calculate hash of newly downloaded content
		hash := sha256.Sum256(archivedFile.Data)
		newContentHash := hex.EncodeToString(hash[:])

		if existingArchive.ContentHash == newContentHash {
			// Content is identical, reuse existing file
			p.api.LogInfo("URL content unchanged (hash match), reusing existing archive", "url", url, "fileID", existingArchive.FileID)
			metadata := p.storageService.CreateMetadataForExistingFile(postID, url, existingArchive)
			// Update ETag if we got one from metadata
			if urlMetadata != nil && urlMetadata.ETag != "" {
				metadata.ETag = urlMetadata.ETag
			}

			// Create thread reply with existing file (include original post ID)
			if err = p.threadReplyService.ReplyWithAttachment(
				postID,
				metadata.FileID,
				url,
				metadata.Filename,
				metadata.MimeType,
				metadata.Size,
				existingArchive.PostID, // Original post where file was first archived
			); err != nil {
				p.api.LogError("Failed to create thread reply with existing attachment", "url", url, "error", err.Error())
				return
			}

			// Store per-post metadata
			if err = p.storageService.StoreArchiveMetadata(metadata); err != nil {
				p.api.LogError("Failed to store archive metadata", "error", err.Error())
			}

			// Update global metadata with new ETag if available
			if urlMetadata != nil && urlMetadata.ETag != "" {
				existingArchive.ETag = urlMetadata.ETag
				existingArchive.ArchivedAt = time.Now()
				if err = p.storageService.StoreGlobalArchiveMetadata(existingArchive); err != nil {
					p.api.LogWarn("Failed to update global archive metadata", "error", err.Error())
				}
			}

			return
		}

		// Content has changed, proceed with new archive
		p.api.LogInfo("URL content changed, creating new archive", "url", url, "oldHash", existingArchive.ContentHash)
	}

	// Store the archived file (new or changed content)
	metadata, err := p.storageService.StoreArchivedFile(postID, url, archivedFile, toolName)
	if err != nil {
		p.api.LogError("Failed to store archived file", "url", url, "error", err.Error())
		// Reply with error in thread
		if replyErr := p.threadReplyService.ReplyWithError(postID, url, err); replyErr != nil {
			p.api.LogError("Failed to create error thread reply", "url", url, "error", replyErr.Error())
		}
		return
	}

	// Store ETag if we got one from metadata
	if urlMetadata != nil && urlMetadata.ETag != "" {
		metadata.ETag = urlMetadata.ETag
	}

	// Create thread reply with attachment (no original post since this is a new archive)
	if err = p.threadReplyService.ReplyWithAttachment(
		postID,
		metadata.FileID,
		url,
		metadata.Filename,
		metadata.MimeType,
		metadata.Size,
		"", // No original post - this is a new archive
	); err != nil {
		p.api.LogError("Failed to create thread reply with attachment", "url", url, "error", err.Error())
		// Don't return - file is already stored
	}

	// Store per-post metadata
	if err = p.storageService.StoreArchiveMetadata(metadata); err != nil {
		p.api.LogError("Failed to store archive metadata", "error", err.Error())
		// Don't return - file is already stored and reply is created
	}

	// Store global metadata (most recent archive for this URL)
	if err = p.storageService.StoreGlobalArchiveMetadata(metadata); err != nil {
		p.api.LogWarn("Failed to store global archive metadata", "error", err.Error())
		// Don't return - per-post metadata is stored
	}

	p.api.LogInfo("Successfully archived URL", "url", url, "postID", postID, "fileID", metadata.FileID)
}

// findArchivalTool finds the appropriate archival tool for a given URL and MIME type
// Rules are evaluated in order, and the first matching rule determines the tool
func (p *ArchiveProcessor) findArchivalTool(urlStr, mimeType string, config *configuration) string {
	// Extract hostname from URL
	hostname := ""
	if parsedURL, err := url.Parse(urlStr); err == nil {
		hostname = parsedURL.Hostname()
	}

	// Log for debugging
	p.api.LogDebug("Finding archival tool", "mimeType", mimeType, "hostname", hostname, "rulesCount", len(config.ArchivalRules))

	// Check archival rules in order
	// The last rule should have an empty pattern and will always match (default rule)
	for i, rule := range config.ArchivalRules {
		p.api.LogDebug("Checking rule", "index", i, "kind", rule.Kind, "pattern", rule.Pattern, "tool", rule.ArchivalTool)
		if p.ruleMatches(hostname, mimeType, rule) {
			p.api.LogInfo("Archival rule matched", "index", i, "hostname", hostname, "mimeType", mimeType, "kind", rule.Kind, "pattern", rule.Pattern, "tool", rule.ArchivalTool)
			return rule.ArchivalTool
		}
	}

	// Fallback to do_nothing if no rules exist (shouldn't happen if default rule is always present)
	p.api.LogInfo("No rules exist, using do_nothing fallback", "hostname", hostname, "mimeType", mimeType)
	return "do_nothing"
}

// ruleMatches checks if a rule matches the given hostname and mimetype
// A rule matches based on its Kind: "hostname" checks hostname, "mimetype" checks mimetype
// An empty pattern means the rule always matches (used for the default rule)
func (p *ArchiveProcessor) ruleMatches(hostname, mimeType string, rule ArchivalRule) bool {
	// Validate rule has required fields
	if rule.Kind == "" {
		return false
	}

	// Empty pattern means always match (default rule)
	if rule.Pattern == "" {
		return true
	}

	// Match based on rule kind
	switch rule.Kind {
	case "hostname":
		return p.hostnameMatches(hostname, rule.Pattern)
	case "mimetype":
		return p.mimeTypeMatches(mimeType, rule.Pattern)
	default:
		// Unknown kind, don't match
		return false
	}
}

// hostnameMatches checks if a hostname matches a pattern
// Supports wildcards like "*.example.com" for subdomain matching
func (p *ArchiveProcessor) hostnameMatches(hostname, pattern string) bool {
	// Exact match
	if hostname == pattern {
		return true
	}

	// Wildcard match: *.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		if suffix == "" {
			return false
		}
		// Match if hostname ends with .suffix or equals suffix
		if hostname == suffix || strings.HasSuffix(hostname, "."+suffix) {
			return true
		}
	}

	return false
}

// mimeTypeMatches checks if a MIME type matches a pattern
// Supports wildcards like "image/*" or exact matches like "application/pdf"
func (p *ArchiveProcessor) mimeTypeMatches(mimeType, pattern string) bool {
	// Exact match
	if mimeType == pattern {
		return true
	}

	// Wildcard match (e.g., "image/*" matches "image/jpeg", "image/png", etc.)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(mimeType, prefix+"/")
	}

	return false
}
