#!/usr/bin/env bash
# scripts/install.sh — curl|sh installer for localvision.
#
# Usage:
#
#	curl -fsSL https://github.com/froggeric/llm/releases/latest/download/install.sh | bash
#
# Or, with a specific version:
#
#	curl -fsSL https://github.com/froggeric/llm/releases/download/mcp%2Flocalvision%2Fv0.1.0/install.sh | bash
#
# The installer:
#   1. Detects OS/arch via uname.
#   2. Rejects unsupported combinations with a clear message.
#   3. Downloads the right release tarball from GitHub Releases.
#   4. Verifies the SHA256 sidecar.
#   5. Extracts `localvision` into /usr/local/bin (or ~/.local/bin).
#
# Supported in v0.1: macOS Apple Silicon (darwin/arm64) only.
#
# This file is the source of truth. goreleaser copies it into each GitHub
# release as a top-level `install.sh` artifact (see release.extra_files in
# .goreleaser.yaml). The README's `latest/download/install.sh` URL
# therefore resolves to this script.
#
# Flags:
#   --dry-run            Print what would happen, then exit 0 without
#                        touching the filesystem or network.
#   --version <ver>      Install a specific version (default: latest).
#   --prefix <dir>       Install into <dir>/bin (default: auto-select
#                        /usr/local or ~/.local).
#   -h, --help           Show this help.

set -euo pipefail

# --- defaults -------------------------------------------------------------

APP_NAME="localvision"
REPO="froggeric/llm"
VERSION="latest"
PREFIX=""
DRY_RUN=0

# --- helpers --------------------------------------------------------------

err()  { echo "install.sh: $*" >&2; }
note() { echo "install.sh: $*"; }

print_usage() {
	cat <<USAGE
Usage: install.sh [options]

Options:
  --dry-run            Print what would happen, then exit 0.
  --version <ver>      Install a specific version (default: latest).
                       Use a tag like "mcp/localvision/v0.1.0" or a
                       bare version like "v0.1.0".
  --prefix <dir>       Install into <dir>/bin (default: /usr/local if
                       sudo is available, otherwise ~/.local).
  -h, --help           Show this help.
USAGE
}

# --- arg parsing ----------------------------------------------------------

while [[ $# -gt 0 ]]; do
	case "$1" in
		--dry-run)        DRY_RUN=1; shift ;;
		--version)        VERSION="${2:?--version requires a value}"; shift 2 ;;
		--prefix)         PREFIX="${2:?--prefix requires a value}"; shift 2 ;;
		-h|--help)        print_usage; exit 0 ;;
		*)
			err "unknown argument: $1"
			print_usage >&2
			exit 2
			;;
	esac
done

# --- OS / arch detection --------------------------------------------------

raw_os="$(uname -s)"
raw_arch="$(uname -m)"

case "$raw_os" in
	Darwin) os="darwin" ;;
	Linux)  os="linux" ;;
	*)      os="$raw_os" ;;
esac

case "$raw_arch" in
	arm64|aarch64) arch="arm64" ;;
	x86_64|amd64)  arch="amd64" ;;
	*)             arch="$raw_arch" ;;
esac

# --- supported target check (v0.1 MVP) -----------------------------------

if [[ "$os" == "darwin" && "$arch" == "arm64" ]]; then
	: # supported
else
	cat >&2 <<EOF
install.sh: target '$os/$arch' is not supported in localvision v0.1.

The v0.1 release is MVP-scoped to macOS Apple Silicon (darwin/arm64).
Support for additional targets will land in v0.2:

  - darwin/amd64     (Intel Mac)
  - linux/amd64      (typical CI / WSL2)
  - linux/arm64      (Raspberry Pi 5, Ampere Altra)
  - windows/amd64
  - windows/arm64

If you are on one of those platforms and want to track v0.2 progress, open
an issue at https://github.com/$REPO/issues.

Detected:
  uname -s = $raw_os  -> $os
  uname -m = $raw_arch -> $arch
EOF
	exit 3
fi

# --- pick an install prefix ----------------------------------------------

if [[ -z "$PREFIX" ]]; then
	if [[ -w /usr/local/bin ]] || command -v sudo >/dev/null 2>&1; then
		PREFIX="/usr/local"
	else
		PREFIX="$HOME/.local"
	fi
fi
install_dir="$PREFIX/bin"
binary_path="$install_dir/$APP_NAME"

# --- resolve the download URL --------------------------------------------

# Normalize VERSION into a GitHub tag. Accept either "latest" or a tag.
if [[ "$VERSION" == "latest" ]]; then
	tag_path="latest"
	dl_base="https://github.com/$REPO/releases/latest/download"
