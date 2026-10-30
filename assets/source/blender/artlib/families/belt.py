"""Utility belts (CBT-3, ART §10, §15-16): worn layers + ground sprites.

A belt is drawn over every other body layer, just under the beard and hair
(the order in armor.py: ... flak jacket -> plate / marine armor -> belt ->
beard -> hair -> helmet). Keys follow the armor family:
    pawn/<belt>_<build>_<facing>   every build (m_avg unsuffixed), S/E/N
    item/<kind with _>              the belt lying coiled on the ground
- shield_belt: a dark belt with a round field emitter at the buckle (blue
  lens) and a capacitor cell on each hip.
- smokepop_pack: an olive belt with three grey canisters, orange caps, in a
  rack on the back and a pull-cord pouch at the buckle.
Fixed colours (no tint mask). All shapes are original.
    make art FAMILIES=belt         (RWG_BELT=shield_belt and RWG_PAWN_BUILDS=m_avg
                                    narrow a review build)
"""
import math
import os

from .. import stage as S
from . import pawn as P
from .gear import _rod
from .pawn import CX, CY, G, _extent, _shell

Z0, Z1 = 0.18, 0.245          # the belt band, over the pants' own belt
GB = G + 0.036                # clears the flak vest's shell


def _front(z, g):
    return CY - _extent(z, g)[1]


def _shield_belt():
    _shell("sbelt", Z0, Z1, GB, "fur_black")
    _shell("sstitch", Z0 + 0.026, Z0 + 0.036, GB + 0.004, "metal_dark")
    zc = (Z0 + Z1) / 2
    y = _front(zc, GB)
    S.ellipsoid("emitter", CX, y - 0.012, zc, 0.06, 0.026, 0.06, "steel", segs=18, rings=8)
    S.ellipsoid("lens", CX, y - 0.034, zc, 0.034, 0.01, 0.034, "crystal_blue", segs=16, rings=6)
    wx, _, _ = _extent(zc, GB)
    for sx in (-1, 1):
        x = CX + sx * (wx + 0.004)
        S.box("cell", x - 0.026, CY - 0.055, Z0 - 0.025, x + 0.026, CY + 0.045, Z1 + 0.02, "metal_dark",
              bevel=0.01)
        S.box("cell_light", x - 0.03 * (sx < 0) - 0.002, CY - 0.035, Z0 + 0.005, x + 0.03 * (sx > 0) + 0.002,
              CY + 0.025, Z0 + 0.02, "crystal_blue", bevel=0.003)


def _smokepop_pack():
    _shell("pbelt", Z0, Z1, GB, "fur_moss")
    zc = (Z0 + Z1) / 2
    y = _front(zc, GB)
    S.box("pouch", CX - 0.045, y - 0.03, Z0 - 0.02, CX + 0.045, y + 0.01, Z1 + 0.015, "fur_moss", bevel=0.012)
    S.box("tab", CX - 0.012, y - 0.04, Z0 - 0.045, CX + 0.012, y - 0.026, Z0 + 0.0, "tooth_orange", bevel=0.004)
    _, _, yb = _extent(zc, GB)
    S.box("rack", CX - 0.12, CY + yb - 0.01, Z0 - 0.01, CX + 0.12, CY + yb + 0.02, Z1 + 0.01, "fur_black",
          bevel=0.008)
    for k in (-1, 0, 1):
        x = CX + k * 0.075
        S.cyl("canister", x, CY + yb + 0.05, 0.034, Z0 - 0.03, Z1 + 0.07, "ash_grey", segs=14, bevel=0.008)
        S.cyl("cap", x, CY + yb + 0.05, 0.036, Z1 + 0.07, Z1 + 0.095, "tooth_orange", segs=14, bevel=0.006)


WORN = {"shield_belt": _shield_belt, "smokepop_pack": _smokepop_pack}


