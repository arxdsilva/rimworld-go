"""Fish farming family (FISH-*; ART spec §12-§16). Our own design: nothing
traced or reconstructed; vanilla has no fish tank to imitate.

    make art FAMILIES=aqua

- furniture/fish_tank_<facing> (1x4, rotatable, authored south like the
  hydroponics basin): a low ribbed steel tub on skids, open water across
  three bays split by straps, a copper return pipe down one rim, an air
  line and two bubblers down the other, a feed hopper on one end and a
  motor-topped aeration pump on the other, with a status lamp.
- furniture/fish_tank_glow_<facing>: the layer drawn while powered: the
  water lifted by a translucent wash, the status lamp at full strength.
  The bubbles and pipes hold it out, so they stay crisp.
- furniture/fish_tank_fish_<1|2|3>_<facing>: the stock under the water at
  three growth stages (fry, half grown, ripe), on the tank's canvas and
  origin, no outline. The tank's rim, straps, pipes and pump hold the fish
  out, so a fish passes under them. Draw it over the tank and its glow.
- item/fish: a ground stack of whole raw fish (`item:` is the economy kind).
- ui_icon/fish_tank: cut from item/fish, for the Architect card.
"""
import math
import random

import numpy as np

from .. import stage as S
from . import overlay as O
from . import power as P
from .item import _ell

FACINGS = ("south", "east", "north", "west")
FOOTPRINT = (1, 4)
ZMAX = 0.84         # top of the pump's motor; shared by every layer's canvas
RIM_Z = 0.44        # top of the tub's walls
WATER_Z = 0.37      # the water surface
WATER_X = (0.11, 0.89)
WATER_Y = (-3.4, -0.11)
LAMP = (0.82, -3.86, 0.6)
WASH_ALPHA = 0.26   # how strongly the powered glow lifts the water
STOCK_ALPHA = 0.84  # how much water shows through a fish
LAMP_R = 0.13       # tiles round the lamp kept at full strength


# --------------------------------------------------------------- the tank
def _tub():
    """Skids, sills and the ribbed steel walls."""
    for x in (0.12, 0.88):
        S.box("skid", x - 0.05, -3.97, 0.0, x + 0.05, -0.03, 0.08, "metal_dark", bevel=0.012)
    for y in (-0.3, -1.3, -2.4, -3.4):
        S.box("sill", 0.06, y - 0.05, 0.03, 0.94, y + 0.05, 0.1, "metal_dark", bevel=0.008)
    S.box("wall_w", 0.05, -3.46, 0.08, 0.11, -0.05, RIM_Z, "steel", bevel=0.012)
    S.box("wall_e", 0.89, -3.46, 0.08, 0.95, -0.05, RIM_Z, "steel", bevel=0.012)
    S.box("wall_n", 0.05, -0.11, 0.08, 0.95, -0.05, RIM_Z, "steel", bevel=0.012)
    S.box("wall_s", 0.05, -3.46, 0.08, 0.95, -3.4, RIM_Z, "steel", bevel=0.012)
    for k in range(7):   # upright ribs, so the long side reads as a framed tank
        y = -0.25 - k * 0.5
        for x0, x1 in ((0.035, 0.055), (0.945, 0.965)):
            S.box("rib", x0, y - 0.03, 0.08, x1, y + 0.03, RIM_Z - 0.02, "steel_dark", bevel=0.0)
    for x0, x1 in ((0.12, 0.3), (0.42, 0.58), (0.7, 0.88)):   # the same on the north end
        S.box("rib_n", x0, -0.05, 0.08, x1, -0.035, RIM_Z - 0.02, "steel_dark", bevel=0.0)
    for y in (-1.2, -2.3):   # straps over the water: three bays
        S.box("strap", 0.04, y - 0.035, RIM_Z - 0.02, 0.96, y + 0.035, RIM_Z + 0.022, "steel_dark", bevel=0.006)


