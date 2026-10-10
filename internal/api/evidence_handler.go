package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/persistence"
)

func (h *Handler) getClaimDocument(
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

	if h.documentStore == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"document storage is not configured",
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

	result, err := h.repository.Get(
		r.Context(),
		claimID,
	)
	if errors.Is(err, persistence.ErrClaimNotFound) {
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
			fmt.Sprintf("failed to retrieve claim: %v", err),
		)
		return
	}

	if result.StoredDocument == nil ||
		strings.TrimSpace(result.StoredDocument.Key) == "" {
		writeError(
			w,
			http.StatusNotFound,
			"claim document is not available",
		)
		return
	}

	data, err := h.documentStore.Get(
		r.Context(),
		result.StoredDocument.Key,
	)
	if errors.Is(err, os.ErrNotExist) {
		writeError(
			w,
			http.StatusNotFound,
			"claim document is not available",
		)
		return
	}
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to read claim document: %v", err),
		)
		return
	}

	mimeType := strings.TrimSpace(
		result.StoredDocument.MIMEType,
	)
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	filename := filepath.Base(
		result.StoredDocument.OriginalName,
	)
	if filename == "." || filename == "" {
		filename = "receipt"
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf("inline; filename=%q", filename),
	)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
