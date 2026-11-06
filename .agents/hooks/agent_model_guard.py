#!/usr/bin/env python3
"""PreToolUse hook: a general-purpose agent must name its model (PARALLEL_WORK §6).

Without one it inherits the lead's model, which is how most subagent work ended up
on Opus. Forks and agent types with their own model are exempt.
"""
import json
import sys

UNTYPED = {None, "", "general-purpose"}
MESSAGE = ("agent_model_guard: pass model for this agent: haiku, sonnet, or opus for hard "
           "reasoning and all art (.agents/specs/PARALLEL_WORK.md §6).")


def check(tool_input):
    """Returns the reason to deny a spawn, or None to allow it."""
    if tool_input.get("subagent_type") in UNTYPED and not tool_input.get("model"):
        return MESSAGE
    return None


def main():
    try:
        event = json.load(sys.stdin)
    except ValueError:
        return 0
    reason = check(event.get("tool_input") or {})
    if reason:
        print(reason, file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
