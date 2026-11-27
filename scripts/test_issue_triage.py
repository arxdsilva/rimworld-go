#!/usr/bin/env python3
"""Unit tests for scripts/issue_triage.py. Run: python3 scripts/test_issue_triage.py"""
import contextlib
import io
import json
import os
import sys
import unittest
from unittest import mock

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import issue_triage as it  # noqa: E402

FIXTURE = os.path.join(HERE, "testdata", "issue_triage", "mixed.json")

GOOD_BODY = (
    "## Summary\ns\n## Where\nw\n## Evidence\ne\n## Expected vs actual\nx\n"
    "## Impact\nPriority: P2 (reason) · Size: S\nmore\n## Suggested fix\nf\n"
    "## Acceptance criteria\n- [ ] a\n## Refs\nn/a\n"
)


def issue(n=1, title="render: flicker", labels=("bug", "priority: P2", "size: S"),
          body=GOOD_BODY, created="2026-11-29T00:00:00Z", **extra):
    d = {"number": n, "title": title, "body": body, "createdAt": created,
         "labels": [{"name": x} for x in labels]}
    d.update(extra)
    return d


def rules(problems):
    return sorted(r for r, _ in problems)


class LintTests(unittest.TestCase):
    def test_clean_issue(self):
        self.assertEqual(it.lint_issue(issue()), [])

    def test_l1_type(self):
        self.assertEqual(rules(it.lint_issue(issue(labels=("priority: P2", "size: S")))), ["L1"])
        self.assertEqual(rules(it.lint_issue(issue(labels=("bug", "debt", "priority: P2", "size: S")))), ["L1"])

    def test_l2_priority(self):
        self.assertEqual(rules(it.lint_issue(issue(labels=("bug", "size: S")))), ["L2"])
        two = it.lint_issue(issue(labels=("bug", "priority: P1", "priority: P2", "size: S")))
        self.assertIn("L2", rules(two))

    def test_l3_size(self):
        self.assertEqual(rules(it.lint_issue(issue(labels=("bug", "priority: P2")))), ["L3"])
        self.assertIn("L3", rules(it.lint_issue(issue(labels=("bug", "priority: P2", "size: S", "size: M")))))

    def test_l4_xl_needs_tracker(self):
        p = it.lint_issue(issue(labels=("bug", "priority: P2", "size: XL"), body=GOOD_BODY.replace("Size: S", "Size: XL")))
        self.assertEqual(rules(p), ["L4"])

    def test_l4_tracker_needs_xl(self):
        p = it.lint_issue(issue(labels=("bug", "tracker", "priority: P2", "size: M")), sub_issues=1)
        self.assertEqual(rules(p), ["L4"])

    def test_l4_tracker_needs_sub_issue(self):
        labels = ("bug", "tracker", "priority: P2", "size: XL")
        self.assertEqual(rules(it.lint_issue(issue(labels=labels), sub_issues=0)), ["L4"])
        self.assertEqual(it.lint_issue(issue(labels=labels), sub_issues=1), [])
        # unknown count (None) is not a violation
        self.assertEqual(it.lint_issue(issue(labels=labels), sub_issues=None), [])

    def test_l5_title(self):
        for bad in ["Render: x", "no colon here", "render:x", "a" * 70 + ": " + "b" * 20]:
            self.assertEqual(rules(it.lint_issue(issue(title=bad))), ["L5"], bad)
        self.assertEqual(it.lint_issue(issue(title="a/b_c.d-e: ok")), [])
        self.assertEqual(it.lint_issue(issue(title="x: " + "y" * 77)), [])
        self.assertEqual(rules(it.lint_issue(issue(title="x: " + "y" * 78))), ["L5"])

    def test_l6_headings(self):
        p = it.lint_issue(issue(body=GOOD_BODY.replace("## Where\nw\n", "")))
        self.assertEqual(rules(p), ["L6"])
        self.assertIn("Where", p[0][1])

    def test_l6_order(self):
        swapped = GOOD_BODY.replace("## Summary\ns\n## Where\nw\n", "## Where\nw\n## Summary\ns\n")
        self.assertEqual(rules(it.lint_issue(issue(body=swapped))), ["L6"])

    def test_l6_empty_body(self):
        self.assertEqual(rules(it.lint_issue(issue(body=""))).count("L6"), 8)
        self.assertIn("L6", rules(it.lint_issue(issue(body=None))))

    def test_l7_impact_line(self):
        for bad in ["Priority: P2 only", "nothing here", "Size: S · Priority: P2"]:
            body = GOOD_BODY.replace("Priority: P2 (reason) · Size: S", bad)
            self.assertEqual(rules(it.lint_issue(issue(body=body))), ["L7"], bad)

    def test_l7_agrees_with_labels(self):
        body = GOOD_BODY.replace("Priority: P2", "Priority: P1")
        self.assertEqual(rules(it.lint_issue(issue(body=body))), ["L7"])
        body = GOOD_BODY.replace("Size: S", "Size: M")
        self.assertEqual(rules(it.lint_issue(issue(body=body))), ["L7"])

    def test_l7_only_first_impact_line(self):
        body = GOOD_BODY.replace("## Impact\n", "## Impact\n\n")
        self.assertEqual(it.lint_issue(issue(body=body)), [])
        body = GOOD_BODY.replace("Priority: P2 (reason) · Size: S\nmore", "intro\nPriority: P2 (r) · Size: S")
        self.assertEqual(rules(it.lint_issue(issue(body=body))), ["L7"])

    def test_cutoff_grandfathers_old_issues(self):
        old = issue(title="Bad", body="free", created="2026-11-27T23:59:59Z")
        self.assertEqual(it.lint_issue(old), [])
        new = issue(title="Bad", body="free", created="2026-11-28T00:00:00Z")
        self.assertIn("L5", rules(it.lint_issue(new)))

    def test_cutoff_constant(self):
        self.assertEqual(it.CUTOFF, "2026-11-28")

    def test_loops_finding_old_still_checked(self):
        old = issue(title="Bad", body="free", created="2026-11-01T00:00:00Z",
                    labels=("bug", "priority: P2", "size: S", "loops-finding"))
        self.assertIn("L5", rules(it.lint_issue(old)))

    def test_spec_backlog_and_tracker_skip_body_rules(self):
        for extra in ("spec-backlog", "tracker"):
            labels = ["bug", "priority: P2", "size: XL", extra]
            if extra == "spec-backlog":
                labels = ["bug", "priority: P2", "size: S", extra]
            i = issue(title="Bad", body="free", labels=labels, created="2026-12-01T00:00:00Z")
            got = rules(it.lint_issue(i, sub_issues=1))
            self.assertNotIn("L5", got)
            self.assertNotIn("L6", got)
            self.assertNotIn("L7", got)

    def test_problems_sorted_by_rule(self):
        i = issue(title="Bad", labels=("priority: P2",), body="x")
        got = [r for r, _ in it.lint_issue(i)]
        self.assertEqual(got, sorted(got))


