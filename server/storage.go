package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"

	"github.com/fmartingrmattermost-plugin-link-archiver/server/archiver"
)

// ArchiveMetadata stores metadata about an archived file
type ArchiveMetadata struct {
	PostID      string    `json:"postId"`
	OriginalURL string    `json:"originalUrl"`
	FileID      string    `json:"fileId"`
	Filename    string    `json:"filename"`
	MimeType    string    `json:"mimeType"`
	ArchivedAt  time.Time `json:"archivedAt"`
	ToolUsed    string    `json:"toolUsed"`
	Size        int64     `json:"size"`
	ETag        string    `json:"etag,omitempty"`
	ContentHash string    `json:"contentHash,omitempty"`
}

// StorageService handles storing archived files in Mattermost
type StorageService struct {
	api plugin.API
}

// NewStorageService creates a new storage service
func NewStorageService(api plugin.API) *StorageService {
	return &StorageService{
		api: api,
	}
}

// StoreArchivedFile stores an archived file in Mattermost file storage
// and associates it with the given post
func (s *StorageService) StoreArchivedFile(postID, originalURL string, archivedFile *archiver.ArchivedFile, toolName string) (*ArchiveMetadata, error) {
	if archivedFile == nil {
		return nil, errors.New("archived file is nil")
	}

	// Get the post to find the channel ID
	post, appErr := s.api.GetPost(postID)
	if appErr != nil {
		return nil, errors.Wrap(appErr, "failed to get post")
	}

	// Upload the file to Mattermost using the plugin API
	fileInfo, appErr := s.api.UploadFile(
		archivedFile.Data,
		post.ChannelId,
		archivedFile.Filename,
	)
	if appErr != nil {
		return nil, errors.Wrap(appErr, "failed to upload file to Mattermost")
	}

	// Calculate content hash
	hash := sha256.Sum256(archivedFile.Data)
	contentHash := hex.EncodeToString(hash[:])

	// Create metadata
	metadata := &ArchiveMetadata{
		PostID:      postID,
		OriginalURL: originalURL,
		FileID:      fileInfo.Id,
		Filename:    archivedFile.Filename,
		MimeType:    archivedFile.MimeType,
		ArchivedAt:  time.Now(),
		ToolUsed:    toolName,
		Size:        archivedFile.Size,
		ContentHash: contentHash,
	}

	return metadata, nil
}

// CreateMetadataForExistingFile creates metadata for an existing file (reused archive)
func (s *StorageService) CreateMetadataForExistingFile(postID, originalURL string, existingMetadata *ArchiveMetadata) *ArchiveMetadata {
	return &ArchiveMetadata{
		PostID:      postID,
		OriginalURL: originalURL,
		FileID:      existingMetadata.FileID,
		Filename:    existingMetadata.Filename,
		MimeType:    existingMetadata.MimeType,
		ArchivedAt:  time.Now(),
		ToolUsed:    existingMetadata.ToolUsed,
		Size:        existingMetadata.Size,
		ETag:        existingMetadata.ETag,
		ContentHash: existingMetadata.ContentHash,
	}
}

// StoreArchiveMetadata stores archive metadata in KV store (per-post)
func (s *StorageService) StoreArchiveMetadata(metadata *ArchiveMetadata) error {
	// Store metadata keyed by post ID and URL hash
	key := getArchiveMetadataKey(metadata.PostID, metadata.OriginalURL)

	// Get existing metadata for this post
	existing, appErr := s.api.KVGet(key)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to get existing metadata")
	}

	// If metadata already exists, append to list
	var metadataList []*ArchiveMetadata
	if existing != nil {
		if err := json.Unmarshal(existing, &metadataList); err != nil {
			// If unmarshal fails, start fresh
			metadataList = []*ArchiveMetadata{metadata}
		} else {
			metadataList = append(metadataList, metadata)
		}
	} else {
		metadataList = []*ArchiveMetadata{metadata}
	}

	// Store updated metadata
	data, err := json.Marshal(metadataList)
	if err != nil {
		return errors.Wrap(err, "failed to marshal metadata")
	}

	appErr = s.api.KVSet(key, data)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to store metadata")
	}

	return nil
}

// getArchiveMetadataKey generates a KV store key for archive metadata (per-post)
func getArchiveMetadataKey(postID, url string) string {
	// Hash the URL to keep key length within limits
	hash := sha256.Sum256([]byte(url))
	urlHash := hex.EncodeToString(hash[:])
	return "archive_post_" + postID + "_" + urlHash
}

// getGlobalArchiveKey generates a KV store key for global URL archive metadata
// Uses hash of URL to keep key within 150 character limit
func getGlobalArchiveKey(url string) string {
	hash := sha256.Sum256([]byte(url))
	urlHash := hex.EncodeToString(hash[:])
	return "archive_url_" + urlHash
}

// IsURLAlreadyArchived checks if a URL has already been archived for a given post
func (s *StorageService) IsURLAlreadyArchived(postID, url string) (bool, error) {
	key := getArchiveMetadataKey(postID, url)
	existing, appErr := s.api.KVGet(key)
	if appErr != nil {
		return false, errors.Wrap(appErr, "failed to check if URL is already archived")
	}

	// If no metadata exists, URL hasn't been archived
	if existing == nil {
		return false, nil
	}

	// Unmarshal to check if this exact URL is in the list
	var metadataList []*ArchiveMetadata
	if err := json.Unmarshal(existing, &metadataList); err != nil {
		// If unmarshal fails, assume not archived
		return false, nil
	}

	// Check if any entry matches this URL
	for _, m := range metadataList {
		if m.OriginalURL == url {
			return true, nil
		}
	}

	return false, nil
}

// GetExistingArchiveForURL retrieves the most recent archive metadata for a URL (globally)
func (s *StorageService) GetExistingArchiveForURL(url string) (*ArchiveMetadata, error) {
	key := getGlobalArchiveKey(url)
	existing, appErr := s.api.KVGet(key)
	if appErr != nil {
		return nil, errors.Wrap(appErr, "failed to get existing archive for URL")
	}

	if existing == nil {
		return nil, nil
	}

	var metadata ArchiveMetadata
	if err := json.Unmarshal(existing, &metadata); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal existing archive metadata")
	}

	return &metadata, nil
}

// StoreGlobalArchiveMetadata stores the most recent archive metadata for a URL (globally)
func (s *StorageService) StoreGlobalArchiveMetadata(metadata *ArchiveMetadata) error {
	key := getGlobalArchiveKey(metadata.OriginalURL)
	data, err := json.Marshal(metadata)
	if err != nil {
		return errors.Wrap(err, "failed to marshal global archive metadata")
	}

	appErr := s.api.KVSet(key, data)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to store global archive metadata")
	}

	return nil
}
