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

CX, CY = 0.5, -0.5
_Z = [0.0]


def _top(z):
    _Z[0] = max(_Z[0], z)


# ------------------------------------------------------------ primitives
def _limb(name, p0, p1, r0, r1, mat, segs=6):
    """Tapered tube from p0 to p1 (any direction)."""
    a, b = Vector(p0), Vector(p1)
    d = b - a
    rot = d.normalized().to_track_quat("Z", "Y").to_matrix().to_4x4()
    m = Matrix.Translation((a + b) / 2) @ rot
    bm = bmesh.new()
    bmesh.ops.create_cone(bm, cap_ends=True, cap_tris=False, segments=segs,
                          radius1=r0, radius2=r1, depth=d.length, matrix=m)
    _top(max(a.z, b.z) + max(r0, r1))
    return S._obj(name, bm, mat, smooth=True)


def _leaf(name, x, y, z, az, pitch, length, width, mat, thick=0.03, segs=8):
    """Lens-shaped leaf growing from (x,y,z) along azimuth/pitch."""
    d = Vector((math.cos(az) * math.cos(pitch), math.sin(az) * math.cos(pitch), math.sin(pitch)))
    rot = d.to_track_quat("X", "Z").to_matrix().to_4x4()
    c = Vector((x, y, z)) + d * length / 2
    m = Matrix.Translation(c) @ rot @ Matrix.Diagonal((length / 2, width / 2, thick / 2, 1))
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=5, radius=1.0, matrix=m)
    _top(z + d.z * length + thick)
    return S._obj(name, bm, mat, smooth=True)


def _ball(name, x, y, z, rx, ry, rz, mat, segs=10, rings=6):
    _top(z + rz)
    return S.ellipsoid(name, x, y, z, rx, ry, rz, mat, segs=segs, rings=rings)


def _cluster(name, x, y, z, r, mat, seed, lumps=0.25, squash=0.8):
    _top(z + r * squash * 1.15)
    return S.blob(name, x, y, z, r, mat, seed=seed, lumps=lumps, squash=squash)


def _asset(build, name, extra, margins, **kw):
    S.clear()
    _Z[0] = 0.0
    build()
    zmax = kw.pop("zmax", None) or max(_Z[0] + 0.03, 0.12)
    return S.render_asset("plant", name, zmax=zmax, margins=margins, extra=extra, **kw)


# ------------------------------------------------------------------ trees
TREE_MARGINS = (0.36, 0.36, 0.15, 0.25)
SWAY = {"trunk": 0.02, "canopy": 0.06}

# canopy colour bands per season: (bottom, mid, top)
_SUMMER = {
    "oak": (("foliage", "foliage"), ("foliage", "vegetation"), ("vegetation", "vegetation")),
    "birch": (("vegetation", "foliage"), ("leaf_light", "vegetation"), ("leaf_light", "leaf_light")),
}
_AUTUMN = {
    "oak": (("leaf_autumn_red", "leaf_autumn_red"), ("leaf_autumn_amber", "leaf_autumn_red"), ("corn_yellow", "leaf_autumn_amber")),
    "birch": (("leaf_autumn_amber", "corn_yellow"), ("corn_yellow", "straw"), ("straw", "leaf_autumn_amber")),
}


def _band(kind, season, layer, k):
    table = (_AUTUMN if season == "autumn" else _SUMMER)[kind]
    return table[layer][k % 2]


def _oak_trunk(i, th, lean):
    top = (CX + lean[0], CY + lean[1], th)
    S.cyl("flare", CX, CY, 0.21, 0.0, 0.12, "bark", segs=10, r_top=0.12, bevel=0.0)
    _limb("trunk", (CX, CY, 0.05), top, 0.125, 0.085, "bark", segs=9)
    for k in range(4):  # bark ridges
        a = math.radians(70 + 90 * k + 17 * i)
        o = (math.cos(a) * 0.1, math.sin(a) * 0.1)
        _limb("ridge%d" % k, (CX + o[0], CY + o[1], 0.04),
              (CX + lean[0] * 0.85 + o[0] * 0.8, CY + lean[1] * 0.85 + o[1] * 0.8, th * 0.85),
              0.026, 0.014, "bark", segs=4)
    for k in range(3):  # surface roots
        a = math.radians(30 + 120 * k + 40 * i)
        _limb("root%d" % k, (CX + math.cos(a) * 0.08, CY + math.sin(a) * 0.08, 0.09),
              (CX + math.cos(a) * 0.27, CY + math.sin(a) * 0.27, 0.0), 0.05, 0.025, "bark", segs=5)
    return top


