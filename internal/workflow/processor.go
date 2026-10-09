package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/audit"
	"github.com/hemanth-shankar/expense-agent/internal/clarification"
	"github.com/hemanth-shankar/expense-agent/internal/document"
	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/storage"
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
	AuditDirectory    string
	DocumentDirectory string

	PrimaryModel  string
	FallbackModel string

	Policy policy.Policy
}

type Processor struct {
	extractor     ReceiptExtractor
	documentStore storage.DocumentStore
	config        Config
}

type Result struct {
	RunID         string `json:"run_id"`
	ReceiptPath   string `json:"receipt_path"`
	ReceiptSHA256 string `json:"receipt_sha256"`

	DocumentMIMEType string `json:"document_mime_type"`
	PageCount        int    `json:"page_count"`

	StoredDocument *storage.StoredDocument `json:"stored_document,omitempty"`

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

		DocumentDirectory: "data/documents",

		PrimaryModel: "qwen3-vl:4b-instruct",

		FallbackModel: "qwen3-vl:8b-instruct",

		Policy: policy.DemoUSDPolicy(),
	}
}

func NewProcessor(
	extractor ReceiptExtractor,
	config Config,
) *Processor {

	config = normalizeConfig(config)

	documentStore :=
		storage.NewLocalDocumentStore(
			config.DocumentDirectory,
		)

	return &Processor{
		extractor:     extractor,
		documentStore: documentStore,
		config:        config,
	}
}

func NewProcessorWithStore(
	extractor ReceiptExtractor,
	documentStore storage.DocumentStore,
	config Config,
) *Processor {

	config = normalizeConfig(config)

	if documentStore == nil {
		documentStore =
			storage.NewLocalDocumentStore(
				config.DocumentDirectory,
			)
	}

	return &Processor{
		extractor:     extractor,
		documentStore: documentStore,
		config:        config,
	}
}

func normalizeConfig(
	config Config,
) Config {

	defaults :=
		DefaultConfig()

	if config.AuditDirectory == "" {
		config.AuditDirectory =
			defaults.AuditDirectory
	}

	if config.DocumentDirectory == "" {
		config.DocumentDirectory =
			defaults.DocumentDirectory
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

	return config
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
	// Validate raw upload.
	//
	if len(rawDocument) == 0 {
		return nil, fmt.Errorf(
			"uploaded document is empty",
		)
	}

	//
	// Perform only cheap deterministic
	// operations at this stage.
	//
	receiptHash :=
		document.CalculateSHA256(
			rawDocument,
		)

	mimeType :=
		document.DetectMIMEType(
			rawDocument,
		)

	result :=
		&Result{
			RunID: runID,

			ReceiptPath: receiptPath,

			ReceiptSHA256: receiptHash,

			DocumentMIMEType: mimeType,

			Employee: employee,

			FinalStatus: StatusHumanReviewRequired,

			Warnings: []string{},
		}

	//
	// STEP 2
	// Exact duplicate detection BEFORE:
	//
	// - storing another physical object
	// - rendering PDFs
	// - running Qwen
	//
	exactDuplicate, err :=
		duplicate.DetectExactHash(
			p.config.AuditDirectory,
			receiptHash,
		)

	if err != nil {

		//
		// Duplicate lookup failure should not
		// prevent a legitimate expense from
		// being processed.
		//
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

		//
		// The duplicate submission attempt
		// itself must still be audited.
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

	//
	// STEP 3
	// Store ORIGINAL evidence.
	//
	// This happens before any AI processing.
	//
	storedDocument, err :=
		p.documentStore.Put(
			ctx,
			filepath.Base(
				receiptPath,
			),
			mimeType,
			receiptHash,
			rawDocument,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"store original expense document: %w",
			err,
		)
	}

	result.StoredDocument =
		storedDocument

	//
	// STEP 4
	// Document normalization.
	//
	// JPG/PNG:
	//   one image page
	//
	// PDF:
	//   rendered into one or more PNG pages
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

	result.DocumentMIMEType =
		doc.MIMEType

	result.PageCount =
		doc.PageCount

	//
	// Defensive consistency check.
	//
	if doc.SHA256 != receiptHash {
		return nil, fmt.Errorf(
			"document hash changed during processing",
		)
	}

	//
	// STEP 5
	// Primary AI extraction using
	// Qwen3-VL 4B.
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

	//
	// STEP 6
	// Deterministic normalization and
	// validation.
	//
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
	// STEP 7
	// Escalate to larger model when
	// deterministic validation fails.
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
	// STEP 8
	// Continue business processing only
	// when extraction passes deterministic
	// validation.
	//
	if result.FinalValidation.Valid {

		//
		// Metadata duplicate detection catches
		// documents that are logically identical
		// but have different file hashes.
		//
		// Examples:
		//
		// - resized images
		// - recompressed images
		// - screenshots
		// - JPG converted to PDF
		//
		dupResult, dupErr :=
			duplicate.Detect(
				p.config.AuditDirectory,
				receiptHash,
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
				//
				// Normally an exact duplicate
				// should already have been caught
				// before AI.
				//
				result.FinalStatus =
					StatusDuplicateDetected

			} else {

				//
				// STEP 9
				// Clarification engine.
				//
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

					//
					// STEP 10
					// Deterministic policy
					// evaluation.
					//
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
	// Invalid extraction even after
	// fallback goes to human review.
	//
	if !result.FinalValidation.Valid {

		result.FinalStatus =
			StatusHumanReviewRequired
	}

	//
	// STEP 11
	// Audit everything.
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

			StoredDocument: result.StoredDocument,

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
