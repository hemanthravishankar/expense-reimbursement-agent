# Docker Compose Deployment

This deployment is intended for local evaluation and reproducible demos.

## Design

Docker Compose runs:

- PostgreSQL
- Expense Agent Go API
- React frontend served by Nginx

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

The frontend uses same-origin API requests in the production build. Nginx proxies those calls to the API container, so no browser-side container hostname is exposed.

## Prerequisites

Required:

- Docker Desktop / Docker Engine with Compose v2
- Ollama running on the host
- `qwen3-vl:4b-instruct`
- `qwen3-vl:8b-instruct`

Install/pull the models:

```bash
ollama pull qwen3-vl:4b-instruct
ollama pull qwen3-vl:8b-instruct
```

Verify Ollama:

```bash
ollama list
curl http://localhost:11434/api/tags
```

## 1. Preflight

From the repository root:

```bash
./scripts/docker-preflight.sh
```

Expected:

```text
Host Ollama and required vision models are ready.
Docker Compose is ready.
```

## 2. Start the stack

```bash
docker compose up --build -d
```

Watch startup:

```bash
docker compose ps
docker compose logs -f api
```

The API automatically creates its PostgreSQL schema and seeds the default published reimbursement policy when the database is empty.

## 3. Verify

```bash
./scripts/docker-smoke-test.sh
```

Or manually:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/api/v1/admin/policies/active
```

Open:

```text
http://localhost:5173
```

## Service endpoints

| Service | URL |
|---|---|
| Frontend | `http://localhost:5173` |
| API | `http://localhost:8080` |
| Host Ollama | `http://localhost:11434` |
| PostgreSQL | internal Compose network only |

PostgreSQL is intentionally not published to a host port. Use:

```bash
docker compose exec postgres \
  psql -U expense_agent -d expense_agent
```

## Data persistence

Two named volumes are used:

```text
postgres_data
expense_data
```

`postgres_data` stores:

- claims
- report lifecycle
- Finance events
- policy versions/rules

`expense_data` stores:

- original uploaded evidence
- technical audit JSON

A normal stop preserves the data:

```bash
docker compose down
```

## Clean reset

For a completely clean demo/evaluation database and evidence store:

```bash
docker compose down -v
docker compose up --build -d
```

Because the application initializes its own schema and default policy, the system returns to a reproducible baseline automatically.

**Warning:** `docker compose down -v` permanently removes the Compose database and evidence volumes.

## Logs

All services:

```bash
docker compose logs -f
```

API only:

```bash
docker compose logs -f api
```

PostgreSQL only:

```bash
docker compose logs -f postgres
```

Frontend/Nginx:

```bash
docker compose logs -f frontend
```

## Rebuild after source changes

Backend or frontend source changes:

```bash
docker compose up --build -d
```

Force a clean image rebuild:

```bash
docker compose build --no-cache
docker compose up -d
```

## Ollama connectivity troubleshooting

The API container uses:

```text
OLLAMA_BASE_URL=http://host.docker.internal:11434
```

Check host Ollama first:

```bash
curl http://localhost:11434/api/tags
```

Then test Docker-to-host connectivity:

```bash
docker run --rm \
  --add-host=host.docker.internal:host-gateway \
  curlimages/curl:8.12.1 \
  -fsS http://host.docker.internal:11434/api/tags
```

If host Ollama is intentionally running on another address:

```bash
cp .env.docker.example .env.docker
```

Edit `OLLAMA_BASE_URL`, then start Compose with:

```bash
docker compose --env-file .env.docker up --build -d
```

## Security scope

The Compose configuration is for local evaluation/demo use.

For production:

- use secret management instead of demo database credentials
- add authentication/authorization
- use TLS
- use managed PostgreSQL
- use object storage for receipt evidence
- use an authenticated inference endpoint
- restrict Finance/admin routes
- add backups and immutable centralized audit retention

Those production controls are intentionally outside the local evaluation deployment.
