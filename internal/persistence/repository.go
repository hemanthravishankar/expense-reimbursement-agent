package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/workflow"
)

var ErrClaimNotFound = errors.New("claim not found")

type ClaimEvent struct {
	ID        int64           `json:"id"`
	ClaimID   string          `json:"claim_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
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
	) ([]ClaimEvent, error)

	Close()
}
