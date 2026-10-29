"""Vanilla-core weapon catalogue (DEF-2, ART §15): held + ground sprites.

Every weapon is authored once, flat and pointing east (+X) along the item
centre line, like `gear.py`; the same geometry gives
    weapon/<key>   the held sprite (rotated by the renderer), attachments
                   `grip`, `support`, and `muzzle` (firearms, bows,
                   launchers) or `tip` (melee, thrown), plus `stock`;
    item/<kind>    the ground sprite: the weapon lying at an angle
                   (ItemSprite's "item/"+kind fallback finds it).
Thrown weapons hold one grenade or bottle and lie on the ground as a
small pile ("frag grenades", "molotov cocktails", "EMP grenades").
"Down" on a gun (grip, magazine) is -Y, as on the bolt-action rifle.
Silhouettes keep the classes apart at gameplay zoom: pistol / revolver /
SMG / rifle / shotgun / long rifle / machine gun / launcher / blade /
blunt / bow / javelin. All shapes are original.
    make art FAMILIES=arms          (RWG_ARMS=mace,lmg narrows a review build)
"""
import math
import os

from .. import stage as S
from .gear import Y, _render, _rod, _wraps


def _rod_y(y0, y1, r, material, x, z=0.03):
    """A round shaft lying along +Y from y0 to y1 at x."""
    from mathutils import Matrix
    S.cyl("rody", 0, 0, r, 0.0, y1 - y0, material, segs=8, bevel=0.0)
    S.objects()[-1].matrix_world = Matrix.Translation((x, y0, z)) @ Matrix.Rotation(math.radians(-90), 4, "X")


def _grip(x, w=0.06, drop=0.13, rake=0.04, material="bark", z1=0.055):
    """A pistol grip hanging down (-Y) from the receiver at x, raked back."""
    S.prism("grip", [(x - w / 2, Y - 0.02), (x + w / 2, Y - 0.02), (x + w / 2 - rake, Y - drop),
                     (x - w / 2 - rake, Y - drop)], 0.0, z1, material, bevel=0.01)


def _guard(x0, x1, drop=0.075, material="metal_dark"):
    """A trigger guard: a thin loop under the receiver."""
    S.prism("guard", [(x0, Y - 0.02), (x1, Y - 0.02), (x1, Y - drop), (x0, Y - drop),
                      (x0, Y - drop + 0.012), (x1 - 0.012, Y - drop + 0.012), (x1 - 0.012, Y - 0.03),
                      (x0, Y - 0.03)], 0.012, 0.03, material, bevel=0.0)


# ------------------------------------------------------------------ melee
def _mace():
    _rod(0.14, 0.74, 0.022, "wood", r_top=0.026)
    _wraps(0.16, 0.36, 0.026, 4)
    S.ellipsoid("pommel", 0.14, Y, 0.03, 0.03, 0.034, 0.03, "metal_dark", segs=10, rings=6)
    S.box("collar", 0.70, Y - 0.034, 0.0, 0.74, Y + 0.034, 0.064, "metal_dark", bevel=0.008)
    S.ellipsoid("core", 0.82, Y, 0.045, 0.09, 0.06, 0.06, "metal_dark", segs=14, rings=8)
    # six flanges fanning out of the core: four in the plane, two standing up
    for k, dy in enumerate((0.14, 0.09, -0.09, -0.14)):
        s = 1 if dy > 0 else -1
        S.prism("flange%d" % k, [(0.73, Y + s * 0.03), (0.89, Y + s * 0.03), (0.86, Y + dy), (0.77, Y + dy)],
                0.02 + 0.01 * (abs(dy) < 0.1), 0.05, "steel", bevel=0.006)
    S.box("fin", 0.74, Y - 0.014, 0.03, 0.88, Y + 0.014, 0.14, "steel", bevel=0.008)
    S.prism("spike", [(0.90, Y - 0.03), (0.98, Y), (0.90, Y + 0.03)], 0.025, 0.06, "steel", bevel=0.004)


def _gladius():
    S.ellipsoid("pommel", 0.16, Y, 0.035, 0.035, 0.05, 0.035, "horn", segs=12, rings=6)
    _rod(0.18, 0.31, 0.024, "horn_dark", r_top=0.022)
    for k, x in enumerate((0.21, 0.25, 0.29)):
        S.box("ring%d" % k, x - 0.006, Y - 0.028, 0.006, x + 0.006, Y + 0.028, 0.056, "horn", bevel=0.003)
    S.ellipsoid("guard", 0.325, Y, 0.035, 0.022, 0.08, 0.035, "gold", segs=12, rings=6)
    # broad leaf blade, waisted near the guard, a long tapering point
    S.prism("blade", [(0.34, Y - 0.045), (0.46, Y - 0.04), (0.64, Y - 0.05), (0.74, Y - 0.04), (0.86, Y),
                      (0.74, Y + 0.04), (0.64, Y + 0.05), (0.46, Y + 0.04), (0.34, Y + 0.045)],
            0.0, 0.03, "steel", bevel=0.006)
    S.prism("ridge", [(0.35, Y - 0.01), (0.74, Y - 0.012), (0.84, Y), (0.74, Y + 0.012), (0.35, Y + 0.01)],
            0.03, 0.04, "silver", bevel=0.003)


