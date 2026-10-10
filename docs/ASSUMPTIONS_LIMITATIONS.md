# Assumptions and Limitations

## Prototype assumptions

### Identity

Employee ID, name, grade, department, and location are supplied by the demo UI.

They are not currently sourced from SSO/HR systems.

### Baseline policy

A fresh database seeds one published demo policy:

```text
NovaTech Demo Expense Policy v1
Currency: USD
Grade: B2
```

Finance can create later versions.

### Category

The model provides `category_suggestion`.

The deterministic policy engine uses that category as input.

A production design may add controlled category mapping/normalization or Finance category correction.

### Duplicate history

The current duplicate detector reads historical audit records from the local audit directory.

This is appropriate for the prototype but should move to indexed database/vector/search infrastructure at large scale.

### Synchronous inference

Receipt processing waits for local model inference.

A production implementation can move processing to queues/workers for better throughput.

## Supported document scope

Implemented document types:

- JPG/JPEG;
- PNG;
- PDF.

PDF rendering depends on Poppler/`pdftoppm`.

## AI limitations

Vision models may:

- misread low-quality text;
- omit fields;
- choose the wrong category;
- misunderstand uncommon fee layouts.

This is why deterministic validation and human-review states exist.

## Policy limitations

The current rule model is intentionally simple:

```text
employee grade
+ category
+ allowed
+ max amount
```

It does not yet model:

- trip duration;
- city tiers;
- per-diem windows;
- manager hierarchy;
- receipt-age limits;
- cost center/project budgets;
- tax jurisdictions;
- exchange-rate conversion;
- mileage calculations;
- multi-level approvals.

## Finance limitations

The prototype provides reviewer/note capture but no authenticated reviewer identity.

Production must bind the reviewer to an authenticated principal.

## Storage limitations

Local evidence/audit files are suitable for local evaluation.

Production should use:

- encrypted object storage;
- retention policies;
- backup;
- access logging;
- immutability controls where required.

## Deployment scope

The Compose deployment is designed to be reproducible locally.

It is not presented as production hardening.

Public cloud hosting is optional and intentionally not required for the submission because local Qwen vision inference is resource-sensitive.

## Known testing command constraint

Use:

```bash
go test ./cmd/... ./internal/...
```

rather than:

```bash
go test ./...
```

because frontend dependency folders can interfere with Go package discovery in the repository.

## Future improvements

- server-side auth/RBAC;
- indexed duplicate history;
- configurable category normalization;
- async worker queue;
- S3/object storage;
- managed inference;
- richer policies;
- currency conversion;
- Finance category correction;
- observability and metrics;
- CI pipeline;
- optional Kubernetes deployment.
