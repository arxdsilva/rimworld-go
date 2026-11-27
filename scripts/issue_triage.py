#!/usr/bin/env python3
"""Lint open issues against .agents/templates/ISSUE.md and suggest label fixes.

    python3 scripts/issue_triage.py lint    [--issue N] [--repo R] [--input FILE]
    python3 scripts/issue_triage.py suggest [--issue N] [--repo R] [--input FILE] [--apply]

Data comes from `gh issue list --json number,title,body,labels,createdAt`, or from
--input (same JSON shape; no network). In --input mode a tracker's sub-issue
counts come from the optional fixture fields `subIssues` (total) and
`subIssuesOpen` (default: all open). Output is sorted and deterministic; the
only date used is CUTOFF. Stdlib only, Python 3.9+.
"""
import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
from collections import namedtuple

DEFAULT_REPO = "arxdsilva/rimworld-go"
# Hand-filed issues created before this date predate the eight-section template
# and the title rule: they are grandfathered from L5-L7 (loops-finding issues are not).
CUTOFF = "2026-11-28"
FIELDS = "number,title,body,labels,createdAt"

TYPES = ("bug", "debt", "flaky", "spec-gap", "infra")
SIZES = ("XS", "S", "M", "L", "XL")
HEADINGS = ("Summary", "Where", "Evidence", "Expected vs actual", "Impact",
            "Suggested fix", "Acceptance criteria", "Refs")

TITLE_RE = re.compile(r"^[a-z0-9][a-z0-9/_.-]*: \S")
IMPACT_RE = re.compile(r"Priority: P([0-3])\b.*· Size: (XS|S|M|L|XL)\b")
ITEM_RE = re.compile(r"^\s*[-*] \[( |x|X)\] (.*)$")
MARKER_RE = re.compile(r"\([^()]*?,\s*P([0-3])\)")
DUP_RE = re.compile(r"Possible duplicate of #(\d+)")

Suggestion = namedtuple("Suggestion", "number field current proposed rule applicable remove",
                        defaults=((),))


def run_gh(cmd):
    """Run a gh command and return stdout. Tests replace this."""
    return subprocess.run(cmd, check=True, capture_output=True, text=True).stdout


# ---- label helpers ---------------------------------------------------------

def label_names(issue):
    out = []
    for lb in issue.get("labels") or []:
        out.append(lb["name"] if isinstance(lb, dict) else str(lb))
    return out


def with_prefix(names, prefix):
    return sorted(n[len(prefix):] for n in names if n.startswith(prefix))


def priorities(issue):
    return with_prefix(label_names(issue), "priority: ")


def sizes(issue):
    return with_prefix(label_names(issue), "size: ")


# ---- lint ------------------------------------------------------------------

def h2_headings(body):
    return [m.group(1).strip() for m in re.finditer(r"^## (.+?)\s*$", body or "", re.M)]


def impact_line(body):
    lines = (body or "").split("\n")
    for i, ln in enumerate(lines):
        if ln.strip() == "## Impact":
            for nxt in lines[i + 1:]:
                if nxt.strip():
                    return nxt
            return None
    return None


def body_rules_apply(issue, names):
    if "spec-backlog" in names or "tracker" in names:
        return False
    return "loops-finding" in names or (issue.get("createdAt") or "")[:10] >= CUTOFF


def lint_issue(issue, sub_issues=None):
    """Return [(rule, message)] sorted by rule then message."""
    names = label_names(issue)
    out = []
    types = [n for n in names if n in TYPES]
    if len(types) != 1:
        out.append(("L1", "expected exactly one type label, found %d (%s)"
                    % (len(types), ", ".join(types) or "none")))
    prios = priorities(issue)
    if len(prios) != 1:
        out.append(("L2", "expected exactly one priority label, found %d" % len(prios)))
    szs = sizes(issue)
    if len(szs) != 1:
        out.append(("L3", "expected exactly one size label, found %d" % len(szs)))
    tracker = "tracker" in names
    if "XL" in szs and not tracker:
        out.append(("L4", "size: XL requires the tracker label (split it into sub-issues)"))
    if tracker and "XL" not in szs:
        out.append(("L4", "tracker requires size: XL"))
    if tracker and sub_issues is not None and sub_issues < 1:
        out.append(("L4", "tracker requires at least one sub-issue"))
    if body_rules_apply(issue, names):
        out += lint_body(issue, prios, szs)
    return sorted(out)


