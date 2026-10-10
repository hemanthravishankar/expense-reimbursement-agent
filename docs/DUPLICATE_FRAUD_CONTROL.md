# Duplicate and Fraud Controls

## Purpose

Duplicate detection is not only a SHA-256 check.

A user can resize, re-encode, screenshot, or otherwise modify the file while preserving the same underlying transaction.

The system therefore uses two stages.

## Stage 1 — exact binary hash

Before AI:

```text
SHA-256(current document)
```

is compared with historical audited documents.

### Same employee

```text
same SHA
-> EXACT_HASH
-> is_duplicate=true
-> AI skipped
-> DUPLICATE_DETECTED
```

### Different employee

```text
same SHA
-> CROSS_EMPLOYEE_EXACT
-> is_duplicate=false
-> requires_admin_review=true
```

It is not labelled as a self-duplicate because the historical submission belongs to a different employee.

## Stage 2 — metadata similarity

After extraction the detector compares:

| Signal | Weight |
|---|---:|
| Merchant | 0.30 |
| Total amount | 0.30 |
| Expense date | 0.25 |
| Receipt number | 0.15 |

Metadata threshold:

```text
0.85
```

## Strong transaction identity

A metadata match is considered **strong identity** when all of the following match:

```text
non-empty receipt number
AND expense date
AND total amount
```

A strong identity score is treated as at least `0.95`.

## Match types

```text
NONE
EXACT_HASH
METADATA_MATCH
CROSS_EMPLOYEE_EXACT
CROSS_EMPLOYEE_METADATA
```

Important result fields:

```json
{
  "is_duplicate": false,
  "requires_admin_review": true,
  "match_type": "CROSS_EMPLOYEE_EXACT",
  "confidence": 1.0,
  "strong_identity": true,
  "matched_run_id": "...",
  "matched_employee_id": "...",
  "reason": "..."
}
```

## Priority

The detector prioritizes the current employee's historical relationship correctly.

Simplified order:

```text
same-employee exact
same-employee metadata
cross-employee exact
cross-employee metadata
none
```

This prevents a cross-employee copy from hiding a more relevant same-employee historical match.

## Clarification behavior

### Same employee + metadata match + NO

```text
CLARIFICATION_REQUIRED
-> employee says NO
-> DUPLICATE_DETECTED
-> $0
```

### Same employee + weak metadata + justified YES

```text
CLARIFICATION_REQUIRED
-> employee says YES with explanation
-> deterministic policy
```

### Same employee + strong identity + YES

```text
CLARIFICATION_REQUIRED
-> employee says YES
-> strong_identity=true
-> MANUAL_REVIEW
-> $0 automatic reimbursement
```

The employee cannot self-authorize a second reimbursement when transaction-level identity evidence is strong.

## Cross-employee behavior

```text
CROSS_EMPLOYEE_EXACT
or CROSS_EMPLOYEE_METADATA
-> requires_admin_review=true
-> MANUAL_REVIEW
-> policy_decision=nil
-> $0 pending Finance
```

Finance can then compare the current and matched original evidence side by side.

## Why cross-employee risk is not automatically fraud

A cross-employee match can have legitimate explanations:

- shared business booking;
- receipt copied to the wrong employee;
- centralized purchase;
- duplicate reimbursement attempt;
- data-entry/testing scenario.

The software therefore raises a risk signal and requires Finance review rather than making an accusation.
