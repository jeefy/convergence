# Quickstart

### Local

```bash
convergence login copilot     # only for the Copilot provider
convergence init              # pick engine, provider, and optional sandbox; writes ~/.convergence
convergence start
convergence status
convergence config opencode   # or claude-code, aider, goose, ...
```

Non-interactive:

```bash
convergence init --yes --engine podman --provider direct \
  --openai-key "$OPENAI_API_KEY" --anthropic-key "$ANTHROPIC_API_KEY" --port 3000
```

With a sandbox:

```bash
convergence init --yes --provider direct --sandbox nono
convergence start
convergence sandbox run opencode
```

`sandbox run hermes` works too. Hermes ignores `OPENAI_BASE_URL`, so for a bare `hermes`, `hermes chat` or leading flags Convergence runs `hermes chat --provider custom --base-url <gateway> --api-key convergence`. Other Hermes subcommands are passed through unchanged.

### Kubernetes

```bash
convergence cluster install --namespace convergence --provider copilot --copilot-token "$GH_TOKEN"
convergence cluster status --namespace convergence
kubectl -n convergence port-forward svc/agentgateway 3000:3000
convergence config opencode --gateway-url http://127.0.0.1:3000
```

### Podman notes

`podman compose` delegates to an external provider. `init --engine podman --compose-provider podman-compose` selects the daemonless provider; `--compose-provider docker-compose` selects Docker Compose talking to Podman's API socket. The choice is saved as `compose_provider` in `config.yaml` and used by the lifecycle commands and `doctor`. Omitting the flag keeps the saved choice; `auto` restores Podman's own selection (`PODMAN_COMPOSE_PROVIDER`, `containers.conf`). An explicit choice overrides that variable and must be on `PATH`. Docker Engine always uses its compose plugin.

The OpenShell sandbox (`--sandbox openshell`, any provider including Copilot) needs Podman's API socket: `systemctl --user enable --now podman.socket` on rootless Linux. Convergence mounts the discovered socket into the OpenShell gateway container and selects OpenShell's Podman driver. On Linux, SELinux labeling is disabled for that container so it can reach the socket. The socket grants control over all of your Podman containers.