def _longsword():
    S.ellipsoid("pommel", 0.06, Y, 0.035, 0.03, 0.04, 0.035, "metal_dark", segs=12, rings=6)
    _rod(0.08, 0.27, 0.02, "leather")
    _wraps(0.09, 0.26, 0.022, 5, material="leather", alt="bark")
    S.prism("cross", [(0.27, Y - 0.13), (0.30, Y - 0.12), (0.31, Y - 0.02), (0.31, Y + 0.02), (0.30, Y + 0.12),
                      (0.27, Y + 0.13), (0.28, Y)], 0.01, 0.06, "metal_dark", bevel=0.008)
    S.prism("blade", [(0.31, Y - 0.034), (0.92, Y - 0.026), (1.03, Y), (0.92, Y + 0.026), (0.31, Y + 0.034)],
            0.015, 0.04, "steel", bevel=0.005)
    S.prism("fuller", [(0.32, Y - 0.008), (0.80, Y - 0.006), (0.80, Y + 0.006), (0.32, Y + 0.008)],
            0.04, 0.046, "silver", bevel=0.002)


def _pila():
    _rod(0.04, 0.68, 0.016, "wood", r_top=0.015)
    S.ellipsoid("butt", 0.04, Y, 0.03, 0.022, 0.022, 0.022, "metal_dark", segs=8, rings=5)
    S.box("binding", 0.36, Y - 0.024, 0.006, 0.46, Y + 0.024, 0.054, "leather", bevel=0.008)
    S.ellipsoid("weight", 0.69, Y, 0.03, 0.04, 0.032, 0.032, "metal_dark", segs=12, rings=8)
    _rod(0.70, 0.96, 0.008, "metal_dark", z=0.03)
    S.prism("head", [(0.95, Y - 0.028), (1.03, Y), (0.95, Y + 0.028), (0.94, Y)], 0.012, 0.048, "steel",
            bevel=0.004)


# ------------------------------------------------------------------- bows
def _bow(span, belly, tips, thick, wood, tip_mat, recurve=0.0, wraps=1):
    """A bow with limbs along Y and its belly towards +X, the string at the
    tips' x, an arrow nocked through the grip and pointing east. `grip` is
    the draw hand at the string, `support` the bow hand."""
    n = 12
    outer, inner = [], []
    for i in range(n + 1):
        t = -1 + 2 * i / n
        x = belly - (belly - tips) * t * t + recurve * max(0.0, abs(t) - 0.8) / 0.2
        w = thick * (1.0 - 0.45 * abs(t))
        outer.append((x + w / 2, Y + span * t))
        inner.append((x - w / 2, Y + span * t))
    S.prism("limbs", outer[::-1] + inner, 0.01, 0.045, wood, bevel=0.004)
    for s in (-1, 1):
        S.ellipsoid("nock", outer[0 if s < 0 else -1][0] - thick * 0.3, Y + s * span, 0.03, 0.022, 0.03, 0.022,
                    tip_mat, segs=10, rings=6)
    S.box("grip", belly - thick * 0.7, Y - 0.05, 0.0, belly + thick * 0.7, Y + 0.05, 0.055, "leather", bevel=0.01)
    if wraps > 1:
        for s in (-1, 1):
            S.box("wrap", belly - thick * 0.6, Y + s * 0.09 - 0.012, 0.004, belly + thick * 0.6,
                  Y + s * 0.09 + 0.012, 0.05, "bark", bevel=0.004)
    sx = tips - 0.004
    S.prism("string", [(sx - 0.005, Y - span), (sx + 0.005, Y - span), (sx + 0.005, Y + span),
                       (sx - 0.005, Y + span)], 0.02, 0.03, "horn", bevel=0.0)
    # the arrow: shaft, fletching at the string, a small head past the belly
    _rod(sx, belly + 0.30, 0.008, "wood", z=0.05)
    for s in (-1, 1):
        S.prism("fletch", [(sx + 0.01, Y), (sx + 0.08, Y), (sx + 0.03, Y + s * 0.03)], 0.045, 0.055, "food_red",
                bevel=0.0)
    S.prism("arrowhead", [(belly + 0.29, Y - 0.02), (belly + 0.35, Y), (belly + 0.29, Y + 0.02)], 0.04, 0.06,
            "stone_dark", bevel=0.003)
    return {"grip": (sx, Y, 0.05), "support": (belly, Y, 0.03), "muzzle": (belly + 0.35, Y, 0.05)}


def _short_bow():
    return _bow(0.30, 0.50, 0.36, 0.034, "wood", "bark")


def _recurve_bow():
    return _bow(0.36, 0.48, 0.30, 0.036, "bark", "horn", recurve=0.16)


def _greatbow():
    return _bow(0.46, 0.46, 0.34, 0.044, "wood", "horn_dark", wraps=2)


