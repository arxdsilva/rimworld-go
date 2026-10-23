"""Plants (ART spec §9): silhouette and growth state first.

Trees (oak, birch, pine) are built from layered leaf-cluster blobs over a
trunk with bark ridges and a root flare; oak and birch also render
`_autumn` and `_bare` (winter) looks. Bushes, berry bushes, grass tufts
and eight crop species with six growth stages x three variants complete
the set. Origin is always the ground point (trunk base / tuft base /
mound centre) so the renderer can bend plants from the root. Variation
is authored per variant index, never RNG at runtime (all RNG is seeded).

Old ids (`tree_00`, `bush_00_harvestable`, `crop_sown`, `corn_mature`...)
are rendered as aliases of the new geometry until the renderer switches.
"""
import math
import random

import bmesh
import numpy as np
from mathutils import Matrix, Vector

from .. import stage as S
from . import trees
from .plant_base import CX, CY, _Z, _asset, _ball, _blade, _cluster, _leaf, _limb, _top
from .trees import _tree  # noqa: F401  (scenes/menu.py calls plant._tree)

BUSH_M = (0.2, 0.2, 0.12, 0.15)


def _bush_geo(i):
    if i == 0:  # round and full
        _cluster("b0", CX, CY, 0.0, 0.34, "foliage", 41)
        _cluster("b1", CX - 0.2, CY + 0.08, 0.04, 0.2, "vegetation", 42)
        _cluster("b2", CX + 0.18, CY - 0.1, 0.06, 0.2, "foliage", 43)
        _cluster("b3", CX, CY + 0.02, 0.22, 0.18, "vegetation", 44)
    elif i == 1:  # wide and low
        for k, (dx, r) in enumerate([(-0.3, 0.2), (-0.1, 0.25), (0.12, 0.24), (0.32, 0.18)]):
            _cluster("b%d" % k, CX + dx, CY + 0.02 * (-1) ** k, 0.0, r, "foliage" if k % 2 == 0 else "vegetation",
                     50 + k, squash=0.7)
    elif i == 2:  # tall and narrow with visible twigs
        for k in range(3):
            _limb("tw%d" % k, (CX + (k - 1) * 0.05, CY, 0.0), (CX + (k - 1) * 0.12, CY, 0.28), 0.022, 0.012, "bark", 5)
        _cluster("b0", CX, CY, 0.04, 0.26, "foliage", 61, squash=0.95)
        _cluster("b1", CX + 0.04, CY, 0.28, 0.2, "vegetation", 62, squash=0.95)
        _cluster("b2", CX - 0.02, CY + 0.02, 0.48, 0.14, "vegetation", 63, squash=1.0)
    else:  # spiky leaf rosette
        for k in range(9):
            a = 2 * math.pi * k / 9 + 0.2
            _leaf("l%d" % k, CX + math.cos(a) * 0.05, CY + math.sin(a) * 0.05, 0.04, a, 0.75 if k % 2 else 0.5,
                  0.42 if k % 2 else 0.34, 0.14, "leaf_light" if k % 2 else "vegetation", thick=0.04)
        _cluster("b0", CX, CY, 0.04, 0.17, "foliage", 71, squash=1.0)


def _bush(i):
    return _asset(lambda: _bush_geo(i), "bush_%02d" % i, {"species": "bush", "variant": i}, BUSH_M)


def _berry_geo(i, ripe):
    rng = random.Random(800 + i)
    parts = (((0.0, 0.0, 0.32), (-0.2, 0.06, 0.2), (0.2, -0.06, 0.2)),
             ((-0.12, 0.0, 0.26), (0.12, 0.04, 0.26), (0.0, -0.1, 0.22), (0.0, 0.14, 0.2)),
             ((0.0, 0.0, 0.3), (-0.22, -0.04, 0.16), (0.22, 0.02, 0.18), (0.04, 0.0, 0.18)))[i]
    cl = []
    for k, (dx, dy, r) in enumerate(parts):
        z = 0.0 if k == 0 or i == 1 else (0.2 if k == len(parts) - 1 and i == 2 else 0.04)
        _cluster("b%d" % k, CX + dx, CY + dy, z, r, "foliage" if k % 2 == 0 else "vegetation", 810 + 10 * i + k,
                 squash=0.85)
        cl.append((CX + dx, CY + dy, z, r))
    for k in range(9 if ripe else 3):
        x, y, z, r = cl[k % len(cl)]
        a = rng.uniform(0, 2 * math.pi)
        e = rng.uniform(0.25, 1.0)
        px = x + math.cos(a) * math.cos(e) * r * 0.95
        py = y + math.sin(a) * math.cos(e) * r * 0.95
        pz = z + math.sin(e) * r * 0.8
        if ripe:
            _ball("berry%d" % k, px, py, pz, 0.05, 0.05, 0.05, "food_red")
        else:
            _ball("bud%d" % k, px, py, pz, 0.03, 0.03, 0.025, "blossom", segs=7, rings=4)


