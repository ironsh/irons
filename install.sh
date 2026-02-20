#!/bin/sh
# install.sh - irons installer
#
# Downloads the latest release of irons from GitHub, validates its checksum,
# and installs the binary to an appropriate location on your PATH.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ironsh/irons/main/install.sh | sh
#   wget -qO- https://raw.githubusercontent.com/ironsh/irons/main/install.sh | sh

set -e

REPO="ironsh/irons"
BINARY="irons"
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"

# ── helpers ────────────────────────────────────────────────────────────────────

say()  { printf "  \033[34m•\033[0m %s\n" "$*"; }
ok()   { printf "  \033[32m✓\033[0m %s\n" "$*"; }
warn() { printf "  \033[33m!\033[0m %s\n" "$*" >&2; }
die()  { printf "\n  \033[31m✗ error:\033[0m %s\n\n" "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || die "required tool not found: $1"
}

# ── preflight ──────────────────────────────────────────────────────────────────

need curl
need tar

# ── detect platform ────────────────────────────────────────────────────────────

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) OS_NAME="Darwin" ;;
  Linux)  OS_NAME="Linux"  ;;
  *) die "unsupported operating system: $OS" ;;
esac

case "$ARCH" in
  x86_64|amd64)   ARCH_NAME="x86_64" ;;
  arm64|aarch64)  ARCH_NAME="arm64"  ;;
  *) die "unsupported architecture: $ARCH" ;;
esac

# Validate against what goreleaser actually builds
if [ "$OS_NAME" = "Linux" ] && [ "$ARCH_NAME" = "arm64" ]; then
  die "no Linux arm64 release is available — only Linux x86_64 is built"
fi

ARCHIVE="${BINARY}_${OS_NAME}_${ARCH_NAME}.tar.gz"

# ── resolve latest release ─────────────────────────────────────────────────────

say "Fetching latest release information…"

API_RESPONSE="$(curl -fsSL "$GITHUB_API")" \
  || die "failed to reach GitHub API — check your internet connection"

# Parse tag_name without requiring jq
TAG="$(printf '%s' "$API_RESPONSE" | grep '"tag_name"' \
       | head -1 \
       | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

[ -n "$TAG" ] || die "could not determine the latest release tag"

# goreleaser's .Version strips the leading 'v' from the tag
VERSION="${TAG#v}"

CHECKSUM_FILE="${BINARY}_${VERSION}_checksums.txt"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
ARCHIVE_URL="${BASE_URL}/${ARCHIVE}"
CHECKSUM_URL="${BASE_URL}/${CHECKSUM_FILE}"

ok "Latest release: ${TAG}"
say "Platform:       ${OS_NAME}/${ARCH_NAME}"
say "Archive:        ${ARCHIVE}"

# ── temp workspace ─────────────────────────────────────────────────────────────

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

# ── download ───────────────────────────────────────────────────────────────────

say "Downloading archive…"
curl -fsSL --progress-bar "$ARCHIVE_URL" -o "${TMP}/${ARCHIVE}" \
  || die "download failed: ${ARCHIVE_URL}"

say "Downloading checksums…"
curl -fsSL "$CHECKSUM_URL" -o "${TMP}/${CHECKSUM_FILE}" \
  || die "download failed: ${CHECKSUM_URL}"

# ── validate checksum ──────────────────────────────────────────────────────────

say "Validating checksum…"

# Extract the expected hash line for our archive from the checksums file
HASH_LINE="$(grep " ${ARCHIVE}$" "${TMP}/${CHECKSUM_FILE}" || true)"
[ -n "$HASH_LINE" ] \
  || die "no checksum entry found for '${ARCHIVE}' in ${CHECKSUM_FILE}"

# Write a single-entry checksums file so the verification tool only checks our file
printf '%s\n' "$HASH_LINE" > "${TMP}/check.txt"

cd "$TMP"

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c check.txt --status \
    || die "checksum validation failed — the downloaded archive may be corrupted"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 -c check.txt --status \
    || die "checksum validation failed — the downloaded archive may be corrupted"
else
  warn "No sha256 tool found (sha256sum / shasum) — skipping checksum validation"
fi

ok "Checksum verified"

# ── extract ────────────────────────────────────────────────────────────────────

say "Extracting…"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP"

EXTRACTED="${TMP}/${BINARY}"
[ -f "$EXTRACTED" ] || die "binary '${BINARY}' not found in archive"
chmod +x "$EXTRACTED"

# ── determine install directory ────────────────────────────────────────────────

# Ordered list of preferred install locations
CANDIDATES="/usr/local/bin ${HOME}/.local/bin ${HOME}/bin"

INSTALL_DIR=""

for DIR in $CANDIDATES; do
  if [ -d "$DIR" ] && [ -w "$DIR" ]; then
    INSTALL_DIR="$DIR"
    break
  fi
done

# If none of the writable candidates exist yet, try /usr/local/bin via sudo,
# then fall back to creating ~/.local/bin.
if [ -z "$INSTALL_DIR" ]; then
  if [ -d "/usr/local/bin" ] && sudo -n true 2>/dev/null; then
    INSTALL_DIR="/usr/local/bin"
    USE_SUDO=1
  elif [ -d "/usr/local/bin" ]; then
    say "/usr/local/bin is not writable — sudo is required"
    if sudo true; then
      INSTALL_DIR="/usr/local/bin"
      USE_SUDO=1
    fi
  fi
fi

if [ -z "$INSTALL_DIR" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  warn "Installing to ${INSTALL_DIR} — make sure it is on your PATH"
fi

# ── install ────────────────────────────────────────────────────────────────────

DEST="${INSTALL_DIR}/${BINARY}"

say "Installing to ${DEST}…"

if [ "${USE_SUDO:-0}" = "1" ]; then
  sudo cp "$EXTRACTED" "$DEST"
  sudo chmod +x "$DEST"
else
  cp "$EXTRACTED" "$DEST"
  chmod +x "$DEST"
fi

# ── verify installation ────────────────────────────────────────────────────────

if command -v "$BINARY" >/dev/null 2>&1; then
  INSTALLED_VERSION="$("$BINARY" --version 2>/dev/null || true)"
  ok "Installed: ${INSTALLED_VERSION:-${BINARY} ${TAG}}"
else
  ok "Installed ${BINARY} ${TAG} → ${DEST}"
  warn "${INSTALL_DIR} does not appear to be on your PATH"
  warn "Add the following to your shell profile and restart your session:"
  warn "  export PATH=\"${INSTALL_DIR}:\$PATH\""
fi

printf "\n  Happy sandboxing! 🏖️\n\n"
