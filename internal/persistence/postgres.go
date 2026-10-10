package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hemanth-shankar/expense-agent/internal/workflow"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(
	ctx context.Context,
	databaseURL string,
) (*PostgresRepository, error) {

	if databaseURL == "" {
		return nil, fmt.Errorf(
			"DATABASE_URL is required",
		)
	}

	pool, err :=
		pgxpool.New(
			ctx,
			databaseURL,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"create PostgreSQL pool: %w",
			err,
		)
	}

	if err :=
		pool.Ping(ctx); err != nil {

		pool.Close()

		return nil, fmt.Errorf(
			"connect to PostgreSQL: %w",
			err,
		)
	}

	repository :=
		&PostgresRepository{
			pool: pool,
		}

	if err :=
		repository.initialize(
			ctx,
		); err != nil {

		pool.Close()

		return nil, err
	}

	return repository, nil
}

func (r *PostgresRepository) initialize(
	ctx context.Context,
) error {

	_, err :=
		r.pool.Exec(
			ctx,
			schema,
		)

	if err != nil {
		return fmt.Errorf(
			"initialize PostgreSQL schema: %w",
			err,
		)
	}

	if err := r.ensureDefaultPolicy(ctx); err != nil {
		return fmt.Errorf(
			"initialize default reimbursement policy: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) Create(
	ctx context.Context,
	result *workflow.Result,
) error {

	if result == nil {
		return fmt.Errorf(
			"claim result is nil",
		)
	}

	resultJSON, err :=
		json.Marshal(result)

	if err != nil {
		return fmt.Errorf(
			"marshal claim result: %w",
			err,
		)
	}

	var documentKey any

	if result.StoredDocument != nil {
		documentKey =
			result.StoredDocument.Key
	}

	_, err =
		r.pool.Exec(
			ctx,
			`
			INSERT INTO claims (
				claim_id,
				employee_id,
				status,
				document_sha256,
				document_key,
				result_json
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6
			)
			`,
			result.RunID,
			result.Employee.ID,
			result.FinalStatus,
			result.ReceiptSHA256,
			documentKey,
			resultJSON,
		)

	if err != nil {
		return fmt.Errorf(
			"create claim: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) Get(
	ctx context.Context,
	claimID string,
) (*workflow.Result, error) {

	var resultJSON []byte

	err :=
		r.pool.QueryRow(
			ctx,
			`
			SELECT result_json
			FROM claims
			WHERE claim_id = $1
			`,
			claimID,
		).
			Scan(
				&resultJSON,
			)

	if errors.Is(
		err,
		pgx.ErrNoRows,
	) {
		return nil,
			ErrClaimNotFound
	}

	if err != nil {
		return nil, fmt.Errorf(
			"get claim: %w",
			err,
		)
	}

	var result workflow.Result

	if err :=
		json.Unmarshal(
			resultJSON,
			&result,
		); err != nil {

		return nil, fmt.Errorf(
			"decode claim result: %w",
			err,
		)
	}

	return &result, nil
}

func (r *PostgresRepository) Update(
	ctx context.Context,
	result *workflow.Result,
) error {

	if result == nil {
		return fmt.Errorf(
			"claim result is nil",
		)
	}

	resultJSON, err :=
		json.Marshal(result)

	if err != nil {
		return fmt.Errorf(
			"marshal claim result: %w",
			err,
		)
	}

	var documentKey any

	if result.StoredDocument != nil {
		documentKey =
			result.StoredDocument.Key
	}

	commandTag, err :=
		r.pool.Exec(
			ctx,
			`
			UPDATE claims
			SET
				status = $2,
				document_sha256 = $3,
				document_key = $4,
				result_json = $5,
				updated_at = NOW()
			WHERE claim_id = $1
			`,
			result.RunID,
			result.FinalStatus,
			result.ReceiptSHA256,
			documentKey,
			resultJSON,
		)

	if err != nil {
		return fmt.Errorf(
			"update claim: %w",
			err,
		)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrClaimNotFound
	}

	return nil
}

func (r *PostgresRepository) AppendEvent(
	ctx context.Context,
	claimID string,
	eventType string,
	payload any,
) error {

	var payloadJSON []byte
	var err error

	if payload != nil {

		payloadJSON, err =
			json.Marshal(payload)

		if err != nil {
			return fmt.Errorf(
				"marshal claim event: %w",
				err,
			)
		}
	}

	_, err =
		r.pool.Exec(
			ctx,
			`
			INSERT INTO claim_events (
				claim_id,
				event_type,
				payload_json
			)
			VALUES (
				$1,
				$2,
				$3
			)
			`,
			claimID,
			eventType,
			payloadJSON,
		)

	if err != nil {
		return fmt.Errorf(
			"append claim event: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) GetEvents(
	ctx context.Context,
	claimID string,
) ([]ClaimEvent, error) {

	rows, err :=
		r.pool.Query(
			ctx,
			`
			SELECT
				id,
				claim_id,
				event_type,
				payload_json,
				created_at
			FROM claim_events
			WHERE claim_id = $1
			ORDER BY id ASC
			`,
			claimID,
		)

	if err != nil {
		return nil, fmt.Errorf(
			"get claim events: %w",
			err,
		)
	}

	defer rows.Close()

	events :=
		[]ClaimEvent{}

	for rows.Next() {

		var event ClaimEvent

		if err :=
			rows.Scan(
				&event.ID,
				&event.ClaimID,
				&event.EventType,
				&event.Payload,
				&event.CreatedAt,
			); err != nil {

			return nil, fmt.Errorf(
				"scan claim event: %w",
				err,
			)
		}

		events =
			append(
				events,
				event,
			)
	}

	if err :=
		rows.Err(); err != nil {

		return nil, fmt.Errorf(
			"iterate claim events: %w",
			err,
		)
	}

	return events, nil
}

func (r *PostgresRepository) Close() {

	if r.pool != nil {
		r.pool.Close()
	}
}
