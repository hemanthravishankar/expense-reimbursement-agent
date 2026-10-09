package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/clarification"
	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

type Record struct {
	RunID         string `json:"run_id"`
	ReceiptPath   string `json:"receipt_path"`
	ReceiptSHA256 string `json:"receipt_sha256"`

	DocumentMIMEType string `json:"document_mime_type,omitempty"`
	PageCount        int    `json:"page_count,omitempty"`

	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`

	AIProcessingSkipped bool `json:"ai_processing_skipped"`

	PrimaryModel  string `json:"primary_model,omitempty"`
	FallbackModel string `json:"fallback_model,omitempty"`
	FallbackUsed  bool   `json:"fallback_used"`

	InitialExtraction *models.ReceiptExtraction `json:"initial_extraction,omitempty"`

	InitialValidation validation.ReceiptValidationResult `json:"initial_validation"`

	FinalExtraction *models.ReceiptExtraction `json:"final_extraction,omitempty"`

	FinalValidation validation.ReceiptValidationResult `json:"final_validation"`

	Employee models.Employee `json:"employee"`

	DuplicateResult *duplicate.Result `json:"duplicate_result,omitempty"`

	ClarificationResult *clarification.Result `json:"clarification_result,omitempty"`

	PolicyDecision *policy.Decision `json:"policy_decision,omitempty"`

	FinalStatus string `json:"final_status"`
}

func Save(
	directory string,
	record Record,
) (string, error) {

	if err := os.MkdirAll(
		directory,
		0755,
	); err != nil {

		return "", fmt.Errorf(
			"create audit directory: %w",
			err,
		)
	}

	filename :=
		fmt.Sprintf(
			"%s.json",
			record.RunID,
		)

	path :=
		filepath.Join(
			directory,
			filename,
		)

	data, err :=
		json.MarshalIndent(
			record,
			"",
			"  ",
		)

	if err != nil {

		return "", fmt.Errorf(
			"marshal audit record: %w",
			err,
		)
	}

	if err := os.WriteFile(
		path,
		data,
		0644,
	); err != nil {

		return "", fmt.Errorf(
			"write audit record: %w",
			err,
		)
	}

	return path, nil
}
