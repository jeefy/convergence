#!/usr/bin/env bash
# tests/test_e2e_cluster.sh - Cluster Manifests and Dry-Run E2E Test
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${ROOT_DIR}/bin/convergence"

if [ ! -x "$BIN" ]; then
  (cd "$ROOT_DIR" && make build)
fi

echo "==> Testing 'convergence cluster install --dry-run'..."
OUTPUT="$("$BIN" cluster install --dry-run --namespace=convergence-test)"

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

echo "==> Cluster manifests E2E dry-run test PASSED!"