def lint_body(issue, prios, szs):
    out = []
    title = issue.get("title") or ""
    if not TITLE_RE.match(title) or len(title) > 80:
        out.append(("L5", "title must match '<area>: <symptom>' (lower-case area) and be at most 80 characters"))
    body = issue.get("body") or ""
    found = [h for h in h2_headings(body) if h in HEADINGS]
    for h in HEADINGS:
        if h not in found:
            out.append(("L6", "missing heading '## %s'" % h))
    if found != [h for h in HEADINGS if h in found]:
        out.append(("L6", "headings are out of order or repeated (expected %s)" % ", ".join(HEADINGS)))
    line = impact_line(body)
    m = IMPACT_RE.search(line) if line else None
    if not m:
        out.append(("L7", "first '## Impact' line must read 'Priority: P<n> (...) · Size: <XS..XL>'"))
    else:
        if prios and prios != ["P" + m.group(1)]:
            out.append(("L7", "Impact says P%s but the label is %s" % (m.group(1), ",".join(prios))))
        if szs and szs != [m.group(2)]:
            out.append(("L7", "Impact says size %s but the label is %s" % (m.group(2), ",".join(szs))))
    return out


# ---- suggest ---------------------------------------------------------------

def unchecked_items(body):
    items = []
    for ln in (body or "").split("\n"):
        m = ITEM_RE.match(ln)
        if m and m.group(1) == " ":
            items.append(ln)
    return items


def size_for_count(n, title):
    if n >= 10:
        return "XL"
    if n >= 4:
        return "L"
    if n >= 2:
        return "M"
    return "S" if re.match(r"(UI|Art|Screens):", title) else "M"


def cur(values):
    return ",".join(values) or "none"


def refs_text(body):
    m = re.search(r"^## Refs\s*$", body or "", re.M)
    return (body or "")[m.end():] if m else ""


def is_older(a, b):
    return (a.get("createdAt") or "", a["number"]) < (b.get("createdAt") or "", b["number"])


def suggest_issue(issue, pool):
    """Rule-based proposals for one issue. `pool` maps number -> open issue."""
    n = issue["number"]
    names = label_names(issue)
    title = issue.get("title") or ""
    prios, szs = priorities(issue), sizes(issue)
    out = []
    eff_prio = prios[0] if len(prios) == 1 else None
    if "spec-backlog" in names and "tracker" not in names:
        items = unchecked_items(issue.get("body"))
        marks = [int(m.group(1)) for ln in items for m in [MARKER_RE.search(ln)] if m]
        if marks:
            want = "P%d" % min(marks)
            if prios != [want]:
                out.append(Suggestion(n, "priority", cur(prios), want, "R1", True,
                                      tuple("priority: " + p for p in prios)))
            eff_prio = want
        if items:
            want = size_for_count(len(items), title)
            if szs != [want]:
                out.append(Suggestion(n, "size", cur(szs), want, "R1", True,
                                      tuple("size: " + s for s in szs)))
    if title.startswith("loops:") and eff_prio in ("P0", "P1"):
        out = [s for s in out if s.field != "priority"]
        out.append(Suggestion(n, "priority", cur(prios), "P2", "R3", True,
                              tuple("priority: " + p for p in prios)))
    m = DUP_RE.search(refs_text(issue.get("body")))
    if m:
        other = pool.get(int(m.group(1)))
        if other is not None and other["number"] != n and is_older(other, issue):
            out.append(Suggestion(n, "state", "open", "close as duplicate of #%d" % other["number"], "R2", False))
    if "tracker" in names:
        total = issue.get("subIssues")
        if total is not None and total >= 1 and issue.get("subIssuesOpen", total) == 0:
            out.append(Suggestion(n, "state", "open", "close tracker", "R4", False))
    return sorted(out, key=lambda s: (s.field, s.rule))


