"""Plants (ART spec §9): silhouette and growth state first.

Trees (oak, birch, pine) are built from layered leaf-cluster blobs over a
trunk with bark ridges and a root flare; oak and birch also render
`_autumn` and `_bare` (winter) looks. Bushes, berry bushes, grass tufts
and ground plants complete the set; crops are units built in
crops_grain.py and crops_leafy.py (PLAY-016). Origin is always the
ground point (trunk base / tuft base) so the renderer can bend plants
from the root. Variation
is authored per variant index, never RNG at runtime (all RNG is seeded).
"""
import math
import random

import bmesh
import numpy as np
from mathutils import Matrix, Vector

from .. import stage as S
from .plant_base import CX, CY, _Z, _asset, _ball, _blade, _cluster, _leaf, _limb, _top
from .trees import _tree  # noqa: F401  (scenes/menu.py calls plant._tree)

BUSH_M = (0.2, 0.2, 0.12, 0.15)
GRASS_M = (0.15, 0.15, 0.1, 0.12)
STAGE_S = {"young": 0.66, "mature": 1.0, "harvestable": 1.0, "dormant": 0.92}


# ------------------------------------------------------------ new primitives
def _lobe(name, x, y, z, az, rl, rt, rz, mat, segs=10):
    """Ellipsoid whose long axis (rl) points along azimuth az, centred at x,y,z."""
    m = Matrix.Translation((x, y, z)) @ Matrix.Rotation(az, 4, "Z") @ Matrix.Diagonal((rl, rt, rz, 1))
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=6, radius=1.0, matrix=m)
    _top(z + rz)
    return S._obj(name, bm, mat, smooth=True)


def _gblade(name, x, y, h, az, lean, mat, r=0.022):
    """Tapered blade in three short segments that curl over towards `az`."""
    pts = [(x, y, 0.0)]
    for t, f in ((0.4, 0.12), (0.75, 0.45), (1.0, 1.0)):
        pts.append((x + math.cos(az) * lean * f, y + math.sin(az) * lean * f, h * t))
    rs = (r, r * 0.8, r * 0.5, 0.003)
    for k in range(3):
        _limb("%s%d" % (name, k), pts[k], pts[k + 1], rs[k], rs[k + 1], mat, segs=4)


def _gmat(k, i=0):
    """Blade colour: mostly grass green, some forest base, a few sage tips."""
    return ("pl_grass", "pl_forest", "pl_grass", "pl_sage", "pl_grass", "pl_forest")[(k + i) % 6]


def _flat(extra, flat):
    if flat:
        extra = dict(extra, flat_normal=True)
    return extra


def _plant_asset(build, name, extra, margins=BUSH_M, flat=False):
    return _asset(build, name, _flat(extra, flat), margins, outline=False, shadow=False)


# ------------------------------------------------------------------ grass
def _grass_short_geo(i, dormant=False):
    rng = random.Random(70 + 3 * i + (400 if dormant else 0))
    n = (5, 7, 4, 9, 6, 8)[i] if not dormant else (5, 4, 6)[i]
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.35, 0.35)
        rr = rng.uniform(0.0, 0.08)
        h = rng.uniform(0.24, 0.42) * (0.8 if dormant else 1.0)
        lean = rng.uniform(0.04, 0.16) * (1.6 if dormant else 1.0)
        mat = ("pl_dried", "pl_khaki", "pl_stem")[(k + i) % 3] if dormant else (
            "pl_dry" if (k + i) % 7 == 0 else _gmat(k, i))
        _gblade("g%d" % k, CX + math.cos(a) * rr, CY + math.sin(a) * rr, h, a, lean, mat,
                r=0.018 if not dormant else 0.014)


def _grass_tall_geo(i):
    rng = random.Random(170 + 5 * i)
    n = (9, 11, 8, 12)[i]
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        rr = rng.uniform(0.0, 0.12)
        h = rng.uniform(0.45, 0.74)
        lean = rng.uniform(0.06, 0.22)
        mat = "pl_dry" if (k + i) % 8 == 0 else ("pl_forest" if k % 3 == 0 else _gmat(k, i))
        _gblade("t%d" % k, CX + math.cos(a) * rr, CY + math.sin(a) * rr, h, a, lean, mat, r=0.022)


