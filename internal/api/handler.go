package api

import (
	"context"

	"encoding/json"

	"errors"

	"fmt"

	"io"

	"net/http"

	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/claims"

	"github.com/hemanth-shankar/expense-agent/internal/finance"

	"github.com/hemanth-shankar/expense-agent/internal/models"

	"github.com/hemanth-shankar/expense-agent/internal/persistence"

	"github.com/hemanth-shankar/expense-agent/internal/policy"

	"github.com/hemanth-shankar/expense-agent/internal/reports"

	"github.com/hemanth-shankar/expense-agent/internal/storage"

	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

const maxUploadSize = 20 << 20 // 20 MB

type ExpenseProcessor interface {
	ProcessReceipt(

		ctx context.Context,

		receiptPath string,

		rawDocument []byte,

		employee models.Employee,

	) (*workflow.Result, error)
}

type expenseReportRefresher interface {
	RefreshExpenseReportForClaim(

		ctx context.Context,

		claimID string,

	) error
}

type ClaimRepository interface {
	Create(

		ctx context.Context,

		result *workflow.Result,

	) error

	Get(

		ctx context.Context,

		claimID string,

	) (*workflow.Result, error)

	Update(

		ctx context.Context,

		result *workflow.Result,

	) error

	AppendEvent(

		ctx context.Context,

		claimID string,

		eventType string,

		payload any,

	) error

	GetEvents(

		ctx context.Context,

		claimID string,

	) ([]persistence.ClaimEvent, error)
}

type Handler struct {
	processor ExpenseProcessor

	repository ClaimRepository

	clarificationService *claims.Service

	financeService *finance.Service

	reportRepository ExpenseReportRepository

	reportService *reports.Service

	documentStore storage.DocumentStore
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(

	processor ExpenseProcessor,

) *Handler {

	return &Handler{

		processor: processor,

		documentStore: storage.NewLocalDocumentStore(
			workflow.DefaultConfig().DocumentDirectory,
		),
	}

}

func NewHandlerWithRepository(

	processor ExpenseProcessor,

	repository interface {
		ClaimRepository

		ExpenseReportRepository
	},

) *Handler {

	return &Handler{

		processor: processor,

		repository: repository,

		clarificationService: claims.NewService(

			repository,

			policy.DemoUSDPolicy(),
		),

		financeService: finance.NewService(

			repository,

			policy.DemoUSDPolicy(),
		),

		reportRepository: repository,

		reportService: reports.NewService(

			processor,

			repository,
		),

		documentStore: storage.NewLocalDocumentStore(
			workflow.DefaultConfig().DocumentDirectory,
		),
	}

}

func (h *Handler) RegisterRoutes(

	mux *http.ServeMux,

) {

	mux.HandleFunc(

		"GET /healthz",

		h.health,
	)

	mux.HandleFunc(

		"POST /api/v1/expenses/analyze",

		h.analyzeExpense,
	)

	mux.HandleFunc(

		"GET /api/v1/claims/{claimID}",

		h.getClaim,
	)

	mux.HandleFunc(

		"GET /api/v1/claims/{claimID}/events",

		h.getClaimEvents,
	)

	mux.HandleFunc(

		"GET /api/v1/claims/{claimID}/document",

		h.getClaimDocument,
	)

	mux.HandleFunc(

		"POST /api/v1/claims/{claimID}/clarifications",

		h.resolveClarification,
	)

	mux.HandleFunc(

		"POST /api/v1/expense-reports",

		h.createExpenseReport,
	)

	mux.HandleFunc(

		"GET /api/v1/expense-reports/{reportID}",

		h.getExpenseReport,
	)

	mux.HandleFunc(

		"GET /api/v1/employees/{employeeID}/expense-reports",

		h.listEmployeeExpenseReports,
	)

	mux.HandleFunc(

		"GET /api/v1/admin/expense-reports",

		h.listAdminExpenseReports,
	)

	mux.HandleFunc(

		"POST /api/v1/admin/claims/{claimID}/review",

		h.reviewClaim,
	)

}

func (h *Handler) health(

	w http.ResponseWriter,

	r *http.Request,

) {

	writeJSON(

		w,

		http.StatusOK,

		map[string]string{

			"status": "ok",
		},
	)

}

func (h *Handler) analyzeExpense(

	w http.ResponseWriter,

	r *http.Request,

) {

	r.Body =

		http.MaxBytesReader(

			w,

			r.Body,

			maxUploadSize,
		)

	if err :=

		r.ParseMultipartForm(

			maxUploadSize,
		); err != nil {

		writeError(

			w,

			http.StatusBadRequest,

			"invalid multipart request or upload exceeds 20 MB",
		)

		return

	}

	file, header, err :=

		r.FormFile(

			"receipt",
		)

	if err != nil {

		writeError(

			w,

			http.StatusBadRequest,

			"receipt file is required",
		)

		return

	}

	defer file.Close()

	data, err :=

		io.ReadAll(

			file,
		)

	if err != nil {

		writeError(

			w,

			http.StatusBadRequest,

			"failed to read uploaded receipt",
		)

		return

	}

	if len(data) == 0 {

		writeError(

			w,

			http.StatusBadRequest,

			"uploaded receipt is empty",
		)

		return

	}

	employee :=

		models.Employee{

			ID: strings.TrimSpace(

				r.FormValue(

					"employee_id",
				),
			),

			Name: strings.TrimSpace(

				r.FormValue(

					"employee_name",
				),
			),

			Grade: strings.TrimSpace(

				r.FormValue(

					"employee_grade",
				),
			),

			Department: strings.TrimSpace(

				r.FormValue(

					"department",
				),
			),

			Location: strings.TrimSpace(

				r.FormValue(

					"location",
				),
			),
		}

	if employee.ID == "" {

		writeError(

			w,

			http.StatusBadRequest,

			"employee_id is required",
		)

		return

	}

	if employee.Name == "" {

		writeError(

			w,

			http.StatusBadRequest,

			"employee_name is required",
		)

		return

	}

	if employee.Grade == "" {

		writeError(

			w,

			http.StatusBadRequest,

			"employee_grade is required",
		)

		return

	}

	result, err :=

		h.processor.ProcessReceipt(

			r.Context(),

			header.Filename,

			data,

			employee,
		)

	if err != nil {

		writeError(

			w,

			http.StatusInternalServerError,

			fmt.Sprintf(

				"expense processing failed: %v",

				err,
			),
		)

		return

	}

	//

	// Persist business claim state.

	//

	if h.repository != nil {

		if err :=

			h.repository.Create(

				r.Context(),

				result,
			); err != nil {

			writeError(

				w,

				http.StatusInternalServerError,

				fmt.Sprintf(

					"failed to persist claim: %v",

					err,
				),
			)

			return

		}

		//

		// Initial business lifecycle event.

		//

		if err :=

			h.repository.AppendEvent(

				r.Context(),

				result.RunID,

				"CLAIM_CREATED",

				map[string]any{

					"status": result.FinalStatus,

					"employee_id": result.Employee.ID,

					"document_sha256": result.ReceiptSHA256,
				},
			); err != nil {

			writeError(

				w,

				http.StatusInternalServerError,

				fmt.Sprintf(

					"failed to persist claim event: %v",

					err,
				),
			)

			return

		}

	}

	writeJSON(

		w,

		http.StatusOK,

		result,
	)

}

func (h *Handler) getClaim(

	w http.ResponseWriter,

	r *http.Request,

) {

	if h.repository == nil {

		writeError(

			w,

			http.StatusServiceUnavailable,

			"claim persistence is not configured",
		)

		return

	}

	claimID :=

		strings.TrimSpace(

			r.PathValue(

				"claimID",
			),
		)

	if claimID == "" {

		writeError(

			w,

			http.StatusBadRequest,

			"claim ID is required",
		)

		return

	}

	result, err :=

		h.repository.Get(

			r.Context(),

			claimID,
		)

	if errors.Is(

		err,

		persistence.ErrClaimNotFound,
	) {

		writeError(

			w,

			http.StatusNotFound,

			"claim not found",
		)

		return

	}

	if err != nil {

		writeError(

			w,

			http.StatusInternalServerError,

			fmt.Sprintf(

				"failed to retrieve claim: %v",

				err,
			),
		)

		return

	}

	writeJSON(

		w,

		http.StatusOK,

		result,
	)

}

func (h *Handler) getClaimEvents(

	w http.ResponseWriter,

	r *http.Request,

) {

	if h.repository == nil {

		writeError(

			w,

			http.StatusServiceUnavailable,

			"claim persistence is not configured",
		)

		return

	}

	claimID :=

		strings.TrimSpace(

			r.PathValue(

				"claimID",
			),
		)

	if claimID == "" {

		writeError(

			w,

			http.StatusBadRequest,

			"claim ID is required",
		)

		return

	}

	//

	// Verify the claim exists.

	//

	_, err :=

		h.repository.Get(

			r.Context(),

			claimID,
		)

	if errors.Is(

		err,

		persistence.ErrClaimNotFound,
	) {

		writeError(

			w,

			http.StatusNotFound,

			"claim not found",
		)

		return

	}

	if err != nil {

		writeError(

			w,

			http.StatusInternalServerError,

			fmt.Sprintf(

				"failed to retrieve claim: %v",

				err,
			),
		)

		return

	}

	events, err :=

		h.repository.GetEvents(

			r.Context(),

			claimID,
		)

	if err != nil {

		writeError(

			w,

			http.StatusInternalServerError,

			fmt.Sprintf(

				"failed to retrieve claim events: %v",

				err,
			),
		)

		return

	}

	writeJSON(

		w,

		http.StatusOK,

		events,
	)

}

func (h *Handler) resolveClarification(

	w http.ResponseWriter,

	r *http.Request,

) {

	if h.clarificationService == nil {

		writeError(

			w,

			http.StatusServiceUnavailable,

			"clarification service is not configured",
		)

		return

	}

	claimID :=

		strings.TrimSpace(

			r.PathValue(

				"claimID",
			),
		)

	if claimID == "" {

		writeError(

			w,

			http.StatusBadRequest,

			"claim ID is required",
		)

		return

	}

	var request claims.ClarificationAnswerRequest

	decoder :=

		json.NewDecoder(

			http.MaxBytesReader(

				w,

				r.Body,

				1<<20,
			),
		)

	decoder.DisallowUnknownFields()

	if err :=

		decoder.Decode(

			&request,
		); err != nil {

		writeError(

			w,

			http.StatusBadRequest,

			"invalid clarification request",
		)

		return

	}

	result, err :=

		h.clarificationService.ResolveClarification(

			r.Context(),

			claimID,

			request,
		)

	switch {

	case err == nil:

		// A successful clarification can change the child claim status.
		// If the claim belongs to an expense report, refresh the parent
		// so its status no longer remains stale (for example,
		// ACTION_REQUIRED after the child becomes LIMIT_EXCEEDED).
		if h.reportRepository != nil {
			if refresher, ok := h.reportRepository.(expenseReportRefresher); ok {
				if refreshErr := refresher.RefreshExpenseReportForClaim(
					r.Context(),
					claimID,
				); refreshErr != nil {
					writeError(
						w,
						http.StatusInternalServerError,
						fmt.Sprintf(
							"clarification was resolved, but failed to refresh parent expense report: %v",
							refreshErr,
						),
					)
					return
				}
			}
		}

		writeJSON(
			w,
			http.StatusOK,
			result,
		)

	case errors.Is(

		err,

		persistence.ErrClaimNotFound,
	):

		writeError(

			w,

			http.StatusNotFound,

			"claim not found",
		)

	case errors.Is(

		err,

		claims.ErrNoPendingClarification,
	):

		writeError(

			w,

			http.StatusConflict,

			err.Error(),
		)

	case errors.Is(

		err,

		claims.ErrQuestionNotFound,
	):

		writeError(

			w,

			http.StatusBadRequest,

			err.Error(),
		)

	case errors.Is(

		err,

		claims.ErrInvalidAnswer,
	):

		writeError(

			w,

			http.StatusBadRequest,

			err.Error(),
		)

	case errors.Is(

		err,

		claims.ErrExplanationRequired,
	):

		writeError(

			w,

			http.StatusBadRequest,

			err.Error(),
		)

	case errors.Is(

		err,

		claims.ErrUnsupportedClarification,
	):

		writeError(

			w,

			http.StatusBadRequest,

			err.Error(),
		)

	default:

		writeError(

			w,

			http.StatusInternalServerError,

			fmt.Sprintf(

				"failed to resolve clarification: %v",

				err,
			),
		)

	}

}

func writeError(

	w http.ResponseWriter,

	status int,

	message string,

) {

	writeJSON(

		w,

		status,

		errorResponse{

			Error: message,
		},
	)

}

func writeJSON(

	w http.ResponseWriter,

	status int,

	value any,

) {

	w.Header().Set(

		"Content-Type",

		"application/json",
	)

	w.WriteHeader(

		status,
	)

	_ =

		json.NewEncoder(

			w,
		).
			Encode(

				value,
			)

}

func WithCORS(

	next http.Handler,

) http.Handler {

	return http.HandlerFunc(

		func(

			w http.ResponseWriter,

			r *http.Request,

		) {

			w.Header().Set(

				"Access-Control-Allow-Origin",

				"http://localhost:5173",
			)

			w.Header().Set(

				"Access-Control-Allow-Methods",

				"GET, POST, OPTIONS",
			)

			w.Header().Set(

				"Access-Control-Allow-Headers",

				"Content-Type",
			)

			if r.Method ==

				http.MethodOptions {

				w.WriteHeader(

					http.StatusNoContent,
				)

				return

			}

			next.ServeHTTP(

				w,

				r,
			)

		},
	)

}
