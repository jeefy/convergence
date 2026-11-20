# Convergence

Run an AI gateway on your workstation or in Kubernetes and point your coding agents at it: one endpoint, secrets scrubbed before they leave your machine, and a model picked per request.

Convergence deploys and wires together [agentgateway](https://agentgateway.dev/docs/) (gateway), [Stand-In](https://github.com/jeefy/stand-in) (secret tokenizing) and [Pinball](https://github.com/jeefy/pinball) (tier routing), writes client configs, and can run agents in a sandbox.

```
client / sandboxed agent ──► agentgateway :3000 /v1
                               ├─ Stand-In  tokenize secrets (and restore them in responses)
                               ├─ Pinball   choose model tier
                               └─► Copilot (copilot-api) │ OpenAI / Anthropic │ Ollama
```

## Quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
convergence login copilot     # Copilot provider only
convergence init              # engine, provider, optional sandbox
convergence start
convergence config opencode   # or claude-code, aider, goose, ...
```

Needs Linux or macOS with Docker or Podman (compose), or `kubectl` for a cluster. Windows: see [Installation](docs/installation.md#windows).

## Documentation

| Doc | Contents |
|---|---|
| [Installation](docs/installation.md) | Prerequisites, installers, Windows, upgrades |
| [Quickstart](docs/quickstart.md) | Local, sandbox, Kubernetes, Podman notes |
| [Providers and model routing](docs/models.md) | Providers, tiers, `models.yaml`, passthrough models |
| [Client configuration](docs/clients.md) | Supported clients and what is written |
| [CLI reference](docs/cli.md) | Every command |
| [Operations](docs/operations.md) | `status`, `doctor`, `stats`, tracing, troubleshooting |
| [Insights and request log](docs/insights.md) | `insights`, optimizer, retention, backup |
| [Skill scanning](docs/skills.md) | SkillSpector vetting |
| [System prompt](docs/system-prompt.md) | Optional guardrail prompt |
| [Security model](docs/security.md) | What is and is not protected |
| [Architecture](docs/architecture.md) | Request flow, ports, files, Kubernetes resources, pinned images |

## Project

[Contributing](CONTRIBUTING.md) · [Changelog](CHANGELOG.md) · [Security policy](SECURITY.md) · [Code of Conduct](CODE_OF_CONDUCT.md) · Apache-2.0, see [LICENSE](LICENSE)