def _birch_trunk(i, th, lean):
    top = (CX + lean[0], CY + lean[1], th)
    S.cyl("flare", CX, CY, 0.11, 0.0, 0.07, "bark_birch", segs=9, r_top=0.07, bevel=0.0)
    _limb("trunk", (CX, CY, 0.04), top, 0.068, 0.04, "bark_birch", segs=8)
    rng = random.Random(50 + i)
    for k in range(7):  # dark papery marks, facing the camera (south side)
        t = 0.12 + 0.8 * k / 6
        a = math.radians(rng.uniform(205, 335))
        r = 0.068 - 0.028 * t
        x = CX + lean[0] * t + math.cos(a) * r
        y = CY + lean[1] * t + math.sin(a) * r
        _ball("mark%d" % k, x, y, th * t, rng.uniform(0.022, 0.04), 0.012, rng.uniform(0.012, 0.02),
              "fur_dark", segs=6, rings=4)
    return top


def _crown(kind, i, season, top, R, th):
    """Layered leaf clusters: wide bottom ring, mid ring, bright cap."""
    rng = random.Random(100 * (kind == "birch") + 7 * i + 3)
    ring_n = ((5, 4, 2), (6, 4, 2), (5, 5, 1))[i] if kind == "oak" else ((6, 5, 3), (7, 5, 2), (6, 6, 3))[i]
    cz = th + (0.16 if kind == "oak" else 0.1)
    tx, ty = top[0], top[1]
    for layer, n in enumerate(ring_n):
        for k in range(n):
            a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3) + layer * 0.9 + i
            if kind == "oak":
                rad = R * (0.58, 0.40, 0.18)[layer] * rng.uniform(0.85, 1.1)
                z = cz + R * (0.22, 0.55, 0.85)[layer] + rng.uniform(-0.03, 0.05)
                cr = R * (0.46, 0.45, 0.40)[layer] * rng.uniform(0.9, 1.1)
                lumps, sq = 0.26, 0.85
            else:
                rad = R * (0.62, 0.45, 0.2)[layer] * rng.uniform(0.8, 1.15)
                z = cz + R * (0.3, 0.75, 1.15)[layer] + rng.uniform(-0.06, 0.1)
                cr = R * (0.32, 0.30, 0.26)[layer] * rng.uniform(0.85, 1.15)
                lumps, sq = 0.3, 0.8
            x, y = tx + math.cos(a) * rad, ty + math.sin(a) * rad * 0.9
            _cluster("leaf%d_%d" % (layer, k), x, y, z, cr, _band(kind, season, layer, k),
                     seed=40 * i + 7 * layer + k + 11, lumps=lumps, squash=sq)
            if kind == "birch" and layer < 2:  # thin twig carrying the cluster
                _limb("twig%d_%d" % (layer, k), (tx, ty, th * (0.8 + 0.1 * layer)), (x, y, z - cr * 0.3),
                      0.016, 0.01, "bark_birch", segs=4)
    if kind == "oak":  # dense core so the crown reads solid
        _cluster("core", tx, ty, cz + R * 0.45, R * 0.55, _band(kind, season, 1, 0), seed=900 + i,
                 lumps=0.2, squash=0.85)
    else:
        _cluster("core", tx, ty, cz + R * 0.55, R * 0.28, _band(kind, season, 1, 1), seed=900 + i,
                 lumps=0.3, squash=0.85)


def _bare(kind, i, top, R, th):
    """Winter branches only; light snow on the upper limbs."""
    rng = random.Random(300 + 11 * i + (kind == "birch") * 5)
    n = 8 if kind == "oak" else 8
    mat = "bark" if kind == "oak" else "bark_birch"
    r0 = 0.065 if kind == "oak" else 0.03
    for k in range(n):
        az = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3) + i
        el = math.radians(rng.uniform(25, 50) if kind == "oak" else rng.uniform(48, 68))
        ln = R * rng.uniform(0.9, 1.25) * (1.0 if kind == "oak" else 1.15)
        sz = th * rng.uniform(0.82, 1.0)
        p0 = (top[0], top[1], sz)
        p1 = (top[0] + math.cos(az) * math.cos(el) * ln, top[1] + math.sin(az) * math.cos(el) * ln,
              sz + math.sin(el) * ln)
        _limb("limb%d" % k, p0, p1, r0, r0 * 0.35, mat, segs=5)
        high = p1[2] > th + R * 0.45
        if high:
            _leaf("snow%d" % k, p0[0], p0[1], p0[2] + r0 * 0.9, az, el, ln * 0.85, 0.07, "snow", thick=0.04)
        for s in range(2):  # twigs
            t = 0.5 + 0.22 * s
            q = [p0[j] + (p1[j] - p0[j]) * t for j in range(3)]
            az2 = az + (1 if s else -1) * rng.uniform(0.5, 0.9)
            el2 = el + rng.uniform(-0.1, 0.3)
            ln2 = ln * 0.45
            q2 = (q[0] + math.cos(az2) * math.cos(el2) * ln2, q[1] + math.sin(az2) * math.cos(el2) * ln2,
                  q[2] + math.sin(el2) * ln2)
            _limb("twig%d_%d" % (k, s), q, q2, r0 * 0.5, r0 * 0.2, mat, segs=4)
            if high and s == 0:
                _leaf("snowt%d" % k, q[0], q[1], q[2] + 0.02, az2, el2, ln2 * 0.8, 0.045, "snow", thick=0.03)


