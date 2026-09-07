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
- deferred: none
- simplified: ponytail-review on 4f1a9c2..HEAD — nothing proposed
- checked: empty policy → no finding; Deny-only policy → no finding

Landed the enricher and the FindingDef row. Gate captured to gate.txt.
"""

DONE_NO_SIMPLIFIED = """## a9s-dev · round 1 · DONE

TASKDIR={taskdir}
WORKTREE=/private/tmp/a9s-wt/w10
- deferred: none
- checked: empty policy → no finding

Landed the enricher and the FindingDef row. Gate captured to gate.txt.
"""

DONE_NO_DEFERRED = """## a9s-dev · round 1 · DONE

TASKDIR={taskdir}
WORKTREE=/private/tmp/a9s-wt/w10
- simplified: ladder on 4f1a9c2..HEAD — nothing proposed
- checked: empty policy → no finding

Landed the enricher and the FindingDef row. Gate captured to gate.txt.
"""

DONE_EMPTY_DEFERRED = """## a9s-dev · round 1 · DONE

TASKDIR={taskdir}
- deferred:
- simplified: ladder on 4f1a9c2..HEAD — nothing proposed
- checked: empty policy → no finding

Landed it.
"""

DONE_NO_CHECKED = """## a9s-dev · round 2 · DONE
- TASKDIR={taskdir}
- from: 4f1a9c2
- deferred: none
- simplified: ladder on 4f1a9c2..HEAD — nothing proposed
"""

DONE_WITH_DEFERRED = """## a9s-dev · round 2 · DONE
- TASKDIR={taskdir}
- from: 9c2d1e0
- deferred: core/aws/sqs.go:91 — first-match phrase loop — out of batch — owner: d1
- simplified: ponytail-review on 9c2d1e0..HEAD — merged two one-row tables into the existing bench sweep
- checked: empty policy → no finding
"""

DONE_R2_NO_SIMPLIFIED = """## a9s-dev · round 2 · DONE
- TASKDIR={taskdir}
- from: 9c2d1e0
- deferred: core/aws/sqs.go:91 — first-match phrase loop — out of batch — owner: d1
- checked: empty policy → no finding
"""

ACCEPT_NO_OBSERVED = """## a9s-acceptance · round 1 · ACCEPT
criteria: 3 checked, 3 witnessed, 0 failed
gates: make test EXIT=0
"""

REJECT_WITH_OBSERVED = """## a9s-acceptance · round 1 · REJECT
criteria: 3 checked, 3 witnessed, 1 failed
1. row 2 → FAIL — capture acceptance/x.txt:4
observed, out of scope: core/aws/ses.go:105 — phrase from findings[0]
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


DONE_WITH_WORKTREE = """## a9s-dev · round 2 · DONE
- TASKDIR={taskdir}
- WORKTREE={worktree}
- from: 9c2d1e0
- simplified: ladder on 9c2d1e0..HEAD — nothing proposed
- deferred: none
- checked: empty policy → no finding
"""


def git(worktree, *args):
    subprocess.run(["git", "-C", worktree, *args], check=True, capture_output=True)