def _water():
    """The water, marked as glowing, its static ripples and the bubblers."""
    wx0, wx1 = WATER_X
    wy0, wy1 = WATER_Y
    P.glow(S.box("water", wx0, wy0, 0.1, wx1, wy1, WATER_Z, "water_deep", bevel=0.0), "indicator")
    for k, (x, y, rx, ry) in enumerate(((0.62, -0.7, 0.14, 0.024), (0.36, -1.72, 0.17, 0.022),
                                        (0.68, -2.0, 0.1, 0.02), (0.56, -2.92, 0.15, 0.022))):
        S.ellipsoid("ripple%d" % k, x, y, WATER_Z, rx, ry, 0.004, "water_shallow", segs=14, rings=4)
    for k, y in enumerate((-0.68, -2.72)):   # an air stone, its boil of bubbles
        rnd = random.Random(7100 + k)
        S.ellipsoid("boil%d" % k, 0.22, y, WATER_Z, 0.085, 0.135, 0.004, "water_shallow", segs=16, rings=4)
        for j in range(11):
            a = rnd.uniform(0, 2 * math.pi)
            d = math.sqrt(rnd.random())
            r = rnd.uniform(0.012, 0.026)
            S.ellipsoid("bub%d_%d" % (k, j), 0.22 + math.cos(a) * d * 0.07, y + math.sin(a) * d * 0.115,
                        WATER_Z + 0.004, r, r, 0.006, "water_foam", segs=8, rings=4)


def _plumbing():
    """Copper return along the east rim, spouting back into the tank; a dark
    air line along the west rim, dropping to the two bubblers."""
    z = RIM_Z + 0.03
    P._rod("return", (0.92, -3.5, z), (0.92, -0.2, z), 0.03, "copper", segs=10, bevel=0.0)
    for y in (-0.6, -1.75, -2.85):
        P._rod("spout", (0.92, y, z), (0.8, y, z - 0.015), 0.024, "copper", segs=8, bevel=0.0)
        S.cyl("collar", 0.92, y, 0.04, z - 0.04, z + 0.04, "steel_dark", segs=10, bevel=0.0)
    P._rod("air", (0.08, -3.5, RIM_Z + 0.02), (0.08, -0.6, RIM_Z + 0.02), 0.018, "metal_dark", segs=8, bevel=0.0)
    for y in (-0.68, -2.72):
        P._rod("drop", (0.08, y, RIM_Z + 0.02), (0.2, y, WATER_Z + 0.01), 0.014, "metal_dark", segs=8, bevel=0.0)


def _pump():
    """South end: housing with a slatted vent, a finned motor lying on top."""
    S.box("housing", 0.05, -3.96, 0.08, 0.95, -3.46, 0.56, "steel_dark", bevel=0.02)
    S.box("vent", 0.18, -3.965, 0.17, 0.68, -3.955, 0.47, "metal_dark", bevel=0.0)
    for k in range(4):
        z = 0.2 + k * 0.07
        S.box("slat", 0.2, -3.972, z, 0.66, -3.963, z + 0.03, "steel", bevel=0.0)
    S.box("cradle", 0.16, -3.84, 0.56, 0.66, -3.56, 0.62, "metal_dark", bevel=0.01)
    P._rod("motor", (0.16, -3.7, 0.7), (0.66, -3.7, 0.7), 0.13, "steel", segs=18, bevel=0.0)
    for k in range(6):
        x = 0.22 + k * 0.08
        P._rod("fin", (x, -3.7, 0.7), (x + 0.025, -3.7, 0.7), 0.14, "metal_dark", segs=18, bevel=0.0)
    S.cyl("port", 0.92, -3.5, 0.045, 0.44, 0.53, "copper", segs=10, bevel=0.0)
    S.cyl("airport", 0.08, -3.5, 0.03, 0.44, 0.5, "metal_dark", segs=10, bevel=0.0)
    P.glow(S.cyl("status", LAMP[0], LAMP[1], 0.05, 0.56, LAMP[2], "backdrop", segs=12, bevel=0.0), "indicator")


def _hopper():
    """North end: a feed hopper straddling the wall, open throat above water."""
    S.prism("hopper", [(0.36, -0.26), (0.72, -0.26), (0.76, 0.0), (0.32, 0.0)], RIM_Z - 0.04, RIM_Z + 0.16, "steel",
            bevel=0.012)
    S.box("hopper_lid", 0.34, -0.2, RIM_Z + 0.16, 0.74, -0.01, RIM_Z + 0.185, "steel_dark", bevel=0.006)


def _tank():
    _tub()
    _water()
    _plumbing()
    _pump()
    _hopper()
    return FOOTPRINT, ZMAX


