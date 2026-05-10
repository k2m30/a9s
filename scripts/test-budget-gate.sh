#!/usr/bin/env bash
# test-budget-gate.sh — capture and gate the Stage 6 test-suite wall budget.
#
# AS-104: enforces the 5-minute (300s) wall budget defined in
# docs/development-process.md §"Test suite wall budget" by failing the build
# when `make test` (non-race) on ubuntu-latest exceeds the budget.
#
# AS-6 baseline (run #25618422075, 2026-05-09):
#   ubuntu 1m07s · macos 1m05s · windows 1m44s
# Headroom on ubuntu is ~4m, so the 5m budget is generous; if a future change
# trips the gate, profile the slow test/package rather than raising the budget.
#
# Usage:
#   scripts/test-budget-gate.sh capture   # run `make test`, write test-budget.json
#   scripts/test-budget-gate.sh gate      # read test-budget.json, exit 1 if over
#
# Environment overrides (for local testing of the gate path):
#   BUDGET_OVERRIDE_SECONDS  — replace 300 with this value when reading JSON
#                              (lets you force-fail the gate locally).

set -euo pipefail

BUDGET_SECONDS=300
ARTIFACT="${ARTIFACT:-test-budget.json}"

usage() {
    cat >&2 <<EOF
Usage: $0 {capture|gate}

  capture   Run \`make test\`, time it, write JSON to ${ARTIFACT}.
            Exits with \`make test\`'s exit code (preserves test failures).

  gate      Read ${ARTIFACT}; exit 1 if wall_seconds > budget_seconds, else 0.
EOF
    exit 2
}

capture() {
    local start_epoch end_epoch wall go_version git_sha captured_at headroom headroom_pct
    local test_exit

    start_epoch=$(date +%s)
    set +e
    make test
    test_exit=$?
    set -e
    end_epoch=$(date +%s)

    wall=$((end_epoch - start_epoch))
    go_version=$(go env GOVERSION 2>/dev/null || echo "unknown")
    git_sha=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    captured_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    headroom=$((BUDGET_SECONDS - wall))
    # bash has no float math; use awk for percent.
    headroom_pct=$(awk -v b="$BUDGET_SECONDS" -v w="$wall" 'BEGIN { if (b == 0) { print 0 } else { printf "%.2f", (b - w) * 100.0 / b } }')

    # Determine OS label without relying on $RUNNER_OS being set (locally we
    # have neither). Pick a stable, descriptive value per platform.
    local os_label="${RUNNER_OS:-}"
    if [ -z "$os_label" ]; then
        case "$(uname -s)" in
            Linux) os_label="ubuntu-latest" ;;
            Darwin) os_label="macos-latest" ;;
            MINGW*|MSYS*|CYGWIN*) os_label="windows-latest" ;;
            *) os_label="$(uname -s)" ;;
        esac
    fi

    cat > "$ARTIFACT" <<EOF
{
  "schema_version": 1,
  "wall_seconds": ${wall},
  "budget_seconds": ${BUDGET_SECONDS},
  "headroom_seconds": ${headroom},
  "headroom_pct": ${headroom_pct},
  "os": "${os_label}",
  "go_version": "${go_version}",
  "git_sha": "${git_sha}",
  "captured_at_utc": "${captured_at}"
}
EOF

    echo "test-budget: wrote ${ARTIFACT} (wall=${wall}s budget=${BUDGET_SECONDS}s headroom=${headroom}s)" >&2
    exit "$test_exit"
}

gate() {
    if [ ! -f "$ARTIFACT" ]; then
        echo "FAIL: ${ARTIFACT} not found — run 'scripts/test-budget-gate.sh capture' first" >&2
        exit 1
    fi

    # Parse with grep+sed to avoid a jq dependency (CI runners have grep, not jq).
    local wall budget
    wall=$(grep -E '"wall_seconds"' "$ARTIFACT" | sed -E 's/.*: *([0-9]+).*/\1/')
    budget=$(grep -E '"budget_seconds"' "$ARTIFACT" | sed -E 's/.*: *([0-9]+).*/\1/')

    if [ -z "$wall" ] || [ -z "$budget" ]; then
        echo "FAIL: ${ARTIFACT} missing wall_seconds or budget_seconds" >&2
        exit 1
    fi

    # Allow local override of the budget to force-fail the gate (negative-path
    # demo without editing JSON by hand).
    if [ -n "${BUDGET_OVERRIDE_SECONDS:-}" ]; then
        budget="$BUDGET_OVERRIDE_SECONDS"
        echo "test-budget: BUDGET_OVERRIDE_SECONDS=${budget} (override active)" >&2
    fi

    if [ "$wall" -gt "$budget" ]; then
        echo "FAIL: test-suite wall ${wall}s > budget ${budget}s — see docs/development-process.md §'Test suite wall budget'" >&2
        exit 1
    fi

    echo "PASS: test-suite wall ${wall}s <= budget ${budget}s" >&2
    exit 0
}

case "${1:-}" in
    capture) capture ;;
    gate)    gate ;;
    *)       usage ;;
esac
