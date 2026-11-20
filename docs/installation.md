# Installation

## Prerequisites

- Linux or macOS, amd64 or arm64. Windows: native binary for client configuration and remote gateways, WSL2 for the local stack (see [Windows](#windows)).
- Local stack: Docker with the compose plugin, or Podman with `podman-compose` or `podman compose`.
- Cluster mode: `kubectl` access to a cluster.
- Credentials for one provider (see [Providers](models.md#providers)).

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
```

The installer downloads `convergence_<os>_<arch>.tar.gz` and `checksums.txt` from the GitHub release and refuses to install if the sha256 doesn't match. It installs to `/usr/local/bin` if writable, otherwise `~/.local/bin`, never runs `sudo`, and prints guidance if docker/podman with compose is missing.

```bash
CONVERGENCE_VERSION=v0.1.0 bash install.sh   # pin a release (default: latest)
INSTALL_DIR=$HOME/bin bash install.sh        # choose the target directory
```

Windows (PowerShell, no elevation):

```powershell
irm https://raw.githubusercontent.com/jeefy/convergence/main/install.ps1 | iex
```

It downloads `convergence_windows_<arch>.zip`, verifies the sha256 against `checksums.txt`, installs to `%LOCALAPPDATA%\Programs\convergence` (or `$env:INSTALL_DIR`) and adds that directory to the user `PATH`. `$env:CONVERGENCE_VERSION` pins a release. See [Windows](#windows) for what runs natively.

From source (Go 1.24+):

```bash
git clone https://github.com/jeefy/convergence.git && cd convergence
make build            # produces bin/convergence
```


## Windows

Releases include `convergence_windows_amd64.zip` and `convergence_windows_arm64.zip` (listed in `checksums.txt`). `install.ps1` (see [Install](#install)) installs and verifies them; or unzip `convergence.exe` somewhere on your `PATH` yourself. `install.sh` refuses to run outside WSL2. The binary is not code-signed, so SmartScreen may warn on first run.

What works natively: the client config writers (`convergence config <client>`, which edit the files of clients that run on Windows and resolve `%USERPROFILE%` and `%LOCALAPPDATA%`), `models`, `version`, and `status`, `doctor`, `stats` and the `cluster` commands against a gateway or cluster reachable from Windows. `doctor` skips the `.env` mode check on NTFS. CI runs those packages' unit tests on `windows-latest`.

The local compose stack is supported through WSL2 only: install Convergence inside a WSL2 distro like on Linux, with Docker or Podman reachable from that distro. `convergence config <client>` there configures clients that run inside the distro; for a client on the Windows side, run the native binary or point it at `http://localhost:3000/v1` with API key `convergence` (`--dry-run` shows the values). Docker Desktop with the WSL2 backend from a native `convergence.exe` is untested. `--sandbox nono` (Landlock and Seatbelt) has no Windows equivalent.

## Versioning and upgrades

Convergence follows [Semantic Versioning](https://semver.org/). Before 1.0, minor releases may include breaking changes; they are called out in [CHANGELOG.md](../CHANGELOG.md). Each release pins agentgateway, Stand-In, Pinball and copilot-api by tag and sha256 digest (see [docs/architecture.md](architecture.md#pinned-images)). To upgrade, install the new binary (re-run the installer), then run `convergence upgrade`.
