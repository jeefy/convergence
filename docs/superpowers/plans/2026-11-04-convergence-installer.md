# Convergence Multi-Target Installer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build and release Convergence (`github.com/jeefy/convergence`), a unified distribution CLI binary and one-line installer for AgentGateway + Stand-In + Pinball supporting local (Docker/Podman) and cluster (Kubernetes) targets.

**Architecture:** A standalone Go CLI binary using `cobra` with embedded templates for docker-compose and Kubernetes manifests, accompanied by a multi-platform shell bootstrap script (`install.sh`) and GoReleaser config.

**Tech Stack:** Go 1.24+, Docker/Podman compose, AgentGateway v1.6.0, Stand-In v0.4.1, Pinball v0.6.1, GitHub Actions / GoReleaser.

---

### Task 1: Initialize Git Repository and Go Module

**Goal:** Establish `~/Code/jeefy/convergence` as a clean git repo and Go module with linting and test harnesses.

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `README.md`

**Acceptance Criteria:**
- [ ] Git repository initialized with `main` branch.
- [ ] `go.mod` declares `module github.com/jeefy/convergence`.
- [ ] `make test` runs without error.

**Verify:** `go test ./...` → passes.

---

### Task 2: Implement Bootstrap Installer Script (`install.sh`)

**Goal:** Create the standalone curl-pipeable installer script for Linux and macOS.

**Files:**
- Create: `install.sh`
- Create: `tests/test_install.sh`

**Acceptance Criteria:**
- [ ] Detects OS (`Darwin`, `Linux`) and architecture (`x86_64`, `arm64`, `aarch64`).
- [ ] Checks for container engine (`docker` or `podman`).
- [ ] Supports `CONVERGENCE_VERSION` and `INSTALL_DIR` overrides.
- [ ] Verified via shellcheck or shell test script.

**Verify:** `bash -n install.sh` and `tests/test_install.sh` exit 0.

---

### Task 3: CLI Framework & Configuration Architecture

**Goal:** Implement the Cobra root command, version info, config types, and path resolution for `~/.convergence/`.

**Files:**
- Create: `cmd/convergence/main.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/cli/root.go`
- Create: `internal/cli/version.go`

**Acceptance Criteria:**
- [ ] `convergence --version` outputs semver version and git commit.
- [ ] `config.Load()` and `config.Save()` manages YAML config in `~/.convergence/config.yaml`.
- [ ] Generates random 32-byte hex salt for Stand-In.

**Verify:** `go test ./internal/config/...` passes.

---

### Task 4: Template Embedding & Asset Rendering Engine

**Goal:** Embed and render runtime templates for `docker-compose.yaml`, `agentgateway.yaml`, and `pinball.yaml`.

**Files:**
- Create: `internal/templates/embed.go`
- Create: `internal/templates/assets/docker-compose.yaml.tmpl`
- Create: `internal/templates/assets/agentgateway.yaml.tmpl`
- Create: `internal/templates/assets/pinball.yaml.tmpl`
- Create: `internal/templates/render.go`
- Create: `internal/templates/render_test.go`

**Acceptance Criteria:**
- [ ] Go `embed.FS` embeds all runtime assets into the compiled binary.
- [ ] Renders valid AgentGateway v1.6 config with Stand-In ext_proc and Pinball ext_authz.
- [ ] Adjusts provider backend based on choice (Copilot, Direct API keys, or Ollama).

**Verify:** `go test ./internal/templates/...` passes.

---

### Task 5: Local Container Lifecycle Commands (`init`, `start`, `stop`, `status`)

**Goal:** Implement the local orchestration engine communicating with Docker / Podman CLI.

**Files:**
- Create: `internal/engine/engine.go`
- Create: `internal/engine/compose.go`
- Create: `internal/engine/compose_test.go`
- Create: `internal/cli/init.go`
- Create: `internal/cli/start.go`
- Create: `internal/cli/stop.go`
- Create: `internal/cli/status.go`

**Acceptance Criteria:**
- [ ] `convergence init` guides provider selection and writes assets to `~/.convergence/`.
- [ ] `convergence start` launches containers in background and waits for health.
- [ ] `convergence status` verifies connectivity to `:3000` and health of components.
- [ ] `convergence stop` cleanly tears down compose stack.

**Verify:** `go test ./internal/engine/...` passes.

---

### Task 6: Usage Telemetry & Stats Command (`convergence stats`)

**Goal:** Parse AgentGateway JSONL access logs and Prometheus metrics to print terminal analytics.

**Files:**
- Create: `internal/telemetry/stats.go`
- Create: `internal/telemetry/stats_test.go`
- Create: `internal/cli/stats.go`

**Acceptance Criteria:**
- [ ] Computes total requests, total tokens (input, cache_read, output), and estimated USD cost.
- [ ] Calculates Pinball routing tier breakdown (light vs standard vs deep).
- [ ] Formats output cleanly with terminal tables or summary panels.

**Verify:** `go test ./internal/telemetry/...` passes.

---

### Task 7: Cluster Target Engine (`convergence cluster install|status|uninstall`)

**Goal:** Embed Kubernetes manifests and implement cluster deployment management.

**Files:**
- Create: `internal/k8s/embed.go`
- Create: `internal/k8s/assets/` (k8s manifests for agentgateway, stand-in, pinball)
- Create: `internal/k8s/installer.go`
- Create: `internal/k8s/installer_test.go`
- Create: `internal/cli/cluster.go`

**Acceptance Criteria:**
- [ ] `convergence cluster install` renders and applies manifests to target namespace.
- [ ] Supports `--dry-run` flag outputting all rendered manifests to stdout.
- [ ] Generates required Stand-In salt Secret and Copilot / provider Secret.
- [ ] `convergence cluster status` checks pod readiness and service endpoints.

**Verify:** `go test ./internal/k8s/...` passes.

---

### Task 8: Release Automation & End-to-End Verification

**Goal:** Add GoReleaser configuration, GitHub Actions workflow, and comprehensive test suite.

**Files:**
- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yaml`
- Create: `.github/workflows/ci.yaml`
- Modify: `README.md` (installation instructions, quickstart guide)

**Acceptance Criteria:**
- [ ] `goreleaser check` succeeds.
- [ ] Full `make build` produces functional standalone binary.
- [ ] README documents one-liner install, local workflow, and cluster workflow.

**Verify:** `make build && ./bin/convergence --help` prints usage with all subcommands.
