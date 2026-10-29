"""Build every world sprite from source.

Headless (reproducible, what `make art` runs):
    blender -b --factory-startup -P assets/source/blender/build.py -- [family ...]
Live (Blender MCP / Text editor):
    p = ".../assets/source/blender/build.py"
    exec(open(p).read(), {"__file__": p, "__name__": "rwg_live"}); build(["furniture"])
Scenes (not part of a default build): build(["menu"]) -> artlib/scenes/menu.py
"""
import importlib
import os
import sys

import bpy


def _here():
    if "__file__" in globals():
        return os.path.dirname(os.path.abspath(__file__))
    if os.environ.get("RWG_BLENDER_DIR"):
        return os.environ["RWG_BLENDER_DIR"]
    if bpy.data.filepath:  # template.blend lives next to this script
        return os.path.dirname(bpy.data.filepath)
    raise RuntimeError("build.py: exec with {'__file__': path} or set RWG_BLENDER_DIR")


HERE = _here()
if HERE not in sys.path:
    sys.path.insert(0, HERE)

import artlib  # noqa: E402

FAMILIES = ["terrain", "rock", "structure", "fence", "power", "defense", "furniture", "bedroom", "plant", "trees", "item", "pawn", "gear", "arms", "armor", "animal", "weather", "overlay", "effect"]
SCENES = ["menu"]   # whole-scene renders; only built when named


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
        pkg = "artlib.scenes." if fam in SCENES else "artlib.families."
        mod = importlib.import_module(pkg + fam)
        importlib.reload(mod)
        done += mod.build()
    if families is None:
        done.append(stage.save_template())
    print("built %d sprites" % len(done))
    return done


if __name__ == "__main__":
    args = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
    build(args or None)
