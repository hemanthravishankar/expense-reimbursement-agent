package policy

import (
	"context"
	"errors"
	"testing"
)

type fakeManagementStore struct {
	created     CreateVersionRequest
	updated     UpdateVersionRequest
	publishedBy string
}

func (f *fakeManagementStore) ActivePolicy(
	ctx context.Context,
) (Policy, error) {
	return DemoUSDPolicy(), nil
}

func (f *fakeManagementStore) ListPolicyVersions(
	ctx context.Context,
) ([]Version, error) {
	return []Version{}, nil
}

func (f *fakeManagementStore) GetPolicyVersion(
	ctx context.Context,
	policyID string,
) (*Version, error) {
	return &Version{PolicyID: policyID}, nil
}

func (f *fakeManagementStore) CreatePolicyVersion(
	ctx context.Context,
	request CreateVersionRequest,
) (*Version, error) {
	f.created = request
	return &Version{PolicyID: "POL-1", Name: request.Name}, nil
}

func (f *fakeManagementStore) UpdatePolicyVersion(
	ctx context.Context,
	policyID string,
	request UpdateVersionRequest,
) (*Version, error) {
	f.updated = request
	return &Version{PolicyID: policyID}, nil
}

func (f *fakeManagementStore) PublishPolicyVersion(
	ctx context.Context,
	policyID string,
	publishedBy string,
) (*Version, error) {
	f.publishedBy = publishedBy
	return &Version{PolicyID: policyID, Status: VersionStatusPublished}, nil
}

func TestManagerCreateNormalizesAndValidates(t *testing.T) {
	store := &fakeManagementStore{}
	manager := NewManager(store)

	_, err := manager.Create(
		context.Background(),
		CreateVersionRequest{
			Name:          " Expense Policy ",
			Currency:      " usd ",
			EffectiveFrom: "2026-10-10",
			CreatedBy:     " finance.admin ",
			Rules: []Rule{
				{
					EmployeeGrade: " B2 ",
					Category:      " Meals ",
					Allowed:       true,
					MaxAmount:     60,
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if store.created.Currency != "USD" {
		t.Fatalf("expected normalized USD currency, got %q", store.created.Currency)
	}

	if store.created.Rules[0].EmployeeGrade != "B2" {
		t.Fatalf("expected normalized grade, got %q", store.created.Rules[0].EmployeeGrade)
	}
}

func TestManagerRejectsDuplicateRules(t *testing.T) {
	manager := NewManager(&fakeManagementStore{})

	_, err := manager.Create(
		context.Background(),
		CreateVersionRequest{
			Name:          "Expense Policy",
			Currency:      "USD",
			EffectiveFrom: "2026-10-10",
			CreatedBy:     "finance.admin",
			Rules: []Rule{
				{EmployeeGrade: "B2", Category: "Meals", Allowed: true, MaxAmount: 50},
				{EmployeeGrade: "b2", Category: "meals", Allowed: true, MaxAmount: 60},
			},
		},
	)

	if !errors.Is(err, ErrInvalidPolicyRule) {
		t.Fatalf("expected ErrInvalidPolicyRule, got %v", err)
	}
}

func TestManagerRequiresPublisher(t *testing.T) {
	manager := NewManager(&fakeManagementStore{})

	_, err := manager.Publish(
		context.Background(),
		"POL-1",
		PublishRequest{},
	)

	if !errors.Is(err, ErrPolicyPublisher) {
		t.Fatalf("expected ErrPolicyPublisher, got %v", err)
	}
}
