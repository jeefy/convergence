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

  PF_PIDS=()
  dump_debug() {
    echo "==> FAILURE: dumping cluster state" >&2
    kubectl -n "${NAMESPACE}" get pods -o wide || true
    kubectl -n "${NAMESPACE}" describe pods || true
    for p in $(kubectl -n "${NAMESPACE}" get pods -o name 2>/dev/null); do
      echo "--- logs ${p}" >&2
      kubectl -n "${NAMESPACE}" logs "${p}" --all-containers --tail=100 || true
    done
  }
  cleanup_all() {
    for pid in "${PF_PIDS[@]:-}"; do [ -n "$pid" ] && kill "$pid" 2>/dev/null || true; done
    cleanup_cluster
  }
  on_err() { dump_debug; }
  trap on_err ERR
  trap cleanup_all EXIT

  echo "==> Deploying Convergence via 'convergence cluster install'..."
  "$BIN" cluster install --namespace="${NAMESPACE}" --provider=direct --openai-key=mock-key --anthropic-key=mock-key

  echo "==> Waiting for Deployments..."
  for d in stand-in pinball agentgateway; do
    kubectl -n "${NAMESPACE}" rollout status "deployment/${d}" --timeout=300s
  done
  kubectl -n "${NAMESPACE}" wait --for=condition=Ready pod --all --timeout=300s

  echo "==> Checking 'convergence cluster status'..."
  "$BIN" cluster status --namespace="${NAMESPACE}"

  kubectl -n "${NAMESPACE}" port-forward svc/pinball 19201:9201 >/dev/null 2>&1 &
  PF_PIDS+=($!)
  kubectl -n "${NAMESPACE}" port-forward svc/agentgateway 13000:3000 >/dev/null 2>&1 &
  PF_PIDS+=($!)

  echo "==> Gateway GET /v1/models must return 200..."
  CODE=000
  for _ in $(seq 1 30); do
    CODE="$(curl -s -o /tmp/models.out -w '%{http_code}' http://localhost:13000/v1/models || true)"
    [ "$CODE" = "200" ] && break
    sleep 2
  done
  echo "models status: ${CODE}"
  [ "$CODE" = "200" ]

  echo "==> Pinball /check must return JSON with a 'model' key..."
  PINBALL_CHECK=""
  for _ in $(seq 1 30); do
    PINBALL_CHECK="$(curl -sf -X POST http://localhost:19201/check \
      -H "Content-Type: application/json" \
      -d '{"model":"auto","messages":[{"role":"user","content":"Investigate memory leak"}]}' || true)"
    [ -n "$PINBALL_CHECK" ] && break
    sleep 2
  done
  echo "Pinball response: ${PINBALL_CHECK}"
  echo "${PINBALL_CHECK}" | python3 -c 'import json,sys; assert "model" in json.load(sys.stdin)'

  echo "==> Testing 'convergence cluster uninstall'..."
  trap - ERR
  "$BIN" cluster uninstall --namespace="${NAMESPACE}" --yes
  echo "==> In-cluster KinD test PASSED!"
else
  echo "==> No live Kubernetes cluster detected. Skipped live deployment checks (dry-run only)."
fi

echo "==> Cluster E2E test finished successfully!"
