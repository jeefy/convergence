# Security model

**What Stand-In does.** It inspects request bodies and replaces detected secrets (API keys, tokens, credentials) with `<<secret-...>>` placeholders, then restores them in responses. Detection is pattern-based; the rules are described in the [Stand-In README](https://github.com/jeefy/stand-in). It won't catch every secret, or any PII it has no rule for, and it doesn't cover anything you send through other paths. Treat it as a safety net, not a guarantee.

**Custom patterns.** `init --standin-patterns-file ./patterns.yaml` (saved as `standin_patterns_file`) or `cluster install --standin-patterns-file ./patterns.yaml`. Compose mounts the file read-only and sets `STANDIN_PATTERNS_FILE`; the cluster install renders ConfigMap `stand-in-patterns` and mounts it. The file **replaces** Stand-In's packaged `patterns.yaml` (Stand-In loads exactly one file), so start from a copy of the upstream file and append your rules.

**Failure modes.** Stand-In is fail-closed by default (`failureMode: failClosed`): if it is down, requests are rejected and nothing leaves unscrubbed. `init --fail-open` or `cluster install --fail-open` opts into passthrough, where requests go upstream **unscrubbed** while Stand-In is down; `status` and `doctor` warn about it. Pinball uses `failureMode: allow` on purpose: it only picks a model, so when it is down the request goes to the standard-tier model and confidentiality is unaffected.

**Exposure.** All published ports bind to `127.0.0.1` by default (`bind_address` in `config.yaml`). The gateway has no authentication of its own: any local process that reaches `127.0.0.1:3000` can use your provider keys through it, and in Kubernetes any pod the NetworkPolicy allows can do the same. Change `bind_address` only if you mean to expose it.

**Secrets at rest.** Locally, provider keys and tokens are stored in plaintext in `~/.convergence/config.yaml` and `~/.convergence/.env`, both mode `0600`. The rendered compose and gateway files contain no secrets; compose reads them from `.env`. In the cluster, keys live in Kubernetes Secrets. Containers use `json-file` log rotation (10 MB, 3 files).

Report vulnerabilities privately: see [SECURITY.md](../SECURITY.md).
