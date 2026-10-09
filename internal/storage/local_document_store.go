package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LocalDocumentStore struct {
	rootDirectory string
}

func NewLocalDocumentStore(
	rootDirectory string,
) *LocalDocumentStore {

	return &LocalDocumentStore{
		rootDirectory: rootDirectory,
	}
}

func (s *LocalDocumentStore) Put(
	ctx context.Context,
	originalName string,
	mimeType string,
	expectedSHA256 string,
	data []byte,
) (*StoredDocument, error) {

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf(
			"cannot store empty document",
		)
	}

	actualSHA256 :=
		calculateSHA256(data)

	if actualSHA256 != expectedSHA256 {
		return nil, fmt.Errorf(
			"document SHA-256 mismatch: expected %s, calculated %s",
			expectedSHA256,
			actualSHA256,
		)
	}

	extension, err :=
		extensionForMIMEType(
			mimeType,
		)

	if err != nil {
		return nil, err
	}

	//
	// Content-addressed storage:
	//
	// data/documents/
	// └── 9a/
	//     └── 9afa1212....jpg
	//
	key :=
		filepath.ToSlash(
			filepath.Join(
				actualSHA256[:2],
				actualSHA256+extension,
			),
		)

	path, err :=
		s.resolveKey(key)

	if err != nil {
		return nil, err
	}

	if err :=
		os.MkdirAll(
			filepath.Dir(path),
			0755,
		); err != nil {

		return nil, fmt.Errorf(
			"create document storage directory: %w",
			err,
		)
	}

	//
	// O_EXCL prevents accidentally overwriting
	// an existing object.
	//
	file, err :=
		os.OpenFile(
			path,
			os.O_WRONLY|
				os.O_CREATE|
				os.O_EXCL,
			0600,
		)

	if err != nil {

		if os.IsExist(err) {

			info, statErr :=
				os.Stat(path)

			if statErr != nil {
				return nil, fmt.Errorf(
					"stat existing document: %w",
					statErr,
				)
			}

			return &StoredDocument{
				Key: key,

				OriginalName: filepath.Base(
					originalName,
				),

				MIMEType: mimeType,

				SHA256: actualSHA256,

				Size: info.Size(),
			}, nil
		}

		return nil, fmt.Errorf(
			"create stored document: %w",
			err,
		)
	}

	writeSucceeded :=
		false

	defer func() {

		_ = file.Close()

		if !writeSucceeded {
			_ = os.Remove(path)
		}
	}()

	if _, err :=
		file.Write(data); err != nil {

		return nil, fmt.Errorf(
			"write stored document: %w",
			err,
		)
	}

	if err :=
		file.Sync(); err != nil {

		return nil, fmt.Errorf(
			"sync stored document: %w",
			err,
		)
	}

	if err :=
		file.Close(); err != nil {

		return nil, fmt.Errorf(
			"close stored document: %w",
			err,
		)
	}

	writeSucceeded =
		true

	return &StoredDocument{
		Key: key,

		OriginalName: filepath.Base(
			originalName,
		),

		MIMEType: mimeType,

		SHA256: actualSHA256,

		Size: int64(
			len(data),
		),
	}, nil
}

func (s *LocalDocumentStore) Get(
	ctx context.Context,
	key string,
) ([]byte, error) {

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path, err :=
		s.resolveKey(key)

	if err != nil {
		return nil, err
	}

	data, err :=
		os.ReadFile(path)

	if err != nil {
		return nil, fmt.Errorf(
			"read stored document: %w",
			err,
		)
	}

	return data, nil
}

func (s *LocalDocumentStore) Exists(
	ctx context.Context,
	key string,
) (bool, error) {

	if err := ctx.Err(); err != nil {
		return false, err
	}

	path, err :=
		s.resolveKey(key)

	if err != nil {
		return false, err
	}

	_, err =
		os.Stat(path)

	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf(
		"check stored document: %w",
		err,
	)
}

func (s *LocalDocumentStore) resolveKey(
	key string,
) (string, error) {

	clean :=
		filepath.Clean(
			filepath.FromSlash(key),
		)

	if clean == "." ||
		clean == ".." ||
		filepath.IsAbs(clean) ||
		strings.HasPrefix(
			clean,
			".."+string(os.PathSeparator),
		) {

		return "", fmt.Errorf(
			"invalid document storage key",
		)
	}

	return filepath.Join(
		s.rootDirectory,
		clean,
	), nil
}

func extensionForMIMEType(
	mimeType string,
) (string, error) {

	switch mimeType {

	case "image/jpeg":
		return ".jpg", nil

	case "image/png":
		return ".png", nil

	case "application/pdf":
		return ".pdf", nil

	default:

		return "", fmt.Errorf(
			"unsupported document MIME type %q",
			mimeType,
		)
	}
}

func calculateSHA256(
	data []byte,
) string {

	hash :=
		sha256.Sum256(data)

	return hex.EncodeToString(
		hash[:],
	)
}
