package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalDocumentStorePutAndGet(
	t *testing.T,
) {

	root :=
		t.TempDir()

	store :=
		NewLocalDocumentStore(root)

	data :=
		[]byte(
			"test-receipt-content",
		)

	hash :=
		testSHA256(data)

	stored, err :=
		store.Put(
			context.Background(),
			"receipt.jpg",
			"image/jpeg",
			hash,
			data,
		)

	if err != nil {
		t.Fatal(err)
	}

	if stored.SHA256 != hash {

		t.Fatalf(
			"expected hash %s, got %s",
			hash,
			stored.SHA256,
		)
	}

	if stored.OriginalName !=
		"receipt.jpg" {

		t.Fatalf(
			"unexpected original name %s",
			stored.OriginalName,
		)
	}

	if stored.Size !=
		int64(len(data)) {

		t.Fatalf(
			"expected size %d, got %d",
			len(data),
			stored.Size,
		)
	}

	exists, err :=
		store.Exists(
			context.Background(),
			stored.Key,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !exists {

		t.Fatal(
			"expected stored document to exist",
		)
	}

	retrieved, err :=
		store.Get(
			context.Background(),
			stored.Key,
		)

	if err != nil {
		t.Fatal(err)
	}

	if string(retrieved) !=
		string(data) {

		t.Fatal(
			"retrieved document differs from original",
		)
	}
}

func TestLocalDocumentStoreDeduplicatesContent(
	t *testing.T,
) {

	root :=
		t.TempDir()

	store :=
		NewLocalDocumentStore(root)

	data :=
		[]byte(
			"identical-document",
		)

	hash :=
		testSHA256(data)

	first, err :=
		store.Put(
			context.Background(),
			"first.jpg",
			"image/jpeg",
			hash,
			data,
		)

	if err != nil {
		t.Fatal(err)
	}

	second, err :=
		store.Put(
			context.Background(),
			"renamed.jpg",
			"image/jpeg",
			hash,
			data,
		)

	if err != nil {
		t.Fatal(err)
	}

	if first.Key !=
		second.Key {

		t.Fatalf(
			"expected identical content to use same key",
		)
	}

	files :=
		0

	err =
		filepath.Walk(
			root,
			func(
				path string,
				info os.FileInfo,
				err error,
			) error {

				if err != nil {
					return err
				}

				if !info.IsDir() {
					files++
				}

				return nil
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if files != 1 {

		t.Fatalf(
			"expected 1 physical document, found %d",
			files,
		)
	}
}

func TestLocalDocumentStoreRejectsHashMismatch(
	t *testing.T,
) {

	store :=
		NewLocalDocumentStore(
			t.TempDir(),
		)

	_, err :=
		store.Put(
			context.Background(),
			"receipt.pdf",
			"application/pdf",
			"incorrect-hash",
			[]byte(
				"pdf-content",
			),
		)

	if err == nil {

		t.Fatal(
			"expected SHA-256 mismatch error",
		)
	}
}

func TestLocalDocumentStoreRejectsTraversal(
	t *testing.T,
) {

	store :=
		NewLocalDocumentStore(
			t.TempDir(),
		)

	_, err :=
		store.Get(
			context.Background(),
			"../../secret.txt",
		)

	if err == nil {

		t.Fatal(
			"expected invalid storage key error",
		)
	}
}

func testSHA256(
	data []byte,
) string {

	hash :=
		sha256.Sum256(data)

	return hex.EncodeToString(
		hash[:],
	)
}
