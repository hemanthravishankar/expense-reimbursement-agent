package claims

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/clarification"
	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

const statusManualReview = "MANUAL_REVIEW"

var (
	ErrNoPendingClarification = errors.New(
		"claim has no pending clarification",
	)

	ErrQuestionNotFound = errors.New(
		"clarification question not found",
	)

	ErrInvalidAnswer = errors.New(
		"clarification answer must be yes or no",
	)

	ErrExplanationRequired = errors.New(
		"explanation is required when confirming a separate transaction",
	)

	ErrUnsupportedClarification = errors.New(
		"unsupported clarification type",
	)
)

type Repository interface {
	Get(
		ctx context.Context,
		claimID string,
	) (*workflow.Result, error)

	Update(
		ctx context.Context,
		result *workflow.Result,
	) error

	AppendEvent(
		ctx context.Context,
		claimID string,
		eventType string,
		payload any,
	) error
}

type ClarificationAnswerRequest struct {
	QuestionCode string `json:"question_code"`
	Answer       string `json:"answer"`
	Explanation  string `json:"explanation,omitempty"`
}

type Service struct {
	repository Repository
	policy     policy.Policy
}

func NewService(
	repository Repository,
	companyPolicy policy.Policy,
) *Service {

	return &Service{
		repository: repository,

		policy: companyPolicy,
	}
}

func (s *Service) ResolveClarification(
	ctx context.Context,
	claimID string,
	request ClarificationAnswerRequest,
) (*workflow.Result, error) {

	//
	// STEP 1
	// Load previously processed claim.
	//
	result, err :=
		s.repository.Get(
			ctx,
			claimID,
		)

	if err != nil {
		return nil,
			err
	}

	//
	// STEP 2
	// Verify the claim is actually waiting
	// for employee clarification.
	//
	if result.FinalStatus !=
		workflow.StatusClarificationRequired {

		return nil,
			ErrNoPendingClarification
	}

	if result.ClarificationResult == nil ||
		!result.ClarificationResult.Required {

		return nil,
			ErrNoPendingClarification
	}

	//
	// STEP 3
	// Validate question code.
	//
	questionCode :=
		strings.TrimSpace(
			request.QuestionCode,
		)

	if questionCode == "" {

		return nil,
			ErrQuestionNotFound
	}

	questionFound :=
		false

	for _, question := range result.ClarificationResult.Questions {

		if question.Code ==
			questionCode {

			questionFound =
				true

			break
		}
	}

	if !questionFound {

		return nil,
			ErrQuestionNotFound
	}

	//
	// STEP 4
	// Route based on clarification type.
	//
	switch questionCode {

	case clarification.QuestionConfirmPossibleDuplicate:

		return s.resolvePossibleDuplicate(
			ctx,
			result,
			request,
		)

	default:

		return nil,
			ErrUnsupportedClarification
	}
}

func duplicateRequiresManualReview(
	matchType string,
	isDuplicate bool,
	strongIdentity bool,
) bool {

	return isDuplicate &&
		matchType == duplicate.MatchMetadata &&
		strongIdentity
}

func (s *Service) resolvePossibleDuplicate(
	ctx context.Context,
	result *workflow.Result,
	request ClarificationAnswerRequest,
) (*workflow.Result, error) {

	answer :=
		strings.ToLower(
			strings.TrimSpace(
				request.Answer,
			),
		)

	if answer != "yes" &&
		answer != "no" {

		return nil,
			ErrInvalidAnswer
	}

	explanation :=
		strings.TrimSpace(
			request.Explanation,
		)

	//
	// When the employee claims the receipt is
	// a separate legitimate transaction,
	// require an explanation.
	//
	if answer == "yes" &&
		explanation == "" {

		return nil,
			ErrExplanationRequired
	}

	clarificationAnswer :=
		clarification.Answer{
			QuestionCode: clarification.QuestionConfirmPossibleDuplicate,

			Answer: answer,

			Explanation: explanation,

			AnsweredAt: time.Now().UTC(),
		}

	//
	// IMPORTANT:
	//
	// We reuse the stored extraction.
	//
	// There is:
	// - no OCR
	// - no PDF rendering
	// - no Qwen invocation
	//
	if answer == "yes" {

		if result.FinalExtraction == nil {

			return nil,
				fmt.Errorf(
					"claim has no validated receipt extraction",
				)
		}

		if !result.FinalValidation.Valid {

			return nil,
				fmt.Errorf(
					"claim extraction is not valid",
				)
		}

		//
		// A self-declaration must not be sufficient to override
		// strong transaction identity evidence.
		//
		// Strong same-employee metadata identity currently means:
		// - same non-empty receipt number
		// - same expense date
		// - same total amount
		//
		// In that case the employee's YES response is recorded,
		// but reimbursement remains pending until Finance/Admin
		// reviews the claim.
		//
		strongDuplicateIdentity := false

		if result.DuplicateResult != nil {

			strongDuplicateIdentity =
				duplicateRequiresManualReview(
					result.DuplicateResult.MatchType,
					result.DuplicateResult.IsDuplicate,
					result.DuplicateResult.StrongIdentity,
				)
		}

		if strongDuplicateIdentity {

			result.PolicyDecision =
				nil

			result.DuplicateResult.RequiresAdminReview =
				true

			result.DuplicateResult.Reason =
				"Strong same-employee duplicate identity evidence was found. The employee states this is a separate transaction, so Finance must review it before reimbursement."

			result.FinalStatus =
				statusManualReview

		} else {

			//
			// For ordinary similarity, an employee YES response
			// with explanation can resume deterministic policy
			// evaluation using the already validated extraction.
			//
			decision :=
				policy.Evaluate(
					result.FinalExtraction,
					result.Employee,
					s.policy,
				)

			result.PolicyDecision =
				&decision

			result.FinalStatus =
				decision.Status
		}
	}

	if answer == "no" {

		//
		// Employee confirms this was not a
		// separate transaction.
		//
		result.PolicyDecision =
			nil

		result.FinalStatus =
			workflow.StatusDuplicateDetected
	}

	result.ClarificationAnswers =
		append(
			result.ClarificationAnswers,
			clarificationAnswer,
		)

	//
	// Mark clarification as resolved.
	//
	result.ClarificationResult.Required =
		false

	//
	// STEP 5
	// Persist updated claim state.
	//
	if err :=
		s.repository.Update(
			ctx,
			result,
		); err != nil {

		return nil,
			fmt.Errorf(
				"update claim after clarification: %w",
				err,
			)
	}

	//
	// STEP 6
	// Append business lifecycle events.
	//
	if err :=
		s.repository.AppendEvent(
			ctx,
			result.RunID,
			"CLARIFICATION_ANSWERED",
			clarificationAnswer,
		); err != nil {

		return nil,
			fmt.Errorf(
				"append clarification event: %w",
				err,
			)
	}

	if err :=
		s.repository.AppendEvent(
			ctx,
			result.RunID,
			"CLAIM_STATUS_CHANGED",
			map[string]any{
				"status": result.FinalStatus,

				"previous_status": workflow.StatusClarificationRequired,

				"reason": "clarification resolved",
			},
		); err != nil {

		return nil,
			fmt.Errorf(
				"append claim status event: %w",
				err,
			)
	}

	return result,
		nil
}
