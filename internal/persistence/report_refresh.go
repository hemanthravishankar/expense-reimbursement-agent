package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/hemanth-shankar/expense-agent/internal/reportstatus"
	"github.com/jackc/pgx/v5"
)

// RefreshExpenseReportForClaim recalculates the parent
// expense-report status whenever a child claim changes.
//
// Claims that do not belong to an expense report are valid,
// so pgx.ErrNoRows is intentionally ignored.
func (r *PostgresRepository) RefreshExpenseReportForClaim(
	ctx context.Context,
	claimID string,
) error {

	var reportID string

	err :=
		r.pool.
			QueryRow(
				ctx,
				`
				SELECT report_id
				FROM expense_report_items
				WHERE claim_id = $1
				LIMIT 1
				`,
				claimID,
			).
			Scan(
				&reportID,
			)

	if errors.Is(
		err,
		pgx.ErrNoRows,
	) {
		return nil
	}

	if err != nil {
		return fmt.Errorf(
			"find parent expense report for claim %s: %w",
			claimID,
			err,
		)
	}

	report, err :=
		r.GetExpenseReport(
			ctx,
			reportID,
		)

	if err != nil {
		return fmt.Errorf(
			"load parent expense report %s: %w",
			reportID,
			err,
		)
	}

	newStatus :=
		reportstatus.Calculate(
			report.Items,
		)

	if report.Status == newStatus {
		return nil
	}

	if err :=
		r.UpdateExpenseReportStatus(
			ctx,
			reportID,
			newStatus,
		); err != nil {

		return fmt.Errorf(
			"update expense report %s from %s to %s: %w",
			reportID,
			report.Status,
			newStatus,
			err,
		)
	}

	return nil
}
