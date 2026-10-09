package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/models"
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

type Handler struct {
	processor ExpenseProcessor
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(
	processor ExpenseProcessor,
) *Handler {

	return &Handler{
		processor: processor,
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
		io.ReadAll(file)

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

	writeJSON(
		w,
		http.StatusOK,
		result,
	)
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

	if err :=
		json.NewEncoder(w).
			Encode(value); err != nil {

		http.Error(
			w,
			`{"error":"failed to encode response"}`,
			http.StatusInternalServerError,
		)
	}
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
