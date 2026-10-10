package validation

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

const monetaryTolerance = 0.01

const (
	SeverityInfo    = "INFO"
	SeverityWarning = "WARNING"
	SeverityError   = "ERROR"
)

type ValidationIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ReceiptValidationResult struct {
	Valid bool `json:"valid"`

	LineItemSum float64 `json:"line_item_sum"`

	Issues []ValidationIssue `json:"issues"`
}

func NormalizeReceipt(
	receipt *models.ReceiptExtraction,
) {
	if receipt == nil {
		return
	}

	receipt.DocumentType =
		strings.TrimSpace(
			receipt.DocumentType,
		)

	receipt.Merchant =
		strings.TrimSpace(
			receipt.Merchant,
		)

	receipt.ExpenseDate =
		strings.TrimSpace(
			receipt.ExpenseDate,
		)

	receipt.Currency =
		NormalizeCurrency(
			receipt.Currency,
		)

	receipt.ReceiptNumber =
		strings.TrimSpace(
			receipt.ReceiptNumber,
		)

	receipt.PaymentMethod =
		strings.TrimSpace(
			receipt.PaymentMethod,
		)

	receipt.CategorySuggestion =
		strings.TrimSpace(
			receipt.CategorySuggestion,
		)

	for i := range receipt.LineItems {

		receipt.LineItems[i].Name =
			strings.TrimSpace(
				receipt.LineItems[i].Name,
			)
	}
}

func NormalizeCurrency(
	currency string,
) string {

	value :=
		strings.ToUpper(
			strings.TrimSpace(
				currency,
			),
		)

	switch value {

	case "$",
		"US$",
		"USD$",
		"USD":
		return "USD"

	case "₹",
		"RS",
		"RS.",
		"INR":
		return "INR"

	case "€",
		"EUR":
		return "EUR"

	case "£",
		"GBP":
		return "GBP"

	case "¥",
		"JPY":
		return "JPY"

	case "AED":
		return "AED"

	case "CAD":
		return "CAD"

	case "AUD":
		return "AUD"

	case "SGD":
		return "SGD"

	default:
		return value
	}
}

func ValidateReceipt(
	receipt *models.ReceiptExtraction,
) ReceiptValidationResult {

	result :=
		ReceiptValidationResult{
			Valid: true,

			Issues: []ValidationIssue{},
		}

	if receipt == nil {
		addError(
			&result,
			"MISSING_RECEIPT",
			"Receipt extraction is missing.",
		)

		return result
	}

	NormalizeReceipt(
		receipt,
	)

	validateRequiredFields(
		receipt,
		&result,
	)

	validateMonetaryFields(
		receipt,
		&result,
	)

	validateLineItems(
		receipt,
		&result,
	)

	validateConfidence(
		receipt,
		&result,
	)

	result.Valid =
		!containsError(
			result.Issues,
		)

	return result
}

func validateRequiredFields(
	receipt *models.ReceiptExtraction,
	result *ReceiptValidationResult,
) {

	if receipt.Merchant == "" {
		addError(
			result,
			"MISSING_MERCHANT",
			"Merchant is missing.",
		)
	}

	if receipt.ExpenseDate == "" {
		addError(
			result,
			"MISSING_EXPENSE_DATE",
			"Expense date is missing.",
		)
	} else {

		if _, err :=
			time.Parse(
				"2006-01-02",
				receipt.ExpenseDate,
			); err != nil {

			addError(
				result,
				"INVALID_EXPENSE_DATE",
				fmt.Sprintf(
					"Expense date %q must use YYYY-MM-DD format.",
					receipt.ExpenseDate,
				),
			)
		}
	}

	if receipt.Currency == "" {
		addError(
			result,
			"MISSING_CURRENCY",
			"Currency is missing.",
		)
	}

	if receipt.TotalAmount <= 0 {
		addError(
			result,
			"INVALID_TOTAL",
			"Receipt total must be greater than zero.",
		)
	}
}

