# Client configuration

`convergence config <client>` edits the client's config to use the gateway. It writes a `<file>.convergence.bak` backup first; `restore` puts it back (`--force` even if the file changed since). `--dry-run` prints the change without writing. `--gateway-url` overrides the default `http://localhost:<gateway_port>`. `convergence config list` shows the supported clients and whether each currently points at Convergence.

| Client | Command | File changed |
|---|---|---|
| OpenCode | `convergence config opencode [--set-default]` | `~/.config/opencode/opencode.json` |
| oh-my-opencode | `convergence config oh-my-opencode` | Ensures the OpenCode provider (as above); `omo.jsonc` is not modified. Set your agents' `model` to `convergence/auto` or `convergence/pinball`. |
| OpenClaw | `convergence config openclaw [--set-default]` | `~/.openclaw/openclaw.json` |
| Claude Code | `convergence config claude-code` | `~/.claude/settings.json` |
| Aider | `convergence config aider` | `~/.aider.conf.yml` |
| Goose | `convergence config goose` | `~/.config/goose/config.yaml` |
| Hermes Agent | `convergence config hermes [--set-default]` | `config.yaml` in `$HERMES_HOME`, else `%LOCALAPPDATA%\hermes` on Windows, else `~/.hermes` |
| Continue | `convergence config continue` | `~/.continue/config.yaml` |
| Zed | `convergence config zed` | `~/.config/zed/settings.json` |
| Cline / Roo Code | `convergence config cline` | `~/.config/Code/User/settings.json` |
| Paseo | `convergence config paseo` | `config.json` in `$PASEO_HOME`, else `~/.paseo` |
| omp | `convergence config omp` | `models.yml` (or an existing `models.yaml`) in `$PI_CODING_AGENT_DIR`, else `~/.omp/agent` (`$PI_CONFIG_DIR` replaces `.omp`); default profile only |
| Anything else | `convergence config copilot` | Nothing. Prints `OPENAI_*` / `ANTHROPIC_*` exports to `eval` in your shell. |

[Paseo](https://paseo.sh) launches other agents, so `config paseo` adds providers under `agents.providers` and leaves the built-in `claude` and `copilot` alone, so each harness can run with or without the gateway:

- `convergence-claude` (extends `claude`): gateway base URL, Claude Code's haiku/sonnet/opus slots mapped to `light`/`standard`/`heavy`, `WebSearch` disallowed because Anthropic's server-side tools are not available through the gateway. Lists `auto`, the tiers and the `passthrough:` names from the local `models.yaml` that are not Claude subscription models.
- `convergence-claude-<account>` (extends `claude`): one per Claude subscription account, i.e. per `api_key_env` of passthrough entries with `auth: oauth`. `<account>` is the env name without a `CLAUDE_`/`ANTHROPIC_` prefix and `_TOKEN`/`_KEY` suffix (`CLAUDE_JEEFY_TOKEN` becomes `jeefy`). Lists only that account's models and maps the opus/sonnet/haiku slots to its newest model of each family, so background and subagent calls also use the subscription. The default model is the newest opus.
- `convergence-copilot` (extends `copilot`): Copilot CLI BYOK variables `COPILOT_PROVIDER_BASE_URL` (gateway `/v1`), `COPILOT_PROVIDER_API_KEY` and `COPILOT_MODEL=standard`, with the `convergence-claude` model list minus `auto` (the Copilot CLI reserves that name and rejects it under BYOK).
- `convergence-opencode` (extends `opencode`): lists the `convergence/*` models, so it also needs `convergence config opencode`.

`models.yaml` is read locally also with `--gateway-url`, so keep it matching the gateway. Run the command on the machine where the Paseo daemon runs, then `paseo reload`. A `config.json` it creates is mode `0600`; an existing one keeps its mode. `config claude-code` instead sets the gateway in `~/.claude/settings.json`, which routes every Claude Code run through it.