def _grass_short(i):
    return _plant_asset(lambda: _grass_short_geo(i), "grass_short_v%d" % i,
                        {"species": "grass", "stage": "mature", "variant": i, "decoration": True},
                        GRASS_M, flat=True)


def _grass_dormant(i):
    return _plant_asset(lambda: _grass_short_geo(i, True), "grass_short_dormant_v%d" % i,
                        {"species": "grass", "stage": "dormant", "variant": i, "decoration": True},
                        GRASS_M, flat=True)


def _grass_tall(i):
    return _plant_asset(lambda: _grass_tall_geo(i), "grass_tall_v%d" % i,
                        {"species": "grass_tall", "stage": "mature", "variant": i, "decoration": True},
                        GRASS_M, flat=True)


# a single tuft / the mature bush, used by scenes/menu.py (not built as sprites)
def _grass(i):
    return _plant_asset(lambda: _grass_short_geo(i % 6), "grass_%02d" % i,
                        {"species": "grass", "variant": i, "decoration": True}, GRASS_M, flat=True)


# ------------------------------------------------------------ bush mounds
# (dx, dy, radius, z) clusters, back to front; wider than tall.
BUSH_LAYOUT = (
    ((0.0, 0.0, 0.3, 0.0), (-0.22, 0.05, 0.2, 0.0), (0.22, -0.04, 0.21, 0.0), (0.02, 0.04, 0.2, 0.14)),
    ((-0.12, 0.0, 0.26, 0.0), (0.14, 0.03, 0.27, 0.0), (0.0, -0.1, 0.2, 0.0), (0.0, 0.12, 0.17, 0.12)),
    ((-0.26, -0.02, 0.17, 0.0), (-0.06, 0.02, 0.28, 0.0), (0.2, -0.05, 0.2, 0.0), (0.34, 0.0, 0.11, 0.0),
     (-0.08, 0.04, 0.16, 0.16)),
    ((-0.3, 0.0, 0.16, 0.0), (-0.1, 0.04, 0.2, 0.0), (0.12, 0.0, 0.2, 0.0), (0.3, -0.02, 0.16, 0.0),
     (0.0, -0.08, 0.15, 0.0)),
)
LIVE = ("pl_dark", "pl_forest", "pl_dark", "pl_forest", "pl_grass")
DRIED = ("pl_dried", "pl_stem", "pl_dried", "pl_khaki", "pl_dried")


def _mound_geo(v, stage, berries=0, ripe=False):
    s = STAGE_S[stage]
    dorm = stage == "dormant"
    lay = BUSH_LAYOUT[v % 4]
    cl = []
    if dorm:  # bare twigs under thin dried clumps
        rng = random.Random(900 + v)
        for k in range(9):
            a = 2 * math.pi * k / 9 + rng.uniform(-0.2, 0.2)
            ln = rng.uniform(0.2, 0.36) * s
            _limb("tw%d" % k, (CX, CY, 0.0), (CX + math.cos(a) * ln, CY + math.sin(a) * ln * 0.8,
                                              rng.uniform(0.12, 0.26) * s), 0.014, 0.006, "pl_stem", segs=4)
    for k, (dx, dy, r, z) in enumerate(lay):
        if stage == "young" and k >= 3:
            continue
        if dorm and k % 2:
            continue
        r2 = r * s * (0.8 if dorm else 1.0)
        mat = (DRIED if dorm else LIVE)[k % 5]
        x, y = CX + dx * s, CY + dy * s
        _cluster("b%d" % k, x, y, z * s, r2, mat, 41 + 7 * v + k, lumps=0.34, squash=0.7)
        cl.append((x, y, z * s, r2))
        if not dorm and k < 3:  # lighter, smaller clump on the crown of the big ones
            _cluster("m%d" % k, x - 0.02, y, z * s + r2 * 0.4, r2 * 0.55, "pl_grass" if k != 1 else "pl_sage",
                     60 + 5 * v + k, lumps=0.3, squash=0.6)
    rng = random.Random(950 + v)
    for k in range(berries):
        x, y, z, r = cl[k % len(cl)]
        a = rng.uniform(0, 2 * math.pi)
        d = rng.uniform(0.2, 0.9) * r
        _ball("berry%d" % k, x + math.cos(a) * d, y + math.sin(a) * d * 0.8, z + r * 0.55, 0.032, 0.032, 0.03,
              "pl_berry", segs=7, rings=4)
    if ripe:
        _ripe_berries(cl, v)


