package clarification

import (
	"fmt"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

type Question struct {
	Code     string `json:"code"`
	Field    string `json:"field"`
	Question string `json:"question"`
	Reason   string `json:"reason"`
	Required bool   `json:"required"`
}

type Result struct {
	Required  bool       `json:"required"`
	Questions []Question `json:"questions"`
}

func Evaluate(
	receipt *models.ReceiptExtraction,
	validationResult validation.ReceiptValidationResult,
	duplicateResult *duplicate.Result,
) Result {

	result := Result{
		Required:  false,
		Questions: []Question{},
	}

	//
	// Invalid extraction should be handled by
	// validation / human review first.
	//
	if !validationResult.Valid {
		return result
	}

	//
	// Exact same file is already a confirmed
	// duplicate signal. No clarification needed.
	//
	if duplicateResult != nil &&
		duplicateResult.IsDuplicate &&
		duplicateResult.MatchType == duplicate.MatchExact {

		return result
	}

	//
	// Metadata match is only a POSSIBLE duplicate.
	// Ask the employee instead of automatically rejecting.
	//
	if duplicateResult != nil &&
		duplicateResult.IsDuplicate &&
		duplicateResult.MatchType == duplicate.MatchMetadata {

		result.addQuestion(
			"CONFIRM_POSSIBLE_DUPLICATE",
			"duplicate",
			"This expense closely matches a previously submitted receipt. Is this a separate transaction? If yes, please explain why it is different.",
			fmt.Sprintf(
				"Matched previous claim %s with confidence %.2f.",
				duplicateResult.MatchedRunID,
				duplicateResult.Confidence,
			),
		)

		//
		// Ask only the most important question first.
		//
		return result
	}

	if strings.TrimSpace(receipt.Merchant) == "" {

		result.addQuestion(
			"MISSING_MERCHANT",
			"merchant",
			"What is the merchant or vendor name for this expense?",
			"The merchant could not be reliably extracted from the receipt.",
		)
	}

	if strings.TrimSpace(receipt.ExpenseDate) == "" {

		result.addQuestion(
			"MISSING_EXPENSE_DATE",
			"expense_date",
			"What date was this expense incurred?",
			"The expense date is missing from the extracted receipt.",
		)
	}

	if receipt.TotalAmount <= 0 {

		result.addQuestion(
			"MISSING_TOTAL_AMOUNT",
			"total_amount",
			"What was the total amount paid for this expense?",
			"A valid total amount could not be extracted.",
		)
	}

	if strings.TrimSpace(receipt.Currency) == "" {

		result.addQuestion(
			"MISSING_CURRENCY",
			"currency",
			"What currency was used for this expense?",
			"The receipt currency could not be determined.",
		)
	}

	if strings.EqualFold(
		strings.TrimSpace(receipt.CategorySuggestion),
		"Other",
	) {

		result.addQuestion(
			"UNCLEAR_CATEGORY",
			"category",
			"What type of business expense was this?",
			"The receipt could not be confidently assigned to a reimbursement category.",
		)
	}

	if receipt.Confidence > 0 &&
		receipt.Confidence < 0.85 {

		result.addQuestion(
			"LOW_EXTRACTION_CONFIDENCE",
			"receipt_details",
			"Some receipt details were unclear. Please confirm that the merchant, date and amount are correct.",
			fmt.Sprintf(
				"Receipt extraction confidence was %.2f.",
				receipt.Confidence,
			),
		)
	}

	return result
}

func (r *Result) addQuestion(
	code string,
	field string,
	question string,
	reason string,
) {

	r.Required = true

	r.Questions = append(
		r.Questions,
		Question{
			Code:     code,
			Field:    field,
			Question: question,
			Reason:   reason,
			Required: true,
		},
	)
}
