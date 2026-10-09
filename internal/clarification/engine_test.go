package clarification

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

func TestMetadataDuplicateRequiresClarification(
	t *testing.T,
) {

	receipt := &models.ReceiptExtraction{
		Merchant:    "FourthStreetMill",
		ExpenseDate: "2018-09-29",
		TotalAmount: 54.20,
		Currency:    "USD",
		Confidence:  0.98,
	}

	validationResult :=
		validation.ReceiptValidationResult{
			Valid: true,
		}

	duplicateResult :=
		&duplicate.Result{
			IsDuplicate:  true,
			MatchType:    duplicate.MatchMetadata,
			Confidence:   1,
			MatchedRunID: "run-001",
		}

	result :=
		Evaluate(
			receipt,
			validationResult,
			duplicateResult,
		)

	if !result.Required {
		t.Fatal(
			"expected clarification to be required",
		)
	}

	if len(result.Questions) != 1 {
		t.Fatalf(
			"expected 1 question, got %d",
			len(result.Questions),
		)
	}

	if result.Questions[0].Code !=
		"CONFIRM_POSSIBLE_DUPLICATE" {

		t.Fatalf(
			"unexpected question code: %s",
			result.Questions[0].Code,
		)
	}
}

func TestExactDuplicateDoesNotAskQuestion(
	t *testing.T,
) {

	receipt :=
		&models.ReceiptExtraction{
			Merchant: "Test Merchant",
		}

	validationResult :=
		validation.ReceiptValidationResult{
			Valid: true,
		}

	duplicateResult :=
		&duplicate.Result{
			IsDuplicate: true,
			MatchType:   duplicate.MatchExact,
			Confidence:  1,
		}

	result :=
		Evaluate(
			receipt,
			validationResult,
			duplicateResult,
		)

	if result.Required {
		t.Fatal(
			"exact duplicate should be blocked without clarification",
		)
	}
}

func TestMissingMerchantRequiresClarification(
	t *testing.T,
) {

	receipt :=
		&models.ReceiptExtraction{
			ExpenseDate:        "2026-10-09",
			TotalAmount:        100,
			Currency:           "INR",
			CategorySuggestion: "Meals",
			Confidence:         0.95,
		}

	validationResult :=
		validation.ReceiptValidationResult{
			Valid: true,
		}

	result :=
		Evaluate(
			receipt,
			validationResult,
			nil,
		)

	if !result.Required {
		t.Fatal(
			"expected clarification for missing merchant",
		)
	}

	if result.Questions[0].Code !=
		"MISSING_MERCHANT" {

		t.Fatalf(
			"expected MISSING_MERCHANT, got %s",
			result.Questions[0].Code,
		)
	}
}

func TestCleanReceiptNeedsNoClarification(
	t *testing.T,
) {

	receipt :=
		&models.ReceiptExtraction{
			Merchant:           "ABC Restaurant",
			ExpenseDate:        "2026-10-09",
			TotalAmount:        500,
			Currency:           "INR",
			CategorySuggestion: "Meals",
			Confidence:         0.97,
		}

	validationResult :=
		validation.ReceiptValidationResult{
			Valid: true,
		}

	result :=
		Evaluate(
			receipt,
			validationResult,
			nil,
		)

	if result.Required {
		t.Fatalf(
			"expected no clarification, got %+v",
			result.Questions,
		)
	}
}