def _berry(i, ripe):
    name = "berry_bush_%02d%s" % (i, "_harvestable" if ripe else "")
    return _asset(lambda: _berry_geo(i, ripe), name,
                  {"species": "berry_bush", "variant": i, "harvestable": ripe}, BUSH_M)


# ------------------------------------------------------------------ grass
GRASS_M = (0.15, 0.15, 0.1, 0.12)


def _grass_geo(i):
    rng = random.Random(70 + 3 * i)
    n = (3, 4, 5, 5, 6, 6, 7, 7)[i]
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        rr = rng.uniform(0.02, 0.14)
        _blade("g%d" % k, CX + math.cos(a) * rr, CY + math.sin(a) * rr, rng.uniform(0.26, 0.45), a,
               rng.uniform(0.06, 0.18), ("vegetation", "foliage", "leaf_light")[(k + i) % 3])
    flower = {2: "blossom", 5: "blossom", 3: "corn_yellow", 7: "corn_yellow"}.get(i)
    if flower:
        for k in range(2 if i < 4 else 3):
            a = 1.1 + 2.1 * k + i
            x, y = CX + math.cos(a) * 0.1, CY + math.sin(a) * 0.1
            h = 0.3 + 0.05 * k
            _limb("stem%d" % k, (x, y, 0.0), (x + 0.02, y, h), 0.012, 0.008, "vegetation", segs=4)
            _ball("fl%d" % k, x + 0.02, y, h + 0.01, 0.045, 0.045, 0.034, flower)
            _ball("fc%d" % k, x + 0.02, y, h + 0.03, 0.014, 0.014, 0.01, "corn_yellow" if flower == "blossom"
                  else "tuber", segs=6, rings=4)
    if i in (4, 1):  # clover patch at the foot of the tuft
        for k in range(3):
            a = 2 * math.pi * k / 3 + 0.5
            _ball("cl%d" % k, CX + math.cos(a) * 0.15, CY + math.sin(a) * 0.12 - 0.04, 0.03, 0.055, 0.045, 0.014,
                  "leaf_light", segs=8, rings=4)
        if i == 4:
            _ball("clf", CX + 0.15, CY - 0.05, 0.07, 0.025, 0.025, 0.02, "blossom", segs=7, rings=4)


def _grass(i):
    return _asset(lambda: _grass_geo(i), "grass_%02d" % i, {"species": "grass", "variant": i, "decoration": True},
                  GRASS_M, outline=True, shadow=False)


def _grass_tall_geo(i):
    rng = random.Random(170 + 5 * i)
    n = (9, 10, 8, 11)[i]
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        rr = rng.uniform(0.02, 0.2)
        x, y = CX + math.cos(a) * rr, CY + math.sin(a) * rr
        h = rng.uniform(0.7, 1.05)
        _blade("t%d" % k, x, y, h, a, rng.uniform(0.08, 0.2), ("vegetation", "foliage", "leaf_light")[(k + i) % 3],
               r=0.034)
        if i in (1, 3) and k % 3 == 0:  # seed heads
            _ball("seed%d" % k, x + math.cos(a) * 0.17, y + math.sin(a) * 0.17, h + 0.02, 0.02, 0.02, 0.05, "straw",
                  segs=6, rings=4)
    if i == 2:
        for k in range(2):
            _limb("ts%d" % k, (CX + 0.1 * k, CY - 0.05, 0.0), (CX + 0.12 * k, CY - 0.05, 0.42), 0.012, 0.008,
                  "vegetation", segs=4)
            _ball("tf%d" % k, CX + 0.12 * k, CY - 0.05, 0.44, 0.035, 0.035, 0.026, "corn_yellow")


def _grass_tall(i):
    return _asset(lambda: _grass_tall_geo(i), "grass_tall_%02d" % i,
                  {"species": "grass_tall", "variant": i, "decoration": True}, GRASS_M, outline=True, shadow=False)


# ------------------------------------------------------------------ crops
CROP_M = (0.15, 0.15, 0.12, 0.15)
LEAF_Z = 0.07
BROWN = ("tuber", "burlap", "straw")
SEED_MAT = {"radish": "food_red", "lettuce": "leaf_light", "beans": "burlap", "potato": "tuber",
            "corn": "corn_yellow", "wheat": "straw", "pumpkin": "straw", "healroot": "vegetation", "cotton": "ceramic"}


def _mound():
    S.cyl("mound", CX, CY - 0.02, 0.30, 0.0, 0.05, "soil_rich", segs=20, r_top=0.24, bevel=0.0)
    _top(0.05)


