# Docker Compose Deployment

This deployment is intended for local evaluation and reproducible demos.

## Design

Docker Compose runs:

- PostgreSQL 16;
- Expense Agent Go API;
- React frontend served by Nginx.

Ollama runs on the host machine.

Keeping Ollama on the host is intentional for the recommended macOS/Apple Silicon setup because native Ollama can use Metal acceleration. The API container reaches it through `host.docker.internal`.

```text
Browser
   |
   v
Frontend / Nginx :5173
   |
   +---- /api/* and /healthz ----> Go API :8080
                                      |
                                      +---- PostgreSQL
                                      |
                                      +---- host Ollama :11434
```

## Prerequisites

- Docker Desktop / Docker Engine with Compose v2
- Ollama running on the host
- `qwen3-vl:4b-instruct`
- `qwen3-vl:8b-instruct`

Install/pull:

```bash
ollama pull qwen3-vl:4b-instruct
ollama pull qwen3-vl:8b-instruct
```

Verify:

```bash
ollama list
curl http://localhost:11434/api/tags
```

## Preflight

```bash
./scripts/docker-preflight.sh
```

Expected:

```text
Host Ollama and required vision models are ready.
Docker Compose is ready.
```

## Start

```bash
docker compose up --build -d
```

Check:

```bash
docker compose ps
```

Expected services:

```text
postgres  healthy
api       healthy
frontend  healthy
```

## Smoke test

```bash
./scripts/docker-smoke-test.sh
```

It validates:

- API health;
- active policy;
- frontend availability.

## Endpoints

| Service | URL |
|---|---|
| Frontend | `http://localhost:5173` |
| API | `http://localhost:8080` |
| Host Ollama | `http://localhost:11434` |
| PostgreSQL | internal Compose network only |

## PostgreSQL access

The database port is intentionally not published to the host.

Use:

```bash
docker compose exec postgres \
  psql -U expense_agent -d expense_agent
```

## Persistence

Named volumes:

```text
postgres_data
expense_data
```

`postgres_data` contains business state and policies.

`expense_data` contains receipt evidence and technical audit files.

A normal stop preserves volumes:

```bash
docker compose down
```

## Clean reproducible reset

```bash
docker compose down -v
docker compose up --build -d
```

This removes the Compose database/evidence volumes and recreates the application from the baseline.

The API schema initialization seeds the default published V1 policy when no published policy exists.

**Warning:** `docker compose down -v` permanently deletes the Compose volumes.

## Logs

All:

```bash
docker compose logs -f
```

API:

```bash
docker compose logs -f api
```

PostgreSQL:

```bash
docker compose logs -f postgres
```

Frontend:

```bash
docker compose logs -f frontend
```

## Rebuild

```bash
docker compose up --build -d
```

No-cache rebuild:

```bash
docker compose build --no-cache
docker compose up -d
```

## Ollama endpoint

Compose sets the API default to:

```text
OLLAMA_BASE_URL=http://host.docker.internal:11434
```

The Go client converts the base URL to `/api/chat`.

A complete endpoint can also be supplied through `OLLAMA_URL`.

## Docker-to-host troubleshooting

Check host:

```bash
curl http://localhost:11434/api/tags
```

Check from a temporary container:

```bash
docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  curlimages/curl:8.12.1 \
  -fsS http://host.docker.internal:11434/api/tags
```

## Production scope

The Compose configuration is a reproducible local evaluation deployment.

For production, add:

- authentication/RBAC;
- TLS;
- secret management;
- managed PostgreSQL;
- encrypted object storage;
- authenticated inference;
- backups;
- centralized immutable audit retention;
- rate limiting/observability.
