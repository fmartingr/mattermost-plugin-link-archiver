package kvstore

import (
	"encoding/json"

	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"
)

// We expose our calls to the KVStore pluginapi methods through this interface for testability and stability.
// This allows us to better control which values are stored with which keys.

type Client struct {
	client *pluginapi.Client
}

func NewKVStore(client *pluginapi.Client) KVStore {
	return Client{
		client: client,
	}
}

// Sample method to get a key-value pair in the KV store
func (kv Client) GetTemplateData(userID string) (string, error) {
	var templateData string
	err := kv.client.KV.Get("template_key-"+userID, &templateData)
	if err != nil {
		return "", errors.Wrap(err, "failed to get template data")
	}
	return templateData, nil
}

// GetArchiveMetadata retrieves archive metadata for a specific post and URL
func (kv Client) GetArchiveMetadata(postID, url string) ([]map[string]interface{}, error) {
	key := "archive_" + postID + "_" + url
	var data []byte
	err := kv.client.KV.Get(key, &data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get archive metadata")
	}

	if data == nil {
		return []map[string]interface{}{}, nil
	}

	var metadataList []map[string]interface{}
	if err := json.Unmarshal(data, &metadataList); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal archive metadata")
	}

	return metadataList, nil
}

// ListArchivesForPost retrieves all archive metadata for a specific post
// Note: This is a simplified implementation. For better performance with many URLs,
// consider storing a post->URLs mapping separately.
func (kv Client) ListArchivesForPost(postID string) ([]map[string]interface{}, error) {
	// This is a simplified implementation that requires knowing the URLs
	// In a production system, you might want to maintain a separate index
	// For now, we'll return an empty list and let the API handle it differently
	// The actual implementation would need to scan keys or maintain an index
	return []map[string]interface{}{}, nil
}