def _ring(n, length, width, pitch, mats, rot, rng, z=LEAF_Z, r0=0.03, thick=0.03, jitter=0.15, x=CX, y=CY):
    for k in range(n):
        a = 2 * math.pi * k / n + rot + rng.uniform(-0.15, 0.15)
        _leaf("lf%d" % k, x + math.cos(a) * r0, y + math.sin(a) * r0, z + 0.015 * (k % 2), a,
              pitch + rng.uniform(-jitter, jitter), length * rng.uniform(1 - jitter, 1 + jitter), width,
              mats[k % len(mats)], thick=thick)


def _sown(species, v):
    _mound()
    ang = (0.0, 0.7, -0.5)[v]
    dx, dy = math.cos(ang), math.sin(ang)
    for s in (-1, 1):  # two lighter ridges flanking the furrow
        ox, oy = -dy * 0.075 * s, dx * 0.075 * s
        _limb("ridge%d" % s, (CX + ox - dx * 0.2, CY + oy - dy * 0.2, 0.075),
              (CX + ox + dx * 0.2, CY + oy + dy * 0.2, 0.075), 0.034, 0.034, "soil", segs=6)
    n = (3, 4, 5)[v]
    for k in range(n):
        t = -0.17 + 0.34 * k / max(n - 1, 1)
        _ball("seed%d" % k, CX + dx * t, CY + dy * t, 0.075, 0.03, 0.022, 0.018, SEED_MAT[species], segs=7, rings=4)


def _sprout_leaves(v, mat="vegetation", n=None, length=0.1, pitch=0.7, width=0.05):
    rng = random.Random(5 + v)
    _ring(n or (2, 3, 2)[v], length, width, pitch, (mat, "leaf_light"), 0.4 + v, rng, r0=0.015)


def _radish(st, v):
    rng = random.Random(10 + v)
    n = (5, 6, 7)[v]
    L = (0.25, 0.3, 0.23)[v]
    if st == "sprout":
        _sprout_leaves(v, "vegetation", length=0.12)
    elif st == "young":
        _ring(n - 1, L * 0.65, 0.09, 0.5, ("foliage", "vegetation"), v, rng)
    elif st in ("mature", "harvestable"):
        _ring(n, L, 0.11, 0.4 - 0.05 * v, ("foliage", "vegetation"), v, rng)
        if st == "harvestable":
            for k in range(3 + (v == 1)):
                a = 2 * math.pi * k / (3 + (v == 1)) + 0.5 + v
                _ball("root%d" % k, CX + math.cos(a) * 0.1, CY + math.sin(a) * 0.1, 0.075, 0.065, 0.065, 0.055,
                      "food_red", segs=12, rings=8)
    else:
        _ring(n - 2, L * 0.85, 0.08, -0.25, BROWN, v, rng)


def _lettuce(st, v):
    rng = random.Random(20 + v)
    n = (6, 8, 10)[v]
    L = (0.26, 0.3, 0.28)[v]
    if st == "sprout":
        _sprout_leaves(v, "leaf_light", length=0.11, width=0.07)
    elif st == "young":
        _ring(n - 1, L * 0.6, 0.1, 0.45, ("leaf_light", "vegetation"), v, rng)
    elif st == "mature":
        _ring(n, L * 0.9, 0.15, 0.6, ("leaf_light", "vegetation"), v, rng, z=0.07)
        _ball("bud", CX, CY, 0.1, 0.07, 0.07, 0.06, "leaf_light")
    elif st == "harvestable":
        _ring(n, L, 0.17, 0.25, ("vegetation", "foliage"), v, rng)
        s = (1.0, 0.9, 1.08)[v]
        _ball("head", CX, CY, 0.12, 0.17 * s, 0.17 * s, 0.12 * s, "leaf_light", segs=16, rings=10)
        _ball("fold", CX + 0.03, CY - 0.02, 0.2 * s, 0.09 * s, 0.09 * s, 0.06 * s, "vegetation")
    else:
        _ring(n - 2, L * 0.9, 0.12, -0.3, BROWN, v, rng)
        _ball("rot", CX, CY, 0.08, 0.07, 0.07, 0.04, "tuber")