def _ripe_berries(cl, v):
    """Harvestable berry bush (PLAY-016): small deep plum-red clusters set
    into the foliage, sunk a little into the clumps so leaves frame them."""
    rng = random.Random(970 + v)
    for k in range((8, 7, 9, 8)[v % 4]):
        x, y, z, r = cl[k % len(cl)]
        a = rng.uniform(-2.8, 1.2)  # the lit and front faces, outside the crown clump
        d = rng.uniform(0.5, 0.8) * r
        bx, by = x + math.cos(a) * d, y + math.sin(a) * d * 0.8
        bz = z + r * 0.7 * math.sqrt(max(0.0, 1.0 - (d / r) ** 2))
        n = 3 + (k + v) % 2
        for j in range(n):
            b = 2 * math.pi * j / n + rng.uniform(-0.3, 0.3)
            _ball("rb%d_%d" % (k, j), bx + math.cos(b) * 0.017, by + math.sin(b) * 0.014, bz + 0.005 * (j % 2),
                  0.016, 0.016, 0.015, "pl_crop_berry", segs=7, rings=4)


def _bush(i):
    """The mature bush (scenes/menu.py)."""
    return _plant_asset(lambda: _mound_geo(i, "mature"), "bush_%02d" % i,
                        {"species": "bush", "variant": i})


# ---------------------------------------------------------------- rosettes
def _fan(n, length, width, pitch, mats, rng, z=0.03, r0=0.02, thick=0.03, a0=0.0, span=2 * math.pi, jit=0.12,
         name="lf"):
    for k in range(n):
        a = a0 + span * (k + 0.5) / n + rng.uniform(-jit, jit)
        _leaf("%s%d" % (name, k), CX + math.cos(a) * r0, CY + math.sin(a) * r0, z + 0.01 * (k % 2), a,
              pitch + rng.uniform(-0.12, 0.12), length * rng.uniform(0.85, 1.12), width, mats[k % len(mats)],
              thick=thick)


