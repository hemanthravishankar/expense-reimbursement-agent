package audit

import (
	"os"
	"testing"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

func TestSaveAuditRecord(t *testing.T) {

	tempDir := t.TempDir()

	record := Record{
		RunID:         "test-run",
		ReceiptPath:   "receipt.jpg",
		ReceiptSHA256: "abc123",

		StartedAt:   time.Now(),
		CompletedAt: time.Now(),

		PrimaryModel: "qwen3-vl:4b-instruct",

		FallbackUsed: false,

		InitialExtraction: &models.ReceiptExtraction{
			Merchant:    "Test Restaurant",
			TotalAmount: 100,
			Currency:    "USD",
		},

		InitialValidation: validation.ReceiptValidationResult{
			Valid: true,
		},

		FinalExtraction: &models.ReceiptExtraction{
			Merchant:    "Test Restaurant",
			TotalAmount: 100,
			Currency:    "USD",
		},

		FinalValidation: validation.ReceiptValidationResult{
			Valid: true,
		},

		FinalStatus: "VALIDATED",
	}

	path, err :=
		Save(tempDir, record)

	if err != nil {
		t.Fatalf(
			"failed to save audit record: %v",
			err,
		)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf(
			"audit file was not created: %v",
			err,
		)
	}
}
