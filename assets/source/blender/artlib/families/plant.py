"""Plants (ART spec §9): silhouette and growth state first.

Trees (oak, birch, pine) are built from layered leaf-cluster blobs over a
trunk with bark ridges and a root flare; oak and birch also render
`_autumn` and `_bare` (winter) looks. Bushes, berry bushes, grass tufts
and eight crop species with six growth stages x three variants complete
the set. Origin is always the ground point (trunk base / tuft base /
mound centre) so the renderer can bend plants from the root. Variation
is authored per variant index, never RNG at runtime (all RNG is seeded).

Crop aliases (`crop_sown`, `corn_mature`...) are still rendered for the
renderer's fallbacks.
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


def _mound_geo(v, stage, berries=0):
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
    rng = random.Random(30 + v)
    s = STAGE_S[stage]
    n = {"young": 5, "mature": (8, 9, 7, 10)[v], "dormant": (6, 5, 7, 6)[v]}[stage]
    dorm = stage == "dormant"
    mats = ("pl_dried", "pl_stem") if dorm else ("pl_grass", "pl_forest", "pl_sage")
    _fan(n, 0.27 * s, 0.085 * s, 0.25 if dorm else 0.55, mats, rng, thick=0.03)
    if not dorm:
        _ball("heart", CX, CY, 0.05, 0.05 * s, 0.05 * s, 0.035, "pl_forest", segs=8, rings=4)


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
                                            {"young": 2, "mature": 4, "harvestable": 11, "dormant": 0}[st]),
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
CROP_M = (0.15, 0.15, 0.12, 0.15)
LEAF_Z = 0.07
BROWN = ("tuber", "burlap", "straw")
SEED_MAT = {"radish": "food_red", "lettuce": "leaf_light", "beans": "burlap", "potato": "tuber",
            "corn": "corn_yellow", "wheat": "straw", "pumpkin": "straw", "healroot": "vegetation", "cotton": "ceramic",
            "brewgrain": "straw", "ashleaf": "leaf_pine_hi"}


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


def _brewgrain(st, v):
    """Brewgrain: barley-like tufts whose heavy ears nod over, bristling with awns."""
    rng = random.Random(110 + v)
    cnt = int({"sprout": 5, "young": 10, "mature": 14, "harvestable": 16, "wilted": 9}[st] * (0.85, 1.0, 1.15)[v])
    h0 = {"sprout": 0.1, "young": 0.26, "mature": 0.44, "harvestable": 0.48, "wilted": 0.36}[st]
    for k in range(cnt):
        a = rng.uniform(0, 2 * math.pi)
        rr = rng.uniform(0.0, 0.18)
        x, y = CX + math.cos(a) * rr, CY + math.sin(a) * rr
        h = h0 * rng.uniform(0.8, 1.1)
        la = rng.uniform(0, 2 * math.pi)
        d = (math.cos(la), math.sin(la))
        lean = rng.uniform(0.03, 0.08) if st != "wilted" else rng.uniform(0.12, 0.24)
        top = (x + d[0] * lean, y + d[1] * lean, 0.05 + h * (1.0 if st != "wilted" else 0.8))
        mat = {"harvestable": "straw", "wilted": "tuber"}.get(st, "vegetation" if k % 3 else "foliage")
        _limb("stem%d" % k, (x, y, 0.05), top, 0.012, 0.007, mat, segs=4)
        if st in ("sprout", "young"):
            continue
        ear = {"harvestable": "corn_yellow", "wilted": "burlap"}.get(st, "leaf_light")
        drop = 0.07 if st != "wilted" else 0.05
        tip = (top[0] + d[0] * drop, top[1] + d[1] * drop, top[2] - 0.04)
        _limb("ear%d" % k, top, tip, 0.03, 0.022, ear, segs=6)          # the nodding ear
        for j in (-1, 1):                                                # awns fanning off the tip
            aw = (tip[0] + d[0] * 0.07 - d[1] * 0.03 * j, tip[1] + d[1] * 0.07 + d[0] * 0.03 * j, tip[2] - 0.05)
            _limb("awn%d_%d" % (k, j), tip, aw, 0.007, 0.002, "straw" if st != "wilted" else "burlap", segs=3)


def _ashleaf(st, v):
    """Ashleaf: a low rosette of broad, dusty blue-green leaves; pale flower spikes when ready."""
    rng = random.Random(120 + v)
    n = (7, 9, 8)[v]
    L = (0.26, 0.3, 0.28)[v]
    if st == "sprout":
        _sprout_leaves(v, "leaf_pine_hi", length=0.1, width=0.06)
    elif st == "young":
        _ring(n - 2, L * 0.62, 0.1, 0.45, ("leaf_pine_hi", "leaf_pine"), v, rng)
    elif st in ("mature", "harvestable"):
        _ring(n, L, 0.14, 0.35 - 0.05 * v, ("leaf_pine", "leaf_pine_hi"), v, rng)
        _ring(n - 3, L * 0.7, 0.11, 0.8, ("leaf_pine_hi", "leaf_pine"), v + 1, rng, z=0.14, r0=0.02)
        if st == "harvestable":
            for k in range(3 + (v == 1)):
                a = 2 * math.pi * k / 4 + 0.6 + v
                x, y = CX + 0.06 * math.cos(a), CY + 0.06 * math.sin(a)
                h = 0.3 + 0.04 * (k % 2)
                _limb("spike%d" % k, (x, y, 0.1), (x, y, h), 0.011, 0.007, "leaf_pine_hi", segs=5)
                for j in range(3):
                    _ball("bl%d_%d" % (k, j), x, y, h - 0.05 * j, 0.032 - 0.004 * j, 0.032 - 0.004 * j, 0.03,
                          "blossom", segs=7, rings=5)
    else:
        _ring(n - 2, L * 0.85, 0.09, -0.25, BROWN, v, rng)


CROPS = {"radish": _radish, "lettuce": _lettuce, "beans": _beans, "potato": _potato,
         "corn": _corn, "wheat": _wheat, "pumpkin": _pumpkin, "healroot": _healroot, "cotton": _cotton,
         "brewgrain": _brewgrain, "ashleaf": _ashleaf}
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
                  {"species": species, "growth": st, "variant": v}, CROP_M, outline=False)


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
    for sp in CROPS:
        for st in STAGES:
            for v in range(3):
                out.append(_crop(sp, st, v, "%s_%s_v%d" % (sp, st, v)))
    out.append(_crop("radish", "sown", 0, "crop_sown"))
    for sp in CROPS:
        for st in ("young", "mature", "harvestable"):
            out.append(_crop(sp, st, 1, "%s_%s" % (sp, st)))
    return out