def _render_tank(f):
    out = P.render("fish_tank_" + f, _tank, facing=f, family="furniture", extra={"building": "fish tank"})
    _soften_wash(out[1], f)
    return out


def _soften_wash(path, facing):
    """The wash is the water's light, not a painted sheet: only the lamp and
    its halo stay at full strength."""
    img = S._read_png(path)
    lx, ly = P.project(P.turn(LAMP, facing, FOOTPRINT), ZMAX)
    h, w = img.shape[:2]
    ys, xs = np.mgrid[0:h, 0:w]
    d = np.hypot(xs - lx, ys - ly) / (LAMP_R * 128.0)
    keep = np.clip(1.5 - d, 0.0, 1.0)
    img[..., 3] *= WASH_ALPHA + (1.0 - WASH_ALPHA) * keep
    S.write_png(img, path)


# ---------------------------------------------------------------- the stock
def _rotated(pts, x, y, yaw):
    c, s = math.cos(yaw), math.sin(yaw)
    return [(x + px * c - py * s, y + px * s + py * c) for px, py in pts]


def _spindle(length, width, n=9):
    """A fish's outline from above, nose at +x, counter-clockwise."""
    half = length / 2
    upper = [(half - length * k / (n - 1),
              width / 2 * math.sin(math.pi * (0.06 + 0.88 * k / (n - 1))) ** 0.8) for k in range(n)]
    return upper + [(px, -py) for px, py in reversed(upper)]


def _swimmer(name, x, y, yaw, length, width, z=WATER_Z + 0.006):
    """A flat dark fish: body, forked tail and, when big enough, two fins."""
    S.prism(name, _rotated(_spindle(length, width), x, y, yaw), z, z + 0.002, "backdrop", bevel=0.0)
    root = -length * 0.47
    t = length * 0.2
    w = width * 0.62
    for side in (1, -1):
        pts = [(root + t * 0.25, 0.0), (root - t, side * w), (root - t * 0.5, 0.0)]
        if side < 0:
            pts.reverse()
        S.prism(name + "_tail", _rotated(pts, x, y, yaw), z, z + 0.002, "backdrop", bevel=0.0)
    if length > 0.2:
        for side in (1, -1):
            f = length * 0.12
            pts = [(f, side * width * 0.42), (-f * 0.6, side * width * 0.42), (-f, side * width * 0.68)]
            if side > 0:
                pts.reverse()
            S.prism(name + "_fin", _rotated(pts, x, y, yaw), z, z + 0.002, "backdrop", bevel=0.0)


# (x, y, heading in degrees from east) per stage; hand laid so no tail is cut
# off by a strap, no fish sits on a bubbler and the stock never reads as a grid.
_RIPE = ((0.66, -0.6, 100), (0.42, -0.92, 272), (0.38, -1.7, 266), (0.68, -1.86, 84), (0.42, -3.02, 94),
         (0.68, -2.72, 264))
_HALF = ((0.62, -0.46, 96), (0.46, -0.86, 262), (0.74, -0.94, 82), (0.36, -1.52, 105), (0.66, -1.58, 275),
         (0.44, -2.02, 80), (0.7, -2.02, 258), (0.55, -2.66, 100), (0.38, -3.1, 280), (0.72, -3.12, 85))
_SCHOOLS = ((0.6, -0.72, 0.17, 0.3, 88, 8), (0.45, -1.8, 0.2, 0.33, 262, 10), (0.6, -2.85, 0.16, 0.28, 110, 7))
LENGTHS = {2: (0.28, 0.08), 3: (0.46, 0.135)}
FRY = (0.13, 0.038)


def _fry():
    rnd = random.Random(7300)
    k = 0
    for cx, cy, rx, ry, yaw, n in _SCHOOLS:
        for _ in range(n):
            a = rnd.uniform(0, 2 * math.pi)
            d = math.sqrt(rnd.random())
            _swimmer("fry%d" % k, cx + math.cos(a) * d * rx, cy + math.sin(a) * d * ry,
                     math.radians(yaw + rnd.uniform(-16, 16)), FRY[0], FRY[1])
            k += 1


def _stock(stage):
    if stage == 1:
        _fry()
        return
    rnd = random.Random(7400 + stage)
    length, width = LENGTHS[stage]
    for k, (x, y, yaw) in enumerate(_RIPE if stage == 3 else _HALF):
        s = rnd.uniform(0.92, 1.06)
        _swimmer("fish%d" % k, x, y, math.radians(yaw), length * s, width * s)


