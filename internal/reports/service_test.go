package reports

import (
	"context"
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

type fakeProcessor struct {
	results []*workflow.Result
	index   int
}

func (f *fakeProcessor) ProcessReceipt(
	ctx context.Context,
	receiptPath string,
	rawDocument []byte,
	employee models.Employee,
) (*workflow.Result, error) {

	result :=
		f.results[f.index]

	f.index++

	result.Employee =
		employee

	result.ReceiptPath =
		receiptPath

	return result, nil
}

type fakeRepository struct {
	report *models.ExpenseReport

	claims map[string]*workflow.Result

	positions map[string]int

	events int
}

func newFakeRepository() *fakeRepository {

	return &fakeRepository{
		claims: map[string]*workflow.Result{},

		positions: map[string]int{},
	}
}

func (f *fakeRepository) CreateExpenseReport(
	ctx context.Context,
	report *models.ExpenseReport,
) error {

	copyValue :=
		*report

	f.report =
		&copyValue

	return nil
}

func (f *fakeRepository) AttachClaimToReport(
	ctx context.Context,
	reportID string,
	claimID string,
	position int,
) error {

	f.positions[claimID] =
		position

	return nil
}

func (f *fakeRepository) UpdateExpenseReportStatus(
	ctx context.Context,
	reportID string,
	status string,
) error {

	f.report.Status =
		status

	return nil
}

func (f *fakeRepository) Create(
	ctx context.Context,
	result *workflow.Result,
) error {

	f.claims[result.RunID] =
		result

	return nil
}

func (f *fakeRepository) AppendEvent(
	ctx context.Context,
	claimID string,
	eventType string,
	payload any,
) error {

	f.events++

	return nil
}

func (f *fakeRepository) GetExpenseReport(
	ctx context.Context,
	reportID string,
) (*models.ExpenseReport, error) {

	copyValue :=
		*f.report

	copyValue.Items =
		[]models.ExpenseReportItem{}

	for claimID, position := range f.positions {

		result :=
			f.claims[claimID]

		item :=
			models.ExpenseReportItem{
				ClaimID: claimID,

				Position: position,

				Status: result.FinalStatus,

				DocumentName: result.ReceiptPath,
			}

		if result.FinalExtraction != nil {

			item.Merchant =
				result.FinalExtraction.Merchant

			item.Category =
				result.FinalExtraction.CategorySuggestion

			item.Currency =
				result.FinalExtraction.Currency

			item.ClaimedAmount =
				result.FinalExtraction.TotalAmount
		}

		if result.PolicyDecision != nil {

			item.ReimbursableAmount =
				result.PolicyDecision.ReimbursableAmount
		}

		copyValue.Items =
			append(
				copyValue.Items,
				item,
			)

		copyValue.TotalClaimed +=
			item.ClaimedAmount

		copyValue.TotalReimbursable +=
			item.ReimbursableAmount
	}

	copyValue.ItemCount =
		len(copyValue.Items)

	return &copyValue, nil
}

func TestCreateMultiReceiptExpenseReport(
	t *testing.T,
) {

	processor :=
		&fakeProcessor{
			results: []*workflow.Result{
				{
					RunID: "claim-1",

					FinalStatus: policy.StatusApproved,

					FinalExtraction: &models.ReceiptExtraction{
						Merchant: "Taxi Co",

						Currency: "USD",

						TotalAmount: 20,

						CategorySuggestion: "Transportation",
					},

					PolicyDecision: &policy.Decision{
						Status: policy.StatusApproved,

						ReimbursableAmount: 20,
					},
				},
				{
					RunID: "claim-2",

					FinalStatus: policy.StatusLimitExceeded,

					FinalExtraction: &models.ReceiptExtraction{
						Merchant: "Restaurant",

						Currency: "USD",

						TotalAmount: 54.20,

						CategorySuggestion: "Meals",
					},

					PolicyDecision: &policy.Decision{
						Status: policy.StatusLimitExceeded,

						ReimbursableAmount: 50,
					},
				},
			},
		}

	repository :=
		newFakeRepository()

	service :=
		NewService(
			processor,
			repository,
		)

	report, err :=
		service.CreateExpenseReport(
			context.Background(),
			CreateRequest{
				Title: "Client Visit",

				Purpose: "Customer meeting",

				Employee: models.Employee{
					ID: "EMP50001",

					Name: "Arjun Rao",

					Grade: "B2",

					Department: "Sales",

					Location: "Bengaluru",
				},

				Files: []UploadedDocument{
					{
						Filename: "taxi.jpg",

						Data: []byte("taxi"),
					},
					{
						Filename: "meal.jpg",

						Data: []byte("meal"),
					},
				},
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if report.Status !=
		models.ReportStatusCompleted {

		t.Fatalf(
			"expected COMPLETED, got %s",
			report.Status,
		)
	}

	if report.ItemCount != 2 {

		t.Fatalf(
			"expected 2 items, got %d",
			report.ItemCount,
		)
	}

	if report.TotalClaimed !=
		74.20 {

		t.Fatalf(
			"expected claimed total 74.20, got %.2f",
			report.TotalClaimed,
		)
	}

	if report.TotalReimbursable !=
		70 {

		t.Fatalf(
			"expected reimbursable total 70, got %.2f",
			report.TotalReimbursable,
		)
	}

	if repository.events !=
		2 {

		t.Fatalf(
			"expected 2 CLAIM_CREATED events, got %d",
			repository.events,
		)
	}
}

func TestReportRequiresActionWhenReceiptNeedsClarification(
	t *testing.T,
) {

	processor :=
		&fakeProcessor{
			results: []*workflow.Result{
				{
					RunID: "claim-1",

					FinalStatus: policy.StatusApproved,
				},
				{
					RunID: "claim-2",

					FinalStatus: workflow.StatusClarificationRequired,
				},
			},
		}

	repository :=
		newFakeRepository()

	service :=
		NewService(
			processor,
			repository,
		)

	report, err :=
		service.CreateExpenseReport(
			context.Background(),
			CreateRequest{
				Title: "Conference Trip",

				Employee: models.Employee{
					ID: "EMP50002",

					Name: "Meera Nair",

					Grade: "B2",
				},

				Files: []UploadedDocument{
					{
						Filename: "hotel.jpg",

						Data: []byte("hotel"),
					},
					{
						Filename: "meal.jpg",

						Data: []byte("meal"),
					},
				},
			},
		)

	if err != nil {
		t.Fatal(err)
	}

	if report.Status !=
		models.ReportStatusActionRequired {

		t.Fatalf(
			"expected ACTION_REQUIRED, got %s",
			report.Status,
		)
	}
}
