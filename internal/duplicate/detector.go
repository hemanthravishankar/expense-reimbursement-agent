package duplicate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

const (
	MatchNone     = "NONE"
	MatchExact    = "EXACT_HASH"
	MatchMetadata = "METADATA_MATCH"
)

type Result struct {
	IsDuplicate  bool    `json:"is_duplicate"`
	MatchType    string  `json:"match_type"`
	Confidence   float64 `json:"confidence"`
	MatchedRunID string  `json:"matched_run_id,omitempty"`
	Reason       string  `json:"reason"`
}

type historicalAuditRecord struct {
	RunID           string                    `json:"run_id"`
	ReceiptSHA256   string                    `json:"receipt_sha256"`
	FinalExtraction *models.ReceiptExtraction `json:"final_extraction"`
}

func DetectExactHash(
	auditDirectory string,
	receiptSHA256 string,
) (Result, error) {

	result := Result{
		IsDuplicate: false,
		MatchType:   MatchNone,
		Confidence:  0,
		Reason:      "No exact receipt hash match was found.",
	}

	if strings.TrimSpace(
		receiptSHA256,
	) == "" {

		return result, nil
	}

	files, err :=
		filepath.Glob(
			filepath.Join(
				auditDirectory,
				"*.json",
			),
		)

	if err != nil {

		return result, fmt.Errorf(
			"scan audit directory: %w",
			err,
		)
	}

	for _, path := range files {

		data, err :=
			os.ReadFile(path)

		if err != nil {
			continue
		}

		var historical historicalAuditRecord

		if err := json.Unmarshal(
			data,
			&historical,
		); err != nil {

			continue
		}

		if historical.ReceiptSHA256 ==
			receiptSHA256 {

			return Result{
				IsDuplicate: true,

				MatchType: MatchExact,

				Confidence: 1.0,

				MatchedRunID: historical.RunID,

				Reason: "Receipt SHA-256 exactly matches a previously processed receipt.",
			}, nil
		}
	}

	return result, nil
}

func Detect(
	auditDirectory string,
	receiptSHA256 string,
	current *models.ReceiptExtraction,
) (Result, error) {

	result := Result{
		IsDuplicate: false,
		MatchType:   MatchNone,
		Confidence:  0,
		Reason:      "No matching historical receipt was found.",
	}

	files, err :=
		filepath.Glob(
			filepath.Join(
				auditDirectory,
				"*.json",
			),
		)

	if err != nil {
		return result, fmt.Errorf(
			"scan audit directory: %w",
			err,
		)
	}

	for _, path := range files {

		data, err :=
			os.ReadFile(path)

		if err != nil {
			continue
		}

		var historical historicalAuditRecord

		if err := json.Unmarshal(
			data,
			&historical,
		); err != nil {
			continue
		}

		//
		// Strongest possible signal:
		// same receipt file bytes.
		//
		if receiptSHA256 != "" &&
			historical.ReceiptSHA256 == receiptSHA256 {

			return Result{
				IsDuplicate:  true,
				MatchType:    MatchExact,
				Confidence:   1.0,
				MatchedRunID: historical.RunID,
				Reason:       "Receipt SHA-256 exactly matches a previously processed receipt.",
			}, nil
		}

		if current == nil ||
			historical.FinalExtraction == nil {

			continue
		}

		score :=
			metadataScore(
				current,
				historical.FinalExtraction,
			)

		//
		// Merchant + amount + date alone = 0.85.
		//
		if score >= 0.85 &&
			score > result.Confidence {

			result = Result{
				IsDuplicate:  true,
				MatchType:    MatchMetadata,
				Confidence:   score,
				MatchedRunID: historical.RunID,
				Reason:       "Receipt closely matches a previously processed expense based on merchant, date, amount, and receipt metadata.",
			}
		}
	}

	return result, nil
}

func metadataScore(
	current *models.ReceiptExtraction,
	historical *models.ReceiptExtraction,
) float64 {

	var score float64

	//
	// Merchant = 30%
	//
	currentMerchant :=
		normalizeText(
			current.Merchant,
		)

	historicalMerchant :=
		normalizeText(
			historical.Merchant,
		)

	if currentMerchant != "" &&
		currentMerchant == historicalMerchant {

		score += 0.30
	}

	//
	// Total amount = 30%
	//
	if current.TotalAmount > 0 &&
		historical.TotalAmount > 0 &&
		moneyEqual(
			current.TotalAmount,
			historical.TotalAmount,
		) {

		score += 0.30
	}

	//
	// Expense date = 25%
	//
	if current.ExpenseDate != "" &&
		current.ExpenseDate ==
			historical.ExpenseDate {

		score += 0.25
	}

	//
	// Receipt number = 15%
	//
	currentReceiptNumber :=
		normalizeText(
			current.ReceiptNumber,
		)

	historicalReceiptNumber :=
		normalizeText(
			historical.ReceiptNumber,
		)

	if currentReceiptNumber != "" &&
		historicalReceiptNumber != "" &&
		currentReceiptNumber ==
			historicalReceiptNumber {

		score += 0.15
	}

	return math.Round(
		score*100,
	) / 100
}

func normalizeText(
	value string,
) string {

	value =
		strings.ToLower(
			strings.TrimSpace(value),
		)

	var builder strings.Builder

	for _, r := range value {

		if unicode.IsLetter(r) ||
			unicode.IsDigit(r) {

			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func moneyEqual(
	a float64,
	b float64,
) bool {

	return math.Abs(a-b) < 0.01
}
