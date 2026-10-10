# Features

## Employee Portal

### Multi-receipt expense reports

An employee can create one report containing multiple receipt/invoice documents.

The report aggregates:

- receipt count;
- total claimed amount;
- total reimbursable amount;
- parent report status.

### Receipt result view

For each receipt the UI can display:

- merchant;
- expense date;
- category;
- receipt number;
- total;
- status;
- claimed amount;
- reimbursable amount;
- policy reason.

Technical information is available on demand rather than dominating the normal employee experience.

### Clarification workflow

When a same-employee metadata match needs clarification, the employee can answer whether the receipt is a separate transaction.

A `YES` response requires an explanation in the UI.

The answer does not blindly authorize reimbursement. Strong transaction identity still routes the claim to Finance.

### History

Employees can browse previously submitted reports and reopen report/receipt details.

### Privacy-aware duplicate messaging

Cross-employee risk cases are not presented to the employee with another employee's identity. The employee-facing message only indicates that Finance review is required.

---

## Finance Portal

### Review queue

Finance can filter reports by review/completed/rejected/all states and inspect cases that need attention.

### Risk evidence

The Finance view exposes the evidence needed to investigate:

- match type;
- match confidence;
- strong-identity flag;
- matched employee ID;
- matched run/claim;
- extraction fields;
- claim timeline.

### Side-by-side receipt evidence

When a duplicate result contains `matched_run_id`, Finance can load both:

- current receipt;
- matched/reference receipt.

Images are displayed directly. PDFs are displayed in an embedded frame and can also be opened separately.

### Approve / Reject

Finance review requires:

- action: `APPROVE` or `REJECT`;
- reviewer identity;
- review note.

Approve means **the transaction is considered legitimate for policy evaluation**. Finance does not manually choose the reimbursable amount.

Reject results in zero reimbursement.

### Event history

Finance actions create business events including:

- `FINANCE_REVIEWED`;
- `CLAIM_STATUS_CHANGED`.

The parent report is refreshed after the review.

---

## Policy Management

### Versioned policies

Finance can:

- list policy versions;
- inspect the active version;
- create a draft;
- edit draft rules;
- publish/activate a draft.

Lifecycle:

```text
DRAFT -> PUBLISHED
previous PUBLISHED -> RETIRED
```

Published and retired versions are read-only.

### Runtime policy resolution

The active published policy is loaded from PostgreSQL at decision time.

It is used for:

- normal new claims;
- weak duplicate clarification that is allowed to continue;
- Finance-approved review cases.

Publishing a new version does not require an API rebuild or restart.

### Historical auditability

Each policy decision stores:

- `policy_id`;
- `policy_name`;
- `policy_version`.

Historical claim decisions are not rewritten when a new policy version becomes active.

---

## AI and Document Features

- JPG support.
- PNG support.
- PDF support.
- Multi-page PDF pages treated as one expense document.
- Qwen3-VL 4B primary extraction.
- Fresh Qwen3-VL 8B fallback.
- Structured JSON extraction.
- Temperature 0.
- Deterministic validation before financial decision.
- Transportation-specific reconciliation logic.

---

## Duplicate and Fraud Controls

- Same-employee exact binary duplicate detection before AI.
- Metadata duplicate detection after extraction.
- Cross-employee exact receipt reuse detection.
- Cross-employee metadata similarity detection.
- Strong transaction identity.
- Finance review gate.
- Zero automatic reimbursement while a gated review is unresolved.

---

## Deployment and Operations

- Dockerized Go API.
- Dockerized frontend/Nginx.
- PostgreSQL 16 container.
- Host Ollama integration.
- Persistent named volumes.
- Health checks.
- Preflight script.
- Smoke-test script.
- Clean reset with `docker compose down -v`.
