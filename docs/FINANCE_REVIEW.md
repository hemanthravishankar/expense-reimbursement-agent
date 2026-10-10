# Finance Review Workflow

## When Finance review is required

Typical gates include:

- cross-employee exact receipt reuse;
- cross-employee metadata similarity;
- strong same-employee transaction identity after a `YES` clarification;
- extraction that remains invalid after model fallback.

Reviewable backend states include:

```text
MANUAL_REVIEW
HUMAN_REVIEW_REQUIRED
```

## Finance queue

The Finance portal groups report cases and exposes:

- report status;
- employee;
- receipt count;
- claimed/reimbursable totals;
- case details;
- receipt-level statuses.

## Evidence comparison

For duplicate/risk cases the claim records:

```text
matched_run_id
matched_employee_id
```

The Finance UI uses `matched_run_id` to load the historical claim and the receipt-evidence route for both claims.

Comparison fields include:

- merchant;
- expense date;
- receipt number;
- total;
- current receipt;
- matched/reference receipt.

## Evidence API

```http
GET /api/v1/claims/{claimID}/document
```

The backend:

1. loads the claim;
2. resolves the stored document key;
3. reads the original from `DocumentStore`;
4. returns the original MIME type;
5. serves it inline;
6. sets `X-Content-Type-Options: nosniff`.

## Review request

```http
POST /api/v1/admin/claims/{claimID}/review
Content-Type: application/json
```

Approve example:

```json
{
  "action": "APPROVE",
  "reviewer": "Finance User",
  "note": "Verified as a legitimate separate business transaction."
}
```

Reject example:

```json
{
  "action": "REJECT",
  "reviewer": "Finance User",
  "note": "Receipt was already reimbursed under another employee."
}
```

Reviewer and note are mandatory.

## Approval semantics

Finance approval does **not** manually enter a reimbursement amount.

It means:

```text
Finance confirms legitimacy
-> load active published policy
-> deterministic policy evaluation
-> APPROVED / LIMIT_EXCEEDED / NOT_ALLOWED /
   CURRENCY_MISMATCH / MANUAL_REVIEW
```

This preserves separation of duties.

## Rejection semantics

```text
REJECT
-> policy_decision=nil
-> REJECTED
-> $0 reimbursement
```

## Events

A successful review appends:

```text
FINANCE_REVIEWED
CLAIM_STATUS_CHANGED
```

The Finance event records:

- action;
- reviewer;
- note;
- previous/new status;
- duplicate match type when available;
- matched claim and employee;
- duplicate confidence;
- strong-identity flag.

## Parent refresh

After a Finance action, the parent report is recalculated so the report queue and employee history reflect the new child status.

## Employee privacy behavior

Cross-employee matched identity is Finance evidence.

The employee-facing portal intentionally avoids exposing the other employee's details and instead shows that Finance review is required.
