# Test Matrix

## Automated commands

```bash
go test ./cmd/... ./internal/...
```

```bash
cd web
npm run build
npm run lint
```

```bash
./scripts/docker-preflight.sh
docker compose up --build -d
./scripts/docker-smoke-test.sh
```

## Functional / E2E matrix

| ID | Scenario | Expected child result | Expected parent result | Automatic reimbursement |
|---|---|---|---|---:|
| E2E-01 | Fresh B2 taxi $26 + meal $55.20 under V1 | Taxi `APPROVED`; meal `LIMIT_EXCEEDED` | `COMPLETED` | $76 total |
| E2E-02 | Same employee submits modified copy and answers NO | `DUPLICATE_DETECTED` | `REJECTED` | $0 |
| E2E-03 | Same employee strong duplicate answers YES | `MANUAL_REVIEW` | `REVIEW_REQUIRED` | $0 pending Finance |
| E2E-04 | Different employee submits exact same historical receipt | `MANUAL_REVIEW` + `CROSS_EMPLOYEE_EXACT` | `REVIEW_REQUIRED` | $0 pending Finance |
| E2E-05 | Finance rejects a review case | `REJECTED` | recalculated | $0 |
| E2E-06 | Finance approves a legitimate review case | active deterministic policy result | recalculated | policy result |
| E2E-07 | Publish V2: B2 Meals $50 -> $60 | policy V2 `PUBLISHED`, V1 `RETIRED` | n/a | n/a |
| E2E-08 | Fresh $55.20 B2 meal after V2 | `APPROVED` | `COMPLETED` | $55.20 |
| DEP-01 | Fresh Compose build/start | all services healthy | n/a | n/a |
| DEP-02 | Smoke script | health + active policy + frontend pass | n/a | n/a |

## Baseline E2E-01

With V1:

```text
Transportation limit: $100
Meals limit:          $50
```

Expected:

```text
Taxi:
  claimed:       26.00
  reimbursable:  26.00
  status:        APPROVED

Meal:
  claimed:       55.20
  reimbursable:  50.00
  status:        LIMIT_EXCEEDED

Parent:
  claimed:       81.20
  reimbursable:  76.00
  status:        COMPLETED
```

## Duplicate assertions

### Exact same employee

```text
same SHA-256
EXACT_HASH
is_duplicate=true
AI skipped
DUPLICATE_DETECTED
```

### Modified same employee

```text
different SHA-256
metadata score >= threshold
METADATA_MATCH
CLARIFICATION_REQUIRED
```

### Strong identity

```text
matching non-empty receipt number
AND same expense date
AND same total
```

Strong identity plus employee `YES`:

```text
MANUAL_REVIEW
policy_decision=nil
$0 pending Finance
```

### Cross-employee exact

```text
match_type=CROSS_EMPLOYEE_EXACT
is_duplicate=false
requires_admin_review=true
strong_identity=true
MANUAL_REVIEW
policy_decision=nil
```

## Finance assertions

### Reject

Verify:

```text
REJECTED
policy_decision=nil
reimbursable=0
FINANCE_REVIEWED event
CLAIM_STATUS_CHANGED event
parent refresh
```

### Approve

Verify:

```text
reviewer and note captured
active policy loaded
policy decision produced
policy_id/name/version present
parent refresh
```

## Policy-version assertions

After publishing V2:

```text
V2 status = PUBLISHED
V1 status = RETIRED
active policy version = 2
```

A claim decided under V1 must keep its historical V1 policy metadata.

## Final recording order

```text
1. Normal report
2. Modified duplicate -> NO
3. Strong duplicate -> YES
4. Cross-employee exact
5. Finance reject
6. Finance approve
7. Publish policy V2
8. New meal proves V2
9. Show tests / Compose health
```

This order creates duplicate evidence naturally during the demonstration.