# -------------------------------------------------------------- handguns
def _revolver():
    _rod(0.44, 0.74, 0.018, "steel", z=0.05)
    S.box("rib", 0.44, Y - 0.01, 0.06, 0.74, Y + 0.01, 0.075, "steel", bevel=0.003)
    S.box("sight", 0.71, Y - 0.006, 0.07, 0.73, Y + 0.006, 0.085, "metal_dark", bevel=0.002)
    S.box("frame", 0.30, Y - 0.035, 0.0, 0.46, Y + 0.035, 0.07, "metal_dark", bevel=0.01)
    S.ellipsoid("cylinder", 0.39, Y, 0.045, 0.055, 0.05, 0.045, "steel", segs=12, rings=8)
    for k, dy in enumerate((-0.03, 0.0, 0.03)):
        S.box("flute%d" % k, 0.355, Y + dy - 0.005, 0.07, 0.425, Y + dy + 0.005, 0.09, "metal_dark", bevel=0.0)
    S.box("hammer", 0.28, Y - 0.012, 0.05, 0.31, Y + 0.012, 0.09, "metal_dark", bevel=0.004)
    S.prism("grip", [(0.27, Y - 0.02), (0.34, Y - 0.02), (0.30, Y - 0.15), (0.22, Y - 0.15), (0.21, Y - 0.12)],
            0.0, 0.055, "wood", bevel=0.012)
    _guard(0.33, 0.40)
    return {"grip": (0.29, Y, 0.04), "support": (0.38, Y, 0.04), "muzzle": (0.75, Y, 0.05)}


def _autopistol():
    S.box("slide", 0.28, Y - 0.032, 0.03, 0.64, Y + 0.032, 0.085, "metal_dark", bevel=0.012)
    S.box("frame", 0.30, Y - 0.04, 0.0, 0.58, Y + 0.02, 0.05, "fur_black", bevel=0.01)
    for k in range(4):
        x = 0.31 + 0.02 * k
        S.box("serr%d" % k, x, Y - 0.026, 0.084, x + 0.008, Y + 0.026, 0.092, "steel", bevel=0.0)
    S.box("ejector", 0.44, Y - 0.018, 0.084, 0.52, Y + 0.018, 0.092, "steel", bevel=0.002)
    _rod(0.63, 0.66, 0.012, "steel", z=0.06)
    S.box("sight", 0.60, Y - 0.006, 0.085, 0.62, Y + 0.006, 0.1, "metal_dark", bevel=0.002)
    _grip(0.34, w=0.07, drop=0.16, rake=0.05, material="fur_black")
    _guard(0.38, 0.47)
    return {"grip": (0.32, Y, 0.04), "support": (0.42, Y, 0.04), "muzzle": (0.66, Y, 0.06)}


def _machine_pistol():
    S.box("body", 0.26, Y - 0.038, 0.0, 0.62, Y + 0.038, 0.08, "metal_dark", bevel=0.012)
    S.box("shroud", 0.52, Y - 0.03, 0.01, 0.70, Y + 0.03, 0.07, "steel", bevel=0.01)
    for k in range(3):
        x = 0.56 + 0.045 * k
        S.box("vent%d" % k, x, Y - 0.012, 0.068, x + 0.024, Y + 0.012, 0.076, "fur_black", bevel=0.0)
    _rod(0.70, 0.74, 0.012, "metal_dark", z=0.04)
    # long stick magazine through the grip and a folded wire stock
    S.box("mag", 0.30, Y - 0.26, 0.005, 0.355, Y - 0.03, 0.05, "fur_black", bevel=0.008)
    S.box("grip", 0.285, Y - 0.12, 0.0, 0.37, Y - 0.03, 0.06, "bark", bevel=0.01)
    _guard(0.37, 0.45)
    S.prism("wire", [(0.10, Y - 0.04), (0.27, Y - 0.02), (0.27, Y), (0.12, Y - 0.02), (0.12, Y + 0.02),
                     (0.27, Y + 0.02), (0.27, Y + 0.035), (0.09, Y + 0.035)], 0.02, 0.04, "steel", bevel=0.0)
    S.box("butt", 0.08, Y - 0.05, 0.0, 0.11, Y + 0.045, 0.05, "metal_dark", bevel=0.006)
    return {"grip": (0.33, Y, 0.04), "support": (0.58, Y, 0.04), "muzzle": (0.74, Y, 0.04),
            "stock": (0.09, Y, 0.03)}


# ------------------------------------------------------- SMGs and rifles
def _heavy_smg():
    S.box("receiver", 0.24, Y - 0.05, 0.0, 0.66, Y + 0.05, 0.09, "metal_dark", bevel=0.014)
    S.box("top", 0.30, Y - 0.02, 0.09, 0.60, Y + 0.02, 0.105, "steel", bevel=0.004)
    S.box("shroud", 0.64, Y - 0.04, 0.01, 0.86, Y + 0.04, 0.08, "metal_dark", bevel=0.012)
    for k in range(4):
        x = 0.67 + 0.045 * k
        S.box("hole%d" % k, x, Y - 0.022, 0.078, x + 0.025, Y + 0.022, 0.086, "fur_black", bevel=0.0)
    _rod(0.86, 0.90, 0.016, "steel", z=0.045)
    S.prism("mag", [(0.48, Y - 0.04), (0.56, Y - 0.04), (0.60, Y - 0.22), (0.53, Y - 0.23)], 0.005, 0.06,
            "fur_black", bevel=0.01)
    _grip(0.34, w=0.07, drop=0.15, material="bark")
    _guard(0.37, 0.46)
    S.prism("stock", [(0.08, Y - 0.07), (0.25, Y - 0.04), (0.25, Y + 0.035), (0.08, Y + 0.05)], 0.0, 0.06,
            "fur_black", bevel=0.012)
    return {"grip": (0.33, Y, 0.045), "support": (0.70, Y, 0.045), "muzzle": (0.90, Y, 0.045),
            "stock": (0.08, Y, 0.03)}


