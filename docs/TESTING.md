# Testing Strategy

## Test philosophy

The highest-risk parts of the project are deterministic controls around AI.

Tests therefore focus on:

- extraction fallback boundaries;
- money validation;
- duplicate routing;
- clarification behavior;
- policy outcomes;
- Finance review semantics;
- parent report refresh;
- persistence;
- document evidence;
- policy version management.

## Backend test command

```bash
go test ./cmd/... ./internal/...
```

Do not use `go test ./...` in this repository because frontend dependencies can interfere with Go package discovery.

## Frontend checks

```bash
cd web
npm run build
npm run lint
```

## Deployment checks

Preflight:

```bash
./scripts/docker-preflight.sh
```

Build/start:

```bash
docker compose up --build -d
docker compose ps
```

Smoke test:

```bash
./scripts/docker-smoke-test.sh
```

The smoke test checks:

- API health;
- active policy retrieval;
- frontend availability.

## Important backend test areas

### AI integration boundary

- primary extraction path;
- fresh fallback when validation fails;
- invalid-after-fallback -> human review;
- endpoint configuration for host/container Ollama.

### Validation

- required fields;
- dates;
- non-negative amounts;
- total formula;
- line-item subtotal consistency;
- low-confidence warning;
- currency normalization;
- transportation reconciliation.

### Duplicate detector

- exact same employee;
- modified/re-encoded same employee;
- strong identity;
- cross-employee exact;
- cross-employee metadata;
- matching priority.

### Clarification

- duplicate confirmation `NO`;
- weak-match `YES`;
- strong identity `YES` -> manual review.

### Policy

- approved;
- limit exceeded;
- not allowed;
- currency mismatch;
- missing grade/category -> manual review;
- policy ID/name/version recorded.

### Policy management

- draft creation;
- draft update;
- immutable non-draft behavior;
- publish;
- retirement of previous active version;
- active policy retrieval.

### Finance

- mandatory action/reviewer/note;
- approve -> active policy applied;
- reject -> zero reimbursement;
- reviewability guard;
- Finance events;
- status events;
- report refresh.

### Evidence

- stored original retrieval;
- content type;
- document-not-found behavior.

### Reports

- multi-receipt creation;
- child linkage;
- status aggregation;
- totals;
- history/query paths.

## E2E philosophy

The final E2E should start from a clean application state and then create the evidence history needed for duplicate scenarios in a deliberate order.

This produces a reproducible demonstration rather than relying on hidden historical data.

See [TEST_MATRIX.md](TEST_MATRIX.md).
