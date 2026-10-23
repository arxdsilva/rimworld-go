"""Natural rock walls (ART spec §9): connect visually, read as solid, and
stay distinct from constructed walls: lumpy tops, no rims. Autotile mask bits
N=1, E=2, S=4, W=8 plus the corner model in linked.py (47 variants each).

Families: rock_wall (rough), rock_smooth (chiselled flat top), ore_steel /
ore_silver / ore_gold (rough rock in the stone_tint material, the only tint
group, with embedded metal flecks). Stone and stone_dark are the
tint-neutral palette bands; the renderer multiplies the stone colour.
"""
import random

from .. import stage as S
from .linked import _variants, relevant, corner_seed

ROCK_H = 1.05
EXT = 0.22
NOTCH = 0.03
BITS = (("N", 1), ("E", 2), ("S", 4), ("W", 8))
CORNER_XY = {"NE": (1, 1), "SE": (1, -1), "SW": (-1, -1), "NW": (-1, 1)}


def _arms(con):
    x0 = -EXT if "W" in con else NOTCH
    x1 = 1 + EXT if "E" in con else 1 - NOTCH
    y1 = EXT if "N" in con else -NOTCH
    y0 = -1 - EXT if "S" in con else -1 + NOTCH
    return x0, y0, x1, y1


def _body(con, filled, rel, mat, bevel, height=0.55):
    """Plus of two boxes; filled relevant corners extend the body to the tile corner."""
    x0, y0, x1, y1 = _arms(con)
    if con & {"E", "W"} or not con & {"N", "S"}:
        S.box("body", x0, -1 + NOTCH, 0.0, x1, -NOTCH, height, mat, bevel=bevel, segments=2)
    if con & {"N", "S"}:
        S.box("body_v", NOTCH, y0, 0.0, 1 - NOTCH, y1, height, mat, bevel=bevel, segments=2)
    for name in rel:
        if name in filled:
            sx, sy = CORNER_XY[name]
            xr = (1 - 0.1, 1 + EXT) if sx > 0 else (-EXT, 0.1)
            yr = (-0.1, EXT) if sy > 0 else (-1 - EXT, -1 + 0.1)
            S.box("corner_" + name, xr[0], yr[0], 0.0, xr[1], yr[1], height - 0.002, mat, bevel=bevel, segments=2)


def _rock(mask, filled, mats=("stone_neutral", "stone_neutral_dark")):
    con = {d for d, b in BITS if mask & b}
    rel = relevant(mask)
    light, dark = mats
    _body(con, filled, rel, dark, 0.08)
    # Interior lumps carry the volume: count, size and position vary with (mask, corners) so
    # neighbouring tiles differ, and they stay fully inside the tile (never touch a linked edge).
    cs = corner_seed(filled) * 1009
    rng = random.Random(mask * 131 + cs + 7)
    n = 7 + rng.randrange(3)
    for i in range(n):
        r = 0.17 + 0.09 * rng.random()
        lo, hi = 0.04 + r, 0.96 - r
        cx, cy = lo + (hi - lo) * rng.random(), -(lo + (hi - lo) * rng.random())
        S.blob("lump%d" % i, cx, cy, 0.50, r, light if i % 2 == 0 else dark,
               seed=mask * 17 + i + cs, lumps=0.32, squash=1.0)
    r = 0.24 + 0.06 * rng.random()
    S.blob("crown", 0.46 + 0.08 * rng.random(), -0.46 - 0.08 * rng.random(), 0.60, r, light,
           seed=mask * 31 + 5 + cs, lumps=0.28, squash=0.9)
    # Lumps on a linked edge sit exactly ON it, low and flat, two per edge at fixed positions with
    # fixed seeds per axis/position (independent of mask): both clipped halves are the same blob
    # so they meet seamlessly.
    edge = {"N": [(0.28, 0.0, 911, dark), (0.74, 0.0, 912, light)],
            "S": [(0.28, -1.0, 911, dark), (0.74, -1.0, 912, light)],
            "E": [(1.0, -0.28, 913, light), (1.0, -0.74, 914, dark)],
            "W": [(0.0, -0.28, 913, light), (0.0, -0.74, 914, dark)]}
    for d in sorted(con):
        for k, (ex, ey, seed, m) in enumerate(edge[d]):
            S.blob("edge_%s%d" % (d, k), ex, ey, 0.44, 0.19, m, seed=seed, lumps=0.30, squash=0.75)
    for name in rel:
        if name in filled:   # centred on the shared tile corner, one fixed seed
            sx, sy = CORNER_XY[name]
            S.blob("clump_" + name, 1.0 if sx > 0 else 0.0, 0.0 if sy > 0 else -1.0, 0.42, 0.18, light,
                   seed=903, lumps=0.30, squash=0.7)
    return con


