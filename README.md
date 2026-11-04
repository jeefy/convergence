# Convergence

**Convergence** is a unified, turnkey distribution for local and cluster AI gateway infrastructure:
- **AgentGateway** (`v1.6.0`): Enterprise-grade LLM routing, cost catalog, and unified telemetry.
- **Stand-In** (`v0.4.1`): Secret tokenization (`<<secret-...>>`) on ingress and restoration on egress.
- **Pinball** (`v0.6.1`): Autonomous model router scoring prompt signals to steer `auto`/`pinball` requests to cost-optimized model tiers.
- **Copilot-API**: Optional GitHub Copilot subscription bridge for OpenAI-compatible tools.

## One-Line Install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
```

Or with Homebrew:

```bash
brew install jeefy/tap/convergence
```

## Quick Start (Local Workstation)

```bash
# 1. Interactive setup (detects Docker/Podman, sets up keys/Copilot)
convergence init

# 2. Launch runtime containers
convergence start

# 3. Check health and connectivity
convergence status

# 4. View real-time token, cost, and routing stats
convergence stats
```

## Quick Start (Kubernetes Cluster)

```bash
# Install directly to your active Kubernetes cluster
convergence cluster install --namespace=convergence

# Check cluster status
convergence cluster status
```
