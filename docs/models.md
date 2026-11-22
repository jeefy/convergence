# Providers and model routing

## Providers

| Provider | Upstream | Credentials |
|---|---|---|
| `copilot` | GitHub Copilot through [copilot-api](https://github.com/ericc-ch/copilot-api) (`v0.7.0`) on `:4141` | A GitHub account with an active Copilot subscription. `convergence login copilot` (device flow) or `--copilot-token`. |
| `direct` | OpenAI and Anthropic APIs (Gemini isn't supported) | `--openai-key` from <https://platform.openai.com/api-keys>, `--anthropic-key` from <https://console.anthropic.com/settings/keys>. Either or both. |
| `ollama` | A local Ollama server | No key. `--ollama-url` defaults to `http://host.docker.internal:11434`. Local mode only. |

Cluster mode supports `copilot` and `direct`.

Copilot is reached through its chat completions API. For requests to Copilot models the gateway sends system text as plain-text `system` messages: Copilot ignored, for Claude models, system text sent as a list of content parts (how Anthropic `system` blocks are converted) and the `developer` role (how Responses API `instructions` are converted). Several system blocks are joined with a blank line.

## Model routing

Pinball scores each request into a tier (`light`, `standard`, `heavy`). Convergence maps each tier to one pinball model, and each pinball model to an upstream model for your provider. `convergence models` shows the active mapping:

```
Provider: direct
TIER      PINBALL MODEL      UPSTREAM
light     claude-haiku-5.5   anthropic/claude-haiku-5-5
standard  claude-sonnet-5.5  anthropic/claude-sonnet-5-5
heavy     claude-opus-5.5    anthropic/claude-opus-5-5
```

Defaults per provider:

| Tier | Pinball model | `copilot` | `direct` | `ollama` |
|---|---|---|---|---|
| light | `claude-haiku-5.5` | `claude-haiku-5.5` | `anthropic/claude-haiku-5-5` | `qwen2.5-coder:7b` |
| standard | `claude-sonnet-5.5` | `claude-sonnet-5.5` | `anthropic/claude-sonnet-5-5` | `qwen2.5-coder:14b` |
| heavy | `claude-opus-5.5` | `claude-opus-5.5` | `anthropic/claude-opus-5-5` | `qwen2.5-coder:32b` |

Providers rename and retire models, so these are defaults, not guarantees.

### How a prompt becomes a model

Clients opt in by sending `model: auto` (or `pinball`). Scoring is Pinball's job; the rules live in the [Pinball README](https://github.com/jeefy/pinball#choosing-a-tier). In short: the score is the best-scoring user message plus request signals (code fences, heavy keywords such as `refactor` or `root cause`, length, `tools` present, reasoning effort, small `max_tokens`). Convergence renders thresholds `standard: 1` and `heavy: 3`: below 1 is `light`, 3 or more is `heavy`, anything between is `standard`. If Pinball doesn't answer, the request goes to the standard-tier model.

The gateway forwards `x-pinball-tier` (force a tier) and `x-pinball-min-tier` (raise to at least that tier) request headers to Pinball. Responses carry the pick in `x-pinball-model`, `x-pinball-tier` and `x-pinball-reason`; the body's `model` is the real upstream model.

Worked examples with the point values from the Pinball README (they may change between Pinball versions; `convergence doctor` shows the real decision for its probe prompts):

| Prompt (sent with `model: auto`) | Signals | Score | Tier |
|---|---|---|---|
| "Generate a title", `max_tokens: 20` | light keyword -1, short -1, small output -1 | -3 | light |
| A 300 character coding question with `tools` set | tools +1 | 1 | standard |
| A pasted traceback and "find the root cause of this deadlock and refactor it" | code +1, two heavy keywords +2 | 3 | heavy |

### Model discovery (opt-in)

`init --discover-models` (provider `copilot` or `ollama`) and `cluster install --discover-models` (provider `copilot`) let Pinball build its pool from the upstream `/v1/models` instead of the rendered `pinball.yaml` pool: copilot-api for Copilot, `<ollama-url>/v1/models` for Ollama. Pinball infers each model's tier from its capabilities and refreshes every 6 hours (`PINBALL_DISCOVERY_INTERVAL_SECONDS`). If discovery fails it serves the last cached pool, else the rendered pool. Provider `direct` is refused: it has no single model list.

With discovery on, the gateway also:

- routes `auto` to the model Pinball picked, falling back to the standard-tier model;
- serves any other model id (`*`) to the upstream unchanged, so `/v1/models` lists the upstream's models once;
- serves `opt-*` with one wildcard route when the optimizer is on, instead of an `opt-<model>` entry per pool model (Ollama keeps those, because pool names differ from upstream ids).

`light`, `standard` and `heavy` still point at the rendered pool. `--discover-models=false` turns discovery off. The cache is `/tmp/pinball-discovery.json` in the local container (lost on recreate) and an emptyDir in Kubernetes.

### Overrides: `models.yaml`

`~/.convergence/models.yaml` overrides the defaults (`convergence models init` writes a commented starter and never overwrites an existing file). Run `convergence restart` after editing. In cluster mode pass the same file with `cluster install --models-file models.yaml`.

```yaml
# provider -> pinball model -> upstream model
direct:
  claude-haiku-5.5: openai:gpt-5-mini   # direct accepts anthropic: (default) or openai:
ollama:
  claude-opus-5.5: qwen2.5-coder:14b
```

Only `copilot`, `direct` and `ollama` are valid provider keys, and only the three pinball model names above are valid; anything else is an error. For Ollama you can instead set tiers at init with `--ollama-model-light`, `--ollama-model-standard` and `--ollama-model-heavy`; `models.yaml` wins over those flags.

**Pinball thresholds and scoring.** An optional top-level `pinball:` block overrides Pinball's score cutoffs and per-signal points (unset keys keep Pinball's defaults; unknown keys are an error). Example: Pinball adds `+1` for requests with tools, which with `thresholds.standard: 1` puts every tool-using agent request in `standard`. To let short agent turns reach `light`:

```yaml
pinball:
  thresholds: {standard: 2, heavy: 4}
  scoring: {tools: 0}   # keys: code, heavy_keyword, max_heavy_keywords, light_keyword, long_chars, long, very_long_chars, very_long, short_chars, short, tools, effort_high, effort_low, small_output_tokens, small_output, structured, large_context_tokens, large_context, very_large_context_tokens, very_large_context
```

`large_context_tokens` / `very_large_context_tokens` add `large_context` / `very_large_context` points once the estimated prompt reaches that size; unset, the signal is off. `very_large_context_tokens` needs a smaller `large_context_tokens`.

`heavy_keywords` and `light_keywords` in the same block replace the keyword lists rendered into Pinball's config (by default Pinball's own lists, see `~/.convergence/pinball.yaml`). Keywords match whole words, case-insensitive, in user text. To add words, for example another language, copy the defaults and extend them; `[]` turns that signal off:

