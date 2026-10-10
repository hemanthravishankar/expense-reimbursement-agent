package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hemanth-shankar/expense-agent/internal/policy"
	"github.com/jackc/pgx/v5"
)

func (r *PostgresRepository) ensureDefaultPolicy(
	ctx context.Context,
) error {
	var publishedCount int

	if err := r.pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM policy_versions WHERE status = $1`,
		policy.VersionStatusPublished,
	).Scan(&publishedCount); err != nil {
		return fmt.Errorf("count published policies: %w", err)
	}

	if publishedCount > 0 {
		return nil
	}

	demo := policy.DemoUSDPolicy()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin default policy transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	policyID := demo.ID

	var existingID string
	err = tx.QueryRow(
		ctx,
		`
		SELECT policy_id
		FROM policy_versions
		WHERE name = $1 AND version = $2
		LIMIT 1
		`,
		demo.Name,
		demo.Version,
	).Scan(&existingID)

	switch {
	case err == nil:
		policyID = existingID
		_, err = tx.Exec(
			ctx,
			`
			UPDATE policy_versions
			SET
				currency = $2,
				status = $3,
				effective_from = CURRENT_DATE,
				effective_to = NULL,
				published_at = NOW(),
				published_by = 'system',
				updated_at = NOW()
			WHERE policy_id = $1
			`,
			policyID,
			demo.Currency,
			policy.VersionStatusPublished,
		)
		if err != nil {
			return fmt.Errorf("activate default policy: %w", err)
		}

	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(
			ctx,
			`
			INSERT INTO policy_versions (
				policy_id,
				name,
				version,
				currency,
				status,
				effective_from,
				created_by,
				published_at,
				published_by
			)
			VALUES ($1, $2, $3, $4, $5, CURRENT_DATE, 'system', NOW(), 'system')
			`,
			policyID,
			demo.Name,
			demo.Version,
			demo.Currency,
			policy.VersionStatusPublished,
		)
		if err != nil {
			return fmt.Errorf("insert default policy: %w", err)
		}

	default:
		return fmt.Errorf("find default policy: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM policy_rules WHERE policy_id = $1`,
		policyID,
	); err != nil {
		return fmt.Errorf("clear default policy rules: %w", err)
	}

	grades := make([]string, 0, len(demo.Limits))
	for grade := range demo.Limits {
		grades = append(grades, grade)
	}
	sort.Strings(grades)

	for _, grade := range grades {
		categories := make([]string, 0, len(demo.Limits[grade]))
		for category := range demo.Limits[grade] {
			categories = append(categories, category)
		}
		sort.Strings(categories)

		for _, category := range categories {
			rule := demo.Limits[grade][category]
			if _, err := tx.Exec(
				ctx,
				`
				INSERT INTO policy_rules (
					policy_id,
					employee_grade,
					category,
					allowed,
					max_amount
				)
				VALUES ($1, $2, $3, $4, $5)
				`,
				policyID,
				grade,
				category,
				rule.Allowed,
				rule.MaxAmount,
			); err != nil {
				return fmt.Errorf("insert default policy rule: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit default policy: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ActivePolicy(
	ctx context.Context,
) (policy.Policy, error) {
	var active policy.Policy

	err := r.pool.QueryRow(
		ctx,
		`
		SELECT
			policy_id,
			name,
			version,
			currency
		FROM policy_versions
		WHERE status = $1
		  AND effective_from <= CURRENT_DATE
		  AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
		ORDER BY effective_from DESC, version DESC, created_at DESC
		LIMIT 1
		`,
		policy.VersionStatusPublished,
	).Scan(
		&active.ID,
		&active.Name,
		&active.Version,
		&active.Currency,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return policy.Policy{}, policy.ErrNoActivePolicy
	}
	if err != nil {
		return policy.Policy{}, fmt.Errorf("get active policy: %w", err)
	}

	rules, err := r.loadPolicyRules(ctx, active.ID)
	if err != nil {
		return policy.Policy{}, err
	}

	active.Limits = rulesToLimits(rules)
	return active, nil
}

func (r *PostgresRepository) ListPolicyVersions(
	ctx context.Context,
) ([]policy.Version, error) {
	rows, err := r.pool.Query(
		ctx,
		`
		SELECT
			policy_id,
			name,
			version,
			currency,
			status,
			effective_from,
			effective_to,
			created_by,
			created_at,
			COALESCE(published_by, ''),
			published_at
		FROM policy_versions
		ORDER BY created_at DESC, version DESC
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list policy versions: %w", err)
	}
	defer rows.Close()

	versions := []policy.Version{}

	for rows.Next() {
		version, err := scanPolicyVersion(rows)
		if err != nil {
			return nil, err
		}

		rules, err := r.loadPolicyRules(ctx, version.PolicyID)
		if err != nil {
			return nil, err
		}
		version.Rules = rules
		versions = append(versions, *version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy versions: %w", err)
	}

	return versions, nil
}

func (r *PostgresRepository) GetPolicyVersion(
	ctx context.Context,
	policyID string,
) (*policy.Version, error) {
	row := r.pool.QueryRow(
		ctx,
		`
		SELECT
			policy_id,
			name,
			version,
			currency,
			status,
			effective_from,
			effective_to,
			created_by,
			created_at,
			COALESCE(published_by, ''),
			published_at
		FROM policy_versions
		WHERE policy_id = $1
		`,
		policyID,
	)

	version, err := scanPolicyVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, policy.ErrPolicyNotFound
	}
	if err != nil {
		return nil, err
	}

	rules, err := r.loadPolicyRules(ctx, policyID)
	if err != nil {
		return nil, err
	}
	version.Rules = rules

	return version, nil
}

func (r *PostgresRepository) CreatePolicyVersion(
	ctx context.Context,
	request policy.CreateVersionRequest,
) (*policy.Version, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create policy version: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var nextVersion int
	if err := tx.QueryRow(
		ctx,
		`
		SELECT COALESCE(MAX(version), 0) + 1
		FROM policy_versions
		WHERE name = $1
		`,
		request.Name,
	).Scan(&nextVersion); err != nil {
		return nil, fmt.Errorf("calculate next policy version: %w", err)
	}

	policyID := generatePolicyID()

	if _, err := tx.Exec(
		ctx,
		`
		INSERT INTO policy_versions (
			policy_id,
			name,
			version,
			currency,
			status,
			effective_from,
			created_by
		)
		VALUES ($1, $2, $3, $4, $5, $6::date, $7)
		`,
		policyID,
		request.Name,
		nextVersion,
		request.Currency,
		policy.VersionStatusDraft,
		request.EffectiveFrom,
		request.CreatedBy,
	); err != nil {
		return nil, fmt.Errorf("insert policy version: %w", err)
	}

	if err := insertPolicyRules(ctx, tx, policyID, request.Rules); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy version: %w", err)
	}

	return r.GetPolicyVersion(ctx, policyID)
}

func (r *PostgresRepository) UpdatePolicyVersion(
	ctx context.Context,
	policyID string,
	request policy.UpdateVersionRequest,
) (*policy.Version, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin update policy version: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	if err := tx.QueryRow(
		ctx,
		`SELECT status FROM policy_versions WHERE policy_id = $1 FOR UPDATE`,
		policyID,
	).Scan(&status); errors.Is(err, pgx.ErrNoRows) {
		return nil, policy.ErrPolicyNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock policy version: %w", err)
	}

	if status != policy.VersionStatusDraft {
		return nil, policy.ErrPolicyNotDraft
	}

	if _, err := tx.Exec(
		ctx,
		`
		UPDATE policy_versions
		SET
			currency = $2,
			effective_from = $3::date,
			updated_at = NOW()
		WHERE policy_id = $1
		`,
		policyID,
		request.Currency,
		request.EffectiveFrom,
	); err != nil {
		return nil, fmt.Errorf("update policy version: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM policy_rules WHERE policy_id = $1`,
		policyID,
	); err != nil {
		return nil, fmt.Errorf("delete policy rules: %w", err)
	}

	if err := insertPolicyRules(ctx, tx, policyID, request.Rules); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy update: %w", err)
	}

	return r.GetPolicyVersion(ctx, policyID)
}

func (r *PostgresRepository) PublishPolicyVersion(
	ctx context.Context,
	policyID string,
	publishedBy string,
) (*policy.Version, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin publish policy version: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	var effectiveFrom time.Time
	if err := tx.QueryRow(
		ctx,
		`
		SELECT status, effective_from
		FROM policy_versions
		WHERE policy_id = $1
		FOR UPDATE
		`,
		policyID,
	).Scan(&status, &effectiveFrom); errors.Is(err, pgx.ErrNoRows) {
		return nil, policy.ErrPolicyNotFound
	} else if err != nil {
		return nil, fmt.Errorf("lock policy for publish: %w", err)
	}

	if status != policy.VersionStatusDraft {
		return nil, policy.ErrPolicyNotDraft
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	effectiveDate := effectiveFrom.UTC().Truncate(24 * time.Hour)
	if effectiveDate.After(today) {
		return nil, policy.ErrFuturePolicyPublish
	}

	if _, err := tx.Exec(
		ctx,
		`
		UPDATE policy_versions
		SET
			status = $1,
			effective_to = CURRENT_DATE,
			updated_at = NOW()
		WHERE status = $2
		  AND policy_id <> $3
		`,
		policy.VersionStatusRetired,
		policy.VersionStatusPublished,
		policyID,
	); err != nil {
		return nil, fmt.Errorf("retire previous policy: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`
		UPDATE policy_versions
		SET
			status = $2,
			effective_to = NULL,
			published_at = NOW(),
			published_by = $3,
			updated_at = NOW()
		WHERE policy_id = $1
		`,
		policyID,
		policy.VersionStatusPublished,
		publishedBy,
	); err != nil {
		return nil, fmt.Errorf("publish policy version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit policy publish: %w", err)
	}

	return r.GetPolicyVersion(ctx, policyID)
}

func (r *PostgresRepository) loadPolicyRules(
	ctx context.Context,
	policyID string,
) ([]policy.Rule, error) {
	rows, err := r.pool.Query(
		ctx,
		`
		SELECT
			employee_grade,
			category,
			allowed,
			COALESCE(max_amount, 0)::DOUBLE PRECISION
		FROM policy_rules
		WHERE policy_id = $1
		ORDER BY employee_grade ASC, category ASC
		`,
		policyID,
	)
	if err != nil {
		return nil, fmt.Errorf("load policy rules: %w", err)
	}
	defer rows.Close()

	rules := []policy.Rule{}
	for rows.Next() {
		var rule policy.Rule
		if err := rows.Scan(
			&rule.EmployeeGrade,
			&rule.Category,
			&rule.Allowed,
			&rule.MaxAmount,
		); err != nil {
			return nil, fmt.Errorf("scan policy rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate policy rules: %w", err)
	}

	return rules, nil
}

func rulesToLimits(
	rules []policy.Rule,
) map[string]map[string]policy.CategoryRule {
	limits := map[string]map[string]policy.CategoryRule{}

	for _, rule := range rules {
		if _, exists := limits[rule.EmployeeGrade]; !exists {
			limits[rule.EmployeeGrade] = map[string]policy.CategoryRule{}
		}

		limits[rule.EmployeeGrade][rule.Category] = policy.CategoryRule{
			Allowed:   rule.Allowed,
			MaxAmount: rule.MaxAmount,
		}
	}

	return limits
}

type policyVersionScanner interface {
	Scan(dest ...any) error
}

func scanPolicyVersion(
	scanner policyVersionScanner,
) (*policy.Version, error) {
	var version policy.Version
	var effectiveFrom time.Time
	var effectiveTo *time.Time
	var publishedAt *time.Time

	if err := scanner.Scan(
		&version.PolicyID,
		&version.Name,
		&version.Version,
		&version.Currency,
		&version.Status,
		&effectiveFrom,
		&effectiveTo,
		&version.CreatedBy,
		&version.CreatedAt,
		&version.PublishedBy,
		&publishedAt,
	); err != nil {
		return nil, err
	}

	version.EffectiveFrom = effectiveFrom.Format("2006-01-02")
	if effectiveTo != nil {
		version.EffectiveTo = effectiveTo.Format("2006-01-02")
	}
	version.PublishedAt = publishedAt
	version.Rules = []policy.Rule{}

	return &version, nil
}

func insertPolicyRules(
	ctx context.Context,
	tx pgx.Tx,
	policyID string,
	rules []policy.Rule,
) error {
	for _, rule := range rules {
		maxAmount := rule.MaxAmount
		if !rule.Allowed {
			maxAmount = 0
		}

		if _, err := tx.Exec(
			ctx,
			`
			INSERT INTO policy_rules (
				policy_id,
				employee_grade,
				category,
				allowed,
				max_amount
			)
			VALUES ($1, $2, $3, $4, $5)
			`,
			policyID,
			rule.EmployeeGrade,
			rule.Category,
			rule.Allowed,
			maxAmount,
		); err != nil {
			return fmt.Errorf("insert policy rule: %w", err)
		}
	}

	return nil
}

func generatePolicyID() string {
	return "POL-" + strings.ReplaceAll(
		time.Now().UTC().Format("20060102T150405.000000000Z"),
		".",
		"",
	)
}
