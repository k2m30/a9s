#!/usr/bin/env bash
# run-claude.sh — Launch Claude Code in a sandboxed Docker container
#
# Usage:
#   ./run-claude.sh                              # interactive
#   ./run-claude.sh -p "fix the failing tests"   # with prompt
#
# Requires: ANTHROPIC_API_KEY set in environment

set -euo pipefail

IMAGE="a9s-claude"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Build if image doesn't exist
if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    echo "Building $IMAGE image..."
    docker build \
        -f "$SCRIPT_DIR/Dockerfile.claude" \
        --build-arg DEV_UID="$(id -u)" \
        --build-arg DEV_GID="$(id -g)" \
        -t "$IMAGE" \
        "$SCRIPT_DIR"
fi

# Validate required env
if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
    echo "Error: ANTHROPIC_API_KEY is not set" >&2
    exit 1
fi

# Collect env vars to forward
ENV_ARGS=(
    -e ANTHROPIC_API_KEY
    -e "TERM=${TERM:-xterm-256color}"
    -e "LANG=${LANG:-en_US.UTF-8}"
    -e "LC_ALL=${LC_ALL:-en_US.UTF-8}"
    -e "COLORTERM=${COLORTERM:-truecolor}"
)
[ -n "${GITHUB_TOKEN:-}" ]           && ENV_ARGS+=(-e GITHUB_TOKEN)
[ -n "${AWS_ACCESS_KEY_ID:-}" ]      && ENV_ARGS+=(-e AWS_ACCESS_KEY_ID)
[ -n "${AWS_SECRET_ACCESS_KEY:-}" ]  && ENV_ARGS+=(-e AWS_SECRET_ACCESS_KEY)
[ -n "${AWS_SESSION_TOKEN:-}" ]      && ENV_ARGS+=(-e AWS_SESSION_TOKEN)
[ -n "${AWS_REGION:-}" ]             && ENV_ARGS+=(-e AWS_REGION)
[ -n "${AWS_PROFILE:-}" ]            && ENV_ARGS+=(-e AWS_PROFILE)

# Volume mounts
VOL_ARGS=(
    -v "$SCRIPT_DIR":/workspace
)

# Host config — read-only
[ -f "$HOME/.gitconfig" ]   && VOL_ARGS+=(-v "$HOME/.gitconfig:/home/dev/.gitconfig:ro")
[ -d "$HOME/.ssh" ]         && VOL_ARGS+=(-v "$HOME/.ssh:/home/dev/.ssh:ro")
[ -d "$HOME/.aws" ]         && VOL_ARGS+=(-v "$HOME/.aws:/home/dev/.aws:ro")
[ -f "$HOME/.zshrc" ]       && VOL_ARGS+=(-v "$HOME/.zshrc:/home/dev/.zshrc:ro")

# ~/.claude — read-only for config, read-write only for state that must persist
CLAUDE_HOME="$HOME/.claude"
if [ -d "$CLAUDE_HOME" ]; then
    # Config (read-only)
    [ -f "$CLAUDE_HOME/CLAUDE.md" ]          && VOL_ARGS+=(-v "$CLAUDE_HOME/CLAUDE.md:/home/dev/.claude/CLAUDE.md:ro")
    [ -f "$CLAUDE_HOME/settings.json" ]      && VOL_ARGS+=(-v "$CLAUDE_HOME/settings.json:/home/dev/.claude/settings.json:ro")
    [ -f "$CLAUDE_HOME/settings.local.json" ] && VOL_ARGS+=(-v "$CLAUDE_HOME/settings.local.json:/home/dev/.claude/settings.local.json:ro")
    [ -d "$CLAUDE_HOME/agents" ]             && VOL_ARGS+=(-v "$CLAUDE_HOME/agents:/home/dev/.claude/agents:ro")
    [ -d "$CLAUDE_HOME/commands" ]           && VOL_ARGS+=(-v "$CLAUDE_HOME/commands:/home/dev/.claude/commands:ro")
    [ -d "$CLAUDE_HOME/skills" ]             && VOL_ARGS+=(-v "$CLAUDE_HOME/skills:/home/dev/.claude/skills:ro")

    # State (read-write — sessions, history, memory, plugins must persist)
    [ -f "$CLAUDE_HOME/history.jsonl" ]      && VOL_ARGS+=(-v "$CLAUDE_HOME/history.jsonl:/home/dev/.claude/history.jsonl")
    [ -d "$CLAUDE_HOME/sessions" ]           && VOL_ARGS+=(-v "$CLAUDE_HOME/sessions:/home/dev/.claude/sessions")
    [ -d "$CLAUDE_HOME/projects" ]           && VOL_ARGS+=(-v "$CLAUDE_HOME/projects:/home/dev/.claude/projects")
    [ -d "$CLAUDE_HOME/plans" ]              && VOL_ARGS+=(-v "$CLAUDE_HOME/plans:/home/dev/.claude/plans")
    [ -d "$CLAUDE_HOME/tasks" ]              && VOL_ARGS+=(-v "$CLAUDE_HOME/tasks:/home/dev/.claude/tasks")
    [ -d "$CLAUDE_HOME/todos" ]              && VOL_ARGS+=(-v "$CLAUDE_HOME/todos:/home/dev/.claude/todos")
    [ -d "$CLAUDE_HOME/statsig" ]            && VOL_ARGS+=(-v "$CLAUDE_HOME/statsig:/home/dev/.claude/statsig")
    [ -d "$CLAUDE_HOME/telemetry" ]          && VOL_ARGS+=(-v "$CLAUDE_HOME/telemetry:/home/dev/.claude/telemetry")
    [ -d "$CLAUDE_HOME/plugins" ]            && VOL_ARGS+=(-v "$CLAUDE_HOME/plugins:/home/dev/.claude/plugins")
fi

# Shell history (read-write)
[ -f "$HOME/.zsh_history" ] && VOL_ARGS+=(-v "$HOME/.zsh_history:/home/dev/.zsh_history")

exec docker run -it --rm \
    "${ENV_ARGS[@]}" \
    "${VOL_ARGS[@]}" \
    "$IMAGE" \
    "$@"
