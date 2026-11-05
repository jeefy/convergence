# Convergence

Convergence is a unified, local and in-cluster orchestration harness for **AgentGateway**, [**Stand-In**](https://github.com/jeefy/stand-in) (PII & credential scrubbing), and [**Pinball**](https://github.com/jeefy/pinball) (intelligent multi-tier model routing and optimization).

It exposes an OpenAI-compatible `/v1` endpoint on port `3000` (with automatic Anthropic Messages translation) that scrubs credentials on the wire and dynamically routes requests to the optimal model tier.

```
Client (OpenCode, Claude Code, Aider, Cursor)
                     │ HTTP :3000
                     ▼
             AgentGateway v1.6
              │             ▲
   (ext_proc) │             │ (ext_authz)
   Sanitize   ▼             ▼ Model Tier
     [Stand-In]          [Pinball]
    (Secret/PII)         (Scorer)
   github.com/jeefy/   github.com/jeefy/
       stand-in             pinball
              │             ▲
              └──────┬──────┘
                     ▼ Upstream :4141 / Cloud
          Copilot / Direct / Ollama
```

---

## Architecture & Ecosystem

Convergence weaves together:
- **AgentGateway (`v1.6.0`)**: High-performance Envoy-based AI gateway providing provider translation, unified access logs, and rate/cost attribution.
- [**jeefy/stand-in**](https://github.com/jeefy/stand-in) (`0.4.1`): Transparent Envoy `ext_proc` service that tokenizes API keys and sensitive tokens on ingress (`<<secret-...>>`) and reconstitutes them on egress.
- [**jeefy/pinball**](https://github.com/jeefy/pinball) (`0.7.2`): Envoy `ext_authz` autopilot that inspects sanitized prompts and assigns optimal model tiers (`light`, `standard`, `deep`).
- **Periodic Optimization & Judgement**: Pinball evaluates traffic patterns, replays historical requests across candidate models, and calculates within-tier re-rankings to continually minimize spend and latency. Defaults to a weekly evaluation schedule (`0 3 * * 0`, configurable).

---

## Quickstart

### Installation

```bash
curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
```

Or build from source:

```bash
git clone https://github.com/jeefy/convergence.git
cd convergence
make build
sudo install -m 755 bin/convergence /usr/local/bin/
```

### Local Setup

```bash
# Interactive setup (selects engine: docker/podman, and provider: copilot/direct/ollama)
convergence init

# Start the stack (AgentGateway, Stand-In, Pinball)
convergence start

# Check health and view live stats
convergence status
convergence stats

# Trigger Pinball usage evaluation & re-ranking check (dry-run or live)
convergence optimize --dry-run

# Stop the stack
convergence stop
```

---

## Client Auto-Configuration

Point your agentic harnesses and tools to Convergence with a single command. Each command creates an automatic backup (`.convergence.bak`) and supports clean rollback via `restore`.

```bash
# Developer CLIs
convergence config opencode              # Point OpenCode (~/.config/opencode/opencode.json)
convergence config opencode restore      # Restore original OpenCode config

convergence config claude-code           # Point Claude Code (~/.claude/settings.json)
convergence config claude-code restore   # Restore original Claude Code settings

convergence config aider                 # Point Aider (~/.aider.conf.yml)
convergence config aider restore         # Restore original Aider config

convergence config oh-my-opencode        # Switch oh-my-opencode profile to Pinball
convergence config oh-my-opencode restore

convergence config goose                 # Point Goose (~/.config/goose/config.yaml)
convergence config goose restore

convergence config hermes                # Point Hermes Agent (~/.hermes/config.yaml)
convergence config hermes restore

# IDEs
convergence config cursor                # Point Cursor OpenAI base URL
convergence config cursor restore

convergence config continue              # Add Convergence provider to Continue.dev
convergence config continue restore

# Shell Exports
convergence config copilot               # Print OPENAI_* and ANTHROPIC_* exports
```

---

## Kubernetes Cluster Deployment

Deploy Convergence directly into your Kubernetes cluster:

```bash
# Preview rendered manifests
convergence cluster install --dry-run --namespace=convergence

# Install to cluster
convergence cluster install --namespace=convergence --provider=copilot

# Check rollout status
convergence cluster status --namespace=convergence
```

---

## CLI Reference

| Command | Description |
|---|---|
| `convergence init` | Run interactive or flag-based configuration |
| `convergence start` | Start containers (docker/podman compose) |
| `convergence stop` | Stop and remove running containers |
| `convergence restart` | Restart all Convergence containers |
| `convergence status` | Show container health and listening ports |
| `convergence logs [-f]` | Tail container logs |
| `convergence stats` | Display request telemetry, token usage, and cost estimates |
| `convergence optimize [--dry-run]` | Run Pinball model evaluation and within-tier re-ranking |
| `convergence cluster install` | Deploy Convergence manifests to Kubernetes (includes weekly CronJob) |
| `convergence cluster status` | Inspect in-cluster deployment health |
| `convergence config <client> [restore]` | Configure or restore external client applications |
| `convergence version` | Display version, commit, and build date |

---

## License

Apache-2.0
