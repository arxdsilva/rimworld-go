"""FOOD-programme crops (ART spec §9.3): rice, strawberry, haygrass, hops.
Original designs; the unit contract is crops_common.py's (one small plant
per sprite, the renderer composes several per tile, no outline, no baked
shadow, the muted two-band `pl_crop_*` palette). BUILDERS maps species ->
fn(stage, v); families/food.py renders them.

Suggested units per tile (crop_batch.go): rice 4 clumps, haygrass 4
tufts, strawberry 3 low rosettes, hops 2 caned bines.

- rice: a clump of narrow upright blades; loose panicles of small grains
  on fine stems, green and nodding when mature, straw-gold and drooping
  over the clump when ripe (wheat's single head and awns read different).
- haygrass: a dense fountain of fine grass blades with slim upright seed
  plumes; ripe tips dry to khaki.
- strawberry: a low rosette of three-leaflet leaves; small white flowers
  when mature, red berries resting on the soil at the edge when ripe.
- hops: a twining bine on a thin cane with lobed leaves; clusters of pale
  green cones hang from the side shoots when ripe.

A variant keeps its structure through every stage (structure RNG seeded
per species and variant), so a unit grows rather than swaps design.
"""
import math
import random

from mathutils import Vector

from .. import stage as S
from .crops_grain import _curve, _palm, _strap
from .crops_grain import _stem as _gstem
from .crops_leafy import _droop, _seeds, _sprout, _trifoliate, _wilt_fan
from .plant_base import CX, CY, _ball, _cluster, _leaf, _limb, _top

SEED = {"rice": 8100, "haygrass": 8200, "strawberry": 8300, "hops": 8400}
# dying foliage: browned, a little dull green left, lighter than the soil
WILT = ("pl_crop_wilt", "pl_dried", "pl_khaki", "pl_crop_wilt", "pl_dry")


def _rng(sp, v, salt=0):
    """Structure RNG: the same per variant at every stage."""
    return random.Random(SEED[sp] + 17 * v + 1000 * salt)


def _blades(rng, n, length, width, rise, end, mats, a0=0.0, r0=0.01, jit=0.4, lj=0.15, thick=0.009, name="bl"):
    """Strap blades fanning out of a narrow base."""
    for k in range(n):
        a = a0 + 2 * math.pi * k / n + rng.uniform(-jit, jit)
        _strap("%s%d" % (name, k), CX + math.cos(a) * r0, CY + math.sin(a) * r0, 0.0, a,
               length * rng.uniform(1 - lj, 1 + lj), width, rise + rng.uniform(-0.15, 0.15),
               end + rng.uniform(-0.15, 0.1), mats[k % len(mats)], n=5, thick=thick)


def _grass_sprout(sp, v, n, length, width):
    rng = _rng(sp, v, 8)
    _blades(rng, n, length, width, 1.1, 0.8, ("pl_crop_leaf", "pl_crop_jade", "pl_crop_deep"), a0=rng.uniform(0, 6.3),
            r0=0.006, thick=0.008, name="sp")


# ------------------------------------------------------------------- rice
RICE_H = {"young": 0.3, "mature": 0.5, "harvestable": 0.52, "wilted": 0.4}
RICE_N = (5, 6, 4, 6)   # panicle stems per variant


