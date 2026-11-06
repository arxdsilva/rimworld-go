#!/usr/bin/env python3
"""Tests for the PreToolUse hooks: python3 .agents/hooks/test_hooks.py"""
import json
import os
import subprocess
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

sys.dont_write_bytecode = True

import agent_model_guard  # noqa: E402
import read_guard  # noqa: E402


class RepoCase(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = os.path.realpath(tmp.name)
        os.makedirs(os.path.join(self.repo, "pkg"))
        self.write("pkg/small.go", 10)
        self.write("pkg/big.go", 600)
        self.write("pkg/other.go", 10)
        self.write("pkg/medium.go", 100)
        self.write("pic.png", 600)

    def write(self, name, lines):
        with open(os.path.join(self.repo, name), "w") as f:
            f.write("x\n" * lines)

    def bash(self, command, cwd=None):
        return read_guard.check("Bash", {"command": command}, cwd or self.repo, self.repo)

    def read(self, name, **extra):
        path = os.path.join(self.repo, name) if not name.startswith("/") else name
        return read_guard.check("Read", dict(file_path=path, **extra), self.repo, self.repo)


class ReadTests(RepoCase):
    def test_whole_big_file_is_denied(self):
        self.assertIn("600 lines", self.read("pkg/big.go"))

    def test_ranged_read_of_big_file_passes(self):
        self.assertIsNone(self.read("pkg/big.go", offset=1, limit=50))
        self.assertIsNone(self.read("pkg/big.go", limit=50))

    def test_small_file_passes(self):
        self.assertIsNone(self.read("pkg/small.go"))

    def test_binary_and_outside_paths_pass(self):
        self.assertIsNone(self.read("pic.png"))
        self.assertIsNone(self.read("/tmp/elsewhere.txt"))
        self.assertIsNone(self.read("pkg/missing.go"))


class BashTests(RepoCase):
    def assertDenied(self, command, cwd=None):
        self.assertIsNotNone(self.bash(command, cwd), command)

    def assertAllowed(self, command, cwd=None):
        self.assertIsNone(self.bash(command, cwd), command)

    def test_long_views_are_denied(self):
        for command in (
            "cat pkg/big.go",
            "cat -n pkg/big.go",
            "cat pkg/*.go",
            "sed -n '1,200p' pkg/big.go",
            "sed -n -e '1,200p' pkg/big.go",
            "sed -n '100,$p' pkg/big.go",
            "sed -n '1,+200p' pkg/big.go",
            "echo ---; cat pkg/big.go",
            "ls && sed -n 1,200p pkg/big.go",
            "(cat pkg/big.go)",
        ):
            self.assertDenied(command)

    def test_views_add_up_across_a_command(self):
        self.assertDenied("sed -n 1,100p pkg/big.go; sed -n 200,300p pkg/big.go")
        self.assertDenied("sed -n '1,100p;200,300p' pkg/big.go")
        self.assertDenied("cat pkg/medium.go; cat pkg/medium.go")

    def test_short_views_pass(self):
        for command in (
            "cat pkg/small.go",
            "cat pkg/small.go pkg/other.go",
            "cat pkg/medium.go",
            "sed -n '10,20p' pkg/big.go",
            "sed -n 1,150p pkg/big.go",
            "sed -n 5p pkg/big.go",
            "sed -n '/func/,/^}/p' pkg/big.go",
        ):
            self.assertAllowed(command)

    def test_cd_is_followed(self):
        self.assertDenied(f"cd {self.repo}/pkg && cat big.go")
        self.assertDenied(f"cd {self.repo}; cd pkg; cat big.go")
        self.assertDenied("cat ../pkg/big.go", cwd=os.path.join(self.repo, "pkg"))

    def test_pipes_that_do_not_shrink_are_denied(self):
        self.assertDenied("cat -n pkg/big.go | sed -n 1,400p")
        self.assertDenied("cat pkg/*.go | grep -v '^//'")

    def test_filters_pass(self):
        for command in (
            "cat pkg/big.go | grep foo",
            "cat pkg/big.go | head -20",
            "sed -n 1,500p pkg/big.go | grep -n foo",
            "cat pkg/big.go | wc -l",
            "grep -n foo pkg/big.go",
            "grep -rn foo pkg | head",
        ):
            self.assertAllowed(command)

    def test_writes_pass(self):
        for command in (
            "cat pkg/big.go > /tmp/copy.go",
            "sed -i '' 's/a/b/' pkg/big.go",
            "sed -i.bak -n 1,300p pkg/big.go",
            "cat > pkg/new.go <<'EOF'\ncat pkg/big.go\nEOF",
            "cat <<EOF\nsed -n 1,300p pkg/big.go\nEOF",
        ):
            self.assertAllowed(command)

    def test_unresolved_and_outside_paths_pass(self):
        for command in (
            "cat /tmp/elsewhere.txt",
            "cat pkg/missing.go",
            'cat "$F"',
            "cd $DIR && cat pkg/big.go",
            'for f in a b; do cat "$f"; done',
            "cd /tmp && cat pkg/big.go",
        ):
            self.assertAllowed(command)

    def test_quotes_and_comments_are_not_commands(self):
        self.assertAllowed("echo 'cat pkg/big.go'")
        self.assertAllowed("git log --grep='cat pkg/big.go; x'")
        self.assertAllowed("# cat pkg/big.go\nls")
        self.assertAllowed("python3 -c 'print(1)\ncat pkg/big.go'")

    def test_substitution_stays_allowed(self):
        self.assertAllowed('X="$(cat pkg/big.go | wc -l)"; echo "$X"')

    def test_bad_quoting_fails_open(self):
        self.assertAllowed("cat 'pkg/big.go")

    def test_other_tools_pass(self):
        self.assertIsNone(read_guard.check("Edit", {"file_path": "x"}, self.repo, self.repo))


class HookProcessTests(RepoCase):
    def run_hook(self, script, event, **env):
        return subprocess.run(
            [sys.executable, os.path.join(HERE, script)],
            input=json.dumps(event), capture_output=True, text=True,
            env=dict(os.environ, CLAUDE_PROJECT_DIR=self.repo, **env),
        )

    def bash_event(self):
        return {"tool_name": "Bash", "cwd": self.repo, "tool_input": {"command": "cat pkg/big.go"}}

    def test_denial_exits_2_with_reason(self):
        done = self.run_hook("read_guard.py", self.bash_event())
        self.assertEqual(done.returncode, 2)
        self.assertIn("read_guard:", done.stderr)

    def test_off_switch(self):
        self.assertEqual(self.run_hook("read_guard.py", self.bash_event(), READ_GUARD="off").returncode, 0)

    def test_view_limit_env(self):
        event = self.bash_event()
        self.assertEqual(self.run_hook("read_guard.py", event, READ_GUARD_MAX_VIEW_LINES="700").returncode, 0)

    def test_threshold_env(self):
        event = {"tool_name": "Read", "cwd": self.repo,
                 "tool_input": {"file_path": os.path.join(self.repo, "pkg/big.go")}}
        self.assertEqual(self.run_hook("read_guard.py", event).returncode, 2)
        self.assertEqual(self.run_hook("read_guard.py", event, READ_GUARD_MIN_LINES="700").returncode, 0)

    def test_bad_input_fails_open(self):
        done = subprocess.run([sys.executable, os.path.join(HERE, "read_guard.py")],
                              input="not json", capture_output=True, text=True)
        self.assertEqual(done.returncode, 0)

    def test_agent_guard_process(self):
        event = {"tool_name": "Agent", "tool_input": {"prompt": "x"}}
        done = self.run_hook("agent_model_guard.py", event)
        self.assertEqual(done.returncode, 2)
        self.assertIn("PARALLEL_WORK", done.stderr)


class AgentGuardTests(unittest.TestCase):
    def test_untyped_without_model_is_denied(self):
        self.assertIsNotNone(agent_model_guard.check({"prompt": "x"}))
        self.assertIsNotNone(agent_model_guard.check({"subagent_type": "general-purpose"}))

    def test_named_model_passes(self):
        self.assertIsNone(agent_model_guard.check({"model": "sonnet"}))
        self.assertIsNone(agent_model_guard.check({"subagent_type": "general-purpose", "model": "opus"}))

    def test_typed_agents_and_forks_pass(self):
        for kind in ("Explore", "Plan", "fork", "web-fetch"):
            self.assertIsNone(agent_model_guard.check({"subagent_type": kind}))


if __name__ == "__main__":
    unittest.main()