def _assault_rifle():
    S.prism("stock", [(0.06, Y - 0.08), (0.12, Y - 0.08), (0.30, Y - 0.035), (0.30, Y + 0.035), (0.06, Y + 0.05)],
            0.0, 0.06, "fur_moss", bevel=0.012)
    S.box("butt", 0.05, Y - 0.085, 0.0, 0.075, Y + 0.055, 0.065, "fur_black", bevel=0.006)
    S.box("receiver", 0.29, Y - 0.042, 0.0, 0.62, Y + 0.042, 0.08, "metal_dark", bevel=0.012)
    S.box("rail", 0.33, Y - 0.014, 0.08, 0.60, Y + 0.014, 0.1, "fur_black", bevel=0.004)
    S.box("optic", 0.40, Y - 0.022, 0.1, 0.50, Y + 0.022, 0.13, "metal_dark", bevel=0.008)
    S.prism("mag", [(0.44, Y - 0.04), (0.52, Y - 0.04), (0.53, Y - 0.12), (0.50, Y - 0.20), (0.43, Y - 0.19),
                    (0.46, Y - 0.12)], 0.005, 0.055, "fur_black", bevel=0.008)
    _grip(0.36, w=0.06, drop=0.13, material="fur_black")
    S.box("handguard", 0.61, Y - 0.036, 0.005, 0.82, Y + 0.036, 0.07, "fur_moss", bevel=0.012)
    _rod(0.82, 0.95, 0.014, "steel", z=0.04)
    S.box("brake", 0.93, Y - 0.02, 0.02, 0.97, Y + 0.02, 0.06, "metal_dark", bevel=0.005)
    S.box("fsight", 0.80, Y - 0.008, 0.06, 0.82, Y + 0.008, 0.1, "metal_dark", bevel=0.002)
    return {"grip": (0.35, Y, 0.045), "support": (0.70, Y, 0.04), "muzzle": (0.97, Y, 0.04),
            "stock": (0.05, Y, 0.03)}


def _sniper_rifle():
    S.prism("stock", [(0.03, Y - 0.07), (0.08, Y - 0.075), (0.30, Y - 0.03), (0.30, Y + 0.035), (0.16, Y + 0.035),
                      (0.12, Y + 0.06), (0.03, Y + 0.06)], 0.0, 0.065, "wood", bevel=0.012)
    S.box("cheek", 0.10, Y - 0.02, 0.065, 0.24, Y + 0.05, 0.08, "wood", bevel=0.008)
    S.box("butt", 0.02, Y - 0.075, 0.0, 0.045, Y + 0.065, 0.07, "fur_black", bevel=0.006)
    S.box("receiver", 0.29, Y - 0.036, 0.0, 0.52, Y + 0.036, 0.07, "metal_dark", bevel=0.01)
    S.box("bolt_arm", 0.44, Y - 0.085, 0.045, 0.46, Y - 0.03, 0.068, "steel", bevel=0.003)
    S.ellipsoid("bolt_knob", 0.45, Y - 0.095, 0.056, 0.018, 0.018, 0.018, "metal_dark", segs=10, rings=6)
    S.box("fore", 0.51, Y - 0.032, 0.0, 0.70, Y + 0.032, 0.055, "wood", bevel=0.012)
    _rod(0.70, 1.04, 0.012, "metal_dark", z=0.032)
    S.box("brake", 1.02, Y - 0.018, 0.016, 1.06, Y + 0.018, 0.05, "metal_dark", bevel=0.004)
    # the long scope on two rings, lenses at both ends
    _rod(0.30, 0.58, 0.024, "metal_dark", z=0.115, y=Y)
    _rod(0.28, 0.31, 0.032, "metal_dark", z=0.115, y=Y)
    _rod(0.56, 0.60, 0.032, "metal_dark", z=0.115, y=Y)
    S.ellipsoid("lens", 0.602, Y, 0.115, 0.006, 0.026, 0.026, "glass", segs=12, rings=6)
    for x in (0.36, 0.50):
        S.box("ring", x - 0.012, Y - 0.026, 0.06, x + 0.012, Y + 0.026, 0.1, "steel", bevel=0.004)
    S.box("turret", 0.43, Y - 0.04, 0.1, 0.46, Y + 0.04, 0.13, "steel", bevel=0.004)
    # bipod folded under the barrel
    for dy in (-0.022, 0.022):
        S.box("leg", 0.72, Y + dy - 0.006, 0.0, 0.92, Y + dy + 0.006, 0.014, "steel", bevel=0.002)
    return {"grip": (0.32, Y, 0.04), "support": (0.62, Y, 0.035), "muzzle": (1.06, Y, 0.032),
            "stock": (0.03, Y, 0.03)}


