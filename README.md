# Convergence

Convergence runs an AI gateway stack on your workstation or in a Kubernetes cluster and points your coding agents at it. One endpoint for every agent, secrets scrubbed before they leave your machine, and a model picked per request, without hand-writing gateway config.

It wires together:

- [agentgateway](https://agentgateway.dev/docs/) (`v1.6.0`): the gateway. Serves an OpenAI-compatible `/v1` endpoint (Anthropic Messages requests are translated) and forwards to your provider.
- [Stand-In](https://github.com/jeefy/stand-in) (`0.6.4`): an `ext_proc` filter that replaces secrets in prompts with opaque tokens and restores them in responses.
- [Pinball](https://github.com/jeefy/pinball) (`0.7.6`): an `ext_authz` service that scores each sanitized request and picks a model tier (`light`, `standard`, `heavy`).
- An optional sandbox for the agent itself: [NVIDIA OpenShell](https://github.com/NVIDIA/OpenShell), [nono](https://github.com/nolabs-ai/nono), [Docker Sandboxes](https://docs.docker.com/engine/security/sandboxes/) (`sbx`) or Kubernetes [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox). Convergence starts the agent inside it with inference routed through the gateway.

```
 Client (OpenCode, Claude Code, Aider, Goose, ...)
 ┌────────────────────────────────────────────────────────┐
 │ [Optional sandbox: OpenShell / nono / sbx / k8s-agent] │
 └───────────────────────────┬────────────────────────────┘
                             │ HTTP 127.0.0.1:3000 (/v1)
                             ▼
 ┌─────────────────── agentgateway ───────────────────┐
 │ 1. ext_proc ──► Stand-In   tokenize secrets        │
 │ 2. ext_authz ─► Pinball    choose model tier       │
 │ 3. route ─────► upstream                           │
 │ 4. ext_proc ──► Stand-In   restore secrets         │
 └───────────────────────────┬────────────────────────┘
                             │
                             ▼
 copilot-api :4141 (Copilot) │ OpenAI / Anthropic (direct) │ Ollama
```

Ports, on-disk layout and the Kubernetes resources are in [docs/architecture.md](docs/architecture.md).

## Prerequisites

- Linux or macOS, amd64 or arm64. On Windows, use WSL2 (see [Windows](#windows-wsl2)).
- Local stack: Docker with the compose plugin, or Podman with `podman-compose` or `podman compose`.
- Cluster mode: `kubectl` access to a cluster.
- Credentials for one provider (see [Providers](#providers)).

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
```

The installer downloads `convergence_<os>_<arch>.tar.gz` and `checksums.txt` from the GitHub release and refuses to install if the sha256 doesn't match. It installs to `/usr/local/bin` if writable, otherwise `~/.local/bin`, never runs `sudo`, and prints guidance if docker/podman with compose is missing.

```bash
CONVERGENCE_VERSION=v0.1.0 bash install.sh   # pin a release (default: latest)
INSTALL_DIR=$HOME/bin bash install.sh        # choose the target directory
```

From source (Go 1.24+):

```bash
git clone https://github.com/jeefy/convergence.git && cd convergence
make build            # produces bin/convergence
```

## Quickstart

### Local

```bash
convergence login copilot     # only for the Copilot provider
convergence init              # pick engine, provider, and optional sandbox; writes ~/.convergence
convergence start
convergence status
convergence config opencode   # or claude-code, aider, goose, ...
```

Non-interactive:

```bash
convergence init --yes --engine podman --provider direct \
  --openai-key "$OPENAI_API_KEY" --anthropic-key "$ANTHROPIC_API_KEY" --port 3000
```

With a sandbox:

```bash
convergence init --yes --provider direct --sandbox nono
convergence start
convergence sandbox run opencode
```

`sandbox run hermes` works too. Hermes ignores `OPENAI_BASE_URL`, so for a bare `hermes`, `hermes chat` or leading flags Convergence runs `hermes chat --provider custom --base-url <gateway> --api-key convergence`. Other Hermes subcommands are passed through unchanged.

### Kubernetes

```bash
convergence cluster install --namespace convergence --provider copilot --copilot-token "$GH_TOKEN"
convergence cluster status --namespace convergence
kubectl -n convergence port-forward svc/agentgateway 3000:3000
convergence config opencode --gateway-url http://127.0.0.1:3000
```

### Podman notes

`podman compose` delegates to an external provider. `init --engine podman --compose-provider podman-compose` selects the daemonless provider; `--compose-provider docker-compose` selects Docker Compose talking to Podman's API socket. The choice is saved as `compose_provider` in `config.yaml` and used by the lifecycle commands and `doctor`. Omitting the flag keeps the saved choice; `auto` restores Podman's own selection (`PODMAN_COMPOSE_PROVIDER`, `containers.conf`). An explicit choice overrides that variable and must be on `PATH`. Docker Engine always uses its compose plugin.

The OpenShell sandbox (`--sandbox openshell`, any provider including Copilot) needs Podman's API socket: `systemctl --user enable --now podman.socket` on rootless Linux. Convergence mounts the discovered socket into the OpenShell gateway container and selects OpenShell's Podman driver. On Linux, SELinux labeling is disabled for that container so it can reach the socket. The socket grants control over all of your Podman containers.

## Providers

| Provider | Upstream | Credentials |
|---|---|---|
| `copilot` | GitHub Copilot through [copilot-api](https://github.com/ericc-ch/copilot-api) (`v0.7.0`) on `:4141` | A GitHub account with an active Copilot subscription. `convergence login copilot` (device flow) or `--copilot-token`. |
| `direct` | OpenAI and Anthropic APIs (Gemini isn't supported) | `--openai-key` from <https://platform.openai.com/api-keys>, `--anthropic-key` from <https://console.anthropic.com/settings/keys>. Either or both. |
| `ollama` | A local Ollama server | No key. `--ollama-url` defaults to `http://host.docker.internal:11434`. Local mode only. |

Cluster mode supports `copilot` and `direct`.

## Model routing

Pinball scores each request into a tier (`light`, `standard`, `heavy`). Convergence maps each tier to one pinball model, and each pinball model to an upstream model for your provider. `convergence models` shows the active mapping:

```
Provider: direct
TIER      PINBALL MODEL      UPSTREAM
light     claude-haiku-5.5   anthropic/claude-haiku-5-5
standard  claude-sonnet-5.5  anthropic/claude-sonnet-5-5
heavy     claude-opus-5.5    anthropic/claude-opus-5-5
```

Defaults per provider:

| Tier | Pinball model | `copilot` | `direct` | `ollama` |
|---|---|---|---|---|
| light | `claude-haiku-5.5` | `claude-haiku-5.5` | `anthropic/claude-haiku-5-5` | `qwen2.5-coder:7b` |
| standard | `claude-sonnet-5.5` | `claude-sonnet-5.5` | `anthropic/claude-sonnet-5-5` | `qwen2.5-coder:14b` |
| heavy | `claude-opus-5.5` | `claude-opus-5.5` | `anthropic/claude-opus-5-5` | `qwen2.5-coder:32b` |

Providers rename and retire models, so these are defaults, not guarantees.

### How a prompt becomes a model

Clients opt in by sending `model: auto` (or `pinball`). Scoring is Pinball's job; the rules live in the [Pinball README](https://github.com/jeefy/pinball#choosing-a-tier). In short: the score is the best-scoring user message plus request signals (code fences, heavy keywords such as `refactor` or `root cause`, length, `tools` present, reasoning effort, small `max_tokens`). Convergence renders thresholds `standard: 1` and `heavy: 3`: below 1 is `light`, 3 or more is `heavy`, anything between is `standard`. If Pinball doesn't answer, the request goes to the standard-tier model.

The gateway forwards `x-pinball-tier` (force a tier) and `x-pinball-min-tier` (raise to at least that tier) request headers to Pinball. Responses carry the pick in `x-pinball-model`, `x-pinball-tier` and `x-pinball-reason`; the body's `model` is the real upstream model.

Worked examples with the point values from the Pinball README (they may change between Pinball versions; `convergence doctor` shows the real decision for its probe prompts):

| Prompt (sent with `model: auto`) | Signals | Score | Tier |
|---|---|---|---|
| "Generate a title", `max_tokens: 20` | light keyword -1, short -1, small output -1 | -3 | light |
| A 300 character coding question with `tools` set | tools +1 | 1 | standard |
| A pasted traceback and "find the root cause of this deadlock and refactor it" | code +1, two heavy keywords +2 | 3 | heavy |

### Overrides: `models.yaml`

`~/.convergence/models.yaml` overrides the defaults (`convergence models init` writes a commented starter and never overwrites an existing file). Run `convergence restart` after editing. In cluster mode pass the same file with `cluster install --models-file models.yaml`.

```yaml
# provider -> pinball model -> upstream model
direct:
  claude-haiku-5.5: openai:gpt-5-mini   # direct accepts anthropic: (default) or openai:
ollama:
  claude-opus-5.5: qwen2.5-coder:14b
```

Only `copilot`, `direct` and `ollama` are valid provider keys, and only the three pinball model names above are valid; anything else is an error. For Ollama you can instead set tiers at init with `--ollama-model-light`, `--ollama-model-standard` and `--ollama-model-heavy`; `models.yaml` wins over those flags.

**Pinball thresholds and scoring.** An optional top-level `pinball:` block overrides Pinball's score cutoffs and per-signal points (unset keys keep Pinball's defaults; unknown keys are an error). Example: Pinball adds `+1` for requests with tools, which with `thresholds.standard: 1` puts every tool-using agent request in `standard`. To let short agent turns reach `light`:

```yaml
pinball:
  thresholds: {standard: 2, heavy: 4}
  scoring: {tools: 0}   # keys: code, heavy_keyword, max_heavy_keywords, light_keyword, long_chars, long, very_long_chars, very_long, short_chars, short, tools, effort_high, effort_low, small_output_tokens, small_output, structured
```

**Local light tier.** Short prompts (titles, one-line questions) land in `light`. You can keep the heavier tiers on your cloud provider and serve `light` from a local Ollama model with the `ollama:` prefix, accepted for `copilot` and `direct`:

```yaml
copilot:
  claude-haiku-5.5: ollama:gemma4:26b
```

Set `ollama_url` in `~/.convergence/config.yaml` if it isn't the default, `ollama pull gemma4:26b`, then `convergence restart`. A 26b-class model has been a good fit for titles and short answers; 7b-class models are faster but noticeably weaker. Local latency depends on your hardware (the first request after idle loads the model) and quality is below the hosted light model, so keep `standard` and `heavy` on the cloud. `convergence doctor` warns when a model mapped to Ollama is not installed. Check routing with a short `model: auto` request and look at the `x-pinball-tier` response header.

With the Ollama provider, `convergence start` warns (with the `ollama pull` commands) when a mapped model isn't installed or Ollama can't be reached; `start --strict` makes that an error.

**Passthrough models.** To call a fixed model by name with no Pinball routing, add a `passthrough:` block. The client uses the key as the model name; Stand-In still scrubs the request and the `x-pinball-*` response headers stay empty.

```yaml
passthrough:
  copilot-fable-5: {provider: copilot, model: claude-fable-5}
  glm-5.2:         {provider: ollama,  model: glm-5.2}
```

`provider` is `copilot`, `openai`, `anthropic` or `ollama`. Names must not be `auto`, `pinball`, `light`, `standard`, `heavy` or start with `pinball-`. On a cluster install, `ollama` entries need a `base_url` and `copilot` entries need `--provider copilot`.

An entry may set `base_url` and `api_key_env` (the **name** of an env var, never the key) to use another endpoint such as Ollama Cloud. Entries with the same `(provider, base_url, api_key_env)` share one gateway provider; entries without them use the shared provider (local Ollama, `--ollama-url`).

```yaml
passthrough:
  glm-5.2: {provider: ollama, model: glm-5.2, base_url: 'https://ollama.com', api_key_env: OLLAMA_CLOUD_KEY}
  gemma4:  {provider: ollama, model: gemma4}   # local Ollama
```

Local: export `OLLAMA_CLOUD_KEY` when running `init`, `restart` or `upgrade`; it is written to the 0600 `.env` (an existing value is kept if the variable is unset) and the rendered config only contains `${OLLAMA_CLOUD_KEY}`. Cluster: create the Secret yourself, e.g. `kubectl -n convergence create secret generic convergence-passthrough --from-literal=OLLAMA_CLOUD_KEY=...` (name via `--passthrough-secret`); the gateway reads each key from its environment. Keys never appear in rendered config or manifests. `STANDIN_SALT`, `GH_TOKEN`, `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` are not allowed as `api_key_env`.

## Client configuration

`convergence config <client>` edits the client's config to use the gateway. It writes a `<file>.convergence.bak` backup first; `restore` puts it back (`--force` even if the file changed since). `--dry-run` prints the change without writing. `--gateway-url` overrides the default `http://127.0.0.1:<port>`. `convergence config list` shows the supported clients and whether each currently points at Convergence.

| Client | Command | File changed |
|---|---|---|
| OpenCode | `convergence config opencode` | `~/.config/opencode/opencode.json` |
| oh-my-opencode | `convergence config oh-my-opencode` | `~/.omo/omo.jsonc` |
| OpenClaw | `convergence config openclaw [--set-default]` | `~/.openclaw/openclaw.json` |
| Claude Code | `convergence config claude-code` | `~/.claude/settings.json` |
| Aider | `convergence config aider` | `~/.aider.conf.yml` |
| Goose | `convergence config goose` | `~/.config/goose/config.yaml` |
| Hermes Agent | `convergence config hermes [--set-default]` | `config.yaml` in `$HERMES_HOME`, else `%LOCALAPPDATA%\hermes` on Windows, else `~/.hermes` |
| Continue | `convergence config continue` | `~/.continue/config.json` |
| Zed | `convergence config zed` | `~/.config/zed/settings.json` |
| Cline / Roo Code | `convergence config cline` | `~/.config/Code/User/settings.json` |
| Anything else | `convergence config copilot` | Nothing. Prints `OPENAI_*` / `ANTHROPIC_*` exports to `eval` in your shell. |

## CLI reference

Flags listed here are the common ones; `convergence <command> --help` is the full list.

| Command | Description |
|---|---|
| `init [--yes] [--engine docker\|podman] [--provider P] [--sandbox S] [--port N] [--fail-open] ...` | Write `~/.convergence/config.yaml` and render the compose stack. Interactive unless `--yes`. Credentials: `--copilot-token`, `--openai-key`, `--anthropic-key`, `--ollama-url`. |
| `login copilot` | GitHub device-flow login; stores the token for the Copilot provider. |
| `start [--strict]` / `stop` / `restart` / `down` | Manage the local containers. `--strict` fails instead of warning when Ollama lacks a mapped model. `down` also removes containers and networks. |
| `upgrade` | Pull the image versions pinned by this binary and recreate the stack. |
| `prune` | Remove dangling images, stopped containers and unused networks. |
| `uninstall [--purge] [--yes]` | Stop and remove the stack. `--purge` also deletes `~/.convergence`. |
| `status [--json]` | Container state and health probe per service; exits non-zero if a service is down. |
| `doctor [--e2e] [--json]` | Self-checks; see [Status and doctor](#status-and-doctor). |
| `logs [-f] [service]` | Logs for all services or one of `agentgateway`, `stand-in`, `pinball`, `copilot-api`. |
| `models` / `models init` | Show the tier to upstream mapping / write a starter `~/.convergence/models.yaml`. |
| `stats [--json] [--watch]` | Request counts, tokens, latency quantiles, tier decisions and redactions from Prometheus metrics. |
| `insights [--since 1d\|7d\|30d] [--coach] [--json]` | Usage, estimated cost and prompt patterns from agentgateway's request log; see [Insights](#insights). |
| `config list` / `config <client> [--dry-run] [--gateway-url URL]` / `config <client> restore [--force]` | See [Client configuration](#client-configuration). |
| `sandbox list` / `sandbox status [--json]` | Supported sandbox runtimes / status of the configured one. |
| `sandbox run [-v path] <agent> [args...]` | Run an agent inside the configured sandbox with inference routed through the gateway. |
| `skills build` / `skills scan [path...] [--llm] [--model M]` | Build the SkillSpector image / scan installed agent skills; see [Skill scanning](#skill-scanning). |
| `cluster install [--namespace NS] [--provider copilot\|direct] [--models-file F] [--fail-open] [--dry-run] ...` | Apply manifests to the current kube context; `--dry-run` prints them. Omitted secrets (Copilot token, Stand-In salt, Postgres password) are reused from a previous install in the namespace. |
| `cluster status` / `cluster logs [-f] [--tail N] [service]` / `cluster uninstall [--yes]` | Rollout and pod health / pod logs / delete the resources. |
| `cluster backup --out DIR` / `cluster restore --from FILE [--yes]` | See [Backup and restore](#backup-and-restore). |
| `optimize --db-dsn DSN [--dry-run]` | **Experimental.** Run the Pinball optimizer against a request-log database. |
| `version` | Version, commit, build date. |

Image overrides (`--image-agentgateway`, `--image-standin`, `--image-pinball`, `--image-copilot-api`, and `--image-openshell` for `init`) exist on both `init` and `cluster install`.

`--extproc-timeout` (config `extproc_timeout`, default `5s`) sets agentgateway's `requestTimeout` for the ext_proc policy: how long it waits for the first gRPC response from Stand-In (agentgateway#4146). Raise it for very large requests; it must be a positive Go duration such as `60s`. In the cluster, Stand-In is CPU-bound on large bodies, so raise `--standin-cpu-limit` (default `500m`; request `50m` via `--standin-cpu-request`) before raising the timeout.

## Status and doctor

`convergence status` probes each service: container state plus a health check (agentgateway is probed over its port because its image has no shell). It exits non-zero if any required service is down; `--json` gives machine-readable output. If Stand-In is down or the stack is fail-open, it prints a loud warning that secret scrubbing is not active.

`convergence doctor` runs these checks and exits 1 if any fails:

| Check | Meaning |
|---|---|
| config | `~/.convergence/config.yaml` parses and `.env` is mode `0600` |
| engine | docker/podman answers and the configured compose provider is available |
| service `<name>` | each service is running and healthy |
| port `<n>` | each published port accepts connections |
| stand-in metrics | Stand-In `/metrics` is scrapeable and in `enforce` mode |
| pinball `/check` | probe prompts get a tier and model back from Pinball |
| provider reachability | the upstream (copilot-api, Ollama, ...) answers; warns when a model mapped to Ollama isn't installed |
| stand-in fail mode | warns if fail-open is configured or rendered |
| skills | result of the last `skills scan` of installed skills: warns when never scanned, changed since, or unsafe; fails on unsafe with `skill_scan: block`; informational while `skill_scan` is `off` |

`--e2e` additionally sends one real request (model `auto`, `max_tokens: 16`) containing a fake GitHub token through the gateway. With a real provider this costs a few tokens, so it's off by default.

## Stats

`convergence stats` scrapes the Prometheus endpoints of agentgateway (`:15020`), Stand-In (`:9102`) and Pinball (`:9201`) and summarizes request counts, tokens, latency quantiles (p50/p90/p99), tier decisions and redactions. Counters reset when containers restart. It shows tokens and tiers, not money. `--json` prints machine-readable output; `--watch` refreshes.

In cluster mode, `cluster install --service-monitor` renders prometheus-operator `ServiceMonitor`s for the three metrics ports plus a NetworkPolicy `allow-monitoring-to-metrics` that lets pods in `--monitoring-namespace` (default `monitoring`) scrape them through the default-deny policy. The monitors carry `release=prometheus-operator` unless you pass `--service-monitor-label key=value` (repeatable) to match your Prometheus `serviceMonitorSelector`. The flag fails before applying anything when the `servicemonitors.monitoring.coreos.com` CRD is absent.

## Insights

`convergence insights` reads agentgateway's Postgres request log over a window (`--since 1d|7d|30d`, default `7d`) and reports:

- request counts, input and output tokens, average latency, per-model breakdown and tier distribution;
- estimated cost in total and per tier, and the estimated saving versus pricing the same tokens on one reference model (`--reference-model`, default the heavy-tier model). Prices come from Pinball's optimizer catalog, not from invoices; rows without a price are excluded and counted ("N of M rows priced");
- patterns worth a look: large prompts with tiny completions, heavy-tier picks for simple requests, rapid duplicate retries, high error rates;
- with `--coach`, recommendations generated from those patterns through the gateway itself (`POST /v1/chat/completions`, model `auto`).

It needs the request log enabled (`init --request-log-db`, or `cluster install --request-log-postgres` / `--request-log-db`), or a DSN via `--db-dsn`. `--json` prints the data.

## Request log and Pinball optimizer

The request log and the optimizer are off by default and experimental.

**Privacy.** Enabling the request log sets agentgateway's LLM logging to `full`: the database stores prompts and completions, after Stand-In scrubbing, which is pattern-based and not a guarantee. Protect and expire that database accordingly. The optimizer replays sampled requests against candidate models, so it makes real model calls.

**Local.** The compose stack ships no Postgres. Point agentgateway at one you run, reachable from the containers: `convergence init --request-log-db postgres://user:pass@host.docker.internal:5432/agentgateway && convergence restart`. The DSN goes to the 0600 `.env` and `config.yaml`, never into the rendered gateway config. `convergence optimize` then uses it without `--db-dsn`.

**Cluster.** `cluster install --request-log-postgres` renders a small Postgres (StatefulSet, 5Gi PVC, Service, NetworkPolicy allowing only agentgateway and the optimizer) with a generated password; `--request-log-db <dsn>` uses your own database instead. Either one also renders the optimizer CronJob `pinball-optimize`, weekly by default (`0 3 * * 0`; change with `--opt-schedule`), using the install's pinball image. `--optimizer-dsn <dsn>` renders only the CronJob against an existing request-log database, without turning logging on. The DSN lives only in Secret `pinball-optimizer-db`; agentgateway and the CronJob read it from the environment. Re-running `cluster install` reuses the Postgres password, Stand-In salt and Copilot token from the previous install's Secrets when the flags are omitted (pass them to rotate), so the existing PVC keeps working.

**Retention.** The log grows without bound by default. `cluster install --request-log-retention 720h` renders a daily CronJob `request-log-retention` (02:17) that deletes rows older than that duration in batches of 5000 (payloads follow via `ON DELETE CASCADE`). Minimum `24h`, the optimizer's sampling window; shorter values fail at install. Dropping the flag on a re-install leaves an existing CronJob in place until you `kubectl delete cronjob request-log-retention`. Deleted rows are gone: back up first.

### Backup and restore

`convergence cluster backup --out ./bk` writes `request_logs-<ts>.dump` (`pg_dump -Fc`) and `pinball-overlay-<ts>.tar` (a missing overlay only warns). `convergence cluster restore --from ./bk/request_logs-<ts>.dump` runs `pg_restore --clean --if-exists` into the running Postgres after confirmation (`--yes` skips it). `cluster install` backs up an existing `request-log-postgres` into `~/.convergence/backups/` first (keeps the last 5; `--no-backup` skips). Dumps contain prompts: protect them like the database.

`cluster uninstall` deletes the namespace and, with most storage classes (reclaim policy `Delete`), the PVs. Back up first, or retain the volumes: `kubectl get pv`, then `kubectl patch pv <name> -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'` for the `request-log-postgres` and `pinball-overlay` claims. Scheduled or off-cluster backups are not included.

## Skill scanning

Agent skills run with the agent's trust. `convergence skills scan` vets them with [NVIDIA SkillSpector](https://github.com/NVIDIA/skillspector) (prompt injection, data exfiltration, supply chain and more) in a container, so the host needs no Python. NVIDIA publishes no image, so build the pinned one once:

```bash
convergence skills build      # localhost/convergence-skillspector:v2.13.4 from a pinned commit
convergence skills scan       # ~/.claude/skills, ~/.config/opencode/skills, ~/.agents/skills, <hermes home>/skills
convergence skills scan ./some-skill
```

Each skill gets a score, a severity and a verdict (`SAFE`, `CAUTION`, `DO_NOT_INSTALL`). The command exits 1 when a skill is `DO_NOT_INSTALL`. The static scan runs with `--network=none` and takes seconds.

`--llm` adds SkillSpector's semantic analysis through the gateway (one to two minutes per skill). The model must honor forced tool calls, so `auto` does not work: set `skill_scan_model` in `config.yaml` or pass `--model`. If the model ignores them, the analysis is incomplete and the command says so and exits 1.

To have `sandbox run` check installed skills before starting the agent, set `skill_scan` in `config.yaml`:

| `skill_scan` | Unsafe skill | Scanner image missing |
|---|---|---|
| `off` (default) | no check | no check |
| `warn` | warning, the agent starts | warning, the agent starts |
| `block` | refuses to start | refuses to start |

It reuses the last static scan (`~/.convergence/skill-scan.json`) and rescans only when the installed skills changed. `doctor` reports the cached result.

## Observability

Optional OTLP trace export, off by default. Convergence only configures the export; you run the collector or backend (OpenTelemetry Collector, Jaeger, Langfuse, Phoenix, Grafana Tempo, ...).

```bash
convergence init --otel-endpoint localhost:4317               # local (gRPC)
convergence init --otel-endpoint collector:4318 --otel-protocol http --otel-sample 0.25
convergence cluster install --provider copilot --otel-endpoint otel-collector.monitoring:4317
```

Or in `~/.convergence/config.yaml`:

```yaml
otel:
  endpoint: localhost:4317   # host:port; unset = no tracing
  protocol: grpc             # grpc (default) or http
  sample: 1                  # 0..1, default 1
```

Spans use `service.name=convergence` and carry agentgateway's `gen_ai.*` attributes (operation, provider, request and response model, input and output tokens), `caller` (from the `x-agw-caller` request header) and, for `model: auto` requests, `pinball.tier`, `pinball.model`, `pinball.reason` and `pinball.score`. Traces carry metadata only, never prompts or completions. agentgateway's exporter honors `HTTP_PROXY`/`HTTPS_PROXY`, which can break OTLP delivery. From a container, reach a collector on the host as `host.docker.internal:4317`.

## Security model

**What Stand-In does.** It inspects request bodies and replaces detected secrets (API keys, tokens, credentials) with `<<secret-...>>` placeholders, then restores them in responses. Detection is pattern-based; the rules are described in the [Stand-In README](https://github.com/jeefy/stand-in). It won't catch every secret, or any PII it has no rule for, and it doesn't cover anything you send through other paths. Treat it as a safety net, not a guarantee.

**Custom patterns.** `init --standin-patterns-file ./patterns.yaml` (saved as `standin_patterns_file`) or `cluster install --standin-patterns-file ./patterns.yaml`. Compose mounts the file read-only and sets `STANDIN_PATTERNS_FILE`; the cluster install renders ConfigMap `stand-in-patterns` and mounts it. The file **replaces** Stand-In's packaged `patterns.yaml` (Stand-In loads exactly one file), so start from a copy of the upstream file and append your rules.

**Failure modes.** Stand-In is fail-closed by default (`failureMode: failClosed`): if it is down, requests are rejected and nothing leaves unscrubbed. `init --fail-open` or `cluster install --fail-open` opts into passthrough, where requests go upstream **unscrubbed** while Stand-In is down; `status` and `doctor` warn about it. Pinball uses `failureMode: allow` on purpose: it only picks a model, so when it is down the request goes to the standard-tier model and confidentiality is unaffected.

**Exposure.** All published ports bind to `127.0.0.1` by default (`bind_address` in `config.yaml`). The gateway has no authentication of its own: any local process that reaches `127.0.0.1:3000` can use your provider keys through it, and in Kubernetes any pod the NetworkPolicy allows can do the same. Change `bind_address` only if you mean to expose it.

**Secrets at rest.** Locally, provider keys and tokens are stored in plaintext in `~/.convergence/config.yaml` and `~/.convergence/.env`, both mode `0600`. The rendered compose and gateway files contain no secrets; compose reads them from `.env`. In the cluster, keys live in Kubernetes Secrets. Containers use `json-file` log rotation (10 MB, 3 files).

Report vulnerabilities privately: see [SECURITY.md](SECURITY.md).

## Troubleshooting

**Port already in use.** Another process has `3000` (or `4141`, `9102`, `9201`, `15020`). Find it with `ss -ltnp | grep 3000` (Linux) or `lsof -i :3000` (macOS). `convergence init --port 3100` moves the gateway; the other ports are fixed.

**Podman rootless errors.** Check that `podman info` works. Docker Compose and OpenShell need Podman's user API socket (`systemctl --user enable --now podman.socket`); `podman-compose` does not. SELinux denials on mounted config files are handled with the `z` volume label; if you edited the compose file by hand, re-run `convergence init`.

**Podman reports `nft` JSON support is not compiled in.** Netavark needs an `nft` binary with JSON support. On Linux, a Homebrew `nftables` can shadow the system binary without providing it. Check `command -v nft`; if it points to Homebrew and a suitable system `nft` exists, `brew unlink nftables` and retry `convergence start` (`brew link nftables` undoes it; other programs using Homebrew's `nft` are affected too).

**OpenShell cannot connect to the Podman API socket.** Check `podman info --format '{{.Host.RemoteSocket.Path}}'` and enable the local socket. A remote socket path must also be reachable by the local compose engine.

**Health check failing.** Run `convergence status`, then `convergence logs <service>`. Common causes: an image pull failed, `STANDIN_SALT missing` (re-run `convergence init`), or a bad provider key (gateway logs show 401s from upstream).

**Copilot auth.** `401`/`403` from `copilot-api` means the token is missing, expired, or the account has no Copilot subscription. Run `convergence login copilot` again, then `convergence restart`. `convergence logs copilot-api` shows the auth result.

## Windows (WSL2)

Run Convergence inside a WSL2 distro, installed like on Linux, with Docker or Podman reachable from that distro. `convergence config <client>` edits files in the distro's home, so it configures clients that run inside WSL2. For a client on the Windows side, point it at `http://localhost:3000/v1` with API key `convergence` yourself (`--dry-run` shows the values). Native Windows builds are not tested: several unit tests assume a Unix home and file modes.

## Versioning and upgrades

Convergence follows [Semantic Versioning](https://semver.org/). Before 1.0, minor releases may include breaking changes; they are called out in [CHANGELOG.md](CHANGELOG.md). Each release pins agentgateway, Stand-In, Pinball and copilot-api by tag and sha256 digest (see [docs/architecture.md](docs/architecture.md#pinned-images)). To upgrade, install the new binary (re-run the installer), then run `convergence upgrade`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Apache-2.0. See [LICENSE](LICENSE).
