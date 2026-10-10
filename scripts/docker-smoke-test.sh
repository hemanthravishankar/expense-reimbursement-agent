#!/usr/bin/env bash
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
UI_URL="${UI_URL:-http://localhost:5173}"

echo "Checking API health ..."
curl -fsS "${API_URL}/healthz"
echo

echo "Checking active policy ..."
curl -fsS "${API_URL}/api/v1/admin/policies/active"
echo

echo "Checking frontend ..."
curl -fsS -o /dev/null "${UI_URL}/"

echo "Smoke test passed."
echo "Frontend: ${UI_URL}"
echo "API:      ${API_URL}"