# --------------------------------------------------------------- shotguns
def _pump_shotgun():
    S.prism("stock", [(0.06, Y - 0.075), (0.14, Y - 0.07), (0.32, Y - 0.032), (0.32, Y + 0.036),
                      (0.14, Y + 0.06), (0.06, Y + 0.065)], 0.0, 0.065, "wood", bevel=0.014)
    S.box("pad", 0.045, Y - 0.08, 0.0, 0.07, Y + 0.07, 0.07, "fur_black", bevel=0.006)
    S.box("receiver", 0.31, Y - 0.045, 0.0, 0.52, Y + 0.045, 0.08, "metal_dark", bevel=0.012)
    S.box("port", 0.38, Y - 0.02, 0.078, 0.46, Y + 0.02, 0.086, "steel", bevel=0.002)
    _guard(0.38, 0.47)
    _rod(0.52, 0.90, 0.022, "steel", z=0.055)
    _rod(0.52, 0.84, 0.017, "metal_dark", z=0.02, y=Y - 0.01)
    S.box("pump", 0.60, Y - 0.045, 0.0, 0.76, Y + 0.035, 0.05, "wood", bevel=0.014)
    for k in range(4):
        x = 0.62 + 0.035 * k
        S.box("groove%d" % k, x, Y - 0.047, 0.035, x + 0.01, Y + 0.037, 0.052, "bark", bevel=0.0)
    S.box("bead", 0.88, Y - 0.006, 0.075, 0.895, Y + 0.006, 0.088, "gold", bevel=0.002)
    return {"grip": (0.33, Y, 0.045), "support": (0.68, Y, 0.035), "muzzle": (0.91, Y, 0.055),
            "stock": (0.05, Y, 0.035)}


def _chain_shotgun():
    S.prism("stock", [(0.08, Y - 0.08), (0.30, Y - 0.04), (0.30, Y + 0.04), (0.08, Y + 0.06), (0.06, Y)],
            0.0, 0.065, "fur_black", bevel=0.014)
    S.box("receiver", 0.29, Y - 0.055, 0.0, 0.62, Y + 0.055, 0.09, "metal_dark", bevel=0.016)
    S.box("rail", 0.32, Y - 0.016, 0.09, 0.60, Y + 0.016, 0.104, "steel", bevel=0.003)
    _grip(0.36, w=0.07, drop=0.14, material="fur_black")
    # a fat drum under the receiver
    S.cyl("drum", 0.52, Y - 0.12, 0.085, 0.0, 0.07, "metal_dark", segs=20, bevel=0.012)
    S.cyl("drum_cap", 0.52, Y - 0.12, 0.05, 0.07, 0.085, "steel", segs=16, bevel=0.006)
    S.box("barrel", 0.62, Y - 0.032, 0.015, 0.84, Y + 0.032, 0.075, "metal_dark", bevel=0.01)
    _rod(0.62, 0.84, 0.02, "steel", z=0.085)
    S.box("brake", 0.84, Y - 0.04, 0.008, 0.90, Y + 0.04, 0.08, "steel", bevel=0.008)
    for dy in (-0.022, 0.022):
        S.box("slot", 0.85, Y + dy - 0.006, 0.075, 0.89, Y + dy + 0.006, 0.084, "fur_black", bevel=0.0)
    return {"grip": (0.35, Y, 0.045), "support": (0.70, Y, 0.045), "muzzle": (0.90, Y, 0.045),
            "stock": (0.06, Y, 0.03)}


# ---------------------------------------------------------- machine guns
def _lmg():
    S.prism("stock", [(0.04, Y - 0.08), (0.10, Y - 0.08), (0.28, Y - 0.04), (0.28, Y + 0.04), (0.04, Y + 0.06)],
            0.0, 0.065, "fur_moss", bevel=0.014)
    S.box("butt", 0.03, Y - 0.085, 0.0, 0.055, Y + 0.065, 0.07, "fur_black", bevel=0.006)
    S.box("receiver", 0.27, Y - 0.055, 0.0, 0.62, Y + 0.055, 0.095, "metal_dark", bevel=0.016)
    S.box("cover", 0.34, Y - 0.04, 0.095, 0.56, Y + 0.04, 0.11, "steel", bevel=0.006)
    _grip(0.34, w=0.065, drop=0.14, material="fur_black")
    # box of belted rounds hanging off the left side, the belt feeding in
    S.box("ammo", 0.40, Y - 0.20, 0.0, 0.54, Y - 0.07, 0.085, "fur_moss", bevel=0.012)
    S.box("lid", 0.40, Y - 0.20, 0.085, 0.54, Y - 0.07, 0.095, "metal_dark", bevel=0.006)
    S.box("belt", 0.44, Y - 0.075, 0.07, 0.50, Y - 0.04, 0.1, "gold", bevel=0.004)
    # heavy barrel in a vented jacket, carry handle, bipod folded forward
    S.box("jacket", 0.62, Y - 0.04, 0.01, 0.92, Y + 0.04, 0.08, "metal_dark", bevel=0.012)
    for k in range(5):
        x = 0.65 + 0.05 * k
        S.box("vent%d" % k, x, Y - 0.025, 0.078, x + 0.026, Y + 0.025, 0.086, "fur_black", bevel=0.0)
    _rod(0.92, 1.02, 0.017, "steel", z=0.045)
    S.box("flash", 1.0, Y - 0.026, 0.02, 1.04, Y + 0.026, 0.07, "metal_dark", bevel=0.006)
    S.prism("handle", [(0.62, Y + 0.035), (0.66, Y + 0.13), (0.78, Y + 0.13), (0.80, Y + 0.035), (0.77, Y + 0.035),
                       (0.755, Y + 0.105), (0.675, Y + 0.105), (0.655, Y + 0.035)], 0.03, 0.06, "fur_black",
            bevel=0.004)
    for dy in (-0.026, 0.026):
        S.box("leg", 0.86, Y + dy - 0.007, 0.0, 1.0, Y + dy + 0.007, 0.016, "steel", bevel=0.002)
    return {"grip": (0.32, Y, 0.045), "support": (0.70, Y + 0.08, 0.045), "muzzle": (1.04, Y, 0.045),
            "stock": (0.03, Y, 0.03)}


