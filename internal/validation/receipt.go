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

	reconcileTransportationCharges(
		receipt,
	)
}

// reconcileTransportationCharges applies a conservative deterministic
// correction for transportation receipts where the vision model extracts
// every fare/fee row as a line item but incorrectly uses only "Base Fare"
// as the subtotal.
//
// The reconciliation is intentionally narrow:
//
//   - it only runs for Transportation expenses
//   - it does nothing when the current extraction already validates
//   - it only uses amounts already present in visible extracted line items
//   - it only commits the correction when the reconstructed arithmetic
//     exactly matches the receipt total within monetary tolerance
//
// Example:
//
//	Base Fare                 14.50
//	Time                       3.20
//	Distance                   4.80
//	Airport Fee                2.00
//	Taxes & Other Fees         1.50
//	Total                     26.00
//
// becomes:
//
//	subtotal    = 22.50
//	other_fees  = 3.50
//	total       = 26.00
//
// and only Base Fare, Time, and Distance remain subtotal line items.
func reconcileTransportationCharges(
	receipt *models.ReceiptExtraction,
) {
	if receipt == nil {
		return
	}

	if !strings.EqualFold(
		strings.TrimSpace(
			receipt.CategorySuggestion,
		),
		"Transportation",
	) {
		return
	}

	if len(receipt.LineItems) == 0 ||
		receipt.TotalAmount <= 0 {

		return
	}

	if currentBreakdownIsConsistent(
		receipt,
	) {
		return
	}

	var (
		subtotal      float64
		taxAmount     float64
		serviceCharge float64
		tipAmount     float64
		discount      float64
		otherFees     float64

		classifiedCharge bool
	)

	baseItems :=
		make(
			[]models.ReceiptLineItem,
			0,
			len(receipt.LineItems),
		)

	for _, item := range receipt.LineItems {

		name :=
			normalizeLineItemName(
				item.Name,
			)

		amount :=
			roundMoney(
				item.Amount,
			)

		switch {

		case isCombinedTaxAndFeeLine(
			name,
		):

			otherFees +=
				amount

			classifiedCharge =
				true

		case isServiceChargeLine(
			name,
		):

			serviceCharge +=
				amount

			classifiedCharge =
				true

		case isTipLine(
			name,
		):

			tipAmount +=
				amount

			classifiedCharge =
				true

		case isDiscountLine(
			name,
		):

			discount +=
				math.Abs(
					amount,
				)

			classifiedCharge =
				true

		case isTaxLine(
			name,
		):

			taxAmount +=
				amount

			classifiedCharge =
				true

		case isTransportationFeeLine(
			name,
		):

			otherFees +=
				amount

			classifiedCharge =
				true

		default:

			subtotal +=
				amount

			baseItems =
				append(
					baseItems,
					item,
				)
		}
	}

	if !classifiedCharge ||
		subtotal <= 0 {

		return
	}

	subtotal =
		roundMoney(
			subtotal,
		)

	taxAmount =
		roundMoney(
			taxAmount,
		)

	serviceCharge =
		roundMoney(
			serviceCharge,
		)

	tipAmount =
		roundMoney(
			tipAmount,
		)

	discount =
		roundMoney(
			discount,
		)

	otherFees =
		roundMoney(
			otherFees,
		)

	expectedTotal :=
		roundMoney(
			subtotal +
				taxAmount +
				serviceCharge +
				tipAmount +
				otherFees -
				discount,
		)

	if !moneyEqual(
		expectedTotal,
		receipt.TotalAmount,
	) {
		return
	}

	receipt.Subtotal =
		subtotal

	receipt.TaxAmount =
		taxAmount

	receipt.ServiceCharge =
		serviceCharge

	receipt.TipAmount =
		tipAmount

	receipt.DiscountAmount =
		discount

	receipt.OtherFees =
		otherFees

	receipt.LineItems =
		baseItems

	addReceiptWarningOnce(
		receipt,
		"Deterministic transportation charge reconciliation applied from visible line items.",
	)
}

func currentBreakdownIsConsistent(
	receipt *models.ReceiptExtraction,
) bool {
	if receipt == nil ||
		receipt.Subtotal <= 0 ||
		receipt.TotalAmount <= 0 {

		return false
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
		return false
	}

	if len(receipt.LineItems) == 0 {
		return true
	}

	var lineItemSum float64

	for _, item := range receipt.LineItems {

		lineItemSum +=
			item.Amount
	}

	return moneyEqual(
		lineItemSum,
		receipt.Subtotal,
	)
}

func normalizeLineItemName(
	name string,
) string {
	return strings.Join(
		strings.Fields(
			strings.ToLower(
				strings.TrimSpace(
					name,
				),
			),
		),
		" ",
	)
}

func isCombinedTaxAndFeeLine(
	name string,
) bool {
	hasTax :=
		strings.Contains(
			name,
			"tax",
		) ||
			strings.Contains(
				name,
				"gst",
			) ||
			strings.Contains(
				name,
				"vat",
			)

	hasFee :=
		strings.Contains(
			name,
			"fee",
		) ||
			strings.Contains(
				name,
				"surcharge",
			)

	return hasTax &&
		hasFee
}

func isServiceChargeLine(
	name string,
) bool {
	return strings.Contains(
		name,
		"service charge",
	)
}

func isTipLine(
	name string,
) bool {
	return strings.Contains(
		name,
		"tip",
	) ||
		strings.Contains(
			name,
			"gratuity",
		)
}

func isDiscountLine(
	name string,
) bool {
	return strings.Contains(
		name,
		"discount",
	) ||
		strings.Contains(
			name,
			"coupon",
		) ||
		strings.Contains(
			name,
			"promotion",
		) ||
		strings.Contains(
			name,
			"promo",
		)
}

func isTaxLine(
	name string,
) bool {
	return strings.Contains(
		name,
		"tax",
	) ||
		strings.Contains(
			name,
			"gst",
		) ||
		strings.Contains(
			name,
			"vat",
		)
}

func isTransportationFeeLine(
	name string,
) bool {
	return strings.Contains(
		name,
		"airport fee",
	) ||
		strings.Contains(
			name,
			"booking fee",
		) ||
		strings.Contains(
			name,
			"convenience fee",
		) ||
		strings.Contains(
			name,
			"platform fee",
		) ||
		strings.Contains(
			name,
			"processing fee",
		) ||
		strings.Contains(
			name,
			"service fee",
		) ||
		strings.Contains(
			name,
			"delivery fee",
		) ||
		strings.Contains(
			name,
			"parking fee",
		) ||
		strings.Contains(
			name,
			"toll",
		) ||
		strings.Contains(
			name,
			"surcharge",
		)
}

func addReceiptWarningOnce(
	receipt *models.ReceiptExtraction,
	message string,
) {
	for _, warning := range receipt.Warnings {

		if warning ==
			message {

			return
		}
	}

	receipt.Warnings =
		append(
			receipt.Warnings,
			message,
		)
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