def _layer(name, facing, builder):
    """One belt over a body holdout, like armor._layer (m_avg unsuffixed)."""
    P._FACING[0] = facing
    S.holdout(P._body())
    builder()
    P._apply_modifiers()
    P._figure(facing)
    extra = {"layer": name, "facing": facing}
    tag = name
    if P._build_key != P.LEGACY_BUILD:
        tag = "%s_%s" % (name, P._build_key)
        extra["build"] = P._build_key
    return S.render_asset("pawn", "%s_%s" % (tag, facing), zmax=P._zmax_screen(), margins=P.MARGINS, extra=extra)


# ------------------------------------------------------------ ground items
def _coil(cx, cy, rx, ry, w, mat, z1=0.03, n=20, gap=0.0):
    """A belt lying in a loose loop: n short segments round an ellipse, an
    opening of `gap` radians at the front for the buckle."""
    a0 = -math.pi / 2 + gap / 2
    span = 2 * math.pi - gap
    for i in range(n):
        t0, t1 = a0 + span * i / n, a0 + span * (i + 1) / n
        pts = []
        for t, r in ((t0, 1.0), (t1, 1.0), (t1, -1.0), (t0, -1.0)):
            pts.append((cx + math.cos(t) * (rx + r * w / 2), cy + math.sin(t) * (ry + r * w / 2)))
        S.prism("coil%d" % i, pts, 0.0, z1, mat, bevel=0.004)


def _shield_belt_item():
    _coil(0.5, -0.52, 0.30, 0.22, 0.08, "fur_black", gap=0.5)
    S.cyl("emitter", 0.5, -0.76, 0.085, 0.0, 0.07, "steel", segs=24, bevel=0.01)
    S.cyl("lens", 0.5, -0.76, 0.05, 0.07, 0.085, "crystal_blue", segs=20, bevel=0.006)
    for x in (0.21, 0.79):
        S.box("cell", x - 0.05, -0.62, 0.0, x + 0.05, -0.44, 0.09, "metal_dark", bevel=0.012)
        S.box("cell_light", x - 0.03, -0.60, 0.09, x + 0.03, -0.46, 0.098, "crystal_blue", bevel=0.003)
    return S.render_asset("item", "shield_belt", zmax=0.14, extra={"item": "shield belt"})


def _smokepop_pack_item():
    _coil(0.5, -0.50, 0.30, 0.22, 0.075, "fur_moss", gap=0.5)
    S.box("pouch", 0.43, -0.80, 0.0, 0.57, -0.70, 0.07, "fur_moss", bevel=0.014)
    S.box("tab", 0.485, -0.86, 0.0, 0.515, -0.79, 0.03, "tooth_orange", bevel=0.004)
    S.box("rack", 0.34, -0.66, 0.0, 0.40, -0.36, 0.05, "fur_black", bevel=0.008)
    for y in (-0.42, -0.51, -0.60):         # the canisters lie in the loop, caps east
        _rod(0.36, 0.62, 0.04, "ash_grey", z=0.045, y=y)
        _rod(0.62, 0.66, 0.042, "tooth_orange", z=0.045, y=y)
    return S.render_asset("item", "smokepop_pack", zmax=0.14, extra={"item": "smokepop pack"})


ITEMS = {"shield_belt": _shield_belt_item, "smokepop_pack": _smokepop_pack_item}


def _env(name):
    v = os.environ.get(name, "").strip()
    return [x for x in v.split(",") if x] or None


def build():
    only = _env("RWG_BELT")
    builds = _env("RWG_PAWN_BUILDS") or list(P.BUILD_KEYS)
    out = []
    try:
        for bk in builds:
            P.use_build(bk)
            for f in P.FACINGS:
                for name, fn in WORN.items():
                    if not only or name in only:
                        out.append(_layer(name, f, fn))
    finally:
        P.use_build(P.LEGACY_BUILD)
        P._FACING[0] = "south"
    for name, fn in ITEMS.items():
        if not only or name in only:
            out.append(fn())
    return out
