#!/usr/bin/env bash
set -euo pipefail

OLLAMA_HOST_URL="${OLLAMA_HOST_URL:-http://localhost:11434}"
REQUIRED_MODELS=(
  "qwen3-vl:4b-instruct"
  "qwen3-vl:8b-instruct"
)

for command in docker curl; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "ERROR: '$command' is required but was not found."
    exit 1
  fi
done

if ! docker compose version >/dev/null 2>&1; then
  echo "ERROR: Docker Compose v2 ('docker compose') is required."
  exit 1
fi

echo "Checking host Ollama at ${OLLAMA_HOST_URL} ..."

tags_json="$(curl -fsS "${OLLAMA_HOST_URL}/api/tags" || true)"

if [[ -z "$tags_json" ]]; then
  cat <<EOF
ERROR: Ollama is not reachable at ${OLLAMA_HOST_URL}.

Start Ollama, then retry.

Typical checks:
  ollama list
  curl ${OLLAMA_HOST_URL}/api/tags
EOF
  exit 1
fi

for model in "${REQUIRED_MODELS[@]}"; do
  if ! printf '%s' "$tags_json" | grep -Fq "\"name\":\"${model}\""; then
    echo "ERROR: Required Ollama model is missing: ${model}"
    echo "Run:"
    echo "  ollama pull ${model}"
    exit 1
  fi
done

echo "Host Ollama and required vision models are ready."
echo "Docker Compose is ready."