def _panicle(name, top, a, ripe, wilt, rng):
    """A loose panicle: a fine rachis arching out and over from the stem
    top, short branches off it, each strung with small grains."""
    droop = 0.9 if ripe else (1.3 if wilt else 0.35)
    d = Vector((math.cos(a), math.sin(a), 0.0))
    ln = 0.15 if not wilt else 0.11
    p0 = Vector(top)
    p1 = p0 + d * ln * 0.45 + Vector((0, 0, ln * (0.55 - 0.5 * droop)))
    p2 = p0 + d * ln * 0.8 + Vector((0, 0, ln * (0.35 - 0.9 * droop)))
    pts = [p0, p1, p2]
    rmat = "pl_crop_straw" if ripe else ("pl_dried" if wilt else "pl_crop_stem")
    gmat = (("pl_crop_corn", "pl_crop_straw") if ripe else
            (("pl_crop_wilt", "pl_dried") if wilt else ("pl_crop_jade", "pl_crop_pod")))
    _gstem(name + "r", pts, 0.006, 0.003, rmat, segs=4)
    nb = 4
    for j in range(nb):
        t = 0.2 + 0.75 * j / (nb - 1)
        q = p0.lerp(p1, min(t * 2, 1.0)) if t < 0.5 else p1.lerp(p2, (t - 0.5) * 2)
        side = 1 if j % 2 else -1
        ba = a + side * rng.uniform(0.5, 0.9)
        bd = Vector((math.cos(ba), math.sin(ba), 0.0))
        bl = 0.055 * (1.0 - 0.3 * t)
        e = q + bd * bl + Vector((0, 0, -bl * (0.3 + 0.8 * droop)))
        _limb("%sb%d" % (name, j), q, e, 0.004, 0.0025, rmat, segs=3)
        for g in range(3):
            c = q.lerp(e, 0.35 + 0.3 * g)
            pitch = -0.4 - 0.6 * droop
            _leaf("%sg%d_%d" % (name, j, g), c.x, c.y, c.z, ba, pitch, 0.026, 0.014, gmat[(g + j) % 2],
                  thick=0.012, segs=6)
    for g in range(2):   # grains at the rachis tip
        c = p1.lerp(p2, 0.7 + 0.3 * g)
        _leaf("%st%d" % (name, g), c.x, c.y, c.z, a, -0.6 - 0.5 * droop, 0.026, 0.014, gmat[g % 2], thick=0.012,
              segs=6)


def _rice(st, v):
    rng = _rng("rice", v)
    if st == "sown":
        _seeds("sd", (4, 3, 5, 3)[v], 0.014, "pl_crop_straw", rng, spread=0.045, flat=0.55)
        return
    if st == "sprout":
        return _grass_sprout("rice", v, (3, 4, 3, 3)[v], 0.15, 0.018)
    H = RICE_H[st] * (0.95, 1.05, 0.9, 1.0)[v]
    ripe, wilt = st == "harvestable", st == "wilted"
    a0 = rng.uniform(0, 2 * math.pi)
    nl = (8, 9, 7, 9)[v] + (1 if st == "young" else 0)
    for k in range(nl):  # long narrow blades, upright with arching tips
        a = a0 + 2 * math.pi * k / nl + rng.uniform(-0.35, 0.35)
        if wilt:
            mat, rise, end = WILT[k % len(WILT)], 0.45, -0.25
        elif ripe:
            mat, rise, end = ("pl_crop_leaf", "pl_crop_straw", "pl_crop_stem", "pl_dry")[k % 4], 1.05, 0.55
        else:
            mat, rise, end = ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf", "pl_crop_jade")[k % 4], 1.25, 0.8
        L = H * rng.uniform(0.85, 1.12) * (1.0 if st == "young" else 0.9)
        _strap("bl%d" % k, CX + math.cos(a) * 0.012, CY + math.sin(a) * 0.012, 0.0, a, L, 0.022,
               rise + rng.uniform(-0.15, 0.15), end + rng.uniform(-0.2, 0.1), mat, n=6, thick=0.008)
    if st == "young":
        return
    n = RICE_N[v]
    for k in range(n):
        a = a0 + 0.3 + 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        d = (math.cos(a), math.sin(a))
        rr = rng.uniform(0.005, 0.03)
        h = H * rng.uniform(0.9, 1.08)
        lean = rng.uniform(0.04, 0.1) * (2.2 if wilt else 1.0)
        bend = h * 0.3 if wilt else (0.04 if ripe else 0.0)
        pts = _curve((CX + d[0] * rr, CY + d[1] * rr, 0.0), d, h, lean, bend, n=3)
        smat = WILT[(k + 1) % len(WILT)] if wilt else ("pl_crop_straw" if ripe and k % 2 else "pl_crop_stem")
        _gstem("st%d_" % k, pts, 0.009, 0.006, smat, segs=4)
        _panicle("pn%d" % k, pts[-1], a, ripe, wilt, rng)


# --------------------------------------------------------------- haygrass
HAY_H = {"young": 0.28, "mature": 0.5, "harvestable": 0.54, "wilted": 0.38}


