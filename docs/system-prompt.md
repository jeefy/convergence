# System prompt

Optional, off by default. When enabled, agentgateway prepends a fixed system prompt to every chat request before it reaches the upstream: eleven short behavioral guardrails (verify before claiming done, no scope creep, no silent test deletion, ...) plus any text you add. The exact lines are in `internal/prompts/prompts.go`; `convergence init --help` shows the flags.

```bash
convergence init --system-prompt                                   # defaults only
convergence init --system-prompt --system-prompt-file ~/prompt.txt # defaults + your lines
convergence cluster install --provider copilot --system-prompt
```

Or in `~/.convergence/config.yaml`:

```yaml
system_prompt:
  enabled: true
  defaults: true      # false = only `extra`
  extra: |
    Prefer small commits.
```

It applies to `/v1/chat/completions` (`messages[0]`, role `system`; a system message from the client stays after it), `/v1/responses` (prepended to `instructions`) and `/v1/messages` (first block of `system`). agentgateway passes other paths through unchanged, so a request sent without the `/v1` prefix does not get the prompt. `/v1/messages/count_tokens` and `/v1/responses/compact` get it as well, so token counts match the real request. Embeddings and other non-chat endpoints are untouched.

The text is added when the request is handed to the model, after Stand-In has scrubbed the request and Pinball has picked a tier: it is not scrubbed (keep secrets out of it) and clients do not see it. It counts as input tokens on every request. It must be valid UTF-8 and must not contain `$` (agentgateway expands `$VAR` in its config) or control characters. `--system-prompt-file` needs `--system-prompt` or `system_prompt.enabled` in `config.yaml`; the setting is kept when `init` is re-run without the flags. With the request log enabled, logged prompts include this text; the optimizer's replays (`opt-` models) are built from those logs and are not prepended again. The optimizer's judge requests (`x-agw-caller: eval-replay-judge`) are exempt.

`cluster install` does not remember the setting: pass the flags on every install. To turn it off locally, run `convergence init --system-prompt=false` or set `system_prompt.enabled: false` in `config.yaml`, then `convergence restart`. The prompt shapes model behavior; it is not an access control, and a client can avoid it.
