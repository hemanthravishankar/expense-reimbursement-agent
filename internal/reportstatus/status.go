package reportstatus

import "github.com/hemanth-shankar/expense-agent/internal/models"

// Calculate determines the parent expense-report status from
// the current statuses of all child receipt claims.
func Calculate(
	items []models.ExpenseReportItem,
) string {

	if len(items) == 0 {
		return "PROCESSING"
	}

	var (
		hasActionRequired bool
		hasReviewRequired bool
		hasProcessing     bool
		hasRejected       bool
		hasAccepted       bool
	)

	for _, item := range items {

		switch item.Status {

		// Employee must answer something.
		case "CLARIFICATION_REQUIRED":
			hasActionRequired = true

		// Human / Finance review is still required.
		case "HUMAN_REVIEW_REQUIRED",
			"MANUAL_REVIEW",
			"CURRENCY_MISMATCH":

			hasReviewRequired = true

		case "PROCESSING":
			hasProcessing = true

		// Final non-reimbursable outcomes.
		case "NOT_ALLOWED",
			"REJECTED",
			"DUPLICATE_DETECTED":

			hasRejected = true

		// Successfully processed outcomes.
		case "APPROVED",
			"LIMIT_EXCEEDED":

			hasAccepted = true

		default:

			// Fail safely rather than incorrectly
			// showing an unknown state as completed.
			hasReviewRequired = true
		}
	}

	// Priority order matters.

	if hasActionRequired {
		return "ACTION_REQUIRED"
	}

	if hasReviewRequired {
		return "REVIEW_REQUIRED"
	}

	if hasProcessing {
		return "PROCESSING"
	}

	if hasRejected {

		if hasAccepted {
			return "PARTIALLY_REJECTED"
		}

		return "REJECTED"
	}

	return "COMPLETED"
}
