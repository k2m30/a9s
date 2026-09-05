#!/usr/bin/env python3
"""Contract tests for require-green-gate.py.

The gate exists because a dev agent's own report is not evidence. "DONE" in a
round entry is a claim; the only thing that makes it true is a captured gate
run showing EXIT=0 for both `make test` and `make lint`. The hook turns the
claim into a checkable fact, so these tests pin exactly when it blocks
(exit 2, with a reason the agent can act on) and when it stays out of the way
(exit 0) -- including every path where the hook has no reliable information,
which must never block.

Run: python3 -m unittest discover -s .claude/hooks -p 'test_*.py'
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import unittest

HOOKS_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HOOKS_DIR))
HOOK = os.path.join(HOOKS_DIR, "require-green-gate.py")

GREEN_GATE = """## gate: make test
ok  \tgithub.com/k2m30/a9s/tests/unit\t18.412s
ok  \tgithub.com/k2m30/a9s/tests/integration\t4.006s
EXIT=0
## gate: make lint
0 issues.
EXIT=0
"""

LINT_RED_GATE = """## gate: make test
ok  \tgithub.com/k2m30/a9s/tests/unit\t18.412s
EXIT=0
## gate: make lint
core/aws/kms.go:41:2: ineffectual assignment to err (ineffassign)
EXIT=1
"""

TEST_ONLY_GATE = """## gate: make test
ok  \tgithub.com/k2m30/a9s/tests/unit\t18.412s
EXIT=0
"""

DONE_MESSAGE = """## a9s-dev · round 1 · DONE

TASKDIR={taskdir}
WORKTREE=/private/tmp/a9s-wt/w10

Landed the enricher and the FindingDef row. Gate captured to gate.txt.
"""

WIP_MESSAGE = """## a9s-dev · round 1 · FINDINGS

TASKDIR={taskdir}

Two callers still fall through; handing back before the gate run.
"""


def run_hook(payload, cwd=REPO_ROOT):
    """Feed the hook a SubagentStop payload; return (exit code, stderr)."""
    proc = subprocess.run(
        [sys.executable, HOOK],
        input=json.dumps(payload) if isinstance(payload, dict) else payload,
        capture_output=True,
        text=True,
        cwd=cwd,
    )
    # The blocking reason may be delivered on stderr or as JSON on stdout;
    # both are legal for a Stop-family hook, so the tests accept either.
    return proc.returncode, proc.stderr + proc.stdout


def payload(message, taskdir):
    return {
        "session_id": "abc123",
        "transcript_path": "/home/user/.claude/projects/x/transcript.jsonl",
        "cwd": REPO_ROOT,
        "permission_mode": "bypassPermissions",
        "hook_event_name": "SubagentStop",
        "agent_id": "subagent-xyz",
        "agent_type": "a9s-dev",
        "last_assistant_message": message.format(taskdir=taskdir),
    }


class GreenGateHookTest(unittest.TestCase):
    def setUp(self):
        self.taskdir = tempfile.mkdtemp(prefix="w10-taskdir-")
        self.addCleanup(shutil.rmtree, self.taskdir, True)
        self.gate = os.path.join(self.taskdir, "gate.txt")

    def write_gate(self, body, age_seconds=0):
        with open(self.gate, "w") as fh:
            fh.write(body)
        if age_seconds:
            when = time.time() - age_seconds
            os.utime(self.gate, (when, when))

    def test_done_without_gate_file_blocks(self):
        """A DONE claim with no captured gate run is unproven, so it blocks."""
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("gate.txt", reason)

    def test_done_with_green_gate_passes(self):
        """Both gates captured at EXIT=0 is the whole requirement; let it stop."""
        self.write_gate(GREEN_GATE)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(0, code, reason)

    def test_done_with_failing_lint_blocks(self):
        """`make lint` at EXIT=1 is a red gate; DONE is false and must not land."""
        self.write_gate(LINT_RED_GATE)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("lint", reason)

    def test_done_with_only_make_test_blocks(self):
        """Half a gate is not a gate: `make lint` must be present, not assumed."""
        self.write_gate(TEST_ONLY_GATE)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("lint", reason)

    def test_done_with_stale_gate_blocks(self):
        """A week-old gate proves an earlier round, not this one."""
        self.write_gate(GREEN_GATE, age_seconds=7 * 24 * 3600)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)

    def test_non_done_message_passes(self):
        """FINDINGS/blocked rounds make no claim to verify; the gate is silent."""
        code, reason = run_hook(payload(WIP_MESSAGE, self.taskdir))
        self.assertEqual(0, code, reason)

    def test_malformed_stdin_passes(self):
        """A hook that crashes the loop on junk input is worse than no hook."""
        for junk in ("", "not json at all", "[1,2,3]", "null"):
            with self.subTest(stdin=junk):
                code, reason = run_hook(junk)
                self.assertEqual(0, code, reason)

    def test_done_without_any_taskdir_blocks(self):
        """Inverted from "passes": a DONE the hook cannot locate used to slip
        through, and every dev round of the first session did exactly that
        (agents run from the primary repo, which has no task-context file, and
        the entry carried no TASKDIR= line), so a red gate.txt was never seen.
        An unlocatable gate is an unproven gate."""
        p = payload(DONE_MESSAGE, self.taskdir)
        p["last_assistant_message"] = "## a9s-dev · round 1 · DONE\n\nno taskdir line here\n"
        empty = tempfile.mkdtemp(prefix="w10-cwd-")
        self.addCleanup(shutil.rmtree, empty, True)
        code, reason = run_hook(p, cwd=empty)
        self.assertEqual(2, code)
        self.assertIn("TASKDIR", reason)

    def test_taskdir_falls_back_to_task_context_file(self):
        """After a compaction the DONE entry can lose its TASKDIR= line; the
        orchestrator-written task-context file is the surviving pointer, and the
        gate must still be enforced through it rather than silently skipped."""
        cwd = tempfile.mkdtemp(prefix="w10-cwd-")
        self.addCleanup(shutil.rmtree, cwd, True)
        os.makedirs(os.path.join(cwd, ".claude"))
        with open(os.path.join(cwd, ".claude", "task-context.md"), "w") as fh:
            fh.write("WORKTREE=/private/tmp/a9s-wt/w10\nTASKDIR=%s\n" % self.taskdir)

        p = payload(DONE_MESSAGE, self.taskdir)
        p["cwd"] = cwd
        p["last_assistant_message"] = "## a9s-dev · round 1 · DONE\n\nlanded it\n"

        code, reason = run_hook(p, cwd=cwd)
        self.assertEqual(2, code)
        self.assertIn("gate.txt", reason)

        self.write_gate(GREEN_GATE)
        code, reason = run_hook(p, cwd=cwd)
        self.assertEqual(0, code, reason)


# The capture shape `a9s-team-loop/SKILL.md` pins for gate.txt: the make output
# writes a `## gate: <name>` marker, redirects the command's output after it,
# and appends the exit code. The marker is what names the gate: a command's own
# text never appears in its output, so nothing else in the file can.
PINNED_CAPTURE_SHAPE = """## gate: make test
make[1]: Entering directory '/private/tmp/a9s-wt/w10'
go test ./tests/unit/ ./tests/integration/
ok  \tgithub.com/k2m30/a9s/tests/unit\t18.412s
ok  \tgithub.com/k2m30/a9s/tests/integration\t4.006s
EXIT=0
## gate: make lint
make[1]: Entering directory '/private/tmp/a9s-wt/w10'
golangci-lint run ./...
0 issues.
EXIT=0
"""

# A red `make test` whose own output quotes an EXIT= line. The repo's captured
# gate convention puts `EXIT=<n>` into logs, and tests that assert on captured
# gate output echo it back, so this shows up in a real `make test` transcript.
RED_TEST_QUOTING_AN_EXIT_LINE = """## gate: make test
--- FAIL: TestGateCaptureShape (0.31s)
    gate_test.go:22: captured gate log was:
        EXIT=0