# (x, y, z) of flecks sitting on the visible top (crown, lumps), below the clip height
FLECK_SPOTS = ((0.50, -0.58, 0.80), (0.40, -0.40, 0.76), (0.62, -0.46, 0.80), (0.30, -0.62, 0.62),
               (0.70, -0.66, 0.62), (0.34, -0.30, 0.68), (0.68, -0.30, 0.66))


def _ore(mask, filled, metal):
    con = _rock(mask, filled, mats=("stone_tint", "stone_tint"))
    for i, (x, y, z) in enumerate(FLECK_SPOTS):
        S.ellipsoid("fleckr%d" % i, x, y, z - 0.03, 0.13, 0.11, 0.05, "metal_dark", segs=10, rings=6)
        S.ellipsoid("fleck%d" % i, x, y - 0.02, z + 0.03, 0.10, 0.085, 0.10, metal, segs=10, rings=6)
    return con


SMOOTH_H = 0.62


def _smooth(mask, filled):
    """Flat chiselled top, crisp bevel, subtle block lines (no lumps)."""
    con = {d for d, b in BITS if mask & b}
    rel = relevant(mask)
    top = 0.55
    _body(con, filled, rel, "stone_neutral_dark", 0.03, height=top)
    x0, y0, x1, y1 = _arms(con)
    # raised chiselled plate on the top, inset so the bevel stays crisp
    ins = 0.07
    px0 = x0 if "W" in con else NOTCH + ins
    px1 = x1 if "E" in con else 1 - NOTCH - ins
    py1 = y1 if "N" in con else -NOTCH - ins
    py0 = y0 if "S" in con else -1 + NOTCH + ins
    if con & {"E", "W"} or not con & {"N", "S"}:
        S.box("plate", px0, -1 + NOTCH + ins, top - 0.01, px1, -NOTCH - ins, top + 0.04, "stone_neutral", bevel=0.015)
    if con & {"N", "S"}:
        S.box("plate_v", NOTCH + ins, py0, top - 0.01, 1 - NOTCH - ins, py1, top + 0.04, "stone_neutral", bevel=0.015)
    for name in rel:
        if name in filled:
            sx, sy = CORNER_XY[name]
            xr = (1 - 0.15, 1 + EXT) if sx > 0 else (-EXT, 0.15)
            yr = (-0.15, EXT) if sy > 0 else (-1 - EXT, -1 + 0.15)
            S.box("plate_c" + name, xr[0], yr[0], top - 0.01, xr[1], yr[1], top + 0.04, "stone_neutral", bevel=0.015)
    # block lines on the south face (grid lines every half tile, subtle)
    if "S" not in con:
        for z in (0.27,):
            S.box("course", x0 + 0.02, y0 - 0.008, z, x1 - 0.02, y0 + 0.01, z + 0.02, "stone_neutral_dark", bevel=0.0)
        for xs in (0.5,):
            if x0 < xs < x1:
                S.box("bjoint", xs - 0.01, y0 - 0.008, 0.0, xs + 0.01, y0 + 0.01, top, "stone_neutral_dark", bevel=0.0)
    return con


# Buried machinery (NG-1 §4.8): an ore whose rock body is the tinted stone and
# whose flecks are wreckage of an older age: a toothed gear, a copper pipe elbow
# and a riveted steel plate, all lying on the visible top below the clip height.
def _gear_top(cx, cy, z, r, teeth=9):
    import math
    S.cyl("gear", cx, cy, r, z, z + 0.06, "steel", segs=teeth * 2, bevel=0.008)
    S.cyl("gear_hole", cx, cy, r * 0.36, z + 0.055, z + 0.075, "metal_dark", segs=10, bevel=0.0)
    for k in range(teeth):
        a = 2 * math.pi * k / teeth
        S.box("tooth%d" % k, cx + math.cos(a) * r * 1.03 - 0.022, cy + math.sin(a) * r * 1.03 - 0.022, z,
              cx + math.cos(a) * r * 1.03 + 0.022, cy + math.sin(a) * r * 1.03 + 0.022, z + 0.06, "steel",
              bevel=0.004)