def scratch_repo():
    """A throwaway git repository with one commit, standing in for a task worktree."""
    repo = tempfile.mkdtemp(prefix="w10-worktree-")
    git(repo, "init", "-q")
    git(repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "base")
    return repo


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

    def test_round_end_with_uncommitted_worktree_blocks(self):
        """Two rounds were reported done with every test file uncommitted
        (w27 and w5, 2026-09-06): the next agent's `from:` named a tree that
        did not hold the work. A round ends with its commit."""
        repo = scratch_repo()
        self.addCleanup(shutil.rmtree, repo, True)
        with open(os.path.join(repo, "stray_test.go"), "w") as fh:
            fh.write("package unit\n")
        p = payload(DONE_WITH_WORKTREE.replace("{worktree}", repo), self.taskdir)
        p["agent_type"] = "a9s-dev"
        code, reason = run_hook(p)
        self.assertEqual(2, code)
        self.assertIn("uncommitted", reason)
        self.assertIn("stray_test.go", reason)

    def test_round_end_with_clean_worktree_passes(self):
        repo = scratch_repo()
        self.addCleanup(shutil.rmtree, repo, True)
        self.write_gate(GREEN_GATE)
        p = payload(DONE_WITH_WORKTREE.replace("{worktree}", repo), self.taskdir)
        p["agent_type"] = "a9s-dev"
        code, reason = run_hook(p)
        self.assertEqual(0, code, reason)

    def test_done_without_gate_file_blocks(self):
        """A DONE claim with no captured gate run is unproven, so it blocks."""
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("gate.txt", reason)

    def test_done_with_green_gate_passes(self):
        """Both gates captured at EXIT=0 plus a deferred line is the whole
        requirement; let it stop."""
        self.write_gate(GREEN_GATE)
        code, reason = run_hook(payload(DONE_MESSAGE, self.taskdir))
        self.assertEqual(0, code, reason)

    def test_done_without_deferred_line_blocks(self):
        """A green gate does not excuse a missing deferred line: the first team
        left 65 items unwritten behind green gates."""
        self.write_gate(GREEN_GATE)
        code, reason = run_hook(payload(DONE_NO_DEFERRED, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("deferred:", reason)

    def test_done_with_empty_deferred_line_blocks(self):
        """`- deferred:` with nothing after it is not a deferral line."""
        self.write_gate(GREEN_GATE)
        code, reason = run_hook(payload(DONE_EMPTY_DEFERRED, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("deferred:", reason)

    def test_done_without_simplified_line_blocks(self):
        """A green gate and a deferred line do not excuse a round nobody
        simplified: four helpers this loop added were deleted by later rounds."""
        self.write_gate(GREEN_GATE)
        code, reason = run_hook(payload(DONE_NO_SIMPLIFIED, self.taskdir))
        self.assertEqual(2, code)
        self.assertIn("simplified:", reason)

    def test_round2_without_simplified_line_blocks(self):
        """QA's own diff gets the same busywork audit before a sign-off."""
        p = payload(DONE_R2_NO_SIMPLIFIED, self.taskdir)
        p["agent_type"] = "a9s-dev"
        code, reason = run_hook(p)
        self.assertEqual(2, code)
        self.assertIn("simplified:", reason)

    def test_done_without_checked_line_blocks(self):
        """A dev round probes its own edge cases; a DONE without the record of
        them is refused like one without gates."""
        self.write_gate(GREEN_GATE)
        p = payload(DONE_NO_CHECKED, self.taskdir)
        p["agent_type"] = "a9s-dev"
        code, reason = run_hook(p)
        self.assertEqual(2, code)
        self.assertIn("checked:", reason)

    def test_done_with_deferred_line_passes(self):
        self.write_gate(GREEN_GATE)
        """QA has no gate.txt contract; the deferred line is its whole check."""
        p = payload(DONE_WITH_DEFERRED, self.taskdir)
        p["agent_type"] = "a9s-dev"
        code, reason = run_hook(p)
        self.assertEqual(0, code, reason)

    def test_acceptance_verdict_without_observed_line_blocks(self):
        """Two acceptance agents kept observations in their heads across two
        rounds each; the verdict now has to say what it saw out of scope."""
        p = payload(ACCEPT_NO_OBSERVED, self.taskdir)
        p["agent_type"] = "a9s-acceptance"
        code, reason = run_hook(p)
        self.assertEqual(2, code)
        self.assertIn("observed, out of scope:", reason)

    def test_acceptance_verdict_with_observed_line_passes(self):
        p = payload(REJECT_WITH_OBSERVED, self.taskdir)
        p["agent_type"] = "a9s-acceptance"
        code, reason = run_hook(p)
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
        p["last_assistant_message"] = "## a9s-dev · round 1 · DONE\n- deferred: none\n- checked: empty input → no finding\n- simplified: ponytail-review on HEAD~1..HEAD — nothing proposed\n\nno taskdir line here\n"
        empty = tempfile.mkdtemp(prefix="w10-cwd-")
        self.addCleanup(shutil.rmtree, empty, True)
        # The hook reads the payload's cwd, not the process cwd, for the
        # context-file fallback; pointing only the process at the empty
        # directory left the test green or red depending on whether the
        # primary repo happened to hold a task-context file.
        p["cwd"] = empty
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
        p["last_assistant_message"] = "## a9s-dev · round 1 · DONE\n- deferred: none\n- checked: empty input → no finding\n- simplified: ponytail-review on HEAD~1..HEAD — nothing proposed\n\nlanded it\n"

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