def _plume(name, x, y, z, a, h, lean, mat, rng):
    """A slim seed plume on a fine stalk: soft, upright, slightly leaning."""
    d = (math.cos(a), math.sin(a))
    pts = _curve((x, y, z), d, h, lean, 0.0, n=3)
    _gstem(name + "s", pts, 0.006, 0.004, "pl_crop_stem" if mat != "pl_dried" else "pl_dried", segs=4)
    top = pts[-1]
    pitch = math.atan2(h, lean * 2.0 + 1e-6)
    hl = 0.16 * rng.uniform(0.85, 1.1)
    _leaf(name + "p", top.x, top.y, top.z - 0.01, a, pitch, hl, 0.046, mat, thick=0.04, segs=8)


def _haygrass(st, v):
    rng = _rng("haygrass", v)
    if st == "sown":
        _seeds("sd", (5, 4, 6, 4)[v], 0.012, "pl_khaki", rng, spread=0.05, flat=0.5)
        return
    if st == "sprout":
        return _grass_sprout("haygrass", v, (4, 3, 4, 5)[v], 0.13, 0.014)
    H = HAY_H[st] * (1.0, 0.92, 1.06, 0.96)[v]
    ripe, wilt = st == "harvestable", st == "wilted"
    a0 = rng.uniform(0, 2 * math.pi)
    n = (14, 16, 13, 15)[v] - (4 if st == "young" else 0)
    for k in range(n):  # a dense fountain of fine blades
        a = a0 + 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        if wilt:
            mat, rise, end = WILT[k % len(WILT)], 0.4, -0.3
        elif ripe:
            mat = ("pl_crop_leaf", "pl_dry", "pl_khaki", "pl_crop_deep", "pl_dry")[k % 5]
            rise, end = 1.0, 0.4
        else:
            mat = ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf", "pl_crop_jade", "pl_crop_stem")[k % 5]
            rise, end = 1.1, 0.5
        L = H * rng.uniform(0.65, 1.0)
        _strap("bl%d" % k, CX + math.cos(a) * 0.014, CY + math.sin(a) * 0.014, 0.0, a, L, 0.017,
               rise + rng.uniform(-0.2, 0.2), end + rng.uniform(-0.25, 0.15), mat, n=6, thick=0.007)
    if st == "young":
        return
    npl = ((2, 3, 2, 3) if st == "mature" else (4, 5, 4, 5))[v]
    for k in range(npl):
        a = a0 + 0.7 + 2 * math.pi * k / npl + rng.uniform(-0.4, 0.4)
        if wilt:
            mat = "pl_dried"
        elif ripe:
            mat = ("pl_khaki", "pl_crop_straw", "pl_dry")[k % 3]
        else:
            mat = ("pl_crop_jade", "pl_crop_pod")[k % 2]
        h = H * rng.uniform(1.05, 1.25) * (0.7 if wilt else 1.0)
        lean = rng.uniform(0.03, 0.08) * (3.0 if wilt else 1.0)
        _plume("pl%d" % k, CX + math.cos(a) * 0.01, CY + math.sin(a) * 0.01, 0.0, a, h, lean, mat, rng)


# ------------------------------------------------------------- strawberry
def _berry(name, x, y, z, a, s, ripe=True, wilt=False):
    """A strawberry lying tip-out on the soil with a green calyx cap."""
    mat = "pl_dried" if wilt else ("pl_crop_strawberry" if ripe else "pl_crop_pod")
    _leaf(name, x, y, z, a, -0.25, 0.05 * s, 0.042 * s, mat, thick=0.04 * s, segs=10)
    for j in range(3):   # calyx: three tiny sepals at the stem end
        _leaf("%sc%d" % (name, j), x, y, z + 0.016 * s, a + math.pi + (j - 1) * 1.1, 0.25, 0.024 * s, 0.012 * s,
              "pl_dried" if wilt else "pl_crop_deep", thick=0.008, segs=6)


def _flower(name, x, y, z):
    S.ellipsoid(name, x, y, z, 0.02, 0.02, 0.01, "pl_crop_cotton", segs=10, rings=5)
    S.ellipsoid(name + "c", x, y, z + 0.008, 0.008, 0.008, 0.006, "pl_crop_corn", segs=6, rings=4)
    _top(z + 0.016)


