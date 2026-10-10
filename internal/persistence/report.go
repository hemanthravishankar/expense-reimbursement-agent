package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/hemanth-shankar/expense-agent/internal/models"

	"github.com/jackc/pgx/v5"
)

var ErrReportNotFound = errors.New("expense report not found")

func (r *PostgresRepository) CreateExpenseReport(
	ctx context.Context,
	report *models.ExpenseReport,
) error {

	if report == nil {
		return fmt.Errorf(
			"expense report is nil",
		)
	}

	if report.ReportID == "" {
		return fmt.Errorf(
			"report ID is required",
		)
	}

	if report.Employee.ID == "" {
		return fmt.Errorf(
			"employee ID is required",
		)
	}

	_, err :=
		r.pool.Exec(
			ctx,
			`
			INSERT INTO expense_reports (
				report_id,
				employee_id,
				employee_name,
				employee_grade,
				department,
				location,
				title,
				purpose,
				status
			)
			VALUES (
				$1,
				$2,
				$3,
				$4,
				$5,
				$6,
				$7,
				$8,
				$9
			)
			`,
			report.ReportID,
			report.Employee.ID,
			report.Employee.Name,
			report.Employee.Grade,
			report.Employee.Department,
			report.Employee.Location,
			report.Title,
			report.Purpose,
			report.Status,
		)

	if err != nil {
		return fmt.Errorf(
			"create expense report: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) AttachClaimToReport(
	ctx context.Context,
	reportID string,
	claimID string,
	position int,
) error {

	_, err :=
		r.pool.Exec(
			ctx,
			`
			INSERT INTO expense_report_items (
				report_id,
				claim_id,
				position
			)
			VALUES (
				$1,
				$2,
				$3
			)
			`,
			reportID,
			claimID,
			position,
		)

	if err != nil {
		return fmt.Errorf(
			"attach claim to expense report: %w",
			err,
		)
	}

	return nil
}

func (r *PostgresRepository) UpdateExpenseReportStatus(
	ctx context.Context,
	reportID string,
	status string,
) error {

	commandTag, err :=
		r.pool.Exec(
			ctx,
			`
			UPDATE expense_reports
			SET
				status = $2,
				updated_at = NOW()
			WHERE report_id = $1
			`,
			reportID,
			status,
		)

	if err != nil {
		return fmt.Errorf(
			"update expense report status: %w",
			err,
		)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrReportNotFound
	}

	return nil
}

func (r *PostgresRepository) GetExpenseReport(
	ctx context.Context,
	reportID string,
) (*models.ExpenseReport, error) {

	var report models.ExpenseReport

	err :=
		r.pool.QueryRow(
			ctx,
			`
			SELECT
				report_id,
				employee_id,
				employee_name,
				employee_grade,
				COALESCE(department, ''),
				COALESCE(location, ''),
				title,
				COALESCE(purpose, ''),
				status,
				created_at,
				updated_at
			FROM expense_reports
			WHERE report_id = $1
			`,
			reportID,
		).
			Scan(
				&report.ReportID,
				&report.Employee.ID,
				&report.Employee.Name,
				&report.Employee.Grade,
				&report.Employee.Department,
				&report.Employee.Location,
				&report.Title,
				&report.Purpose,
				&report.Status,
				&report.CreatedAt,
				&report.UpdatedAt,
			)

	if errors.Is(
		err,
		pgx.ErrNoRows,
	) {
		return nil,
			ErrReportNotFound
	}

	if err != nil {
		return nil,
			fmt.Errorf(
				"get expense report: %w",
				err,
			)
	}

	items, err :=
		r.getExpenseReportItems(
			ctx,
			reportID,
		)

	if err != nil {
		return nil, err
	}

	report.Items =
		items

	report.ItemCount =
		len(items)

	for _, item := range items {

		report.TotalClaimed +=
			item.ClaimedAmount

		report.TotalReimbursable +=
			item.ReimbursableAmount
	}

	return &report, nil
}

func (r *PostgresRepository) getExpenseReportItems(
	ctx context.Context,
	reportID string,
) ([]models.ExpenseReportItem, error) {

	rows, err :=
		r.pool.Query(
			ctx,
			`
			SELECT
				c.claim_id,
				eri.position,
				c.status,

				COALESCE(
					c.result_json
						-> 'final_extraction'
						->> 'merchant',
					''
				),

				COALESCE(
					c.result_json
						-> 'final_extraction'
						->> 'category_suggestion',
					''
				),

				COALESCE(
					c.result_json
						-> 'final_extraction'
						->> 'currency',
					''
				),

				COALESCE(
					(
						c.result_json
							-> 'final_extraction'
							->> 'total_amount'
					)::DOUBLE PRECISION,
					0
				),

				COALESCE(
					(
						c.result_json
							-> 'policy_decision'
							->> 'reimbursable_amount'
					)::DOUBLE PRECISION,
					0
				),

				COALESCE(
					c.result_json
						-> 'stored_document'
						->> 'original_name',
					c.result_json
						->> 'receipt_path',
					''
				)

			FROM expense_report_items eri

			JOIN claims c
				ON c.claim_id =
					eri.claim_id

			WHERE eri.report_id = $1

			ORDER BY eri.position ASC
			`,
			reportID,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"get expense report items: %w",
				err,
			)
	}

	defer rows.Close()

	items :=
		[]models.ExpenseReportItem{}

	for rows.Next() {

		var item models.ExpenseReportItem

		if err :=
			rows.Scan(
				&item.ClaimID,
				&item.Position,
				&item.Status,
				&item.Merchant,
				&item.Category,
				&item.Currency,
				&item.ClaimedAmount,
				&item.ReimbursableAmount,
				&item.DocumentName,
			); err != nil {

			return nil,
				fmt.Errorf(
					"scan expense report item: %w",
					err,
				)
		}

		items =
			append(
				items,
				item,
			)
	}

	if err :=
		rows.Err(); err != nil {

		return nil,
			fmt.Errorf(
				"iterate expense report items: %w",
				err,
			)
	}

	return items, nil
}

func (r *PostgresRepository) ListEmployeeExpenseReports(
	ctx context.Context,
	employeeID string,
) ([]models.ExpenseReport, error) {

	rows, err :=
		r.pool.Query(
			ctx,
			`
			SELECT
				er.report_id,
				er.employee_id,
				er.employee_name,
				er.employee_grade,
				COALESCE(er.department, ''),
				COALESCE(er.location, ''),
				er.title,
				COALESCE(er.purpose, ''),
				er.status,

				COUNT(eri.claim_id)::INTEGER,

				COALESCE(
					SUM(
						COALESCE(
							NULLIF(
								c.result_json
									-> 'final_extraction'
									->> 'total_amount',
								''
							)::DOUBLE PRECISION,
							0
						)
					),
					0
				)::DOUBLE PRECISION,

				COALESCE(
					SUM(
						COALESCE(
							NULLIF(
								c.result_json
									-> 'policy_decision'
									->> 'reimbursable_amount',
								''
							)::DOUBLE PRECISION,
							0
						)
					),
					0
				)::DOUBLE PRECISION,

				er.created_at,
				er.updated_at

			FROM expense_reports er

			LEFT JOIN expense_report_items eri
				ON eri.report_id =
					er.report_id

			LEFT JOIN claims c
				ON c.claim_id =
					eri.claim_id

			WHERE er.employee_id = $1

			GROUP BY
				er.report_id,
				er.employee_id,
				er.employee_name,
				er.employee_grade,
				er.department,
				er.location,
				er.title,
				er.purpose,
				er.status,
				er.created_at,
				er.updated_at

			ORDER BY er.created_at DESC
			`,
			employeeID,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"list employee expense reports: %w",
				err,
			)
	}

	defer rows.Close()

	reports :=
		[]models.ExpenseReport{}

	for rows.Next() {

		var report models.ExpenseReport

		if err :=
			rows.Scan(
				&report.ReportID,
				&report.Employee.ID,
				&report.Employee.Name,
				&report.Employee.Grade,
				&report.Employee.Department,
				&report.Employee.Location,
				&report.Title,
				&report.Purpose,
				&report.Status,
				&report.ItemCount,
				&report.TotalClaimed,
				&report.TotalReimbursable,
				&report.CreatedAt,
				&report.UpdatedAt,
			); err != nil {

			return nil,
				fmt.Errorf(
					"scan expense report: %w",
					err,
				)
		}

		reports =
			append(
				reports,
				report,
			)
	}

	if err :=
		rows.Err(); err != nil {

		return nil,
			fmt.Errorf(
				"iterate expense reports: %w",
				err,
			)
	}

	return reports, nil
}

