# Security and Trust Model

## Security objective

The central security goal is to prevent probabilistic AI output or untrusted user input from directly authorizing reimbursement.

## Implemented controls

### AI isolation from financial authority

The model can extract/suggest.

It cannot directly decide:

- reimbursable amount;
- policy limit;
- duplicate gate;
- Finance legitimacy;
- active policy version.

### Exact duplicate pre-check

SHA-256 can reject a same-employee exact re-upload before AI processing.

Benefits:

- deterministic evidence;
- faster rejection;
- reduced unnecessary AI processing.

### Modified-copy detection

Metadata matching addresses simple hash evasion through:

- resize;
- re-encoding;
- screenshot/copy;
- other binary changes.

### Strong-identity gate

Matching:

```text
receipt number + date + amount
```

is treated as strong transaction identity.

An employee's `YES` clarification does not automatically bypass this evidence.

### Cross-employee Finance gate

Cross-employee exact/metadata risk results in:

```text
MANUAL_REVIEW
policy_decision=nil
$0 pending Finance
```

### Finance audit fields

Finance review requires a reviewer and note.

Events record action and status transition.

### Original evidence preservation

The original file is stored separately from derived pages used for AI processing.

### Evidence response hardening

The document endpoint serves the stored MIME type and uses:

```text
X-Content-Type-Options: nosniff
```

### Versioned policy audit

Claims preserve the policy identity/version used for the decision.

Published/retired policy versions are read-only in the UI.

## Privacy considerations

The default AI path uses local Ollama rather than an external hosted AI API.

Cross-employee identity/matched claim information is displayed in Finance views and masked from the employee-facing duplicate message.

## Current prototype security gaps

The following are **not yet production controls**:

- authentication;
- authorization/RBAC;
- SSO;
- CSRF protection strategy for authenticated browser sessions;
- TLS termination;
- production secret management;
- encryption-at-rest policy for receipt evidence;
- malware scanning of uploaded documents;
- enterprise DLP;
- immutable centralized audit retention;
- database backup/restore policy;
- rate limiting;
- tenant isolation.

The Compose configuration is therefore intended for local evaluation/demo use.

## Production security evolution

Recommended controls:

1. Put the UI/API behind SSO/OIDC.
2. Derive employee identity from the authenticated session, not form fields.
3. Enforce Finance/Admin RBAC server-side.
4. Use TLS for every network boundary.
5. Store secrets in a secret manager.
6. Move evidence to encrypted object storage with restricted access.
7. Use signed/short-lived evidence URLs or authorized streaming.
8. Add upload malware/content scanning.
9. Centralize tamper-resistant audit events.
10. Add rate limits and request tracing.
11. Add database backups and retention controls.
12. Protect the inference endpoint with network/auth controls.

## Important distinction

The duplicate engine is a **risk-control system**, not a fraud conviction system.

Cross-employee matches are intentionally routed for human investigation rather than automatically accusing an employee.
