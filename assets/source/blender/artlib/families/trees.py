"""Trees (ART spec §9.2): oak, birch, pine (summer, autumn, bare, snow) and
jungle trees. Split out of plant.py in PLAY-010 so tree and ground-plant
art can change independently; plant.build() renders both.
"""
import math
import random

from mathutils import Vector

from .. import stage as S
from .plant_base import CX, CY, _asset, _ball, _cluster, _leaf, _limb, _top

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

def _pine_snow(i):
    tiers = (6, 7, 5)[i]
    R0 = (0.6, 0.66, 0.5)[i]
    z0 = (0.26, 0.22, 0.26)[i]
    step = (0.21, 0.19, 0.25)[i]
    S.cyl("flare", CX, CY, 0.15, 0.0, 0.1, "bark", segs=9, r_top=0.09, bevel=0.0)
    S.cyl("trunk", CX, CY, 0.085, 0.0, z0 + 0.3, "bark", segs=8, r_top=0.06)
    rng = random.Random(600 + i)
    for j in range(tiers):
        f = j / float(tiers)
        r = R0 * (1.0 - f) ** 0.9 + 0.09
        z = z0 + j * step
        cx, cy = CX + rng.uniform(-0.02, 0.02), CY + rng.uniform(-0.02, 0.02)
        segs = (8, 9, 7)[(i + j) % 3]
        S.cyl("tier%d" % j, cx, cy, r, z, z + step * 1.15, "leaf_pine" if j % 2 == 0 else "leaf_pine_hi",
              segs=segs, r_top=r * 0.55, bevel=0.0)
        # snow load: a smaller cone resting on the tier's shoulder
        S.cyl("snow%d" % j, cx, cy, r * 0.8, z + step * 0.55, z + step * 1.2, "snow", segs=segs,
              r_top=r * 0.45, bevel=0.0)
        _top(z + step * 1.2)
    S.cyl("tip", CX, CY, 0.07, z0 + tiers * step - 0.05, z0 + tiers * step + 0.28, "snow", segs=6, r_top=0.005,
          bevel=0.0)
    _top(z0 + tiers * step + 0.28)


def _jungle(i):
    th = (0.95, 0.85, 1.0)[i]
    lean = ((0.0, 0.0), (0.06, 0.02), (-0.05, 0.03))[i]
    top = (CX + lean[0], CY + lean[1], th)
    S.cyl("flare", CX, CY, 0.25, 0.0, 0.14, "bark", segs=10, r_top=0.12, bevel=0.0)
    _limb("trunk", (CX, CY, 0.05), top, 0.12, 0.075, "bark", segs=9)
    for k in range(4):  # buttress roots
        a = math.radians(40 + 90 * k + 25 * i)
        _limb("butt%d" % k, (CX + math.cos(a) * 0.06, CY + math.sin(a) * 0.06, 0.28),
              (CX + math.cos(a) * 0.3, CY + math.sin(a) * 0.3, 0.0), 0.06, 0.03, "bark", segs=5)
    rng = random.Random(1200 + i)
    R = (0.82, 0.76, 0.86)[i]
    rings = ((7, 0.62, 0.0), (5, 0.36, 0.1), (1, 0.0, 0.2))  # flat, wide umbrella of big leaf clusters
    for layer, (n, rad, dz) in enumerate(rings):
        for k in range(n):
            a = 2 * math.pi * k / max(n, 1) + rng.uniform(-0.3, 0.3) + layer
            x = top[0] + math.cos(a) * R * rad
            y = top[1] + math.sin(a) * R * rad * 0.9
            z = th + 0.05 + dz + rng.uniform(-0.03, 0.04)
            cr = R * (0.38, 0.4, 0.42)[layer] * rng.uniform(0.9, 1.1)
            _cluster("jl%d_%d" % (layer, k), x, y, z, cr, ("jungle_leaf", "jungle_leaf_hi")[(k + layer) % 2],
                     seed=1210 + 17 * i + 5 * layer + k, lumps=0.28, squash=0.55)
    for k in range(3):  # hanging vine
        a = rng.uniform(0, 2 * math.pi)
        x, y = top[0] + math.cos(a) * R * 0.5, top[1] + math.sin(a) * R * 0.45
        _limb("vine%d" % k, (x, y, th + 0.02), (x + 0.02, y, th - 0.28), 0.014, 0.008, "jungle_leaf", segs=4)



def build():
    """Every tree sprite, in the order plant.build() used to render them."""
    out = []
    for kind in ("oak", "birch", "pine"):
        for i in range(3):
            out.append(_tree(kind, i, "summer", "tree_%s_%02d" % (kind, i)))
            if kind != "pine":
                out.append(_tree(kind, i, "autumn", "tree_%s_%02d_autumn" % (kind, i)))
                out.append(_tree(kind, i, "bare", "tree_%s_%02d_bare" % (kind, i)))
    return out


def build_biome():
    """The snowy pines and jungle trees (formerly in _build_biome_plants)."""
    out = []
    for i in range(3):
        out.append(_asset(lambda i=i: _pine_snow(i), "tree_pine_%02d_snow" % i,
                          {"species": "tree", "kind": "pine", "variant": i, "season": "snow", "yields": "wood",
                           "sway": SWAY}, TREE_MARGINS))
        out.append(_asset(lambda i=i: _jungle(i), "tree_jungle_%02d" % i,
                          {"species": "tree", "kind": "jungle", "variant": i, "season": "summer",
                           "yields": "wood", "sway": SWAY}, TREE_MARGINS))
    return out
