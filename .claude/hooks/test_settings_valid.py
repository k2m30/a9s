#!/usr/bin/env python3
"""Contract tests for the harness config the team loop depends on.

Hooks and CLAUDE.md are load-bearing process code with no compiler behind
them: a trailing comma in settings.json silently disables every hook, and a
CLAUDE.md that keeps growing stops being read. These tests are that compiler.
Each assertion pins a property the loop breaks without -- not a formatting
preference.

Run: python3 -m unittest discover -s .claude/hooks -p 'test_*.py'
"""

import json
import os
import unittest

HOOKS_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(HOOKS_DIR))
SETTINGS = os.path.join(REPO_ROOT, ".claude", "settings.json")

CLAUDE_MD_MAX_LINES = 180
# Retired agents. A prompt naming a subagent that no longer exists produces a
# dispatch failure, not a warning, so every mention has to go.
RETIRED_AGENTS = ("a9s-coder", "a9s-fixtures")
# The confidence bar that contradicted the autonomy rule: "95%+ confidence ...
# ask me follow up questions" told the agent to stop where it is told to decide.
RETIRED_CONFIDENCE_BAR = "95%"

# Paths whose files are worth a knowledge-graph lookup. Scratchpad tasks, /tmp
# and docs are not in the graph, so nudging there is pure noise.
SCOPE_DIRS = ("core/", "internal/", "cmd/", "tests/")


def read(*parts):
    with open(os.path.join(REPO_ROOT, *parts)) as fh:
        return fh.read()


def hooks_for(settings, event):
    return settings.get("hooks", {}).get(event, [])


def commands(entries):
    return [h.get("command", "") for e in entries for h in e.get("hooks", [])]


class SettingsTest(unittest.TestCase):
    def setUp(self):
        with open(SETTINGS) as fh:
            raw = fh.read()
        try:
            self.settings = json.loads(raw)
        except json.JSONDecodeError as exc:
            self.fail("settings.json is not valid JSON: %s" % exc)

    def test_graphify_hooks_drop_the_shouting(self):
        """An all-caps MANDATORY on every read trains the agent to skip the
        hook's text entirely, which costs the rule that actually matters."""
        for cmd in commands(hooks_for(self.settings, "PreToolUse")):
            if "graphify" not in cmd:
                continue
            self.assertNotIn("MANDATORY", cmd)

    def test_graphify_hooks_are_path_scoped(self):
        """The nudge only makes sense for files the graph indexes; firing it on
        scratchpad, /tmp and docs reads made it constant background noise."""
        graphify_cmds = [
            c for c in commands(hooks_for(self.settings, "PreToolUse")) if "graphify" in c
        ]
        self.assertTrue(graphify_cmds, "no graphify PreToolUse hook found")
        for cmd in graphify_cmds:
            self.assertTrue(
                any(d in cmd for d in SCOPE_DIRS),
                "graphify hook is not path-scoped to %s: %s" % (SCOPE_DIRS, cmd),
            )

    def test_session_start_reinjects_task_context_after_compaction(self):
        """Compaction is where WORKTREE/TASKDIR get dropped and an agent starts
        editing the wrong tree; the compact-matcher hook is the only thing that
        puts them back."""
        matchers = [e.get("matcher", "") for e in hooks_for(self.settings, "SessionStart")]
        self.assertIn("compact", matchers)

    def test_subagent_stop_enforces_the_green_gate(self):
        """The gate hook is inert unless it is wired to dev's stop event."""
        entries = hooks_for(self.settings, "SubagentStop")
        wired = [
            e
            for e in entries
            if "a9s-dev" in e.get("matcher", "")
            and any("require-green-gate" in c for c in commands([e]))
        ]
        self.assertTrue(wired, "no SubagentStop hook runs require-green-gate.py for a9s-dev")
        self.assertTrue(os.path.exists(os.path.join(HOOKS_DIR, "require-green-gate.py")))

    def test_gofmt_notice_runs_after_go_edits(self):
        """gofmt drift is found by `make lint` minutes later, in a log the agent
        has to re-read; a notice at the edit is the cheap place to catch it."""
        entries = hooks_for(self.settings, "PostToolUse")
        wired = [
            e
            for e in entries
            if "Edit" in e.get("matcher", "")
            and "Write" in e.get("matcher", "")
            and any("gofmt" in c for c in commands([e]))
        ]
        self.assertTrue(wired, "no PostToolUse gofmt notice on Edit|Write")


class ClaudeMdTest(unittest.TestCase):
    def setUp(self):
        self.text = read("CLAUDE.md")
        self.lines = self.text.splitlines()

    def test_within_line_budget(self):
        self.assertLessEqual(len(self.lines), CLAUDE_MD_MAX_LINES)

    def test_no_retired_agent_names(self):
        for name in RETIRED_AGENTS:
            self.assertNotIn(name, self.text)

    def test_no_confidence_bar_contradicting_autonomy(self):
        self.assertNotIn(RETIRED_CONFIDENCE_BAR, self.text)

    def test_load_bearing_sections_survive_the_trim(self):
        """Shrinking CLAUDE.md by deleting the rules it exists to carry would
        pass the line budget and lose the point."""
        for heading in (
            "Bug Protocol",
            "External Review Protocol",
            "Gate Results",
            "Architecture Principles",
        ):
            self.assertIn(heading, self.text)

    def test_docs_sync_rule_loads_where_it_forbids_an_edit(self):
        """The rule's hardest line is "never edit README.md directly -- it will
        be overwritten by readmegen". A rule scoped only to the code paths that
        trigger a docs update is absent at the moment an agent opens the
        generated file, which is the moment it has to stop."""
        text = read(".claude", "rules", "docs-sync.md")
        head = text.split("---")[1]
        for surface in ("README.md", "website/**"):
            with self.subTest(surface=surface):
                self.assertIn(surface, head)


class ProcessDocsTest(unittest.TestCase):
    def test_team_loop_has_no_wait_and_retry_rule(self):
        """Waiting on a teammate's half-written file only made sense while two
        agents shared a worktree; under the sequenced loop it is dead advice
        that turns a real breakage into a ten-minute stall."""
        text = read(".claude", "skills", "a9s-team-loop", "SKILL.md")
        for remnant in ("wait 30 s", "30 s and retry", "up to 10 times"):
            self.assertNotIn(remnant, text)
        self.assertNotIn("same worktree concurrently", text)

    def test_dev_documents_red_first_and_its_own_probes(self):
        """One implementer: the failing test precedes the fix and its red
        output is on the record, and the round probes its own edge cases."""
        text = read(".claude", "agents", "a9s-dev.md")
        self.assertIn("red", text.lower())
        self.assertIn("checked:", text)

    def test_finding_discipline_reaches_both_reviewing_roles(self):
        """Same sentence, both roles: a gap tied to neither correctness nor a
        stated requirement is disproved, not filed."""
        for agent in ("a9s-acceptance.md",):
            with self.subTest(agent=agent):
                self.assertIn("disproved, not filed", read(".claude", "agents", agent))


if __name__ == "__main__":
    unittest.main()
