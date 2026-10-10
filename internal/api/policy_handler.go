package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hemanth-shankar/expense-agent/internal/policy"
)

func (h *Handler) listPolicies(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.policyManager == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"policy management is not configured",
		)
		return
	}

	versions, err := h.policyManager.List(r.Context())
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to list policies: %v", err),
		)
		return
	}

	writeJSON(w, http.StatusOK, versions)
}

func (h *Handler) getActivePolicy(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.policyManager == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"policy management is not configured",
		)
		return
	}

	active, err := h.policyManager.Active(r.Context())
	if errors.Is(err, policy.ErrNoActivePolicy) {
		writeError(
			w,
			http.StatusNotFound,
			err.Error(),
		)
		return
	}
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to load active policy: %v", err),
		)
		return
	}

	writeJSON(w, http.StatusOK, active)
}

func (h *Handler) getPolicy(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.policyManager == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"policy management is not configured",
		)
		return
	}

	policyID := strings.TrimSpace(r.PathValue("policyID"))
	if policyID == "" {
		writeError(w, http.StatusBadRequest, "policy ID is required")
		return
	}

	version, err := h.policyManager.Get(r.Context(), policyID)
	if errors.Is(err, policy.ErrPolicyNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to load policy: %v", err),
		)
		return
	}

	writeJSON(w, http.StatusOK, version)
}

func (h *Handler) createPolicy(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.policyManager == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"policy management is not configured",
		)
		return
	}

	var request policy.CreateVersionRequest
	if err := decodePolicyJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	version, err := h.policyManager.Create(r.Context(), request)
	if err != nil {
		if isPolicyValidationError(err) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to create policy: %v", err),
		)
		return
	}

	writeJSON(w, http.StatusCreated, version)
}

func (h *Handler) updatePolicy(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.policyManager == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"policy management is not configured",
		)
		return
	}

	policyID := strings.TrimSpace(r.PathValue("policyID"))
	if policyID == "" {
		writeError(w, http.StatusBadRequest, "policy ID is required")
		return
	}

	var request policy.UpdateVersionRequest
	if err := decodePolicyJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	version, err := h.policyManager.Update(
		r.Context(),
		policyID,
		request,
	)

	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, version)

	case errors.Is(err, policy.ErrPolicyNotFound):
		writeError(w, http.StatusNotFound, err.Error())

	case errors.Is(err, policy.ErrPolicyNotDraft):
		writeError(w, http.StatusConflict, err.Error())

	case isPolicyValidationError(err):
		writeError(w, http.StatusBadRequest, err.Error())

	default:
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to update policy: %v", err),
		)
	}
}

func (h *Handler) publishPolicy(
	w http.ResponseWriter,
	r *http.Request,
) {
	if h.policyManager == nil {
		writeError(
			w,
			http.StatusServiceUnavailable,
			"policy management is not configured",
		)
		return
	}

	policyID := strings.TrimSpace(r.PathValue("policyID"))
	if policyID == "" {
		writeError(w, http.StatusBadRequest, "policy ID is required")
		return
	}

	var request policy.PublishRequest
	if err := decodePolicyJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	version, err := h.policyManager.Publish(
		r.Context(),
		policyID,
		request,
	)

	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, version)

	case errors.Is(err, policy.ErrPolicyNotFound):
		writeError(w, http.StatusNotFound, err.Error())

	case errors.Is(err, policy.ErrPolicyNotDraft):
		writeError(w, http.StatusConflict, err.Error())

	case errors.Is(err, policy.ErrFuturePolicyPublish),
		errors.Is(err, policy.ErrPolicyPublisher):
		writeError(w, http.StatusBadRequest, err.Error())

	default:
		writeError(
			w,
			http.StatusInternalServerError,
			fmt.Sprintf("failed to publish policy: %v", err),
		)
	}
}

func decodePolicyJSON(
	w http.ResponseWriter,
	r *http.Request,
	destination any,
) error {
	decoder := json.NewDecoder(
		http.MaxBytesReader(
			w,
			r.Body,
			2<<20,
		),
	)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid policy request")
	}

	return nil
}

func isPolicyValidationError(err error) bool {
	return errors.Is(err, policy.ErrPolicyNameRequired) ||
		errors.Is(err, policy.ErrPolicyCurrency) ||
		errors.Is(err, policy.ErrPolicyCreator) ||
		errors.Is(err, policy.ErrPolicyPublisher) ||
		errors.Is(err, policy.ErrPolicyRulesRequired) ||
		errors.Is(err, policy.ErrInvalidPolicyRule) ||
		strings.Contains(err.Error(), "effective_from")
}
