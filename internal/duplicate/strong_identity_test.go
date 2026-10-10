package duplicate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestMetadataScoreStrongIdentity(t *testing.T) {
	current := &models.ReceiptExtraction{
		Merchant:      "THE DAILY GRILL",
		ExpenseDate:   "2024-02-12",
		TotalAmount:   55.20,
		ReceiptNumber: "78452",
	}

	historical := &models.ReceiptExtraction{
		Merchant:      "The Daily Grill Restaurant & Bar",
		ExpenseDate:   "2024-02-12",
		TotalAmount:   55.20,
		ReceiptNumber: "78452",
	}

	score, strongIdentity := metadataScore(current, historical)

	if !strongIdentity {
		t.Fatal("expected receipt number + date + amount to be strong identity evidence")
	}

	if score < 0.95 {
		t.Fatalf("expected strong identity score >= 0.95, got %.2f", score)
	}
}

func TestMetadataScoreOrdinarySimilarityIsNotStrongIdentity(t *testing.T) {
	current := &models.ReceiptExtraction{
		Merchant:      "THE DAILY GRILL",
		ExpenseDate:   "2024-02-12",
		TotalAmount:   55.20,
		ReceiptNumber: "78453",
	}

	historical := &models.ReceiptExtraction{
		Merchant:      "THE DAILY GRILL",
		ExpenseDate:   "2024-02-12",
		TotalAmount:   55.20,
		ReceiptNumber: "78452",
	}

	score, strongIdentity := metadataScore(current, historical)

	if strongIdentity {
		t.Fatal("different receipt numbers must not be classified as strong identity")
	}

	if score < metadataThreshold {
		t.Fatalf("expected ordinary metadata similarity to still reach threshold %.2f, got %.2f", metadataThreshold, score)
	}
}

func TestDetectForEmployeePropagatesStrongIdentity(t *testing.T) {
	dir := t.TempDir()

	historical := historicalAuditRecord{
		RunID:         "historical-run",
		ReceiptSHA256: "historical-hash",
		Employee: models.Employee{
			ID:   "EMP60001",
			Name: "Kiran Kumar",
		},
		FinalExtraction: &models.ReceiptExtraction{
			Merchant:           "THE DAILY GRILL",
			ExpenseDate:        "2024-02-12",
			TotalAmount:        55.20,
			ReceiptNumber:      "78452",
			Currency:           "USD",
			CategorySuggestion: "Meals",
		},
	}

	data, err := json.Marshal(historical)
	if err != nil {
		t.Fatalf("marshal historical audit: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(dir, "historical-run.json"),
		data,
		0o600,
	); err != nil {
		t.Fatalf("write historical audit: %v", err)
	}

	current := &models.ReceiptExtraction{
		Merchant:           "THE DAILY GRILL",
		ExpenseDate:        "2024-02-12",
		TotalAmount:        55.20,
		ReceiptNumber:      "78452",
		Currency:           "USD",
		CategorySuggestion: "Meals",
	}

	result, err := DetectForEmployee(
		dir,
		"different-binary-hash",
		current,
		"EMP60001",
	)
	if err != nil {
		t.Fatalf("DetectForEmployee returned error: %v", err)
	}

	if result.MatchType != MatchMetadata {
		t.Fatalf("expected %s, got %s", MatchMetadata, result.MatchType)
	}

	if !result.IsDuplicate {
		t.Fatal("expected same-employee metadata match to be a duplicate")
	}

	if !result.StrongIdentity {
		t.Fatal("expected strong identity to be propagated into duplicate.Result")
	}
}
