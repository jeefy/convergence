#!/usr/bin/env bash
# tests/test_e2e_local.sh - End-to-End Local Smoke Test for Convergence
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${ROOT_DIR}/bin/convergence"

# Ensure binary exists
if [ ! -x "$BIN" ]; then
  echo "Building convergence binary..."
  (cd "$ROOT_DIR" && make build)
fi

TEST_HOME="$(mktemp -d -t convergence-e2e-XXXXXX)"
export HOME="$TEST_HOME"

cleanup() {
  echo "==> Cleaning up test containers and temp directory..."
  "$BIN" stop || true
  rm -rf "$TEST_HOME"
}
trap cleanup EXIT

echo "==> Running 'convergence init' with mock Direct API keys..."
"$BIN" init << 'EOF'
2
sk-ant-mock-key-for-testing
sk-mock-key-for-testing
EOF

echo "==> Validating rendered AgentGateway configuration with container engine..."
CONFIG_DIR="${TEST_HOME}/.convergence"
chmod 755 "$CONFIG_DIR"
chmod 644 "$CONFIG_DIR"/*.yaml

ENGINE="docker"
if ! command -v docker >/dev/null 2>&1 && command -v podman >/dev/null 2>&1; then
  ENGINE="podman"
fi

echo "Using engine: $ENGINE"
$ENGINE run --rm -v "${CONFIG_DIR}":/c:Z cr.agentgateway.dev/agentgateway:v1.6.0 -f /c/agentgateway.yaml --validate-only

echo "==> Starting Convergence stack in background..."
"$BIN" start

echo "==> Waiting for services to become healthy..."
MAX_ATTEMPTS=30
ATTEMPT=0
HEALTHY=false

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
  if curl -sf http://localhost:3000/v1/models >/dev/null 2>&1; then
    HEALTHY=true
    break
  fi
  ATTEMPT=$((ATTEMPT + 1))
  sleep 2
done

if [ "$HEALTHY" = "false" ]; then
  echo "ERROR: AgentGateway failed to become healthy within 60s"
  "$BIN" status || true
  exit 1
fi

echo "==> Checking 'convergence status'..."
"$BIN" status

echo "==> Testing Pinball ext_authz endpoint directly (:9201)..."
PINBALL_RESP="$(curl -sf -X POST http://localhost:9201/check \
  -H "Content-Type: application/json" \
  -d '{"model":"auto","messages":[{"role":"user","content":"How do I debug this distributed security race condition?"}]}')"

echo "Pinball check response: $PINBALL_RESP"
if ! echo "$PINBALL_RESP" | grep -q "model"; then
  echo "ERROR: Unexpected Pinball check response"
  exit 1
fi

echo "==> Testing Stand-In metrics endpoint directly (:9102)..."
STANDIN_METRICS="$(curl -sf http://localhost:9102/metrics)"
if ! echo "$STANDIN_METRICS" | grep -q "standin_requests_total"; then
  echo "ERROR: Stand-In metrics not reachable or missing standin_requests_total"
  exit 1
fi

echo "==> Checking 'convergence stats'..."
"$BIN" stats

echo "==> Stopping Convergence stack..."
"$BIN" stop

echo "==> Local E2E smoke test PASSED!"
