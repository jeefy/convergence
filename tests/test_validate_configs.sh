#!/usr/bin/env bash
# tests/test_validate_configs.sh - render every config variant and run the pinned
# agentgateway image's --validate-only against it (local compose and Kubernetes).
# Needs docker or podman; does not start the stack.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BIN="${ROOT_DIR}/bin/convergence"
AGW_IMAGE="${AGW_IMAGE:-cr.agentgateway.dev/agentgateway:v1.6.0}"
ENGINE="${ENGINE:-$(command -v docker >/dev/null 2>&1 && echo docker || echo podman)}"

[ -x "$BIN" ] || (cd "$ROOT_DIR" && make build)

ORIG_HOME="$HOME"
export XDG_DATA_HOME="${XDG_DATA_HOME:-$ORIG_HOME/.local/share}"
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-$ORIG_HOME/.config}"
WORK="$(mktemp -d -t convergence-validate-XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

FAKE_GH_TOKEN="ghp_$(printf 'a%.0s' $(seq 36))"

fail() { echo "ERROR: $*" >&2; exit 1; }

validate() { # validate <name> <config.yaml>  (unset $VARs get dummy values: agentgateway refuses unset ones)
  local name="$1" cfg="$2" envf="$WORK/${1//\//-}.env"
  : >"$envf"
  { grep -o '\${\?[A-Z][A-Z0-9_]*' "$cfg" || true; } | sort -u | tr -d '${' | while read -r v; do echo "$v=dummy" >>"$envf"; done
  chmod 644 "$cfg"
  echo "==> validate: $name"
  "$ENGINE" run --rm --env-file "$envf" -v "$cfg":/c/config.yaml:ro,Z "$AGW_IMAGE" -f /c/config.yaml --validate-only \
    || fail "agentgateway rejected the $name config"
}

local_variant() { # local_variant <name> <init args...>
  local name="$1"; shift
  export HOME="$WORK/home-$name"
  mkdir -p "$HOME"
  "$BIN" init --yes --engine "$ENGINE" "$@" >/dev/null
  validate "local/$name" "$HOME/.convergence/agentgateway.yaml"
  export HOME="$ORIG_HOME"
}

local_variant ollama --provider ollama --ollama-url http://host.docker.internal:11434
local_variant direct --provider direct --openai-key sk-test --anthropic-key sk-ant-test
local_variant copilot --provider copilot --copilot-token "$FAKE_GH_TOKEN"
local_variant fail-open --provider ollama --ollama-url http://host.docker.internal:11434 --fail-open
local_variant otel --provider ollama --ollama-url http://host.docker.internal:11434 --otel-endpoint collector:4317 --otel-sample 0.5
local_variant otel-http --provider ollama --ollama-url http://host.docker.internal:11434 --otel-endpoint collector:4318 --otel-protocol http
printf 'Prefer small commits.\nkey: value # looks like yaml\n' >"$WORK/extra-prompt.txt"
local_variant system-prompt --provider ollama --ollama-url http://host.docker.internal:11434 \
  --system-prompt --system-prompt-file "$WORK/extra-prompt.txt"
# --system-prompt=false on a re-run turns the saved setting off again
export HOME="$WORK/home-system-prompt"
"$BIN" init --yes --engine "$ENGINE" --provider ollama --ollama-url http://host.docker.internal:11434 >/dev/null
grep -q 'transformation:' "$HOME/.convergence/agentgateway.yaml" || fail "re-init without flags dropped the system prompt"
"$BIN" init --yes --engine "$ENGINE" --provider ollama --ollama-url http://host.docker.internal:11434 --system-prompt=false >/dev/null
if grep -q 'transformation:' "$HOME/.convergence/agentgateway.yaml"; then fail "--system-prompt=false did not turn the system prompt off"; fi
export HOME="$ORIG_HOME"
local_variant request-log-db --provider ollama --ollama-url http://host.docker.internal:11434 \
  --request-log-db 'postgres://u:pw@db:5432/ag'

# passthrough models (models.yaml) on top of a copilot render
export HOME="$WORK/home-passthrough"; mkdir -p "$HOME/.convergence"
cat >"$HOME/.convergence/models.yaml" <<'Y'
passthrough:
  fable: {provider: copilot, model: claude-fable-5}
  glm: {provider: ollama, model: glm-5.2}
Y
"$BIN" init --yes --engine "$ENGINE" --provider copilot --copilot-token "$FAKE_GH_TOKEN" \
  --ollama-url http://host.docker.internal:11434 >/dev/null
grep -q 'name: fable' "$HOME/.convergence/agentgateway.yaml" || fail "passthrough model not rendered"
validate local/passthrough "$HOME/.convergence/agentgateway.yaml"
# the same passthrough models with the system prompt transformation on them
"$BIN" init --yes --engine "$ENGINE" --provider copilot --copilot-token "$FAKE_GH_TOKEN" \
  --ollama-url http://host.docker.internal:11434 --system-prompt >/dev/null
grep -A6 'name: fable' "$HOME/.convergence/agentgateway.yaml" | grep -q 'transformation:' || fail "system prompt not rendered on passthrough model"
validate local/passthrough-system-prompt "$HOME/.convergence/agentgateway.yaml"
# two Claude subscription accounts, one per model name
cat >"$HOME/.convergence/models.yaml" <<'Y'
passthrough:
  claude-lf-sonnet: {provider: anthropic, model: claude-sonnet-4-5, api_key_env: CLAUDE_LF_TOKEN, auth: oauth}
  claude-jeefy-sonnet: {provider: anthropic, model: claude-sonnet-4-5, api_key_env: CLAUDE_JEEFY_TOKEN, auth: oauth}
Y
CLAUDE_LF_TOKEN=lf CLAUDE_JEEFY_TOKEN=jeefy "$BIN" init --yes --engine "$ENGINE" --provider copilot --copilot-token "$FAKE_GH_TOKEN" >/dev/null
grep -q 'anthropic-beta' "$HOME/.convergence/agentgateway.yaml" || fail "oauth passthrough not rendered"
validate local/claude-oauth "$HOME/.convergence/agentgateway.yaml"
cp "$HOME/.convergence/models.yaml" "$WORK/claude-models.yaml"
export HOME="$ORIG_HOME"

# Kubernetes: extract the agentgateway config from the rendered Secret
k8s_variant() { # k8s_variant <name> <cluster install args...>
  local name="$1"; shift
  local out="$WORK/k8s-$name.yaml"
  "$BIN" cluster install --dry-run "$@" >"$out"
  python3 - "$out" "$WORK/k8s-$name-config.yaml" <<'P'
import sys, yaml
for d in yaml.safe_load_all(open(sys.argv[1])):
    if d and d.get("kind") == "Secret":
        for v in (d.get("stringData") or {}).values():
            if "gateways:" in v:
                open(sys.argv[2], "w").write(v); sys.exit(0)
sys.exit("no agentgateway config found in rendered Secret")
P
  validate "k8s/$name" "$WORK/k8s-$name-config.yaml"
}

k8s_variant copilot --provider copilot --copilot-token "$FAKE_GH_TOKEN"
k8s_variant direct --provider direct --openai-key sk-test
k8s_variant fail-open --provider direct --openai-key sk-test --fail-open
k8s_variant otel --provider direct --openai-key sk-test --otel-endpoint collector:4317
k8s_variant system-prompt --provider direct --openai-key sk-test --system-prompt --system-prompt-file "$WORK/extra-prompt.txt"
k8s_variant claude-oauth --provider copilot --copilot-token "$FAKE_GH_TOKEN" --models-file "$WORK/claude-models.yaml"

echo "==> all configs validated"
