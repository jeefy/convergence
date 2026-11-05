#!/usr/bin/env bash
# tests/test_e2e_cluster.sh - Cluster Manifests and Live KinD E2E Test
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${ROOT_DIR}/bin/convergence"
NAMESPACE="convergence-e2e"

if [ ! -x "$BIN" ]; then
  (cd "$ROOT_DIR" && make build)
fi

echo "==> Testing 'convergence cluster install --dry-run'..."
OUTPUT="$("$BIN" cluster install --dry-run --namespace="${NAMESPACE}")"

# Verify key resources exist in rendered YAML
for resource in "Namespace" "Secret" "ConfigMap" "Deployment" "Service"; do
  if ! echo "$OUTPUT" | grep -q "kind: $resource"; then
    echo "ERROR: Rendered manifests missing resource kind: $resource"
    exit 1
  fi
done

if command -v kubectl >/dev/null 2>&1; then
  echo "==> Validating manifests against kubectl client schema..."
  echo "$OUTPUT" | kubectl apply --dry-run=client -f -
fi

# Check if a live cluster is reachable
if command -v kubectl >/dev/null 2>&1 && kubectl cluster-info >/dev/null 2>&1; then
  echo "==> Live Kubernetes cluster detected! Running in-cluster E2E test..."

  cleanup_cluster() {
    echo "==> Cleaning up namespace ${NAMESPACE}..."
    kubectl delete namespace "${NAMESPACE}" --ignore-not-found=true || true
  }
  trap cleanup_cluster EXIT

  echo "==> Deploying Convergence via 'convergence cluster install'..."
  "$BIN" cluster install --namespace="${NAMESPACE}" --provider=direct --openai-key=dummy-test-key --anthropic-key=dummy-test-key

  echo "==> Waiting for Deployments to become available in namespace ${NAMESPACE}..."
  kubectl -n "${NAMESPACE}" rollout status deployment/stand-in --timeout=120s
  kubectl -n "${NAMESPACE}" rollout status deployment/pinball --timeout=120s
  kubectl -n "${NAMESPACE}" rollout status deployment/agentgateway --timeout=180s

  echo "==> Checking 'convergence cluster status'..."
  "$BIN" cluster status --namespace="${NAMESPACE}"

  echo "==> Testing in-cluster Service endpoints..."
  # Port-forward and probe Pinball :9201 and AgentGateway :3000
  kubectl -n "${NAMESPACE}" port-forward svc/pinball 19201:9201 &
  PF_PINBALL_PID=$!

  kubectl -n "${NAMESPACE}" port-forward svc/agentgateway 13000:3000 &
  PF_GATEWAY_PID=$!

  sleep 3

  echo "==> Testing Pinball health via port-forward..."
  PINBALL_CHECK="$(curl -sf -X POST http://localhost:19201/check \
    -H "Content-Type: application/json" \
    -d '{"model":"auto","messages":[{"role":"user","content":"Investigate memory leak"}]}' || true)"

  echo "Pinball response: ${PINBALL_CHECK}"
  if ! echo "${PINBALL_CHECK}" | grep -q "model"; then
    echo "ERROR: Pinball in-cluster check failed"
    kill "$PF_PINBALL_PID" "$PF_GATEWAY_PID" || true
    exit 1
  fi

  echo "==> Testing AgentGateway models endpoint via port-forward..."
  GATEWAY_MODELS="$(curl -sf http://localhost:13000/v1/models || true)"
  echo "Gateway models response: ${GATEWAY_MODELS}"
  if ! echo "${GATEWAY_MODELS}" | grep -q "object"; then
    echo "ERROR: AgentGateway in-cluster models endpoint failed"
    kill "$PF_PINBALL_PID" "$PF_GATEWAY_PID" || true
    exit 1
  fi

  kill "$PF_PINBALL_PID" "$PF_GATEWAY_PID" || true
  echo "==> In-cluster KinD test PASSED!"
else
  echo "==> No live Kubernetes cluster detected. Skipped live deployment checks (dry-run only)."
fi

echo "==> Cluster E2E test finished successfully!"
