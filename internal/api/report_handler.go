package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/models"
	"github.com/hemanth-shankar/expense-agent/internal/persistence"
	"github.com/hemanth-shankar/expense-agent/internal/reports"
)

const (
	maxReportUploadSize = 100 << 20 // 100 MB total request
	maxReceiptCount     = 10
)

type ExpenseReportRepository interface {
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

	ListEmployeeExpenseReports(
		ctx context.Context,
		employeeID string,
	) ([]models.ExpenseReport, error)

	ListExpenseReports(
		ctx context.Context,
	) ([]models.ExpenseReport, error)
}

func (h *Handler) createExpenseReport(
	w http.ResponseWriter,
	r *http.Request,
) {

	if h.reportService == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"expense report service is not configured",
		)

		return
	}

	//
	// Protect the API from excessively large
	// multipart requests.
	//
	r.Body =
		http.MaxBytesReader(
			w,
			r.Body,
			maxReportUploadSize,
		)

	if err :=
		r.ParseMultipartForm(
			32 << 20,
		); err != nil {

		writeError(
			w,
			http.StatusBadRequest,
			"invalid multipart request or upload exceeds maximum size",
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

	title :=
		strings.TrimSpace(
			r.FormValue(
				"title",
			),
		)

	purpose :=
		strings.TrimSpace(
			r.FormValue(
				"purpose",
			),
		)

	//
	// Basic employee validation.
	//
	if employee.ID == "" ||
		employee.Name == "" ||
		employee.Grade == "" {

		writeError(
			w,
			http.StatusBadRequest,
			"employee_id, employee_name and employee_grade are required",
		)

		return
	}

	if title == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"expense report title is required",
		)

		return
	}

	if r.MultipartForm == nil {
		writeError(
			w,
			http.StatusBadRequest,
			"multipart form is missing",
		)

		return
	}

	//
	// Preferred field:
	//
	// receipts
	//
	// Multiple files can use the same field name.
	//
	fileHeaders :=
		r.MultipartForm.File["receipts"]

	//
	// Also accept the old singular field name
	// for convenience and backward compatibility.
	//
	if len(fileHeaders) == 0 {
		fileHeaders =
			r.MultipartForm.File["receipt"]
	}

	if len(fileHeaders) == 0 {
		writeError(
			w,
			http.StatusBadRequest,
			"at least one receipt is required",
		)

		return
	}

	if len(fileHeaders) >
		maxReceiptCount {

		writeError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf(
				"maximum %d receipts are allowed per expense report",
				maxReceiptCount,
			),
		)

		return
	}

	files :=
		make(
			[]reports.UploadedDocument,
			0,
			len(fileHeaders),
		)

	//
	// Read every receipt independently.
	//
	for _, header := range fileHeaders {

		file, err :=
			header.Open()

		if err != nil {
			writeError(
				w,
				http.StatusBadRequest,
				fmt.Sprintf(
					"failed to open %s",
					header.Filename,
				),
			)

			return
		}

		//
		// Limit each individual receipt to
		// maxUploadSize + 1 so we can detect
		// oversized files without loading
		// arbitrary amounts into memory.
		//
		data, readErr :=
			io.ReadAll(
				io.LimitReader(
					file,
					int64(maxUploadSize+1),
				),
			)

		closeErr :=
			file.Close()

		if readErr != nil {
			writeError(
				w,
				http.StatusBadRequest,
				fmt.Sprintf(
					"failed to read %s",
					header.Filename,
				),
			)

			return
		}

		if closeErr != nil {
			writeError(
				w,
				http.StatusInternalServerError,
				fmt.Sprintf(
					"failed to close uploaded file %s",
					header.Filename,
				),
			)

			return
		}

		if len(data) >
			maxUploadSize {

			writeError(
				w,
				http.StatusBadRequest,
				fmt.Sprintf(
					"%s exceeds the 20 MB per-receipt limit",
					header.Filename,
				),
			)

			return
		}

		if len(data) == 0 {
			writeError(
				w,
				http.StatusBadRequest,
				fmt.Sprintf(
					"%s is empty",
					header.Filename,
				),
			)

			return
		}

		files =
			append(
				files,
				reports.UploadedDocument{
					Filename: header.Filename,

					Data: data,
				},
			)
	}

	//
	// Pass all documents to the report service.
	//
	// Each document will still be processed
	// independently using our existing receipt
	// workflow.
	//
	report, err :=
		h.reportService.CreateExpenseReport(
			r.Context(),
			reports.CreateRequest{
				Title: title,

				Purpose: purpose,

				Employee: employee,

				Files: files,
			},
		)

	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf(
				"failed to create expense report: %v",
				err,
			),
		)

		return
	}

	writeJSON(
		w,
		http.StatusCreated,
		report,
	)
}

func (h *Handler) getExpenseReport(
	w http.ResponseWriter,
	r *http.Request,
) {

	if h.reportRepository == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"expense report persistence is not configured",
		)

		return
	}

	reportID :=
		strings.TrimSpace(
			r.PathValue(
				"reportID",
			),
		)

	if reportID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"expense report ID is required",
		)

		return
	}

	report, err :=
		h.reportRepository.GetExpenseReport(
			r.Context(),
			reportID,
		)

	if errors.Is(
		err,
		persistence.ErrReportNotFound,
	) {
		writeError(
			w,
			http.StatusNotFound,
			"expense report not found",
		)

		return
	}

	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf(
				"failed to retrieve expense report: %v",
				err,
			),
		)

		return
	}

	writeJSON(
		w,
		http.StatusOK,
		report,
	)
}

func (h *Handler) listEmployeeExpenseReports(
	w http.ResponseWriter,
	r *http.Request,
) {

	if h.reportRepository == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"expense report persistence is not configured",
		)

		return
	}

	employeeID :=
		strings.TrimSpace(
			r.PathValue(
				"employeeID",
			),
		)

	if employeeID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"employee ID is required",
		)

		return
	}

	reportList, err :=
		h.reportRepository.ListEmployeeExpenseReports(
			r.Context(),
			employeeID,
		)

	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf(
				"failed to list employee expense reports: %v",
				err,
			),
		)

		return
	}

	writeJSON(
		w,
		http.StatusOK,
		reportList,
	)
}

func (h *Handler) listAdminExpenseReports(
	w http.ResponseWriter,
	r *http.Request,
) {

	if h.reportRepository == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"expense report persistence is not configured",
		)

		return
	}

	reportList, err :=
		h.reportRepository.ListExpenseReports(
			r.Context(),
		)

	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf(
				"failed to list expense reports: %v",
				err,
			),
		)

		return
	}

	writeJSON(
		w,
		http.StatusOK,
		reportList,
	)
}