def _minigun():
    # motor housing with a top carry handle and a rear spade grip
    S.box("housing", 0.20, Y - 0.08, 0.0, 0.46, Y + 0.08, 0.13, "metal_dark", bevel=0.02, segments=2)
    S.box("motor", 0.24, Y - 0.1, 0.02, 0.36, Y - 0.06, 0.1, "steel", bevel=0.01)
    S.prism("handle", [(0.24, Y - 0.02), (0.26, Y + 0.0), (0.44, Y + 0.0), (0.46, Y - 0.02), (0.46, Y + 0.02),
                       (0.24, Y + 0.02)], 0.13, 0.16, "fur_black", bevel=0.006)
    for dy in (-0.05, 0.05):
        S.box("spade", 0.08, Y + dy - 0.018, 0.02, 0.21, Y + dy + 0.018, 0.08, "fur_black", bevel=0.008)
    S.box("spade_bar", 0.07, Y - 0.07, 0.03, 0.10, Y + 0.07, 0.07, "metal_dark", bevel=0.006)
    # six barrels round a spindle, clamped twice
    for k in range(6):
        a = 2 * math.pi * k / 6 + 0.26
        _rod(0.46, 1.0, 0.014, "steel" if k % 2 == 0 else "metal_dark", z=0.065 + 0.04 * math.sin(a),
             y=Y + 0.042 * math.cos(a))
    for x in (0.62, 0.92):
        _rod(x, x + 0.04, 0.062, "metal_dark", z=0.065)
    _rod(0.46, 0.50, 0.07, "metal_dark", z=0.065)
    # ammunition belt trailing back from the feed
    for k in range(7):
        t = k / 6.0
        x = 0.33 - 0.22 * t
        y = Y - 0.09 - 0.17 * math.sin(t * math.pi * 0.6)
        S.box("round%d" % k, x - 0.018, y - 0.03, 0.0, x + 0.018, y + 0.03, 0.035,
              "gold" if k % 2 == 0 else "horn_dark", bevel=0.006)
    return {"grip": (0.14, Y, 0.05), "support": (0.35, Y, 0.15), "muzzle": (1.0, Y, 0.065),
            "stock": (0.07, Y, 0.05)}


# --------------------------------------------------------------- thrown
def _frag_body(cx, cy, z=0.0, s=1.0):
    """An olive segmented frag grenade lying on its side, lever and pin."""
    S.ellipsoid("frag", cx, cy, z + 0.06 * s, 0.07 * s, 0.06 * s, 0.06 * s, "fur_moss", segs=16, rings=10)
    for k, dx in enumerate((-0.035, 0.0, 0.035)):
        S.box("seg%d" % k, cx + dx * s - 0.004 * s, cy - 0.062 * s, z + 0.005 * s, cx + dx * s + 0.004 * s,
              cy + 0.062 * s, z + 0.12 * s, "metal_dark", bevel=0.002)
    S.box("fuse", cx + 0.06 * s, cy - 0.022 * s, z + 0.04 * s, cx + 0.1 * s, cy + 0.022 * s, z + 0.08 * s,
          "steel", bevel=0.006)
    S.box("lever", cx - 0.03 * s, cy - 0.072 * s, z + 0.05 * s, cx + 0.09 * s, cy - 0.055 * s, z + 0.075 * s,
          "steel", bevel=0.003)
    S.cyl("pin", cx + 0.1 * s, cy + 0.03 * s, 0.022 * s, z + 0.055 * s, z + 0.065 * s, "gold", segs=12,
          bevel=0.0)


def _molotov_body(cx, cy, z=0.0, s=1.0):
    """A green glass bottle on its side, a rag stuffed in the neck, its end alight."""
    _rod(cx - 0.13 * s, cx + 0.05 * s, 0.058 * s, "leaf_pine", z=z + 0.058 * s, y=cy)
    S.ellipsoid("shoulder", cx + 0.05 * s, cy, z + 0.058 * s, 0.04 * s, 0.058 * s, 0.058 * s, "leaf_pine",
                segs=14, rings=8)
    _rod(cx + 0.07 * s, cx + 0.15 * s, 0.022 * s, "leaf_pine", z=z + 0.058 * s, y=cy)
    S.box("glint", cx - 0.10 * s, cy + 0.012 * s, z + 0.1 * s, cx + 0.02 * s, cy + 0.03 * s, z + 0.118 * s,
          "glass", bevel=0.004)
    S.box("label", cx - 0.07 * s, cy - 0.062 * s, z + 0.01 * s, cx - 0.01 * s, cy + 0.062 * s, z + 0.12 * s,
          "burlap", bevel=0.004)
    S.prism("rag", [(cx + 0.14 * s, cy - 0.035 * s), (cx + 0.22 * s, cy - 0.06 * s), (cx + 0.27 * s, cy - 0.01 * s),
                    (cx + 0.23 * s, cy + 0.05 * s), (cx + 0.14 * s, cy + 0.03 * s)], z + 0.03 * s,
            z + 0.085 * s, "burlap", bevel=0.006)
    S.prism("flame", [(cx + 0.22 * s, cy - 0.045 * s), (cx + 0.30 * s, cy - 0.02 * s), (cx + 0.33 * s, cy + 0.01 * s),
                      (cx + 0.28 * s, cy + 0.04 * s), (cx + 0.22 * s, cy + 0.035 * s)],
            z + 0.06 * s, z + 0.09 * s, "tooth_orange", bevel=0.006)
    S.prism("flame_core", [(cx + 0.235 * s, cy - 0.02 * s), (cx + 0.29 * s, cy), (cx + 0.235 * s, cy + 0.02 * s)],
            z + 0.09 * s, z + 0.1 * s, "corn_yellow", bevel=0.003)