def _render_stock(stage, f):
    """Tank as an occluder, then the stock, on the tank's own canvas."""
    _tank()
    S.holdout(S.objects())
    _stock(stage)
    fp = S.face(S.objects(), f, FOOTPRINT)
    path = S.render_asset("furniture", "fish_tank_fish_%d_%s" % (stage, f), footprint=fp, zmax=ZMAX, outline=False,
                          tint=False, extra={"layer": "stock", "of": "furniture/fish_tank_" + f, "stage": stage,
                                             "rotatable": True, "facing": f})
    img = S._read_png(path)   # under water: the water tints them
    img[..., 3] *= STOCK_ALPHA
    S.write_png(img, path)
    return path


# ----------------------------------------------------------------- the item
def _whole_fish(name, x, y, z, yaw_deg, s=1.0):
    """One whole raw fish lying on its side: silver flank, dark back, pale
    belly, forked tail, dorsal fin, red gill slit and a round eye."""
    yaw = math.radians(yaw_deg)

    def at(px, py):
        return _rotated([(px * s, py * s)], x, y, yaw)[0]

    def fin(n, pts, z0, z1, mat):
        S.prism(n, _rotated([(px * s, py * s) for px, py in pts], x, y, yaw), z0, z1, mat, bevel=0.0)

    tail = [(-0.27, 0.0), (-0.42, 0.11), (-0.37, 0.0)]
    fin(name + "_tail0", tail, z + 0.012, z + 0.034, "fur_steel")
    fin(name + "_tail1", [(px, -py) for px, py in reversed(tail)], z + 0.012, z + 0.034, "fur_steel")
    fin(name + "_dorsal", [(0.1, 0.07), (0.05, 0.13), (-0.13, 0.11), (-0.16, 0.06)], z + 0.012, z + 0.03, "fur_steel")
    fin(name + "_anal", [(-0.14, -0.06), (-0.17, -0.1), (-0.07, -0.1), (-0.04, -0.07)], z + 0.012, z + 0.03,
        "fur_steel")
    _ell(name + "_body", (x, y, z + 0.03), (0.3 * s, 0.095 * s, 0.05 * s), "silver", yaw=yaw_deg, segs=20, rings=10)
    cx, cy = at(-0.01, 0.038)
    _ell(name + "_back", (cx, cy, z + 0.05), (0.26 * s, 0.05 * s, 0.038 * s), "fur_steel", yaw=yaw_deg, segs=16,
         rings=8)
    cx, cy = at(0.135, 0.0)
    _ell(name + "_gill", (cx, cy, z + 0.062), (0.012 * s, 0.062 * s, 0.018 * s), "food_red", yaw=yaw_deg, segs=10,
         rings=5)
    cx, cy = at(0.215, 0.018)
    _ell(name + "_iris", (cx, cy, z + 0.066), (0.024 * s, 0.024 * s, 0.012 * s), "ceramic", segs=10, rings=5)
    _ell(name + "_eye", (cx, cy, z + 0.074), (0.014 * s, 0.014 * s, 0.008 * s), "backdrop", segs=8, rings=4)


def _fish_item():
    """Three whole fish, heads and tails alternating, the top one across."""
    _whole_fish("a", 0.51, -0.68, 0.0, 6, s=0.86)
    _whole_fish("b", 0.49, -0.44, 0.0, 186, s=0.86)
    _whole_fish("c", 0.5, -0.56, 0.06, 12, s=0.92)
    return S.render_asset("item", "fish", zmax=0.22, extra={"item": "fish", "stack": True})


# -------------------------------------------------------------------- build
def build(only=None):
    """Every fish-farming sprite; `only` (any of "tank", "stock", "item")
    renders just those groups."""
    out = []
    want = (lambda g: True) if not only else (lambda g: g in only)
    if want("tank"):
        for f in FACINGS:
            out += _render_tank(f)
    if want("stock"):
        for f in FACINGS:
            for stage in (1, 2, 3):
                out.append(_render_stock(stage, f))
    if want("item"):
        out.append(_fish_item())
        p = O._icon("fish_tank", "item", "fish")
        if p:
            out.append(p)
    return out
