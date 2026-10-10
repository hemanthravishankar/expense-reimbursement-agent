# Submission Demo Script

Target duration: **7–9 minutes**.

The goal is to prove the controls live, not only show prepared screens.

## Before recording

Do one rehearsal first.

Then reset the Compose application:

```bash
docker compose down -v
docker compose up --build -d
./scripts/docker-smoke-test.sh
```

Confirm active policy V1:

```bash
curl -s http://localhost:8080/api/v1/admin/policies/active \
| python3 -m json.tool
```

Confirm:

```text
B2 Meals = $50
B2 Transportation = $100
```

Have receipt files ready and close unrelated tabs.

## 0:00–0:40 — Problem and principle

Show the Employee/Finance application briefly.

Say:

> Expense automation becomes risky when an LLM is allowed to both read a receipt and authorize money. This agent separates those responsibilities: Qwen3-VL understands the document, while deterministic Go code validates the extraction, detects duplicate risk and enforces reimbursement policy.

Show the architecture diagram.

## 0:40–1:50 — Normal multi-receipt report

Use B2 employee A and upload:

```text
Taxi: $26
Meal: $55.20
```

While inference runs, explain:

> The file is hashed first, exact duplicates are checked before AI, the original is preserved, Qwen3-VL extracts structured fields, and Go validates the arithmetic. The larger model is only used if deterministic validation rejects the first extraction.

Show expected result:

```text
Taxi:
APPROVED
$26 -> $26

Meal:
LIMIT_EXCEEDED
$55.20 -> $50

Report:
$81.20 claimed
$76 reimbursable
COMPLETED
```

Say:

> The model did not decide that the meal reimbursement is fifty dollars. Version 1 of the policy did.

## 1:50–2:45 — Modified duplicate

Submit a modified/re-encoded copy of the meal as the same employee.

Show:

```text
different SHA-256
METADATA_MATCH
CLARIFICATION_REQUIRED
```

Answer `NO`.

Show:

```text
DUPLICATE_DETECTED
$0
```

Explain that hash-only detection is insufficient.

## 2:45–3:35 — Adversarial strong duplicate

Submit another modified copy of the same transaction.

Answer `YES` with an explanation.

Show:

```text
strong_identity=true
MANUAL_REVIEW
$0 pending Finance
```

Say:

> Employee clarification is evidence, not authority. Matching receipt number, date and amount is strong identity, so the employee cannot self-authorize a second reimbursement.

## 3:35–4:30 — Cross-employee reuse

Switch to employee B and submit the original historical receipt.

Show:

```text
CROSS_EMPLOYEE_EXACT
requires_admin_review=true
MANUAL_REVIEW
$0 automatic reimbursement
```

Explain that the system does not accuse the second employee automatically; it creates a Finance investigation case.

## 4:30–5:45 — Finance review

Open Finance -> Review Queue.

Show:

- current receipt;
- reference receipt;
- matched employee;
- match confidence;
- strong identity;
- merchant/date/receipt number/amount;
- timeline.

For one case, reject with a note.

Show:

```text
REJECTED
$0
FINANCE_REVIEWED
```

For another case, approve as legitimate.

Explain:

> Finance approves legitimacy, not an amount. The currently published deterministic policy still calculates reimbursement.

## 5:45–6:50 — Dynamic policy management

Open Policy Management.

Show V1 active.

Create a draft from active and change:

```text
B2 Meals: $50 -> $60
```

Publish.

Show:

```text
V2 PUBLISHED / active
V1 RETIRED / historical
```

Explain that no application rebuild/restart is required.

## 6:50–7:35 — Prove policy version change

Return to Employee and submit a fresh $55.20 meal.

Expected:

```text
APPROVED
$55.20 reimbursable
policy version 2
```

Contrast:

```text
before V2 -> LIMIT_EXCEEDED -> $50
after V2  -> APPROVED       -> $55.20
```

## 7:35–8:15 — Engineering proof

Briefly show repository docs/tests.

Terminal:

```bash
go test ./cmd/... ./internal/...
```

Optionally:

```bash
cd web
npm run build
npm run lint
```

And:

```bash
docker compose ps
```

Mention:

- local vision inference;
- fresh model fallback;
- deterministic validation;
- two-stage duplicate detection;
- Finance evidence;
- versioned policies;
- PostgreSQL;
- Docker Compose;
- audit trail.

## Closing

> The core idea is not merely using AI to read receipts. It is building a financially safe agent around AI: extraction can be probabilistic, but reimbursement authorization remains deterministic, reviewable and auditable.
