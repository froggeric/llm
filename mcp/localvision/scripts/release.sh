#!/usr/bin/env bash
# release.sh — thin wrapper around `goreleaser release` for the
# localvision subdirectory.
#
# This script exists because goreleaser config lives at
# mcp/localvision/.goreleaser.yaml (NOT at the repo root), so the
# command must be run from inside the subdirectory. Running it from the
# root silently picks up the wrong config or none at all.
#
# Usage:
#
#	# From anywhere in the repo:
#	mcp/localvision/scripts/release.sh
#
#	# With explicit prereqs:
#	GITHUB_TOKEN=ghp_xxx mcp/localvision/scripts/release.sh
#
# Prerequisites:
#
#  1. goreleaser CLI installed (brew install goreleaser).
#  2. A git tag matching mcp/localvision/v* pushed to the remote.
#  3. The GITHUB_TOKEN environment variable set to a PAT with
#     `repo` scope (for uploading release artifacts).
#  4. Run from a clean working tree (goreleaser refuses to release with
#     uncommitted changes by default).
#
# Optional env vars:
#
#   GITHUB_TOKEN       Required for real releases. Skipped for --snapshot.
#   HOMEBREW_TAP_GITHUB_TOKEN
#                      Documented in .github/workflows/vision-mcp-release.yml.
#                      Unused in v0.1 (Homebrew tap is a v0.2 deliverable);
#                      plumbed through so the workflow YAML stays stable.

set -euo pipefail

# Resolve the script's own directory so it works from any CWD.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$MODULE_DIR"

# Make sure goreleaser is on PATH.
if ! command -v goreleaser >/dev/null 2>&1; then
	echo "release.sh: goreleaser not found on PATH." >&2
	echo "Install it with: brew install goreleaser" >&2
	exit 1
fi

# Default to a real release; allow --snapshot for a local dry run.
SNAPSHOT=0
EXTRA_ARGS=()
for arg in "$@"; do
	case "$arg" in
		--snapshot)
			SNAPSHOT=1
			EXTRA_ARGS+=(--snapshot --clean)
			;;
		--skip-publish)
			EXTRA_ARGS+=(--skip=publish)
			;;
		*)
			EXTRA_ARGS+=("$arg")
			;;
	esac
done

if [[ "$SNAPSHOT" -eq 0 ]]; then
	if [[ -z "${GITHUB_TOKEN:-}" ]]; then
		echo "release.sh: GITHUB_TOKEN env var is required for real releases." >&2
		echo "Use --snapshot for a local dry run that does not push." >&2
		exit 1
	fi
fi

echo "==> Running goreleaser from $MODULE_DIR"
exec goreleaser release "${EXTRA_ARGS[@]}"
