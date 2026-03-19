#!/usr/bin/env bash
set -euo pipefail

COMPOSE_FILE="docker-compose.e2e.yml"
API_BASE_URL="http://127.0.0.1:9877"
RETRY_TIMEOUT=90
RETRY_INTERVAL=2

if ! command -v docker >/dev/null 2>&1; then
  echo "[e2e] docker: not found in PATH"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "[e2e] docker compose: not available"
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "[e2e] curl: not found in PATH"
  exit 1
fi

cleanup() {
  local cleanup_timeout=30
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans \
    >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo "[e2e] building and starting service"
if ! docker compose -f "$COMPOSE_FILE" up -d --build; then
  echo "[e2e] failed to start container"
  exit 1
fi

echo "[e2e] waiting for API readiness (max ${RETRY_TIMEOUT}s)"
elapsed=0
while [[ "$elapsed" -lt "$RETRY_TIMEOUT" ]]; do
  if curl -fsS "$API_BASE_URL/api/server" >/dev/null 2>&1; then
    echo "[e2e] API ready after ${elapsed}s"
    break
  fi
  
  echo "[e2e]   still waiting... (${elapsed}s)"
  sleep "$RETRY_INTERVAL"
  elapsed=$((elapsed + RETRY_INTERVAL))
done

if [[ "$elapsed" -ge "$RETRY_TIMEOUT" ]]; then
  echo "[e2e] API did not become ready within ${RETRY_TIMEOUT}s; dumping logs:"
  docker compose -f "$COMPOSE_FILE" logs goaway-e2e || true
  exit 1
fi

echo "[e2e] running smoke tests"
echo "[e2e] [✓] GET /api/server"
curl -fsS "$API_BASE_URL/api/server" | grep -q '"version"' || {
  echo "[e2e] [✗] /api/server did not contain version field"
  exit 1
}

echo "[e2e] [✓] GET /api/dnsMetrics"
curl -fsS "$API_BASE_URL/api/dnsMetrics" | grep -q '"total"' || {
  echo "[e2e] [✗] /api/dnsMetrics did not contain total field"
  exit 1
}

echo "[e2e] [✓] GET /metrics (Prometheus)"
curl -fsS "$API_BASE_URL/metrics" | grep -q '^# HELP' || {
  echo "[e2e] [✗] /metrics endpoint did not return valid Prometheus format"
  exit 1
}

echo "[e2e] ✓✓✓ all smoke checks passed"
