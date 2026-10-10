package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/reportstatus"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

// UploadedDocument represents one receipt uploaded as part of
// an expense report.
type UploadedDocument struct {
	Filename string

	Data []byte
}

// CreateRequest represents a parent expense report containing
// one or more receipt-level expense claims.
type CreateRequest struct {
	Title string

	Purpose string

	Employee models.Employee

	Files []UploadedDocument
}

// ExpenseProcessor allows the report service to execute the
// existing receipt workflow without depending directly on a
// concrete workflow processor implementation.
type ExpenseProcessor interface {
	ProcessReceipt(
		ctx context.Context,
		receiptPath string,
		rawDocument []byte,
		employee models.Employee,
	) (*workflow.Result, error)
}

// Repository contains the persistence operations needed by the
// expense report service.
type Repository interface {
	// Parent report operations.
	CreateExpenseReport(
		ctx context.Context,
		report *models.ExpenseReport,
	) error

	AttachClaimToReport(
		ctx context.Context,
		reportID string,
		claimID string,
		position int,
	) error

	UpdateExpenseReportStatus(
		ctx context.Context,
		reportID string,
		status string,
	) error

	GetExpenseReport(
		ctx context.Context,
		reportID string,
	) (*models.ExpenseReport, error)

	// Receipt-level claim operations.
	Create(
		ctx context.Context,
		result *workflow.Result,
	) error

	AppendEvent(
		ctx context.Context,
		claimID string,
		eventType string,
		payload any,
	) error
}

// Service coordinates creation of an expense report and all
// receipt-level child claims belonging to that report.
type Service struct {
	processor ExpenseProcessor

	repository Repository
}

// NewService creates an expense report service.
func NewService(
	processor ExpenseProcessor,
	repository Repository,
) *Service {

	return &Service{
		processor: processor,

		repository: repository,
	}
}