def _shrub_geo(v, stage):
    rng = random.Random(1500 + v)
    s = STAGE_S[stage]
    n = {"young": 6, "mature": (9, 8, 11, 10)[v], "dormant": (7, 6, 8, 7)[v]}[stage]
    dorm = stage == "dormant"
    mats = ("pl_dried", "pl_stem", "pl_khaki") if dorm else ("pl_grass", "pl_forest", "pl_sage")
    _fan(n, 0.3 * s, 0.13 * s, 0.2 if dorm else 0.38, mats, rng, thick=0.035)
    if not dorm:
        _fan(max(n // 2, 3), 0.2 * s, 0.1 * s, 0.75, ("pl_sage", "pl_grass"), rng, z=0.07, a0=0.4, name="in")


def _reeds_geo(v, stage):
    rng = random.Random(1700 + v)
    s = STAGE_S[stage]
    n = {"young": 6, "mature": (10, 12, 9, 11)[v], "dormant": (8, 7, 9, 8)[v]}[stage]
    dorm = stage == "dormant"
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        rr = rng.uniform(0.02, 0.17)
        h = rng.uniform(0.55, 0.95) * s
        x, y = CX + math.cos(a) * rr, CY + math.sin(a) * rr * 0.7
        lean = rng.uniform(0.02, 0.1) * (2.2 if dorm else 1.0)
        mat = ("pl_dried", "pl_khaki")[k % 2] if dorm else ("pl_grass", "pl_forest", "pl_sage")[k % 3]
        _gblade("r%d" % k, x, y, h, a, lean, mat, r=0.016)
        if stage != "young" and k % 3 == 0:
            ex, ey = x + math.cos(a) * lean, y + math.sin(a) * lean
            _ball("head%d" % k, ex, ey, h - 0.06, 0.03, 0.03, 0.085, "pl_stem", segs=7, rings=5)


def _cactus_geo(v, stage):
    s = STAGE_S[stage]
    mat = "pl_khaki" if stage == "dormant" else "pl_cactus"
    young = stage == "young"
    H = (0.72, 0.6, 0.66, 0.5)[v] * s
    cols = [(0.0, 0.0, H)]
    if v == 3:
        cols = [(-0.1, 0.0, H), (0.12, 0.02, H * 0.78)]
    for k, (dx, dy, h) in enumerate(cols):
        S.cyl("col%d" % k, CX + dx, CY + dy, 0.1 * s, 0.0, h, mat, segs=8, r_top=0.085 * s, bevel=0.0)
        _ball("cap%d" % k, CX + dx, CY + dy, h, 0.085 * s, 0.085 * s, 0.07 * s, mat, segs=8, rings=4)
        for r in range(4):  # faint ribs: thin darker strips down the column
            a = 2 * math.pi * r / 4 + 0.4
            _limb("rb%d_%d" % (k, r), (CX + dx + math.cos(a) * 0.1 * s, CY + dy + math.sin(a) * 0.1 * s, 0.03),
                  (CX + dx + math.cos(a) * 0.085 * s, CY + dy + math.sin(a) * 0.085 * s, h - 0.02), 0.012, 0.01,
                  "pl_forest" if stage != "dormant" else "pl_dried", segs=4)
    if young or v == 3:
        return
    arms = {0: ((-1, 0.32), (1, 0.44)), 1: (), 2: ((1, 0.34),)}[v]
    for sgn, h in arms:
        h *= s
        _limb("armh%d" % sgn, (CX + sgn * 0.07 * s, CY, h), (CX + sgn * 0.24 * s, CY, h), 0.055 * s, 0.055 * s,
              mat, segs=7)
        _limb("armv%d" % sgn, (CX + sgn * 0.24 * s, CY, h), (CX + sgn * 0.24 * s, CY, h + 0.22 * s), 0.055 * s,
              0.047 * s, mat, segs=7)
        _ball("armc%d" % sgn, CX + sgn * 0.24 * s, CY, h + 0.22 * s, 0.047 * s, 0.047 * s, 0.045 * s, mat,
              segs=7, rings=4)


def _healroot_geo(v, stage):
    # PLAY-016: the crop's healroot herb in the wild palette (crops_leafy)
    from .crops_leafy import healroot_wild
    healroot_wild(v, stage)


def _bramble_geo(v, stage):
    rng = random.Random(1900 + v)
    s = STAGE_S[stage]
    n = {"young": 9, "mature": (20, 17, 22, 18)[v], "dormant": (18, 15, 19, 16)[v]}[stage]
    dorm = stage == "dormant"
    ends = []
    for k in range(n):
        a = rng.uniform(0, 2 * math.pi)
        ln = rng.uniform(0.3, 0.5) * s
        ox, oy = rng.uniform(-0.12, 0.12) * s, rng.uniform(-0.08, 0.08) * s
        z0 = rng.uniform(0.03, 0.09)
        mid = (CX + ox + math.cos(a + 0.35) * ln * 0.5, CY + oy + math.sin(a + 0.35) * ln * 0.4,
               z0 + rng.uniform(0.04, 0.12) * s)
        end = (CX + ox + math.cos(a) * ln, CY + oy + math.sin(a) * ln * 0.75, z0 + rng.uniform(0.0, 0.07))
        st = ("pl_stem", "pl_dark", "pl_stem")[k % 3] if not dorm else ("pl_stem", "pl_dried")[k % 2]
        _limb("st%d" % k, (CX + ox, CY + oy, z0), mid, 0.024, 0.018, st, segs=5)
        _limb("sv%d" % k, mid, end, 0.018, 0.008, st, segs=5)
        ends.append((mid, end))
    if not dorm:
        for k in range(n * 2 if stage == "mature" else n):
            p = ends[k % len(ends)][k % 2]
            _cluster("lc%d" % k, p[0], p[1], p[2] + 0.01, 0.075 * s, ("pl_forest", "pl_grass", "pl_dark")[k % 3],
                     1950 + k, lumps=0.35, squash=0.55)
    else:
        for k in range(4):
            p = ends[k][1]
            _leaf("dl%d" % k, p[0], p[1], p[2], k * 1.7, -0.1, 0.09, 0.05, "pl_dried", thick=0.02, segs=6)


def _moss_geo(v, stage):
    rng = random.Random(2100 + v)
    s = STAGE_S[stage]
    n = {"young": 4, "mature": (8, 7, 9, 8)[v], "dormant": (7, 6, 7, 6)[v]}[stage]
    dorm = stage == "dormant"
    mats = ("pl_dried", "pl_khaki", "pl_stem") if dorm else ("pl_moss", "pl_moss_dark", "pl_sage")
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.4, 0.4)
        d = rng.uniform(0.04, 0.22) * s
        r = rng.uniform(0.09, 0.15) * s
        S.blob("mo%d" % k, CX + math.cos(a) * d, CY + math.sin(a) * d * 0.75, 0.0, r, mats[k % 3],
               seed=2110 + 9 * v + k, lumps=0.4, squash=0.22)
        _top(r * 0.25)


def _dusk_strand(name, x, y, az, length, h, r, mat, rng):
    """Low wandering fibre: five short segments whose heading drifts, rising then settling."""
    pts = [(x, y, 0.012)]
    for i in range(1, 6):
        az += rng.uniform(-0.8, 0.8)
        step = length / 5
        x, y = x + math.cos(az) * step, y + math.sin(az) * step * 0.8
        pts.append((x, y, 0.012 + h * math.sin(math.pi * min(i / 5.0 * 0.9 + 0.1, 1.0)) * 0.9))
    for k in range(5):
        _limb("%s%d" % (name, k), pts[k], pts[k + 1], r * (1 - k * 0.14), r * (1 - (k + 1) * 0.14), mat, segs=5)
    return pts[-1]


def _duskmoss_geo(v, stage):
    """Duskmoss: a charcoal root mass with low, tangled slate / grey-blue fibres wandering
    outward under flat leafy lobes; dormant is greyer and flatter. No contour."""
    rng = random.Random(3100 + v)
    s = STAGE_S[stage]
    dorm = stage == "dormant"
    n = {"young": (8, 9, 8, 9)[v], "mature": (18, 17, 19, 18)[v], "dormant": (14, 13, 14, 15)[v]}[stage]
    mats = (("pl_dusk_dormant", "pl_dusk_char", "pl_dusk_dormant") if dorm
            else ("pl_dusk_slate", "pl_dusk_grey", "pl_dusk_teal", "pl_dusk_slate", "pl_dusk_char"))
    S.blob("root", CX, CY, 0.0, 0.10 * s, "pl_dusk_char", seed=3120 + v, lumps=0.3, squash=0.5)
    _top(0.06)
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.4, 0.4)
        r0 = rng.uniform(0.02, 0.06) * s
        h = rng.uniform(0.04, 0.085) * s * (0.5 if dorm else 1.0)
        end = _dusk_strand("ds%d_" % k, CX + math.cos(a) * r0, CY + math.sin(a) * r0 * 0.8, a,
                           rng.uniform(0.22, 0.42) * s, h, 0.02, mats[k % len(mats)], rng)
        _top(h + 0.02)
        if k % 3 == 0:   # a leafy tuft at the fibre tip
            S.blob("dt%d" % k, end[0], end[1], 0.0, 0.034 * s, mats[(k + 1) % len(mats)],
                   seed=3140 + 7 * v + k, lumps=0.5, squash=0.22)
    for k in range(4 if stage == "young" else 8):
        a = rng.uniform(0, 2 * math.pi)
        d = rng.uniform(0.05, 0.15) * s
        S.blob("dl%d" % k, CX + math.cos(a) * d, CY + math.sin(a) * d * 0.8, 0.0, rng.uniform(0.06, 0.09) * s,
               mats[(k + 2) % len(mats)], seed=3160 + 9 * v + k, lumps=0.4, squash=0.25)


