#!/usr/bin/env bash
# knet installer — https://github.com/themark007/knew
# Usage: curl -fsSL https://raw.githubusercontent.com/themark007/knew/main/scripts/install.sh | bash
# Or:    curl -fsSL https://raw.githubusercontent.com/themark007/knew/main/scripts/install.sh | bash -s -- --version v0.1.0

set -euo pipefail

REPO="themark007/knew"
INSTALL_DIR="${KNET_INSTALL_DIR:-/usr/local/bin}"
VERSION="${1:-}"   # optional --version flag or first arg

# ─── helpers ──────────────────────────────────────────────────────────────────

info()  { echo "[knet] $*"; }
error() { echo "[knet] ERROR: $*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || error "Required command '$1' not found. Please install it first."
}

# ─── detect OS / arch ─────────────────────────────────────────────────────────

detect_os() {
  case "$(uname -s)" in
    Linux*)  echo "linux";;
    Darwin*) echo "darwin";;
    CYGWIN*|MINGW*|MSYS*) echo "windows";;
    *) error "Unsupported OS: $(uname -s)";;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "x86_64";;
    aarch64|arm64) echo "arm64";;
    *) error "Unsupported architecture: $(uname -m)";;
  esac
}

# ─── resolve version ──────────────────────────────────────────────────────────

resolve_version() {
  if [[ -n "$VERSION" ]]; then
    echo "$VERSION"
    return
  fi
  need curl
  local latest
  latest=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  if [[ -z "$latest" ]]; then
    error "Could not determine latest release. Set KNET_VERSION or pass --version vX.Y.Z"
  fi
  echo "$latest"
}

# ─── main ─────────────────────────────────────────────────────────────────────

main() {
  local os arch version tarball url tmpdir

  need curl
  need tar

  os=$(detect_os)
  arch=$(detect_arch)
  version=$(resolve_version)

  info "Installing knet ${version} for ${os}/${arch}..."

  # Build download URL
  if [[ "$os" == "windows" ]]; then
    tarball="knet_${version#v}_windows_${arch}.zip"
  else
    tarball="knet_${version#v}_${os}_${arch}.tar.gz"
  fi

  url="https://github.com/${REPO}/releases/download/${version}/${tarball}"
  checksum_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT

  info "Downloading ${url}..."
  curl -fsSL "$url" -o "${tmpdir}/${tarball}"

  # Verify checksum if shasum/sha256sum is available
  if command -v sha256sum >/dev/null 2>&1; then
    info "Verifying checksum..."
    curl -fsSL "$checksum_url" -o "${tmpdir}/checksums.txt"
    (cd "$tmpdir" && grep "$tarball" checksums.txt | sha256sum --check --status) \
      || error "Checksum verification failed!"
    info "Checksum OK."
  elif command -v shasum >/dev/null 2>&1; then
    info "Verifying checksum..."
    curl -fsSL "$checksum_url" -o "${tmpdir}/checksums.txt"
    (cd "$tmpdir" && grep "$tarball" checksums.txt | shasum -a 256 --check --status) \
      || error "Checksum verification failed!"
    info "Checksum OK."
  else
    info "Skipping checksum verification (shasum/sha256sum not available)."
  fi

  # Extract
  if [[ "$os" == "windows" ]]; then
    need unzip
    unzip -q "${tmpdir}/${tarball}" -d "$tmpdir"
  else
    tar -xzf "${tmpdir}/${tarball}" -C "$tmpdir"
  fi

  # Install
  local binary="knet"
  [[ "$os" == "windows" ]] && binary="knet.exe"

  if [[ ! -w "$INSTALL_DIR" ]]; then
    info "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo install -m 0755 "${tmpdir}/${binary}" "${INSTALL_DIR}/${binary}"
  else
    install -m 0755 "${tmpdir}/${binary}" "${INSTALL_DIR}/${binary}"
  fi

  info "✓ knet ${version} installed at ${INSTALL_DIR}/${binary}"
  info ""
  info "Quick start:"
  info "  knet version"
  info "  knet scan"
  info "  knet scan -A"
  info "  knet graph full --format tui"
  info "  knet --help"
}

main "$@"
