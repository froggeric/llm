#!/usr/bin/env bash
# build-llama-cpp.sh — build a single-target llama-server binary for
# localvision's release pipeline.
#
# Usage:
#
#	scripts/build-llama-cpp.sh \
#		--target darwin-arm64 \
#		--llama-version b4400 \
#		--out dist/llama-server-darwin-arm64
#
# The script:
#   1. Clones llama.cpp at the requested tag.
#   2. Configures the build for the requested target (Metal on darwin-arm64;
#      other targets print a clear error and exit non-zero until v0.2).
#   3. Produces the binary at --out.
#   4. Writes a <out>.sha256 sidecar alongside the binary.
#
# Requirements:
#   - git, cmake, a working C/C++ toolchain (Xcode CLT on macOS).
#   - Internet access to clone llama.cpp.
#
# This script is invoked by goreleaser via the release workflow. It is also
# runnable standalone for local experimentation.

set -euo pipefail

# --- defaults -------------------------------------------------------------

TARGET=""
LLAMA_VERSION=""
OUT=""
WORKDIR="${TMPDIR:-/tmp}/lvc-build-$$"

# --- arg parsing ----------------------------------------------------------

print_usage() {
	cat <<'USAGE'
Usage: build-llama-cpp.sh --target <platform> --llama-version <tag> --out <path>

Options:
  --target           The target platform. Currently supported:
                       darwin-arm64  (Apple Silicon; Metal-accelerated)
                     Stubs (fail with a clear message until v0.2):
                       darwin-amd64, linux-amd64, linux-arm64,
                       windows-amd64, windows-arm64
  --llama-version    A llama.cpp git tag, branch, or commit (e.g. b4400).
  --out              Destination path for the built binary.
  --workdir          Optional. Where to clone & build. Defaults to a
                     temp directory.
  -h, --help         Show this help.

Environment:
  LLAMA_REPO         Override the llama.cpp git URL
                     (default: https://github.com/ggml-org/llama.cpp)
USAGE
}

while [[ $# -gt 0 ]]; do
	case "$1" in
		--target)         TARGET="$2"; shift 2 ;;
		--llama-version)  LLAMA_VERSION="$2"; shift 2 ;;
		--out)            OUT="$2"; shift 2 ;;
		--workdir)        WORKDIR="$2"; shift 2 ;;
		-h|--help)        print_usage; exit 0 ;;
		*)
			echo "build-llama-cpp.sh: unknown argument: $1" >&2
			print_usage >&2
			exit 2
			;;
	esac
done

if [[ -z "$TARGET" ]]; then
	echo "build-llama-cpp.sh: --target is required" >&2
	exit 2
fi
if [[ -z "$LLAMA_VERSION" ]]; then
	echo "build-llama-cpp.sh: --llama-version is required" >&2
	exit 2
fi
if [[ -z "$OUT" ]]; then
	echo "build-llama-cpp.sh: --out is required" >&2
	exit 2
fi

LLAMA_REPO="${LLAMA_REPO:-https://github.com/ggml-org/llama.cpp}"

# --- per-target setup -----------------------------------------------------

# Each tuple is: <GOOS> <GOARCH> <cmake-toolchain-file-or-empty> <extra-cmake-flags...>
# We support only darwin-arm64 in v0.1. The other targets intentionally exit
# non-zero with a clear message; this prevents the release pipeline from
# silently shipping a broken build.
case "$TARGET" in
	darwin-arm64)
		# Native build with Metal. We assume this script runs on Apple Silicon
		# during the release job (goreleaser is set up for macos-14 runners).
		GGML_METAL="ON"
		EXTRA_FLAGS=(-DLLAMA_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON)
		;;
	darwin-amd64)
		cat >&2 <<EOF
build-llama-cpp.sh: target '$TARGET' is stubbed in v0.1.

The v0.1 release is MVP-scoped to Apple Silicon (darwin-arm64). Cross builds
for Intel macOS will be enabled in v0.2 once we have a CI matrix entry for
them. If you need this today, run the build manually on an Intel Mac.

Ref: PLAN-v2.md F2.2 ("stubbed code that compiles is worse than missing
code that fails").
EOF
		exit 3
		;;
	linux-amd64|linux-arm64|windows-amd64|windows-arm64)
		cat >&2 <<EOF
build-llama-cpp.sh: target '$TARGET' is stubbed in v0.1.

The v0.1 release is MVP-scoped to Apple Silicon (darwin-arm64) only. The
Linux and Windows builds will be enabled in v0.2 along with the
corresponding hardware-detection code paths in internal/models.

Ref: PLAN-v2.md F2.2 ("do not write Linux/Windows files in MVP").
EOF
		exit 3
		;;
	*)
		echo "build-llama-cpp.sh: unknown target '$TARGET'" >&2
		echo "Supported: darwin-arm64 (other targets are v0.2)." >&2
		exit 2
		;;
esac

# --- sanity check the environment ----------------------------------------

require() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "build-llama-cpp.sh: missing required tool '$1'" >&2
		exit 1
	}
}
require git
require cmake
require shasum

# --- do the build --------------------------------------------------------

echo "==> Cloning $LLAMA_REPO @ $LLAMA_VERSION into $WORKDIR"
mkdir -p "$WORKDIR"
git clone --depth 1 --branch "$LLAMA_VERSION" "$LLAMA_REPO" "$WORKDIR/src"

echo "==> Configuring build for $TARGET (Metal=$GGML_METAL)"
cmake -S "$WORKDIR/src" -B "$WORKDIR/build" \
	-DCMAKE_BUILD_TYPE=Release \
	-DBUILD_SHARED_LIBS=OFF \
	-DLLAMA_BUILD_TESTS=OFF \
	-DLLAMA_BUILD_EXAMPLES=OFF \
	-DLLAMA_BUILD_SERVER=ON \
	"${EXTRA_FLAGS[@]}"

echo "==> Building llama-server"
cmake --build "$WORKDIR/build" --config Release -j"$(sysctl -n hw.ncpu 2>/dev/null || nproc)" --target llama-server

# The CMake target name and output path are stable across recent llama.cpp
# releases; the binary is built into <build-dir>/bin/llama-server.
SRC_BIN="$WORKDIR/build/bin/llama-server"
if [[ ! -x "$SRC_BIN" ]]; then
	# Fallback for older layouts where the binary was at <build-dir>/llama-server
	SRC_BIN="$WORKDIR/build/llama-server"
fi
if [[ ! -x "$SRC_BIN" ]]; then
	echo "build-llama-cpp.sh: built binary not found at $WORKDIR/build/bin/llama-server" >&2
	echo "(checked fallbacks; this usually means the llama.cpp layout changed)" >&2
	exit 1
fi

mkdir -p "$(dirname "$OUT")"
cp "$SRC_BIN" "$OUT"
chmod +x "$OUT"

echo "==> Writing SHA256 sidecar"
shasum -a 256 "$OUT" | awk '{print $1}' > "$OUT.sha256"

echo "==> Done."
echo "    binary: $OUT"
echo "    sha256: $(cat "$OUT.sha256")"
