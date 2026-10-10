# API Reference

Base development URL:

```text
http://localhost:8080
```

In the Docker frontend, Nginx proxies same-origin `/api/*` and `/healthz` requests to the API service.

## Health

### `GET /healthz`

Response:

```json
{
  "status": "ok"
}
```

---

## Expense reports and claims

### `POST /api/v1/expenses/analyze`

Legacy/single-expense multipart processing route.

### `POST /api/v1/expense-reports`

Creates a multi-receipt report from multipart form data.

### `GET /api/v1/expense-reports/{reportID}`

Returns report detail and child items.

### `GET /api/v1/employees/{employeeID}/expense-reports`

Returns the employee's report history.

### `GET /api/v1/admin/expense-reports`

Returns the Finance/Admin report queue.

### `GET /api/v1/claims/{claimID}`

Returns the detailed claim/workflow result.

### `GET /api/v1/claims/{claimID}/events`

Returns claim lifecycle events.

### `GET /api/v1/claims/{claimID}/document`

Returns the original stored receipt/invoice inline using its original MIME type.

---

## Clarification

### `POST /api/v1/claims/{claimID}/clarifications`

Typical payload:

```json
{
  "question_code": "DUPLICATE_CONFIRMATION",
  "answer": "yes",
  "explanation": "This is a separate legitimate transaction."
}
```

A `YES` does not automatically override strong identity.

---

## Finance review

### `POST /api/v1/admin/claims/{claimID}/review`

Approve:

```json
{
  "action": "APPROVE",
  "reviewer": "Finance User",
  "note": "Verified as a legitimate separate business transaction."
}
```

Reject:

```json
{
  "action": "REJECT",
  "reviewer": "Finance User",
  "note": "Receipt is not eligible for reimbursement."
}
```

Rules:

- action must be `APPROVE` or `REJECT`;
- reviewer is required;
- note is required;
- claim must be in a reviewable state.

Approve loads the current active policy and deterministically evaluates the validated extraction.

---

## Policy administration

### `GET /api/v1/admin/policies`

Returns policy version history.

### `GET /api/v1/admin/policies/active`

Returns the currently effective published policy in runtime `limits` form.

Example:

```json
{
  "policy_id": "POL-DEMO-USD-V1",
  "name": "NovaTech Demo Expense Policy",
  "version": 1,
  "currency": "USD",
  "limits": {
    "B2": {
      "Meals": {
        "allowed": true,
        "max_amount": 50
      }
    }
  }
}
```

### `GET /api/v1/admin/policies/{policyID}`

Returns one version including its rule list.

### `POST /api/v1/admin/policies`

Creates a draft.

Example:

```json
{
  "name": "NovaTech Demo Expense Policy",
  "currency": "USD",
  "effective_from": "2026-10-11",
  "created_by": "Finance User",
  "rules": [
    {
      "employee_grade": "B2",
      "category": "Meals",
      "allowed": true,
      "max_amount": 60
    }
  ]
}
```

Response status: `201 Created`.

### `PUT /api/v1/admin/policies/{policyID}`

Updates a draft.

Example:

```json
{
  "currency": "USD",
  "effective_from": "2026-10-11",
  "rules": [
    {
      "employee_grade": "B2",
      "category": "Meals",
      "allowed": true,
      "max_amount": 60
    }
  ]
}
```

Only `DRAFT` versions can be changed.

### `POST /api/v1/admin/policies/{policyID}/publish`

Example:

```json
{
  "published_by": "Finance User"
}
```

Publishing activates the draft and retires the previously published version.

---

## Common behavior

JSON request decoders for Finance/policy routes reject unknown fields.

Errors are returned as JSON:

```json
{
  "error": "message"
}
```

## Current prototype access model

The API currently does not implement authentication/RBAC. Admin route naming represents intended role boundaries in the prototype; production deployment must enforce them with identity and authorization.