```yaml
pinball:
  heavy_keywords: [architecture, refactor, "root cause", debug, architektur, ursache]
  light_keywords: [title, summarize, translate, titel, zusammenfassung]
```

`wrapped_user_messages` lists user-role messages that are mostly client scaffolding (an injected skill file, say). Pinball scores only the text after the last `instruction_markers` hit, cut at `instruction_end`; with no marker the message is not scored, and it still counts toward prompt size. `prefix` is required. Pinball `0.8.0` accepts this key. Unset, nothing is rendered.

`sticky: {sessions, ttl_seconds}` keeps a conversation on its tier (as a floor) and model while it still fits, so a config reload cannot move it mid-session and drop the prompt cache. `sessions` (conversations remembered, LRU) is required and greater than 0, `ttl_seconds` defaults to 3600. State is per Pinball process: lost on restart, not shared between replicas. **Needs a Pinball image newer than the pinned `0.8.0`** (`cluster install --image-pinball`, or `image_pinball` in `config.yaml` locally): `0.8.0` rejects the `sticky` key and does not start. Unset, nothing is rendered.

```yaml
pinball:
  sticky: {sessions: 500, ttl_seconds: 3600}
  wrapped_user_messages:
    - {prefix: "<skill>", instruction_markers: ["Task:"], instruction_end: "</skill>"}
```

`normalize_reasoning_effort: true` makes the gateway send Pinball's per-model `reasoning_effort` upstream. Pinball (after 0.8.0) answers a normalized word for the chosen model in its `/check` reply (for example Copilot rejects `none` and `minimal`, and haiku-4.5 rejects any value); the gateway copies it into the ext_authz metadata and every `pinball-*` model sets `reasoning_effort` from it, next to the existing Copilot `messages` transformation. When Pinball sends none, the expression fails and agentgateway removes the field. **Needs a Pinball image newer than the pinned `0.8.0` (`cluster install --image-pinball`, or `image_pinball` in `config.yaml` locally):** `0.8.0` answers no `reasoning_effort`, so with the key on, the clients' own effort would be dropped. Off by default; nothing is rendered then.

