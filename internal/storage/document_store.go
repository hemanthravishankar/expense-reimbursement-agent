package storage

import "context"

type StoredDocument struct {
	Key          string `json:"key"`
	OriginalName string `json:"original_name"`
	MIMEType     string `json:"mime_type"`
	SHA256       string `json:"sha256"`
	Size         int64  `json:"size"`
}

type DocumentStore interface {
	Put(
		ctx context.Context,
		originalName string,
		mimeType string,
		sha256 string,
		data []byte,
	) (*StoredDocument, error)

	Get(
		ctx context.Context,
		key string,
	) ([]byte, error)

	Exists(
		ctx context.Context,
		key string,
	) (bool, error)
}
