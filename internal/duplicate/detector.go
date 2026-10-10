package duplicate

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/models"
)

const (
	MatchNone                  = "NONE"
	MatchExact                 = "EXACT_HASH"
	MatchMetadata              = "METADATA_MATCH"
	MatchCrossEmployeeExact    = "CROSS_EMPLOYEE_EXACT"
	MatchCrossEmployeeMetadata = "CROSS_EMPLOYEE_METADATA"
)

const metadataThreshold = 0.85

type Result struct {
	IsDuplicate bool `json:"is_duplicate"`

	RequiresAdminReview bool `json:"requires_admin_review"`

	MatchType string `json:"match_type"`

	Confidence float64 `json:"confidence"`

	// StrongIdentity means the match is supported by transaction-level
	// identity evidence, not only general similarity. For metadata matches
	// this currently requires the same non-empty receipt number, expense
	// date, and total amount.
	StrongIdentity bool `json:"strong_identity"`

	MatchedRunID string `json:"matched_run_id,omitempty"`

	MatchedEmployeeID string `json:"matched_employee_id,omitempty"`

	Reason string `json:"reason"`
}

type historicalAuditRecord struct {
	RunID string `json:"run_id"`

	ReceiptSHA256 string `json:"receipt_sha256"`

	Employee models.Employee `json:"employee"`

	InitialExtraction *models.ReceiptExtraction `json:"initial_extraction"`

	FinalExtraction *models.ReceiptExtraction `json:"final_extraction"`
}

type metadataCandidate struct {
	RunID string

	EmployeeID string

	Score float64

	StrongIdentity bool
}

var nonAlphaNumeric = regexp.MustCompile(
	`[^a-z0-9]+`,
)

// ---------------------------------------------------------
// Backward-compatible APIs
// ---------------------------------------------------------

func DetectExactHash(
	auditDirectory string,
	receiptSHA256 string,
) (Result, error) {

	return DetectExactHashForEmployee(
		auditDirectory,
		receiptSHA256,
		"",
	)
}

func Detect(
	auditDirectory string,
	receiptSHA256 string,
	current *models.ReceiptExtraction,
) (Result, error) {

	return DetectForEmployee(
		auditDirectory,
		receiptSHA256,
		current,
		"",
	)
}

// ---------------------------------------------------------
// Employee-aware APIs
// ---------------------------------------------------------

func DetectExactHashForEmployee(
	auditDirectory string,
	receiptSHA256 string,
	employeeID string,
) (Result, error) {

	if strings.TrimSpace(
		receiptSHA256,
	) == "" {

		return noMatch(), nil
	}

	records, err :=
		loadAuditRecords(
			auditDirectory,
		)

	if err != nil {
		return Result{}, err
	}

	var crossEmployeeMatch *historicalAuditRecord

	for i := range records {

		record :=
			records[i]

		if !strings.EqualFold(
			strings.TrimSpace(
				record.ReceiptSHA256,
			),
			strings.TrimSpace(
				receiptSHA256,
			),
		) {
			continue
		}

		if sameEmployee(
			employeeID,
			record.Employee.ID,
		) {

			return Result{
				IsDuplicate: true,

				RequiresAdminReview: false,

				MatchType: MatchExact,

				Confidence: 1.0,

				StrongIdentity: true,

				MatchedRunID: record.RunID,

				MatchedEmployeeID: record.Employee.ID,

				Reason: "An identical document was previously submitted by the same employee.",
			}, nil
		}

		//
		// Do not immediately return a cross-employee
		// match. A same-employee match has higher
		// priority if one exists elsewhere in history.
		//
		if crossEmployeeMatch == nil {
			copied :=
				record

			crossEmployeeMatch =
				&copied
		}
	}

	if crossEmployeeMatch != nil {

		return Result{
			IsDuplicate: false,

			RequiresAdminReview: true,

			MatchType: MatchCrossEmployeeExact,

			Confidence: 1.0,

			StrongIdentity: true,

			MatchedRunID: crossEmployeeMatch.RunID,

			MatchedEmployeeID: crossEmployeeMatch.Employee.ID,

			Reason: "An identical document was previously submitted by a different employee. Finance review is required before any reimbursement can be authorized.",
		}, nil
	}

	return noMatch(), nil
}

