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
`
