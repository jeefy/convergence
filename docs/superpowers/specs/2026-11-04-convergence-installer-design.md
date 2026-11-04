# Convergence (AgentGateway + Stand-In + Pinball) Multi-Target Installer Design

- **Date:** 2026-11-04
- **Status:** Approved / Ready for Implementation Planning
- **Author:** Sisyphus & Jeefy
- **Project:** Convergence (`github.com/jeefy/convergence`)

---

## 1. Executive Summary

Convergence is a unified distribution of our AI gateway and autopilot stack:
1. **AgentGateway** (`v1.6.0`): High-performance LLM proxy with multi-listener routing, cost-catalog logging, and metrics.
2. **Stand-In** (`v0.4.1`): Envoy `ext_proc` processor enforcing secret tokenization (`<<secret-...>>`) on ingress and original credential restoration on egress.
3. **Pinball** (`v0.6.1`): `ext_authz` intelligent model router scoring prompt signals (tokens, tools, keywords, code) to steer `auto`/`pinball` requests to cost-optimized model tiers.
4. **Copilot-API** (Optional): Local proxy bridging GitHub Copilot subscriptions to OpenAI-compatible endpoints.

Convergence delivers a single Go CLI binary (`convergence`) and a one-liner install script (`curl -fsSL https://.../install.sh | bash`) supporting two primary runtime targets:
- **Local Target (`docker`/`podman`):** macOS and Linux workstations with zero separate DB container requirements and local file-backed persistence.
- **Cluster Target (`kubernetes`):** Single-command Kubernetes cluster deployments (`convergence cluster install`) supporting built-in DB or external Postgres DSNs, network policies, and Prometheus/Grafana service monitors.

---

## 2. Delivery & Packaging Architecture

### 2.1 The One-Liner Script
A cross-platform bootstrap script:
```bash
curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
```
The script performs:
1. OS and Architecture detection (`darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`).
2. Release asset download from GitHub Releases.
3. Binary placement into `/usr/local/bin/convergence` (or `~/.local/bin/convergence` with PATH verification).
4. Verification run: `convergence --version`.

Homebrew formula alternative:
```bash
brew install jeefy/tap/convergence
```

### 2.2 CLI Architecture (`convergence`)
A single Go binary compiled with GoReleaser containing embedded templates:
- Embedded compose specifications (`docker-compose.yaml`).
- Embedded AgentGateway, Stand-In, and Pinball configuration defaults.
- Embedded Helm chart / Kubernetes manifests for cluster deployments.

---

## 3. Local Target Design (`convergence init` & `convergence start`)

### 3.1 Local Container Topology
```
                             Client (OpenCode / Cursor / Hermes / CLI)
                                                │
                                                ▼
                             ┌─────────────────────────────────────┐
                             │    AgentGateway (:3000 / :4002)     │
                             └──────┬──────────────────────┬───────┘
                                    │                      │
             ext_proc (Ingress/Egress)                     ▼   ext_authz (Request)
        ┌───────────────────────────┐             ┌───────────────────────────┐
        │  Stand-In (:9101)         │             │  Pinball (:9201)          │
        │  Scrub secrets on ingress,│             │  Scores prompt -> picks   │
        │  Restores on egress       │             │  model tier for 'auto'    │
        └───────────────────────────┘             └───────────────────────────┘
                                    │
                                    ▼ Upstream
                      ┌───────────────────────────┐
                      │ Provider Backend:         │
                      │ - copilot-api (:4141) OR  │
                      │ - Direct APIs (OpenAI/Anth)│
                      │ - Local Ollama / vLLM     │
                      └───────────────────────────┘
```