func DetectForEmployee(
	auditDirectory string,
	receiptSHA256 string,
	current *models.ReceiptExtraction,
	employeeID string,
) (Result, error) {

	//
	// First check exact hashes.
	//
	// IMPORTANT:
	//
	// A same-employee exact match can immediately
	// stop processing.
	//
	// A cross-employee exact match must NOT return
	// yet, because there may still be a historical
	// same-employee metadata match which has higher
	// duplicate-handling priority.
	//
	exact, err :=
		DetectExactHashForEmployee(
			auditDirectory,
			receiptSHA256,
			employeeID,
		)

	if err != nil {
		return Result{}, err
	}

	//
	// Same employee + same binary.
	//
	if exact.IsDuplicate &&
		exact.MatchType ==
			MatchExact {

		return exact, nil
	}

	hasCrossEmployeeExact :=
		exact.MatchType ==
			MatchCrossEmployeeExact

	//
	// Without extracted metadata we cannot do
	// the second-stage comparison.
	//
	// In that case preserve any cross-employee
	// exact risk signal.
	//
	if current == nil {

		if hasCrossEmployeeExact {
			return exact, nil
		}

		return noMatch(), nil
	}

	records, err :=
		loadAuditRecords(
			auditDirectory,
		)

	if err != nil {
		return Result{}, err
	}

	var bestSameEmployee *metadataCandidate
	var bestCrossEmployee *metadataCandidate

	for _, record := range records {

		historical :=
			record.FinalExtraction

		if historical == nil {
			historical =
				record.InitialExtraction
		}

		if historical == nil {
			continue
		}

		score,
			strongIdentity :=
			metadataScore(
				current,
				historical,
			)

		if score <
			metadataThreshold &&
			!strongIdentity {

			continue
		}

		//
		// Receipt number + date + amount is
		// strong identity evidence even if
		// merchant wording differs slightly.
		//
		if strongIdentity &&
			score < 0.95 {

			score =
				0.95
		}

		candidate :=
			metadataCandidate{
				RunID: record.RunID,

				EmployeeID: record.Employee.ID,

				Score: score,

				StrongIdentity: strongIdentity,
			}

		if sameEmployee(
			employeeID,
			record.Employee.ID,
		) {

			if bestSameEmployee == nil ||
				candidate.Score >
					bestSameEmployee.Score {

				copied :=
					candidate

				bestSameEmployee =
					&copied
			}

			continue
		}

		if bestCrossEmployee == nil ||
			candidate.Score >
				bestCrossEmployee.Score {

			copied :=
				candidate

			bestCrossEmployee =
				&copied
		}
	}

	// =====================================================
	// PRIORITY 1
	// SAME EMPLOYEE METADATA DUPLICATE
	// =====================================================
	//
	// This deliberately outranks a cross-employee
	// exact match.
	//
	// Example:
	//
	// Kiran previously submitted receipt A.
	// Meera submitted an exact copy B.
	// Kiran now submits B.
	//
	// For Kiran, the important employee-facing
	// result is still:
	//
	//     METADATA_MATCH against Kiran's
	//     historical receipt.
	//
	if bestSameEmployee != nil {

		return Result{
			IsDuplicate: true,

			RequiresAdminReview: false,

			MatchType: MatchMetadata,

			Confidence: bestSameEmployee.Score,

			StrongIdentity: bestSameEmployee.StrongIdentity,

			MatchedRunID: bestSameEmployee.RunID,

			MatchedEmployeeID: bestSameEmployee.EmployeeID,

			Reason: "The receipt closely matches a previous submission by the same employee.",
		}, nil
	}

	// =====================================================
	// PRIORITY 2
	// CROSS-EMPLOYEE EXACT RISK
	// =====================================================

	if hasCrossEmployeeExact {
		return exact, nil
	}

	// =====================================================
	// PRIORITY 3
	// CROSS-EMPLOYEE METADATA RISK
	// =====================================================

	if bestCrossEmployee != nil {

		return Result{
			IsDuplicate: false,

			RequiresAdminReview: true,

			MatchType: MatchCrossEmployeeMetadata,

			Confidence: bestCrossEmployee.Score,

			StrongIdentity: bestCrossEmployee.StrongIdentity,

			MatchedRunID: bestCrossEmployee.RunID,

			MatchedEmployeeID: bestCrossEmployee.EmployeeID,

			Reason: "The receipt closely matches a submission from a different employee. Finance review is required before any reimbursement can be authorized.",
		}, nil
	}

	return noMatch(), nil
}

