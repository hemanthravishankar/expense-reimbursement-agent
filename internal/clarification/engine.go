package clarification

import (
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

const (
	QuestionConfirmPossibleDuplicate = "CONFIRM_POSSIBLE_DUPLICATE"

	QuestionMissingMerchant = "MISSING_MERCHANT"
	QuestionMissingDate     = "MISSING_DATE"
	QuestionMissingAmount   = "MISSING_AMOUNT"
	QuestionMissingCurrency = "MISSING_CURRENCY"

	QuestionConfirmCategory = "CONFIRM_CATEGORY"
	QuestionConfirmDetails  = "CONFIRM_DETAILS"
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

type Answer struct {
	QuestionCode string    `json:"question_code"`
	Answer       string    `json:"answer"`
	Explanation  string    `json:"explanation,omitempty"`
	AnsweredAt   time.Time `json:"answered_at"`
}

func Evaluate(
	receipt *models.ReceiptExtraction,
	validationResult validation.ReceiptValidationResult,
	duplicateResult *duplicate.Result,
) Result {

	result :=
		Result{
			Required: false,

			Questions: []Question{},
		}

	if receipt == nil {
		return result
	}

	//
	// Invalid AI extraction should not be
	// clarified by the employee.
	//
	// It should instead be handled through
	// fallback extraction or human review.
	//
	if !validationResult.Valid {
		return result
	}

	//
	// Exact duplicate:
	//
	// no clarification is required.
	// The system can deterministically block it.
	//
	if duplicateResult != nil &&
		duplicateResult.IsDuplicate &&
		duplicateResult.MatchType ==
			duplicate.MatchExact {

		return result
	}

	//
	// Metadata duplicate:
	//
	// The document bytes may be different
	// because of resizing, recompression,
	// screenshots, JPG -> PDF conversion, etc.
	//
	// Ask the employee whether this is
	// genuinely a separate expense.
	//
	if duplicateResult != nil &&
		duplicateResult.IsDuplicate &&
		duplicateResult.MatchType ==
			duplicate.MatchMetadata {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionConfirmPossibleDuplicate,

					Field: "duplicate",

					Question: "This expense closely matches a receipt you previously submitted. " +
						"Is this a separate expense from the previously submitted receipt? " +
						"Answer YES only if this is a genuinely separate transaction and explain why it is different. " +
						"Answer NO if this is the same expense, a re-upload, resized image, edited image, or another copy of the same receipt.",

					Reason: "A previous claim has matching receipt metadata such as merchant, date, amount, or receipt number.",

					Required: true,
				},
			)

		//
		// Duplicate clarification takes priority.
		// We do not want to overwhelm the employee
		// with several questions at once.
		//
		return result
	}

	merchant :=
		strings.TrimSpace(
			receipt.Merchant,
		)

	if merchant == "" {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionMissingMerchant,

					Field: "merchant",

					Question: "Please provide the merchant or vendor name for this expense.",

					Reason: "The merchant name could not be determined reliably from the uploaded document.",

					Required: true,
				},
			)
	}

	if strings.TrimSpace(
		receipt.ExpenseDate,
	) == "" {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionMissingDate,

					Field: "expense_date",

					Question: "Please provide the date of this expense.",

					Reason: "The expense date could not be determined reliably from the uploaded document.",

					Required: true,
				},
			)
	}

	if receipt.TotalAmount <= 0 {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionMissingAmount,

					Field: "total_amount",

					Question: "Please confirm the total amount paid for this expense.",

					Reason: "A valid total amount could not be determined from the uploaded document.",

					Required: true,
				},
			)
	}

	if strings.TrimSpace(
		receipt.Currency,
	) == "" {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionMissingCurrency,

					Field: "currency",

					Question: "Please confirm the currency used for this expense.",

					Reason: "The currency could not be determined reliably from the uploaded document.",

					Required: true,
				},
			)
	}

	if strings.EqualFold(
		strings.TrimSpace(
			receipt.CategorySuggestion,
		),
		"Other",
	) {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionConfirmCategory,

					Field: "category",

					Question: "Please confirm the expense category for this transaction.",

					Reason: "The system could not confidently map the expense to a supported reimbursement category.",

					Required: true,
				},
			)
	}

	//
	// Low-confidence extraction can still be
	// mathematically valid, but we want the
	// employee to verify it.
	//
	if receipt.Confidence > 0 &&
		receipt.Confidence < 0.85 {

		result.Required =
			true

		result.Questions =
			append(
				result.Questions,
				Question{
					Code: QuestionConfirmDetails,

					Field: "receipt_details",

					Question: "Please confirm that the extracted merchant, date, amount, and category are correct.",

					Reason: "The AI extraction confidence is below the automatic-processing threshold.",

					Required: true,
				},
			)
	}

	return result
}
