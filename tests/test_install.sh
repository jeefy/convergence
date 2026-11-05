#!/usr/bin/env bash
# shellcheck disable=SC1090 # sourcing install.sh under test
# Offline tests for install.sh. curl is replaced by a PATH shim that serves
# files from a local fixture directory and records requested URLs.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/../install.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

PASS=0
fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "ok: $*"; PASS=$((PASS + 1)); }

bash -n "$INSTALL_SCRIPT" || fail "syntax"
ok "syntax"

# --- fixtures: fake release for linux/amd64 ---
FIX="${WORK}/release"
mkdir -p "${FIX}/pkg" "${WORK}/shim"
printf '#!/bin/sh\necho convergence-test\n' >"${FIX}/pkg/convergence"
chmod +x "${FIX}/pkg/convergence"
tar -czf "${FIX}/convergence_linux_amd64.tar.gz" -C "${FIX}/pkg" convergence
if command -v sha256sum >/dev/null 2>&1; then
  SUM="$(sha256sum "${FIX}/convergence_linux_amd64.tar.gz" | awk '{print $1}')"
else
  SUM="$(shasum -a 256 "${FIX}/convergence_linux_amd64.tar.gz" | awk '{print $1}')"
fi
echo "${SUM}  convergence_linux_amd64.tar.gz" >"${FIX}/checksums.good"
echo "0000000000000000000000000000000000000000000000000000000000000000  convergence_linux_amd64.tar.gz" >"${FIX}/checksums.bad"

# curl shim: curl -fsSL URL -o OUT ; serves basename(URL) from $FIX, checksums from $CHECKSUMS
cat >"${WORK}/shim/curl" <<'EOF'
#!/usr/bin/env bash
url="" out=""
while [ $# -gt 0 ]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
echo "$url" >>"$CURL_LOG"
name="${url##*/}"
if [ "$name" = "checksums.txt" ]; then src="$CHECKSUMS"; else src="${FIX}/${name}"; fi
[ -f "$src" ] || exit 22
cp "$src" "$out"
EOF
chmod +x "${WORK}/shim/curl"

# run_install <checksums file> <version> <install dir>; returns exit code
run_install() {
  (
    export PATH="${WORK}/shim:${PATH}" FIX CURL_LOG="${WORK}/curl.log" CHECKSUMS="$1"
    export CONVERGENCE_VERSION="$2" INSTALL_DIR="$3"
    # shellcheck source=/dev/null
    source "$INSTALL_SCRIPT"
    detect_os_arch linux x86_64
    install_binary
  ) >"${WORK}/out.log" 2>&1
}

# 1. platform detection
( source "$INSTALL_SCRIPT"; detect_os_arch Linux aarch64 >/dev/null; [ "$OS/$ARCH" = "linux/arm64" ] ) || fail "linux/aarch64"
( source "$INSTALL_SCRIPT"; detect_os_arch Darwin x86_64 >/dev/null; [ "$OS/$ARCH" = "darwin/amd64" ] ) || fail "darwin/x86_64"
ok "supported platforms map correctly"

if ( source "$INSTALL_SCRIPT"; detect_os_arch MINGW64_NT-10.0 x86_64 ) 2>"${WORK}/err"; then fail "windows accepted"; fi
grep -q "Windows is not supported" "${WORK}/err" || fail "windows message"
if ( source "$INSTALL_SCRIPT"; detect_os_arch FreeBSD amd64 ) 2>/dev/null; then fail "freebsd accepted"; fi
if ( source "$INSTALL_SCRIPT"; detect_os_arch Linux i686 ) 2>"${WORK}/err"; then fail "i686 accepted"; fi
grep -q "Unsupported architecture" "${WORK}/err" || fail "arch message"
ok "unsupported platforms rejected"

# 2. successful install, latest
rm -f "${WORK}/curl.log"
run_install "${FIX}/checksums.good" latest "${WORK}/bin" || { cat "${WORK}/out.log"; fail "good install"; }
[ "$("${WORK}/bin/convergence")" = "convergence-test" ] || fail "installed binary"
grep -qx "https://github.com/jeefy/convergence/releases/latest/download/convergence_linux_amd64.tar.gz" "${WORK}/curl.log" || fail "latest url"
grep -qx "https://github.com/jeefy/convergence/releases/latest/download/checksums.txt" "${WORK}/curl.log" || fail "latest checksums url"
ok "latest install verifies checksum and installs"

# 3. pinned version URL (with and without leading v)
for v in v0.1.0 0.1.0; do
  rm -f "${WORK}/curl.log"
  run_install "${FIX}/checksums.good" "$v" "${WORK}/bin-$v" || fail "pinned install $v"
  grep -qx "https://github.com/jeefy/convergence/releases/download/v0.1.0/convergence_linux_amd64.tar.gz" "${WORK}/curl.log" || fail "pinned url $v"
done
ok "pinned version URL construction"

# 4. checksum mismatch refuses and installs nothing
if run_install "${FIX}/checksums.bad" latest "${WORK}/bin-bad"; then fail "mismatch accepted"; fi
grep -q "Checksum mismatch" "${WORK}/out.log" || fail "mismatch message"
[ ! -e "${WORK}/bin-bad/convergence" ] || fail "binary installed despite mismatch"
ok "checksum mismatch refused"

# 5. missing checksum entry refuses
: >"${WORK}/empty.txt"
if run_install "${WORK}/empty.txt" latest "${WORK}/bin-empty"; then fail "missing entry accepted"; fi
ok "missing checksum entry refused"

# 6. installer never invokes sudo
if grep -nE '^[^#]*\bsudo[[:space:]]+[a-z]' "$INSTALL_SCRIPT" | grep -v 'echo\|log_' >/dev/null; then fail "sudo used"; fi
ok "no sudo invocation"

echo "Installer tests passed (${PASS})."
