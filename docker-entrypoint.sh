#!/bin/bash
# docker-entrypoint.sh — Pre-accept trust dialog, inject auth, launch Claude Code
#
# Host ~/.claude.json is mounted read-only at /tmp/host-claude.json.
# OAuth credentials are passed via CLAUDE_CREDENTIALS env var (JSON blob).
# This script sets up ~/.claude.json with workspace trust and writes
# credentials to ~/.claude/.credentials.json (plaintext fallback for no-keychain envs).

CLAUDE_JSON="$HOME/.claude.json"
HOST_JSON="/tmp/host-claude.json"

if [ -f "$HOST_JSON" ]; then
    cp "$HOST_JSON" "$CLAUDE_JSON"
else
    echo '{"numStartups":1}' > "$CLAUDE_JSON"
fi

# Inject /workspace trust
if command -v jq >/dev/null 2>&1; then
    jq '
      .projects["/workspace"] //= {} |
      .projects["/workspace"].hasTrustDialogAccepted = true |
      .projects["/workspace"].allowedTools //= ["*"] |
      .hasCompletedOnboarding = true
    ' "$CLAUDE_JSON" > /tmp/.claude.json.tmp && mv /tmp/.claude.json.tmp "$CLAUDE_JSON"
fi

# Write OAuth credentials to plaintext fallback (used when keychain is unavailable)
if [ -n "${CLAUDE_CREDENTIALS:-}" ]; then
    echo "$CLAUDE_CREDENTIALS" > "$HOME/.claude/.credentials.json"
    chmod 600 "$HOME/.claude/.credentials.json"
    unset CLAUDE_CREDENTIALS
fi

exec claude --dangerously-skip-permissions "$@"
