#!/usr/bin/env bash
# install.sh - Convergence One-Liner Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/jeefy/convergence/main/install.sh | bash

set -euo pipefail

REPO="jeefy/convergence"
CONVERGENCE_VERSION="${CONVERGENCE_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# ANSI Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

detect_os_arch() {
  local os arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"

  case "$os" in
    darwin) OS="darwin" ;;
    linux)  OS="linux" ;;
    *)
      log_error "Unsupported operating system: $os"
      exit 1
      ;;
  esac

  case "$arch" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
      log_error "Unsupported architecture: $arch"
      exit 1
      ;;
  esac

  log_info "Detected platform: ${OS}/${ARCH}"
}

check_container_engine() {
  if command -v docker >/dev/null 2>&1; then
    log_success "Found container engine: docker"
  elif command -v podman >/dev/null 2>&1; then
    log_success "Found container engine: podman"
  else
    log_warn "Neither docker nor podman was found in PATH."
    log_warn "Convergence requires Docker or Podman to run containers locally."
  fi
}

install_binary() {
  local target_dir="$INSTALL_DIR"
  local bin_name="convergence"

  if [ ! -w "$target_dir" ]; then
    if [ -d "$HOME/.local/bin" ]; then
      target_dir="$HOME/.local/bin"
    else
      mkdir -p "$HOME/.local/bin"
      target_dir="$HOME/.local/bin"
    fi
  fi

  local asset_url
  if [ "$CONVERGENCE_VERSION" = "latest" ]; then
    asset_url="https://github.com/${REPO}/releases/latest/download/convergence_${OS}_${ARCH}.tar.gz"
  else
    asset_url="https://github.com/${REPO}/releases/download/${CONVERGENCE_VERSION}/convergence_${OS}_${ARCH}.tar.gz"
  fi

  log_info "Downloading Convergence binary from ${asset_url}..."
  local tmp_dir
  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  if curl -fsSL "$asset_url" -o "${tmp_dir}/convergence.tar.gz" 2>/dev/null; then
    tar -xzf "${tmp_dir}/convergence.tar.gz" -C "${tmp_dir}"
    mv "${tmp_dir}/convergence" "${target_dir}/${bin_name}"
    chmod +x "${target_dir}/${bin_name}"
    log_success "Installed ${bin_name} to ${target_dir}/${bin_name}"
  else
    log_warn "Pre-compiled release artifact not found (or repo release not yet published)."
    log_info "Attempting build from source via go if available..."
    if command -v go >/dev/null 2>&1; then
      GOBIN="$target_dir" go install "github.com/${REPO}/cmd/convergence@latest"
      log_success "Installed ${bin_name} from source into ${target_dir}"
    else
      log_error "Could not download binary and 'go' is not installed."
      exit 1
    fi
  fi

  # Path guidance
  if [[ ":$PATH:" != *":$target_dir:"* ]]; then
    log_warn "$target_dir is not in your PATH. Add it with:"
    echo "  export PATH=\"\$PATH:${target_dir}\""
  fi
}

main() {
  echo -e "${BLUE}=== Convergence Installer ===${NC}"
  detect_os_arch
  check_container_engine
  install_binary

  echo ""
  log_success "Convergence installation complete!"
  echo "Next steps:"
  echo "  1. Run 'convergence init' to set up your AI Gateway"
  echo "  2. Run 'convergence start' to launch services"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
