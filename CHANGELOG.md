# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `--discover-models` on `init` (provider `copilot` or `ollama`) and `cluster install` (provider `copilot`), off by default: Pinball builds its pool from the upstream `/v1/models` (copilot-api or Ollama) and infers tiers, falling back to its cache and then the rendered pool. The gateway routes `auto` to Pinball's pick, passes any other model id through (`*`) and, with the optimizer on, serves `opt-*` as one route so `/v1/models` lists no duplicates. Provider `direct` is refused before any file is written or manifest applied. Covered by `tests/test_e2e_local.sh` (Ollama against the stub) and `tests/test_e2e_cluster.sh` (refusal).
- `models.yaml` `pinball:` block: `heavy_keywords` and `light_keywords` replace the keyword lists rendered into Pinball's config (compose and Kubernetes; Pinball's own lists stay the default, `[]` turns that signal off), and `scoring` accepts the context-size keys `large_context_tokens`, `large_context`, `very_large_context_tokens`, `very_large_context`, validated (cutoffs positive, very large above large). Covered by `tests/test_e2e_local.sh`.

### Changed

- Documentation: `README.md` is now a short index; details moved to `docs/` pages (installation, quickstart, models, clients, cli, operations, insights, skills, system-prompt, security). Corrected the Continue config path (`~/.continue/config.yaml`), the `config` default gateway URL (`http://localhost:<port>`), the `doctor` check list, `oh-my-opencode` behavior (does not edit `omo.jsonc`) and missing flags in the CLI reference; removed a non-existent `logs/` directory from the file list.

## [0.6.0] - 2026-11-19

### Added

- `convergence config paseo`: adds providers `convergence-claude` (extends `claude`) and `convergence-opencode` (extends `opencode`) under `agents.providers` in Paseo's `config.json` (`$PASEO_HOME`, else `~/.paseo`). The built-in providers are not changed. The Claude provider maps Claude Code's haiku/sonnet/opus slots to `light`/`standard`/`heavy`, disallows `WebSearch` and lists `auto`, the tiers and the local `models.yaml` passthrough names. A created file is `0600`. Covered by `tests/test_e2e_local.sh`.

### Changed

- Bump pinned Pinball image to `0.8.0` (`ghcr.io/jeefy/pinball:0.8.0@sha256:62550d3a0df6119168adee301d268b4a368d000f456c91109471b9fda53a28c4`). Pinball 0.8.0 can build its pool from the upstream `/v1/models` (`PINBALL_DISCOVERY_URL`); Convergence does not enable it yet, so routing uses the rendered pool as before. It also skips a model that cannot serve the request's API shape or `reasoning_effort` (only when the pool carries that data, which the rendered pool does not).

### Fixed

- Local stack: the compose `pinball` service did not set `PINBALL_CONFIG`, so Pinball ignored the mounted `pinball.yaml` and served its packaged model pool, thresholds and keywords. `models.yaml` overrides and `pinball:` settings never reached it. The service now sets `PINBALL_CONFIG`. The rendered `pinball.yaml` (local and the Kubernetes `pinball-config` ConfigMap) now includes Pinball 0.7.7's `heavy_keywords` and `light_keywords`, because a mounted file replaces the packaged one whole and the Kubernetes deployment was routing without keyword scoring. Covered by `tests/test_e2e_local.sh`.
- Release: the winget publisher failed with `function "envOrDefault" not defined` because GoReleaser only accepts a bare `{{ .Env.NAME }}` for `repository.token`. The v0.5.0 archives, checksums and SBOMs were published; the winget submission was not.

## [0.5.0] - 2026-11-17

### Added

