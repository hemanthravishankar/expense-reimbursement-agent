package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/audit"
	"github.com/hemanth-shankar/expense-agent/internal/clarification"
	"github.com/hemanth-shankar/expense-agent/internal/document"
	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/validation"
)

const (
	StatusHumanReviewRequired   = "HUMAN_REVIEW_REQUIRED"
	StatusDuplicateDetected     = "DUPLICATE_DETECTED"
	StatusClarificationRequired = "CLARIFICATION_REQUIRED"
)

type ReceiptExtractor interface {
	ExtractReceipt(
		ctx context.Context,
		pages [][]byte,
	) (*models.ReceiptExtraction, error)

	ExtractReceiptWithFallback(
		ctx context.Context,
		pages [][]byte,
	) (*models.ReceiptExtraction, error)
}

type Config struct {
	AuditDirectory string
	PrimaryModel   string
	FallbackModel  string
	Policy         policy.Policy
}

type Processor struct {
	extractor ReceiptExtractor
	config    Config
}

type Result struct {
	RunID         string `json:"run_id"`
	ReceiptPath   string `json:"receipt_path"`
	ReceiptSHA256 string `json:"receipt_sha256"`

	DocumentMIMEType string `json:"document_mime_type"`
	PageCount        int    `json:"page_count"`

	AIProcessingSkipped bool `json:"ai_processing_skipped"`

	Employee models.Employee `json:"employee"`

	InitialExtraction *models.ReceiptExtraction `json:"initial_extraction,omitempty"`

	InitialValidation validation.ReceiptValidationResult `json:"initial_validation"`

	FinalExtraction *models.ReceiptExtraction `json:"final_extraction,omitempty"`

	FinalValidation validation.ReceiptValidationResult `json:"final_validation"`

	FinalModel string `json:"final_model,omitempty"`

	FallbackUsed bool `json:"fallback_used"`

	DuplicateResult *duplicate.Result `json:"duplicate_result,omitempty"`

	ClarificationResult *clarification.Result `json:"clarification_result,omitempty"`

	PolicyDecision *policy.Decision `json:"policy_decision,omitempty"`

	FinalStatus string `json:"final_status"`

	AuditPath string `json:"audit_path"`

	Warnings []string `json:"warnings,omitempty"`
}

func DefaultConfig() Config {
	return Config{
		AuditDirectory: "data/audit",
		PrimaryModel:   "qwen3-vl:4b-instruct",
		FallbackModel:  "qwen3-vl:8b-instruct",
		Policy:         policy.DemoUSDPolicy(),
	}
}

func NewProcessor(
	extractor ReceiptExtractor,
	config Config,
) *Processor {

	defaults :=
		DefaultConfig()

	if config.AuditDirectory == "" {
		config.AuditDirectory =
			defaults.AuditDirectory
	}

	if config.PrimaryModel == "" {
		config.PrimaryModel =
			defaults.PrimaryModel
	}

	if config.FallbackModel == "" {
		config.FallbackModel =
			defaults.FallbackModel
	}

	if len(config.Policy.Limits) == 0 {
		config.Policy =
			defaults.Policy
	}

	return &Processor{
		extractor: extractor,
		config:    config,
	}
}

