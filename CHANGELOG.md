# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/jeefy/convergence/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jeefy/convergence/releases/tag/v0.1.0
