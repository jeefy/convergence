# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-11-05

### Added

- Local stack on Docker or Podman compose: agentgateway `v1.6.0`, Stand-In `0.4.1`, Pinball `0.7.2`, and copilot-api `v0.7.0` for the Copilot provider.
- Providers: `copilot`, `direct` (OpenAI and Anthropic keys) and `ollama`.
- `init` with interactive prompts or flags (`--yes`, `--provider`, `--engine`, `--copilot-token`, `--openai-key`, `--anthropic-key`, `--ollama-url`, `--port`).
- `login copilot` (GitHub device flow).
- Lifecycle commands: `start`, `stop`, `restart`, `status`, `logs [-f] [service]`, `upgrade`, `uninstall [--purge] [--yes]`.
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
- Stand-In runs fail-open: if it's unavailable, requests are sent upstream unscrubbed.

[Unreleased]: https://github.com/jeefy/convergence/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/jeefy/convergence/releases/tag/v0.1.0