```yaml
pinball:
  normalize_reasoning_effort: true
```

**Local light tier.** Short prompts (titles, one-line questions) land in `light`. You can keep the heavier tiers on your cloud provider and serve `light` from a local Ollama model with the `ollama:` prefix, accepted for `copilot` and `direct`:

```yaml
copilot:
  claude-haiku-5.5: ollama:gemma4:26b
```

Set `ollama_url` in `~/.convergence/config.yaml` if it isn't the default, `ollama pull gemma4:26b`, then `convergence restart`. A 26b-class model has been a good fit for titles and short answers; 7b-class models are faster but noticeably weaker. Local latency depends on your hardware (the first request after idle loads the model) and quality is below the hosted light model, so keep `standard` and `heavy` on the cloud. `convergence doctor` warns when a model mapped to Ollama is not installed. Check routing with a short `model: auto` request and look at the `x-pinball-tier` response header.

With the Ollama provider, `convergence start` warns (with the `ollama pull` commands) when a mapped model isn't installed or Ollama can't be reached; `start --strict` makes that an error.

**Passthrough models.** To call a fixed model by name with no Pinball routing, add a `passthrough:` block. The client uses the key as the model name; Stand-In still scrubs the request and the `x-pinball-*` response headers stay empty.

```yaml
passthrough:
  copilot-fable-5: {provider: copilot, model: claude-fable-5}
  glm-5.2:         {provider: ollama,  model: glm-5.2}
```

`provider` is `copilot`, `openai`, `anthropic` or `ollama`. Names must not be `auto`, `pinball`, `light`, `standard`, `heavy` or start with `pinball-`. On a cluster install, `ollama` entries need a `base_url` and `copilot` entries need `--provider copilot`.

An entry may set `base_url` and `api_key_env` (the **name** of an env var, never the key) to use another endpoint such as Ollama Cloud. Entries with the same `(provider, base_url, api_key_env)` share one gateway provider; entries without them use the shared provider (local Ollama, `--ollama-url`).

```yaml
passthrough:
  glm-5.2: {provider: ollama, model: glm-5.2, base_url: 'https://ollama.com', api_key_env: OLLAMA_CLOUD_KEY}
  gemma4:  {provider: ollama, model: gemma4}   # local Ollama
```

Local: export `OLLAMA_CLOUD_KEY` when running `init`, `restart` or `upgrade`; it is written to the 0600 `.env` (an existing value is kept if the variable is unset) and the rendered config only contains `${OLLAMA_CLOUD_KEY}`. Cluster: create the Secret yourself, e.g. `kubectl -n convergence create secret generic convergence-passthrough --from-literal=OLLAMA_CLOUD_KEY=...` (name via `--passthrough-secret`); the gateway reads each key from its environment. Keys never appear in rendered config or manifests. `STANDIN_SALT`, `GH_TOKEN`, `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` are not allowed as `api_key_env`.

**Two Claude Code accounts.** An `anthropic` entry with `auth: oauth` sends its key as `Authorization: Bearer` with `anthropic-beta: oauth-2025-04-20` (a Claude subscription token from `claude setup-token`) instead of `x-api-key`. The beta flag is appended to the client's own `anthropic-beta` value (Claude Code sends several), not substituted for it. The client's Convergence key is replaced by the account token and never reaches Anthropic. Give each account its own `api_key_env`; the client picks the account by model name.

```yaml
passthrough:
  claude-lf-sonnet:    {provider: anthropic, model: claude-sonnet-4-5, api_key_env: CLAUDE_LF_TOKEN,    auth: oauth}
  claude-jeefy-sonnet: {provider: anthropic, model: claude-sonnet-4-5, api_key_env: CLAUDE_JEEFY_TOKEN, auth: oauth}
```

`auth: oauth` requires `provider: anthropic` and `api_key_env`. If you set `base_url`, include `/v1` (agentgateway appends `/messages`). Passthrough names may contain only letters, digits and `. _ : / -`. Omit `auth` to use a plain Anthropic API key from `api_key_env` (`x-api-key`). Run Claude Code with `ANTHROPIC_MODEL=claude-lf-sonnet`. Whether Anthropic permits subscription tokens through a gateway is governed by their terms; check them first.