def _emp_body(cx, cy, z=0.0, s=1.0):
    """A squat metal canister with glowing blue coil bands and a cap."""
    _rod(cx - 0.08 * s, cx + 0.07 * s, 0.05 * s, "metal_dark", z=z + 0.05 * s, y=cy)
    for dx in (-0.045, 0.02):
        _rod(cx + dx * s, cx + (dx + 0.025) * s, 0.056 * s, "crystal_blue", z=z + 0.05 * s, y=cy)
    _rod(cx + 0.07 * s, cx + 0.11 * s, 0.03 * s, "steel", z=z + 0.05 * s, y=cy)
    S.box("lever", cx - 0.06 * s, cy - 0.06 * s, z + 0.04 * s, cx + 0.1 * s, cy - 0.045 * s, z + 0.07 * s, "steel",
          bevel=0.003)


def _frag_grenade():
    _frag_body(0.45, Y, s=1.5)
    return {"grip": (0.45, Y, 0.09), "support": (0.52, Y, 0.09), "tip": (0.60, Y, 0.09)}


def _molotov_cocktail():
    _molotov_body(0.40, Y, s=1.4)
    return {"grip": (0.34, Y, 0.08), "support": (0.46, Y, 0.08), "tip": (0.86, Y, 0.08)}


def _emp_grenade():
    _emp_body(0.45, Y, s=1.5)
    return {"grip": (0.45, Y, 0.075), "support": (0.53, Y, 0.075), "tip": (0.62, Y, 0.075)}


# -------------------------------------------------------------- launchers
def _launcher(band, housing):
    """A single-shot, wide-bore launcher with a coloured warhead band."""
    S.prism("stock", [(0.08, Y - 0.07), (0.30, Y - 0.035), (0.30, Y + 0.035), (0.08, Y + 0.05)], 0.0, 0.06,
            "fur_black", bevel=0.012)
    S.box("pad", 0.065, Y - 0.075, 0.0, 0.09, Y + 0.055, 0.065, "metal_dark", bevel=0.006)
    S.box("breech", 0.29, Y - 0.05, 0.0, 0.46, Y + 0.05, 0.09, "metal_dark", bevel=0.014)
    _grip(0.34, w=0.065, drop=0.14, material="fur_black")
    _guard(0.37, 0.45)
    _rod(0.44, 0.90, 0.048, "metal_dark", z=0.05)
    _rod(0.86, 0.92, 0.056, "steel", z=0.05)
    _rod(0.50, 0.56, 0.052, band, z=0.05)
    S.box("fgrip", 0.62, Y - 0.12, 0.01, 0.67, Y - 0.04, 0.06, "fur_black", bevel=0.01)
    if housing:
        # EMP: a ribbed capacitor box on top with a blue coil window
        S.box("capacitor", 0.56, Y - 0.04, 0.09, 0.80, Y + 0.04, 0.14, "steel", bevel=0.01)
        for k in range(4):
            x = 0.59 + 0.05 * k
            S.box("rib%d" % k, x, Y - 0.045, 0.1, x + 0.018, Y + 0.045, 0.148, "metal_dark", bevel=0.003)
        S.box("window", 0.36, Y - 0.025, 0.09, 0.44, Y + 0.025, 0.105, band, bevel=0.004)
    else:
        # incendiary: a ring sight and a heat shield over the tube
        S.box("shield", 0.62, Y - 0.03, 0.09, 0.82, Y + 0.03, 0.11, "steel", bevel=0.006)
        S.cyl("ring", 0.40, Y, 0.03, 0.09, 0.15, "metal_dark", segs=12, bevel=0.0)
        S.cyl("ring_hole", 0.40, Y, 0.016, 0.15, 0.152, "fur_black", segs=12, bevel=0.0)
        S.box("hazard", 0.72, Y - 0.032, 0.105, 0.76, Y + 0.032, 0.116, band, bevel=0.002)
    return {"grip": (0.33, Y, 0.045), "support": (0.645, Y - 0.04, 0.04), "muzzle": (0.92, Y, 0.05),
            "stock": (0.07, Y, 0.03)}


def _incendiary_launcher():
    return _launcher("tooth_orange", housing=False)


def _emp_launcher():
    return _launcher("crystal_blue", housing=True)


# ------------------------------------------------------------- melee pts
def _melee(fn, pts):
    def run():
        fn()
        return pts
    return run


