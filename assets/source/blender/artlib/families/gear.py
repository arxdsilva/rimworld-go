"""Held gear: weapons (ART §15, §27.2).

Each item lies flat pointing east (+X); the renderer rotates it. Attachments
are image px: `grip` (main hand), `support` (off hand), `muzzle` (firearms),
`tip` (knife, spear, club striking end) and `stock` (rifle butt). The
renderer places a weapon by its centre (ART-027): carried low across the
body on duty, aimed out from the body centre; nothing is drawn off duty and
no tools are drawn at all.
Extracted from pawn.py so pawn poses and gear can be built separately:
    make art FAMILIES=gear
"""
import math

from .. import contracts as C
from .. import stage as S
from .pawn import _project

Y = -0.5  # item centre line


def _origin(zmax):
    """Same canvas origin render_asset derives from the default margins."""
    return (int(round(C.DEFAULT_MARGIN * C.PPT)),
            int(math.ceil((C.SHEAR_K * zmax + C.DEFAULT_MARGIN) * C.PPT)))


def _render(family, name, zmax, key, pts, rotatable=True, extra=None):
    """Render the staged item; `pts` maps attachment name -> (x, y, z) blender."""
    m = _origin(zmax)
    ex = {key[0]: key[1], "rotatable": rotatable}
    if rotatable:
        ex["points"] = "east"
    ex.update(extra or {})
    ex["attachments"] = {k: _project(x, y, z, m) for k, (x, y, z) in pts.items()}
    return S.render_asset(family, name, zmax=zmax, extra=ex)


def _rod(x0, x1, r, material, z=0.03, y=Y, r_top=None):
    """A round shaft lying along +X from x0 to x1."""
    from mathutils import Matrix
    S.cyl("rod", 0, 0, r, 0.0, x1 - x0, material, segs=8, r_top=r_top, bevel=0.0)
    S.objects()[-1].matrix_world = Matrix.Translation((x0, y, z)) @ Matrix.Rotation(math.radians(90), 4, "Y")


def _wraps(x0, x1, r, n, material="leather", alt="bark", z=0.03, y=Y):
    """Binding bands around a shaft."""
    step = (x1 - x0) / n
    for i in range(n):
        S.box("wrap", x0 + i * step, y - r - 0.006, z - r - 0.006, x0 + (i + 0.62) * step, y + r + 0.006,
              z + r + 0.006, material if i % 2 == 0 else alt, bevel=0.004)


# ----------------------------------------------------------------- rifle
def _rifle_geo():
    """Bolt-action rifle, butt at x~0.1, muzzle at x~0.93."""
    y = Y
    # stock: tapered wooden butt, slim wrist, then the fore-end
    S.prism("stock", [(0.10, y - 0.068), (0.20, y - 0.058), (0.34, y - 0.036), (0.34, y + 0.036),
                      (0.20, y + 0.056), (0.10, y + 0.066)], 0.0, 0.065, "wood", bevel=0.012)
    S.box("butt_plate", 0.085, y - 0.07, 0.0, 0.11, y + 0.07, 0.07, "metal_dark", bevel=0.006)
    S.box("receiver", 0.32, y - 0.042, 0.0, 0.58, y + 0.042, 0.075, "metal_dark", bevel=0.012)
    S.box("bolt", 0.40, y - 0.014, 0.075, 0.54, y + 0.014, 0.092, "steel", bevel=0.004)
    S.box("bolt_arm", 0.50, y - 0.095, 0.05, 0.525, y - 0.03, 0.075, "steel", bevel=0.004)
    S.ellipsoid("bolt_knob", 0.512, y - 0.105, 0.062, 0.022, 0.022, 0.022, "metal_dark", segs=10, rings=6)
    S.box("mag", 0.42, y - 0.13, 0.0, 0.50, y - 0.035, 0.05, "metal_dark", bevel=0.01)
    S.box("fore_end", 0.57, y - 0.04, 0.0, 0.76, y + 0.04, 0.058, "wood", bevel=0.012)
    S.box("band", 0.74, y - 0.045, 0.0, 0.775, y + 0.045, 0.062, "metal_dark", bevel=0.006)
    _rod(0.76, 0.93, 0.016, "steel", z=0.034)
    S.box("sight", 0.88, y - 0.011, 0.04, 0.905, y + 0.011, 0.075, "metal_dark", bevel=0.004)
    # sling: strap swivel to swivel, hanging in a loop beside the rifle
    loop = [(0.18, y + 0.055)]
    for i in range(1, 8):
        t = i / 8.0
        loop.append((0.18 + 0.50 * t, y + 0.055 + 0.085 * math.sin(math.pi * t)))
    loop.append((0.68, y + 0.04))
    outer = loop
    inner = [(x, yy - 0.014) for x, yy in reversed(loop)]
    S.prism("sling", outer + inner, 0.0, 0.016, "leather", bevel=0.002)
    S.cyl("swivel_a", 0.18, y + 0.052, 0.012, 0.0, 0.04, "steel", segs=6, bevel=0.0)
    S.cyl("swivel_b", 0.68, y + 0.04, 0.012, 0.0, 0.04, "steel", segs=6, bevel=0.0)


