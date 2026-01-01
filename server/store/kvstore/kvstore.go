package kvstore

type KVStore interface {
	// Define your methods here. This package is used to access the KVStore pluginapi methods.
	GetTemplateData(userID string) (string, error)

	// Archive metadata methods
	// Note: ArchiveMetadata is defined in the main package, so these methods
	// would need to be implemented in the main package or use a shared types package
	GetArchiveMetadata(postID, url string) ([]map[string]interface{}, error)
	ListArchivesForPost(postID string) ([]map[string]interface{}, error)
}
