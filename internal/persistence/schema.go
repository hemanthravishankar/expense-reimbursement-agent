package persistence

const schema = `
CREATE TABLE IF NOT EXISTS claims (
    claim_id TEXT PRIMARY KEY,

    employee_id TEXT NOT NULL,

    status TEXT NOT NULL,

    document_sha256 TEXT NOT NULL,

    document_key TEXT,

    result_json JSONB NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_claims_employee_id
ON claims(employee_id);

CREATE INDEX IF NOT EXISTS idx_claims_status
ON claims(status);

CREATE INDEX IF NOT EXISTS idx_claims_document_sha256
ON claims(document_sha256);


CREATE TABLE IF NOT EXISTS claim_events (
    id BIGSERIAL PRIMARY KEY,

    claim_id TEXT NOT NULL
        REFERENCES claims(claim_id)
        ON DELETE CASCADE,

    event_type TEXT NOT NULL,

    payload_json JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_claim_events_claim_id
ON claim_events(claim_id);


-- =========================================================
-- MULTI-RECEIPT EXPENSE REPORTS
-- =========================================================

CREATE TABLE IF NOT EXISTS expense_reports (
    report_id TEXT PRIMARY KEY,

    employee_id TEXT NOT NULL,

    employee_name TEXT NOT NULL,

    employee_grade TEXT NOT NULL,

    department TEXT,

    location TEXT,

    title TEXT NOT NULL,

    purpose TEXT,

    status TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_expense_reports_employee_id
ON expense_reports(employee_id);

CREATE INDEX IF NOT EXISTS idx_expense_reports_status
ON expense_reports(status);

CREATE INDEX IF NOT EXISTS idx_expense_reports_created_at
ON expense_reports(created_at DESC);


CREATE TABLE IF NOT EXISTS expense_report_items (
    report_id TEXT NOT NULL
        REFERENCES expense_reports(report_id)
        ON DELETE CASCADE,

    claim_id TEXT NOT NULL
        REFERENCES claims(claim_id)
        ON DELETE CASCADE,

    position INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (report_id, claim_id),

    UNIQUE (claim_id)
);

CREATE INDEX IF NOT EXISTS idx_expense_report_items_report
ON expense_report_items(report_id);


-- =========================================================
-- ADMIN MANAGED REIMBURSEMENT POLICIES
-- =========================================================

CREATE TABLE IF NOT EXISTS policy_versions (
    policy_id TEXT PRIMARY KEY,

    name TEXT NOT NULL,

    version INTEGER NOT NULL,

    currency TEXT NOT NULL,

    status TEXT NOT NULL,

    effective_from DATE NOT NULL,

    effective_to DATE,

    created_by TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    published_at TIMESTAMPTZ,

    UNIQUE (name, version)
);

CREATE INDEX IF NOT EXISTS idx_policy_versions_status
ON policy_versions(status);

CREATE INDEX IF NOT EXISTS idx_policy_versions_effective
ON policy_versions(effective_from, effective_to);

ALTER TABLE policy_versions
ADD COLUMN IF NOT EXISTS published_by TEXT;

ALTER TABLE policy_versions
ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();


CREATE TABLE IF NOT EXISTS policy_rules (
    id BIGSERIAL PRIMARY KEY,

    policy_id TEXT NOT NULL
        REFERENCES policy_versions(policy_id)
        ON DELETE CASCADE,

    employee_grade TEXT NOT NULL,

    category TEXT NOT NULL,

    allowed BOOLEAN NOT NULL,

    max_amount NUMERIC(14,2),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (
        policy_id,
        employee_grade,
        category
    )
);

CREATE INDEX IF NOT EXISTS idx_policy_rules_policy
ON policy_rules(policy_id);

CREATE INDEX IF NOT EXISTS idx_policy_rules_grade
ON policy_rules(employee_grade);
`
