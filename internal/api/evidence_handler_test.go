package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/persistence"
	"github.com/hemanth-shankar/expense-agent/internal/storage"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

type evidenceClaimRepository struct {
	result *workflow.Result
}

func (f *evidenceClaimRepository) Create(
	ctx context.Context,
	result *workflow.Result,
) error {
	return nil
}

func (f *evidenceClaimRepository) Get(
	ctx context.Context,
	claimID string,
) (*workflow.Result, error) {
	if f.result == nil {
		return nil, persistence.ErrClaimNotFound
	}
	return f.result, nil
}

func (f *evidenceClaimRepository) Update(
	ctx context.Context,
	result *workflow.Result,
) error {
	f.result = result
	return nil
}

func (f *evidenceClaimRepository) AppendEvent(
	ctx context.Context,
	claimID string,
	eventType string,
	payload any,
) error {
	return nil
}

func (f *evidenceClaimRepository) GetEvents(
	ctx context.Context,
	claimID string,
) ([]persistence.ClaimEvent, error) {
	return nil, nil
}

type evidenceDocumentStore struct {
	data []byte
	err  error
}

func (f *evidenceDocumentStore) Put(
	ctx context.Context,
	originalName string,
	mimeType string,
	sha256 string,
	data []byte,
) (*storage.StoredDocument, error) {
	return nil, errors.New("not implemented")
}

func (f *evidenceDocumentStore) Get(
	ctx context.Context,
	key string,
) ([]byte, error) {
	return f.data, f.err
}

func (f *evidenceDocumentStore) Exists(
	ctx context.Context,
	key string,
) (bool, error) {
	return f.err == nil, f.err
}

func TestGetClaimDocument(t *testing.T) {
	repository := &evidenceClaimRepository{
		result: &workflow.Result{
			RunID: "claim-1",
			StoredDocument: &storage.StoredDocument{
				Key:          "ab/receipt.png",
				OriginalName: "receipt.png",
				MIMEType:     "image/png",
			},
		},
	}

	documentStore := &evidenceDocumentStore{
		data: []byte("fake-png-bytes"),
	}

	handler := &Handler{
		repository:    repository,
		documentStore: documentStore,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /api/v1/claims/{claimID}/document",
		handler.getClaimDocument,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/claims/claim-1/document",
		nil,
	)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("unexpected content type: %s", response.Header().Get("Content-Type"))
	}

	if response.Body.String() != "fake-png-bytes" {
		t.Fatalf("unexpected document bytes: %q", response.Body.String())
	}
}
