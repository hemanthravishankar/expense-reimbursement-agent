package policy

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestMealExpenseExceedsLimit(t *testing.T) {

	employee := models.Employee{
		ID:    "EMP10042",
		Name:  "Ananya Rao",
		Grade: "B2",
	}

	receipt := models.ReceiptExtraction{
		CategorySuggestion: "Meals",
		TotalAmount:        54.20,
		Currency:           "USD",
	}

	result :=
		Evaluate(
			&receipt,
			employee,
			DemoUSDPolicy(),
		)

	if result.Status != StatusLimitExceeded {
		t.Fatalf(
			"expected LIMIT_EXCEEDED, got %s",
			result.Status,
		)
	}

	if result.ReimbursableAmount != 50 {
		t.Fatalf(
			"expected reimbursable amount 50.00, got %.2f",
			result.ReimbursableAmount,
		)
	}

	if result.ExcessAmount != 4.20 {
		t.Fatalf(
			"expected excess amount 4.20, got %.2f",
			result.ExcessAmount,
		)
	}
}

func TestMealExpenseWithinLimit(t *testing.T) {

	employee := models.Employee{
		ID:    "EMP10042",
		Name:  "Ananya Rao",
		Grade: "B2",
	}

	receipt := models.ReceiptExtraction{
		CategorySuggestion: "Meals",
		TotalAmount:        40,
		Currency:           "USD",
	}

	result :=
		Evaluate(
			&receipt,
			employee,
			DemoUSDPolicy(),
		)

	if result.Status != StatusApproved {
		t.Fatalf(
			"expected APPROVED, got %s",
			result.Status,
		)
	}

	if result.ReimbursableAmount != 40 {
		t.Fatalf(
			"expected reimbursable amount 40.00, got %.2f",
			result.ReimbursableAmount,
		)
	}
}