def _healroot(st, v):
    rng = random.Random(30 + v)
    n = (7, 9, 6)[v]
    L = (0.28, 0.32, 0.3)[v]
    if st == "sprout":
        _sprout_leaves(v, "leaf_light", length=0.1, width=0.035, pitch=0.9)
    elif st == "young":
        _ring(n - 1, L * 0.6, 0.045, 0.65, ("leaf_light", "foliage"), v, rng)
    elif st in ("mature", "harvestable"):
        _ring(n, L, 0.055, 0.7 - 0.1 * v, ("leaf_light", "foliage"), v, rng)
        for k in range(2 + v if st == "mature" else 3 + v):
            a = 2 * math.pi * k / 4 + 0.9 + v
            x, y = CX + 0.07 * math.cos(a), CY + 0.07 * math.sin(a)
            h = 0.22 + 0.04 * (k % 2)
            _limb("stalk%d" % k, (x, y, 0.06), (x, y, h), 0.012, 0.008, "vegetation", segs=5)
            if st == "harvestable":
                _ball("fl%d" % k, x, y, h + 0.015, 0.038, 0.038, 0.028, "blossom")
                _ball("fc%d" % k, x, y, h + 0.035, 0.014, 0.014, 0.01, "corn_yellow", segs=6, rings=4)
            else:
                _ball("bud%d" % k, x, y, h + 0.01, 0.02, 0.02, 0.03, "vegetation", segs=6, rings=4)
        if st == "harvestable":
            for k in range(3):
                a = 2 * math.pi * k / 3 + 0.3 + v
                _cluster("tub%d" % k, CX + math.cos(a) * 0.2, CY + math.sin(a) * 0.2, 0.03, 0.085, "tuber",
                         90 + k + v, lumps=0.3, squash=0.7)
    else:
        _ring(n - 2, L * 0.85, 0.045, -0.2, BROWN, v, rng)


def _beans(st, v):
    rng = random.Random(40 + v)
    lean = (0.0, 0.05, -0.05)[v]
    if v:
        _limb("pole", (CX + 0.12, CY + 0.08, 0.05), (CX + 0.12 + lean, CY + 0.08, 0.55 + 0.05 * v), 0.017, 0.013, "wood",
              segs=5)
    if st == "sprout":
        _limb("stem", (CX, CY, 0.05), (CX + lean, CY, 0.14), 0.012, 0.008, "vegetation", segs=4)
        _sprout_leaves(v, "vegetation", length=0.09, width=0.06, pitch=0.3)
    elif st == "young":
        _limb("stem", (CX, CY, 0.05), (CX + lean, CY, 0.24), 0.015, 0.01, "vegetation", segs=5)
        _ring(5 + v, 0.15, 0.08, 0.45, ("vegetation", "foliage"), v, rng, z=0.16, r0=0.02)
    elif st in ("mature", "harvestable"):
        h = (0.3, 0.36, 0.42)[v]
        _cluster("b0", CX, CY, 0.05, 0.25, "foliage", 81 + v, lumps=0.3, squash=0.9)
        _cluster("b1", CX - 0.1, CY + 0.07, 0.1, 0.16, "vegetation", 82 + v, lumps=0.3, squash=0.9)
        _cluster("b2", CX + 0.1, CY - 0.07, h * 0.5, 0.15, "vegetation", 83 + v, lumps=0.3, squash=0.9)
        if v == 2:
            _cluster("b3", CX + 0.1 + lean, CY + 0.08, h * 0.9, 0.12, "foliage", 84, lumps=0.3, squash=0.9)
        if st == "harvestable":
            for k in range(8):
                a = 2 * math.pi * k / 8 + 0.2 + v
                x, y = CX + math.cos(a) * 0.23, CY + math.sin(a) * 0.23
                z = 0.2 + 0.05 * (k % 2)
                _limb("pod%d" % k, (x, y, z), (x + math.cos(a) * 0.03, y + math.sin(a) * 0.03 - 0.02, z - 0.13),
                      0.026, 0.014, "straw", segs=5)
    else:
        _limb("stem", (CX, CY, 0.05), (CX + 0.1, CY - 0.05, 0.2), 0.016, 0.01, "tuber", segs=5)
        _ring(5 + v, 0.15, 0.07, -0.3, BROWN, v, rng, z=0.15, r0=0.02)


