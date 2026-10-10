# Technology Stack

## Stack overview

| Layer | Technology | Responsibility |
|---|---|---|
| Web UI | React + Vite | Employee submission/history and Finance control center |
| Web serving | Nginx | Production static hosting and `/api` reverse proxy |
| REST API | Go | HTTP layer, services, orchestration |
| Workflow | Go | Receipt lifecycle and state transitions |
| Validation | Go | Required fields, money arithmetic, dates, line-item consistency |
| Duplicate/risk | Go | Hash matching, metadata matching, strong transaction identity |
| Policy | Go + PostgreSQL | Deterministic rules and version management |
| Business persistence | PostgreSQL 16 | Claims, reports, events, policy versions/rules |
| Vision inference | Ollama | Local model serving |
| Primary vision model | `qwen3-vl:4b-instruct` | Fast first-pass structured extraction |
| Fallback vision model | `qwen3-vl:8b-instruct` | Fresh extraction after validation failure |
| Evidence store | Local `DocumentStore` | Original uploaded receipts/invoices |
| PDF processing | Poppler / `pdftoppm` | Converts PDF pages into image inputs |
| Technical audit | JSON files | Processing, model, validation, duplicate, policy evidence |
| Deployment | Docker Compose | Reproducible local evaluation |
| Tests | Go `testing`, Vite build, ESLint, smoke scripts | Reliability and regression |

## Why Go

The financially sensitive parts are deterministic and explicit:

- validation;
- duplicate routing;
- policy evaluation;
- state transitions;
- Finance actions;
- audit/event writes.

Go gives the project a small dependency surface, compile-time types, straightforward concurrency options, and simple container builds.

## Why local Ollama

For the demo architecture, local Ollama offers:

- no dependency on an external AI API;
- receipt images can stay on the local machine;
- model behavior is reproducible from named local models;
- Apple Silicon can use native Metal acceleration when Ollama remains on the host.

The Dockerized API can point to another inference endpoint later through `OLLAMA_BASE_URL` / `OLLAMA_URL`.

## Why PostgreSQL

PostgreSQL holds mutable business state:

- claims;
- report membership;
- claim events;
- report status;
- policy versions;
- policy rules.

Receipt evidence and technical audit data are deliberately separate from business tables.

## Why Nginx in the Compose build

Nginx serves the frontend build and proxies:

```text
/api/*   -> Go API
/healthz -> Go API
```

This gives the browser a same-origin production path while the development server can continue using `http://localhost:8080`.

## Why Docker Compose instead of Kubernetes for the submission

The goal is judge reproducibility, not infrastructure complexity. Compose makes the required application services easy to build and run with a small number of commands.

Kubernetes/Helm would be a valid production evolution, but it would add deployment surface without improving the core evaluation of the agent.
