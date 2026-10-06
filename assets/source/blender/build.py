"""Build every world sprite from source.

Headless (reproducible, what `make art` runs):
    blender -b --factory-startup -P assets/source/blender/build.py -- [family ...]
Live (Blender MCP / Text editor):
    exec(open(".../assets/source/blender/build.py").read())
"""
import importlib
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__)) if "__file__" in globals() else \
    "/Users/asilva/youtube/rimworld-go/assets/source/blender"
if HERE not in sys.path:
    sys.path.insert(0, HERE)

import artlib  # noqa: E402

FAMILIES = ["terrain", "structure", "furniture", "plant", "item", "pawn", "overlay"]


def _reload():
    for name in sorted([m for m in sys.modules if m == "artlib" or m.startswith("artlib.")], key=len, reverse=True):
        importlib.reload(sys.modules[name])


def build(families=None):
    _reload()
    from artlib import stage
    stage.reset_palette()
    stage.clear()
    done = []
    for fam in families or FAMILIES:
        mod = importlib.import_module("artlib.families." + fam)
        importlib.reload(mod)
        done += mod.build()
    if families is None:
        done.append(stage.save_template())
    print("built %d sprites" % len(done))
    return done


if __name__ == "__main__":
    args = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    build(args or None)
