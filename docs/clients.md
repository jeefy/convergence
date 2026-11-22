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

[Paseo](https://paseo.sh) launches other agents, so `config paseo` adds two providers under `agents.providers` and leaves the built-in ones alone: `convergence-claude` (extends `claude`: gateway base URL, Claude Code's haiku/sonnet/opus slots mapped to `light`/`standard`/`heavy`, `WebSearch` disallowed because Anthropic's server-side tools are not available through the gateway) and `convergence-opencode` (extends `opencode`, lists the `convergence/*` models, so it also needs `convergence config opencode`). The Claude provider offers `auto`, the three tiers and, the `passthrough:` names from the local `models.yaml` (also with `--gateway-url`, so keep it matching the gateway). Run the command on the machine where the Paseo daemon runs, then `paseo reload`. A `config.json` it creates is mode `0600`; an existing one keeps its mode.