# key, item kind, builder (returns attachments in blender units), held zmax, ground yaw (deg)
WEAPONS = [
    ("mace", "mace", _melee(_mace, {"grip": (0.26, Y, 0.04), "support": (0.42, Y, 0.04),
                                    "tip": (0.98, Y, 0.045)}), 0.15, -24),
    ("gladius", "gladius", _melee(_gladius, {"grip": (0.25, Y, 0.035), "support": (0.33, Y, 0.035),
                                             "tip": (0.86, Y, 0.02)}), 0.07, 20),
    ("longsword", "longsword", _melee(_longsword, {"grip": (0.17, Y, 0.035), "support": (0.24, Y, 0.035),
                                                   "tip": (1.03, Y, 0.03)}), 0.07, -18),
    ("pila", "pila", _melee(_pila, {"grip": (0.41, Y, 0.03), "support": (0.56, Y, 0.03),
                                    "tip": (1.03, Y, 0.03)}), 0.06, 16),
    ("short_bow", "short bow", _short_bow, 0.07, -16),
    ("recurve_bow", "recurve bow", _recurve_bow, 0.07, 18),
    ("greatbow", "greatbow", _greatbow, 0.07, -12),
    ("revolver", "revolver", _revolver, 0.1, 18),
    ("autopistol", "autopistol", _autopistol, 0.11, -20),
    ("machine_pistol", "machine pistol", _machine_pistol, 0.09, 14),
    ("heavy_smg", "heavy smg", _heavy_smg, 0.12, -16),
    ("assault_rifle", "assault rifle", _assault_rifle, 0.14, 14),
    ("sniper_rifle", "sniper rifle", _sniper_rifle, 0.15, -12),
    ("pump_shotgun", "pump shotgun", _pump_shotgun, 0.1, 16),
    ("chain_shotgun", "chain shotgun", _chain_shotgun, 0.12, -14),
    ("lmg", "lmg", _lmg, 0.12, 12),
    ("minigun", "minigun", _minigun, 0.17, -10),
    ("frag_grenade", None, _frag_grenade, 0.19, 0),
    ("molotov_cocktail", None, _molotov_cocktail, 0.17, 0),
    ("emp_grenade", None, _emp_grenade, 0.16, 0),
    ("incendiary_launcher", "incendiary launcher", _incendiary_launcher, 0.16, 14),
    ("emp_launcher", "emp launcher", _emp_launcher, 0.16, -14),
]

LONG = ("longsword", "pila", "sniper_rifle", "lmg", "minigun", "greatbow")


def _held(key, fn, zmax):
    pts = fn()
    kind = "thrown" if key.endswith(("grenade", "cocktail")) else "weapon"
    return _render("weapon", key, zmax, ("weapon", key), pts, extra={"class": kind})


def _ground(key, kind, fn, zmax, yaw):
    fn()
    if key in LONG:  # shrink the longest a touch so they lie inside the tile
        from mathutils import Matrix
        piv = Matrix.Translation((0.5, Y, 0))
        S.transform(S.objects(), piv @ Matrix.Scale(0.9, 4) @ piv.inverted())
    S.rotate_z(S.objects(), yaw)
    return S.render_asset("item", key if kind is None else kind.replace(" ", "_"), zmax=zmax,
                          extra={"item": kind})


# ground piles of the thrown weapons
def _pile(body, name, kind, spots, zmax=0.14):
    for k, (x, y, yaw) in enumerate(spots):
        before = set(S.objects())
        body(x, y)
        S.rotate_z([o for o in S.objects() if o not in before], yaw, pivot=(x, y))
    return S.render_asset("item", name, zmax=zmax, extra={"item": kind, "stack": True})


def _frag_pile():
    return _pile(_frag_body, "frag_grenades", "frag grenades",
                 [(0.36, -0.62, 20), (0.62, -0.66, -30), (0.50, -0.40, 160)])


def _molotov_pile():
    # three bottles in a low open crate
    S.box("crate", 0.18, -0.78, 0.0, 0.82, -0.30, 0.10, "wood_plank", bevel=0.012)
    S.box("crate_in", 0.22, -0.74, 0.04, 0.78, -0.34, 0.11, "bark", bevel=0.006)
    for x0 in (0.18, 0.80):
        S.box("rim", x0, -0.78, 0.10, x0 + 0.02, -0.30, 0.115, "wood", bevel=0.004)
    for y in (-0.42, -0.54, -0.66):
        _molotov_body(0.44, y, z=0.04, s=0.9)
    return S.render_asset("item", "molotov_cocktails", zmax=0.2, extra={"item": "molotov cocktails",
                                                                        "stack": True})


def _emp_pile():
    return _pile(_emp_body, "emp_grenades", "EMP grenades",
                 [(0.40, -0.64, 10), (0.64, -0.56, -70), (0.42, -0.40, 150)])


def _only():
    v = os.environ.get("RWG_ARMS", "").strip()
    return {x for x in v.split(",") if x} or None


def build():
    only = _only()
    out = []
    for key, kind, fn, zmax, yaw in WEAPONS:
        if only and key not in only:
            continue
        out.append(_held(key, fn, zmax))
        if kind is not None:
            out.append(_ground(key, kind, fn, zmax, yaw))
    if not only or "piles" in only:
        out += [_frag_pile(), _molotov_pile(), _emp_pile()]
    return out
