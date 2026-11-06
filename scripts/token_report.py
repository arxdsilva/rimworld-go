#!/usr/bin/env python3
"""Token usage scorecard from Claude Code transcripts (read-only).

Prints the metrics tracked in .agents/handoffs/TOKEN-001-usage-guards.md, so a
later run can be compared with the baseline.

  python3 scripts/token_report.py --last 400
  python3 scripts/token_report.py --since 2026-11-02
  python3 scripts/token_report.py --last 400 --replay   # dry-run the read guard
"""
import argparse
import collections
import datetime
import glob
import importlib.util
import json
import os
import re
import statistics
import sys

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
GUARD_MARK = "read_guard:"
SHELL_READERS = ("grep", "rg", "sed", "cat", "head", "tail", "ls", "find", "wc",
                 "awk", "git", "python3", "python", "for", "echo", "curl", "gh")


def transcripts_dir():
    return os.path.expanduser("~/.claude/projects/" + re.sub(r"[^A-Za-z0-9-]", "-", REPO))


def select_files(root, since, last):
    files = glob.glob(os.path.join(root, "**", "*.jsonl"), recursive=True)
    files.sort(key=os.path.getmtime)
    if since:
        cutoff = datetime.datetime.fromisoformat(since).timestamp()
        files = [f for f in files if os.path.getmtime(f) >= cutoff]
    return files[-last:] if last else files


def result_text(block):
    content = block.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        return "".join(x.get("text", "") for x in content if isinstance(x, dict))
    return ""


def classify_bash(command):
    """Names the command that produced the output, ignoring cd and env prefixes."""
    c = re.sub(r"^\s*(cd\s+\S+\s*(&&|;)\s*)+", "", command).strip()
    c = re.sub(r"^([A-Z_]+=\S+\s+)+", "", c)
    if re.search(r"\bgo test\b", c):
        return "go test"
    go = re.search(r"\bgo (build|vet|run|tool)\b", c)
    if go:
        return "go " + go.group(1)
    if re.match(r"make\b", c):
        return "make"
    word = (c.split() or ["?"])[0]
    return word if word in SHELL_READERS else "other"


def agent_type(path):
    try:
        with open(path[:-len(".jsonl")] + ".meta.json") as f:
            return json.load(f).get("agentType") or "?"
    except (OSError, ValueError):
        return "?"


def short_model(model):
    return (model or "?").replace("claude-", "").split("-")[0]


class Report:
    def __init__(self):
        self.seen = set()
        self.calls = collections.Counter()
        self.reads = collections.Counter()
        self.writes = 0
        self.output = 0
        self.model_reads = collections.defaultdict(collections.Counter)
        self.type_reads = collections.Counter()
        self.type_agents = collections.Counter()
        self.workers = []
        self.tools = collections.Counter()
        self.tool_calls = collections.Counter()
        self.bash = collections.Counter()
        self.denials = 0
        self.calls_log = []

    def add_file(self, path):
        surface = "subagent" if "/subagents/" in path else "main"
        kind = agent_type(path) if surface == "subagent" else "main"
        pending = {}
        reads = peak = calls = 0
        model = None
        with open(path, errors="ignore") as f:
            for line in f:
                try:
                    entry = json.loads(line)
                except ValueError:
                    continue
                msg = entry.get("message") or {}
                if entry.get("type") == "assistant":
                    used = self.add_usage(msg, surface)
                    if used:
                        reads += used[0]
                        peak = max(peak, used[1])
                        calls += 1
                        model = msg.get("model")
                content = msg.get("content")
                if not isinstance(content, list):
                    continue
                for block in content:
                    if isinstance(block, dict):
                        self.add_block(block, pending)
        if calls and surface == "subagent":
            self.workers.append((reads, peak))
            key = (kind, short_model(model))
            self.type_reads[key] += reads
            self.type_agents[key] += 1

    def add_usage(self, msg, surface):
        """Counts one API call once: a message spans several transcript lines."""
        mid = msg.get("id")
        model = str(msg.get("model", ""))
        if not mid or mid in self.seen or model.startswith("<"):
            return None
        self.seen.add(mid)
        usage = msg.get("usage") or {}
        read = usage.get("cache_read_input_tokens") or 0
        write = usage.get("cache_creation_input_tokens") or 0
        self.calls[surface] += 1
        self.reads[surface] += read
        self.writes += write
        self.output += usage.get("output_tokens") or 0
        self.model_reads[surface][short_model(model)] += read
        return read, read + write + (usage.get("input_tokens") or 0)

    def add_block(self, block, pending):
        if block.get("type") == "tool_use":
            pending[block.get("id")] = (block.get("name"), block.get("input") or {})
        elif block.get("type") == "tool_result" and block.get("tool_use_id") in pending:
            name, tool_input = pending.pop(block["tool_use_id"])
            text = result_text(block)
            self.tools[name] += len(text)
            self.tool_calls[name] += 1
            if name == "Bash":
                self.bash[classify_bash(tool_input.get("command", ""))] += len(text)
            if name in ("Read", "Bash"):
                if GUARD_MARK in text[:200]:
                    self.denials += 1
                self.calls_log.append((name, tool_input, len(text)))


