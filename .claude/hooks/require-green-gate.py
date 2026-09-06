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

The same hook enforces the other line a round cannot end without: `deferred:`.
When the first team was asked what it had left open it produced 65 items, a
dozen of them never written anywhere and most of the rest parked under
"pre-existing" or "out of batch". So a dev or QA entry that ends a round
(DONE, FINDINGS, SIGN-OFF) must carry a `deferred:` line -- `none`, or one
item per line with file:line and an owner -- and a `simplified:` line naming
the ponytail-review pass on the round's own diff and its outcome, and an
acceptance verdict (ACCEPT, REJECT) must carry an `observed, out of scope:`
line. The orchestrator routes those lines; the hook only makes sure they exist.

The hook never blocks when it has no reliable information — no parseable
payload, no round-ending status. A DONE it cannot locate is refused, because an
unlocatable gate is an unproven one.
"""

import json
import os
import re
import subprocess
import sys
import time

AGENT = "a9s-dev"
REQUIRED_GATES = ("make test", "make lint")

# Statuses that end a round, per agent, and the line each such entry must carry.
ROUND_END = {
    "a9s-dev": ("DONE",),
    "a9s-qa": ("DONE", "FINDINGS", "SIGN-OFF"),
    "a9s-acceptance": ("ACCEPT", "REJECT"),
}
REQUIRED_LINES = {
    "a9s-dev": ("deferred:", "simplified:"),
    "a9s-qa": ("deferred:", "simplified:"),
    "a9s-acceptance": ("observed, out of scope:",),
}

# A gate older than this is from an earlier round no matter what else is true.
MAX_GATE_AGE_SECONDS = 6 * 60 * 60

# Directories a9s-dev owns. An edit under any of them after the gate run
# invalidates the gate.
OWNED_DIRS = ("core", "internal", "cmd", "scripts", ".a9s")

EXIT_RE = re.compile(r"\bEXIT=(\d+)\b")
GATE_MARKER_RE = re.compile(r"^##\s*gate:\s*(.+?)\s*$")

CAPTURE_RECIPE = """  : > $TASKDIR/gate.txt
  printf '## gate: make test\\n' >> $TASKDIR/gate.txt
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


def uncommitted(worktree):
    """The porcelain status of the round's worktree, or None when it is clean
    or cannot be read. A round that ends with edits outside a commit hands the
    next agent a tree no `from:` line names."""
    if not worktree or not os.path.isdir(worktree):
        return None
    try:
        proc = subprocess.run(
            ["git", "-C", worktree, "status", "--porcelain"],
            capture_output=True,
            text=True,
            timeout=30,
        )
    except (OSError, subprocess.SubprocessError):
        return None
    if proc.returncode != 0:
        return None
    return proc.stdout.strip() or None


def gate_results(text):
    """Map each gate named by a `## gate:` marker to one exit code per section.

    The last exit line in a section wins. A command's output can contain an
    `EXIT=` line of its own -- a test asserting on a captured gate log does
    exactly that -- and only the one the recipe appends last is the command's.

    Sections are kept per occurrence rather than collapsed, because a gate
    appearing twice means the file was appended across two runs and its other
    sections describe an earlier tree.
    """
    sections = {}
    current = None
    for line in text.splitlines():
        marker = GATE_MARKER_RE.match(line)
        if marker:
            current = marker.group(1)
            sections.setdefault(current, []).append(None)
            continue
        if current is None:
            continue
        found = EXIT_RE.search(line)
        if found:
            sections[current][-1] = int(found.group(1))
    return sections


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
        codes = results.get(gate, [])
        if not codes or codes[-1] is None:
            return (
                "gate.txt at %s has no `## gate: %s` section with an exit code."
                % (gate_path, gate)
            )
        if len(codes) > 1:
            return (
                "gate.txt at %s holds %d `## gate: %s` sections, so it spans more "
                "than one run and its earlier sections describe an earlier tree. "
                "Truncate it and capture every gate again." % (gate_path, len(codes), gate)
            )
        if codes[0] != 0:
            return "`%s` exited %d in %s. DONE requires EXIT=0." % (
                gate,
                codes[0],
                gate_path,
            )
    return None


def has_line(message, key):
    """True when some line of the entry starts with `key` and says something
    after it; `- deferred:` with nothing behind it is not a deferral line."""
    for line in message.splitlines():
        stripped = line.strip().lstrip("-*# ").strip()
        if stripped.lower().startswith(key) and stripped[len(key) :].strip():
            return True
    return False


def main():
    payload = read_payload()
    if payload is None:
        return

    agent = payload.get("agent_type") or AGENT
    if agent not in ROUND_END:
        return

    message = str(payload.get("last_assistant_message") or "")
    if not any(status in message for status in ROUND_END[agent]):
        return

    for required in REQUIRED_LINES[agent]:
        if has_line(message, required):
            continue
        if required == "simplified:":
            sys.stderr.write(
                "Round entry ends a round but carries no `simplified:` line. Run "
                "/ponytail-review on the round's own diff, apply what survives the "
                "deletion rule, and write the range reviewed and what was cut or "
                "refused. A round nobody simplified is not finished.\n"
            )
        else:
            sys.stderr.write(
                "Round entry ends a round but carries no `%s` line. Add it: `none`, "
                "or one item per line as file:line — what — why left — owner. "
                "Anything noticed and not written there is a defect of the round.\n"
                % required
            )
        sys.exit(2)

    taskdir, worktree = resolve_paths(message, str(payload.get("cwd") or ""))

    dirty = uncommitted(worktree) if agent in ("a9s-dev", "a9s-qa") else None
    if dirty:
        sys.stderr.write(
            "Round entry ends a round but the worktree at %s has uncommitted "
            "changes:\n%s\nA round is its commit: commit it in the worktree, put the "
            "hash on the entry, and only then end the round.\n" % (worktree, dirty)
        )
        sys.exit(2)

    if agent != AGENT or "DONE" not in message:
        return

    if not taskdir:
        reason = (
            "the round entry has no `TASKDIR=` line and no .claude/task-context.md "
            "exists under the working directory, so the gate cannot be located. "
            "Add `- TASKDIR=<path>` and `- WORKTREE=<path>` to the entry."
        )
    else:
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
