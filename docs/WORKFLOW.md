# End-to-End Workflow

## 1. Report submission

The employee submits report metadata plus one or more receipt/invoice files.

The parent report is created in a processing state and each document becomes a child claim.

## 2. Cheap deterministic document checks

Before AI work:

1. ensure document bytes are present;
2. calculate SHA-256;
3. detect MIME type;
4. perform employee-aware exact-hash duplicate lookup.

If the same employee already submitted the exact same binary:

```text
EXACT_HASH
-> DUPLICATE_DETECTED
-> AIProcessingSkipped=true
-> $0 automatic reimbursement
```

Cross-employee exact reuse is not blocked as a self-duplicate; it is retained as Finance risk evidence.

## 3. Preserve original evidence

The original file is stored in the `DocumentStore`.

Processing derivatives, such as rendered PDF pages, are not treated as the original evidence artifact.

## 4. Normalize the document

Supported inputs:

- JPEG;
- PNG;
- PDF.

PDF pages are rendered for vision processing. Multiple pages from the same PDF are treated as one expense document.

## 5. Primary AI extraction

`qwen3-vl:4b-instruct` returns structured JSON.

No policy or reimbursement decision is accepted from the model.

## 6. Normalize and validate

Go performs deterministic normalization and validation.

Checks include:

- merchant/date/currency/total required fields;
- `YYYY-MM-DD` date format;
- non-negative money fields;
- total formula;
- line-item sum vs subtotal;
- currency normalization;
- extraction confidence warning;
- transportation-specific reconciliation.

## 7. Model escalation

Only if the primary extraction is invalid:

```text
Qwen3-VL 4B invalid
-> fresh Qwen3-VL 8B extraction
-> deterministic re-validation
```

The 8B call is fresh. The first model's output is not used as trusted context for the second model.

If the result remains invalid:

```text
HUMAN_REVIEW_REQUIRED
```

## 8. Metadata duplicate/risk detection

After a valid extraction the system compares transaction metadata with prior audited claims.

Signals include:

- merchant;
- amount;
- expense date;
- receipt number.

The detector distinguishes:

```text
NONE
EXACT_HASH
METADATA_MATCH
CROSS_EMPLOYEE_EXACT
CROSS_EMPLOYEE_METADATA
```

## 9. Employee clarification

A same-employee metadata match can require clarification.

- `NO`: treat as duplicate and reimburse zero.
- `YES` + weak similarity: continue to policy.
- `YES` + strong transaction identity: route to `MANUAL_REVIEW`.

## 10. Finance gate

Cross-employee risk sets:

```text
requires_admin_review=true
```

This is a reimbursement gate:

```text
MANUAL_REVIEW
policy_decision=nil
automatic reimbursement = 0
```

## 11. Policy evaluation

For a normal claim, weak duplicate accepted after clarification, or Finance-approved claim:

1. load the active published policy from PostgreSQL;
2. match employee grade;
3. match category;
4. verify currency;
5. apply allowed/maximum rules;
6. record policy ID/name/version.

## 12. Business persistence and audit

The system persists:

### PostgreSQL

- claim state;
- report state;
- report/claim relationship;
- business events;
- policy versions/rules.

### DocumentStore

- original receipt/invoice evidence.

### Audit JSON

- document hash/MIME;
- model usage;
- extraction;
- validation;
- duplicate result;
- clarification/policy result;
- final status.

## 13. Parent report refresh

Whenever a child claim changes through clarification or Finance review, the parent report is recalculated so the UI reflects the latest report-level status and totals.
