# Architecture

## Request flow

Every request to the gateway goes through the same four steps.

```
 [client / sandboxed agent] ──► agentgateway :3000 /v1
                                  │
                                  │ 1. ext_proc (request)  ──► Stand-In :9101 (gRPC)
                                  │      secrets in the body are replaced with <<secret-...>> tokens
                                  │
                                  │ 2. ext_authz            ──► Pinball :9201
                                  │      scores the sanitized prompt, sets the model tier
                                  │      (light / standard / heavy) and target model
                                  │
                                  │ 3. route to upstream
                                  │      copilot: copilot-api :4141 ──► GitHub Copilot
                                  │      direct:  api.openai.com / api.anthropic.com
                                  │      ollama:  Ollama (host.docker.internal:11434 by default)
                                  │
                                  │ 4. ext_proc (response) ──► Stand-In
                                  │      tokens in the response are restored to the original values
                                  ▼
 [client / sandboxed agent] ◄── response
```

Failure behavior is set in the gateway config:

- Stand-In ext_proc: `failureMode: failClosed` by default. If Stand-In is unreachable, requests are rejected. `init --fail-open` (local) or `cluster install --fail-open` switch it to `failOpen`, where requests continue **unscrubbed**.
- Pinball ext_authz: `failureMode: allow`. If Pinball is unreachable, the request goes to the standard-tier model. Pinball only chooses a model, so this doesn't affect confidentiality.

Anthropic Messages requests (`/v1/messages`) are translated by agentgateway, so Claude Code and OpenAI-style clients share the same endpoint.

## Local mode (docker/podman compose)

Docker uses its compose plugin. Podman uses `podman compose`, which delegates to the provider selected by `compose_provider` in `config.yaml`: `docker-compose` (needs the Podman API socket) or `podman-compose` (talks to Podman directly). Empty or `auto` leaves the choice to Podman (`PODMAN_COMPOSE_PROVIDER`, `containers.conf`). `doctor` probes the configured provider.

Before `start`, Convergence lists the stack's running services (Docker: `compose ps`; Podman: `podman ps` filtered by the compose project labels) so its own published ports are not reported as conflicts. On Linux with netavark it also checks that `nft` has JSON support.

With the OpenShell sandbox, `start` discovers the engine's API socket, mounts it at `/var/run/docker.sock` in the OpenShell gateway container and selects the matching OpenShell driver. On Linux with Podman that container runs with `label=disable` so it can reach the socket; the other services keep their SELinux labels. Whoever holds the socket controls all of the user's containers.

### Ports

All host ports bind to `bind_address` (default `127.0.0.1`).

| Service | Container port | Host port | Purpose |
|---|---|---|---|
| agentgateway | 3000 | `gateway_port` (default 3000) | OpenAI-compatible `/v1` |
| agentgateway | 15020 | 15020 | Prometheus metrics |
| Stand-In | 9101 | not published | ext_proc gRPC (internal) |
| Stand-In | 9102 | 9102 | Prometheus metrics |
| Pinball | 9201 | 9201 | ext_authz and metrics |
| copilot-api | 4141 | 4141 | Copilot upstream (copilot provider only) |
| openshell | 17670 | 17670 | OpenShell gRPC/HTTP gateway (openshell sandbox only) |
| openshell health | 17671 | 17671 | OpenShell health check (openshell sandbox only) |

### Files under `~/.convergence`

Secrets live only in `config.yaml` and `.env`. The rendered files are world-readable because non-root containers bind-mount them; they contain `${VAR}` references, never values.

| Path | Mode | Contents |
|---|---|---|
| `config.yaml` | 0600 | Engine, provider, sandbox, ports, `bind_address`, provider credentials (plaintext) |
| `.env` | 0600 | Secrets injected into compose (`STANDIN_SALT`, API keys, Copilot token, passthrough keys) |
| `docker-compose.yaml` | 0644 | Rendered compose stack |
| `agentgateway.yaml` | 0644 | Rendered gateway config (listeners, ext_proc/ext_authz, backends) |
| `pinball.yaml` | 0644 | Model catalog and tier assignments, generated from the model mapping |
| `openshell.toml` | 0644 | Rendered OpenShell gateway config (openshell sandbox only) |
| `models.yaml` | user | Optional tier to upstream model overrides (`convergence models init`) |
| `skill-scan.json` | user | Cached result of the last `skills scan` |
| `backups/` | user | Request-log dumps taken by `cluster install` before an upgrade (last 5) |
| `logs/` | 0700 | Gateway access logs |

