#!/usr/bin/env python3
"""PreToolUse hook: long file views go through ranged Read, not shell dumps or whole files.

Small shell views stay allowed: through Read they would print the same text, so
denying them only adds a call, and every call re-reads the whole context. Replay the
rule with scripts/token_report.py --replay before changing it. Fails open: anything it
cannot parse or resolve is allowed.

READ_GUARD=off disables it. READ_GUARD_MIN_LINES sets the whole-file Read limit and
READ_GUARD_MAX_VIEW_LINES the lines one shell command may print from repo files.
See .cursor/AI_TOKEN_OPTIMIZATION.md.
"""
import glob
import json
import os
import re
import sys

DEFAULT_MIN_LINES = 500
DEFAULT_MAX_VIEW_LINES = 150
BINARY_EXT = {".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".ipynb", ".wav", ".ogg"}
# A later pipeline stage from this set does not shrink what the viewer printed.
PASS_THROUGH = {"cat", "sed", "nl", "tr", "cut", "less", "more", "tee", "expand"}
LEADERS = {"do", "then", "else", "elif", "if", "while", "{", "!", "time"}
OPERATORS = ("&&", "||", ";;", ">>", ";", "|", "&", "\n", "(", ")", ">")
HEREDOC = re.compile(r"<<-?\s*(['\"]?)(\w+)\1[^\n]*\n.*?\n\s*\2[ \t]*(?=\n|$)", re.S)
ENV_PREFIX = re.compile(r"^[A-Za-z_]\w*=")
SED_RANGE = re.compile(r"(\d+)\s*(?:,\s*(\+?\d+|\$))?\s*p")

READ_MSG = ("read_guard: {path} has {lines} lines. Use Read with offset and limit, "
            "after grep -n finds the range, or use the Explore agent for a broad question.")
BASH_MSG = ("read_guard: this command prints about {lines} lines of repo files ({path}). Use "
            "Read with offset and limit for the part you need, grep -n to find it, or the "
            "Explore agent for a broad question. Several Reads can go in one message.")


def tokenize(command):
    """Splits a shell command into words and operators, honouring quotes."""
    tokens, word, quote, i, has_word = [], [], None, 0, False
    n = len(command)
    while i < n:
        ch = command[i]
        if quote:
            if ch == quote:
                quote = None
            elif ch == "\\" and quote == '"' and i + 1 < n:
                i += 1
                word.append(command[i])
            else:
                word.append(ch)
            i += 1
            continue
        if ch in "'\"":
            quote, has_word = ch, True
            i += 1
            continue
        if ch == "\\" and i + 1 < n:
            if command[i + 1] != "\n":
                word.append(command[i + 1])
                has_word = True
            i += 2
            continue
        if ch == "#" and not has_word:
            end = command.find("\n", i)
            i = n if end < 0 else end
            continue
        op = next((o for o in OPERATORS if command.startswith(o, i)), None)
        if op or ch in " \t\r":
            if has_word:
                tokens.append("".join(word))
                word, has_word = [], False
            if op:
                tokens.append(("op", op))
                i += len(op)
            else:
                i += 1
            continue
        word.append(ch)
        has_word = True
        i += 1
    if quote:
        raise ValueError("unterminated quote")
    if has_word:
        tokens.append("".join(word))
    return tokens


def pipelines(tokens):
    """Groups tokens into pipelines of stages, marking redirected pipelines."""
    result, stages, stage, redirected = [], [], [], False

    def end_stage():
        nonlocal stage
        if stage:
            stages.append(stage)
        stage = []

    def end_pipeline():
        nonlocal stages, redirected
        end_stage()
        if stages:
            result.append((stages, redirected))
        stages, redirected = [], False

    for token in tokens:
        if isinstance(token, tuple):
            if token[1] == "|":
                end_stage()
            elif token[1] in (">", ">>"):
                redirected = True
            else:
                end_pipeline()
        else:
            stage.append(token)
    end_pipeline()
    return result


def command_words(stage):
    words = list(stage)
    while words and (words[0] in LEADERS or ENV_PREFIX.match(words[0])):
        words.pop(0)
    return words


def count_lines(path):
    with open(path, "rb") as f:
        return sum(chunk.count(b"\n") for chunk in iter(lambda: f.read(1 << 16), b""))


def repo_files(path, base, repo):
    """Returns the existing repo files a path or glob names."""
    if "$" in path or "`" in path:
        return []
    full = os.path.normpath(os.path.join(base, os.path.expanduser(path)))
    matches = glob.glob(full) if any(c in full for c in "*?[") else [full]
    root = os.path.normpath(repo) + os.sep
    return [m for m in matches if m.startswith(root) and os.path.isfile(m)]


def parse_sed(args):
    """Returns (script, paths) for a printing sed -n, or None for anything else."""
    flags = [a for a in args if a.startswith("-") and not a.startswith("--")]
    if not any("n" in f[1:] for f in flags) or any("i" in f[1:] or "f" in f[1:] for f in flags):
        return None
    scripts, rest, take_script = [], [], False
    for arg in args:
        if take_script:
            scripts.append(arg)
            take_script = False
        elif arg.startswith("-"):
            take_script = "e" in arg[1:]
        else:
            rest.append(arg)
    if not scripts and rest:
        scripts, rest = [rest[0]], rest[1:]
    return " ".join(scripts), rest


def sed_lines(script, total):
    """Counts the lines numeric ranges print. Pattern ranges are unknown and count as 0."""
    lines = 0
    for start, end in SED_RANGE.findall(script):
        first = int(start)
        if not end:
            lines += 1
        elif end == "$":
            lines += max(total - first + 1, 0)
        elif end.startswith("+"):
            lines += int(end) + 1
        else:
            lines += max(int(end) - first + 1, 0)
    return min(lines, total)


def view(words, base, repo):
    """Returns (lines printed, first repo file) for a cat or sed -n view."""
    if words[0] == "cat":
        script, paths = None, [a for a in words[1:] if not a.startswith("-")]
    else:
        parsed = parse_sed(words[1:])
        if not parsed:
            return 0, None
        script, paths = parsed
    files = [f for p in paths for f in repo_files(p, base, repo)]
    if not files:
        return 0, None
    total = sum(count_lines(f) for f in files)
    return (total if script is None else sed_lines(script, total)), os.path.relpath(files[0], repo)


def shrinks_output(stages):
    for stage in stages[1:]:
        words = command_words(stage)
        if not words or words[0] in PASS_THROUGH:
            continue
        if words[0] == "grep" and any(w.startswith("-") and "v" in w[1:] for w in words[1:]):
            continue
        return True
    return False


def next_dir(words, base):
    """Follows cd so later relative paths resolve. None means the target is unknown."""
    if len(words) < 2:
        return None
    target = words[1]
    if "$" in target or "`" in target or target == "-":
        return None
    return os.path.normpath(os.path.join(base, os.path.expanduser(target)))


def check_bash(command, cwd, repo, max_view):
    tokens = tokenize(HEREDOC.sub("\n", command))
    base = cwd
    lines, first = 0, None
    for stages, redirected in pipelines(tokens):
        words = command_words(stages[0])
        if not words:
            continue
        if words[0] == "cd":
            base = next_dir(words, base)
            if base is None:
                break
            continue
        if words[0] not in ("cat", "sed") or redirected or shrinks_output(stages):
            continue
        printed, name = view(words, base, repo)
        lines += printed
        first = first or name
    if lines > max_view:
        return BASH_MSG.format(lines=lines, path=first)
    return None


def check_read(tool_input, repo, min_lines):
    if tool_input.get("offset") or tool_input.get("limit"):
        return None
    path = tool_input.get("file_path") or ""
    if not path.startswith(os.path.normpath(repo) + os.sep):
        return None
    if os.path.splitext(path)[1].lower() in BINARY_EXT or not os.path.isfile(path):
        return None
    lines = count_lines(path)
    if lines <= min_lines:
        return None
    return READ_MSG.format(path=os.path.relpath(path, repo), lines=lines)


def check(tool_name, tool_input, cwd, repo, min_lines=DEFAULT_MIN_LINES, max_view=DEFAULT_MAX_VIEW_LINES):
    """Returns the reason to deny a call, or None to allow it."""
    try:
        if tool_name == "Read":
            return check_read(tool_input, repo, min_lines)
        if tool_name == "Bash":
            return check_bash(tool_input.get("command") or "", cwd, repo, max_view)
    except (OSError, ValueError):
        return None
    return None


def main():
    if os.environ.get("READ_GUARD") == "off":
        return 0
    try:
        event = json.load(sys.stdin)
        min_lines = int(os.environ.get("READ_GUARD_MIN_LINES") or DEFAULT_MIN_LINES)
        max_view = int(os.environ.get("READ_GUARD_MAX_VIEW_LINES") or DEFAULT_MAX_VIEW_LINES)
    except ValueError:
        return 0
    cwd = event.get("cwd") or os.getcwd()
    repo = os.environ.get("CLAUDE_PROJECT_DIR") or cwd
    reason = check(event.get("tool_name"), event.get("tool_input") or {}, cwd, repo, min_lines, max_view)
    if reason:
        print(reason, file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    sys.exit(main())
