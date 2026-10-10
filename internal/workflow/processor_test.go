package workflow

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/audit"
	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
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

func TestCrossEmployeeExactRequiresManualReview(
	t *testing.T,
) {

	auditDir :=
		t.TempDir()

	documentDir :=
		t.TempDir()

	documentData :=
		createWorkflowPNG(t)

	extraction :=
		&models.ReceiptExtraction{
			DocumentType: "receipt",

			Merchant: "THE DAILY GRILL",

			ExpenseDate: "2024-02-12",

			Subtotal: 48,

			TaxAmount: 2.4,

			ServiceCharge: 4.8,

			TotalAmount: 55.2,

			Currency: "USD",

			ReceiptNumber: "78452",

			CategorySuggestion: "Meals",

			Confidence: 0.98,

			LineItems: []models.ReceiptLineItem{
				{
					Name:     "Meal",
					Quantity: 1,
					Amount:   48,
				},
			},
		}

	config :=
		DefaultConfig()

	config.AuditDirectory =
		auditDir

	config.DocumentDirectory =
		documentDir

	firstEmployee :=
		models.Employee{
			ID:    "EMP60001",
			Name:  "Kiran Kumar",
			Grade: "B2",
		}

	firstProcessor :=
		NewProcessor(
			&fakeExtractor{
				primary: extraction,
			},
			config,
		)

	firstResult, err :=
		firstProcessor.ProcessReceipt(
			context.Background(),
			"report-meal.png",
			documentData,
			firstEmployee,
		)

	if err != nil {
		t.Fatal(err)
	}

	if firstResult.FinalStatus !=
		"LIMIT_EXCEEDED" {

		t.Fatalf(
			"expected first employee claim to reach policy and be LIMIT_EXCEEDED, got %s",
			firstResult.FinalStatus,
		)
	}

	secondExtractor :=
		&fakeExtractor{
			primary: extraction,
		}

	secondEmployee :=
		models.Employee{
			ID:    "EMP60002",
			Name:  "Meera Nair",
			Grade: "B2",
		}

	secondProcessor :=
		NewProcessor(
			secondExtractor,
			config,
		)

	result, err :=
		secondProcessor.ProcessReceipt(
			context.Background(),
			"same-receipt.png",
			documentData,
			secondEmployee,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !secondExtractor.primaryCalled {

		t.Fatal(
			"cross-employee exact match should continue through extraction before review routing",
		)
	}

	if result.DuplicateResult == nil {

		t.Fatal(
			"expected duplicate result",
		)
	}

	if result.DuplicateResult.MatchType !=
		duplicate.MatchCrossEmployeeExact {

		t.Fatalf(
			"expected %s, got %s",
			duplicate.MatchCrossEmployeeExact,
			result.DuplicateResult.MatchType,
		)
	}

	if !result.DuplicateResult.RequiresAdminReview {

		t.Fatal(
			"expected requires_admin_review=true",
		)
	}

	if !result.DuplicateResult.StrongIdentity {

		t.Fatal(
			"expected cross-employee exact match to be strong identity",
		)
	}

	if result.FinalStatus !=
		StatusManualReview {

		t.Fatalf(
			"expected %s, got %s",
			StatusManualReview,
			result.FinalStatus,
		)
	}

	if result.PolicyDecision != nil {

		t.Fatal(
			"policy decision must remain nil while Finance/Admin review is pending",
		)
	}

	if result.AuditPath == "" {

		t.Fatal(
			"manual-review claim should still be audited",
		)
	}
}

func TestCrossEmployeeMetadataRequiresManualReview(
	t *testing.T,
) {

	auditDir :=
		t.TempDir()

	documentDir :=
		t.TempDir()

	firstDocument :=
		createWorkflowPNGWithSize(
			t,
			4,
			4,
		)

	secondDocument :=
		createWorkflowPNGWithSize(
			t,
			5,
			5,
		)

	extraction :=
		&models.ReceiptExtraction{
			DocumentType: "receipt",

			Merchant: "THE DAILY GRILL",

			ExpenseDate: "2024-02-12",

			Subtotal: 48,

			TaxAmount: 2.4,

			ServiceCharge: 4.8,

			TotalAmount: 55.2,

			Currency: "USD",

			ReceiptNumber: "78452",

			CategorySuggestion: "Meals",

			Confidence: 0.98,

			LineItems: []models.ReceiptLineItem{
				{
					Name:     "Meal",
					Quantity: 1,
					Amount:   48,
				},
			},
		}

	config :=
		DefaultConfig()

	config.AuditDirectory =
		auditDir

	config.DocumentDirectory =
		documentDir

	firstProcessor :=
		NewProcessor(
			&fakeExtractor{
				primary: extraction,
			},
			config,
		)

	_, err :=
		firstProcessor.ProcessReceipt(
			context.Background(),
			"original.png",
			firstDocument,
			models.Employee{
				ID:    "EMP70001",
				Name:  "Employee One",
				Grade: "B2",
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	secondProcessor :=
		NewProcessor(
			&fakeExtractor{
				primary: extraction,
			},
			config,
		)

	result, err :=
		secondProcessor.ProcessReceipt(
			context.Background(),
			"reencoded.png",
			secondDocument,
			models.Employee{
				ID:    "EMP70002",
				Name:  "Employee Two",
				Grade: "B2",
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if result.DuplicateResult == nil {

		t.Fatal(
			"expected duplicate result",
		)
	}

	if result.DuplicateResult.MatchType !=
		duplicate.MatchCrossEmployeeMetadata {

		t.Fatalf(
			"expected %s, got %s",
			duplicate.MatchCrossEmployeeMetadata,
			result.DuplicateResult.MatchType,
		)
	}

	if !result.DuplicateResult.RequiresAdminReview {

		t.Fatal(
			"expected requires_admin_review=true",
		)
	}

	if !result.DuplicateResult.StrongIdentity {

		t.Fatal(
			"expected receipt number + date + amount to be strong identity evidence",
		)
	}

	if result.FinalStatus !=
		StatusManualReview {

		t.Fatalf(
			"expected %s, got %s",
			StatusManualReview,
			result.FinalStatus,
		)
	}

	if result.PolicyDecision != nil {

		t.Fatal(
			"cross-employee metadata risk must not authorize reimbursement automatically",
		)
	}
}

func TestNormalReceiptStillReachesPolicyAfterAdminReviewGate(
	t *testing.T,
) {

	auditDir :=
		t.TempDir()

	documentDir :=
		t.TempDir()

	extractor :=
		&fakeExtractor{
			primary: &models.ReceiptExtraction{
				DocumentType: "receipt",

				Merchant: "Independent Cafe",

				ExpenseDate: "2026-10-10",

				Subtotal: 20,

				TotalAmount: 20,

				Currency: "USD",

				ReceiptNumber: "UNIQUE-9001",

				CategorySuggestion: "Meals",

				Confidence: 0.99,

				LineItems: []models.ReceiptLineItem{
					{
						Name:     "Lunch",
						Quantity: 1,
						Amount:   20,
					},
				},
			},
		}

	config :=
		DefaultConfig()

	config.AuditDirectory =
		auditDir

	config.DocumentDirectory =
		documentDir

	processor :=
		NewProcessor(
			extractor,
			config,
		)

	result, err :=
		processor.ProcessReceipt(
			context.Background(),
			"unique.png",
			createWorkflowPNG(t),
			models.Employee{
				ID:    "EMP90001",
				Name:  "Unique Employee",
				Grade: "B2",
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if result.FinalStatus !=
		"APPROVED" {

		t.Fatalf(
			"expected APPROVED, got %s",
			result.FinalStatus,
		)
	}

	if result.PolicyDecision == nil {

		t.Fatal(
			"expected normal claim to receive policy decision",
		)
	}

	if result.DuplicateResult == nil {

		t.Fatal(
			"expected duplicate result object",
		)
	}

	if result.DuplicateResult.RequiresAdminReview {

		t.Fatal(
			"normal unique claim must not require admin review",
		)
	}
}

func createWorkflowPNG(
	t *testing.T,
) []byte {

	return createWorkflowPNGWithSize(
		t,
		4,
		4,
	)
}

func createWorkflowPNGWithSize(
	t *testing.T,
	width int,
	height int,
) []byte {

	t.Helper()

	img :=
		image.NewRGBA(
			image.Rect(
				0,
				0,
				width,
				height,
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