def _strawberry(st, v):
    rng = _rng("strawberry", v)
    if st == "sown":
        _seeds("sd", (3, 2, 3, 4)[v], 0.013, "pl_crop_tuber", rng, spread=0.035, flat=0.65)
        return
    if st == "sprout":
        _sprout("sp", 3, 0.04, 0.032, ("pl_crop_deep", "pl_crop_leaf"), rng, h=0.022, pitch=0.45)
        return
    a0 = rng.uniform(0, 2 * math.pi)
    if st == "wilted":
        _wilt_fan("wi", (5, 4, 6, 5)[v], 0.09, 0.05, rng, z=0.04, a0=a0)
        for k in range((1, 2, 1, 2)[v]):
            a = -math.pi / 2 + (k - 0.5) * 1.1
            _berry("wb%d" % k, CX + math.cos(a) * 0.11, CY + math.sin(a) * 0.1, 0.018, a, 0.8, wilt=True)
        return
    young = st == "young"
    s = 0.62 if young else 1.0
    n = (4, 3, 4, 4)[v] if young else (6, 7, 6, 7)[v]
    if not young:   # dark leafy core so the rosette never shows soil through its middle
        _cluster("core", CX, CY + 0.005, 0.0, 0.06, "pl_crop_deep", 8350 + v, lumps=0.3, squash=0.7)
    for k in range(n):
        a = a0 + 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        h = rng.uniform(0.04, 0.07) * s
        p = (CX + math.cos(a) * 0.05 * s, CY + math.sin(a) * 0.05 * s, h)
        _limb("pt%d" % k, (CX, CY, 0.0), p, 0.006, 0.005, "pl_crop_stem", segs=4)
        _trifoliate("lf%d" % k, p, a, rng.uniform(0.15, 0.35), rng.uniform(0.07, 0.085) * s,
                    ("pl_crop_leaf", "pl_crop_deep", "pl_crop_jade")[k % 3], rng)
    if young:
        return
    nf = (2, 3, 2, 2)[v] if st == "mature" else (1, 1, 0, 1)[v]
    for k in range(nf):   # small white flowers held just over the leaves
        a = a0 + 0.6 + 2 * math.pi * k / max(nf, 1) + rng.uniform(-0.3, 0.3)
        x, y = CX + math.cos(a) * 0.06, CY + math.sin(a) * 0.05
        h = rng.uniform(0.11, 0.13)
        _limb("fs%d" % k, (CX, CY, 0.04), (x, y, h), 0.004, 0.003, "pl_crop_stem", segs=4)
        _flower("fl%d" % k, x, y, h)
    if st == "mature":
        if v % 2:   # one green berry setting
            a = -math.pi / 2 + 0.6
            _berry("gb", CX + math.cos(a) * 0.12, CY + math.sin(a) * 0.1, 0.02, a, 0.75, ripe=False)
        return
    nb = (3, 4, 4, 5)[v]
    for k in range(nb):   # ripe berries resting on the soil round the front edge
        a = -math.pi / 2 + (k - (nb - 1) / 2) * 0.75 + rng.uniform(-0.2, 0.2)
        d = rng.uniform(0.11, 0.14)
        x, y = CX + math.cos(a) * d, CY + math.sin(a) * d * 0.9
        _limb("bs%d" % k, (CX, CY, 0.04), (x, y, 0.035), 0.004, 0.003, "pl_crop_stem", segs=4)
        _berry("rb%d" % k, x, y, 0.022, a, rng.uniform(0.9, 1.1), ripe=k != nb - 1 or v % 2 == 0)


# ------------------------------------------------------------------- hops
HOPS_H = {"young": 0.5, "mature": 0.98, "harvestable": 1.02, "wilted": 0.9}


def _cone(name, x, y, z, s, mat):
    """A hop cone: stacked overlapping bracts tapering to a point, hanging."""
    for j in range(3):
        r = (0.04, 0.034, 0.022)[j] * s
        _ball("%s%d" % (name, j), x, y, z - 0.032 * j * s, r, r, r * 1.2, mat, segs=8, rings=5)


