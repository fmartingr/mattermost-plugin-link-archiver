package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
)

// ThreadReplyService handles creating thread replies with attachments and error messages
type ThreadReplyService struct {
	api   plugin.API
	botID string
}

// NewThreadReplyService creates a new thread reply service
func NewThreadReplyService(api plugin.API, botID string) *ThreadReplyService {
	return &ThreadReplyService{
		api:   api,
		botID: botID,
	}
}

// ReplyWithAttachment creates a thread reply with a file attachment and success message
// originalPostID is optional - if provided, a link to the original post will be included
func (t *ThreadReplyService) ReplyWithAttachment(postID, fileID, url, filename, mimeType string, size int64, originalPostID string) error {
	// Get the original post to get channel ID and determine root ID
	post, appErr := t.api.GetPost(postID)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to get original post")
	}

	// Determine the root ID for the thread
	// If the post is already a reply (has RootId), use that. Otherwise, use the post ID itself.
	rootID := postID
	if post.RootId != "" {
		rootID = post.RootId
	}

	// Format success message
	message := fmt.Sprintf("✅ Successfully archived: %s\n\n**File:** %s\n**Size:** %s\n**Type:** %s",
		url,
		filename,
		formatFileSize(size),
		mimeType,
	)

	// If originalPostID is provided and different from current post, add link to original post
	if originalPostID != "" && originalPostID != postID {
		// Get the original post to construct the permalink
		originalPost, appErr := t.api.GetPost(originalPostID)
		if appErr == nil && originalPost != nil {
			// Get the channel to find the team
			channel, appErr := t.api.GetChannel(originalPost.ChannelId)
			if appErr == nil && channel != nil {
				var permalink string
				// For team channels, include team name in permalink: /<team-name>/pl/<post-id>
				// For DM/GM channels, use simple format: /pl/<post-id>
				if channel.TeamId != "" {
					team, appErr := t.api.GetTeam(channel.TeamId)
					if appErr == nil && team != nil {
						permalink = fmt.Sprintf("/%s/pl/%s", team.Name, originalPostID)
					} else {
						// Fallback to simple format if team lookup fails
						permalink = fmt.Sprintf("/pl/%s", originalPostID)
					}
				} else {
					// DM or GM channel - use simple format
					permalink = fmt.Sprintf("/pl/%s", originalPostID)
				}
				message += fmt.Sprintf("\n\n📎 Originally archived in [this post](%s)", permalink)
			}
		}
	}

	// Create thread reply post
	replyPost := &model.Post{
		UserId:    t.botID,
		ChannelId: post.ChannelId,
		RootId:    rootID,
		Message:   message,
		FileIds:   []string{fileID},
		CreateAt:  model.GetMillis(),
	}

	_, appErr = t.api.CreatePost(replyPost)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to create thread reply")
	}

	return nil
}

// ReplyWithError creates a thread reply with an error message
func (t *ThreadReplyService) ReplyWithError(postID, url string, err error) error {
	// Get the original post to get channel ID and determine root ID
	post, appErr := t.api.GetPost(postID)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to get original post")
	}

	// Determine the root ID for the thread
	// If the post is already a reply (has RootId), use that. Otherwise, use the post ID itself.
	rootID := postID
	if post.RootId != "" {
		rootID = post.RootId
	}

	// Format error message
	errorMsg := err.Error()
	reason := extractErrorReason(err)

	message := fmt.Sprintf("❌ Failed to archive: %s\n\n**Error:** %s\n**Reason:** %s",
		url,
		errorMsg,
		reason,
	)

	// Create thread reply post
	replyPost := &model.Post{
		UserId:    t.botID,
		ChannelId: post.ChannelId,
		RootId:    rootID,
		Message:   message,
		CreateAt:  model.GetMillis(),
	}

	_, appErr = t.api.CreatePost(replyPost)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to create error thread reply")
	}

	return nil
}

// formatFileSize formats file size in human-readable format
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// extractErrorReason extracts a user-friendly reason from an error
func extractErrorReason(err error) string {
	errStr := err.Error()

	// Check for common error patterns
	if contains(errStr, "timeout") || contains(errStr, "Timeout") {
		return "Timeout while fetching URL"
	}
	if contains(errStr, "invalid URL") || contains(errStr, "Invalid URL") {
		return "Invalid URL format"
	}
	if contains(errStr, "download") && contains(errStr, "failed") {
		return "Failed to download file"
	}
	if contains(errStr, "store") && contains(errStr, "failed") {
		return "Failed to store file"
	}
	if contains(errStr, "MIME type") || contains(errStr, "content type") {
		return "Could not determine content type"
	}
	if contains(errStr, "too large") || contains(errStr, "exceeds maximum") {
		return "File too large"
	}
	if contains(errStr, "status") && contains(errStr, "4") {
		return "HTTP client error"
	}
	if contains(errStr, "status") && contains(errStr, "5") {
		return "HTTP server error"
	}

	return "Unknown error"
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}
