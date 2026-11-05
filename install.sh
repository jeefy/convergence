#!/usr/bin/env bash
# install.sh - Convergence installer
# Usage: curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash
#
# Environment:
#   CONVERGENCE_VERSION  release tag to install (e.g. v0.1.0) or "latest" (default)
#   INSTALL_DIR          target directory (default: /usr/local/bin if writable, else ~/.local/bin)
#
# This script never calls sudo. If you want a system-wide install, run it with
# INSTALL_DIR pointing at a directory you can write to, or move the binary yourself.

set -euo pipefail

REPO="jeefy/convergence"
CONVERGENCE_VERSION="${CONVERGENCE_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-}"
DEFAULT_SYSTEM_DIR="/usr/local/bin"
BIN_NAME="convergence"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# detect_os_arch [os] [arch]: sets OS and ARCH. Args override uname (used by tests).
detect_os_arch() {
  local os arch
  os="$(echo "${1:-$(uname -s)}" | tr '[:upper:]' '[:lower:]')"
  arch="${2:-$(uname -m)}"

  case "$os" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    mingw* | msys* | cygwin* | windows*)
      log_error "Windows is not supported. Use WSL2 (Linux) and run this installer inside it."
      return 1
      ;;
    *)
      log_error "Unsupported operating system: ${os}. Supported: linux, darwin."
      return 1
      ;;
  esac

  case "$arch" in
    x86_64 | amd64) ARCH="amd64" ;;
    arm64 | aarch64) ARCH="arm64" ;;
    *)
      log_error "Unsupported architecture: ${arch}. Supported: amd64, arm64."
      return 1
      ;;
  esac

  log_info "Detected platform: ${OS}/${ARCH}"
}

# release_base_url: prints the download base for the selected version.
release_base_url() {
  if [ "$CONVERGENCE_VERSION" = "latest" ]; then
    echo "https://github.com/${REPO}/releases/latest/download"
  else
    local tag="$CONVERGENCE_VERSION"
    case "$tag" in v*) ;; *) tag="v${tag}" ;; esac
    echo "https://github.com/${REPO}/releases/download/${tag}"
  fi
}

archive_name() {
  echo "${BIN_NAME}_${OS}_${ARCH}.tar.gz"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    log_error "Neither sha256sum nor shasum is available; cannot verify the download."
    return 1
  fi
}

# verify_checksum <file> <checksums.txt> <asset name>
verify_checksum() {
  local file="$1" sums="$2" name="$3" expected actual
  expected="$(awk -v n="$name" '$2 == n || $2 == "*"n {print $1}' "$sums" | head -n1)"
  if [ -z "$expected" ]; then
    log_error "No checksum for ${name} in checksums.txt. Refusing to install."
    return 1
  fi
  actual="$(sha256_of "$file")" || return 1
  if [ "$expected" != "$actual" ]; then
    log_error "Checksum mismatch for ${name}: expected ${expected}, got ${actual}. Refusing to install."
    return 1
  fi
  log_success "Checksum verified (sha256 ${actual})"
}

check_prerequisites() {
  local found=""
  if command -v docker >/dev/null 2>&1; then
    if docker compose version >/dev/null 2>&1; then
      log_success "Found docker with compose plugin"
      found=1
    else
      log_warn "docker found but 'docker compose' is not available. Install the Docker Compose plugin."
    fi
  fi
  if command -v podman >/dev/null 2>&1; then
    if podman compose version >/dev/null 2>&1 || command -v podman-compose >/dev/null 2>&1; then
      log_success "Found podman with compose support"
      found=1
    else
      log_warn "podman found but no compose provider. Install podman-compose or docker-compose."
    fi
  fi
  if [ -z "$found" ]; then
    log_warn "No container engine with compose found. 'convergence start' needs one of:"
    log_warn "  Docker Engine/Desktop with the compose plugin: https://docs.docker.com/engine/install/"
    log_warn "  Podman with podman-compose: https://podman.io/docs/installation"
    log_warn "Kubernetes-only use ('convergence cluster ...') needs kubectl access instead."
  fi
}

resolve_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    TARGET_DIR="$INSTALL_DIR"
    if [ ! -d "$TARGET_DIR" ]; then
      log_info "Creating ${TARGET_DIR}"
      mkdir -p "$TARGET_DIR"
    fi
    if [ ! -w "$TARGET_DIR" ]; then
      log_error "${TARGET_DIR} is not writable. This installer does not use sudo."
      log_error "Re-run with a writable INSTALL_DIR, or run: sudo env INSTALL_DIR=${TARGET_DIR} bash install.sh"
      return 1
    fi
  elif [ -w "$DEFAULT_SYSTEM_DIR" ]; then
    TARGET_DIR="$DEFAULT_SYSTEM_DIR"
  else
    TARGET_DIR="${HOME}/.local/bin"
    log_info "${DEFAULT_SYSTEM_DIR} is not writable; installing to ${TARGET_DIR} (no sudo used)."
    mkdir -p "$TARGET_DIR"
  fi
}

install_binary() {
  local base asset tmp_dir
  base="$(release_base_url)"
  asset="$(archive_name)"
  tmp_dir="$(mktemp -d)"
  # shellcheck disable=SC2064 # expand tmp_dir now
  trap "rm -rf '${tmp_dir}'" EXIT

  log_info "Downloading ${base}/${asset}"
  if ! curl -fsSL "${base}/${asset}" -o "${tmp_dir}/${asset}"; then
    log_error "Download failed. Check that release ${CONVERGENCE_VERSION} exists for ${OS}/${ARCH}."
    log_error "Releases: https://github.com/${REPO}/releases"
    return 1
  fi
  if ! curl -fsSL "${base}/checksums.txt" -o "${tmp_dir}/checksums.txt"; then
    log_error "Could not download checksums.txt. Refusing to install an unverified binary."
    return 1
  fi
  verify_checksum "${tmp_dir}/${asset}" "${tmp_dir}/checksums.txt" "$asset" || return 1

  tar -xzf "${tmp_dir}/${asset}" -C "$tmp_dir" || return 1
  resolve_install_dir || return 1
  install -m 0755 "${tmp_dir}/${BIN_NAME}" "${TARGET_DIR}/${BIN_NAME}"
  log_success "Installed ${TARGET_DIR}/${BIN_NAME}"

  case ":${PATH}:" in
    *":${TARGET_DIR}:"*) ;;
    *)
      log_warn "${TARGET_DIR} is not in your PATH. Add it with:"
      echo "  export PATH=\"\$PATH:${TARGET_DIR}\""
      ;;
  esac
}

main() {
  echo -e "${BLUE}=== Convergence Installer ===${NC}"
  detect_os_arch || exit 1
  check_prerequisites
  install_binary || exit 1
  echo ""
  log_success "Convergence installation complete."
  echo "Next steps:"
  echo "  convergence init     # choose engine and provider"
  echo "  convergence start    # start the local stack"
}

# Run main when executed directly or piped (curl | bash), not when sourced by tests.
if [[ -z "${BASH_SOURCE[0]:-}" || "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