def _machinery(mask, filled):
    con = _rock(mask, filled, mats=("stone_tint", "stone_tint"))
    _gear_top(0.50, -0.56, 0.86, 0.17)
    S.box("pipe", 0.16, -0.38, 0.84, 0.50, -0.30, 0.92, "copper", bevel=0.02)
    S.cyl("pipe_up", 0.50, -0.34, 0.05, 0.84, 0.97, "copper", segs=10, bevel=0.01)
    S.cyl("pipe_cap", 0.50, -0.34, 0.065, 0.95, 0.99, "metal_dark", segs=10, bevel=0.005)
    S.box("plate", 0.58, -0.76, 0.82, 0.84, -0.58, 0.88, "steel", bevel=0.01)
    for k, (x, y) in enumerate(((0.61, -0.73), (0.81, -0.73), (0.61, -0.61), (0.81, -0.61))):
        S.cyl("rivet%d" % k, x, y, 0.016, 0.88, 0.91, "metal_dark", segs=6, bevel=0.0)
    S.ellipsoid("cog_small", 0.28, -0.70, 0.84, 0.08, 0.08, 0.035, "metal_dark", segs=10, rings=6)
    return con


def _cave_mouth(facing):
    """A cave entrance in tint-neutral stone: a rock horseshoe round a dark opening,
    open to `facing`. Authored facing south, rotated by stage.face()."""
    L, D = "stone_neutral", "stone_neutral_dark"
    S.box("back", 0.04, -0.46, 0.0, 0.96, -0.04, 0.56, D, bevel=0.07, segments=2)
    S.box("arm_w", 0.04, -0.96, 0.0, 0.28, -0.30, 0.36, D, bevel=0.07, segments=2)
    S.box("arm_e", 0.72, -0.96, 0.0, 0.96, -0.30, 0.36, D, bevel=0.07, segments=2)
    S.box("lintel", 0.22, -0.52, 0.34, 0.78, -0.34, 0.52, L, bevel=0.06, segments=2)
    S.box("dark_face", 0.28, -0.465, 0.0, 0.72, -0.44, 0.34, "eye_black", bevel=0.0)
    S.box("dark_floor", 0.28, -0.97, 0.0, 0.72, -0.44, 0.012, "eye_black", bevel=0.0)
    S.box("dark_floor2", 0.31, -0.99, 0.0, 0.69, -0.9, 0.014, "backdrop", bevel=0.0)
    for k, (x, y, z, r, sd) in enumerate(((0.30, -0.22, 0.46, 0.21, 41), (0.70, -0.24, 0.46, 0.20, 42),
                                           (0.50, -0.14, 0.52, 0.19, 43), (0.16, -0.58, 0.28, 0.14, 44),
                                           (0.84, -0.60, 0.28, 0.14, 45), (0.18, -0.84, 0.18, 0.12, 46),
                                           (0.82, -0.84, 0.18, 0.12, 47))):
        S.blob("lump%d" % k, x, y, z, r, L if k % 2 == 0 else D, seed=sd, lumps=0.3, squash=0.9)
    for k, (x, y, r, sd) in enumerate(((0.34, -0.93, 0.07, 51), (0.62, -0.95, 0.06, 52), (0.50, -0.90, 0.04, 53))):
        S.blob("rubble%d" % k, x, y, 0.0, r, L, seed=sd, lumps=0.3, squash=0.6)
    fp = S.face(S.objects(), facing, (1, 1))
    return S.render_asset("rock", "cave_mouth_" + facing, footprint=fp, zmax=0.65,
                          extra={"overlay": "cave_mouth", "facing": facing, "rotatable": True, "material": "stone"})


def _emit(name, builder, zmax, zclip, material, extra=None):
    out = []
    for mask in range(16):
        for filled, suffix in _variants(mask):
            con = builder(mask, filled)
            bits = corner_seed(filled)
            ex = {"autotile": "rock", "mask": mask, "corners": bits, "material": material}
            ex.update(extra or {})
            out.append(S.render_asset("rock", "%s_%02d%s" % (name, mask, suffix), zmax=zmax, clip=tuple(con),
                                      zclip=zclip, extra=ex))
    return out


def build():
    out = _emit("rock_wall", _rock, ROCK_H, 0.80, "stone")
    out += _emit("rock_smooth", _smooth, SMOOTH_H, 0.60, "stone")
    for ore, metal in (("steel", "steel"), ("silver", "silver"), ("gold", "gold")):
        out += _emit("ore_" + ore, lambda m, f, metal=metal: _ore(m, f, metal), ROCK_H, 0.80, "stone",
                     {"ore": ore})
    out += build_ng1()
    return out


def build_ng1():
    """Buried machinery ore (same linked set as the metal ores) and the cave mouths."""
    out = _emit("ore_machinery", _machinery, ROCK_H, 0.80, "stone", {"ore": "machinery"})
    out += [_cave_mouth(f) for f in ("south", "east", "north", "west")]
    return out