- `system_prompt` (config.yaml), `init --system-prompt [--system-prompt-file F]` and `cluster install --system-prompt [--system-prompt-file F]`: off by default. When enabled, every model in the rendered agentgateway config gets a CEL request `transformation` that prepends a fixed set of behavioral guardrail lines (`internal/prompts`) plus optional extra text to OpenAI chat `messages`, Responses `instructions` and Anthropic Messages `system` on the `/v1` endpoints (agentgateway passes other paths through unchanged). The text is not scrubbed by Stand-In. The optimizer's `opt-` twin models are left without the transformation because their replays come from the request log, which already holds the prompt, and the optimizer's judge requests (`x-agw-caller: eval-replay-judge`) are exempt. The text must be valid UTF-8 without `$` or control characters. Covered by `tests/test_e2e_local.sh` and `tests/test_validate_configs.sh`.
- Windows release artifacts: `convergence_windows_amd64.zip` and `convergence_windows_arm64.zip`, covered by `checksums.txt` and the SBOMs. The client config writers, `models`, `version` and the commands that talk to a remote gateway or cluster work natively, the local stack stays WSL2-only (#40).
- `install.ps1`: PowerShell installer (`irm .../install.ps1 | iex`) that downloads the zip, verifies the sha256 against `checksums.txt`, installs to `%LOCALAPPDATA%\Programs\convergence` (or `$env:INSTALL_DIR`), adds it to the user `PATH` and never elevates. Offline tests in `tests/test_install.ps1`.
- CI runs the unit tests of the engine-free packages and the PowerShell installer tests on `windows-latest`.
- GoReleaser winget publisher for `jeefy.convergence`; it submits to `microsoft/winget-pkgs` once the `WINGET_GITHUB_TOKEN` secret exists (see CONTRIBUTING).

### Changed

- Bump pinned Pinball image to `0.7.7` (`ghcr.io/jeefy/pinball:0.7.7@sha256:7b610b1426eb4667a60930967c91dd2bab5540f4dd70e75ff8a4dbe6ee2d3881`): the optimizer reads agentgateway 1.6.0 request-log payloads. With `0.7.6` every optimizer run against a 1.6.0 request log ended with `samples: 0`.
- `doctor` no longer fails the `config` check on Windows for the `.env` file mode (NTFS has no POSIX mode).
- Unit tests set `USERPROFILE` next to `HOME` (`internal/testhome`) so running them on Windows no longer writes into the real profile; file-mode assertions are skipped there.
- `install.sh` points Windows users at the zip asset in addition to WSL2.

### Fixed

- Request log: the `x-agw-caller` request header is now stored as `request_logs.agentgateway_user` (agentgateway `config.standardAttributes.user`). The column stayed empty, so the optimizer's per-caller results and its `eval-*` caller exclusion had nothing to work with. Rendered only when the request log is enabled.
- Copilot provider: system text sent as Anthropic `system` blocks or as Responses API `instructions` did not reach Claude models. agentgateway converts those to a system message with a list of content parts and to the `developer` role, and Copilot ignored both (observed with `claude-haiku-5.5` through copilot-api `v0.7.0`). Models served by Copilot now carry a `finalTransformation` that sends them as plain-text `system` messages. `tests/test_e2e_copilot.sh` checks all three APIs against the real provider.

## [0.4.3] - 2026-11-13

### Fixed

- Cluster e2e test: the `--service-monitor` render check no longer client-dry-runs ServiceMonitor documents on clusters without the prometheus-operator CRD (CI KinD), and the intentional CRD-missing failure no longer triggers the cluster-state dump.

## [0.4.2] - 2026-11-13

### Added

- Kubernetes: `cluster install --service-monitor` renders prometheus-operator ServiceMonitors for the agentgateway, Stand-In and Pinball metrics ports and a NetworkPolicy allowing scrapes from `--monitoring-namespace` (default `monitoring`). `--service-monitor-label key=value` (repeatable, default `release=prometheus-operator`) sets the labels your Prometheus `serviceMonitorSelector` matches. Install fails before applying when the ServiceMonitor CRD is missing.

## [0.4.1] - 2026-11-13

### Changed

- Bump pinned Stand-In image to `0.6.4` (`ghcr.io/jeefy/stand-in:0.6.4@sha256:11abda87cd135cf60e1c028ee1e35ccab9999cc206cdd8cc2971148bffa5e6c7`): fixes quadratic prefilter scans in the Go tokenizer that made 200 KB+ tool-output-heavy requests take minutes and hit the gateway timeout (stand-in#24).

## [0.4.0] - 2026-11-13

### Added

- `convergence skills scan` and `skills build`: vet agent skills with NVIDIA SkillSpector (pinned v2.13.4) in a container. Static by default (`--network=none`); `--llm` adds semantic analysis through the gateway with a model that honors forced tool calls (`skill_scan_model`).
- `skill_scan: warn|block` in `config.yaml` (default `off`) makes `sandbox run` check installed skills first, reusing the cached scan until the skills change. `doctor` reports the cached result.
- `insights` reports total cost, cost per tier (with an "N of M rows priced" coverage line) and estimated savings versus a reference model (`--reference-model`, default the heavy-tier model), also in `--json`. Prices are Pinball's optimizer catalog estimates, not invoices.
- `doctor` warns when a model that `models.yaml` maps to Ollama is not installed on the configured endpoint. The README documents serving the light tier from a local Ollama model.
- `cluster install --request-log-retention DUR` renders a daily CronJob that deletes request-log rows older than DUR in batches (off by default, minimum 24h, the optimizer's sampling window).
- `sandbox run hermes`: Hermes ignores `OPENAI_BASE_URL`, so Convergence runs `hermes chat --provider custom --base-url <gateway> --api-key convergence`. Other Hermes subcommands are passed through.
- `convergence config openclaw` adds a `convergence` provider (models `auto`, `light`, `standard`, `heavy`) to `~/.openclaw/openclaw.json`; `--set-default` also sets `agents.defaults.model.primary`.
- `convergence config hermes --set-default` makes `convergence` / `auto` the Hermes default model (`model.provider`, `model.default`, `model.base_url`). Without the flag the default is left alone.

### Changed

- Bump pinned Stand-In image to `0.6.3` (`ghcr.io/jeefy/stand-in:0.6.3@sha256:e1c150aada9e059601e7c0552b0d9fa30921c0e92f343fcba0a5b66f08066f01`): checksum-validated payment card and IBAN rules, base64 image payloads skipped, Meta `EAA` rule no longer rewrites base64 image data.
- README, `docs/architecture.md` and `CONTRIBUTING.md` reorganized: model routing collected in one section, trimmed CLI table (`--help` is authoritative), corrected file modes and the digest-pinning note, and the Kubernetes resource table completed.

### Fixed

- `config hermes` writes to the config Hermes reads: `$HERMES_HOME`, else `%LOCALAPPDATA%\hermes` on Windows, else `~/.hermes`. It always used `~/.hermes`.

## [0.3.1] - 2026-11-10

### Added

- `cluster backup --out DIR` and `cluster restore --from FILE [--yes]` for the request-log Postgres (and pinball overlay). `cluster install` backs up an existing request-log Postgres first (last 5 kept in `~/.convergence/backups/`, `--no-backup` skips).

### Fixed

- `cluster install` re-runs no longer rotate the Stand-In salt, replace `GH_COPILOT_TOKEN` with an empty value or generate a new request-log Postgres password: omitted secrets are read back from the Secrets of the previous install (`stand-in`, `convergence-copilot`, `pinball-optimizer-db`). `--provider copilot` now fails early when no token is given and none exists in the cluster.

## [0.3.0] - 2026-11-10

### Added

- `cluster install --standin-cpu-request` / `--standin-cpu-limit` set Stand-In's CPU (defaults `50m` / `500m`, unchanged). Raise the limit if ext_proc times out on large request bodies.
- agentgateway now records `pinball.score` and `pinball.est_tokens` from Pinball's `/check` in the access/request log (compose and Kubernetes), so routing thresholds can be tuned from real traffic.

### Fixed

- Pinball optimizer could never replay candidates: it sends them as `opt-<pinball model>` (`PINBALL_OPT_MODEL_PREFIX`), but only the internal `pinball-<model>` aliases were rendered, so every candidate call was a 404. The gateway config (local and cluster) now renders public `opt-` twins when the request log is configured. The CronJob and `convergence optimize` also set `PINBALL_OPT_JUDGES=heavy`; pinball's default judges (`judge-opus`, `judge-argon`) are aliases Convergence never renders.

## [0.2.5] - 2026-11-09

### Changed

- Bump pinned Stand-In image to `0.6.2` (`ghcr.io/jeefy/stand-in:0.6.2@sha256:846044102ae1990e924bf43f345addaaa63a5a387c12ad26cb0393f98716d414`).
- Bump pinned Pinball image to `0.7.6` (`ghcr.io/jeefy/pinball:0.7.6@sha256:c34b38b977b057c3a05e4309e68b8ced10da019b377a267e3f740909f6de2fee`).

### Added

- `--extproc-timeout` flag for `init` and `cluster install` (config field `extproc_timeout`) to configure the ext_proc `requestTimeout` (default `5s`, unchanged).

### Fixed

- Empty `request_logs.cost`: the `copilot-completions` provider used `providerOverride: openai`, so agentgateway looked pricing up under `openai/claude-*` (no match). It now uses the `copilot` catalog key.
- OpenShell on Podman: discover and mount the Podman API socket and select OpenShell's Podman driver. On Linux, the OpenShell gateway disables SELinux labeling to access the socket.
- Podman startup: detect `nft` builds without JSON support before Compose starts and stream Compose startup output.

## [0.2.4] - 2026-11-08

### Changed

- Bump pinned Pinball image to `0.7.5` (`ghcr.io/jeefy/pinball:0.7.5@sha256:3e70723ef85ba4f6d80d2f2ece790263f6e7819880c7b4e59ed35b3cbb8b7b17`).

## [0.2.3] - 2026-11-08

### Changed

- Bump pinned Stand-In image to `0.6.1` (`ghcr.io/jeefy/stand-in:0.6.1@sha256:baa7bd9bb4de581114ba9e58ecc36355703a742b2170e73d8324d61f09134a3e`).

### Added

- `init --compose-provider auto|docker-compose|podman-compose`: persist the Podman Compose provider in `config.yaml` for lifecycle commands and `doctor`. Empty or `auto` preserves Podman's default selection; explicit choices override `PODMAN_COMPOSE_PROVIDER` and require the provider on `PATH`. Docker Engine is unchanged.

### Fixed

- Podman: read Compose service names from the `Labels` map instead of the newer `Label` template helper, fixing running-service checks on older distro releases such as Ubuntu 24.04's Podman.

## [0.2.2] - 2026-11-08

### Added

- `convergence insights`: analyze 1d, 7d, or 30d of prompt, completion, token usage, and Pinball tier allocation patterns from agentgateway's Postgres request log (`request_logs` and `request_log_payloads`). Identifies context bloat, heavy-tier over-utilization, and rapid retry loops. Optional `--coach` synthesizes actionable recommendations via the Convergence gateway (`POST /v1/chat/completions`). Supports `--json` for machine-readable output.

## [0.2.1] - 2026-11-08

### Fixed

- Compose: keep the OpenShell gateway under `services` and declare optional named volumes in one mapping, fixing startup with the Copilot provider and OpenShell sandbox enabled together.

## [0.2.0] - 2026-11-08

### Added

- `models.yaml` passthrough entries accept `base_url` and `api_key_env` (env var name) for custom endpoints such as Ollama Cloud; one gateway provider per distinct endpoint (local and cluster). Local keys go to the 0600 `.env`; cluster keys come from a user-created Secret (`cluster install --passthrough-secret`, default `convergence-passthrough`). Cluster installs now allow `ollama` passthrough when `base_url` is set.
- `init --standin-patterns-file` and `cluster install --standin-patterns-file`: use a custom Stand-In `patterns.yaml` (compose bind mount / ConfigMap `stand-in-patterns`, plus `STANDIN_PATTERNS_FILE`). Replaces the image default; unset is unchanged.
- `models.yaml`: optional `passthrough:` block (`name: {provider: copilot|openai|anthropic|ollama, model: id}`) exposes fixed model names that bypass Pinball (public `llm.models` entries; local and cluster). Names colliding with `auto`/`pinball`/`light`/`standard`/`heavy`/`pinball-*`, unknown providers and empty models are errors. Stand-In still scrubs every request.
- `cluster install --request-log-postgres` (in-cluster Postgres with generated password) or `--request-log-db <dsn>`: agentgateway writes its request log to Postgres with LLM logging `full` (stores prompts and completions after Stand-In scrubbing) and the optimizer CronJob reads the same Secret `pinball-optimizer-db` via `PINBALL_OPT_DB_DSN`. Off by default; the DSN is never rendered into a ConfigMap. New `--request-log-postgres-password`.
- agentgateway now returns Pinball's pick on every `auto`/`pinball` response as headers `x-pinball-model`, `x-pinball-tier` and `x-pinball-reason` (a `transformations.response` policy reading ext_authz metadata; local and cluster). The body's `model` still names the real upstream model, so clients that compare it to the requested `auto` can read the header instead of reporting a route change.
- `models.yaml`: optional `pinball:` block (`thresholds`, `scoring`) overrides Pinball's score cutoffs and per-signal points, rendered into `pinball.yaml` (local and cluster ConfigMap). Defaults unchanged; unknown keys and invalid cutoff order are errors.

### Fixed

- Podman: detect running services with native `podman ps` and Compose labels instead of Docker-only `compose ps --services --filter` flags. Running-service check failures now include stderr.
- copilot-api: fold `max_completion_tokens` into `max_tokens` (same preloaded shim). copilot-api fills in `max_tokens` when it is missing, so any request carrying `max_completion_tokens` (the current OpenAI parameter, sent by default by clients like OpenClaw) reached Copilot with both and failed with 400 `max_tokens and max_completion_tokens cannot both be set`.

## [0.1.4] - 2026-11-06

### Changed

- Updated pinned component images:
  - `ghcr.io/jeefy/stand-in` to `0.6.0` (digest `sha256:39db3c7318d49b9e6d2f7bde7ec2c7828522437dddbc02b0a1bd9c07b391c2b2`)
  - `ghcr.io/jeefy/pinball` to `0.7.4` (digest `sha256:0f70fd86a0b1fd1926909df11bfa6d9c388568d01a107c842677af3417be9b4d`)

### Fixed

- copilot-api: disable Bun's 10s `idleTimeout` via a preloaded shim (compose + Kubernetes). Long or silent upstream completions were cut mid-response (`request timed out after 10 seconds`) and agentgateway then waited on a dead upstream. Upstream issue: ericc-ch/copilot-api#153.
- compose: update stand-in service dependency and remove python3 healthcheck for distroless Go container.

## [0.1.3] - 2026-11-05

### Added

- Optional request-log database for local `convergence optimize`: `init --request-log-db <postgres-dsn>` (config `request_log_db`) renders `config.logging.database` plus `frontendPolicies.accessLog.database.llm: full` in the compose config; the DSN is passed via `.env`. `convergence optimize` falls back to it when `--db-dsn` is not given. Off by default. `full` stores prompts and completions.
- agentgateway `frontendPolicies.accessLog.add` now records `pinball.model`, `pinball.tier` and `pinball.reason` (null when the request was not routed by Pinball), in the compose and Kubernetes configs. `pinball-optimize` filters on these attributes in the request log.
- Optional OTLP trace export: `otel.endpoint` (+ `protocol` grpc|http, `sample`) in config, `--otel-endpoint/--otel-protocol/--otel-sample` on `init` and `cluster install`. Renders `frontendPolicies.tracing` (service `convergence`, attributes `caller`, `pinball.tier|model|reason`) for compose and Kubernetes. Unset = rendered files unchanged.
- Model tier routes: virtual models `light`, `standard`, and `heavy` exposed directly in gateway configs without requiring `x-pinball-tier` header overrides.
- Client configs: support for Zed (`convergence config zed`) and Cline / Roo Code (`convergence config cline`), plus tier model exports (`convergence/light`, `convergence/standard`, `convergence/heavy`) for OpenCode, Continue, and Hermes.
- Model mapping: support multi-provider mixing in `models.yaml` (e.g. `ollama:` for light tier and `anthropic:` for heavy tier) and dynamic upstream instantiation in gateway templates.
- Kubernetes: `--ingress-host` and `--enable-pdb` flags on `convergence cluster install` for optional Ingress and PodDisruptionBudgets, plus `convergence cluster logs` command to stream in-cluster pod logs.
- Security: image SHA256 digest pinning for immutable releases across agentgateway, Stand-In, Pinball, and copilot-api.
- Lifecycle & compose cleanup: added `convergence down` and `convergence prune` commands for local Docker/Podman compose environments.
- Observability: latency quantiles (P50, P90, P99) in `convergence stats` and bundled sample Grafana dashboard in `docs/grafana-dashboard.json`.
- Sandboxing: `-v` / `--mount` flag for `convergence sandbox run` to pass workspace directories into nono, OpenShell, and Docker Sandboxes (`sbx`).

### Fixed

- Test suite: podman smoke test no longer requires a live OpenShell docker socket.

## [0.1.2] - 2026-11-05

### Added

- Pluggable agent sandboxing runtime support (`internal/sandbox`): `none`, `openshell`, `nono`, `docker-sbx`, and `k8s-agent`.
- `nono` driver for kernel-enforced process sandboxing (Landlock on Linux, Seatbelt on macOS) with ~0ms latency and zero background daemons.
- `convergence sandbox run <agent> [args...]` command to launch agents directly inside the configured sandbox with inference pre-routed to Convergence.
- `convergence init --sandbox <runtime>` flag to choose or enable sandbox integration.
- Optional NVIDIA OpenShell gateway compose service bundled with schema v2 `openshell.toml` config, exposing ports 17670 (gRPC/HTTP) and 17671 (healthz).
- `convergence sandbox status` and `convergence sandbox list` CLI commands.

## [0.1.1] - 2026-11-05

### Added

- Component image overrides: `--image-agentgateway`, `--image-standin`, `--image-pinball`, and `--image-copilot-api` flags for `init` and `cluster install` (persisted in `config.yaml` for local compose), allowing deployment of specific image tags, dev SHAs, or custom registries.

### Fixed

- Kubernetes mode: agentgateway is started with `--file` instead of `-f`; with `-f` v1.6.0 exits in-cluster (`configuration is required when running in Kubernetes`).

## [0.1.0] - 2026-11-05

### Added

- Local stack on Docker or Podman compose: agentgateway `v1.6.0`, Stand-In `0.4.1`, Pinball `0.7.3`, and copilot-api `v0.7.0` for the Copilot provider.
- Providers: `copilot`, `direct` (OpenAI and Anthropic keys) and `ollama`.
- `init` with interactive prompts or flags (`--yes`, `--provider`, `--engine`, `--copilot-token`, `--openai-key`, `--anthropic-key`, `--ollama-url`, `--ollama-model-light/-standard/-heavy`, `--bind-address`, `--port`, `--fail-open`).
- `login copilot` (GitHub device flow).
- Lifecycle commands: `start`, `stop`, `restart`, `status`, `logs [-f] [service]`, `upgrade`, `uninstall [--purge] [--yes]`.
- Stand-In is fail-closed by default (requests are rejected when it's down). `init --fail-open` and `cluster install --fail-open` opt into unscrubbed passthrough. Pinball ext_authz stays `allow` and falls back to the standard-tier model.
- `doctor [--e2e] [--json]` self-diagnosis, and `status [--json]` per-service probes with loud warnings when Stand-In is down or fail-open.
- Model mapping: `models`, `models init`, `~/.convergence/models.yaml` overrides, `cluster install --models-file`, per-tier Ollama flags, and a missing-model warning at `start` (`start --strict` to fail).
- Direct providers: OpenAI and Anthropic. Gemini isn't supported.
- Compose hardening: `json-file` log rotation, healthchecks for Stand-In, Pinball and copilot-api, secrets in a `0600` `.env`, loopback-only port binding.
- Kubernetes: the Stand-In Service exposes metrics on 9102.
- Dev: `make lint` (golangci-lint, gofmt, vet), race-detector tests and a coverage summary in CI; the local e2e sends a real chat through a stub upstream and the KinD e2e does a real chat plus a fail-closed check.
- `stats [--json] [--watch]` built from the Prometheus metrics of the gateway, Stand-In and Pinball.
- Client configuration with backup and restore: OpenCode, oh-my-opencode, Claude Code, Aider, Goose, Hermes Agent, Continue, plus `config copilot` shell exports. `config list`, `--dry-run`, `--gateway-url`, `restore --force`.
- Kubernetes mode: `cluster install`, `cluster status`, `cluster uninstall`, with NetworkPolicies and an optional weekly Pinball optimizer CronJob.
- Experimental `optimize --db-dsn` for on-demand Pinball evaluation.
- Local ports bind to `127.0.0.1` by default (`bind_address`); config and secret files are written `0600`.
- Installer with sha256 verification against `checksums.txt`, version pinning and no `sudo`.
- Release builds for linux and darwin on amd64 and arm64, with checksums and SBOMs.

### Known limitations

- Windows isn't supported. WSL2 works.
- The Pinball optimizer is experimental and opt-in.
- Container images are pinned by tag, not by digest.
- The model ids in the default mapping are defaults; providers rename models, so override them in `models.yaml` if a request fails with an unknown model.
- The gateway has no authentication of its own: any local process can use your keys via `127.0.0.1:3000`, and in Kubernetes any pod the NetworkPolicy allows can.
- Fail-closed means a Stand-In outage rejects requests.

[Unreleased]: https://github.com/jeefy/convergence/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/jeefy/convergence/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/jeefy/convergence/compare/v0.4.3...v0.5.0
[0.4.3]: https://github.com/jeefy/convergence/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/jeefy/convergence/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/jeefy/convergence/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/jeefy/convergence/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/jeefy/convergence/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/jeefy/convergence/compare/v0.2.5...v0.3.0
[0.2.5]: https://github.com/jeefy/convergence/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/jeefy/convergence/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/jeefy/convergence/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/jeefy/convergence/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/jeefy/convergence/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/jeefy/convergence/compare/v0.1.4...v0.2.0
[0.1.4]: https://github.com/jeefy/convergence/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/jeefy/convergence/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/jeefy/convergence/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/jeefy/convergence/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jeefy/convergence/releases/tag/v0.1.0
