package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/policy"
)

type fakePolicyStore struct {
	active    policy.Policy
	versions  []policy.Version
	created   policy.CreateVersionRequest
	published string
}

func (f *fakePolicyStore) ActivePolicy(
	ctx context.Context,
) (policy.Policy, error) {
	return f.active, nil
}

func (f *fakePolicyStore) ListPolicyVersions(
	ctx context.Context,
) ([]policy.Version, error) {
	return f.versions, nil
}

func (f *fakePolicyStore) GetPolicyVersion(
	ctx context.Context,
	policyID string,
) (*policy.Version, error) {
	for _, item := range f.versions {
		if item.PolicyID == policyID {
			copy := item
			return &copy, nil
		}
	}
	return nil, policy.ErrPolicyNotFound
}

func (f *fakePolicyStore) CreatePolicyVersion(
	ctx context.Context,
	request policy.CreateVersionRequest,
) (*policy.Version, error) {
	f.created = request
	return &policy.Version{
		PolicyID: "POL-2",
		Name:     request.Name,
		Version:  2,
		Currency: request.Currency,
		Status:   policy.VersionStatusDraft,
		Rules:    request.Rules,
	}, nil
}

func (f *fakePolicyStore) UpdatePolicyVersion(
	ctx context.Context,
	policyID string,
	request policy.UpdateVersionRequest,
) (*policy.Version, error) {
	return &policy.Version{
		PolicyID: policyID,
		Currency: request.Currency,
		Status:   policy.VersionStatusDraft,
		Rules:    request.Rules,
	}, nil
}

func (f *fakePolicyStore) PublishPolicyVersion(
	ctx context.Context,
	policyID string,
	publishedBy string,
) (*policy.Version, error) {
	f.published = publishedBy
	return &policy.Version{
		PolicyID:    policyID,
		Status:      policy.VersionStatusPublished,
		PublishedBy: publishedBy,
	}, nil
}

func TestGetActivePolicy(t *testing.T) {
	store := &fakePolicyStore{
		active: policy.DemoUSDPolicy(),
	}

	handler := &Handler{
		policyManager: policy.NewManager(store),
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/policies/active",
		nil,
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	var active policy.Policy
	if err := json.Unmarshal(response.Body.Bytes(), &active); err != nil {
		t.Fatal(err)
	}

	if active.Name == "" || active.Currency != "USD" {
		t.Fatalf("unexpected active policy: %+v", active)
	}
}

func TestCreatePolicy(t *testing.T) {
	store := &fakePolicyStore{}
	handler := &Handler{
		policyManager: policy.NewManager(store),
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body := []byte(`{
		"name":"NovaTech Expense Policy",
		"currency":"USD",
		"effective_from":"2026-10-10",
		"created_by":"finance.admin",
		"rules":[{
			"employee_grade":"B2",
			"category":"Meals",
			"allowed":true,
			"max_amount":60
		}]
	}`)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/policies",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}

	if store.created.Rules[0].MaxAmount != 60 {
		t.Fatalf("expected stored meal limit 60, got %.2f", store.created.Rules[0].MaxAmount)
	}
}

func TestPublishPolicy(t *testing.T) {
	store := &fakePolicyStore{}
	handler := &Handler{
		policyManager: policy.NewManager(store),
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/policies/POL-2/publish",
		bytes.NewBufferString(`{"published_by":"finance.admin"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if store.published != "finance.admin" {
		t.Fatalf("expected publisher finance.admin, got %q", store.published)
	}
}