def _potato(st, v):
    rng = random.Random(50 + v)
    n = (8, 10, 12)[v]
    if st == "sprout":
        _sprout_leaves(v, "vegetation", length=0.1, width=0.05, pitch=0.5)
    elif st == "young":
        _ring(5, 0.17, 0.08, 0.6, ("vegetation", "foliage"), v, rng, z=0.07)
    elif st in ("mature", "harvestable"):
        _ring(n - 2, 0.22, 0.1, 0.55 + 0.05 * v, ("foliage", "vegetation"), v, rng, z=0.08)
        _ring(n // 2, 0.15, 0.09, 0.95, ("vegetation", "leaf_light"), v + 1, rng, z=0.16, r0=0.02)
        h = 0.3 + 0.04 * v
        for k in range(3):
            a = 2 * math.pi * k / 3 + v
            x, y = CX + 0.07 * math.cos(a), CY + 0.07 * math.sin(a)
            if st == "harvestable":
                _ball("fl%d" % k, x, y, h, 0.04, 0.04, 0.03, "blossom")
                _ball("fc%d" % k, x, y, h + 0.02, 0.015, 0.015, 0.012, "corn_yellow", segs=6, rings=4)
            else:
                _ball("bud%d" % k, x, y, h - 0.04, 0.02, 0.02, 0.03, "vegetation", segs=6, rings=4)
        if st == "harvestable":
            for k, (x, y) in enumerate([(0.26, -0.7), (0.72, -0.72), (0.76, -0.4), (0.3, -0.34)][:3 + (v == 1)]):
                _ball("tuber%d" % k, x, y, 0.05, 0.075, 0.06, 0.05, "tuber", segs=12, rings=8)
    else:
        _ring(n - 3, 0.2, 0.08, -0.3, BROWN, v, rng, z=0.1)


def _corn(st, v):
    rng = random.Random(60 + v)
    offs = {0: [(-0.05, 0.02), (0.1, -0.08)], 1: [(-0.1, 0.04), (0.08, 0.06), (0.02, -0.1)],
            2: [(-0.07, -0.03), (0.1, 0.0)]}[v]
    H = {"sprout": 0.12, "young": 0.38, "mature": (0.9, 0.98, 0.85)[v], "harvestable": (1.0, 1.08, 0.95)[v],
         "wilted": (0.62, 0.7, 0.55)[v]}[st]
    wilt = st == "wilted"
    for s, (ox, oy) in enumerate(offs):
        x, y = CX + ox, CY + oy
        lean = (0.2 if wilt else 0.04) * (1 if s % 2 else -1)
        top = (x + lean, y, 0.05 + H)
        mat = "tuber" if wilt else "vegetation"
        if st == "sprout":
            _sprout_leaves(v + s, "vegetation", n=2, length=0.1, width=0.035, pitch=0.8)
            break
        _limb("stalk%d" % s, (x, y, 0.05), top, 0.035, 0.02, mat, segs=6)
        nl = 3 if st == "young" else 5
        for j in range(nl):
            t = 0.3 + 0.6 * j / max(nl - 1, 1) if nl > 1 else 0.5
            a = 0.6 + 2.4 * j + 1.3 * s + v
            _leaf("cl%d_%d" % (s, j), x + lean * t, y, 0.05 + H * t, a, (-0.35 if wilt else 0.45 - 0.2 * t),
                  0.16 + 0.22 * (H / 1.0) * (1.0 - 0.3 * t), 0.06, "tuber" if wilt else ("foliage", "vegetation")[j % 2],
                  thick=0.025)
        if st in ("mature", "harvestable"):
            _limb("tassel%d" % s, top, (top[0] + lean * 0.4, top[1], top[2] + 0.12), 0.01, 0.004, "straw", segs=4)
        if st == "harvestable":
            a = 0.3 + s * 2.2 + v
            cz = 0.05 + H * 0.55
            _leaf("husk%d" % s, x + lean * 0.55, y, cz, a, 0.85, 0.17, 0.075, "leaf_light", thick=0.075)
            _leaf("cob%d" % s, x + lean * 0.55 + math.cos(a) * 0.02, y + math.sin(a) * 0.02, cz + 0.04, a, 0.85,
                  0.14, 0.06, "corn_yellow", thick=0.06)


def _wheat(st, v):
    rng = random.Random(70 + v)
    cnt = {"sprout": 5, "young": 10, "mature": 15, "harvestable": 17, "wilted": 10}[st]
    cnt = int(cnt * (1.5 if st != "sprout" else 1))
    cnt = int(cnt * (0.85, 1.0, 1.15)[v])
    h0 = {"sprout": 0.12, "young": 0.32, "mature": 0.6, "harvestable": 0.66, "wilted": 0.5}[st] * (0.95, 1.0, 1.08)[v]
    for k in range(cnt):
        a = rng.uniform(0, 2 * math.pi)
        rr = rng.uniform(0.0, 0.2)
        x, y = CX + math.cos(a) * rr, CY + math.sin(a) * rr
        h = h0 * rng.uniform(0.82, 1.08)
        la = rng.uniform(0, 2 * math.pi)
        lean = rng.uniform(0.02, 0.07) if st != "wilted" else rng.uniform(0.12, 0.26)
        top = (x + math.cos(la) * lean, y + math.sin(la) * lean, 0.05 + h * (1.0 if st != "wilted" else 0.8))
        mat = {"harvestable": "straw", "wilted": "tuber"}.get(st, "vegetation" if k % 3 else "foliage")
        _limb("stem%d" % k, (x, y, 0.05), top, 0.014, 0.008, mat, segs=4)
        if st in ("mature", "harvestable", "wilted"):
            d = (math.cos(la), math.sin(la))
            _ball("ear%d" % k, top[0] + d[0] * 0.01, top[1] + d[1] * 0.01, top[2] + 0.045, 0.022, 0.022, 0.06,
                  {"harvestable": "corn_yellow", "wilted": "burlap"}.get(st, "leaf_light"), segs=6, rings=5)


def _boll(name, x, y, z, r, open_=True):
    """A cotton boll: a puff of fluff cupped in a brown calyx."""
    _ball(name + "c", x, y, z - r * 0.35, r * 0.7, r * 0.7, r * 0.5, "tuber", segs=6, rings=4)
    _ball(name + "a", x, y, z + r * 0.35, r, r, r * 0.95, "cotton_boll", segs=10, rings=7)
    if open_:
        _ball(name + "b", x + r * 0.55, y - r * 0.2, z + r * 0.15, r * 0.7, r * 0.7, r * 0.65, "cotton_boll",
              segs=8, rings=5)
        _ball(name + "d", x - r * 0.5, y + r * 0.1, z + r * 0.2, r * 0.65, r * 0.65, r * 0.6, "cotton_boll",
              segs=8, rings=5)


def _cotton(st, v):
    """Bushy shrub: a main stem, side branches carrying broad leaf tufts;
    mature plants show green bolls and a pale blossom, harvestable ones
    burst white bolls out of every tuft."""
    rng = random.Random(90 + v)
    if st == "sprout":
        _sprout_leaves(v, "vegetation", n=2, length=0.11, width=0.08, pitch=0.55)
        return
    wilt = st == "wilted"
    H = {"young": 0.3, "mature": (0.52, 0.58, 0.48)[v], "harvestable": (0.55, 0.62, 0.5)[v],
         "wilted": (0.4, 0.44, 0.36)[v]}[st]
    lean = (0.0, 0.04, -0.04)[v] * (3 if wilt else 1)
    top = (CX + lean, CY, 0.05 + H)
    _limb("stem", (CX, CY, 0.05), top, 0.026, 0.014, "tuber" if wilt else "vegetation", segs=6)
    lm = "tuber" if wilt else None
    if st == "young":
        for j in range(5):
            t = 0.25 + 0.7 * j / 4
            a = 0.7 + 2.3 * j + v
            _leaf("yl%d" % j, CX + lean * t, CY, 0.05 + H * t, a, 0.35, 0.17, 0.12,
                  ("foliage", "vegetation")[j % 2], thick=0.03)
        return
    nb = (3, 4, 3)[v] + (1 if st == "harvestable" else 0)
    tips = []
    for k in range(nb):
        t = 0.3 + 0.55 * k / max(nb - 1, 1)
        a = 2 * math.pi * k / nb + 0.6 + 0.8 * v
        reach = 0.2 - 0.05 * t + rng.uniform(-0.02, 0.03)
        z0 = 0.05 + H * t
        sx, sy = CX + lean * t, CY
        ex, ey = sx + math.cos(a) * reach, sy + math.sin(a) * reach
        ez = z0 + (0.06 if not wilt else -0.07)
        _limb("br%d" % k, (sx, sy, z0), (ex, ey, ez), 0.014, 0.008, "tuber" if wilt else "vegetation", segs=5)
        tips.append((ex, ey, ez, a))
        _cluster("tuft%d" % k, ex, ey, ez + 0.01, 0.1, lm or ("foliage", "vegetation")[k % 2], 70 + 3 * k + v,
                 lumps=0.35, squash=0.7)
        for j in range(2):
            _leaf("bl%d_%d" % (k, j), ex, ey, ez, a + (j * 2 - 1) * 0.9, -0.1 if wilt else 0.2, 0.13, 0.09,
                  lm or "leaf_light", thick=0.025)
    # crown tuft on the main stem
    _cluster("crown", top[0], top[1], top[2], 0.1, lm or "vegetation", 80 + v, lumps=0.35, squash=0.7)
    for j in range(4):
        _leaf("cl%d" % j, top[0], top[1], top[2] - 0.02, 0.8 + 1.7 * j + v, 0.3 if not wilt else -0.4, 0.15, 0.1,
              lm or ("foliage", "leaf_light")[j % 2], thick=0.025)
    if st == "mature":  # green bolls swelling and a first blossom
        for k, (ex, ey, ez, a) in enumerate(tips[:3]):
            _ball("gb%d" % k, ex + math.cos(a) * 0.05, ey + math.sin(a) * 0.05 - 0.02, ez - 0.01, 0.028, 0.028, 0.034,
                  "foliage", segs=7, rings=5)
        ex, ey, ez, a = tips[-1]
        _ball("bloom", ex, ey - 0.03, ez + 0.07, 0.04, 0.04, 0.03, "blossom", segs=8, rings=5)
    if st == "harvestable":
        for k, (ex, ey, ez, a) in enumerate(tips):
            _boll("bo%d" % k, ex + math.cos(a) * 0.035, ey + math.sin(a) * 0.035 - 0.015, ez + 0.07, 0.048)
        _boll("bt", top[0], top[1] - 0.02, top[2] + 0.06, 0.05)


def _pumpkin(st, v):
    rng = random.Random(80 + v)
    n = (4, 5, 6)[v]
    wilt = st == "wilted"
    if st == "sprout":
        _ring(2, 0.1, 0.09, 0.3, ("vegetation",), v, rng, r0=0.015)
        return
    scale = {"young": 0.65, "mature": 1.0, "harvestable": 1.0, "wilted": 0.85}[st]
    for k in range(n if st != "young" else n - 1):
        a = 2 * math.pi * k / n + 0.5 + 0.4 * v
        ln = 0.3 * scale * rng.uniform(0.85, 1.1)
        ex, ey = CX + math.cos(a) * ln, CY + math.sin(a) * ln
        _limb("vine%d" % k, (CX, CY, 0.06), (ex, ey, 0.065), 0.018, 0.01, "tuber" if wilt else "vegetation", segs=5)
        _leaf("pl%d" % k, CX + math.cos(a) * ln * 0.35, CY + math.sin(a) * ln * 0.35, 0.075, a + 0.25 * (-1) ** k,
              -0.28 if wilt else 0.22, 0.26 * scale, 0.19 * scale, "tuber" if wilt else ("foliage", "vegetation")[k % 2],
              thick=0.03)
    if st in ("mature", "harvestable"):
        for k in range(2 + (v == 2) if st == "mature" else 1):
            a = 1.0 + 2.6 * k + v
            _ball("bloom%d" % k, CX + math.cos(a) * 0.17, CY + math.sin(a) * 0.17, 0.14, 0.045, 0.045, 0.035,
                  "corn_yellow")
    if st == "harvestable":
        spots = [(-0.17, -0.14, 0.15), (0.17, -0.17, 0.12), (0.05, 0.18, 0.13)][:(2, 3, 2)[v]]
        for k, (dx, dy, r) in enumerate(spots):
            _ball("pk%d" % k, CX + dx, CY + dy, r * 0.75, r, r * 0.95, r * 0.78, "pumpkin", segs=16, rings=10)
            _ball("rib%d" % k, CX + dx, CY + dy, r * 0.8, r * 0.55, r * 0.95, r * 0.7, "crop_fruit", segs=10, rings=6)
            S.cyl("pstem%d" % k, CX + dx, CY + dy, 0.02, r * 1.3, r * 1.7, "bark", segs=6, bevel=0.0)
            _top(r * 1.7)


CROPS = {"radish": _radish, "lettuce": _lettuce, "beans": _beans, "potato": _potato,
         "corn": _corn, "wheat": _wheat, "pumpkin": _pumpkin, "healroot": _healroot, "cotton": _cotton}
STAGES = ("sown", "sprout", "young", "mature", "harvestable", "wilted")


GROW = {"corn": (1.3, 1.6), "wheat": (1.4, 1.45), "pumpkin": (1.5, 1.5), "beans": (1.45, 1.5), "cotton": (1.4, 1.5)}


def _crop_geo(species, st, v):
    if st in ("sown", "sprout"):
        _mound()
        if st == "sown":
            _sown(species, v)
        else:
            CROPS[species](st, v)
        return
    # grown plants: build, enlarge about the root, then a smaller mound underneath
    S.cyl("mound", CX, CY - 0.02, 0.2, 0.0, 0.035, "soil_rich", segs=18, r_top=0.15, bevel=0.0)
    before = set(o.name for o in S.objects())
    _Z[0] = 0.0
    CROPS[species](st, v)
    gx, gz = GROW.get(species, (1.45, 1.5))
    new = [o for o in S.objects() if o.name not in before]
    m = (Matrix.Translation((CX, CY, 0)) @ Matrix.Diagonal((gx, gx, gz, 1)) @ Matrix.Translation((-CX, -CY, 0)))
    S.transform(new, m)
    _Z[0] = max(_Z[0] * gz, 0.05)


def _crop(species, st, v, name):
    return _asset(lambda: _crop_geo(species, st, v), name,
                  {"species": species, "growth": st, "variant": v}, CROP_M)


# ------------------------------------------------------------ biome plants
# pine in snow, jungle tree, cactus, shrub and reeds (biome work).
# Like the other trees these are drawn at one size and scaled by the
# renderer with Growth; dry cactus/shrub is the withered look.
def _cactus(i, dry):
    mat = "cactus_dry" if dry else "cactus"
    if i == 0:  # saguaro: tall column, two arms
        S.cyl("col", CX, CY, 0.11, 0.0, 0.72, mat, segs=8, r_top=0.09, bevel=0.0)
        _ball("cap", CX, CY, 0.72, 0.09, 0.09, 0.07, mat, segs=8, rings=4)
        for sgn, h in ((-1, 0.34), (1, 0.46)):
            _limb("armh%d" % sgn, (CX + sgn * 0.08, CY, h), (CX + sgn * 0.26, CY, h), 0.06, 0.06, mat, segs=7)
            _limb("armv%d" % sgn, (CX + sgn * 0.26, CY, h), (CX + sgn * 0.26, CY, h + 0.26), 0.06, 0.05, mat, segs=7)
            _ball("armc%d" % sgn, CX + sgn * 0.26, CY, h + 0.26, 0.05, 0.05, 0.05, mat, segs=7, rings=4)
        bloom = (CX, CY, 0.79)
    elif i == 1:  # barrel
        _ball("barrel", CX, CY, 0.24, 0.2, 0.2, 0.26, mat, segs=10, rings=7)
        for k in range(6):
            a = 2 * math.pi * k / 6
            _limb("rib%d" % k, (CX + math.cos(a) * 0.19, CY + math.sin(a) * 0.19, 0.04),
                  (CX + math.cos(a) * 0.1, CY + math.sin(a) * 0.1, 0.5), 0.016, 0.01,
                  "shrub_dry" if not dry else "cactus", segs=4)
        bloom = (CX, CY, 0.5)
    else:  # prickly pear pads
        for k, (dx, z, rx, rz) in enumerate(((0.0, 0.0, 0.22, 0.24), (-0.1, 0.3, 0.17, 0.2), (0.15, 0.26, 0.16, 0.18))):
            _ball("pad%d" % k, CX + dx, CY, z + rz, rx, 0.07, rz, mat, segs=9, rings=5)
        bloom = (CX - 0.1, CY, 0.52)
    if not dry:
        _ball("bloom", bloom[0], bloom[1], bloom[2], 0.05, 0.05, 0.04, "cactus_bloom", segs=7, rings=4)


def _shrub(i, dry):
    leaf = "shrub_dry" if dry else "shrub_leaf"
    rng = random.Random(1500 + i)
    n = (6, 5, 7)[i]
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        ln = rng.uniform(0.34, 0.55)
        el = rng.uniform(0.7, 1.2)
        p0 = (CX, CY, 0.0)
        p1 = (CX + math.cos(a) * math.cos(el) * ln, CY + math.sin(a) * math.cos(el) * ln, math.sin(el) * ln)
        _limb("tw%d" % k, p0, p1, 0.02, 0.008, "bark", segs=4)
        if not dry or k % 2 == 0:
            _cluster("sl%d" % k, p1[0], p1[1], p1[2] - 0.02, 0.15 if not dry else 0.09, leaf,
                     seed=1510 + 10 * i + k, lumps=0.3, squash=0.8)
    if not dry:
        _cluster("core", CX, CY, 0.06, 0.22, leaf, seed=1530 + i, squash=0.8)


def _reeds(i):
    rng = random.Random(1700 + i)
    n = (14, 16, 12)[i]
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        rr = rng.uniform(0.03, 0.2)
        h = rng.uniform(0.6, 1.0)
        _blade("r%d" % k, CX + math.cos(a) * rr, CY + math.sin(a) * rr, h, a, rng.uniform(0.04, 0.16),
               ("reed", "vegetation")[k % 2], r=0.04)
        if k % 3 == 0:  # cattail head
            _ball("head%d" % k, CX + math.cos(a) * (rr + 0.09), CY + math.sin(a) * (rr + 0.09), h - 0.1,
                  0.045, 0.045, 0.12, "reed_head", segs=7, rings=5)


def _build_biome_plants():
    out = []
    for i in range(3):
        for dry in (False, True):
            sfx = "_dry" if dry else ""
            out.append(_asset(lambda i=i, d=dry: _cactus(i, d), "cactus_%02d%s" % (i, sfx),
                              {"species": "cactus", "variant": i, "dry": dry}, BUSH_M))
            out.append(_asset(lambda i=i, d=dry: _shrub(i, d), "shrub_%02d%s" % (i, sfx),
                              {"species": "shrub", "variant": i, "dry": dry}, BUSH_M))
        out.append(_asset(lambda i=i: _reeds(i), "reeds_%02d" % i, {"species": "reeds", "variant": i}, BUSH_M))
    return out


# ------------------------------------------------------------------ build
def build():
    out = trees.build()
    for i in range(4):
        out.append(_bush(i))
    for i in range(3):
        out.append(_berry(i, False))
        out.append(_berry(i, True))
    for i in range(8):
        out.append(_grass(i))
    for i in range(4):
        out.append(_grass_tall(i))
    for sp in CROPS:
        for st in STAGES:
            for v in range(3):
                out.append(_crop(sp, st, v, "%s_%s_v%d" % (sp, st, v)))
    # legacy aliases, to be deleted by the lead once the renderer switches
    for i in range(3):
        out.append(_tree("oak", i, "summer", "tree_%02d" % i))
    for i in range(2):
        out.append(_alias_berry(i))
    out.append(_crop("radish", "sown", 0, "crop_sown"))
    for sp in CROPS:
        for st in ("young", "mature", "harvestable"):
            out.append(_crop(sp, st, 1, "%s_%s" % (sp, st)))
    out += trees.build_biome()
    out += _build_biome_plants()
    return out


def _alias_berry(i):
    return _asset(lambda: _berry_geo(i, True), "bush_%02d_harvestable" % i,
                  {"species": "berry_bush", "variant": i, "harvestable": True}, BUSH_M)