def _agave_geo(v, stage):
    rng = random.Random(2300 + v)
    s = STAGE_S[stage]
    n = {"young": 8, "mature": (12, 10, 14, 11)[v], "dormant": (9, 8, 10, 9)[v]}[stage]
    dorm = stage == "dormant"
    mats = ("pl_khaki", "pl_dry") if dorm else ("pl_agave", "pl_agave", "pl_cactus")
    _fan(n, 0.36 * s, 0.1 * s, 0.35 if dorm else 0.6, mats, rng, thick=0.055, r0=0.02)
    if not dorm:
        _fan(max(n // 2, 4), 0.26 * s, 0.08 * s, 1.05, ("pl_agave", "pl_sage"), rng, z=0.04, a0=0.3, thick=0.05,
             r0=0.01, name="up")


def _ball_cactus_geo(v, stage):
    s = STAGE_S[stage]
    dorm = stage == "dormant"
    n = (8, 9, 10, 8)[v]
    R = (0.17, 0.2, 0.18, 0.15)[v] * s
    H = R * 0.95
    mats = ("pl_khaki", "pl_dry") if dorm else ("pl_cactus", "pl_forest")
    for k in range(n):
        a = 2 * math.pi * k / n
        _lobe("rib%d" % k, CX + math.cos(a) * R * 0.45, CY + math.sin(a) * R * 0.45, H, a, R * 0.62, R * 0.36 + 0.01,
              H, mats[k % 2], segs=10)
    _ball("top", CX, CY, H * 1.5, R * 0.4, R * 0.4, H * 0.35, mats[0], segs=8, rings=4)
    if stage == "mature":
        _ball("bloom", CX, CY, H * 1.78, 0.04, 0.04, 0.025, "pl_dandelion", segs=7, rings=4)


def _arrow(name, x, y, z, az, pitch, length, width, mat, vein="pl_sage"):
    """Sagittate leaf on a short stalk with a pale central vein."""
    st = 0.07 * length
    sx, sy = x + math.cos(az) * st, y + math.sin(az) * st
    sz = z + st * math.sin(pitch) * 0.6
    _limb(name + "p", (x, y, z), (sx, sy, sz), 0.012, 0.01, "pl_stem", segs=4)
    _leaf(name, sx, sy, sz, az, pitch, length * 0.78, width, mat, thick=0.035)
    for sgn in (-1, 1):  # basal lobes make the arrow head
        _leaf(name + "l%d" % sgn, sx, sy, sz, az + sgn * 2.45, pitch * 0.5, length * 0.34, width * 0.6, mat,
              thick=0.035, segs=6)
    d = (math.cos(az) * math.cos(pitch), math.sin(az) * math.cos(pitch), math.sin(pitch))
    p0 = (sx, sy, sz + 0.02)
    p1 = (sx + d[0] * length * 0.7, sy + d[1] * length * 0.7, sz + 0.02 + d[2] * length * 0.7)
    _limb(name + "v", p0, p1, 0.011, 0.005, vein, segs=4)


def _elephant_ear_geo(v, stage):
    rng = random.Random(2500 + v)
    s = STAGE_S[stage]
    n = {"young": 5, "mature": (8, 7, 9, 6)[v], "dormant": (5, 4, 6, 4)[v]}[stage]
    dorm = stage == "dormant"
    mats = ("pl_dried", "pl_stem") if dorm else ("pl_forest", "pl_dark", "pl_forest", "pl_grass")
    for k in range(n):
        a = 2 * math.pi * (k + 0.5) / n + rng.uniform(-0.15, 0.15)
        _arrow("ea%d" % k, CX + math.cos(a) * 0.02, CY + math.sin(a) * 0.02, 0.03 + 0.015 * (k % 2), a,
               0.12 if dorm else 0.42 + rng.uniform(-0.08, 0.1), 0.5 * s * rng.uniform(0.9, 1.08), 0.25 * s,
               mats[k % len(mats)], "pl_dried" if dorm else "pl_sage")


def _fire_lily_geo(v, stage):
    rng = random.Random(2700 + v)
    s = STAGE_S[stage]
    n = {"young": 5, "mature": (8, 7, 9, 8)[v], "dormant": (6, 5, 6, 5)[v]}[stage]
    dorm = stage == "dormant"
    base = (-1.3, -1.0, -1.6, -1.15)[v]
    mats = ("pl_dried", "pl_stem") if dorm else ("pl_grass", "pl_forest", "pl_sage")
    _fan(n, 0.5 * s, 0.065 * s, 0.4 if dorm else 1.0, mats, rng, thick=0.025, a0=base - 1.25, span=2.5, name="st")
    if stage == "mature":
        for k in range(2 + (v % 2)):
            a = base + (k - 1) * 0.5
            x, y = CX + math.cos(a) * 0.04, CY + math.sin(a) * 0.04
            h = 0.42 + 0.08 * k
            _limb("fs%d" % k, (x, y, 0.0), (x + 0.03 * (k - 1), y, h), 0.01, 0.008, "pl_forest", segs=4)
            for j in range(3):
                b = 2 * math.pi * j / 3 + k
                _ball("fl%d_%d" % (k, j), x + 0.03 * (k - 1) + math.cos(b) * 0.03, y + math.sin(b) * 0.025,
                      h + 0.015 * j, 0.036, 0.036, 0.032, "pl_fire", segs=7, rings=4)


def _dandelion_geo(v, stage):
    rng = random.Random(2900 + v)
    s = STAGE_S[stage]
    dorm = stage == "dormant"
    n = {"young": 5, "mature": (7, 6, 8, 7)[v], "dormant": 5}[stage]
    mats = ("pl_dried", "pl_stem") if dorm else ("pl_grass", "pl_forest")
    _fan(n, 0.13 * s, 0.045, 0.2, mats, rng, thick=0.02, r0=0.01)
    if stage == "mature":
        for k in range(1 + (v % 2)):
            a = rng.uniform(0, 2 * math.pi)
            x, y = CX + math.cos(a) * 0.04, CY + math.sin(a) * 0.04
            h = 0.12 + 0.05 * k
            _limb("ds%d" % k, (x, y, 0.0), (x + 0.015, y, h), 0.007, 0.005, "pl_grass", segs=4)
            _ball("dd%d" % k, x + 0.015, y, h + 0.012, 0.03, 0.03, 0.02, "pl_dandelion", segs=7, rings=4)


# kinds: key -> (builder, stages, variant count, flat)
KINDS = {
    "bush": (lambda v, st: _mound_geo(v, st), ("young", "mature", "dormant"), 4, False),
    "berry_bush": (lambda v, st: _mound_geo(v, "mature" if st == "harvestable" else st,
                                            {"young": 2, "mature": 4, "harvestable": 0, "dormant": 0}[st],
                                            ripe=st == "harvestable"),
                   ("young", "mature", "harvestable", "dormant"), 4, False),
    "shrub": (_shrub_geo, ("young", "mature", "dormant"), 4, False),
    "reeds": (_reeds_geo, ("young", "mature", "dormant"), 4, False),
    "cactus": (_cactus_geo, ("young", "mature", "dormant"), 4, False),
    "healroot_wild": (_healroot_geo, ("young", "mature", "dormant"), 4, False),
    "bramble": (_bramble_geo, ("young", "mature", "dormant"), 4, False),
    "moss": (_moss_geo, ("young", "mature", "dormant"), 4, True),
    "duskmoss": (_duskmoss_geo, ("young", "mature", "dormant"), 4, True),
    "agave": (_agave_geo, ("young", "mature", "dormant"), 4, False),
    "ball_cactus": (_ball_cactus_geo, ("young", "mature", "dormant"), 4, False),
    "elephant_ear": (_elephant_ear_geo, ("young", "mature", "dormant"), 4, False),
    "fire_lily": (_fire_lily_geo, ("young", "mature", "dormant"), 4, False),
    "dandelion": (_dandelion_geo, ("young", "mature", "dormant"), 4, True),
}


def _ground(key, stage, v, name=None):
    fn, _, _, flat = KINDS[key]
    return _plant_asset(lambda: fn(v, stage), name or "%s_%s_v%d" % (key, stage, v),
                        {"species": key, "stage": stage, "variant": v}, BUSH_M, flat=flat)


# ------------------------------------------------------------------ crops
def _crop_builders():
    """Every crop's unit builder (PLAY-016, ART spec §9.3)."""
    from . import crops_grain, crops_leafy
    out = dict(crops_grain.BUILDERS)
    out.update(crops_leafy.BUILDERS)
    return out


def _crop(species, st, v, name):
    """One crop unit sprite (also placed by the main-menu scene)."""
    from . import crops_common
    return crops_common.unit(species, st, v, _crop_builders()[species], name)


# ------------------------------------------------------------------ build
def build():
    out = []
    for i in range(6):
        out.append(_grass_short(i))
    for i in range(4):
        out.append(_grass_tall(i))
    for i in range(3):
        out.append(_grass_dormant(i))
    for key, (_, stages, nv, _flat) in KINDS.items():
        for st in stages:
            for v in range(nv):
                out.append(_ground(key, st, v))
    from . import crops_common
    for sp, fn in _crop_builders().items():
        for st in crops_common.STAGES:
            for v in range(crops_common.VARIANTS):
                out.append(crops_common.unit(sp, st, v, fn))
    return out
