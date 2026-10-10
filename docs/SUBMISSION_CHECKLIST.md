# Submission Checklist

## Code

- [ ] Working tree clean.
- [ ] Final branch pushed to GitHub.
- [ ] Final E2E completed after clean reset.
- [ ] Merge stable branch into `main`.
- [ ] Tag submission version, e.g. `v1.0.0`.
- [ ] No `.env`, runtime receipts, audit files, database volumes, `node_modules`, or build output committed.

## Backend validation

- [ ] `go test ./cmd/... ./internal/...` passes.
- [ ] API starts from a fresh PostgreSQL database.
- [ ] V1 default policy is seeded.
- [ ] Normal multi-receipt flow passes.
- [ ] Same-employee modified duplicate flow passes.
- [ ] Strong duplicate `YES` routes to Finance.
- [ ] Cross-employee exact match routes to Finance.
- [ ] Finance reject passes.
- [ ] Finance approve applies active policy.
- [ ] Policy draft/publish passes.
- [ ] New claim proves new policy version.

## Frontend validation

- [ ] `npm run build` passes.
- [ ] `npm run lint` passes.
- [ ] Employee history cards render correctly.
- [ ] Finance review queue renders correctly.
- [ ] Current/reference evidence renders correctly.
- [ ] Finance reviewer/note validation works.
- [ ] Policy Management draft/save/publish works.
- [ ] Employee view does not expose cross-employee identity details.

## Deployment

- [ ] `./scripts/docker-preflight.sh` passes.
- [ ] `docker compose up --build -d` succeeds.
- [ ] PostgreSQL healthy.
- [ ] API healthy.
- [ ] Frontend healthy.
- [ ] `./scripts/docker-smoke-test.sh` passes.
- [ ] One real receipt successfully processes through the Dockerized UI.
- [ ] Clean reset documented with `docker compose down -v`.

## Documentation

- [ ] Root `README.md`.
- [ ] `docs/INDEX.md`.
- [ ] Architecture.
- [ ] Tech stack.
- [ ] Features.
- [ ] AI design.
- [ ] Workflow.
- [ ] Duplicate/fraud controls.
- [ ] Policy engine.
- [ ] Finance review.
- [ ] Data model.
- [ ] API reference.
- [ ] Security.
- [ ] Deployment.
- [ ] Testing.
- [ ] E2E matrix.
- [ ] Assumptions/limitations.
- [ ] Demo script.

## GitHub presentation

- [ ] Repository description is concise.
- [ ] README renders correctly.
- [ ] Mermaid diagrams render on GitHub.
- [ ] Add 2–4 screenshots under `docs/assets/`.
- [ ] Add final YouTube demo link to README.
- [ ] Repository is accessible to evaluators.
- [ ] No secrets are present in Git history.

## Demo video

- [ ] 7–9 minutes target.
- [ ] Clean state before recording.
- [ ] Normal policy flow.
- [ ] Modified duplicate.
- [ ] Strong duplicate.
- [ ] Cross-employee case.
- [ ] Side-by-side Finance evidence.
- [ ] Finance reject.
- [ ] Finance approve.
- [ ] Policy V2 publish.
- [ ] Fresh claim demonstrates V2.
- [ ] Tests / Compose health shown.
- [ ] Video uploaded and accessible.

## Final submission narrative

Make these points explicit:

1. **Problem:** AI extraction alone is unsafe for financial authorization.
2. **Innovation:** probabilistic extraction surrounded by deterministic validation, fraud controls, human review, and versioned policy.
3. **Complexity:** multi-stage AI fallback, duplicate identity, multi-receipt reports, Finance review, policy lifecycle, evidence/audit separation.
4. **Reliability:** unit tests, E2E matrix, health checks, smoke test.
5. **Reproducibility:** Docker Compose.
6. **Auditability:** original evidence + business event history + technical audit + policy version identity.
