# CLI reference

Flags listed here are the common ones; `convergence <command> --help` is the full list.

| Command | Description |
|---|---|
| `init [--yes] [--bind-address A] [--compose-provider P] [--engine docker\|podman] [--provider P] [--sandbox S] [--port N] [--fail-open] [--discover-models] ...` | Write `~/.convergence/config.yaml` and render the compose stack. Interactive unless `--yes`. Credentials: `--copilot-token`, `--openai-key`, `--anthropic-key`, `--ollama-url`. |
| `login copilot` | GitHub device-flow login; stores the token for the Copilot provider. |
| `start [--strict]` / `stop` / `restart` / `down` | Manage the local containers. `--strict` fails instead of warning when Ollama lacks a mapped model. `down` also removes containers and networks. |
| `upgrade` | Pull the image versions pinned by this binary and recreate the stack. |
| `prune` | Remove dangling images, stopped containers and unused networks. |
| `uninstall [--purge] [--yes]` | Stop and remove the stack. `--purge` also deletes `~/.convergence`. |
| `status [--json]` | Container state and health probe per service; exits non-zero if a service is down. |
| `doctor [--e2e] [--json] [--timeout D]` | Self-checks; see [Status and doctor](operations.md#status-and-doctor). |
| `logs [-f] [service]` | Logs for all services or one of `agentgateway`, `stand-in`, `pinball`, `copilot-api`. |
| `models` / `models init` | Show the tier to upstream mapping / write a starter `~/.convergence/models.yaml`. |
| `stats [--json] [--watch] [--interval N]` | Request counts, tokens, latency quantiles, tier decisions and redactions from Prometheus metrics. |
| `insights [--since 1d\|7d\|30d] [--coach] [--json]` | Usage, estimated cost and prompt patterns from agentgateway's request log; see [Insights](insights.md#insights). |
| `config list` / `config <client> [--dry-run] [--gateway-url URL]` / `config <client> restore [--force]` | See [Client configuration](clients.md). |
| `sandbox list` / `sandbox status [--json]` | Supported sandbox runtimes / status of the configured one. |
| `sandbox run [-v\|--mount path] <agent> [args...]` | Run an agent inside the configured sandbox with inference routed through the gateway. |
| `skills build` / `skills scan [path...] [--llm] [--model M] [--build] [--no-save]` | Build the SkillSpector image / scan installed agent skills (`--build` builds the image if missing, `--no-save` skips updating the cached result); see [Skill scanning](skills.md). |
| `cluster install [--namespace NS] [--provider copilot\|direct] [--models-file F] [--fail-open] [--discover-models] [--dry-run] ...` | Apply manifests to the current kube context; `--dry-run` prints them. Omitted secrets (Copilot token, Stand-In salt, Postgres password) are reused from a previous install in the namespace. |
| `cluster status` / `cluster logs [-f] [--tail N] [service]` / `cluster uninstall [--yes]` | Rollout and pod health / pod logs / delete the resources. |
| `cluster backup --out DIR` / `cluster restore --from FILE [--yes]` | See [Backup and restore](insights.md#backup-and-restore). |
| `optimize --db-dsn DSN [--dry-run] [--check-revert]` | **Experimental.** Run the Pinball optimizer against a request-log database. `--check-revert` checks whether a recent re-ranking regressed and reverts it. |
| `version` | Version, commit, build date. |

Image overrides (`--image-agentgateway`, `--image-standin`, `--image-pinball`, `--image-copilot-api`, and `--image-openshell` for `init` only) exist on both `init` and `cluster install`.

`--extproc-timeout` (config `extproc_timeout`, default `5s`) sets agentgateway's `requestTimeout` for the ext_proc policy: how long it waits for the first gRPC response from Stand-In (agentgateway#4146). Raise it for very large requests; it must be a positive Go duration such as `60s`. In the cluster, Stand-In is CPU-bound on large bodies, so raise `--standin-cpu-limit` (default `500m`; request `50m` via `--standin-cpu-request`) before raising the timeout.
