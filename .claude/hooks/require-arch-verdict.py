#!/usr/bin/env python3
"""Block fix dispatches that skipped the architectural question.

CLAUDE.md's Bug Protocol requires every defect to be questioned as
architectural BEFORE the fix is written. Prose in CLAUDE.md failed at that
repeatedly: the question gets applied to items labelled "bug" and skipped for
items labelled "review finding", "test failure", "cleanup" or "residual" —
and the labelling is the same judgement that is supposed to be under scrutiny.

So it is enforced here instead. A dispatch to an implementation agent must
carry an explicit ARCH: verdict per finding. Writing "ARCH: one-off, no class
fix — <reason>" is a perfectly good verdict; the requirement is that the
question was asked on the record, not that the answer be yes.
"""

import json
import sys

IMPLEMENTATION_AGENTS = {"a9s-dev", "a9s-qa"}
MARKER = "ARCH:"

REASON = (
    "Bug Protocol (CLAUDE.md): this dispatch to {agent} contains no 'ARCH:' verdict.\n"
    "\n"
    "Every finding being handed to an implementation agent must first be "
    "questioned as architectural. Add one line per finding:\n"
    "\n"
    "  ARCH: <finding> — architectural: <the class-level fix>\n"
    "  ARCH: <finding> — one-off: <why no class fix applies>\n"
    "\n"
    "Both answers are acceptable. Skipping the question is not. The recurring "
    "failure is fixing the instance a reporter happened to name while the same "
    "defect stays live in a sibling field, a sibling call site, or the other lane."
)


def main() -> None:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return  # never block on a parse failure

    tool_input = payload.get("tool_input", payload)
    agent = str(tool_input.get("subagent_type", ""))
    prompt = str(tool_input.get("prompt", ""))

    if agent not in IMPLEMENTATION_AGENTS:
        return
    if MARKER in prompt:
        return

    print(
        json.dumps(
            {
                "hookSpecificOutput": {
                    "hookEventName": "PreToolUse",
                    "permissionDecision": "deny",
                    "permissionDecisionReason": REASON.format(agent=agent),
                }
            }
        )
    )


if __name__ == "__main__":
    main()