class SuggestTests(unittest.TestCase):
    def sb(self, body, title="render: things", labels=("spec-gap", "spec-backlog", "priority: P3", "size: S")):
        return issue(7, title=title, body=body, labels=labels)

    def run_suggest(self, i, others=()):
        pool = {x["number"]: x for x in [i, *others]}
        return it.suggest_issue(i, pool)

    def test_r1_priority_highest_unchecked(self):
        body = "- [ ] **a** (open, P2) x\n- [x] **b** (done, P0) x\n- [ ] **c** (open, P1) x\n- [ ] **d** (open, P3) x\n"
        got = {(s.field, s.proposed): s for s in self.run_suggest(self.sb(body))}
        self.assertIn(("priority", "P1"), got)
        self.assertEqual(got[("priority", "P1")].rule, "R1")
        self.assertEqual(got[("priority", "P1")].current, "P3")

    def test_r1_no_change_when_equal(self):
        body = "- [ ] **a** (open, P3) x\n- [ ] **b** (open, P3) x\n- [ ] **c** (open, P3) x\n"
        got = [s for s in self.run_suggest(self.sb(body)) if s.field == "priority"]
        self.assertEqual(got, [])

    def test_r1_size_buckets(self):
        def size_for(n, title="render: x"):
            body = "".join("- [ ] **i%d** (open, P3) x\n" % k for k in range(n))
            labels = ("spec-gap", "spec-backlog", "priority: P3", "size: XS")
            got = [s for s in self.run_suggest(self.sb(body, title, labels)) if s.field == "size"]
            return got[0].proposed if got else None
        self.assertEqual(size_for(1), "M")
        self.assertEqual(size_for(1, "UI: x"), "S")
        self.assertEqual(size_for(1, "Art: x"), "S")
        self.assertEqual(size_for(1, "Screens: x"), "S")
        self.assertEqual(size_for(2), "M")
        self.assertEqual(size_for(3), "M")
        self.assertEqual(size_for(4), "L")
        self.assertEqual(size_for(9), "L")
        self.assertEqual(size_for(10), "XL")
        self.assertEqual(size_for(25), "XL")

    def test_r1_ignores_markerless_and_checked(self):
        body = "- [x] **a** (done, P0) x\n- [ ] **b** no marker\n"
        got = [s for s in self.run_suggest(self.sb(body)) if s.field == "priority"]
        self.assertEqual(got, [])

    def test_r1_skips_tracker(self):
        body = "- [ ] **a** (open, P0) x\n"
        i = self.sb(body, labels=("spec-gap", "spec-backlog", "tracker", "priority: P3", "size: XL"))
        self.assertEqual([s for s in self.run_suggest(i) if s.rule == "R1"], [])

    def test_r1_skips_non_spec_backlog(self):
        i = self.sb("- [ ] **a** (open, P0) x\n", labels=("bug", "priority: P3", "size: S"))
        self.assertEqual(self.run_suggest(i), [])

    def test_r2_duplicate_of_older_open(self):
        older = issue(3, created="2026-11-20T00:00:00Z")
        dup = issue(9, body="## Refs\nPossible duplicate of #3\n", created="2026-11-25T00:00:00Z")
        got = self.run_suggest(dup, [older])
        self.assertEqual([(s.field, s.proposed, s.rule, s.applicable) for s in got],
                         [("state", "close as duplicate of #3", "R2", False)])

    def test_r2_not_when_newer_or_missing(self):
        newer = issue(3, created="2026-12-20T00:00:00Z")
        dup = issue(9, body="## Refs\nPossible duplicate of #3\n", created="2026-11-25T00:00:00Z")
        self.assertEqual(self.run_suggest(dup, [newer]), [])
        self.assertEqual(self.run_suggest(dup, []), [])

    def test_r3_loops_area(self):
        for pr in ("P0", "P1"):
            i = issue(4, title="loops: harness slow", labels=("infra", "priority: " + pr, "size: S"))
            got = self.run_suggest(i)
            self.assertEqual([(s.field, s.current, s.proposed, s.rule) for s in got],
                             [("priority", pr, "P2", "R3")])
        for pr in ("P2", "P3"):
            i = issue(4, title="loops: harness slow", labels=("infra", "priority: " + pr, "size: S"))
            self.assertEqual(self.run_suggest(i), [])
        i = issue(4, title="loop: other", labels=("infra", "priority: P0", "size: S"))
        self.assertEqual(self.run_suggest(i), [])

    def test_r4_tracker_all_closed(self):
        labels = ("infra", "tracker", "priority: P2", "size: XL")
        done = issue(5, labels=labels, subIssues=3, subIssuesOpen=0)
        got = self.run_suggest(done)
        self.assertEqual([(s.field, s.proposed, s.rule, s.applicable) for s in got],
                         [("state", "close tracker", "R4", False)])
        self.assertEqual(self.run_suggest(issue(5, labels=labels, subIssues=3, subIssuesOpen=1)), [])
        self.assertEqual(self.run_suggest(issue(5, labels=labels, subIssues=0, subIssuesOpen=0)), [])

    def test_format_line(self):
        s = it.Suggestion(7, "priority", "P1", "P2", "R3", True)
        self.assertEqual(it.format_suggestion(s), "#7 priority: P1 -> P2 (R3)")


