package finance

import (
	"context"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

type fakeRepository struct {
	result *workflow.Result
	events []string
}

func (f *fakeRepository) Get(
	ctx context.Context,
	claimID string,
) (*workflow.Result, error) {
	return f.result, nil
}

func (f *fakeRepository) Update(
	ctx context.Context,
	result *workflow.Result,
) error {
	f.result = result
	return nil
}

func (f *fakeRepository) AppendEvent(
	ctx context.Context,
	claimID string,
	eventType string,
	payload any,
) error {
	f.events = append(f.events, eventType)
	return nil
}

func reviewableMealClaim() *workflow.Result {
	return &workflow.Result{
		RunID:       "claim-1",
		FinalStatus: workflow.StatusManualReview,
		Employee: models.Employee{
			ID:    "EMP60003",
			Grade: "B2",
		},
		FinalExtraction: &models.ReceiptExtraction{
			Merchant:           "THE DAILY GRILL",
			ExpenseDate:        "2024-02-12",
			Subtotal:           48,
			TaxAmount:          2.4,
			ServiceCharge:      4.8,
			TotalAmount:        55.2,
			Currency:           "USD",
			CategorySuggestion: "Meals",
		},
		FinalValidation: validation.ReceiptValidationResult{
			Valid: true,
		},
	}
}

func TestApproveReviewRunsPolicy(t *testing.T) {
	repo := &fakeRepository{result: reviewableMealClaim()}
	service := NewService(repo, policy.DemoUSDPolicy())

	result, err := service.ReviewClaim(
		context.Background(),
		"claim-1",
		ReviewRequest{
			Action:   ActionApprove,
			Reviewer: "Finance User",
			Note:     "Verified as legitimate.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.FinalStatus != policy.StatusLimitExceeded {
		t.Fatalf("expected LIMIT_EXCEEDED, got %s", result.FinalStatus)
	}

	if result.PolicyDecision == nil {
		t.Fatal("expected policy decision after approval")
	}

	if result.PolicyDecision.ReimbursableAmount != 50 {
		t.Fatalf("expected reimbursable 50, got %.2f", result.PolicyDecision.ReimbursableAmount)
	}

	if len(repo.events) != 2 || repo.events[0] != "FINANCE_REVIEWED" || repo.events[1] != "CLAIM_STATUS_CHANGED" {
		t.Fatalf("unexpected events: %#v", repo.events)
	}
}

func TestRejectReviewKeepsZeroReimbursement(t *testing.T) {
	repo := &fakeRepository{result: reviewableMealClaim()}
	service := NewService(repo, policy.DemoUSDPolicy())

	result, err := service.ReviewClaim(
		context.Background(),
		"claim-1",
		ReviewRequest{
			Action:   ActionReject,
			Reviewer: "Finance User",
			Note:     "Confirmed duplicate reimbursement risk.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.FinalStatus != StatusRejected {
		t.Fatalf("expected REJECTED, got %s", result.FinalStatus)
	}

	if result.PolicyDecision != nil {
		t.Fatal("expected policy decision to remain nil after rejection")
	}
}

func TestReviewRequiresNote(t *testing.T) {
	repo := &fakeRepository{result: reviewableMealClaim()}
	service := NewService(repo, policy.DemoUSDPolicy())

	_, err := service.ReviewClaim(
		context.Background(),
		"claim-1",
		ReviewRequest{
			Action:   ActionApprove,
			Reviewer: "Finance User",
		},
	)

	if err != ErrReviewNoteRequired {
		t.Fatalf("expected ErrReviewNoteRequired, got %v", err)
	}
}

type fakeActivePolicyProvider struct {
	companyPolicy policy.Policy
}

func (f fakeActivePolicyProvider) ActivePolicy(
	ctx context.Context,
) (policy.Policy, error) {
	return f.companyPolicy, nil
}

func TestApproveReviewUsesActivePolicyProvider(t *testing.T) {
	repo := &fakeRepository{result: reviewableMealClaim()}

	active := policy.DemoUSDPolicy()
	active.ID = "POL-NEW"
	active.Version = 2
	active.Limits["B2"]["Meals"] = policy.CategoryRule{
		Allowed:   true,
		MaxAmount: 60,
	}

	service := NewService(
		repo,
		policy.DemoUSDPolicy(),
		fakeActivePolicyProvider{companyPolicy: active},
	)

	result, err := service.ReviewClaim(
		context.Background(),
		"claim-1",
		ReviewRequest{
			Action:   ActionApprove,
			Reviewer: "Finance User",
			Note:     "Verified as legitimate.",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.FinalStatus != policy.StatusApproved {
		t.Fatalf("expected APPROVED, got %s", result.FinalStatus)
	}

	if result.PolicyDecision == nil {
		t.Fatal("expected policy decision after approval")
	}

	if result.PolicyDecision.PolicyID != "POL-NEW" ||
		result.PolicyDecision.PolicyVersion != 2 {
		t.Fatalf(
			"expected POL-NEW v2, got %s v%d",
			result.PolicyDecision.PolicyID,
			result.PolicyDecision.PolicyVersion,
		)
	}

	if result.PolicyDecision.ReimbursableAmount != 55.2 {
		t.Fatalf(
			"expected reimbursable 55.20 under active policy, got %.2f",
			result.PolicyDecision.ReimbursableAmount,
		)
	}
}