else
	# Allow the user to pass either "v0.1.0" or "mcp/localvision/v0.1.0".
	clean_ver="$VERSION"
	if [[ "$clean_ver" != mcp/localvision/* ]] && [[ "$clean_ver" != mcp%2F* ]]; then
		tag="mcp/localvision/$clean_ver"
	else
		tag="$clean_ver"
	fi
	# URL-encode the slashes for the GitHub tag URL.
	tag_path=$(printf '%s' "$tag" | sed 's|/|%2F|g')
	dl_base="https://github.com/$REPO/releases/download/$tag_path"
fi

archive_name="${APP_NAME}_${os}-${arch}.tar.gz"
checksum_name="checksums.txt"
archive_url="$dl_base/$archive_name"
checksum_url="$dl_base/$checksum_name"

# --- dry-run branch ------------------------------------------------------

if [[ "$DRY_RUN" -eq 1 ]]; then
	cat <<EOF
install.sh --dry-run
  detected OS/arch : $os/$arch
  version          : $VERSION
  install prefix   : $PREFIX
  binary path      : $binary_path
  archive URL      : $archive_url
  checksum URL     : $checksum_url
  tarball extract  : $APP_NAME -> $binary_path

No filesystem or network operations were performed.
EOF
	exit 0
fi

# --- real install --------------------------------------------------------

note "Detected platform: $os/$arch"
note "Installing $APP_NAME $VERSION into $install_dir"

# Make sure the install dir exists.
if [[ ! -d "$install_dir" ]]; then
	if [[ "$PREFIX" == "/usr/local" ]] && command -v sudo >/dev/null 2>&1 && [[ ! -w "/usr/local" ]]; then
		sudo mkdir -p "$install_dir"
	else
		mkdir -p "$install_dir"
	fi
fi

# Download to a temp dir so we can verify SHA256 before touching $PATH.
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

note "Downloading $archive_url"
if ! curl -fsSL "$archive_url" -o "$work/$archive_name"; then
	err "download failed for $archive_url"
	err "(if you targeted a version that does not exist yet, double-check the tag)"
	exit 1
fi

note "Downloading checksums"
if ! curl -fsSL "$checksum_url" -o "$work/$checksum_name"; then
	err "download failed for $checksum_url"
	exit 1
fi

# Verify the SHA256. shasum on macOS, sha256sum on Linux.
if command -v shasum >/dev/null 2>&1; then
	hasher="shasum -a 256"
elif command -v sha256sum >/dev/null 2>&1; then
	hasher="sha256sum"
else
	err "neither shasum nor sha256sum is installed; cannot verify archive integrity"
	exit 1
fi

expected_line=$(grep "  $archive_name\$" "$work/$checksum_name" || true)
if [[ -z "$expected_line" ]]; then
	err "checksum for $archive_name not present in $checksum_url"
	exit 1
fi
expected_hash="$(awk '{print $1}' <<<"$expected_line")"

note "Verifying SHA256 ($expected_hash)"
actual_hash="$(cd "$work" && $hasher "$archive_name" | awk '{print $1}')"
if [[ "$actual_hash" != "$expected_hash" ]]; then
	err "SHA256 mismatch!"
	err "  expected: $expected_hash"
	err "  actual  : $actual_hash"
	err "Refusing to install; the archive may have been corrupted or tampered with."
	exit 1
fi

note "Extracting"
tar -xzf "$work/$archive_name" -C "$work"

# Locate the binary inside the archive (top-level or under a single dir).
src_bin=""
for candidate in "$work/$APP_NAME" "$work/$APP_NAME/$APP_NAME"; do
	if [[ -x "$candidate" ]]; then
		src_bin="$candidate"
		break
	fi
done
if [[ -z "$src_bin" ]]; then
	err "could not find $APP_NAME inside the archive"
	exit 1
fi

note "Installing to $binary_path"
needs_sudo=0
if [[ "$PREFIX" == "/usr/local" ]] && [[ ! -w "$install_dir" ]] && command -v sudo >/dev/null 2>&1; then
	needs_sudo=1
fi

if [[ "$needs_sudo" -eq 1 ]]; then
	sudo install -m 0755 "$src_bin" "$binary_path"
else
	install -m 0755 "$src_bin" "$binary_path"
fi

# --- next steps ----------------------------------------------------------

cat <<EOF

  $APP_NAME installed successfully at $binary_path

Next steps:

  1. Verify the install:
       $binary_path version

  2. Run the doctor to download llama-server + the smallest model
     (5-15 minutes on first run):
       $binary_path doctor

  3. Add it to your MCP client config. For Claude Code:

       {
         "mcpServers": {
           "localvision": {
             "command": "$binary_path",
             "args": ["run"]
           }
         }
       }

  4. Make sure $install_dir is on your \$PATH if you want to call
     '$APP_NAME' directly.

Docs: https://github.com/$REPO/tree/main/mcp/localvision#readme
Issues: https://github.com/$REPO/issues

EOF
