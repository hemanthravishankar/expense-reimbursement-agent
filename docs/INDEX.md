# Documentation Index

This folder is the technical and product documentation set for the Expense & Reimbursement Agent.

## Recommended reading order

1. [FEATURES.md](FEATURES.md) — what the product does.
2. [ARCHITECTURE.md](ARCHITECTURE.md) — how the system is structured.
3. [WORKFLOW.md](WORKFLOW.md) — how a receipt becomes a reimbursement decision.
4. [AI_DESIGN.md](AI_DESIGN.md) — how the vision models are constrained.
5. [DUPLICATE_FRAUD_CONTROL.md](DUPLICATE_FRAUD_CONTROL.md) — fraud/risk controls.
6. [POLICY_ENGINE.md](POLICY_ENGINE.md) — deterministic and versioned reimbursement.
7. [FINANCE_REVIEW.md](FINANCE_REVIEW.md) — manual review and evidence.
8. [DATA_MODEL.md](DATA_MODEL.md) — database and audit persistence.
9. [API_REFERENCE.md](API_REFERENCE.md) — REST endpoints.
10. [DEPLOYMENT.md](DEPLOYMENT.md) — reproducible Docker Compose setup.
11. [TESTING.md](TESTING.md) and [TEST_MATRIX.md](TEST_MATRIX.md) — validation strategy.
12. [SECURITY.md](SECURITY.md) and [ASSUMPTIONS_LIMITATIONS.md](ASSUMPTIONS_LIMITATIONS.md) — trust model and prototype boundaries.
13. [DEMO_SCRIPT.md](DEMO_SCRIPT.md) — final submission recording.
14. [SUBMISSION_CHECKLIST.md](SUBMISSION_CHECKLIST.md) — final handoff.

## Core principle

> **LLM understands/proposes; deterministic Go code verifies/enforces.**

The project is designed around this boundary. AI can be wrong without being allowed to directly authorize reimbursement.