func (p *Processor) ProcessReceipt(
	ctx context.Context,
	receiptPath string,
	rawDocument []byte,
	employee models.Employee,
) (*Result, error) {

	startedAt :=
		time.Now().UTC()

	runID :=
		generateRunID()

	//
	// STEP 1
	// Document ingestion.
	//
	doc, err :=
		document.Process(
			ctx,
			receiptPath,
			rawDocument,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"process uploaded document: %w",
			err,
		)
	}

	result :=
		&Result{
			RunID: runID,

			ReceiptPath: receiptPath,

			ReceiptSHA256: doc.SHA256,

			DocumentMIMEType: doc.MIMEType,

			PageCount: doc.PageCount,

			Employee: employee,

			FinalStatus: StatusHumanReviewRequired,

			Warnings: []string{},
		}

	//
	// STEP 2
	// CHEAP exact duplicate check BEFORE AI.
	//
	exactDuplicate, err :=
		duplicate.DetectExactHash(
			p.config.AuditDirectory,
			doc.SHA256,
		)

	if err != nil {

		result.Warnings =
			append(
				result.Warnings,
				fmt.Sprintf(
					"exact duplicate pre-check failed: %v",
					err,
				),
			)

	} else if exactDuplicate.IsDuplicate {

		result.DuplicateResult =
			&exactDuplicate

		result.FinalStatus =
			StatusDuplicateDetected

		result.AIProcessingSkipped =
			true

		if err :=
			p.saveAudit(
				result,
				startedAt,
			); err != nil {

			return result, err
		}

		return result, nil
	}

	//
	// STEP 3
	// Primary AI extraction.
	//
	extraction, err :=
		p.extractor.ExtractReceipt(
			ctx,
			doc.Pages,
		)

	if err != nil {

		return nil, fmt.Errorf(
			"primary receipt extraction failed: %w",
			err,
		)
	}

	validation.NormalizeReceipt(
		extraction,
	)

	result.InitialExtraction =
		cloneExtraction(
			extraction,
		)

	result.InitialValidation =
		validation.ValidateReceipt(
			extraction,
		)

	result.FinalExtraction =
		cloneExtraction(
			extraction,
		)

	result.FinalValidation =
		result.InitialValidation

	result.FinalModel =
		p.config.PrimaryModel

	//
	// STEP 4
	// Fallback model.
	//
	if !result.FinalValidation.Valid {

		fallbackExtraction, fallbackErr :=
			p.extractor.ExtractReceiptWithFallback(
				ctx,
				doc.Pages,
			)

		if fallbackErr != nil {

			result.Warnings =
				append(
					result.Warnings,
					fmt.Sprintf(
						"fallback extraction failed: %v",
						fallbackErr,
					),
				)

		} else {

			result.FallbackUsed =
				true

			validation.NormalizeReceipt(
				fallbackExtraction,
			)

			result.FinalExtraction =
				cloneExtraction(
					fallbackExtraction,
				)

			result.FinalValidation =
				validation.ValidateReceipt(
					fallbackExtraction,
				)

			result.FinalModel =
				p.config.FallbackModel
		}
	}

	//
	// STEP 5
	// Business processing only when
	// extraction passed validation.
	//
	if result.FinalValidation.Valid {

		//
		// This remains useful for detecting:
		// - resized images
		// - recompressed images
		// - converted image formats
		// - screenshots of the same receipt
		//
		dupResult, dupErr :=
			duplicate.Detect(
				p.config.AuditDirectory,
				doc.SHA256,
				result.FinalExtraction,
			)

		if dupErr != nil {

			result.Warnings =
				append(
					result.Warnings,
					fmt.Sprintf(
						"metadata duplicate detection failed: %v",
						dupErr,
					),
				)

			result.FinalStatus =
				StatusHumanReviewRequired

		} else {

			result.DuplicateResult =
				&dupResult

			if dupResult.IsDuplicate &&
				dupResult.MatchType ==
					duplicate.MatchExact {

				//
				// Defensive backup.
				// Normally exact duplicates will already
				// have been caught before AI.
				//
				result.FinalStatus =
					StatusDuplicateDetected

			} else {

				clarify :=
					clarification.Evaluate(
						result.FinalExtraction,
						result.FinalValidation,
						result.DuplicateResult,
					)

				result.ClarificationResult =
					&clarify

				if clarify.Required {

					result.FinalStatus =
						StatusClarificationRequired

				} else {

					decision :=
						policy.Evaluate(
							result.FinalExtraction,
							employee,
							p.config.Policy,
						)

					result.PolicyDecision =
						&decision

					result.FinalStatus =
						decision.Status
				}
			}
		}
	}

	//
	// STEP 6
	// Audit.
	//
	if err :=
		p.saveAudit(
			result,
			startedAt,
		); err != nil {

		return result, err
	}

	return result, nil
}

func (p *Processor) saveAudit(
	result *Result,
	startedAt time.Time,
) error {

	record :=
		audit.Record{
			RunID: result.RunID,

			ReceiptPath: result.ReceiptPath,

			ReceiptSHA256: result.ReceiptSHA256,

			DocumentMIMEType: result.DocumentMIMEType,

			PageCount: result.PageCount,

			StartedAt: startedAt,

			CompletedAt: time.Now().UTC(),

			AIProcessingSkipped: result.AIProcessingSkipped,

			PrimaryModel: p.config.PrimaryModel,

			FallbackUsed: result.FallbackUsed,

			InitialExtraction: result.InitialExtraction,

			InitialValidation: result.InitialValidation,

			FinalExtraction: result.FinalExtraction,

			FinalValidation: result.FinalValidation,

			Employee: result.Employee,

			DuplicateResult: result.DuplicateResult,

			ClarificationResult: result.ClarificationResult,

			PolicyDecision: result.PolicyDecision,

			FinalStatus: result.FinalStatus,
		}

	if result.FallbackUsed {

		record.FallbackModel =
			p.config.FallbackModel
	}

	path, err :=
		audit.Save(
			p.config.AuditDirectory,
			record,
		)

	if err != nil {

		return fmt.Errorf(
			"save workflow audit record: %w",
			err,
		)
	}

	result.AuditPath =
		path

	return nil
}

func generateRunID() string {

	return time.Now().
		UTC().
		Format(
			"20060102T150405.000000000Z",
		)
}

func cloneExtraction(
	source *models.ReceiptExtraction,
) *models.ReceiptExtraction {

	if source == nil {
		return nil
	}

	copyValue :=
		*source

	copyValue.LineItems =
		append(
			[]models.ReceiptLineItem(nil),
			source.LineItems...,
		)

	copyValue.Warnings =
		append(
			[]string(nil),
			source.Warnings...,
		)

	return &copyValue
}