def pct(part, whole):
    return 100 * part / whole if whole else 0


def per_call(reads, calls):
    return f"{reads / calls / 1000:.0f}k" if calls else "n/a"


def print_scorecard(r, nfiles):
    calls = sum(r.calls.values())
    reads = sum(r.reads.values())
    sub = r.reads["subagent"]
    output = sum(r.tools.values())
    print(f"transcripts {nfiles}, API calls {calls}")
    print(f"cache reads {reads / 1e9:.2f}B, cache writes {r.writes / 1e6:.0f}M, output {r.output / 1e6:.1f}M")
    print(f"subagent share of cache reads    {pct(sub, reads):.0f}%")
    print()
    print("SCORECARD")
    print(f"cache reads per call, all        {per_call(reads, calls)}")
    print(f"cache reads per call, main       {per_call(r.reads['main'], r.calls['main'])}")
    print(f"cache reads per call, subagents  {per_call(sub, r.calls['subagent'])}")
    print(f"opus share of subagent reads     {pct(r.model_reads['subagent']['opus'], sub):.0f}%")
    explore = {m: (r.type_agents[(t, m)], v) for (t, m), v in r.type_reads.items() if t == "Explore"}
    for model, (agents, value) in sorted(explore.items()):
        print(f"Explore on {model:<21} {agents} agents, {pct(value, reads):.1f}% of all reads")
    if not explore:
        print("Explore                          no agents")
    print(f"sed + cat share of tool output   {pct(r.bash['sed'] + r.bash['cat'], output):.0f}%")
    print(f"grep share of tool output        {pct(r.bash['grep'], output):.0f}%")
    print(f"Read share of tool output        {pct(r.tools['Read'], output):.1f}%")
    top = sorted(r.workers, reverse=True)[:50]
    if top:
        print(f"median peak context, top workers {statistics.median(p for _, p in top) / 1000:.0f}k (top {len(top)}: {pct(sum(x for x, _ in top), sub):.0f}% of subagent reads)")
    print(f"guard denials, of Read + Bash    {r.denials} ({pct(r.denials, len(r.calls_log)):.1f}%)")
    print()
    print("tool output by tool")
    for name, value in r.tools.most_common(5):
        print(f"  {name:<12} {pct(value, output):5.1f}%  calls={r.tool_calls[name]}")
    print("Bash output by command (shares of Bash)")
    total_bash = sum(r.bash.values())
    for name, value in r.bash.most_common(10):
        print(f"  {name:<12} {pct(value, total_bash):5.1f}%")


def load_guard():
    sys.dont_write_bytecode = True
    path = os.path.join(REPO, ".agents", "hooks", "read_guard.py")
    spec = importlib.util.spec_from_file_location("read_guard", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def print_replay(r, sample):
    """Runs today's guard over past calls. File sizes are today's, not the originals."""
    guard = load_guard()
    denied = collections.Counter()
    chars = collections.Counter()
    examples = []
    for name, tool_input, size in r.calls_log:
        reason = guard.check(name, tool_input, REPO, REPO)
        if reason:
            denied[name] += 1
            chars[name] += size
            if name == "Bash":
                examples.append(tool_input.get("command", ""))
    output = sum(r.tools.values())
    print()
    print("REPLAY")
    for name in ("Read", "Bash"):
        print(f"{name}: {denied[name]} of {r.tool_calls[name]} calls denied, {pct(chars[name], output):.1f}% of tool output")
    step = max(1, len(examples) // sample) if sample else 0
    for command in examples[::step][:sample] if step else []:
        print("  " + command[:150].replace("\n", " "))


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--since", help="only transcripts modified on or after this date (YYYY-MM-DD)")
    parser.add_argument("--last", type=int, help="only the N most recently modified transcripts")
    parser.add_argument("--replay", action="store_true", help="dry-run the read guard over past calls")
    parser.add_argument("--sample", type=int, default=40, help="denied commands to print with --replay")
    args = parser.parse_args()
    files = select_files(transcripts_dir(), args.since, args.last)
    report = Report()
    for path in files:
        report.add_file(path)
    print_scorecard(report, len(files))
    if args.replay:
        print_replay(report, args.sample)


if __name__ == "__main__":
    main()
