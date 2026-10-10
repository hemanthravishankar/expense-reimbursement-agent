package reports

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

type UploadedDocument struct {
	Filename string
	Data     []byte
}

type CreateRequest struct {
	Title    string
	Purpose  string
	Employee models.Employee
	Files    []UploadedDocument
}

type ExpenseProcessor interface {
	ProcessReceipt(
		ctx context.Context,
		receiptPath string,
		rawDocument []byte,
		employee models.Employee,
	) (*workflow.Result, error)
}

type Repository interface {
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

type Service struct {
	processor  ExpenseProcessor
	repository Repository
}

func NewService(
	processor ExpenseProcessor,
	repository Repository,
) *Service {

	return &Service{
		processor:  processor,
		repository: repository,
	}
}

func (s *Service) CreateExpenseReport(
	ctx context.Context,
	request CreateRequest,
) (*models.ExpenseReport, error) {

	if s.processor == nil {
		return nil, fmt.Errorf(
			"expense processor is not configured",
		)
	}

	if s.repository == nil {
		return nil, fmt.Errorf(
			"expense report repository is not configured",
		)
	}

	title :=
		strings.TrimSpace(
			request.Title,
		)

	if title == "" {
		return nil, fmt.Errorf(
			"expense report title is required",
		)
	}

	if request.Employee.ID == "" {
		return nil, fmt.Errorf(
			"employee ID is required",
		)
	}

	if request.Employee.Name == "" {
		return nil, fmt.Errorf(
			"employee name is required",
		)
	}

	if request.Employee.Grade == "" {
		return nil, fmt.Errorf(
			"employee grade is required",
		)
	}

	if len(request.Files) == 0 {
		return nil, fmt.Errorf(
			"at least one receipt is required",
		)
	}

	reportID :=
		generateReportID()

	report :=
		&models.ExpenseReport{
			ReportID: reportID,

			Title: title,

			Purpose: strings.TrimSpace(
				request.Purpose,
			),

			Employee: request.Employee,

			Status: models.ReportStatusProcessing,
		}

	//
	// STEP 1
	// Create parent expense report.
	//
	if err :=
		s.repository.CreateExpenseReport(
			ctx,
			report,
		); err != nil {

		return nil, fmt.Errorf(
			"create expense report: %w",
			err,
		)
	}

	results :=
		make(
			[]*workflow.Result,
			0,
			len(request.Files),
		)

	//
	// STEP 2
	// Process every receipt independently
	// using the existing production workflow.
	//
	for index, file := range request.Files {

		if len(file.Data) == 0 {

			_ =
				s.repository.UpdateExpenseReportStatus(
					ctx,
					reportID,
					models.ReportStatusReviewRequired,
				)

			return nil, fmt.Errorf(
				"receipt %q is empty",
				file.Filename,
			)
		}

		result, err :=
			s.processor.ProcessReceipt(
				ctx,
				file.Filename,
				file.Data,
				request.Employee,
			)

		if err != nil {

			_ =
				s.repository.UpdateExpenseReportStatus(
					ctx,
					reportID,
					models.ReportStatusReviewRequired,
				)

			return nil, fmt.Errorf(
				"process receipt %q: %w",
				file.Filename,
				err,
			)
		}

		//
		// STEP 3
		// Persist the receipt-level claim.
		//
		if err :=
			s.repository.Create(
				ctx,
				result,
			); err != nil {

			_ =
				s.repository.UpdateExpenseReportStatus(
					ctx,
					reportID,
					models.ReportStatusReviewRequired,
				)

			return nil, fmt.Errorf(
				"persist receipt claim %q: %w",
				file.Filename,
				err,
			)
		}

		//
		// Record standard claim lifecycle event.
		//
		if err :=
			s.repository.AppendEvent(
				ctx,
				result.RunID,
				"CLAIM_CREATED",
				map[string]any{
					"status": result.FinalStatus,

					"employee_id": result.Employee.ID,

					"document_sha256": result.ReceiptSHA256,

					"expense_report_id": reportID,
				},
			); err != nil {

			return nil, fmt.Errorf(
				"append claim-created event: %w",
				err,
			)
		}

		//
		// STEP 4
		// Attach the receipt claim to
		// the parent report.
		//
		if err :=
			s.repository.AttachClaimToReport(
				ctx,
				reportID,
				result.RunID,
				index+1,
			); err != nil {

			return nil, fmt.Errorf(
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

	//
	// STEP 5
	// Calculate overall report state.
	//
	reportStatus :=
		determineReportStatus(
			results,
		)

	if err :=
		s.repository.UpdateExpenseReportStatus(
			ctx,
			reportID,
			reportStatus,
		); err != nil {

		return nil, fmt.Errorf(
			"update final expense report status: %w",
			err,
		)
	}

	//
	// STEP 6
	// Reload report so totals and item details
	// are aggregated from the latest claims.
	//
	finalReport, err :=
		s.repository.GetExpenseReport(
			ctx,
			reportID,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"load completed expense report: %w",
			err,
		)
	}

	return finalReport, nil
}

func determineReportStatus(
	results []*workflow.Result,
) string {

	if len(results) == 0 {
		return models.ReportStatusReviewRequired
	}

	hasActionRequired :=
		false

	hasReviewRequired :=
		false

	hasRejected :=
		false

	hasAccepted :=
		false

	for _, result := range results {

		if result == nil {
			hasReviewRequired =
				true

			continue
		}

		switch result.FinalStatus {

		case workflow.StatusClarificationRequired:

			hasActionRequired =
				true

		case workflow.StatusHumanReviewRequired,
			workflow.StatusDuplicateDetected,
			policy.StatusManualReview,
			policy.StatusCurrencyMismatch:

			hasReviewRequired =
				true

		case policy.StatusNotAllowed:

			hasRejected =
				true

		case policy.StatusApproved,
			policy.StatusLimitExceeded:

			hasAccepted =
				true

		default:

			hasReviewRequired =
				true
		}
	}

	//
	// Employee action takes priority because
	// the report cannot progress until the
	// employee responds.
	//
	if hasActionRequired {
		return models.ReportStatusActionRequired
	}

	if hasReviewRequired {
		return models.ReportStatusReviewRequired
	}

	if hasRejected &&
		hasAccepted {

		return models.ReportStatusPartiallyRejected
	}

	if hasRejected {
		return models.ReportStatusRejected
	}

	return models.ReportStatusCompleted
}

func generateReportID() string {

	return "RPT-" +
		time.Now().
			UTC().
			Format(
				"20060102T150405.000000000Z",
			)
}
