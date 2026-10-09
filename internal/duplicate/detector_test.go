package duplicate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

func TestDetectExactDuplicate(t *testing.T) {

	dir :=
		t.TempDir()

	writeHistory(
		t,
		dir,
		"run-001",
		"abc123",
		&models.ReceiptExtraction{
			Merchant:      "FourthStreetMill",
			ExpenseDate:   "2018-09-29",
			TotalAmount:   54.20,
			ReceiptNumber: "4J33",
		},
	)

	result, err :=
		Detect(
			dir,
			"abc123",
			nil,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !result.IsDuplicate {
		t.Fatal(
			"expected exact duplicate",
		)
	}

	if result.MatchType != MatchExact {
		t.Fatalf(
			"expected EXACT_HASH, got %s",
			result.MatchType,
		)
	}

	if result.Confidence != 1 {
		t.Fatalf(
			"expected confidence 1.0, got %.2f",
			result.Confidence,
		)
	}
}

func TestDetectMetadataDuplicate(t *testing.T) {

	dir :=
		t.TempDir()

	historical :=
		&models.ReceiptExtraction{
			Merchant: "FourthStreetMill",

			ExpenseDate: "2018-09-29",

			TotalAmount: 54.20,
		}

	writeHistory(
		t,
		dir,
		"run-002",
		"old-hash",
		historical,
	)

	current :=
		&models.ReceiptExtraction{
			Merchant: "Fourth Street Mill",

			ExpenseDate: "2018-09-29",

			TotalAmount: 54.20,
		}

	result, err :=
		Detect(
			dir,
			"different-hash",
			current,
		)

	if err != nil {
		t.Fatal(err)
	}

	if !result.IsDuplicate {
		t.Fatal(
			"expected metadata duplicate",
		)
	}

	if result.MatchType != MatchMetadata {
		t.Fatalf(
			"expected METADATA_MATCH, got %s",
			result.MatchType,
		)
	}

	if result.Confidence < 0.85 {
		t.Fatalf(
			"expected confidence >= 0.85, got %.2f",
			result.Confidence,
		)
	}
}

func TestNoDuplicate(t *testing.T) {

	dir :=
		t.TempDir()

	writeHistory(
		t,
		dir,
		"run-003",
		"old-hash",
		&models.ReceiptExtraction{
			Merchant:    "Hotel ABC",
			ExpenseDate: "2026-10-01",
			TotalAmount: 200,
		},
	)

	current :=
		&models.ReceiptExtraction{
			Merchant: "Taxi XYZ",

			ExpenseDate: "2026-10-02",

			TotalAmount: 30,
		}

	result, err :=
		Detect(
			dir,
			"new-hash",
			current,
		)

	if err != nil {
		t.Fatal(err)
	}

	if result.IsDuplicate {
		t.Fatal(
			"expected receipt not to be duplicate",
		)
	}
}

func writeHistory(
	t *testing.T,
	dir string,
	runID string,
	hash string,
	extraction *models.ReceiptExtraction,
) {

	t.Helper()

	record :=
		map[string]any{
			"run_id": runID,

			"receipt_sha256": hash,

			"final_extraction": extraction,
		}

	data, err :=
		json.Marshal(record)

	if err != nil {
		t.Fatal(err)
	}

	path :=
		filepath.Join(
			dir,
			runID+".json",
		)

	if err := os.WriteFile(
		path,
		data,
		0644,
	); err != nil {

		t.Fatal(err)
	}
}

func TestDetectExactHashBeforeExtraction(
	t *testing.T,
) {

	dir :=
		t.TempDir()

	writeHistory(
		t,
		dir,
		"existing-run",
		"same-sha256",
		&models.ReceiptExtraction{
			Merchant: "ABC Restaurant",
		},
	)

	result, err :=
		DetectExactHash(
			dir,
			"same-sha256",
		)

	if err != nil {
		t.Fatal(err)
	}

	if !result.IsDuplicate {

		t.Fatal(
			"expected exact duplicate",
		)
	}

	if result.MatchType != MatchExact {

		t.Fatalf(
			"expected %s, got %s",
			MatchExact,
			result.MatchType,
		)
	}

	if result.Confidence != 1 {

		t.Fatalf(
			"expected confidence 1.0, got %.2f",
			result.Confidence,
		)
	}
}