def _deciduous(kind, i, season):
    if kind == "oak":
        th = (0.7, 0.6, 0.7)[i]
        lean = ((0.0, 0.0), (0.04, 0.0), (-0.03, 0.02))[i]
        R = (0.74, 0.68, 0.76)[i]
        top = _oak_trunk(i, th, lean)
    else:
        th = (0.72, 0.74, 0.78)[i]
        lean = ((0.05, 0.02), (-0.06, 0.0), (0.0, 0.05))[i]
        R = (0.62, 0.6, 0.62)[i]
        top = _birch_trunk(i, th, lean)
    if season == "bare":
        _bare(kind, i, top, R, th)
    else:
        _crown(kind, i, season, top, R, th)


def _pine(i):
    tiers = (6, 7, 5)[i]
    R0 = (0.6, 0.66, 0.5)[i]
    z0 = (0.26, 0.22, 0.26)[i]
    step = (0.21, 0.19, 0.25)[i]
    S.cyl("flare", CX, CY, 0.15, 0.0, 0.1, "bark", segs=9, r_top=0.09, bevel=0.0)
    S.cyl("trunk", CX, CY, 0.085, 0.0, z0 + 0.3, "bark", segs=8, r_top=0.06)
    for k in range(3):
        a = math.radians(60 + 120 * k + 30 * i)
        _limb("ridge%d" % k, (CX + math.cos(a) * 0.075, CY + math.sin(a) * 0.075, 0.04),
              (CX + math.cos(a) * 0.06, CY + math.sin(a) * 0.06, z0 + 0.1), 0.02, 0.012, "bark", segs=4)
    rng = random.Random(600 + i)
    for j in range(tiers):
        f = j / float(tiers)
        r = R0 * (1.0 - f) ** 0.9 + 0.09
        z = z0 + j * step
        # each tier is a short cone with a broad flat-ish top so the stack reads as layers
        S.cyl("tier%d" % j, CX + rng.uniform(-0.02, 0.02), CY + rng.uniform(-0.02, 0.02), r, z, z + step * 1.15,
              "leaf_pine" if j % 2 == 0 else "leaf_pine_hi", segs=(8, 9, 7)[(i + j) % 3], r_top=r * 0.55, bevel=0.0)
        _top(z + step * 1.15)
    S.cyl("tip", CX, CY, 0.07, z0 + tiers * step - 0.05, z0 + tiers * step + 0.28, "leaf_pine", segs=6, r_top=0.005,
          bevel=0.0)
    _top(z0 + tiers * step + 0.28)


def _tree(kind, i, season, name):
    extra = {"species": "tree", "kind": kind, "variant": i, "season": season, "yields": "wood", "sway": SWAY}
    if kind == "pine":
        return _asset(lambda: _pine(i), name, extra, TREE_MARGINS)
    return _asset(lambda: _deciduous(kind, i, season), name, extra, TREE_MARGINS)


# ----------------------------------------------------------------- bushes
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


def _blade(name, x, y, h, lean_az, lean, mat, r=0.03):
    mid = (x + math.cos(lean_az) * lean * 0.35, y + math.sin(lean_az) * lean * 0.35, h * 0.55)
    tip = (x + math.cos(lean_az) * lean, y + math.sin(lean_az) * lean, h)
    _limb(name + "a", (x, y, 0.0), mid, r, r * 0.7, mat, segs=4)
    _limb(name + "b", mid, tip, r * 0.7, 0.004, mat, segs=4)


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
            "corn": "corn_yellow", "wheat": "straw", "pumpkin": "straw", "healroot": "vegetation"}


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
         "corn": _corn, "wheat": _wheat, "pumpkin": _pumpkin, "healroot": _healroot}
STAGES = ("sown", "sprout", "young", "mature", "harvestable", "wilted")


GROW = {"corn": (1.3, 1.6), "wheat": (1.4, 1.45), "pumpkin": (1.5, 1.5), "beans": (1.45, 1.5)}


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


# ------------------------------------------------------------------ build
def build():
    out = []
    for kind in ("oak", "birch", "pine"):
        for i in range(3):
            out.append(_tree(kind, i, "summer", "tree_%s_%02d" % (kind, i)))
            if kind != "pine":
                out.append(_tree(kind, i, "autumn", "tree_%s_%02d_autumn" % (kind, i)))
                out.append(_tree(kind, i, "bare", "tree_%s_%02d_bare" % (kind, i)))
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
    return out


def _alias_berry(i):
    return _asset(lambda: _berry_geo(i, True), "bush_%02d_harvestable" % i,
                  {"species": "berry_bush", "variant": i, "harvestable": True}, BUSH_M)
