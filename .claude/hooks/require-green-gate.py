#!/usr/bin/env python3
"""Block a dev round that reports DONE without a green gate to show for it.

The team protocol says a round ends `DONE` only when the gates are green, and
that gate results are pasted from captured output. Both halves are prose, and
prose is what a tired agent skips at the end of a long round: the entry says
DONE, the gate was run three edits ago, or run and not read, or not run.

So it is enforced here. On `SubagentStop` for a9s-dev, a message claiming DONE
must be backed by `$TASKDIR/gate.txt` holding EXIT=0 for `make test` and for
`make lint`, captured after the last edit to the worktree. Exit 2 hands the
reason back and the agent keeps working.

gate.txt has one shape, pinned by the capture recipe in the a9s-team-loop
skill: a `## gate: <name>` marker line, the command's output, then its exit
code. This file parses that shape and nothing else, so the recipe and the
parser cannot drift apart. Within a section the LAST exit line wins -- an
`EXIT=0` quoted inside a test transcript must never rescue a red gate.

The hook never blocks when it has no reliable information — no parseable
payload, no TASKDIR, no DONE claim. A gate that guesses is worse than no gate.
"""

import json
import os
import re
import sys
import time

AGENT = "a9s-dev"
REQUIRED_GATES = ("make test", "make lint")

# A gate older than this is from an earlier round no matter what else is true.
MAX_GATE_AGE_SECONDS = 6 * 60 * 60

# Directories a9s-dev owns. An edit under any of them after the gate run
# invalidates the gate.
OWNED_DIRS = ("core", "internal", "cmd", "scripts", ".a9s")

EXIT_RE = re.compile(r"\bEXIT=(\d+)\b")
GATE_MARKER_RE = re.compile(r"^##\s*gate:\s*(.+?)\s*$")

CAPTURE_RECIPE = """  printf '## gate: make test\\n' >> $TASKDIR/gate.txt
  make -C $WORKTREE test >> $TASKDIR/gate.txt 2>&1
  printf 'EXIT=%s\\n' $? >> $TASKDIR/gate.txt
  printf '## gate: make lint\\n' >> $TASKDIR/gate.txt
  make -C $WORKTREE lint >> $TASKDIR/gate.txt 2>&1
  printf 'EXIT=%s\\n' $? >> $TASKDIR/gate.txt"""


def read_payload():
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return None
    return payload if isinstance(payload, dict) else None


def path_from_lines(text, key):
    for line in text.splitlines():
        stripped = line.strip().lstrip("-*# ").strip()
        if stripped.startswith(key + "="):
            value = stripped[len(key) + 1 :].strip().strip("`\"'")
            if value:
                return value
    return None


def resolve_paths(message, cwd):
    """TASKDIR and WORKTREE come from the round entry, or from the context file
    the orchestrator writes per dispatch (the entry loses them on compaction)."""
    taskdir = path_from_lines(message, "TASKDIR")
    worktree = path_from_lines(message, "WORKTREE")
    if taskdir:
        return taskdir, worktree

    context = os.path.join(cwd or ".", ".claude", "task-context.md")
    try:
        with open(context) as fh:
            body = fh.read()
    except OSError:
        return None, worktree
    return path_from_lines(body, "TASKDIR"), worktree or path_from_lines(body, "WORKTREE")


def gate_results(text):
    """Map each gate named by a `## gate:` marker to the exit code under it.

    The last exit line in a section wins. A command's output can contain an
    `EXIT=` line of its own -- a test asserting on a captured gate log does
    exactly that -- and only the one the recipe appends last is the command's.
    """
    results = {}
    current = None
    for line in text.splitlines():
        marker = GATE_MARKER_RE.match(line)
        if marker:
            current = marker.group(1)
            continue
        if current is None:
            continue
        found = EXIT_RE.search(line)
        if found:
            results[current] = int(found.group(1))
    return results


def newest_owned_mtime(worktree):
    newest = 0.0
    for owned in OWNED_DIRS:
        root = os.path.join(worktree, owned)
        for dirpath, dirnames, filenames in os.walk(root):
            dirnames[:] = [d for d in dirnames if d != ".git"]
            for name in filenames:
                try:
                    mtime = os.stat(os.path.join(dirpath, name)).st_mtime
                except OSError:
                    continue
                newest = max(newest, mtime)
    return newest


def failure(gate_path, worktree):
    """The reason this DONE is not proven, or None when it is."""
    try:
        gate_mtime = os.stat(gate_path).st_mtime
        with open(gate_path) as fh:
            body = fh.read()
    except OSError:
        return (
            "no gate.txt at %s. Run the gates, capture each exit code into it, "
            "then close the round." % gate_path
        )

    if time.time() - gate_mtime > MAX_GATE_AGE_SECONDS:
        return "gate.txt at %s is older than this round. Re-run the gates." % gate_path

    if worktree and os.path.isdir(worktree):
        newest = newest_owned_mtime(worktree)
        if newest > gate_mtime:
            return (
                "gate.txt at %s predates the last edit under %s. The gate proves "
                "an earlier tree; re-run it." % (gate_path, worktree)
            )

    results = gate_results(body)
    for gate in REQUIRED_GATES:
        if gate not in results:
            return (
                "gate.txt at %s has no `## gate: %s` section with an exit code."
                % (gate_path, gate)
            )
        if results[gate] != 0:
            return "`%s` exited %d in %s. DONE requires EXIT=0." % (
                gate,
                results[gate],
                gate_path,
            )
    return None


def main():
    payload = read_payload()
    if payload is None:
        return

    if payload.get("agent_type") and payload["agent_type"] != AGENT:
        return

    message = str(payload.get("last_assistant_message") or "")
    if "DONE" not in message:
        return

    taskdir, worktree = resolve_paths(message, str(payload.get("cwd") or ""))
    if not taskdir:
        return

    reason = failure(os.path.join(taskdir, "gate.txt"), worktree)
    if reason is None:
        return

    sys.stderr.write(
        "Round reports DONE but the gate is not proven: %s\n"
        "\n"
        "Capture both gates into $TASKDIR/gate.txt with this recipe:\n"
        "%s\n" % (reason, CAPTURE_RECIPE)
    )
    sys.exit(2)


if __name__ == "__main__":
    main()
