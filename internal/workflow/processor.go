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
	StatusHumanReviewRequired = "HUMAN_REVIEW_REQUIRED"

	StatusDuplicateDetected = "DUPLICATE_DETECTED"

	StatusClarificationRequired = "CLARIFICATION_REQUIRED"

	StatusManualReview = "MANUAL_REVIEW"
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

	DocumentDirectory string

	PrimaryModel string

	FallbackModel string

	Policy policy.Policy
}

type Result struct {
	RunID string `json:"run_id"`

	ReceiptPath string `json:"receipt_path"`

	ReceiptSHA256 string `json:"receipt_sha256"`

	DocumentMIMEType string `json:"document_mime_type"`

	PageCount int `json:"page_count"`

	StoredDocument *storage.StoredDocument `json:"stored_document,omitempty"`

	AIProcessingSkipped bool `json:"ai_processing_skipped"`

	Employee models.Employee `json:"employee"`

	InitialExtraction *models.ReceiptExtraction `json:"initial_extraction,omitempty"`

	InitialValidation validation.ReceiptValidationResult `json:"initial_validation"`

	FinalExtraction *models.ReceiptExtraction `json:"final_extraction,omitempty"`

	FinalValidation validation.ReceiptValidationResult `json:"final_validation"`

	FinalModel string `json:"final_model"`

	FallbackUsed bool `json:"fallback_used"`

	DuplicateResult *duplicate.Result `json:"duplicate_result,omitempty"`

	ClarificationResult *clarification.Result `json:"clarification_result,omitempty"`

	ClarificationAnswers []clarification.Answer `json:"clarification_answers,omitempty"`

	PolicyDecision *policy.Decision `json:"policy_decision,omitempty"`

	FinalStatus string `json:"final_status"`

	AuditPath string `json:"audit_path,omitempty"`

	Warnings []string `json:"warnings"`
}

type Processor struct {
	extractor ReceiptExtractor

	documentStore storage.DocumentStore

	config Config
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

	config =
		normalizeConfig(
			config,
		)

	documentStore :=
		storage.NewLocalDocumentStore(
			config.DocumentDirectory,
		)

	return &Processor{
		extractor: extractor,

		documentStore: documentStore,

		config: config,
	}
}