func validateMonetaryFields(
	receipt *models.ReceiptExtraction,
	result *ReceiptValidationResult,
) {

	if receipt.Subtotal < 0 {
		addError(
			result,
			"INVALID_SUBTOTAL",
			"Subtotal cannot be negative.",
		)
	}

	if receipt.TaxAmount < 0 {
		addError(
			result,
			"INVALID_TAX",
			"Tax amount cannot be negative.",
		)
	}

	if receipt.ServiceCharge < 0 {
		addError(
			result,
			"INVALID_SERVICE_CHARGE",
			"Service charge cannot be negative.",
		)
	}

	if receipt.TipAmount < 0 {
		addError(
			result,
			"INVALID_TIP",
			"Tip amount cannot be negative.",
		)
	}

	if receipt.DiscountAmount < 0 {
		addError(
			result,
			"INVALID_DISCOUNT",
			"Discount amount must be represented as a positive value.",
		)
	}

	if receipt.OtherFees < 0 {
		addError(
			result,
			"INVALID_OTHER_FEES",
			"Other fees cannot be negative.",
		)
	}

	//
	// If subtotal is unavailable, we cannot
	// reliably verify the final arithmetic.
	//
	if receipt.Subtotal <= 0 {
		return
	}

	expectedTotal :=
		receipt.Subtotal +
			receipt.TaxAmount +
			receipt.ServiceCharge +
			receipt.TipAmount +
			receipt.OtherFees -
			receipt.DiscountAmount

	if !moneyEqual(
		expectedTotal,
		receipt.TotalAmount,
	) {

		addError(
			result,
			"TOTAL_MISMATCH",
			fmt.Sprintf(
				"Subtotal %.2f + tax %.2f + service charge %.2f + tip %.2f + other fees %.2f - discount %.2f = %.2f, but receipt total is %.2f.",
				receipt.Subtotal,
				receipt.TaxAmount,
				receipt.ServiceCharge,
				receipt.TipAmount,
				receipt.OtherFees,
				receipt.DiscountAmount,
				expectedTotal,
				receipt.TotalAmount,
			),
		)
	}
}

func validateLineItems(
	receipt *models.ReceiptExtraction,
	result *ReceiptValidationResult,
) {

	if len(
		receipt.LineItems,
	) == 0 {

		return
	}

	var lineItemSum float64

	for _, item := range receipt.LineItems {

		lineItemSum +=
			item.Amount
	}

	result.LineItemSum =
		roundMoney(
			lineItemSum,
		)

	if receipt.Subtotal <= 0 {
		return
	}

	if !moneyEqual(
		result.LineItemSum,
		receipt.Subtotal,
	) {

		addError(
			result,
			"LINE_ITEM_SUM_MISMATCH",
			fmt.Sprintf(
				"Line items total %.2f, but receipt subtotal is %.2f.",
				result.LineItemSum,
				receipt.Subtotal,
			),
		)
	}
}

func validateConfidence(
	receipt *models.ReceiptExtraction,
	result *ReceiptValidationResult,
) {

	if receipt.Confidence > 0 &&
		receipt.Confidence < 0.85 {

		result.Issues =
			append(
				result.Issues,
				ValidationIssue{
					Code: "LOW_CONFIDENCE",

					Severity: SeverityWarning,

					Message: fmt.Sprintf(
						"AI extraction confidence is %.2f.",
						receipt.Confidence,
					),
				},
			)
	}
}

func addError(
	result *ReceiptValidationResult,
	code string,
	message string,
) {

	result.Issues =
		append(
			result.Issues,
			ValidationIssue{
				Code: code,

				Severity: SeverityError,

				Message: message,
			},
		)

	result.Valid =
		false
}

func containsError(
	issues []ValidationIssue,
) bool {

	for _, issue := range issues {

		if issue.Severity ==
			SeverityError {

			return true
		}
	}

	return false
}

func moneyEqual(
	left float64,
	right float64,
) bool {

	return math.Abs(
		left-right,
	) <= monetaryTolerance
}

func roundMoney(
	value float64,
) float64 {

	return math.Round(
		value*100,
	) / 100
}
