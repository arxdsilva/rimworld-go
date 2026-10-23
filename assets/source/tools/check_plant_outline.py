#!/usr/bin/env python3
"""Fail if any plant or tree sprite recipe renders with an outline.

Plants (ground plants, bushes, crops, trees) never have a contour
(ART spec 'Housekeeping (PLAY-010)'). Outline is a render_asset() argument,
so this reads the recipe sources with ast instead of looking at pixels.

Usage: check_plant_outline.py [file.py ...]   (default: the plant recipes)
Rules, per file:
  * a call to anything named *asset must pass outline=False, or pass **kw
    where the enclosing function builds that dict with outline=False, or
    forwards its own **kwargs (its callers are checked instead);
  * any `outline=` keyword must be the constant False;
  * any `outline_width=` keyword must be the constant 0;
  * no call to post.outline() / outline().
Exit 1 and one 'file:line: reason' per violation.
"""
import ast
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
FAMILIES = os.path.join(HERE, "..", "blender", "artlib", "families")
DEFAULT = [os.path.join(FAMILIES, n) for n in ("plant.py", "plant_base.py", "trees.py")]


def _const(node, value):
    return isinstance(node, ast.Constant) and node.value is value or (
        isinstance(node, ast.Constant) and node.value == value and type(node.value) is type(value))


def _name(call):
    f = call.func
    return f.attr if isinstance(f, ast.Attribute) else getattr(f, "id", "")


def _dict_sets_outline_false(fn):
    for n in ast.walk(fn):
        if isinstance(n, ast.Call) and _name(n) == "dict":
            for kw in n.keywords:
                if kw.arg == "outline" and _const(kw.value, False):
                    return True
    return False


def check(path):
    with open(path, encoding="utf-8") as f:
        tree = ast.parse(f.read(), path)
    out = []
    parents = {}
    for p in ast.walk(tree):
        for c in ast.iter_child_nodes(p):
            parents[c] = p

    def enclosing(n):
        while n in parents:
            n = parents[n]
            if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)):
                return n
        return tree

    for n in ast.walk(tree):
        if not isinstance(n, ast.Call):
            continue
        name = _name(n)
        kws = {k.arg: k.value for k in n.keywords if k.arg}
        for arg, want in (("outline", False), ("outline_width", 0)):
            if arg in kws and not _const(kws[arg], want):
                out.append("%s:%d: %s must be the constant %r" % (path, n.lineno, arg, want))
        if name == "outline":
            out.append("%s:%d: plants must not call outline()" % (path, n.lineno))
        if name.endswith("asset") and name != "_plant_asset":
            if "outline" not in kws:
                fn = enclosing(n)
                stars = [k.value for k in n.keywords if k.arg is None]
                passthru = (isinstance(fn, ast.FunctionDef) and fn.args.kwarg is not None
                            and any(isinstance(v, ast.Name) and v.id == fn.args.kwarg.arg for v in stars))
                if not (passthru or (stars and _dict_sets_outline_false(fn))):
                    out.append("%s:%d: %s() without outline=False" % (path, n.lineno, name))
    return out


def main(argv):
    bad = []
    for p in argv or DEFAULT:
        bad += check(p)
    for b in bad:
        print(b)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
