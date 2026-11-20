# Insights, request log and optimizer

## Insights

`convergence insights` reads agentgateway's Postgres request log over a window (`--since 1d|7d|30d`, default `7d`) and reports:

- request counts, input and output tokens, average latency, per-model breakdown and tier distribution;
- estimated cost in total and per tier, and the estimated saving versus pricing the same tokens on one reference model (`--reference-model`, default the heavy-tier model). Prices come from Pinball's optimizer catalog, not from invoices; rows without a price are excluded and counted ("N of M rows priced");
- patterns worth a look: large prompts with tiny completions, heavy-tier picks for simple requests, rapid duplicate retries, high error rates;
- with `--coach`, recommendations generated from those patterns through the gateway itself (`POST /v1/chat/completions`, model `auto`).

It needs the request log enabled (`init --request-log-db`, or `cluster install --request-log-postgres` / `--request-log-db`), or a DSN via `--db-dsn`. `--json` prints the data.

## Request log and Pinball optimizer

The request log and the optimizer are off by default and experimental.

**Callers.** With the request log enabled, the `x-agw-caller` request header is stored as the request's user (`request_logs.agentgateway_user`). The optimizer reports per caller and skips its own replays (`eval-*`) by that value; requests without the header are stored with no user.

**Privacy.** Enabling the request log sets agentgateway's LLM logging to `full`: the database stores prompts and completions, after Stand-In scrubbing, which is pattern-based and not a guarantee. Protect and expire that database accordingly. The optimizer replays sampled requests against candidate models, so it makes real model calls.

**Local.** The compose stack ships no Postgres. Point agentgateway at one you run, reachable from the containers: `convergence init --request-log-db postgres://user:pass@host.docker.internal:5432/agentgateway && convergence restart`. The DSN goes to the 0600 `.env` and `config.yaml`, never into the rendered gateway config. `convergence optimize` then uses it without `--db-dsn`.

**Cluster.** `cluster install --request-log-postgres` renders a small Postgres (StatefulSet, 5Gi PVC, Service, NetworkPolicy allowing only agentgateway and the optimizer) with a generated password; `--request-log-db <dsn>` uses your own database instead. Either one also renders the optimizer CronJob `pinball-optimize`, weekly by default (`0 3 * * 0`; change with `--opt-schedule`), using the install's pinball image. `--optimizer-dsn <dsn>` renders only the CronJob against an existing request-log database, without turning logging on. The DSN lives only in Secret `pinball-optimizer-db`; agentgateway and the CronJob read it from the environment. Re-running `cluster install` reuses the Postgres password, Stand-In salt and Copilot token from the previous install's Secrets when the flags are omitted (pass them to rotate), so the existing PVC keeps working.

**Retention.** The log grows without bound by default. `cluster install --request-log-retention 720h` renders a daily CronJob `request-log-retention` (02:17) that deletes rows older than that duration in batches of 5000 (payloads follow via `ON DELETE CASCADE`). Minimum `24h`, the optimizer's sampling window; shorter values fail at install. Dropping the flag on a re-install leaves an existing CronJob in place until you `kubectl delete cronjob request-log-retention`. Deleted rows are gone: back up first.

### Backup and restore

`convergence cluster backup --out ./bk` writes `request_logs-<ts>.dump` (`pg_dump -Fc`) and `pinball-overlay-<ts>.tar` (a missing overlay only warns). `convergence cluster restore --from ./bk/request_logs-<ts>.dump` runs `pg_restore --clean --if-exists` into the running Postgres after confirmation (`--yes` skips it). `cluster install` backs up an existing `request-log-postgres` into `~/.convergence/backups/` first (keeps the last 5; `--no-backup` skips). Dumps contain prompts: protect them like the database.

`cluster uninstall` deletes the namespace and, with most storage classes (reclaim policy `Delete`), the PVs. Back up first, or retain the volumes: `kubectl get pv`, then `kubectl patch pv <name> -p '{"spec":{"persistentVolumeReclaimPolicy":"Retain"}}'` for the `request-log-postgres` and `pinball-overlay` claims. Scheduled or off-cluster backups are not included.
