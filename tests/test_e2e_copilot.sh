#!/usr/bin/env bash
# tests/test_e2e_copilot.sh - E2E verification of real Copilot integration via Convergence.
# Prerequisites: user has authorized Copilot (via `convergence login copilot` or COPILOT_TOKEN env var).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${ROOT_DIR}/bin/convergence"
GW=3000; STANDIN=9102; PINBALL=9201

[ -x "$BIN" ] || { echo "Building convergence binary..."; (cd "$ROOT_DIR" && make build); }

fail() { echo "ERROR: $*" >&2; exit 1; }

metric() { # metric <port> <name> -> sum of all non-bucket series
  curl -sf "http://127.0.0.1:$1/metrics" | awk -v n="$2" '$0 !~ /^#/ { split($1,a,"{"); if (a[1]==n) s+=$NF } END{printf "%d", s+0}'
}

wait_for() { # wait_for <desc> <tries> <cmd...>
  local desc="$1" tries="$2"; shift 2
  for _ in $(seq "$tries"); do "$@" >/dev/null 2>&1 && return 0; sleep 2; done
  fail "timed out waiting for $desc"
}

# 1. Check if token is available
COPILOT_TOKEN="${COPILOT_TOKEN:-${GH_TOKEN:-}}"
if [ -z "$COPILOT_TOKEN" ] && [ -f "$HOME/.convergence/.env" ]; then
  COPILOT_TOKEN="$(grep -E '^(COPILOT_TOKEN|GH_TOKEN)=' "$HOME/.convergence/.env" | head -n1 | cut -d= -f2- || true)"
fi

if [ -z "$COPILOT_TOKEN" ]; then
  fail "No Copilot token found. Run 'convergence login copilot' first or set COPILOT_TOKEN."
fi

echo "==> Ensuring Convergence is initialized with Copilot provider"
if [ ! -f "$HOME/.convergence/config.yaml" ]; then
  "$BIN" init --yes --provider copilot --copilot-token "$COPILOT_TOKEN"
fi

echo "==> Ensuring stack is running"
if ! curl -sf "http://127.0.0.1:$GW/v1/models" >/dev/null 2>&1; then
  "$BIN" start
fi

wait_for "gateway /v1/models" 30 curl -sf "http://127.0.0.1:$GW/v1/models"
wait_for "stand-in metrics" 30 curl -sf "http://127.0.0.1:$STANDIN/metrics"
wait_for "pinball metrics" 30 curl -sf "http://127.0.0.1:$PINBALL/metrics"

echo "==> Running convergence status"
"$BIN" status

echo "==> Baseline metrics"
base_matches="$(metric $STANDIN standin_matches_total)"
base_dec="$(metric $PINBALL pinball_decisions_total)"

echo "==> Sending test prompt through gateway (testing Pinball tier routing + Stand-In scrubbing)"
TOKEN="$(python3 -c 'import random, string; print("ghp_" + "".join(random.choices(string.ascii_letters + string.digits, k=36)))')"

RESP="$(TOKEN="$TOKEN" GW="$GW" python3 -c '
import urllib.request, json, os, sys

token = os.environ["TOKEN"]
gw = os.environ["GW"]
body = json.dumps({
    "model": "auto",
    "max_tokens": 16,
    "messages": [{"role": "user", "content": f"hello {token}"}]
}).encode()
req = urllib.request.Request(f"http://127.0.0.1:{gw}/v1/chat/completions", data=body, headers={"Content-Type": "application/json"})
with urllib.request.urlopen(req, timeout=30) as r:
    sys.stdout.write(r.read().decode())
')"

echo "Response received: $RESP"
echo "$RESP" | grep -q '"choices"' || fail "Invalid chat completions response from gateway: $RESP"

echo "==> Checking that metrics incremented"
sleep 1
curr_matches="$(metric $STANDIN standin_matches_total)"
curr_dec="$(metric $PINBALL pinball_decisions_total)"

[ "$curr_matches" -ge $((base_matches + 1)) ] || fail "standin_matches_total did not increase ($base_matches -> $curr_matches)"
[ "$curr_dec" -ge $((base_dec + 1)) ] || fail "pinball_decisions_total did not increase ($base_dec -> $curr_dec)"

echo "==> Running doctor (standard checks)"
"$BIN" doctor

echo "==> Summary stats"
"$BIN" stats

echo; echo "==> Copilot E2E test PASSED!"