def _rifle():
    _rifle_geo()
    return _render("weapon", "rifle_basic", 0.11, ("weapon", "rifle"),
                   {"grip": (0.37, Y, 0.05), "support": (0.64, Y, 0.045),
                    "muzzle": (0.94, Y, 0.034), "stock": (0.10, Y, 0.04)})


# ----------------------------------------------------------- melee weapons
def _knife_geo():
    y = Y
    _rod(0.20, 0.47, 0.026, "wood", r_top=0.03)
    _wraps(0.23, 0.45, 0.03, 5)
    S.ellipsoid("pommel", 0.205, y, 0.03, 0.03, 0.034, 0.03, "wood", segs=10, rings=6)
    S.box("lash", 0.45, y - 0.045, 0.0, 0.50, y + 0.045, 0.07, "straw", bevel=0.008)
    # knapped flint blade: leaf shaped, one edge more curved than the other
    S.prism("blade", [(0.49, y - 0.05), (0.60, y - 0.078), (0.72, y - 0.06), (0.86, y),
                      (0.72, y + 0.05), (0.60, y + 0.062), (0.49, y + 0.045)], 0.0, 0.035, "stone",
            bevel=0.006)
    S.prism("ridge", [(0.50, y - 0.012), (0.70, y - 0.015), (0.84, y), (0.70, y + 0.012), (0.50, y + 0.012)],
            0.035, 0.05, "stone_dark", bevel=0.004)


def _knife():
    _knife_geo()
    return _render("weapon", "knife", 0.07, ("weapon", "knife"),
                   {"grip": (0.30, Y, 0.04), "support": (0.43, Y, 0.04), "tip": (0.86, Y, 0.03)})


def _spear_geo():
    y = Y
    _rod(0.06, 0.88, 0.017, "wood", r_top=0.014)
    _wraps(0.14, 0.26, 0.019, 3)
    S.ellipsoid("butt", 0.06, y, 0.03, 0.02, 0.02, 0.02, "bark", segs=8, rings=5)
    # knapped stone head, bound into a split shaft
    S.prism("head", [(0.80, y - 0.05), (0.88, y - 0.085), (0.95, y - 0.045), (1.02, y), (0.95, y + 0.045),
                     (0.88, y + 0.085), (0.80, y + 0.05)], 0.0, 0.035, "stone", bevel=0.006)
    S.prism("head_ridge", [(0.83, y - 0.012), (0.93, y - 0.01), (1.0, y), (0.93, y + 0.01), (0.83, y + 0.012)],
            0.035, 0.05, "stone_dark", bevel=0.004)
    S.box("binding", 0.795, y - 0.034, 0.0, 0.845, y + 0.034, 0.062, "straw", bevel=0.008)
    S.box("binding2", 0.775, y - 0.03, 0.0, 0.79, y + 0.03, 0.058, "leather", bevel=0.004)


def _spear():
    _spear_geo()
    return _render("weapon", "spear", 0.08, ("weapon", "spear"),
                   {"grip": (0.17, Y, 0.04), "support": (0.42, Y, 0.04), "tip": (1.02, Y, 0.03)})


def _club():
    y = Y
    _rod(0.18, 0.72, 0.024, "wood", r_top=0.04)
    _wraps(0.20, 0.38, 0.028, 4)
    S.ellipsoid("pommel", 0.18, y, 0.03, 0.026, 0.034, 0.03, "bark", segs=8, rings=5)
    S.ellipsoid("head", 0.80, y, 0.06, 0.15, 0.115, 0.08, "bark", segs=14, rings=8)
    S.ellipsoid("head_hi", 0.80, y + 0.03, 0.095, 0.11, 0.06, 0.05, "wood", segs=12, rings=6)
    for k, (dx, dy) in enumerate([(0.0, 0.11), (0.07, -0.1), (-0.06, -0.1), (0.1, 0.07), (0.13, 0.0)]):
        S.cyl("knob%d" % k, 0.80 + dx, y + dy, 0.02, 0.05, 0.11, "stone_dark", segs=6, r_top=0.006, bevel=0.0)
    return _render("weapon", "club", 0.12, ("weapon", "club"),
                   {"grip": (0.28, Y, 0.04), "support": (0.42, Y, 0.04), "tip": (0.94, Y, 0.06)})


def build():
    return [_rifle(), _knife(), _spear(), _club()]
