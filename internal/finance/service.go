package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

const (
	ActionApprove = "APPROVE"
	ActionReject  = "REJECT"

	StatusRejected = "REJECTED"
)

var (
	ErrInvalidReviewAction = errors.New("finance review action must be APPROVE or REJECT")
	ErrReviewerRequired    = errors.New("finance reviewer is required")
	ErrReviewNoteRequired  = errors.New("finance review note is required")
	ErrClaimNotReviewable  = errors.New("claim is not waiting for Finance review")
	ErrMissingExtraction   = errors.New("claim has no valid extraction for policy evaluation")
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

type ReviewRequest struct {
	Action   string `json:"action"`
	Reviewer string `json:"reviewer"`
	Note     string `json:"note"`
}

type Service struct {
	repository     Repository
	policy         policy.Policy
	policyProvider policy.Provider
}

func NewService(
	repository Repository,
	companyPolicy policy.Policy,
	providers ...policy.Provider,
) *Service {
	service := &Service{
		repository: repository,
		policy:     companyPolicy,
	}

	if len(providers) > 0 {
		service.policyProvider = providers[0]
	}

	return service
}

func (s *Service) ReviewClaim(
	ctx context.Context,
	claimID string,
	request ReviewRequest,
) (*workflow.Result, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("finance review repository is not configured")
	}

	claimID = strings.TrimSpace(claimID)
	request.Action = strings.ToUpper(strings.TrimSpace(request.Action))
	request.Reviewer = strings.TrimSpace(request.Reviewer)
	request.Note = strings.TrimSpace(request.Note)

	if request.Action != ActionApprove && request.Action != ActionReject {
		return nil, ErrInvalidReviewAction
	}

	if request.Reviewer == "" {
		return nil, ErrReviewerRequired
	}

	if request.Note == "" {
		return nil, ErrReviewNoteRequired
	}

	result, err := s.repository.Get(ctx, claimID)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, fmt.Errorf("claim result is nil")
	}

	if result.FinalStatus != workflow.StatusManualReview &&
		result.FinalStatus != workflow.StatusHumanReviewRequired {
		return nil, ErrClaimNotReviewable
	}

	previousStatus := result.FinalStatus

	switch request.Action {
	case ActionApprove:
		if result.FinalExtraction == nil || !result.FinalValidation.Valid {
			return nil, ErrMissingExtraction
		}

		companyPolicy := s.policy
		if s.policyProvider != nil {
			activePolicy, policyErr := s.policyProvider.ActivePolicy(ctx)
			if policyErr != nil {
				return nil, fmt.Errorf(
					"load active reimbursement policy: %w",
					policyErr,
				)
			}
			companyPolicy = activePolicy
		}

		decision := policy.Evaluate(
			result.FinalExtraction,
			result.Employee,
			companyPolicy,
		)

		result.PolicyDecision = &decision
		result.FinalStatus = decision.Status

	case ActionReject:
		result.PolicyDecision = nil
		result.FinalStatus = StatusRejected
	}

	if err := s.repository.Update(ctx, result); err != nil {
		return nil, err
	}

	reviewPayload := map[string]any{
		"action":          request.Action,
		"reviewer":        request.Reviewer,
		"note":            request.Note,
		"previous_status": previousStatus,
		"new_status":      result.FinalStatus,
	}

	if result.DuplicateResult != nil {
		reviewPayload["duplicate_match_type"] = result.DuplicateResult.MatchType
		reviewPayload["matched_run_id"] = result.DuplicateResult.MatchedRunID
		reviewPayload["matched_employee_id"] = result.DuplicateResult.MatchedEmployeeID
		reviewPayload["duplicate_confidence"] = result.DuplicateResult.Confidence
		reviewPayload["strong_identity"] = result.DuplicateResult.StrongIdentity
	}

	if err := s.repository.AppendEvent(
		ctx,
		claimID,
		"FINANCE_REVIEWED",
		reviewPayload,
	); err != nil {
		return nil, err
	}

	if err := s.repository.AppendEvent(
		ctx,
		claimID,
		"CLAIM_STATUS_CHANGED",
		map[string]any{
			"from":     previousStatus,
			"to":       result.FinalStatus,
			"source":   "FINANCE_REVIEW",
			"reviewer": request.Reviewer,
		},
	); err != nil {
		return nil, err
	}

	return result, nil
}