// ---------------------------------------------------------
// Matching
// ---------------------------------------------------------

func metadataScore(
	current *models.ReceiptExtraction,
	historical *models.ReceiptExtraction,
) (float64, bool) {

	if current == nil ||
		historical == nil {

		return 0, false
	}

	var score float64

	merchantMatch :=
		merchantEquivalent(
			current.Merchant,
			historical.Merchant,
		)

	amountMatch :=
		moneyEqual(
			current.TotalAmount,
			historical.TotalAmount,
		)

	dateMatch :=
		strings.TrimSpace(
			current.ExpenseDate,
		) != "" &&
			strings.TrimSpace(
				current.ExpenseDate,
			) ==
				strings.TrimSpace(
					historical.ExpenseDate,
				)

	receiptNumberMatch :=
		receiptNumberEquivalent(
			current.ReceiptNumber,
			historical.ReceiptNumber,
		)

	if merchantMatch {
		score += 0.30
	}

	if amountMatch {
		score += 0.30
	}

	if dateMatch {
		score += 0.25
	}

	if receiptNumberMatch {
		score += 0.15
	}

	//
	// A matching non-empty receipt number,
	// transaction date and exact total is
	// especially strong evidence.
	//
	strongIdentity :=
		receiptNumberMatch &&
			dateMatch &&
			amountMatch

	return score,
		strongIdentity
}

func merchantEquivalent(
	left string,
	right string,
) bool {

	leftValue :=
		normalizeMerchant(
			left,
		)

	rightValue :=
		normalizeMerchant(
			right,
		)

	if leftValue == "" ||
		rightValue == "" {

		return false
	}

	//
	// Normal word-level comparison.
	//
	if leftValue ==
		rightValue {

		return true
	}

	//
	// Also compare without spaces.
	//
	// This makes OCR/model variations such as:
	//
	// FourthStreetMill
	// Fourth Street Mill
	//
	// equivalent.
	//
	leftCompact :=
		strings.ReplaceAll(
			leftValue,
			" ",
			"",
		)

	rightCompact :=
		strings.ReplaceAll(
			rightValue,
			" ",
			"",
		)

	if leftCompact ==
		rightCompact {

		return true
	}

	//
	// Avoid fuzzy containment for very short
	// merchant names because it can create
	// false positives.
	//
	if len(leftCompact) < 4 ||
		len(rightCompact) < 4 {

		return false
	}

	//
	// Handles cases such as:
	//
	// THE DAILY GRILL
	//
	// versus:
	//
	// THE DAILY GRILL RESTAURANT & BAR
	//
	// after generic descriptors are removed.
	//
	if strings.Contains(
		leftValue,
		rightValue,
	) ||
		strings.Contains(
			rightValue,
			leftValue,
		) {

		return true
	}

	//
	// Final compact comparison handles
	// punctuation/spacing differences.
	//
	return strings.Contains(
		leftCompact,
		rightCompact,
	) ||
		strings.Contains(
			rightCompact,
			leftCompact,
		)
}

