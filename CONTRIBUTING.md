# Contributing

Bug reports, fixes and new client integrations are welcome. By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Dev setup

You need Go (version in `go.mod`), make, and Docker or Podman with compose for the end-to-end tests. Cluster e2e also needs `kubectl` and a disposable cluster (kind works).

```bash
make build                     # bin/convergence
make test                      # go test ./...
make lint                      # golangci-lint, go vet, gofmt check; must be 0 issues
make fmt                       # format the tree
go test -race -count=1 ./...   # what CI runs
make test-e2e                  # local compose stack, tests/test_e2e_local.sh
make test-e2e-cluster          # Kubernetes, tests/test_e2e_cluster.sh
make test-e2e-copilot          # compose stack with the copilot-api sidecar
bash tests/test_install.sh     # installer tests, offline
pwsh -File tests/test_install.ps1  # Windows installer tests, offline (pwsh on any OS)
bash tests/test_validate_configs.sh   # rendered configs against the real images
```

What the e2e tests do:

- `tests/test_e2e_local.sh` brings up the real compose stack with the Ollama provider pointed at a stub upstream (`tests/stub-upstream/`), then sends real chat requests (plain and streaming) through the gateway. CI runs it on Docker, Podman with docker-compose and Podman with podman-compose.
- `tests/test_e2e_cluster.sh` installs into a disposable cluster (KinD in CI), sends a real chat through the gateway, then checks fail-closed behavior with Stand-In down.
- `cluster install` has a hidden `--openai-base-url` flag (provider `direct`) that points the OpenAI backend at a stub. It is for tests only and not a supported option.

## Layout

```
cmd/convergence/        main package
internal/cli/           cobra commands
internal/config/        ~/.convergence/config.yaml model and defaults
internal/models/        tier to upstream model mapping and models.yaml overrides
internal/templates/     compose, agentgateway and pinball templates (local mode)
internal/engine/        docker/podman compose invocation, Ollama probes
internal/k8s/           cluster manifests and installer
internal/clientconfig/  per-client config writers with backup/restore
internal/sandbox/       sandbox runtimes behind `sandbox run`
internal/skillscan/     SkillSpector image build and scan behind `skills`
internal/doctor/        checks behind `doctor`
internal/health/        service probes behind `status`
internal/telemetry/     metrics scraping for `stats`
internal/insights/      request-log analysis and cost estimates for `insights`
internal/version/       build metadata (set via ldflags)
tests/                  shell e2e and installer tests
docs/                   user and architecture documentation
```

## Adding a client config

1. In `internal/clientconfig/`, add a path helper for the client's config file and a configure function. Check the client's documentation for the exact keys. Write a `<file>.convergence.bak` backup before changing anything, keep the user's comments and other keys, and reuse the shared restore helper.
2. Add tests in `clientconfig_test.go` that run against a temp `HOME`: configure, restore, and restore when no backup exists.
3. Register the subcommand in `internal/cli/clientconfig.go` so it gets `--dry-run`, `--gateway-url` and `restore [--force]`, and shows up in `config list`.
4. Add a row to the client table in `docs/clients.md` and an entry under `[Unreleased]` in `CHANGELOG.md`.

## Bumping pinned image versions

Image tags appear in several places. Change all of them in one PR:

```bash
grep -rn 'agentgateway:v\|stand-in:\|pinball:\|copilot-api:v\|skillspector' --include='*.go' --include='*.tmpl' --include='*.sh' --include='*.md' .
```

That covers `internal/templates/render.go`, `internal/k8s/assets/manifests.yaml.tmpl`, `internal/cli/optimize.go`, `tests/`, `docs/models.md`, `docs/installation.md` and `docs/architecture.md`. SkillSpector is built from source, so its pin is a commit SHA plus tag (`SourceCommit` and `DefaultImage` in `internal/skillscan/skillscan.go`, mirrored in `tests/test_e2e_local.sh`, `docs/skills.md` and `docs/architecture.md`). Read the upstream changelog (agentgateway config schema changes are the usual breakage), run `make test-e2e` and `make test-e2e-cluster`, and note the bump in `CHANGELOG.md`.

## Pull requests

- Keep changes focused; one topic per PR.
- `make test` and `make lint` must pass.
- Anything that drives a container engine or the stack (new subcommand, flag, config key) needs coverage in `tests/test_e2e_local.sh` or `tests/test_e2e_cluster.sh`. Unit tests with a fake engine are not a substitute.
- Update the relevant page under `docs/` (and `README.md` only if the doc index changes) and `[Unreleased]` in `CHANGELOG.md` when behavior changes.
- No secrets or real tokens in tests or fixtures.

## Releases

Maintainers tag `vX.Y.Z` on `main`. The release workflow runs the tests and GoReleaser, which publishes archives, `checksums.txt` and SBOMs. Check the config locally with:

```bash
go run github.com/goreleaser/goreleaser/v2@v2.6.1 check
go run github.com/goreleaser/goreleaser/v2@v2.6.1 release --snapshot --clean --skip=publish,sbom
```

**Winget.** `.goreleaser.yaml` has a `winget:` publisher for `jeefy.convergence`. It opens a PR against `microsoft/winget-pkgs` only when the release workflow has a `WINGET_GITHUB_TOKEN` secret: a fine-grained PAT with contents and pull-request write access to a fork of `winget-pkgs` under `jeefy`. Without it the manifest is only written to `dist/`. The first submission to winget-pkgs is reviewed by hand by Microsoft; later versions are mostly automatic.