func NewProcessorWithStore(
	extractor ReceiptExtractor,
	documentStore storage.DocumentStore,
	config Config,
) *Processor {

	config =
		normalizeConfig(
			config,
		)

	if documentStore == nil {
		documentStore =
			storage.NewLocalDocumentStore(
				config.DocumentDirectory,
			)
	}

	return &Processor{
		extractor: extractor,

		documentStore: documentStore,

		config: config,
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

	if len(
		config.Policy.Limits,
	) == 0 {

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
		time.Now().
			UTC()

	if len(rawDocument) == 0 {
		return nil,
			fmt.Errorf(
				"receipt document is empty",
			)
	}

	if p.extractor == nil {
		return nil,
			fmt.Errorf(
				"receipt extractor is not configured",
			)
	}

	if p.documentStore == nil {
		return nil,
			fmt.Errorf(
				"document store is not configured",
			)
	}

	//
	// Calculate cheap deterministic properties
	// before doing any AI work.
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
			RunID: generateRunID(),

			ReceiptPath: receiptPath,

			ReceiptSHA256: receiptHash,

			DocumentMIMEType: mimeType,

			Employee: employee,

			FinalStatus: StatusHumanReviewRequired,

			Warnings: []string{},
		}

	// =====================================================
	// STEP 1
	// PRE-AI EXACT DUPLICATE CHECK
	// =====================================================
	//
	// IMPORTANT:
	//
	// Duplicate checks are employee-aware.
	//
	// Same employee:
	//     exact hash -> true duplicate -> stop before AI
	//
	// Different employee:
	//     exact hash -> Admin risk signal
	//     processing continues
	//
	exactDuplicate, err :=
		duplicate.DetectExactHashForEmployee(
			p.config.AuditDirectory,
			receiptHash,
			employee.ID,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"detect exact duplicate: %w",
				err,
			)
	}

	if exactDuplicate.MatchType !=
		duplicate.MatchNone {

		result.DuplicateResult =
			&exactDuplicate
	}

	//
	// Only same-employee exact duplicates
	// are blocked here.
	//
	if exactDuplicate.IsDuplicate {

		result.FinalStatus =
			StatusDuplicateDetected

		result.AIProcessingSkipped =
			true

		if err :=
			p.saveAudit(
				result,
				startedAt,
			); err != nil {

			return nil, err
		}

		return result, nil
	}

	// =====================================================
	// STEP 2
	// STORE ORIGINAL DOCUMENT
	// =====================================================

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
		return nil,
			fmt.Errorf(
				"store original receipt document: %w",
				err,
			)
	}

	result.StoredDocument =
		storedDocument

	// =====================================================
	// STEP 3
	// DOCUMENT NORMALIZATION
	// =====================================================
	//
	// JPG / PNG:
	//     one image page
	//
	// PDF:
	//     render into page images
	//
	doc, err :=
		document.Process(
			ctx,
			receiptPath,
			rawDocument,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"process receipt document: %w",
				err,
			)
	}

	//
	// Defensive consistency check.
	//
	if doc.SHA256 !=
		receiptHash {

		return nil,
			fmt.Errorf(
				"document hash changed during processing",
			)
	}

	result.DocumentMIMEType =
		doc.MIMEType

	result.PageCount =
		doc.PageCount

	// =====================================================
	// STEP 4
	// PRIMARY VISION MODEL
	// =====================================================

	initialExtraction, err :=
		p.extractor.ExtractReceipt(
			ctx,
			doc.Pages,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"primary receipt extraction failed: %w",
				err,
			)
	}

	validation.NormalizeReceipt(
		initialExtraction,
	)

	initialValidation :=
		validation.ValidateReceipt(
			initialExtraction,
		)

	result.InitialExtraction =
		cloneExtraction(
			initialExtraction,
		)

	result.InitialValidation =
		initialValidation

	result.FinalExtraction =
		cloneExtraction(
			initialExtraction,
		)

	result.FinalValidation =
		initialValidation

	result.FinalModel =
		p.config.PrimaryModel

	// =====================================================
	// STEP 5
	// MODEL ESCALATION
	// =====================================================
	//
	// The larger model is used only when
	// deterministic validation rejects the
	// primary extraction.
	//
	if !initialValidation.Valid {

		result.FallbackUsed =
			true

		fallbackExtraction,
			fallbackErr :=
			p.extractor.
				ExtractReceiptWithFallback(
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

			validation.NormalizeReceipt(
				fallbackExtraction,
			)

			fallbackValidation :=
				validation.ValidateReceipt(
					fallbackExtraction,
				)

			result.FinalExtraction =
				cloneExtraction(
					fallbackExtraction,
				)

			result.FinalValidation =
				fallbackValidation

			result.FinalModel =
				p.config.FallbackModel
		}
	}

	// =====================================================
	// STEP 6
	// INVALID EVEN AFTER FALLBACK
	// =====================================================

	if result.FinalExtraction == nil ||
		!result.FinalValidation.Valid {

		result.FinalStatus =
			StatusHumanReviewRequired

		if err :=
			p.saveAudit(
				result,
				startedAt,
			); err != nil {

			return nil, err
		}

		return result, nil
	}

	// =====================================================
	// STEP 7
	// EMPLOYEE-AWARE METADATA DUPLICATE CHECK
	// =====================================================
	//
	// Same employee:
	//
	//     metadata match
	//         -> clarification required
	//
	// Different employee:
	//
	//     exact / metadata match
	//         -> RequiresAdminReview = true
	//         -> MANUAL_REVIEW
	//         -> no automatic reimbursement
	//
	dupResult, err :=
		duplicate.DetectForEmployee(
			p.config.AuditDirectory,
			receiptHash,
			result.FinalExtraction,
			employee.ID,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"detect receipt duplicate: %w",
				err,
			)
	}

	result.DuplicateResult =
		&dupResult

	//
	// Defensive backup:
	//
	// Normally exact same-employee duplicates
	// were already caught before AI.
	//
	if dupResult.IsDuplicate &&
		dupResult.MatchType ==
			duplicate.MatchExact {

		result.FinalStatus =
			StatusDuplicateDetected

		if err :=
			p.saveAudit(
				result,
				startedAt,
			); err != nil {

			return nil, err
		}

		return result, nil
	}

	//
	// Cross-employee receipt reuse is not presented to
	// the employee as "you already submitted this" because
	// the historical claim belongs to another employee.
	//
	// However, RequiresAdminReview is a reimbursement gate,
	// not merely an informational flag. Finance/Admin must
	// review the claim before any policy reimbursement can
	// be authorized.
	//
	if dupResult.RequiresAdminReview {

		result.PolicyDecision =
			nil

		result.FinalStatus =
			StatusManualReview

		if err :=
			p.saveAudit(
				result,
				startedAt,
			); err != nil {

			return nil, err
		}

		return result, nil
	}

	// =====================================================
	// STEP 8
	// CLARIFICATION ENGINE
	// =====================================================
	//
	// Cross-employee risk matches have:
	//
	//     IsDuplicate = false
	//
	// so they will NOT incorrectly trigger
	// employee duplicate clarification.
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

		if err :=
			p.saveAudit(
				result,
				startedAt,
			); err != nil {

			return nil, err
		}

		return result, nil
	}

	// =====================================================
	// STEP 9
	// DETERMINISTIC POLICY ENGINE
	// =====================================================
	//
	// The LLM understands the receipt.
	//
	// Go code decides reimbursement.
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

	// =====================================================
	// STEP 10
	// TECHNICAL AUDIT
	// =====================================================

	if err :=
		p.saveAudit(
			result,
			startedAt,
		); err != nil {

		return nil, err
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

			CompletedAt: time.Now().
				UTC(),

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
	extraction *models.ReceiptExtraction,
) *models.ReceiptExtraction {

	if extraction == nil {
		return nil
	}

	cloned :=
		*extraction

	cloned.LineItems =
		append(
			[]models.ReceiptLineItem(nil),
			extraction.LineItems...,
		)

	cloned.Warnings =
		append(
			[]string(nil),
			extraction.Warnings...,
		)

	return &cloned
}
