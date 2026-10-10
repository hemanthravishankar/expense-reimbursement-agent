package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/finance"
	"github.com/hemanth-shankar/expense-agent/internal/persistence"
)

func (h *Handler) reviewClaim(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.financeService == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"finance review service is not configured",
		)
		return
	}

	claimID := strings.TrimSpace(
		r.PathValue("claimID"),
	)
	if claimID == "" {
		writeError(
			w,
			http.StatusBadRequest,
			"claim ID is required",
		)
		return
	}

	var request finance.ReviewRequest
	decoder := json.NewDecoder(
		http.MaxBytesReader(
			w,
			r.Body,
			1<<20,
		),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid finance review request",
		)
		return
	}

	result, err := h.financeService.ReviewClaim(
		r.Context(),
		claimID,
		request,
	)

	switch {
	case err == nil:
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
							"finance review was saved, but failed to refresh parent expense report: %v",
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

	case errors.Is(err, persistence.ErrClaimNotFound):
		writeError(
			w,
			http.StatusNotFound,
			"claim not found",
		)

	case errors.Is(err, finance.ErrClaimNotReviewable):
		writeError(
			w,
			http.StatusConflict,
			err.Error(),
		)

	case errors.Is(err, finance.ErrInvalidReviewAction),
		errors.Is(err, finance.ErrReviewerRequired),
		errors.Is(err, finance.ErrReviewNoteRequired),
		errors.Is(err, finance.ErrMissingExtraction):

		writeError(
			w,
			http.StatusBadRequest,
			err.Error(),
		)

	default:
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to review claim: %v", err),
		)
	}
}
