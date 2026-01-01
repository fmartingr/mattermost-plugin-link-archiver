package archiver

// ArchivedFile represents a file that has been archived
type ArchivedFile struct {
	Filename string
	Data     []byte
	MimeType string
	Size     int64
}

// ArchivalTool is the interface for archival tools
type ArchivalTool interface {
	Archive(url string, mimeType string) (*ArchivedFile, error)
	Name() string
}
