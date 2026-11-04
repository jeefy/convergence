#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/../install.sh"

echo "=== Testing install.sh syntax and functions ==="

# 1. Syntax check
bash -n "$INSTALL_SCRIPT"
echo "Syntax check passed."

# 2. Source functions in subshell
(
  source "$INSTALL_SCRIPT"
  detect_os_arch
  echo "Detected OS: $OS, ARCH: $ARCH"
  [ -n "$OS" ] || { echo "OS was empty"; exit 1; }
  [ -n "$ARCH" ] || { echo "ARCH was empty"; exit 1; }
  check_container_engine
)

echo "Installer test passed."
