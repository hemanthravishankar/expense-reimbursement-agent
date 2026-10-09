package validation

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

type Severity string

const (
	SeverityInfo    Severity = "INFO"
	SeverityWarning Severity = "WARNING"
	SeverityError   Severity = "ERROR"
)

type ValidationIssue struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type ReceiptValidationResult struct {
	Valid       bool              `json:"valid"`
	LineItemSum float64           `json:"line_item_sum"`
	Issues      []ValidationIssue `json:"issues"`
}

func NormalizeReceipt(receipt *models.ReceiptExtraction) {
	receipt.Currency = NormalizeCurrency(receipt.Currency)
}

func NormalizeCurrency(currency string) string {
	switch strings.TrimSpace(strings.ToUpper(currency)) {
	case "$", "US$", "USD":
		return "USD"
	case "₹", "RS", "RS.", "INR":
		return "INR"
	case "€", "EUR":
		return "EUR"
	case "£", "GBP":
		return "GBP"
	default:
		return strings.ToUpper(strings.TrimSpace(currency))
	}
}

func ValidateReceipt(
	receipt *models.ReceiptExtraction,
) ReceiptValidationResult {

	result := ReceiptValidationResult{
		Valid:  true,
		Issues: []ValidationIssue{},
	}

	if strings.TrimSpace(receipt.Merchant) == "" {
		result.addIssue(
			"MISSING_MERCHANT",
			SeverityWarning,
			"Merchant name is missing.",
		)
	}

	if receipt.TotalAmount <= 0 {
		result.addIssue(
			"INVALID_TOTAL",
			SeverityError,
			"Receipt total must be greater than zero.",
		)
	}

	if receipt.ExpenseDate == "" {
		result.addIssue(
			"MISSING_DATE",
			SeverityWarning,
			"Expense date is missing.",
		)
	} else {
		_, err := time.Parse(
			"2006-01-02",
			receipt.ExpenseDate,
		)

		if err != nil {
			result.addIssue(
				"INVALID_DATE_FORMAT",
				SeverityError,
				"Expense date must use YYYY-MM-DD format.",
			)
		}
	}

	if receipt.Currency == "" {
		result.addIssue(
			"MISSING_CURRENCY",
			SeverityWarning,
			"Currency could not be determined.",
		)
	}

	if receipt.Subtotal > 0 {
		expectedTotal :=
			receipt.Subtotal + receipt.TaxAmount

		if !moneyEqual(
			expectedTotal,
			receipt.TotalAmount,
		) {
			result.addIssue(
				"TOTAL_MISMATCH",
				SeverityError,
				fmt.Sprintf(
					"Subtotal %.2f + tax %.2f = %.2f, but receipt total is %.2f.",
					receipt.Subtotal,
					receipt.TaxAmount,
					expectedTotal,
					receipt.TotalAmount,
				),
			)
		}
	}

	var lineItemSum float64

	for _, item := range receipt.LineItems {
		if item.Amount > 0 {
			lineItemSum += item.Amount
		}
	}

	result.LineItemSum = roundMoney(lineItemSum)

	if receipt.Subtotal > 0 &&
		!moneyEqual(lineItemSum, receipt.Subtotal) {

		result.addIssue(
			"LINE_ITEM_SUM_MISMATCH",
			SeverityError,
			fmt.Sprintf(
				"Line items total %.2f, but receipt subtotal is %.2f.",
				lineItemSum,
				receipt.Subtotal,
			),
		)
	}

	if receipt.Confidence < 0.70 {
		result.addIssue(
			"LOW_EXTRACTION_CONFIDENCE",
			SeverityWarning,
			fmt.Sprintf(
				"AI extraction confidence is %.2f.",
				receipt.Confidence,
			),
		)
	}

	return result
}

func (r *ReceiptValidationResult) addIssue(
	code string,
	severity Severity,
	message string,
) {

	r.Issues = append(
		r.Issues,
		ValidationIssue{
			Code:     code,
			Severity: severity,
			Message:  message,
		},
	)

	if severity == SeverityError {
		r.Valid = false
	}
}

func moneyEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}
