"""Held gear: weapons and hand tools (ART §15, §27.2).

Each item lies flat pointing east (+X); the renderer rotates it. Attachments
are image px: `grip` (main hand), `muzzle` (rifle) and `head` (tools).
Extracted from pawn.py so pawn poses and gear can be built separately:
    make art FAMILIES=gear
"""
from .. import contracts as C
from .. import stage as S
from .pawn import _project


def _rifle():
    """Lying flat, muzzle pointing east (+X). Grip/muzzle in image px."""
    y = -0.5
    S.box("stock", 0.06, y - 0.045, 0.0, 0.30, y + 0.045, 0.06, "wood", bevel=0.02)
    S.box("receiver", 0.28, y - 0.04, 0.0, 0.58, y + 0.04, 0.07, "metal_dark", bevel=0.012)
    S.box("grip", 0.30, y - 0.10, 0.0, 0.36, y - 0.03, 0.05, "wood", bevel=0.012)
    S.box("mag", 0.44, y - 0.13, 0.0, 0.51, y - 0.03, 0.045, "metal_dark", bevel=0.01)
    S.box("guard", 0.56, y - 0.035, 0.0, 0.74, y + 0.035, 0.055, "wood", bevel=0.012)
    S.cyl("barrel", 0, 0, 0.018, 0.0, 0.24, "steel", segs=8, bevel=0.0)
    import math
    from mathutils import Matrix
    S.objects()[-1].matrix_world = Matrix.Translation((0.72, y, 0.03)) @ Matrix.Rotation(math.radians(90), 4, "Y")
    S.box("sight", 0.40, y - 0.015, 0.07, 0.46, y + 0.015, 0.10, "metal_dark", bevel=0.005)
    S.transform(S.objects(), Matrix.Translation((0.5, y, 0)) @ Matrix.Scale(0.8, 4) @ Matrix.Translation((-0.5, -y, 0)))
    m = (int(round(0.125 * C.PPT)), int(-(-((C.SHEAR_K * 0.11 + 0.125) * C.PPT) // 1)))
    return S.render_asset("weapon", "rifle_basic", zmax=0.11,
                          extra={"weapon": "rifle", "rotatable": True, "points": "east",
                                 "attachments": {"grip": _project(0.5 + (0.33 - 0.5) * 0.8, y - 0.056, 0.016, m),
                                                 "muzzle": _project(0.5 + (0.96 - 0.5) * 0.8, y, 0.024, m)}})


def _tool_render(name, length, grip_x):
    """Render the staged tool lying flat, pointing east (head at +X)."""
    m = (int(round(0.125 * C.PPT)), int(-(-((C.SHEAR_K * 0.12 + 0.125) * C.PPT) // 1)))
    return S.render_asset("tool", name, zmax=0.12,
                          extra={"tool": name, "rotatable": True, "points": "east",
                                 "attachments": {"grip": _project(grip_x, -0.5, 0.02, m),
                                                 "head": _project(grip_x + length, -0.5, 0.04, m)}})


def _tools():
    """Hand tools held during work: hammer (building), axe (chopping),
    pickaxe (mining). Lying flat, pointing east, like the rifle."""
    import math
    from mathutils import Matrix
    y = -0.5
    out = []

    def handle(x0, x1):
        S.cyl("handle", 0, 0, 0.022, 0.0, x1 - x0, "wood", segs=8, bevel=0.0)
        S.objects()[-1].matrix_world = Matrix.Translation((x0, y, 0.03)) @ Matrix.Rotation(math.radians(90), 4, "Y")

    handle(0.18, 0.74)
    S.box("head", 0.70, y - 0.10, 0.0, 0.80, y + 0.07, 0.07, "steel", bevel=0.012)
    S.box("claw", 0.72, y + 0.07, 0.01, 0.78, y + 0.13, 0.05, "metal_dark", bevel=0.008)
    out.append(_tool_render("hammer", 0.56, 0.26))

    handle(0.14, 0.82)
    S.prism("blade", [(0.70, y - 0.03), (0.86, y - 0.03), (0.90, y - 0.17), (0.66, y - 0.15)], 0.0, 0.05,
            "steel", bevel=0.01)
    S.box("poll", 0.74, y - 0.03, 0.0, 0.84, y + 0.05, 0.06, "metal_dark", bevel=0.01)
    out.append(_tool_render("axe", 0.64, 0.22))

    handle(0.16, 0.80)
    S.prism("pick", [(0.78, y + 0.20), (0.84, y + 0.02), (0.84, y - 0.02), (0.78, y - 0.20),
                     (0.74, y - 0.04), (0.74, y + 0.04)], 0.0, 0.05, "steel", bevel=0.008)
    out.append(_tool_render("pickaxe", 0.62, 0.24))
    return out


def build():
    return [_rifle()] + _tools()