class CliTests(unittest.TestCase):
    def run_cli(self, *argv):
        out = io.StringIO()
        with contextlib.redirect_stdout(out):
            code = it.main(list(argv))
        return code, out.getvalue()

    def test_lint_input_end_to_end(self):
        code, out = self.run_cli("lint", "--input", FIXTURE)
        self.assertEqual(code, 1)
        lines = out.splitlines()
        self.assertEqual(lines[0], "#2 L1: expected exactly one type label, found 2 (bug, debt)")
        self.assertTrue(all(l.startswith("#") for l in lines))
        keys = [(int(l.split()[0][1:]), l.split()[1]) for l in lines]
        self.assertEqual(keys, sorted(keys))
        self.assertEqual({k[0] for k in keys}, {2, 4})
        self.assertEqual(sorted({k[1] for k in keys if k[0] == 2}), ["L1:", "L2:", "L5:", "L6:", "L7:"])
        self.assertEqual([k[1] for k in keys if k[0] == 4], ["L4:"])

    def test_lint_deterministic(self):
        self.assertEqual(self.run_cli("lint", "--input", FIXTURE), self.run_cli("lint", "--input", FIXTURE))

    def test_lint_single_issue(self):
        code, out = self.run_cli("lint", "--input", FIXTURE, "--issue", "1")
        self.assertEqual((code, out), (0, ""))
        code, out = self.run_cli("lint", "--input", FIXTURE, "--issue", "3")
        self.assertEqual((code, out), (0, ""))
        code, out = self.run_cli("lint", "--input", FIXTURE, "--issue", "4")
        self.assertEqual(code, 1)
        self.assertTrue(out.startswith("#4 L4:"))

    def test_lint_clean_exit_zero(self):
        import tempfile
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
            json.dump([issue()], f)
        try:
            self.assertEqual(self.run_cli("lint", "--input", f.name), (0, ""))
        finally:
            os.unlink(f.name)

    def test_suggest_input_end_to_end(self):
        code, out = self.run_cli("suggest", "--input", FIXTURE)
        self.assertEqual(code, 0)
        self.assertEqual(out.splitlines(), [
            "#4 priority: P3 -> P1 (R1)",
            "#4 size: XL -> M (R1)",
            "#5 state: open -> close as duplicate of #3 (R2)",
            "#6 state: open -> close tracker (R4)",
        ])

    def test_suggest_single_issue(self):
        code, out = self.run_cli("suggest", "--input", FIXTURE, "--issue", "5")
        self.assertEqual(out.splitlines(), ["#5 state: open -> close as duplicate of #3 (R2)"])

    def test_apply_rejected_with_input(self):
        with self.assertRaises(SystemExit):
            self.run_cli("suggest", "--input", FIXTURE, "--apply")

    def test_no_network_in_input_mode(self):
        with mock.patch.object(it, "run_gh", side_effect=AssertionError("network")):
            self.run_cli("lint", "--input", FIXTURE)
            self.run_cli("suggest", "--input", FIXTURE)

    def test_apply_edits_labels_and_comments(self):
        calls, bodies = [], []

        def fake(cmd):
            calls.append(list(cmd))
            if "--body-file" in cmd:
                with open(cmd[cmd.index("--body-file") + 1], encoding="utf-8") as f:
                    bodies.append(f.read())
            if cmd[:3] == ["gh", "issue", "list"]:
                with open(FIXTURE, encoding="utf-8") as f:
                    return f.read()
            return ""

        with mock.patch.object(it, "run_gh", side_effect=fake):
            out = io.StringIO()
            with contextlib.redirect_stdout(out):
                code = it.main(["suggest", "--apply", "--repo", "o/r"])
        self.assertEqual(code, 0)
        edits = [c for c in calls if c[:3] == ["gh", "issue", "edit"]]
        self.assertEqual(len(edits), 2)
        self.assertIn("--remove-label", edits[0])
        self.assertIn("priority: P3", edits[0])
        self.assertIn("priority: P1", edits[0])
        self.assertIn("size: M", edits[1])
        self.assertEqual(bodies, ["Priority P3 → P1: R1"])
        self.assertTrue(all(c[c.index("--repo") + 1] == "o/r" for c in edits))
        # never closes or edits titles/bodies
        flat = [a for c in calls for a in c]
        for forbidden in ("close", "--title", "--body"):
            self.assertNotIn(forbidden, flat)

    def test_apply_does_not_comment_on_size_only(self):
        calls = []

        def fake(cmd):
            calls.append(list(cmd))
            return json.dumps([issue(7, title="UI: x", labels=("spec-gap", "spec-backlog", "priority: P3", "size: XL"),
                                     body="- [ ] **a** (open, P3) x\n")]) if cmd[:3] == ["gh", "issue", "list"] else ""

        with mock.patch.object(it, "run_gh", side_effect=fake), contextlib.redirect_stdout(io.StringIO()):
            it.main(["suggest", "--apply"])
        self.assertFalse([c for c in calls if c[:3] == ["gh", "issue", "comment"]])
        self.assertEqual(len([c for c in calls if c[:3] == ["gh", "issue", "edit"]]), 1)

    def test_live_fetch_command_and_subissues(self):
        calls = []
        data = [issue(6, labels=("infra", "tracker", "priority: P2", "size: XL"), body="t")]

        def fake(cmd):
            calls.append(list(cmd))
            if cmd[:3] == ["gh", "issue", "list"]:
                return json.dumps(data)
            if cmd[:2] == ["gh", "api"]:
                return "open\nclosed\n"
            return ""

        with mock.patch.object(it, "run_gh", side_effect=fake), contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(it.main(["lint"]), 0)
        lst = calls[0]
        self.assertEqual(lst[:4], ["gh", "issue", "list", "--repo"])
        self.assertIn("arxdsilva/rimworld-go", lst)
        self.assertIn("2000", lst)
        self.assertIn("number,title,body,labels,createdAt", lst)
        api = [c for c in calls if c[:2] == ["gh", "api"]]
        self.assertEqual(len(api), 1)
        self.assertIn("repos/arxdsilva/rimworld-go/issues/6/sub_issues", api[0])

    def test_lint_single_live_skips_closed(self):
        def fake(cmd):
            self.assertEqual(cmd[:3], ["gh", "issue", "view"])
            return json.dumps(dict(issue(), state="CLOSED", title="Bad"))
        with mock.patch.object(it, "run_gh", side_effect=fake), contextlib.redirect_stdout(io.StringIO()) as o:
            self.assertEqual(it.main(["lint", "--issue", "1"]), 0)
        self.assertEqual(o.getvalue(), "")


if __name__ == "__main__":
    unittest.main()
