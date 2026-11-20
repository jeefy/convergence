# Operations

## Status and doctor

`convergence status` probes each service: container state plus a health check (agentgateway is probed over its port because its image has no shell). It exits non-zero if any required service is down; `--json` gives machine-readable output. If Stand-In is down or the stack is fail-open, it prints a loud warning that secret scrubbing is not active.

`convergence doctor` runs these checks and exits 1 if any fails (`--timeout` sets the overall limit, default 1m; `--gateway-url` overrides the gateway URL):

| Check | Meaning |
|---|---|
| config | `~/.convergence/config.yaml` parses and `.env` is mode `0600` |
| container engine | docker/podman answers and the configured compose provider is available |
| service `<name>` | each service is running and healthy |
| bind address | warns if `bind_address` exposes the gateway beyond loopback |
| port `<n>` (`<name>`) | each published port accepts connections |
| stand-in metrics | Stand-In `/metrics` is scrapeable and in `enforce` mode |
| pinball /check (`<probe>`) | each probe prompt gets a tier and model back from Pinball |
| provider reachability | the upstream (copilot-api, Ollama, ...) answers |
| ollama models | warns when a model mapped to Ollama isn't installed |
| stand-in fail mode | warns if fail-open is configured or rendered |
| skills | result of the last `skills scan` of installed skills: warns when never scanned, changed since, or unsafe; fails on unsafe with `skill_scan: block`; informational while `skill_scan` is `off` |
| e2e round trip | only with `--e2e` |

`--e2e` additionally sends one real request (model `auto`, `max_tokens: 16`) containing a fake GitHub token through the gateway. With a real provider this costs a few tokens, so it's off by default.

## Stats

`convergence stats` scrapes the Prometheus endpoints of agentgateway (`:15020`), Stand-In (`:9102`) and Pinball (`:9201`) and summarizes request counts, tokens, latency quantiles (p50/p90/p99), tier decisions and redactions. Counters reset when containers restart. It shows tokens and tiers, not money. `--json` prints machine-readable output; `--watch` refreshes.

In cluster mode, `cluster install --service-monitor` renders prometheus-operator `ServiceMonitor`s for the three metrics ports plus a NetworkPolicy `allow-monitoring-to-metrics` that lets pods in `--monitoring-namespace` (default `monitoring`) scrape them through the default-deny policy. The monitors carry `release=prometheus-operator` unless you pass `--service-monitor-label key=value` (repeatable) to match your Prometheus `serviceMonitorSelector`. The flag fails before applying anything when the `servicemonitors.monitoring.coreos.com` CRD is absent.

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

## Troubleshooting

**Port already in use.** Another process has `3000` (or `4141`, `9102`, `9201`, `15020`). Find it with `ss -ltnp | grep 3000` (Linux) or `lsof -i :3000` (macOS). `convergence init --port 3100` moves the gateway; the other ports are fixed.

**Podman rootless errors.** Check that `podman info` works. Docker Compose and OpenShell need Podman's user API socket (`systemctl --user enable --now podman.socket`); `podman-compose` does not. SELinux denials on mounted config files are handled with the `z` volume label; if you edited the compose file by hand, re-run `convergence init`.

**Podman reports `nft` JSON support is not compiled in.** Netavark needs an `nft` binary with JSON support. On Linux, a Homebrew `nftables` can shadow the system binary without providing it. Check `command -v nft`; if it points to Homebrew and a suitable system `nft` exists, `brew unlink nftables` and retry `convergence start` (`brew link nftables` undoes it; other programs using Homebrew's `nft` are affected too).

**OpenShell cannot connect to the Podman API socket.** Check `podman info --format '{{.Host.RemoteSocket.Path}}'` and enable the local socket. A remote socket path must also be reachable by the local compose engine.

**Health check failing.** Run `convergence status`, then `convergence logs <service>`. Common causes: an image pull failed, `STANDIN_SALT missing` (re-run `convergence init`), or a bad provider key (gateway logs show 401s from upstream).

**Copilot auth.** `401`/`403` from `copilot-api` means the token is missing, expired, or the account has no Copilot subscription. Run `convergence login copilot` again, then `convergence restart`. `convergence logs copilot-api` shows the auth result.
