package claims

import (
	"context"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/clarification"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

type fakeRepository struct {
	claim *workflow.Result

	events []string

	updateCalled bool
}

func (f *fakeRepository) Get(
	ctx context.Context,
	claimID string,
) (*workflow.Result, error) {

	return f.claim,
		nil
}

func (f *fakeRepository) Update(
	ctx context.Context,
	result *workflow.Result,
) error {

	f.claim =
		result

	f.updateCalled =
		true

	return nil
}

func (f *fakeRepository) AppendEvent(
	ctx context.Context,
	claimID string,
	eventType string,
	payload any,
) error {

	f.events =
		append(
			f.events,
			eventType,
		)

	return nil
}

func TestResolveDuplicateClarificationYes(
	t *testing.T,
) {

	repository :=
		&fakeRepository{
			claim: createPendingDuplicateClaim(),
		}

	service :=
		NewService(
			repository,
			policy.DemoUSDPolicy(),
		)

	result, err :=
		service.ResolveClarification(
			context.Background(),
			"claim-001",
			ClarificationAnswerRequest{
				QuestionCode: clarification.QuestionConfirmPossibleDuplicate,

				Answer: "yes",

				Explanation: "This was a separate business meal with a different client.",
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if result.FinalStatus !=
		policy.StatusLimitExceeded {

		t.Fatalf(
			"expected LIMIT_EXCEEDED, got %s",
			result.FinalStatus,
		)
	}

	if result.PolicyDecision == nil {

		t.Fatal(
			"expected policy decision",
		)
	}

	if result.PolicyDecision.ReimbursableAmount !=
		50 {

		t.Fatalf(
			"expected reimbursable amount 50, got %.2f",
			result.PolicyDecision.ReimbursableAmount,
		)
	}

	if result.PolicyDecision.ExcessAmount !=
		4.20 {

		t.Fatalf(
			"expected excess amount 4.20, got %.2f",
			result.PolicyDecision.ExcessAmount,
		)
	}

	if result.ClarificationResult == nil {

		t.Fatal(
			"expected clarification result",
		)
	}

	if result.ClarificationResult.Required {

		t.Fatal(
			"clarification should be resolved",
		)
	}

	if len(result.ClarificationAnswers) !=
		1 {

		t.Fatalf(
			"expected 1 clarification answer, got %d",
			len(result.ClarificationAnswers),
		)
	}

	if result.ClarificationAnswers[0].Answer !=
		"yes" {

		t.Fatalf(
			"expected answer yes, got %s",
			result.ClarificationAnswers[0].Answer,
		)
	}

	if !repository.updateCalled {

		t.Fatal(
			"expected claim state to be updated",
		)
	}

	if len(repository.events) !=
		2 {

		t.Fatalf(
			"expected 2 events, got %d",
			len(repository.events),
		)
	}

	if repository.events[0] !=
		"CLARIFICATION_ANSWERED" {

		t.Fatalf(
			"expected first event CLARIFICATION_ANSWERED, got %s",
			repository.events[0],
		)
	}

	if repository.events[1] !=
		"CLAIM_STATUS_CHANGED" {

		t.Fatalf(
			"expected second event CLAIM_STATUS_CHANGED, got %s",
			repository.events[1],
		)
	}
}

func TestResolveDuplicateClarificationNo(
	t *testing.T,
) {

	repository :=
		&fakeRepository{
			claim: createPendingDuplicateClaim(),
		}

	service :=
		NewService(
			repository,
			policy.DemoUSDPolicy(),
		)

	result, err :=
		service.ResolveClarification(
			context.Background(),
			"claim-001",
			ClarificationAnswerRequest{
				QuestionCode: clarification.QuestionConfirmPossibleDuplicate,

				Answer: "no",
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if result.FinalStatus !=
		workflow.StatusDuplicateDetected {

		t.Fatalf(
			"expected DUPLICATE_DETECTED, got %s",
			result.FinalStatus,
		)
	}

	if result.PolicyDecision != nil {

		t.Fatal(
			"duplicate claim should not have a policy decision",
		)
	}

	if len(result.ClarificationAnswers) !=
		1 {

		t.Fatalf(
			"expected 1 clarification answer, got %d",
			len(result.ClarificationAnswers),
		)
	}

	if result.ClarificationAnswers[0].Answer !=
		"no" {

		t.Fatalf(
			"expected answer no, got %s",
			result.ClarificationAnswers[0].Answer,
		)
	}

	if !repository.updateCalled {

		t.Fatal(
			"expected claim state to be updated",
		)
	}

	if len(repository.events) !=
		2 {

		t.Fatalf(
			"expected 2 events, got %d",
			len(repository.events),
		)
	}
}

func TestSeparateTransactionRequiresExplanation(
	t *testing.T,
) {

	repository :=
		&fakeRepository{
			claim: createPendingDuplicateClaim(),
		}

	service :=
		NewService(
			repository,
			policy.DemoUSDPolicy(),
		)

	_, err :=
		service.ResolveClarification(
			context.Background(),
			"claim-001",
			ClarificationAnswerRequest{
				QuestionCode: clarification.QuestionConfirmPossibleDuplicate,

				Answer: "yes",
			},
		)

	if err !=
		ErrExplanationRequired {

		t.Fatalf(
			"expected ErrExplanationRequired, got %v",
			err,
		)
	}

	if repository.updateCalled {

		t.Fatal(
			"claim should not be updated when explanation is missing",
		)
	}
}

func TestRejectInvalidClarificationAnswer(
	t *testing.T,
) {

	repository :=
		&fakeRepository{
			claim: createPendingDuplicateClaim(),
		}

	service :=
		NewService(
			repository,
			policy.DemoUSDPolicy(),
		)

	_, err :=
		service.ResolveClarification(
			context.Background(),
			"claim-001",
			ClarificationAnswerRequest{
				QuestionCode: clarification.QuestionConfirmPossibleDuplicate,

				Answer: "maybe",
			},
		)

	if err !=
		ErrInvalidAnswer {

		t.Fatalf(
			"expected ErrInvalidAnswer, got %v",
			err,
		)
	}
}

func TestRejectClaimWithoutPendingClarification(
	t *testing.T,
) {

	claim :=
		createPendingDuplicateClaim()

	claim.FinalStatus =
		policy.StatusApproved

	repository :=
		&fakeRepository{
			claim: claim,
		}

	service :=
		NewService(
			repository,
			policy.DemoUSDPolicy(),
		)

	_, err :=
		service.ResolveClarification(
			context.Background(),
			"claim-001",
			ClarificationAnswerRequest{
				QuestionCode: clarification.QuestionConfirmPossibleDuplicate,

				Answer: "no",
			},
		)

	if err !=
		ErrNoPendingClarification {

		t.Fatalf(
			"expected ErrNoPendingClarification, got %v",
			err,
		)
	}
}

func TestRejectUnknownQuestionCode(
	t *testing.T,
) {

	repository :=
		&fakeRepository{
			claim: createPendingDuplicateClaim(),
		}

	service :=
		NewService(
			repository,
			policy.DemoUSDPolicy(),
		)

	_, err :=
		service.ResolveClarification(
			context.Background(),
			"claim-001",
			ClarificationAnswerRequest{
				QuestionCode: "SOME_UNKNOWN_QUESTION",

				Answer: "yes",

				Explanation: "Test",
			},
		)

	if err !=
		ErrQuestionNotFound {

		t.Fatalf(
			"expected ErrQuestionNotFound, got %v",
			err,
		)
	}
}

func createPendingDuplicateClaim() *workflow.Result {

	return &workflow.Result{
		RunID: "claim-001",

		Employee: models.Employee{
			ID: "EMP10042",

			Name: "Ananya Rao",

			Grade: "B2",

			Department: "Engineering",

			Location: "Bengaluru",
		},

		FinalExtraction: &models.ReceiptExtraction{
			DocumentType: "receipt",

			Merchant: "FourthStreetMill",

			ExpenseDate: "2018-09-29",

			Subtotal: 49.50,

			TaxAmount: 4.70,

			TotalAmount: 54.20,

			Currency: "USD",

			ReceiptNumber: "4J33",

			CategorySuggestion: "Meals",

			Confidence: 0.98,
		},

		FinalValidation: validation.ReceiptValidationResult{
			Valid: true,
		},

		ClarificationResult: &clarification.Result{
			Required: true,

			Questions: []clarification.Question{
				{
					Code: clarification.QuestionConfirmPossibleDuplicate,

					Field: "duplicate",

					Question: "This expense closely matches a previously submitted receipt. Is this a separate transaction? If yes, please explain why it is different.",

					Reason: "A previous claim contains matching receipt metadata.",

					Required: true,
				},
			},
		},

		FinalStatus: workflow.StatusClarificationRequired,
	}
}
