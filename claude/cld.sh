#!/bin/bash

# Script to launch Claude Code with different configurations

set -euo pipefail  # Exit on error, undefined vars, pipe failures

CLAUDE_SETTINGS="$HOME/.claude/settings.json"
NO_LAUNCH=false
SET_PERMS=false

# Default permissions to ensure are always present in settings.json
DEFAULT_PERMISSIONS=(
  # Original permissions
  "Bash(mkdir:*)"
  "Bash(go:*)"
  "Write(*)"
  "Bash(ls:*)"
  "Bash(git:*)"
  "Update(*:*)"
  "Bash(mv:*)"
  "Bash(echo:*)"
  "Bash(sed:*)"
  "Bash(source:*)"
  "Bash(head:*)"
  "Bash(tail:*)"
  "Bash(npm:*)"
  "Bash(npx:*)"
  "Bash(pkill:*)"
  "Bash(touch:*)"
  "Bash(grep:*)"
  "Bash(deadcode:*)"
  "Bash(sqlite3:*)"
  "Bash(curl:*)"
  "Bash(rg:*)"
  "Bash(chmod:*)"
  "Bash(lsof:*)"
  "Bash(make:*)"
  "Bash(PORT=:*)"
  "Bash(find:*)"
  "Bash(docker:*)"
  "Bash(poetry:*)"
  "Bash(python:*)"
  "Bash(python3:*)"
  "Update(*)"
  "Bash(jq:*)"
  # High Priority additions
  "Bash(cat:*)"
  "Bash(cp:*)"
  "Bash(node:*)"
  "Bash(brew:*)"
  "Bash(awk:*)"
  "Bash(sort:*)"
  "Bash(ps:*)"
  # Medium Priority additions
  "Bash(yarn:*)"
  "Bash(pnpm:*)"
  "Bash(gh:*)"
  "Bash(docker-compose:*)"
  "Bash(cargo:*)"
  "Bash(swift:*)"
  "Bash(ssh:*)"
  "Bash(wc:*)"
  "Bash(uniq:*)"
  "Bash(cut:*)"
  # Lower Priority additions
  "Bash(tar:*)"
  "Bash(zip:*)"
  "Bash(unzip:*)"
  "Bash(clang:*)"
  "Bash(xargs:*)"
  "Bash(file:*)"
  "Bash(du:*)"
  "Bash(df:*)"
  "Bash(tree:*)"
  "Bash(open:*)"
)

# Cleanup function for trap
cleanup() {
    if [ -n "${TEMP_FILE:-}" ] && [ -f "$TEMP_FILE" ]; then
        rm -f "$TEMP_FILE" 2>/dev/null || true
    fi
}

trap cleanup EXIT INT TERM

# Function to display usage
show_usage() {
    cat << EOF
Usage: $0 <option> [--nolaunch] [--set-perms]

Options (can be prefixed with --, /, or no prefix):
  unsafe        Launch Claude with --dangerously-skip-permissions flag
  gemini        Use Gemini API (injects env vars for gemini-3 models)
  glm           Use GLM API (injects env vars for api.z.ai)

Flags:
  --nolaunch        Update settings without launching Claude Code
  --set-perms       Ensure default permissions are present in settings.json

If no option is specified, this help message is shown.

Examples:
  $0 unsafe
  $0 --gemini
  $0 /glm --nolaunch
  $0 gemini --set-perms
EOF
}

# Function to check prerequisites
check_prerequisites() {
    local needs_jq="$1"

    # Check if claude command exists
    if ! command -v claude &> /dev/null; then
        echo "Error: 'claude' command not found in PATH"
        echo "Please install Claude Code first"
        exit 1
    fi

    # Check if jq is needed and available
    if [ "$needs_jq" = "true" ]; then
        if ! command -v jq &> /dev/null; then
            echo "Error: jq is required but not installed."
            echo "Install with: brew install jq"
            exit 1
        fi
    fi

    # Check write permissions for .claude directory
    local claude_dir
    claude_dir="$(dirname "$CLAUDE_SETTINGS")"

    if [ -d "$claude_dir" ] && [ ! -w "$claude_dir" ]; then
        echo "Error: No write permission for $claude_dir"
        exit 1
    fi
}