def format_suggestion(s):
    return "#%d %s: %s -> %s (%s)" % (s.number, s.field, s.current, s.proposed, s.rule)


# ---- data ------------------------------------------------------------------

def load(args):
    if args.input:
        with open(args.input, encoding="utf-8") as f:
            return json.load(f)
    return json.loads(run_gh(["gh", "issue", "list", "--repo", args.repo, "--state", "open",
                              "--limit", "2000", "--json", FIELDS]))


def attach_sub_issues(issues, args):
    """Live mode: count sub-issues for trackers only."""
    if args.input:
        return
    for i in issues:
        if "tracker" in label_names(i) and "subIssues" not in i:
            out = run_gh(["gh", "api", "repos/%s/issues/%d/sub_issues" % (args.repo, i["number"]),
                          "--paginate", "--jq", ".[].state"])
            states = [s.strip().lower() for s in out.split("\n") if s.strip()]
            i["subIssues"] = len(states)
            i["subIssuesOpen"] = states.count("open")


def select(issues, number):
    if number is None:
        return sorted(issues, key=lambda i: i["number"])
    return [i for i in issues if i["number"] == number]


# ---- commands --------------------------------------------------------------

def cmd_lint(args):
    if args.issue is not None and not args.input:
        one = json.loads(run_gh(["gh", "issue", "view", str(args.issue), "--repo", args.repo,
                                 "--json", FIELDS + ",state"]))
        if str(one.get("state", "OPEN")).upper() != "OPEN":
            return 0
        issues = [one]
    else:
        issues = select(load(args), args.issue)
    attach_sub_issues(issues, args)
    lines = []
    for i in issues:
        for rule, msg in lint_issue(i, i.get("subIssues")):
            lines.append((i["number"], rule, msg))
    for n, rule, msg in sorted(lines):
        print("#%d %s: %s" % (n, rule, msg))
    return 1 if lines else 0


def apply_suggestion(s, args):
    cmd = ["gh", "issue", "edit", str(s.number), "--repo", args.repo]
    for name in s.remove:
        cmd += ["--remove-label", name]
    cmd += ["--add-label", "%s: %s" % (s.field, s.proposed)]
    run_gh(cmd)
    if s.field == "priority":
        fd, path = tempfile.mkstemp(suffix=".md")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as f:
                f.write("Priority %s → %s: %s" % (s.current, s.proposed, s.rule))
            run_gh(["gh", "issue", "comment", str(s.number), "--repo", args.repo, "--body-file", path])
        finally:
            os.unlink(path)


def cmd_suggest(args):
    issues = load(args)
    attach_sub_issues([i for i in select(issues, args.issue)], args)
    pool = {i["number"]: i for i in issues}
    found = []
    for i in select(issues, args.issue):
        found += suggest_issue(i, pool)
    found.sort(key=lambda s: (s.number, s.field, s.rule))
    for s in found:
        print(format_suggestion(s))
        if args.apply and s.applicable:
            apply_suggestion(s, args)
    return 0


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    sub = p.add_subparsers(dest="cmd", required=True)
    for name in ("lint", "suggest"):
        sp = sub.add_parser(name)
        sp.add_argument("--issue", type=int)
        sp.add_argument("--repo", default=DEFAULT_REPO)
        sp.add_argument("--input", help="JSON file shaped like `gh issue list --json` (no network)")
        if name == "suggest":
            sp.add_argument("--apply", action="store_true",
                            help="apply R1/R3 label changes (never closes, never edits bodies or titles)")
    args = p.parse_args(argv)
    if getattr(args, "apply", False) and args.input:
        p.error("--apply cannot be combined with --input")
    return cmd_lint(args) if args.cmd == "lint" else cmd_suggest(args)


if __name__ == "__main__":
    sys.exit(main())