### 3.2 User Workflow
1. **Interactive Setup (`convergence init`):**
   - Engine check: Detects `docker` or `podman`.
   - Provider choice:
     - `[1] GitHub Copilot`: Deploys `copilot-api`, guides Copilot auth token creation.
     - `[2] Direct API Keys`: Prompts for OpenAI, Anthropic, Gemini, Groq, or Ollama URL.
     - `[3] Local Only (Ollama)`: Bypasses cloud tokens, routes straight to host/container Ollama.
   - Salt generation: Automatically generates high-entropy `STANDIN_SALT` (`openssl rand -hex 32`).
   - Configuration asset rendering into `~/.convergence/`:
     - `~/.convergence/config.yaml`
     - `~/.convergence/agentgateway.yaml`
     - `~/.convergence/pinball.yaml`
     - `~/.convergence/docker-compose.yaml`
2. **Lifecycle Control:**
   - `convergence start`: Runs container compose setup.
   - `convergence stop`: Stops services cleanly.
   - `convergence restart`: Rolls containers.
3. **Usage & Telemetry (`convergence stats`):**
   - AgentGateway writes structured access logs and cost attribution to `~/.convergence/logs/access.jsonl`.
   - Admin metrics are queryable at `http://localhost:15020/stats/prometheus`.
   - `convergence stats` parses the access log and metrics endpoint to print:
     - Total request count and HTTP status distribution.
     - Total tokens consumed (input, cache read, output) and cache hit rate.
     - Total estimated USD cost across providers.
     - Pinball deflection rate (% routed to Flash/Haiku vs Opus/Sonnet).
     - Stand-In tokenized secrets count.

---

## 4. Cluster Target Design (`convergence cluster install`)

### 4.1 Kubernetes Architecture
Target namespace: `convergence` (configurable via `--namespace`).
- **Deployments:**
  - `agentgateway` (Replicas: 2, PDB, Service, Ingress/Gateway API).
  - `stand-in` (Replicas: 2, PDB, Service, ext_proc on `:9101`).
  - `pinball` (Replicas: 2, PDB, Service, ext_authz on `:9201`).
  - `copilot-api` (Optional, Replicas: 2).
- **Persistence & DB Support:**
  - **Option A (Default built-in DB):** Deploys a managed PostgreSQL instance or PVC-backed datastore for AgentGateway request logging and Web UI analytics.
  - **Option B (External Postgres DSN):** Takes `--db-dsn="postgresql://user:pass@host:5432/dbname"` and generates Kubernetes Secrets without spinning up an in-cluster DB.
  - **Option C (Stateless):** Skips database deployment; sets up Prometheus `ServiceMonitor` for cluster metrics collection.

### 4.2 CLI Cluster Commands
- `convergence cluster install`:
  - Flags: `--kubeconfig`, `--namespace=convergence`, `--provider=copilot|direct|ollama`, `--db=builtin|external|none`, `--dry-run`.
  - Generates Stand-In salt Secret and provider credentials Secret.
  - Applies rendered manifests / Helm chart directly to cluster.
- `convergence cluster status`:
  - Inspects pod readiness, PDBs, services, and endpoints in target namespace.
- `convergence cluster upgrade`:
  - Performs declarative rollout upgrades of AgentGateway, Stand-In, or Pinball images.
- `convergence cluster uninstall`:
  - Safely deletes resources with confirmation and optional PVC retention.

---

## 5. Security & Invariant Guarantees

1. **Stand-In Invariant:** Pinball's `ext_authz` MUST only execute on requests that have already passed through Stand-In's `ext_proc` ingress filter. Pinball never sees plaintext secrets or tokens.
2. **Egress Sanitization:** Stand-In restores tokens on response streams as the final transformation before bytes return to the client.
3. **Fail-Open Policy:** If Pinball or Stand-In becomes unreachable or times out (>2s), AgentGateway fails open to the configured fallback model (`pinball-claude-sonnet-5.5` or direct upstream) rather than dropping client turns.
4. **Secret Isolation:**
   - Local: Secrets stored with `0600` permissions in `~/.convergence/config.yaml`.
   - Cluster: Stored exclusively in native Kubernetes Secrets (`stand-in`, `convergence-copilot`).