func (r *PostgresRepository) ListExpenseReports(
	ctx context.Context,
) ([]models.ExpenseReport, error) {

	rows, err :=
		r.pool.Query(
			ctx,
			`
			SELECT
				er.report_id,
				er.employee_id,
				er.employee_name,
				er.employee_grade,
				COALESCE(er.department, ''),
				COALESCE(er.location, ''),
				er.title,
				COALESCE(er.purpose, ''),
				er.status,

				COUNT(eri.claim_id)::INTEGER,

				COALESCE(
					SUM(
						COALESCE(
							NULLIF(
								c.result_json
									-> 'final_extraction'
									->> 'total_amount',
								''
							)::DOUBLE PRECISION,
							0
						)
					),
					0
				)::DOUBLE PRECISION,

				COALESCE(
					SUM(
						COALESCE(
							NULLIF(
								c.result_json
									-> 'policy_decision'
									->> 'reimbursable_amount',
								''
							)::DOUBLE PRECISION,
							0
						)
					),
					0
				)::DOUBLE PRECISION,

				er.created_at,
				er.updated_at

			FROM expense_reports er

			LEFT JOIN expense_report_items eri
				ON eri.report_id =
					er.report_id

			LEFT JOIN claims c
				ON c.claim_id =
					eri.claim_id

			GROUP BY
				er.report_id,
				er.employee_id,
				er.employee_name,
				er.employee_grade,
				er.department,
				er.location,
				er.title,
				er.purpose,
				er.status,
				er.created_at,
				er.updated_at

			ORDER BY er.created_at DESC
			`,
		)

	if err != nil {
		return nil,
			fmt.Errorf(
				"list expense reports: %w",
				err,
			)
	}

	defer rows.Close()

	reports :=
		[]models.ExpenseReport{}

	for rows.Next() {

		var report models.ExpenseReport

		if err :=
			rows.Scan(
				&report.ReportID,
				&report.Employee.ID,
				&report.Employee.Name,
				&report.Employee.Grade,
				&report.Employee.Department,
				&report.Employee.Location,
				&report.Title,
				&report.Purpose,
				&report.Status,
				&report.ItemCount,
				&report.TotalClaimed,
				&report.TotalReimbursable,
				&report.CreatedAt,
				&report.UpdatedAt,
			); err != nil {

			return nil,
				fmt.Errorf(
					"scan expense report: %w",
					err,
				)
		}

		reports =
			append(
				reports,
				report,
			)
	}

	if err :=
		rows.Err(); err != nil {

		return nil,
			fmt.Errorf(
				"iterate expense reports: %w",
				err,
			)
	}

	return reports, nil
}
