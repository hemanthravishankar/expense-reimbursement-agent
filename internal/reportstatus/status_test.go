package reportstatus

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestCalculateCompleted(t *testing.T) {

	items := []models.ExpenseReportItem{
		{
			ClaimID: "C1",
			Status:  "APPROVED",
		},
		{
			ClaimID: "C2",
			Status:  "LIMIT_EXCEEDED",
		},
	}

	got := Calculate(items)

	if got != "COMPLETED" {
		t.Fatalf(
			"expected COMPLETED, got %s",
			got,
		)
	}
}

func TestCalculateActionRequired(t *testing.T) {

	items := []models.ExpenseReportItem{
		{
			ClaimID: "C1",
			Status:  "CLARIFICATION_REQUIRED",
		},
	}

	got := Calculate(items)

	if got != "ACTION_REQUIRED" {
		t.Fatalf(
			"expected ACTION_REQUIRED, got %s",
			got,
		)
	}
}

func TestCalculateDuplicateRejected(t *testing.T) {

	items := []models.ExpenseReportItem{
		{
			ClaimID: "C1",
			Status:  "DUPLICATE_DETECTED",
		},
	}

	got := Calculate(items)

	if got != "REJECTED" {
		t.Fatalf(
			"expected REJECTED, got %s",
			got,
		)
	}
}

func TestCalculatePartialRejection(t *testing.T) {

	items := []models.ExpenseReportItem{
		{
			ClaimID: "C1",
			Status:  "APPROVED",
		},
		{
			ClaimID: "C2",
			Status:  "DUPLICATE_DETECTED",
		},
	}

	got := Calculate(items)

	if got != "PARTIALLY_REJECTED" {
		t.Fatalf(
			"expected PARTIALLY_REJECTED, got %s",
			got,
		)
	}
}

func TestCalculateHumanReview(t *testing.T) {

	items := []models.ExpenseReportItem{
		{
			ClaimID: "C1",
			Status:  "HUMAN_REVIEW_REQUIRED",
		},
	}

	got := Calculate(items)

	if got != "REVIEW_REQUIRED" {
		t.Fatalf(
			"expected REVIEW_REQUIRED, got %s",
			got,
		)
	}
}

func TestCalculateAllRejected(t *testing.T) {

	items := []models.ExpenseReportItem{
		{
			ClaimID: "C1",
			Status:  "NOT_ALLOWED",
		},
		{
			ClaimID: "C2",
			Status:  "DUPLICATE_DETECTED",
		},
	}

	got := Calculate(items)

	if got != "REJECTED" {
		t.Fatalf(
			"expected REJECTED, got %s",
			got,
		)
	}
}