func normalizeMerchant(
	value string,
) string {

	value =
		strings.ToLower(
			strings.TrimSpace(
				value,
			),
		)

	value =
		nonAlphaNumeric.
			ReplaceAllString(
				value,
				" ",
			)

	tokens :=
		strings.Fields(
			value,
		)

	//
	// Generic business descriptors add little
	// identity value and frequently vary between
	// OCR/model runs.
	//
	stopWords :=
		map[string]bool{
			"restaurant":  true,
			"restaurants": true,
			"bar":         true,
			"cafe":        true,
			"café":        true,
			"hotel":       true,
			"hotels":      true,
			"store":       true,
			"stores":      true,
			"shop":        true,
			"shops":       true,
			"pvt":         true,
			"private":     true,
			"ltd":         true,
			"limited":     true,
			"inc":         true,
			"llc":         true,
			"company":     true,
			"co":          true,
		}

	filtered :=
		make(
			[]string,
			0,
			len(tokens),
		)

	for _, token := range tokens {

		if stopWords[token] {
			continue
		}

		filtered =
			append(
				filtered,
				token,
			)
	}

	return strings.Join(
		filtered,
		" ",
	)
}

func receiptNumberEquivalent(
	left string,
	right string,
) bool {

	leftValue :=
		normalizeIdentifier(
			left,
		)

	rightValue :=
		normalizeIdentifier(
			right,
		)

	return leftValue != "" &&
		rightValue != "" &&
		leftValue ==
			rightValue
}

func normalizeIdentifier(
	value string,
) string {

	value =
		strings.ToLower(
			strings.TrimSpace(
				value,
			),
		)

	return nonAlphaNumeric.
		ReplaceAllString(
			value,
			"",
		)
}

func sameEmployee(
	currentEmployeeID string,
	historicalEmployeeID string,
) bool {

	current :=
		strings.TrimSpace(
			currentEmployeeID,
		)

	historical :=
		strings.TrimSpace(
			historicalEmployeeID,
		)

	//
	// Empty current employee ID means a caller
	// is using the legacy API. Preserve the old
	// behavior where all history belongs to the
	// same duplicate scope.
	//
	if current == "" {
		return true
	}

	return strings.EqualFold(
		current,
		historical,
	)
}

func moneyEqual(
	left float64,
	right float64,
) bool {

	return math.Abs(
		left-right,
	) <= 0.01
}

// ---------------------------------------------------------
// Audit loading
// ---------------------------------------------------------

func loadAuditRecords(
	auditDirectory string,
) ([]historicalAuditRecord, error) {

	entries, err :=
		os.ReadDir(
			auditDirectory,
		)

	if os.IsNotExist(err) {
		return []historicalAuditRecord{},
			nil
	}

	if err != nil {
		return nil,
			fmt.Errorf(
				"read audit directory: %w",
				err,
			)
	}

	records :=
		make(
			[]historicalAuditRecord,
			0,
			len(entries),
		)

	for _, entry := range entries {

		if entry.IsDir() {
			continue
		}

		if strings.ToLower(
			filepath.Ext(
				entry.Name(),
			),
		) != ".json" {

			continue
		}

		path :=
			filepath.Join(
				auditDirectory,
				entry.Name(),
			)

		data, err :=
			os.ReadFile(
				path,
			)

		if err != nil {
			return nil,
				fmt.Errorf(
					"read audit file %s: %w",
					path,
					err,
				)
		}

		var record historicalAuditRecord

		if err :=
			json.Unmarshal(
				data,
				&record,
			); err != nil {

			//
			// A malformed historical audit file
			// should not make all future expense
			// submissions unavailable.
			//
			continue
		}

		records =
			append(
				records,
				record,
			)
	}

	return records, nil
}

func noMatch() Result {

	return Result{
		IsDuplicate: false,

		RequiresAdminReview: false,

		MatchType: MatchNone,

		Confidence: 0,

		StrongIdentity: false,

		Reason: "No duplicate match found.",
	}
}