# Function to inject environment variables into settings.json
inject_env_vars() {
    local mode="$1"
    local temp_file
    temp_file="$(mktemp)" || { echo "Error: Failed to create temp file"; exit 1; }
    TEMP_FILE="$temp_file"

    # Create directory if it doesn't exist
    mkdir -p "$(dirname "$CLAUDE_SETTINGS")"

    # Create backup before making changes
    if [ -f "$CLAUDE_SETTINGS" ]; then
        cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup.$(date +%Y%m%d_%H%M%S)" || {
            echo "Error: Failed to create backup"
            exit 1
        }
    fi

    if [ "$mode" = "gemini" ]; then
        # Gemini configuration
        local env_config='{
  "ANTHROPIC_AUTH_TOKEN": "sk-dummy",
  "ANTHROPIC_BASE_URL": "http://127.0.0.1:8317",
  "API_TIMEOUT_MS": "3000000",
  "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
  "ANTHROPIC_DEFAULT_OPUS_MODEL": "gemini-3-pro-preview",
  "ANTHROPIC_DEFAULT_SONNET_MODEL": "gemini-3-pro-preview",
  "ANTHROPIC_DEFAULT_HAIKU_MODEL": "gemini-3-flash-preview"
}'
    elif [ "$mode" = "glm" ]; then
        # GLM configuration
        local env_config='{
  "ANTHROPIC_AUTH_TOKEN": "-=- ENTER YOUR Z.AI API KEY HERE -=-",
  "ANTHROPIC_BASE_URL": "https://api.z.ai/api/anthropic",
  "API_TIMEOUT_MS": "3000000",
  "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
}'
    else
        echo "Error: Unknown mode '$mode'"
        exit 1
    fi

    # Update or create settings.json using jq
    if [ -f "$CLAUDE_SETTINGS" ]; then
        # Validate existing JSON first
        if ! jq empty "$CLAUDE_SETTINGS" &> /dev/null; then
            echo "Error: Existing $CLAUDE_SETTINGS is not valid JSON"
            echo "Backup saved at ${CLAUDE_SETTINGS.backup.*}"
            exit 1
        fi

        # Merge with existing settings
        if ! jq --argjson env "$env_config" '.env = $env' "$CLAUDE_SETTINGS" > "$temp_file"; then
            echo "Error: Failed to update settings.json"
            exit 1
        fi
    else
        # Create new settings file
        if ! echo "{\"env\": $env_config}" | jq '.' > "$temp_file"; then
            echo "Error: Failed to create settings.json"
            exit 1
        fi
    fi

    # Atomic move to final location
    if ! mv "$temp_file" "$CLAUDE_SETTINGS"; then
        echo "Error: Failed to write settings to $CLAUDE_SETTINGS"
        exit 1
    fi

    # Clear temp file reference so cleanup doesn't remove our new file
    TEMP_FILE=""

    echo "✓ Environment variables injected for $mode mode"
}

