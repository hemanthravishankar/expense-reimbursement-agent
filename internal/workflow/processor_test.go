package workflow

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/audit"
	"github.com/hemanth-shankar/expense-agent/internal/models"
)

type fakeExtractor struct {
	primary  *models.ReceiptExtraction
	fallback *models.ReceiptExtraction

	primaryCalled  bool
	fallbackCalled bool
}

func (f *fakeExtractor) ExtractReceipt(
	ctx context.Context,
	pages [][]byte,
) (*models.ReceiptExtraction, error) {

	f.primaryCalled =
		true

	return cloneExtraction(
		f.primary,
	), nil
}

func (f *fakeExtractor) ExtractReceiptWithFallback(
	ctx context.Context,
	pages [][]byte,
) (*models.ReceiptExtraction, error) {

	f.fallbackCalled =
		true

	return cloneExtraction(
		f.fallback,
	), nil
}

func TestProcessCleanReceipt(
	t *testing.T,
) {

	auditDir :=
		t.TempDir()

	extractor :=
		&fakeExtractor{
			primary: &models.ReceiptExtraction{
				DocumentType: "receipt",
				Merchant:     "ABC Restaurant",
				ExpenseDate:  "2026-10-09",
				Subtotal:     40,
				TaxAmount:    0,
				TotalAmount:  40,
				Currency:     "USD",

				ReceiptNumber: "R100",

				CategorySuggestion: "Meals",

				Confidence: 0.98,

				LineItems: []models.ReceiptLineItem{
					{
						Name:     "Dinner",
						Quantity: 1,
						Amount:   40,
					},
				},
			},
		}

	config :=
		DefaultConfig()

	config.AuditDirectory =
		auditDir

	processor :=
		NewProcessor(
			extractor,
			config,
		)

	employee :=
		models.Employee{
			ID:    "EMP10042",
			Name:  "Ananya Rao",
			Grade: "B2",
		}

	result, err :=
		processor.ProcessReceipt(
			context.Background(),
			"receipt.png",
			createWorkflowPNG(t),
			employee,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !extractor.primaryCalled {

		t.Fatal(
			"expected primary extractor to run",
		)
	}

	if !result.FinalValidation.Valid {

		t.Fatalf(
			"expected valid receipt, got %+v",
			result.FinalValidation.Issues,
		)
	}

	if result.FinalStatus != "APPROVED" {

		t.Fatalf(
			"expected APPROVED, got %s",
			result.FinalStatus,
		)
	}

	if result.PageCount != 1 {

		t.Fatalf(
			"expected 1 page, got %d",
			result.PageCount,
		)
	}

	if result.AuditPath == "" {

		t.Fatal(
			"expected audit path",
		)
	}

	if _, err :=
		os.Stat(
			result.AuditPath,
		); err != nil {

		t.Fatalf(
			"expected audit file: %v",
			err,
		)
	}
}

func TestFallbackCorrectsInvalidExtraction(
	t *testing.T,
) {

	auditDir :=
		t.TempDir()

	extractor :=
		&fakeExtractor{
			primary: &models.ReceiptExtraction{
				DocumentType: "receipt",

				Merchant: "FourthStreetMill",

				ExpenseDate: "2018-09-29",

				Subtotal: 49.50,

				TaxAmount: 4.70,

				TotalAmount: 54.20,

				Currency: "USD",

				CategorySuggestion: "Meals",

				Confidence: 0.98,

				LineItems: []models.ReceiptLineItem{
					{
						Name: "Incorrect items",

						Amount: 59.50,
					},
				},
			},

			fallback: &models.ReceiptExtraction{
				DocumentType: "receipt",

				Merchant: "FourthStreetMill",

				ExpenseDate: "2018-09-29",

				Subtotal: 49.50,

				TaxAmount: 4.70,

				TotalAmount: 54.20,

				Currency: "USD",

				CategorySuggestion: "Meals",

				Confidence: 0.98,

				LineItems: []models.ReceiptLineItem{
					{
						Name: "Purchased items",

						Amount: 49.50,
					},
				},
			},
		}

	config :=
		DefaultConfig()

	config.AuditDirectory =
		auditDir

	processor :=
		NewProcessor(
			extractor,
			config,
		)

	employee :=
		models.Employee{
			ID:    "EMP10042",
			Name:  "Ananya Rao",
			Grade: "B2",
		}

	result, err :=
		processor.ProcessReceipt(
			context.Background(),
			"receipt.png",
			createWorkflowPNG(t),
			employee,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !extractor.fallbackCalled {

		t.Fatal(
			"expected fallback extractor to be called",
		)
	}

	if !result.FallbackUsed {

		t.Fatal(
			"expected fallback_used=true",
		)
	}

	if !result.FinalValidation.Valid {

		t.Fatal(
			"expected fallback extraction to pass validation",
		)
	}

	if result.FinalStatus !=
		"LIMIT_EXCEEDED" {

		t.Fatalf(
			"expected LIMIT_EXCEEDED, got %s",
			result.FinalStatus,
		)
	}
}

func TestExactDuplicateSkipsAI(
	t *testing.T,
) {

	auditDir :=
		t.TempDir()

	documentData :=
		createWorkflowPNG(t)

	//
	// Process once to create historical audit.
	//
	firstExtractor :=
		&fakeExtractor{
			primary: &models.ReceiptExtraction{
				DocumentType: "receipt",

				Merchant: "ABC Restaurant",

				ExpenseDate: "2026-10-09",

				Subtotal: 40,

				TotalAmount: 40,

				Currency: "USD",

				ReceiptNumber: "R200",

				CategorySuggestion: "Meals",

				Confidence: 0.98,

				LineItems: []models.ReceiptLineItem{
					{
						Name: "Dinner",

						Quantity: 1,

						Amount: 40,
					},
				},
			},
		}

	config :=
		DefaultConfig()

	config.AuditDirectory =
		auditDir

	employee :=
		models.Employee{
			ID:    "EMP10042",
			Name:  "Ananya Rao",
			Grade: "B2",
		}

	firstProcessor :=
		NewProcessor(
			firstExtractor,
			config,
		)

	_, err :=
		firstProcessor.ProcessReceipt(
			context.Background(),
			"receipt.png",
			documentData,
			employee,
		)

	if err != nil {
		t.Fatal(err)
	}

	//
	// Process exact same file again.
	//
	secondExtractor :=
		&fakeExtractor{
			primary: firstExtractor.primary,
		}

	secondProcessor :=
		NewProcessor(
			secondExtractor,
			config,
		)

	result, err :=
		secondProcessor.ProcessReceipt(
			context.Background(),
			"renamed-receipt.png",
			documentData,
			employee,
		)

	if err != nil {
		t.Fatal(err)
	}

	if secondExtractor.primaryCalled {

		t.Fatal(
			"AI extraction should NOT run for exact duplicate",
		)
	}

	if !result.AIProcessingSkipped {

		t.Fatal(
			"expected ai_processing_skipped=true",
		)
	}

	if result.FinalStatus !=
		StatusDuplicateDetected {

		t.Fatalf(
			"expected %s, got %s",
			StatusDuplicateDetected,
			result.FinalStatus,
		)
	}

	if result.DuplicateResult == nil ||
		!result.DuplicateResult.IsDuplicate {

		t.Fatal(
			"expected exact duplicate result",
		)
	}

	if result.AuditPath == "" {

		t.Fatal(
			"duplicate attempt should still be audited",
		)
	}

	//
	// Ensure audit file exists.
	//
	_, err =
		os.Stat(
			result.AuditPath,
		)

	if err != nil {
		t.Fatal(err)
	}

	//
	// Compile-time use to ensure audit package
	// remains integrated.
	//
	_ = audit.Record{}
}

func createWorkflowPNG(
	t *testing.T,
) []byte {

	t.Helper()

	img :=
		image.NewRGBA(
			image.Rect(
				0,
				0,
				4,
				4,
			),
		)

	var buffer bytes.Buffer

	if err :=
		png.Encode(
			&buffer,
			img,
		); err != nil {

		t.Fatal(err)
	}

	return buffer.Bytes()
}
