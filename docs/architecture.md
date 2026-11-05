# Architecture

## Request flow

Every request to the gateway goes through the same four steps.

```
client ──► agentgateway :3000 /v1
             │
             │ 1. ext_proc (request)  ──► Stand-In :9101 (gRPC)
             │      secrets in the body are replaced with <<secret-...>> tokens
             │
             │ 2. ext_authz            ──► Pinball :9201
             │      scores the sanitized prompt, sets the model tier
             │      (light / standard / deep) and target model
             │
             │ 3. route to upstream
             │      copilot: copilot-api :4141 ──► GitHub Copilot
             │      direct:  api.openai.com / api.anthropic.com
             │      ollama:  Ollama (host.docker.internal:11434 by default)
             │
             │ 4. ext_proc (response) ──► Stand-In
             │      tokens in the response are restored to the original values
             ▼
client ◄── response
```

Failure behavior is set in the gateway config:

- Stand-In ext_proc: `failureMode: failOpen`. If Stand-In is unreachable, requests continue **unscrubbed**.
- Pinball ext_authz: `failureMode: allow`. If Pinball is unreachable, the request uses the model the client asked for.

Anthropic Messages requests (`/v1/messages`) are translated by agentgateway, so Claude Code and OpenAI-style clients share the same endpoint.

## Local mode (docker/podman compose)

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

### Files under `~/.convergence`

| Path | Mode | Contents |
|---|---|---|
| `config.yaml` | 0600 | Engine, provider, ports, `bind_address`, provider credentials (plaintext) |
| `.env` | 0600 | Secrets injected into compose (`STANDIN_SALT`, API keys, Copilot token) |
| `docker-compose.yaml` | 0600 | Rendered compose stack, no secrets |
| `agentgateway.yaml` | 0600 | Rendered gateway config (listeners, ext_proc/ext_authz, backends) |
| `pinball.yaml` | 0600 | Model catalog and tier assignments |
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
| ConfigMap | `pinball-config` | Model catalog |
| Deployment + Service | `agentgateway` | ports 3000 (http), 15020 (metrics) |
| Deployment + Service | `stand-in` | ports 9101 (gRPC), 9102 (metrics) |
| Deployment + Service | `pinball` | port 9201 |
| Deployment + Service | `copilot-api` | port 4141, copilot provider only |
| Secret, PVC, CronJob | `pinball-optimizer-db`, `pinball-overlay`, `pinball-optimize` | only with `--optimizer-dsn`; schedule `0 3 * * 0` unless `--opt-schedule` is set |
| NetworkPolicy | `default-deny-ingress` plus allow rules | only the gateway can reach Stand-In, Pinball and copilot-api |

Services are `ClusterIP`. Reach the gateway with `kubectl port-forward svc/agentgateway 3000:3000` or put your own Ingress/Gateway in front of it. The gateway has no client authentication, so restrict who can reach it.

## Pinned images

| Component | Image |
|---|---|
| agentgateway | `cr.agentgateway.dev/agentgateway:v1.6.0` |
| Stand-In | `ghcr.io/jeefy/stand-in:0.4.1` |
| Pinball | `ghcr.io/jeefy/pinball:0.7.2` |
| copilot-api | `ghcr.io/ericc-ch/copilot-api:v0.7.0` |

Tags are pinned, digests aren't yet.