copilot-api and the OpenShell gateway are optional compose services with named volumes `copilot-data` and `openshell-data`. `convergence uninstall --purge` deletes the whole directory.

## Kubernetes mode

`convergence cluster install` renders one manifest bundle into the target namespace (default `convergence`).

| Resource | Name | Notes |
|---|---|---|
| Namespace | `<namespace>` | |
| Secret | `stand-in` | Stand-In salt |
| Secret | `convergence-copilot` | Copilot token (copilot provider only) |
| Secret | `agentgateway-config` | Rendered gateway config, including direct-provider API keys |
| ConfigMap | `pinball-config` | Model catalog (honors `--models-file`) |
| ConfigMap | `stand-in-patterns` | Only with `--standin-patterns-file` |
| Deployment + Service | `agentgateway` | ports 3000 (http), 15020 (metrics) |
| Deployment + Service | `stand-in` | ports 9101 (gRPC), 9102 (metrics) |
| Deployment + Service | `pinball` | port 9201 |
| Deployment + Service | `copilot-api` | port 4141, copilot provider only |
| StatefulSet + Service + PVC | `request-log-postgres` | Only with `--request-log-postgres` (5Gi) |
| Secret | `pinball-optimizer-db` | Request-log / optimizer DSN; with `--request-log-*` or `--optimizer-dsn` |
| PVC + CronJob | `pinball-overlay`, `pinball-optimize` | Optimizer; with `--request-log-*` or `--optimizer-dsn`; schedule `--opt-schedule` (default `0 3 * * 0`) |
| CronJob | `request-log-retention` | Only with `--request-log-retention` |
| NetworkPolicy | `default-deny-ingress` plus allow rules | Only the gateway reaches Stand-In, Pinball and copilot-api; only the gateway and optimizer reach Postgres |
| Ingress | `agentgateway` | Only with `--ingress-host` |
| PodDisruptionBudget | `agentgateway-pdb`, `stand-in-pdb`, `pinball-pdb` | Only with `--enable-pdb` |

Omitted secrets (`--copilot-token`, Stand-In salt, Postgres password) are read back from the previous install's Secrets, so re-running `cluster install` does not rotate them.

Services are `ClusterIP`. Reach the gateway with `kubectl port-forward svc/agentgateway 3000:3000` or put your own Ingress or Gateway in front of it. The gateway has no client authentication, so restrict who can reach it.

## Pinned images

| Component | Image |
|---|---|
| agentgateway | `cr.agentgateway.dev/agentgateway:v1.6.0@sha256:482921556876a503ad3675b29223b1897b63a316983e46797ff99ebf83a2a6a2` |
| Stand-In | `ghcr.io/jeefy/stand-in:0.6.3@sha256:e1c150aada9e059601e7c0552b0d9fa30921c0e92f343fcba0a5b66f08066f01` |
| Pinball | `ghcr.io/jeefy/pinball:0.7.6@sha256:c34b38b977b057c3a05e4309e68b8ced10da019b377a267e3f740909f6de2fee` |
| copilot-api | `ghcr.io/ericc-ch/copilot-api:v0.7.0@sha256:acf4f91931b78b7a4366e72447cfd170a3f3cf6388b9189b098c7236510fbb29` |
| openshell | `ghcr.io/nvidia/openshell/gateway:latest` (not pinned) |
| SkillSpector | built locally by `skills build` as `localhost/convergence-skillspector:v2.13.4` from commit `54824463814a0be01360792523ec7245a69190fc` |

`--image-*` flags on `init` and `cluster install` override these.

## Model mapping

`internal/models` is the single source for the pinball model pool, tiers, thresholds and per-provider upstream ids. `pinball.yaml` and the gateway's `auto` virtual model are both generated from it. Defaults are overridden by `~/.convergence/models.yaml` (local) or `cluster install --models-file`. See [Model routing](../README.md#model-routing) in the README.