# Function to set allowed tools permissions based on current configuration
set_permissions() {
    local temp_file
    temp_file="$(mktemp)" || { echo "Error: Failed to create temp file"; exit 1; }
    TEMP_FILE="$temp_file"

    # Create directory if it doesn't exist
    mkdir -p "$(dirname "$CLAUDE_SETTINGS")"

    # Create backup before making changes
    if [ -f "$CLAUDE_SETTINGS" ]; then
        cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup.$(date +%Y%m%d_%H%M%S)" || {
            echo "Error: Failed to create backup"
            exit 1
        }

        # Validate existing JSON first
        if ! jq empty "$CLAUDE_SETTINGS" &> /dev/null; then
            echo "Error: Existing $CLAUDE_SETTINGS is not valid JSON"
            echo "Backup saved at ${CLAUDE_SETTINGS.backup.*}"
            exit 1
        fi

        # Build JSON array of default permissions
        local perms_json
        perms_json=$(printf '%s\n' "${DEFAULT_PERMISSIONS[@]}" | jq -R '.' | jq -s .)

        # Merge default permissions with existing permissions.allow, avoiding duplicates
        if ! jq --argjson new_perms "$perms_json" '
            if .permissions then
                if .permissions.allow then
                    .permissions.allow |= (. + $new_perms | unique)
                else
                    .permissions.allow = $new_perms
                end
            else
                .permissions = {"allow": $new_perms, "deny": []}
            end
        ' "$CLAUDE_SETTINGS" > "$temp_file"; then
            echo "Error: Failed to update permissions"
            exit 1
        fi

        local perm_count
        perm_count=$(jq -r '.permissions.allow | length' "$temp_file")
        echo "✓ Permissions updated ($perm_count permissions allowed)"
    else
        # Create new settings file with default permissions
        local perms_json
        perms_json=$(printf '%s\n' "${DEFAULT_PERMISSIONS[@]}" | jq -R '.' | jq -s .)

        if ! jq --argjson new_perms "$perms_json" '.permissions = {"allow": $new_perms, "deny": []}' <<< '{}' > "$temp_file"; then
            echo "Error: Failed to create settings.json"
            exit 1
        fi

        local perm_count
        perm_count=$(jq -r '.permissions.allow | length' "$temp_file")
        echo "✓ Permissions initialized ($perm_count permissions allowed)"
    fi

    # Atomic move to final location
    if ! mv "$temp_file" "$CLAUDE_SETTINGS"; then
        echo "Error: Failed to write settings to $CLAUDE_SETTINGS"
        exit 1
    fi

    # Clear temp file reference so cleanup doesn't remove our new file
    TEMP_FILE=""
}

# Parse arguments
MODE=""
for arg in "$@"; do
    case "$arg" in
        --nolaunch)
            NO_LAUNCH=true
            ;;
        --set-perms)
            SET_PERMS=true
            ;;
        unsafe|--unsafe|/unsafe)
            if [ -n "$MODE" ]; then
                echo "Error: Multiple modes specified"
                show_usage
                exit 1
            fi
            MODE="/unsafe"
            ;;
        gemini|--gemini|/gemini)
            if [ -n "$MODE" ]; then
                echo "Error: Multiple modes specified"
                show_usage
                exit 1
            fi
            MODE="/gemini"
            ;;
        glm|--glm|/glm)
            if [ -n "$MODE" ]; then
                echo "Error: Multiple modes specified"
                show_usage
                exit 1
            fi
            MODE="/glm"
            ;;
        *)
            echo "Error: Unknown option '$arg'"
            echo ""
            show_usage
            exit 1
            ;;
    esac
done

# Main script logic
case "$MODE" in
    /unsafe)
        check_prerequisites false
        if [ "$SET_PERMS" = true ]; then
            check_prerequisites true
            set_permissions
        fi
        echo "Launching Claude Code with --dangerously-skip-permissions..."
        claude --dangerously-skip-permissions
        ;;
    /gemini)
        check_prerequisites true
        inject_env_vars "gemini"
        if [ "$SET_PERMS" = true ]; then
            set_permissions
        fi
        if [ "$NO_LAUNCH" = false ]; then
            echo "Launching Claude Code with Gemini configuration..."
            claude
        else
            echo "Settings updated. Use '$0 gemini' to launch with these settings."
        fi
        ;;
    /glm)
        check_prerequisites true
        inject_env_vars "glm"
        if [ "$SET_PERMS" = true ]; then
            set_permissions
        fi
        if [ "$NO_LAUNCH" = false ]; then
            echo "Launching Claude Code with GLM configuration..."
            claude
        else
            echo "Settings updated. Use '$0 glm' to launch with these settings."
        fi
        ;;
    "")
        # Handle case where only --set-perms is provided
        if [ "$SET_PERMS" = true ]; then
            check_prerequisites true
            set_permissions
        else
            show_usage
        fi
        exit 0
        ;;
esac

exit 0