def _hops(st, v):
    rng = _rng("hops", v)
    if st == "sown":
        _seeds("sd", (2, 3, 2, 2)[v], 0.016, "pl_crop_tuber", rng, spread=0.03, flat=0.6)
        return
    if st == "sprout":
        _sprout("sp", 2, 0.05, 0.034, ("pl_crop_leaf", "pl_crop_deep"), rng, h=0.04, pitch=0.4)
        _limb("tip", (CX, CY, 0.04), (CX + 0.01, CY, 0.09), 0.005, 0.003, "pl_crop_stem", segs=4)
        return
    H = HOPS_H[st] * (1.0, 0.94, 1.05, 0.97)[v]
    ripe, wilt = st == "harvestable", st == "wilted"
    a0 = rng.uniform(0, 2 * math.pi)
    # the cane: a thin pole the bine climbs, a little taller than the plant
    cx, cy = CX + 0.01, CY + 0.005
    _limb("cane", (cx, cy, 0.0), (cx, cy, H + 0.06), 0.011, 0.008, "pl_stem", segs=6)
    # the bine: a stem spiralling up round the cane
    turns = 2.5
    pts = []
    for i in range(13):
        t = i / 12
        a = a0 + 2 * math.pi * turns * t
        r = 0.022
        z = H * t
        if wilt:   # the top slumps off the cane
            z = H * t * (1.0 - 0.25 * t * t)
            r += 0.05 * t * t
        pts.append(Vector((cx + math.cos(a) * r, cy + math.sin(a) * r, z)))
    _gstem("bine", pts, 0.009, 0.005, "pl_dried" if wilt else "pl_crop_stem", segs=5)
    nleaf = (6, 7, 6, 8)[v] if st != "young" else 4
    tips = []
    for k in range(nleaf):   # lobed leaves in pairs up the bine, smaller near the top
        t = 0.12 + 0.82 * k / max(nleaf - 1, 1)
        i = min(int(t * 12), 11)
        p = pts[i].lerp(pts[i + 1], t * 12 - i)
        a = a0 + k * 2.4 + rng.uniform(-0.3, 0.3)
        size = (0.15 - 0.05 * t) * (0.8 if st == "young" else 1.0)
        if wilt:
            mat = WILT[(k + v) % len(WILT)]
            _droop("lf%d" % k, p.x, p.y, p.z, a, size * 0.85, size * 0.5, mat, rng, thick=0.018)
            continue
        mat = ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf", "pl_crop_jade")[k % 4]
        if ripe and k == 0:
            mat = "pl_dry"   # the lowest leaf yellows as the cones ripen
        _palm("lf%d" % k, p.x + math.cos(a) * 0.02, p.y + math.sin(a) * 0.02, p.z, a, rng.uniform(0.0, 0.25), size,
              mat, ("pl_crop_leaf", "pl_crop_deep")[k % 2])
        tips.append((p, a))
    if st == "young":
        return
    # side shoots: short arms off the bine that carry the cone clusters
    nshoot = (3, 3, 4, 3)[v]
    for k in range(nshoot):
        t = 0.35 + 0.55 * k / max(nshoot - 1, 1)
        i = min(int(t * 12), 11)
        p = pts[i].lerp(pts[i + 1], t * 12 - i)
        a = a0 + 0.9 + k * 2.1 + rng.uniform(-0.3, 0.3)
        e = p + Vector((math.cos(a) * 0.14, math.sin(a) * 0.1, -0.02 if not wilt else -0.06))
        _limb("sh%d" % k, p, e, 0.006, 0.004, "pl_dried" if wilt else "pl_crop_stem", segs=4)
        if st == "mature":
            _palm("sl%d" % k, e.x, e.y, e.z, a, 0.1, 0.085, ("pl_crop_leaf", "pl_crop_jade")[k % 2])
        if st == "mature":
            if k % 2 == 0:   # small young cones forming
                _cone("yc%d" % k, e.x, e.y - 0.02, e.z - 0.01, 0.6, "pl_crop_jade")
            continue
        nc = 3 if not wilt else 2
        for j in range(nc):   # a hanging cluster of pale cones
            ca = a + (j - (nc - 1) / 2) * 1.1
            x = e.x + math.cos(ca) * 0.04
            y = e.y + math.sin(ca) * 0.03 - 0.02
            _cone("cn%d_%d" % (k, j), x, y, e.z - 0.02 - 0.012 * (j % 2), 1.0, "pl_dried" if wilt else "pl_crop_hop")
    if not wilt:   # a crown of young leaves at the top of the bine
        top = pts[-1]
        for j in range(2):
            _palm("cr%d" % j, top.x, top.y, top.z, a0 + j * math.pi, 0.5, 0.07, "pl_crop_jade")


BUILDERS = {
    "rice": _rice,
    "haygrass": _haygrass,
    "strawberry": _strawberry,
    "hops": _hops,
}
