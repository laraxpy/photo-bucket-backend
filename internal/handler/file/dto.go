package file

// DownloadZipRequest selects which files go into a bulk zip download.
// At least one of FileIDs or FolderID must be provided; if both are given,
// the result is their union (deduplicated).
type DownloadZipRequest struct {
	FileIDs  []string `json:"fileIds"`
	FolderID string   `json:"folderId"`
}
