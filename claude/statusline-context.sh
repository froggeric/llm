#!/bin/bash

# Read JSON input from stdin
input=$(cat)

# Extract model display name
model_name=$(echo "$input" | jq -r '.model.display_name // "Unknown"')
model_indicator="👾 ${model_name}"

# Extract project and current directories
project_dir=$(echo "$input" | jq -r '.workspace.project_dir // ""')
current_dir=$(echo "$input" | jq -r '.workspace.current_dir // ""')

# Build project indicator (handles all 5 cases correctly)
project_indicator=""
if [ -n "$project_dir" ]; then
    project_indicator="🗄️  $(basename "$project_dir")"
fi
if [ -n "$current_dir" ]; then
    folder_name=$(basename "$current_dir")
    if [ -z "$project_dir" ] || [ "$folder_name" != "$(basename "$project_dir")" ]; then
        project_indicator="${project_indicator}${project_indicator:+ }📂  ${folder_name}"
    fi
fi

# ANSI color codes
PALE_LAVENDER='\033[0;38;5;147m'
RESET='\033[0m'

# Emoji function
get_status_emoji() {
    local percent=$1
    if [ "$percent" -lt 70 ]; then echo "🟢"
    elif [ "$percent" -lt 75 ]; then echo "🟡"
    elif [ "$percent" -lt 90 ]; then echo "🟠"
    else echo "🔴"; fi
}

# Format k
format_k() {
    local num=$1
    if [ "$num" -ge 1000 ]; then echo "$((num / 1000))k"
    else echo "$num"; fi
}

# Get context info and calculate percentage from raw token counts
CONTEXT_SIZE=$(echo "$input" | jq -r '.context_window.context_window_size // "200000"')
# Even with auto compact turned off, 1.5% (3k) is reserved for the manual compact buffer
# In addition, the system prompt, tools, agents, etc, take space this can vary per project.
# Reserved space = 24500 (21.5k + 3k)
CURRENT_TOKENS=$(echo "$input" | jq -r '(.context_window.current_usage // {}) | [ 24500, .input_tokens, .cache_creation_input_tokens, .cache_read_input_tokens, .output_tokens] | add // 0')

# Calculate percentage from actual token usage
if [ -n "$CONTEXT_SIZE" ] && [ "$CONTEXT_SIZE" -gt 0 ]; then
    PERCENT_USED=$((CURRENT_TOKENS * 100 / CONTEXT_SIZE))
    used_size_fmt=$(format_k "$CURRENT_TOKENS")
    window_size_fmt=$(format_k "$CONTEXT_SIZE")
    emoji=$(get_status_emoji "$PERCENT_USED")

    if [ -n "$project_indicator" ]; then
        echo -e "${model_indicator} ${project_indicator} ${emoji} ${PALE_LAVENDER}Context: ${used_size_fmt}/${window_size_fmt} used${RESET}"
    else
        echo -e "${model_indicator} ${emoji} ${PALE_LAVENDER}Context: ${used_size_fmt}/${window_size_fmt} used${RESET}"
    fi
else
    # Context window size unavailable
    if [ -n "$project_indicator" ]; then
        echo -e "${model_indicator} ${project_indicator} ⏳ ${PALE_LAVENDER}Context: calculating...${RESET}"
    else
        echo -e "${model_indicator} ⏳ ${PALE_LAVENDER}Context: calculating...${RESET}"
    fi
fi
