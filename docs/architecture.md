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

- Stand-In ext_proc: `failureMode: failClosed` by default. If Stand-In is unreachable, requests are rejected. `init --fail-open` (local) or `cluster install --fail-open` (cluster) switch it to `failOpen`, where requests continue **unscrubbed**.
- Pinball ext_authz: `failureMode: allow`. If Pinball is unreachable, the request goes to the fallback target (the standard-tier model). Pinball only chooses a model, so this doesn't affect confidentiality.

Anthropic Messages requests (`/v1/messages`) are translated by agentgateway, so Claude Code and OpenAI-style clients share the same endpoint.

## Local mode (docker/podman compose)

Podman lifecycle commands use `podman compose`, which delegates to an external provider. `compose_provider` in `config.yaml` may select `docker-compose` (requires the Podman API socket) or `podman-compose` (invokes Podman directly). Empty or `auto` keeps Podman's environment/config-based selection; explicit choices override `PODMAN_COMPOSE_PROVIDER`. Docker Engine always uses its Compose plugin. `doctor` probes the configured provider.

Before starting the stack, Convergence checks for running services so it does not treat its own published ports as conflicts. Docker uses `compose ps`; Podman uses native `ps` filtered by the rendered Compose file's `com.docker.compose.project.config_files` label and running state, reading service names from the `Labels` map's `com.docker.compose.service` entry. This works on older Podman releases without the `Label` template helper. Both `podman-compose` and Docker Compose set these labels. Failed checks include stderr.

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

The optional copilot-api and OpenShell gateway are Compose services. Their named volumes, `copilot-data` and `openshell-data`, share one top-level `volumes` mapping when both are enabled.

| Path | Mode | Contents |
|---|---|---|
| `config.yaml` | 0600 | Engine, provider, sandbox, ports, `bind_address`, provider credentials (plaintext) |
| `.env` | 0600 | Secrets injected into compose (`STANDIN_SALT`, API keys, Copilot token) |
| `docker-compose.yaml` | 0600 | Rendered compose stack, no secrets |
| `agentgateway.yaml` | 0600 | Rendered gateway config (listeners, ext_proc/ext_authz, backends) |
| `pinball.yaml` | 0600 | Model catalog and tier assignments, generated from the model mapping |
| `openshell.toml` | 0644 | Rendered OpenShell gateway config (schema v2, DooD Docker driver, openshell sandbox only) |
| `models.yaml` | user | Optional tier to upstream model overrides (`convergence models init`) |
| `logs/` | 0700 | Gateway access logs |

`convergence uninstall --purge` deletes this directory.

## Kubernetes mode

`convergence cluster install` renders one manifest bundle into the target namespace (default `convergence`).

| Resource | Name | Notes |
|---|---|---|
| Namespace | `<namespace>` | |
| Secret | `stand-in` | Stand-In salt |
| Secret | `convergence-copilot` | Copilot token (copilot provider only) |
| Secret | `agentgateway-config` | Rendered gateway config, including direct-provider API keys |
| ConfigMap | `pinball-config` | Model catalog (honors `--models-file`) |
| Deployment + Service | `agentgateway` | ports 3000 (http), 15020 (metrics) |
| Deployment + Service | `stand-in` | ports 9101 (gRPC), 9102 (metrics, also exposed on the Service) |
| Deployment + Service | `pinball` | port 9201 |
| Deployment + Service | `copilot-api` | port 4141, copilot provider only |
| Secret, PVC, CronJob | `pinball-optimizer-db`, `pinball-overlay`, `pinball-optimize` | only with `--optimizer-dsn`; schedule `0 3 * * 0` unless `--opt-schedule` is set |
| NetworkPolicy | `default-deny-ingress` plus allow rules | only the gateway can reach Stand-In, Pinball and copilot-api |

Services are `ClusterIP`. Reach the gateway with `kubectl port-forward svc/agentgateway 3000:3000` or put your own Ingress/Gateway in front of it. The gateway has no client authentication, so restrict who can reach it.

## Pinned images

| Component | Image |
|---|---|
| agentgateway | `cr.agentgateway.dev/agentgateway:v1.6.0@sha256:482921556876a503ad3675b29223b1897b63a316983e46797ff99ebf83a2a6a2` |
| Stand-In | `ghcr.io/jeefy/stand-in:0.6.0@sha256:39db3c7318d49b9e6d2f7bde7ec2c7828522437dddbc02b0a1bd9c07b391c2b2` |
| Pinball | `ghcr.io/jeefy/pinball:0.7.4@sha256:0f70fd86a0b1fd1926909df11bfa6d9c388568d01a107c842677af3417be9b4d` |
| copilot-api | `ghcr.io/ericc-ch/copilot-api:v0.7.0@sha256:acf4f91931b78b7a4366e72447cfd170a3f3cf6388b9189b098c7236510fbb29` |
| openshell | `ghcr.io/nvidia/openshell/gateway:latest` |

Tags and sha256 digests are pinned for release immutability.

## Model mapping

`internal/models` is the single source for the pinball model pool, tiers, thresholds and per-provider upstream ids. `pinball.yaml` and the gateway's `auto` virtual model are both generated from it. Defaults are overridden by `~/.convergence/models.yaml` (local) or `cluster install --models-file`. See the README's model mapping section.