FAIL\tgithub.com/k2m30/a9s/tests/unit\t18.412s
make: *** [test] Error 1
EXIT=1
## gate: make lint
0 issues.
EXIT=0
"""


class GateFileShapeTest(unittest.TestCase):
    """The parser and the capture recipe have to describe the same file.

    Every path into gate.txt is written by an agent following the recipe the
    team-loop skill pins and the hook's own reason text repeats. A parser that
    only accepts a shape neither of them produces does not gate the round, it
    ends it: the agent re-runs the gates, gets the same file back, and is
    blocked again with the same message.
    """

    def setUp(self):
        self.taskdir = tempfile.mkdtemp(prefix="w10-shape-")
        self.addCleanup(shutil.rmtree, self.taskdir, True)

    def write_gate(self, body):
        with open(os.path.join(self.taskdir, "gate.txt"), "w") as fh:
            fh.write(body)

    def test_gate_captured_the_documented_way_is_accepted(self):
        """A green gate.txt produced by the pinned recipe must close the round."""
        self.write_gate(PINNED_CAPTURE_SHAPE)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(0, code, reason)

    def test_red_gate_is_not_rescued_by_an_exit_line_in_its_own_output(self):
        """The exit code of a command is the last EXIT= line under it, not the
        first. Reading the first lets any test that prints EXIT=0 launder a
        failing gate into a green one -- the exact claim this hook exists to
        refuse."""
        self.write_gate(RED_TEST_QUOTING_AN_EXIT_LINE)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("make test", reason)


# gate.txt after a round that appended to the previous round's file: the
# `make test` section is from the earlier tree, and the gate was re-run only
# for lint. Nothing in the file says which section belongs to which tree.
APPENDED_ACROSS_TWO_RUNS = """## gate: make test
ok  \tgithub.com/k2m30/a9s/tests/unit\t18.412s
EXIT=0
## gate: make lint
core/aws/kms.go:41:2: ineffectual assignment to err (ineffassign)
EXIT=1
## gate: make lint
0 issues.
EXIT=0
"""


class GateFileSingleRunTest(unittest.TestCase):
    """A gate.txt must describe one run of the gates against one tree.

    Two sections for the same gate prove the file spans more than one run, so
    the sections that appear once were captured against an earlier tree. The
    lint fix that produced the second lint section edited code the earlier
    `make test` never saw, which is the same "the gate proves an earlier tree"
    failure the mtime check already refuses -- it is just invisible to mtime,
    because appending refreshes the whole file.
    """

    def setUp(self):
        self.taskdir = tempfile.mkdtemp(prefix="w10-onerun-")
        self.addCleanup(shutil.rmtree, self.taskdir, True)

    def test_gate_spanning_two_runs_does_not_close_the_round(self):
        with open(os.path.join(self.taskdir, "gate.txt"), "w") as fh:
            fh.write(APPENDED_ACROSS_TWO_RUNS)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)


if __name__ == "__main__":
    unittest.main()