// CreateExpenseReport creates a parent expense report and
// processes every uploaded receipt independently.
//
// Each receipt:
//
//   - runs through the existing workflow processor
//   - becomes an individual claim
//   - receives its own business/audit status
//   - is attached to the parent expense report
//
// The parent report status is derived from all child claim
// statuses using the shared reportstatus package.
func (s *Service) CreateExpenseReport(
	ctx context.Context,
	request CreateRequest,
) (*models.ExpenseReport, error) {

	// =====================================================
	// DEPENDENCY VALIDATION
	// =====================================================

	if s.processor == nil {
		return nil,
			fmt.Errorf(
				"expense processor is not configured",
			)
	}

	if s.repository == nil {
		return nil,
			fmt.Errorf(
				"expense report repository is not configured",
			)
	}

	// =====================================================
	// REQUEST VALIDATION
	// =====================================================

	request.Title =
		strings.TrimSpace(
			request.Title,
		)

	request.Purpose =
		strings.TrimSpace(
			request.Purpose,
		)

	request.Employee.ID =
		strings.TrimSpace(
			request.Employee.ID,
		)

	request.Employee.Name =
		strings.TrimSpace(
			request.Employee.Name,
		)

	request.Employee.Grade =
		strings.TrimSpace(
			request.Employee.Grade,
		)

	request.Employee.Department =
		strings.TrimSpace(
			request.Employee.Department,
		)

	request.Employee.Location =
		strings.TrimSpace(
			request.Employee.Location,
		)

	if request.Title == "" {
		return nil,
			fmt.Errorf(
				"expense report title is required",
			)
	}

	if request.Employee.ID == "" ||
		request.Employee.Name == "" ||
		request.Employee.Grade == "" {

		return nil,
			fmt.Errorf(
				"employee ID/name/grade is required",
			)
	}

	if len(request.Files) == 0 {
		return nil,
			fmt.Errorf(
				"at least one receipt is required",
			)
	}

	//
	// Validate all uploaded documents before creating the
	// parent record so we do not create an empty report
	// because of an obviously invalid file.
	//
	for _, file := range request.Files {

		if len(file.Data) == 0 {
			return nil,
				fmt.Errorf(
					"receipt %q is empty",
					file.Filename,
				)
		}
	}

	// =====================================================
	// CREATE PARENT REPORT
	// =====================================================

	now :=
		time.Now().
			UTC()

	report :=
		&models.ExpenseReport{
			ReportID: generateReportID(),

			Title: request.Title,

			Purpose: request.Purpose,

			Employee: request.Employee,

			Status: "PROCESSING",

			ItemCount: len(request.Files),

			TotalClaimed: 0,

			TotalReimbursable: 0,

			Items: []models.ExpenseReportItem{},

			CreatedAt: now,

			UpdatedAt: now,
		}

	if err :=
		s.repository.
			CreateExpenseReport(
				ctx,
				report,
			); err != nil {

		return nil,
			fmt.Errorf(
				"create expense report: %w",
				err,
			)
	}

	// =====================================================
	// PROCESS EACH RECEIPT
	// =====================================================

	results :=
		make(
			[]*workflow.Result,
			0,
			len(request.Files),
		)

	for position, file := range request.Files {

		result, err :=
			s.processor.
				ProcessReceipt(
					ctx,
					file.Filename,
					file.Data,
					request.Employee,
				)

		if err != nil {
			return nil,
				fmt.Errorf(
					"process receipt %q: %w",
					file.Filename,
					err,
				)
		}

		if result == nil {
			return nil,
				fmt.Errorf(
					"process receipt %q returned an empty result",
					file.Filename,
				)
		}

		// =============================================
		// Persist receipt-level claim
		// =============================================

		if err :=
			s.repository.
				Create(
					ctx,
					result,
				); err != nil {

			return nil,
				fmt.Errorf(
					"persist receipt claim %q: %w",
					file.Filename,
					err,
				)
		}

		// =============================================
		// Business event history
		// =============================================

		if err :=
			s.repository.
				AppendEvent(
					ctx,
					result.RunID,
					"CLAIM_CREATED",
					map[string]any{
						"status": result.FinalStatus,

						"employee_id": result.Employee.ID,

						"document_sha256": result.ReceiptSHA256,

						"expense_report_id": report.ReportID,
					},
				); err != nil {

			return nil,
				fmt.Errorf(
					"append claim-created event: %w",
					err,
				)
		}

		// =============================================
		// Attach claim to parent report
		// =============================================

		if err :=
			s.repository.
				AttachClaimToReport(
					ctx,
					report.ReportID,
					result.RunID,
					position+1,
				); err != nil {

			return nil,
				fmt.Errorf(
					"attach claim to expense report: %w",
					err,
				)
		}

		results =
			append(
				results,
				result,
			)
	}

	// =====================================================
	// CALCULATE FINAL PARENT STATUS
	// =====================================================

	finalStatus :=
		determineReportStatus(
			results,
		)

	if err :=
		s.repository.
			UpdateExpenseReportStatus(
				ctx,
				report.ReportID,
				finalStatus,
			); err != nil {

		return nil,
			fmt.Errorf(
				"update final expense report status: %w",
				err,
			)
	}

	// =====================================================
	// RELOAD FULL REPORT
	// =====================================================
	//
	// GetExpenseReport hydrates:
	//
	//   - receipt items
	//   - claimed totals
	//   - reimbursable totals
	//   - latest child statuses
	//
	completedReport,
		err :=
		s.repository.
			GetExpenseReport(
				ctx,
				report.ReportID,
			)

	if err != nil {
		return nil,
			fmt.Errorf(
				"load completed expense report: %w",
				err,
			)
	}

	return completedReport,
		nil
}

// determineReportStatus converts the workflow results into the
// shared parent-report status model.
//
// This is intentionally delegated to reportstatus.Calculate so
// initial report creation and later parent refresh use exactly
// the same business rules.
func determineReportStatus(
	results []*workflow.Result,
) string {

	if len(results) == 0 {
		return reportstatus.Calculate(
			nil,
		)
	}

	items :=
		make(
			[]models.ExpenseReportItem,
			0,
			len(results),
		)

	for position, result := range results {

		if result == nil {

			items =
				append(
					items,
					models.ExpenseReportItem{
						Position: position + 1,

						Status: "PROCESSING",
					},
				)

			continue
		}

		items =
			append(
				items,
				models.ExpenseReportItem{
					ClaimID: result.RunID,

					Position: position + 1,

					Status: result.FinalStatus,
				},
			)
	}

	return reportstatus.Calculate(
		items,
	)
}

// generateReportID creates a readable globally unique-enough ID
// for the demo/application.
//
// Example:
//
// RPT-20261010T124749.241147000Z
func generateReportID() string {

	return "RPT-" +
		time.Now().
			UTC().
			Format(
				"20060102T150405.000000000Z",
			)
}
