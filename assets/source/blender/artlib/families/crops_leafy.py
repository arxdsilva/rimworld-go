"""Leafy, root and vine crops (PLAY-016, ART spec §9.3): potato, beans,
lettuce, radish, pumpkin, healroot, ashleaf. Owned by the ART-LEAFY agent.
BUILDERS maps species -> fn(stage, v); species absent here still use the
legacy builders in plant.py.

Every builder draws ONE unit (the renderer composes 3-4 per tile, one
pumpkin sprawl): lettuce and radish about a grass tuft, potato, beans,
healroot and ashleaf a small leafy clump, pumpkin a low vine about a small
bush. No soil mound, no outline; shapes separate by the two-band value of
the `pl_crop_*` palette alone (darker core, lighter exposed leaves).
Produce half-sinks into the ground (`_dome` flattens below z=0) so it
peeks out of the soil instead of sitting on it like a toy.

`healroot_wild` is the same herb as the crop in the wild ground-plant
palette, for plant.py's ground-plant kinds.
"""
import math
import random

import bmesh
from mathutils import Matrix

from .. import stage as S
from .crops_common import STAGES
from .plant_base import CX, CY, _cluster, _leaf, _limb, _top

# browned, drooping leaves of a dying crop; mostly the paler dried tones,
# since pl_crop_wilt alone sinks into the soil colour
WILT = ("pl_dried", "pl_khaki", "pl_crop_wilt", "pl_dry", "pl_dried")
# sprouts read as a speck of green at colony zoom, not a dot
SPROUT_K = 1.4
SPECIES = ("potato", "beans", "lettuce", "radish", "pumpkin", "healroot", "ashleaf")


def _rng(species, stage, v):
    return random.Random(7000 + 1000 * SPECIES.index(species) + 100 * STAGES.index(stage) + v)


def _dir(az, pitch):
    return (math.cos(az) * math.cos(pitch), math.sin(az) * math.cos(pitch), math.sin(pitch))


def _at(p, az, pitch, ln):
    d = _dir(az, pitch)
    return (p[0] + d[0] * ln, p[1] + d[1] * ln, p[2] + d[2] * ln)


# ------------------------------------------------------------- primitives
def _dome(name, x, y, z, rx, ry, rz, mat, segs=12, rings=8):
    """Ellipsoid cut flat at the soil line: produce peeking out of the ground."""
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=rings, radius=1.0)
    for vt in bm.verts:
        vt.co.x = x + vt.co.x * rx
        vt.co.y = y + vt.co.y * ry
        vt.co.z = max(0.0, z + vt.co.z * rz)
    _top(z + rz)
    return S._obj(name, bm, mat, smooth=True)


def _rib(name, x, y, z, az, rl, rt, rz, mat, segs=10):
    """Ellipsoid whose long axis points along az (a pumpkin rib)."""
    m = Matrix.Translation((x, y, z)) @ Matrix.Rotation(az, 4, "Z") @ Matrix.Diagonal((rl, rt, rz, 1))
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=6, radius=1.0, matrix=m)
    for vt in bm.verts:
        vt.co.z = max(0.0, vt.co.z)
    _top(z + rz)
    return S._obj(name, bm, mat, smooth=True)


def _speck(name, x, y, rx, ry, rz, az, mat):
    """A seed lying on the soil."""
    m = Matrix.Translation((x, y, rz * 0.6)) @ Matrix.Rotation(az, 4, "Z") @ Matrix.Diagonal((rx, ry, rz, 1))
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=8, v_segments=5, radius=1.0, matrix=m)
    _top(rz * 1.6)
    return S._obj(name, bm, mat, smooth=True)


def _stem(name, pts, r0, r1, mat, segs=4):
    """Polyline stem through pts, tapering r0 -> r1."""
    n = len(pts) - 1
    for k in range(n):
        _limb("%s%d" % (name, k), pts[k], pts[k + 1], r0 + (r1 - r0) * k / n, r0 + (r1 - r0) * (k + 1) / n, mat,
              segs=segs)


def _droop(name, x, y, z, az, length, width, mat, rng, thick=0.02):
    """A wilted leaf hanging from (x,y,z) with its tip resting on the soil."""
    pitch = -math.asin(min(0.95, max(z - 0.004, 0.0) / length)) * rng.uniform(0.55, 0.85)
    _leaf(name, x, y, z, az, pitch, length, width, mat, thick=thick, segs=8)


def _fan(name, n, length, width, pitch, mats, rng, z=0.02, r0=0.01, a0=0.0, span=2 * math.pi, jit=0.15,
         thick=0.022, lj=0.12, pj=0.12, x=CX, y=CY):
    """Leaves spread round a centre (a rosette when span is a full turn)."""
    out = []
    for k in range(n):
        a = a0 + span * (k + 0.5) / n + rng.uniform(-jit, jit)
        ln = length * rng.uniform(1 - lj, 1 + lj)
        p = pitch + rng.uniform(-pj, pj)
        _leaf("%s%d" % (name, k), x + math.cos(a) * r0, y + math.sin(a) * r0, z + 0.006 * (k % 2), a, p, ln,
              width, mats[k % len(mats)], thick=thick, segs=10)
        out.append((a, ln, p))
    return out


def _wilt_fan(name, n, length, width, rng, z=0.035, a0=0.0, mats=WILT, thick=0.02):
    for k in range(n):
        a = a0 + 2 * math.pi * (k + 0.5) / n + rng.uniform(-0.3, 0.3)
        _droop("%s%d" % (name, k), CX + math.cos(a) * 0.01, CY + math.sin(a) * 0.01, z + rng.uniform(0, 0.015), a,
               length * rng.uniform(0.8, 1.1), width * rng.uniform(0.8, 1.0), mats[k % len(mats)], rng, thick)


def _seeds(name, n, size, mat, rng, spread=0.05, flat=0.55):
    """Sown: a few seeds lying loose in a short drill line, nothing else."""
    ang = rng.uniform(-0.5, 0.5)
    for k in range(n):
        t = (k - (n - 1) / 2) / max(n - 1, 1)
        x = CX + math.cos(ang) * t * spread * 2 + rng.uniform(-0.012, 0.012)
        y = CY + math.sin(ang) * t * spread * 2 + rng.uniform(-0.014, 0.014)
        s = size * rng.uniform(0.85, 1.15)
        _speck("%s%d" % (name, k), x, y, s, s * flat * 1.25, s * flat, rng.uniform(0, math.pi), mat)


def _sprout(name, n, length, width, mats, rng, h=0.02, pitch=0.35, a0=None, thick=0.016):
    """2-3 tiny seed leaves on a short stem."""
    a0 = rng.uniform(0, math.pi) if a0 is None else a0
    length, width = length * SPROUT_K, width * SPROUT_K
    _limb(name + "s", (CX, CY, 0.0), (CX, CY, h), 0.007, 0.005, "pl_crop_stem", segs=4)
    for k in range(n):
        a = a0 + 2 * math.pi * k / n + rng.uniform(-0.25, 0.25)
        _leaf("%s%d" % (name, k), CX, CY, h, a, pitch + rng.uniform(-0.1, 0.1), length * rng.uniform(0.85, 1.1),
              width, mats[k % len(mats)], thick=thick, segs=8)


# ------------------------------------------------------------------ potato
def _compound(name, p, az, pitch, length, mat, rng, lw=0.04):
    """A potato leaf: a short stalk with one pair of oval leaflets and a
    larger terminal one."""
    end = _at(p, az, pitch, length * 0.6)
    _limb(name + "p", p, end, 0.007, 0.005, "pl_crop_stem", segs=4)
    q = _at(p, az, pitch, length * 0.35)
    for sgn in (-1, 1):
        _leaf("%s%d" % (name, sgn), q[0], q[1], q[2], az + sgn * rng.uniform(0.8, 1.1), pitch * 0.5 + 0.05,
              length * 0.45, lw * 0.85, mat, thick=0.022, segs=8)
    _leaf(name + "t", end[0], end[1], end[2] - 0.004, az, pitch * 0.5, length * 0.5, lw, mat, thick=0.022, segs=10)


def _potato(st, v):
    rng = _rng("potato", st, v)
    if st == "sown":  # seed-potato pieces
        _seeds("sd", (2, 1, 2, 2)[v], 0.022, "pl_crop_tuber", rng, spread=0.035, flat=0.7)
        return
    if st == "sprout":
        _sprout("sp", 3, 0.045, 0.03, ("pl_crop_deep", "pl_crop_leaf"), rng, h=0.025, pitch=0.5)
        return
    if st == "wilted":  # slumped stalks, browned leaflets lying on the soil
        for k in range((5, 4, 6, 5)[v]):
            a = 2 * math.pi * k / 5 + rng.uniform(-0.3, 0.3) + v
            pts = [(CX, CY, 0.0), _at((CX, CY, 0.0), a, 0.9, 0.05)]
            pts.append(_at(pts[-1], a, -0.2, 0.06))
            _stem("ws%d" % k, pts, 0.008, 0.005, ("pl_dried", "pl_crop_wilt")[k % 2])
            x, y, z = pts[-1]
            _droop("wl%d" % k, x, y, max(z, 0.015), a + rng.uniform(-0.4, 0.4), 0.075, 0.05, WILT[k % len(WILT)],
                   rng)
        _wilt_fan("wi", (4, 5, 4, 5)[v], 0.07, 0.045, rng, z=0.05, a0=v)
        return
    young = st == "young"
    s = 0.6 if young else 1.0
    n = (4, 5, 4, 5)[v] if young else (7, 8, 7, 8)[v]
    a0 = rng.uniform(0, 2 * math.pi)
    if not young:  # dark leafy core so the clump never shows the soil through
        _cluster("core", CX, CY + 0.005, 0.0, 0.08, "pl_crop_deep", 7100 + v, lumps=0.3, squash=0.8)
    for k in range(n):
        a = a0 + 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        z = rng.uniform(0.02, 0.05) * s
        _compound("cl%d" % k, (CX + math.cos(a) * 0.02, CY + math.sin(a) * 0.02, z), a, rng.uniform(0.3, 0.5),
                  rng.uniform(0.15, 0.18) * s, ("pl_crop_leaf", "pl_crop_deep")[k % 2], rng,
                  lw=0.036 if young else 0.045)
    if young:
        return
    for k in range((3, 3, 4, 3)[v]):  # upright crown leaves
        a = a0 + 0.5 + 2 * math.pi * k / 3 + rng.uniform(-0.4, 0.4)
        _compound("cu%d" % k, (CX, CY, 0.07), a, rng.uniform(0.7, 0.9), 0.12, "pl_crop_leaf", rng, lw=0.042)
    if st != "harvestable":
        return
    for k in range((3, 4, 3, 4)[v]):  # small pale flowers held over the leaves
        a = a0 + 2 * math.pi * k / 4 + rng.uniform(-0.4, 0.4)
        x, y = CX + math.cos(a) * 0.045, CY + math.sin(a) * 0.045
        h = rng.uniform(0.17, 0.2)
        _limb("fs%d" % k, (CX, CY, 0.07), (x, y, h), 0.005, 0.004, "pl_crop_stem", segs=4)
        S.ellipsoid("fl%d" % k, x, y, h, 0.019, 0.019, 0.01, "pl_crop_cotton", segs=10, rings=5)
        S.ellipsoid("fc%d" % k, x, y, h + 0.008, 0.007, 0.007, 0.006, "pl_crop_corn", segs=6, rings=4)
        _top(h + 0.015)
    for k in range((2, 3, 2, 3)[v]):  # ochre tubers peeking out of the soil at the front edge
        a = -math.pi / 2 + (k - 1) * 0.95 + rng.uniform(-0.25, 0.25)
        d = rng.uniform(0.12, 0.145)
        _dome("tb%d" % k, CX + math.cos(a) * d, CY + math.sin(a) * d, 0.0, 0.036, 0.029, 0.026, "pl_crop_tuber")


# ------------------------------------------------------------------- beans
def _trifoliate(name, p, az, pitch, size, mat, rng):
    """A bean leaf: a short stalk with three broad pointed leaflets."""
    q = _at(p, az, pitch + 0.25, size * 0.3)
    _limb(name + "p", p, q, 0.006, 0.005, "pl_crop_stem", segs=4)
    _leaf(name + "t", q[0], q[1], q[2], az + rng.uniform(-0.15, 0.15), pitch, size, size * 0.68, mat, thick=0.022,
          segs=10)
    for sgn in (-1, 1):
        _leaf("%s%d" % (name, sgn), q[0], q[1], q[2], az + sgn * rng.uniform(1.1, 1.35), pitch * 0.7,
              size * 0.82, size * 0.58, mat, thick=0.022, segs=10)


def _tendril(name, p, az, length, rng, mat="pl_crop_stem", rise=0.04):
    pts = [p]
    for k in range(4):
        az += 0.9 + rng.uniform(-0.2, 0.2)
        x, y, z = pts[-1]
        step = length / 4 * (1 - 0.15 * k)
        pts.append((x + math.cos(az) * step, y + math.sin(az) * step, z + rise / 4))
    _stem(name, pts, 0.004, 0.0025, mat)


def _beans(st, v):
    rng = _rng("beans", st, v)
    if st == "sown":
        _seeds("sd", (3, 2, 3, 2)[v], 0.018, "pl_khaki", rng, spread=0.04, flat=0.6)
        return
    if st == "sprout":  # two thick round seed leaves and a first heart leaf
        a0 = rng.uniform(0, math.pi)
        _sprout("sp", 2, 0.035, 0.03, ("pl_crop_leaf", "pl_crop_leaf"), rng, h=0.035, pitch=0.15, a0=a0, thick=0.02)
        _leaf("sh", CX, CY, 0.04, a0 + math.pi / 2, 0.7, 0.06, 0.048, "pl_crop_deep", thick=0.016, segs=8)
        return
    if st == "wilted":
        for k in range((3, 4, 3, 4)[v]):
            a = 2 * math.pi * k / 3.5 + rng.uniform(-0.3, 0.3) + v
            pts = [(CX, CY, 0.0), _at((CX, CY, 0.0), a, 0.8, 0.05)]
            pts.append(_at(pts[-1], a + 0.4, -0.3, 0.06))
            _stem("ws%d" % k, pts, 0.008, 0.005, "pl_dried")
            x, y, z = pts[-1]
            for j in range(2):
                _droop("wl%d_%d" % (k, j), x, y, max(z, 0.02), a + (j - 0.5) * 1.4, 0.07, 0.048,
                       WILT[(k + j) % len(WILT)], rng)
        if v % 2:
            _limb("wpod", (CX + 0.04, CY - 0.05, 0.03), (CX + 0.1, CY - 0.08, 0.006), 0.01, 0.006, "pl_dried",
                  segs=5)
        return
    young = st == "young"
    s = 0.6 if young else 1.0
    nv = 3 if young else (4, 5, 4, 5)[v]
    mats = ("pl_crop_deep", "pl_crop_leaf", "pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf")
    nodes = []
    a0 = rng.uniform(0, 2 * math.pi)
    for k in range(nv):  # short twining runners sprawling from the crown
        a = a0 + 2 * math.pi * k / nv + rng.uniform(-0.3, 0.3)
        pts = [(CX, CY, 0.0)]
        for j in range(2):
            a += rng.uniform(-0.4, 0.4)
            x, y, z = pts[-1]
            pts.append((x + math.cos(a) * 0.06 * s, y + math.sin(a) * 0.06 * s,
                        (0.08, 0.07)[j] * s + rng.uniform(-0.01, 0.01)))
        _stem("rn%d" % k, pts, 0.008, 0.005, "pl_crop_stem")
        nodes += [(pts[j], a, j) for j in (1, 2)]
        if not young and k % 2 == 0:
            _tendril("td%d" % k, (pts[1][0], pts[1][1], pts[1][2] + 0.04), a + 0.5, 0.07, rng, rise=0.05)
    for i, (p, a, j) in enumerate(nodes):
        if young and j == 1 and i % 2:
            continue
        _trifoliate("tf%d" % i, p, a + rng.uniform(-0.7, 0.7), rng.uniform(0.15, 0.35),
                    (0.085 if j == 2 else 0.075) * (0.75 if young else 1.0), mats[i % 5], rng)
    if young:
        return
    for k in range(2):
        _trifoliate("tc%d" % k, (CX, CY, 0.09 + 0.02 * k), a0 + 1.0 + 3.0 * k, 0.65, 0.08,
                    ("pl_crop_leaf", "pl_crop_deep")[k], rng)
    if st != "harvestable":
        return
    # slim pale pods hanging out from under the leaves, mostly at the front
    for k in range((5, 6, 5, 6)[v]):
        a = -math.pi / 2 + (k - 2.5) * 0.75 + rng.uniform(-0.2, 0.2)
        r0 = rng.uniform(0.09, 0.12)
        x, y = CX + math.cos(a) * r0, CY + math.sin(a) * r0
        b = a + rng.uniform(-0.35, 0.35)
        m = (x + math.cos(b) * 0.04, y + math.sin(b) * 0.04, 0.035)
        e = (m[0] + math.cos(b + 0.25) * 0.045, m[1] + math.sin(b + 0.25) * 0.045, 0.008)
        _limb("pd%da" % k, (x, y, 0.07), m, 0.012, 0.015, "pl_crop_pod", segs=6)
        _limb("pd%db" % k, m, e, 0.015, 0.007, "pl_crop_pod", segs=6)


# ----------------------------------------------------------------- lettuce
def _lettuce(st, v):
    rng = _rng("lettuce", st, v)
    if st == "sown":
        _seeds("sd", (4, 3, 5, 4)[v], 0.011, "pl_khaki", rng, spread=0.035, flat=0.5)
        return
    if st == "sprout":
        _sprout("sp", 2 + v % 2, 0.035, 0.03, ("pl_crop_jade", "pl_crop_leaf"), rng, h=0.015, pitch=0.3)
        return
    if st == "wilted":
        _wilt_fan("wl", (6, 5, 7, 6)[v], 0.11, 0.085, rng, z=0.03)
        _dome("wc", CX, CY, 0.0, 0.035, 0.035, 0.03, "pl_crop_wilt", segs=10, rings=6)
        return
    a0 = rng.uniform(0, 2 * math.pi)
    if st == "young":
        _fan("lf", (5, 6, 5, 6)[v], 0.075, 0.065, 0.45, ("pl_crop_jade", "pl_crop_leaf"), rng, a0=a0)
        return
    if st == "mature":  # open rosette of round leaves, paler ones cupping the heart
        _fan("lo", (7, 8, 7, 8)[v], 0.12, 0.105, 0.3, ("pl_crop_leaf", "pl_crop_deep", "pl_crop_leaf"), rng, a0=a0)
        _fan("li", (4, 4, 5, 4)[v], 0.085, 0.08, 0.95, ("pl_crop_jade",), rng, z=0.025, a0=a0 + 0.5, r0=0.015)
        return
    # harvestable: a closed pale jade head sitting in a ring of darker outer leaves
    _fan("lo", (7, 7, 8, 8)[v], 0.125, 0.105, 0.18, ("pl_crop_leaf", "pl_crop_deep"), rng, a0=a0)
    r = (0.068, 0.072, 0.065, 0.07)[v]
    _dome("hd", CX, CY, r * 0.55, r, r * 0.95, r * 0.85, "pl_crop_jade", segs=14, rings=8)
    for k in range(6):  # leaves wrapped up round the head
        a = a0 + 0.3 + 2 * math.pi * k / 6 + rng.uniform(-0.15, 0.15)
        _leaf("wr%d" % k, CX + math.cos(a) * r * 0.55, CY + math.sin(a) * r * 0.55, 0.005, a,
              1.2 + rng.uniform(-0.1, 0.1), r * 1.5, r * 1.1, ("pl_crop_jade", "pl_crop_leaf")[k % 2],
              thick=0.024, segs=10)


# ------------------------------------------------------------------ radish
def _radish(st, v):
    rng = _rng("radish", st, v)
    if st == "sown":
        _seeds("sd", (3, 4, 3, 4)[v], 0.013, "pl_crop_tuber", rng, spread=0.035, flat=0.75)
        return
    if st == "sprout":  # broad seed leaves on reddish stalks
        a0 = rng.uniform(0, math.pi)
        for k in range(2 + (v == 3)):
            a = a0 + 2 * math.pi * k / 2 + rng.uniform(-0.2, 0.2)
            q = _at((CX, CY, 0.0), a, 1.2, 0.025)
            _limb("st%d" % k, (CX, CY, 0.0), q, 0.006, 0.005, "pl_crop_radish", segs=4)
            _leaf("sp%d" % k, q[0], q[1], q[2], a, 0.25, 0.045, 0.045, "pl_crop_leaf", thick=0.016, segs=8)
        return
    if st == "wilted":
        _wilt_fan("wl", (6, 5, 7, 6)[v], 0.12, 0.05, rng, z=0.03)
        if v % 2 == 0:
            _dome("sh", CX, CY - 0.01, 0.0, 0.035, 0.032, 0.02, "pl_crop_wilt", segs=10, rings=6)
        return
    young = st == "young"
    s = 0.6 if young else 1.0
    n = (5, 6, 5, 6)[v] if young else (8, 9, 7, 9)[v]
    a0 = rng.uniform(0, 2 * math.pi)
    span = 2 * math.pi
    if st == "harvestable":  # leaves fan up and back, leaving the shoulder bare in front
        a0, span = -math.pi / 2 + 0.55, 2 * math.pi - 1.1
    for k in range(n):  # leaf fountain: reddish stalks rise, blades arch over
        a = a0 + span * (k + 0.5) / n + rng.uniform(-0.2, 0.2)
        q = _at((CX + math.cos(a) * 0.008, CY + math.sin(a) * 0.008, 0.01), a, rng.uniform(1.1, 1.35), 0.075 * s)
        _limb("pt%d" % k, (CX, CY, 0.01), q, 0.006, 0.005, ("pl_crop_radish", "pl_crop_stem")[k % 2], segs=4)
        _leaf("bl%d" % k, q[0], q[1], q[2], a, rng.uniform(0.1, 0.45), 0.115 * s * rng.uniform(0.85, 1.1),
              0.055 * s, ("pl_crop_leaf", "pl_crop_deep", "pl_crop_leaf")[k % 3], thick=0.02, segs=10)
    if st == "harvestable":  # the dusty rose shoulder pushing up at the soil line, in front
        r = (0.046, 0.05, 0.044, 0.048)[v]
        _dome("sh", CX, CY - 0.022, 0.0, r, r * 0.9, r * 0.8, "pl_crop_radish", segs=14, rings=8)


# ----------------------------------------------------------------- pumpkin
def _vine(name, a, segs, step, rng, s=1.0, z=0.012):
    pts = [(CX, CY, 0.0)]
    for j in range(segs):
        a += rng.uniform(-0.4, 0.4)
        x, y, _ = pts[-1]
        pts.append((x + math.cos(a) * step * s, y + math.sin(a) * step * s, z + rng.uniform(0, 0.012)))
    return pts, a


def _pleaf(name, p, az, size, mat, pitch=0.3, stalk=0.05):
    """A big lobed pumpkin leaf (five overlapping lobes) on an upright stalk."""
    q = (p[0] + math.cos(az) * 0.01, p[1] + math.sin(az) * 0.01, p[2] + stalk)
    _limb(name + "p", p, q, 0.007, 0.006, "pl_crop_stem", segs=4)
    _leaf(name + "c", q[0], q[1], q[2], az, pitch, size, size * 0.66, mat, thick=0.024, segs=10)
    for sgn in (-1, 1):
        _leaf("%sa%d" % (name, sgn), q[0], q[1], q[2], az + sgn * 0.72, pitch * 0.9, size * 0.85, size * 0.58, mat,
              thick=0.024, segs=10)
        _leaf("%sb%d" % (name, sgn), q[0], q[1], q[2] - 0.003, az + sgn * 1.42, pitch * 0.75, size * 0.6,
              size * 0.46, mat, thick=0.024, segs=8)
    return q


def _pumpkin_fruit(name, x, y, r, v):
    """A squat ribbed pumpkin: four crossed ellipsoids give eight soft lobes."""
    for k in range(4):
        _rib("%s%d" % (name, k), x, y, r * 0.78, k * math.pi / 4 + 0.2 * v, r, r * 0.7, r * 0.78,
             "pl_crop_pumpkin", segs=14)
    _limb(name + "s", (x, y, r * 1.45), (x + r * 0.18, y + r * 0.08, r * 1.75), r * 0.13, r * 0.1, "pl_stem",
          segs=5)


def _pumpkin(st, v):
    rng = _rng("pumpkin", st, v)
    if st == "sown":
        _seeds("sd", (3, 2, 3, 2)[v], 0.02, "pl_khaki", rng, spread=0.04, flat=0.4)
        return
    if st == "sprout":  # two big oval seed leaves and a first rough leaf
        a0 = rng.uniform(0, math.pi)
        _sprout("sp", 2, 0.06, 0.04, ("pl_crop_leaf",), rng, h=0.025, pitch=0.2, a0=a0, thick=0.018)
        if v % 2:
            _leaf("tl", CX, CY, 0.03, a0 + math.pi / 2, 0.6, 0.07, 0.06, "pl_crop_deep", thick=0.016, segs=8)
        return
    wilt = st == "wilted"
    young = st == "young"
    s = {"young": 0.55, "mature": 1.0, "harvestable": 1.0, "wilted": 0.85}[st]
    nv = 2 if young else (3, 4, 3, 4)[v]
    a0 = rng.uniform(0, 2 * math.pi)
    nodes = []
    for k in range(nv):
        a = a0 + 2 * math.pi * k / nv + rng.uniform(-0.35, 0.35)
        pts, a = _vine("vn%d" % k, a, 2, 0.105, rng, s)
        _stem("vn%d" % k, pts, 0.012, 0.007, "pl_dried" if wilt else "pl_crop_stem")
        nodes += [(pts[j], a, j) for j in range(1, len(pts))]
        if not young and not wilt and k % 2 == 0 and st != "harvestable":
            _tendril("td%d" % k, pts[-1], a, 0.06, rng, rise=0.02)
    if wilt:  # vines lying bare, a few shrivelled leaves fallen flat
        for i, (p, a, j) in enumerate(nodes):
            if i % 3 == 2:
                continue
            q = (p[0], p[1], 0.04)
            for sgn in (-1, 1):
                _droop("wl%d_%d" % (i, sgn), q[0], q[1], q[2], a + sgn * 0.6 + rng.uniform(-0.3, 0.3), 0.09, 0.065,
                       WILT[(i + sgn) % len(WILT)], rng)
        return
    mats = ("pl_crop_leaf", "pl_crop_deep", "pl_crop_leaf", "pl_crop_deep")
    # a crown of big leaves over the root, smaller ones along the runners
    for k in range(2 if young else 3):
        _pleaf("cl%d" % k, (CX, CY, 0.0), a0 + 0.9 + 2.1 * k, 0.15 * s, ("pl_crop_deep", "pl_crop_leaf")[k % 2],
               pitch=0.35, stalk=(0.07 + 0.02 * k) * s)
    for i, (p, a, j) in enumerate(nodes):
        side = 1 if i % 2 else -1
        _pleaf("pl%d" % i, p, a + side * rng.uniform(0.4, 0.9), rng.uniform(0.11, 0.13) * s, mats[i % 4],
               pitch=rng.uniform(0.2, 0.4), stalk=rng.uniform(0.04, 0.06) * s)
    if st == "mature":
        for k in range((1, 2, 1, 2)[v]):  # a few muted gold blossoms, no fruit yet
            p, a, j = nodes[(2 * k + 1) % len(nodes)]
            S.ellipsoid("bf%d" % k, p[0], p[1] - 0.02, 0.075, 0.024, 0.024, 0.016, "pl_crop_corn", segs=10, rings=5)
            _top(0.09)
        return
    if st != "harvestable":
        return
    # one or two burnt-orange pumpkins at the runner tips, a leaf half over each
    for k in range((2, 1, 2, 1)[v]):
        p, a, j = nodes[(2 * (k + v) + 1) % len(nodes)]
        r = (0.085, 0.092, 0.08, 0.088)[v] * (0.85 if k else 1.0)
        b = a + (0.8 if k else -0.8)
        x, y = p[0] + math.cos(b) * 0.04, p[1] + math.sin(b) * 0.04
        _pumpkin_fruit("pk%d" % k, x, y, r, v + k)
        c = math.atan2(y - CY, x - CX)  # from the crown side, reaching over the fruit's inner flank
        _pleaf("ov%d" % k, (x - math.cos(c) * r * 1.5, y - math.sin(c) * r * 1.5, 0.0), c + 0.5, 0.1,
               "pl_crop_leaf", pitch=0.15, stalk=r * 1.75)


# ---------------------------------------------------------------- healroot
def _umbel(name, x, y, h, rng, mat="pl_crop_bloom", n=5, r=0.014):
    """A small flat flower head of florets on top of a stalk."""
    for k in range(n):
        a = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3)
        d = 0.0 if k == 0 else 0.02
        S.ellipsoid("%s%d" % (name, k), x + math.cos(a) * d, y + math.sin(a) * d, h - (0.004 if k else -0.004),
                    r, r, r * 0.75, mat, segs=8, rings=5)
    _top(h + r)


def herb(name, st, v, rng, mats, bloom=True, s=1.0):
    """The healroot herb: lance leaves round a centre, thin flower stalks
    over them carrying pale blue-white umbels (green buds while unripe)."""
    young = st == "young"
    n = (5, 6, 5, 6)[v] if young else (8, 9, 7, 10)[v]
    a0 = rng.uniform(0, 2 * math.pi)
    _fan(name + "lo", n, 0.13 * s, 0.045 * s, 0.42, mats, rng, a0=a0)
    if young:
        return
    _fan(name + "li", max(n // 2, 3), 0.1 * s, 0.04 * s, 0.95, mats[::-1], rng, z=0.02, a0=a0 + 0.4, r0=0.006)
    k_n = (3, 4, 3, 4)[v] if bloom else (2, 3, 2, 3)[v]
    for k in range(k_n):
        a = a0 + 2 * math.pi * k / k_n + rng.uniform(-0.4, 0.4)
        lean = rng.uniform(0.05, 0.08) * s
        h = rng.uniform(0.18, 0.23) * s
        x, y = CX + math.cos(a) * lean, CY + math.sin(a) * lean
        _limb(name + "fs%d" % k, (CX, CY, 0.01), (x, y, h), 0.006, 0.004, "pl_crop_stem", segs=4)
        if bloom:
            _umbel(name + "fl%d_" % k, x, y, h, rng)
        else:
            S.ellipsoid(name + "bd%d" % k, x, y, h, 0.012, 0.012, 0.012, "pl_crop_jade", segs=6, rings=4)
            _top(h + 0.012)


def _healroot(st, v):
    rng = _rng("healroot", st, v)
    if st == "sown":
        _seeds("sd", (4, 3, 4, 5)[v], 0.012, "pl_khaki", rng, spread=0.035, flat=0.6)
        return
    if st == "sprout":
        _sprout("sp", 3, 0.045, 0.02, ("pl_crop_leaf", "pl_crop_deep"), rng, h=0.012, pitch=0.6)
        return
    if st == "wilted":
        _wilt_fan("wl", (6, 7, 5, 7)[v], 0.12, 0.04, rng, z=0.035)
        for k in range(v % 2 + 1):  # a dry flower stalk keeled over
            a = rng.uniform(0, 6.3)
            _limb("ws%d" % k, (CX, CY, 0.01), _at((CX, CY, 0.01), a, 0.25, 0.15), 0.005, 0.004, "pl_dried", segs=4)
        return
    mats = ("pl_crop_leaf", "pl_crop_deep", "pl_crop_leaf", "pl_crop_stem")
    herb("", st, v, rng, mats, bloom=st == "harvestable", s=0.6 if st == "young" else 1.0)


def healroot_wild(v, stage):
    """Wild healroot (plant.py ground kind): the crop's herb in the wild
    palette, a little larger, flowering when mature."""
    rng = random.Random(30 + v)
    if stage == "dormant":
        n = (6, 5, 7, 6)[v]
        _wilt_fan("dl", n, 0.2, 0.06, rng, z=0.04, mats=("pl_dried", "pl_stem", "pl_dried", "pl_khaki"))
        _limb("ds", (CX, CY, 0.01), _at((CX, CY, 0.01), rng.uniform(0, 6.3), 0.6, 0.18), 0.007, 0.005, "pl_dried",
              segs=4)
        return
    mats = ("pl_grass", "pl_forest", "pl_grass", "pl_sage")
    herb("", stage, v, rng, mats, bloom=True, s=1.45 if stage == "mature" else 1.2)


# ----------------------------------------------------------------- ashleaf
def _ashleaf(st, v):
    rng = _rng("ashleaf", st, v)
    if st == "sown":
        _seeds("sd", (3, 4, 3, 4)[v], 0.014, "pl_crop_silver", rng, spread=0.035, flat=0.55)
        return
    if st == "sprout":
        _sprout("sp", 2 + (v % 2), 0.042, 0.03, ("pl_crop_silver", "pl_sage"), rng, h=0.012, pitch=0.35)
        return
    if st == "wilted":
        _wilt_fan("wl", (6, 5, 7, 6)[v], 0.12, 0.065, rng, z=0.03,
                  mats=("pl_khaki", "pl_dried", "pl_crop_wilt", "pl_khaki", "pl_dry"))
        return
    a0 = rng.uniform(0, 2 * math.pi)
    if st == "young":
        _fan("lf", (5, 6, 5, 6)[v], 0.08, 0.045, 0.4, ("pl_sage", "pl_crop_leaf", "pl_crop_silver"), rng, a0=a0)
        return
    if st == "mature":  # broad leaves still greener than silver
        _fan("lo", (7, 8, 7, 8)[v], 0.135, 0.068, 0.3, ("pl_crop_leaf", "pl_sage", "pl_crop_deep", "pl_sage"), rng,
             a0=a0)
        _fan("li", (4, 4, 5, 4)[v], 0.095, 0.058, 0.85, ("pl_crop_silver", "pl_sage"), rng, z=0.025, a0=a0 + 0.4,
             r0=0.008)
        return
    # harvestable: a full rosette of broad silver-sage leaves
    _fan("lo", (8, 9, 8, 10)[v], 0.15, 0.085, 0.26, ("pl_crop_silver", "pl_sage", "pl_crop_silver", "pl_crop_deep"),
         rng, a0=a0)
    _fan("li", (5, 5, 6, 5)[v], 0.11, 0.075, 0.8, ("pl_crop_silver", "pl_crop_silver", "pl_sage"), rng, z=0.03,
         a0=a0 + 0.35, r0=0.01)


BUILDERS = {
    "potato": _potato,
    "beans": _beans,
    "lettuce": _lettuce,
    "radish": _radish,
    "pumpkin": _pumpkin,
    "healroot": _healroot,
    "ashleaf": _ashleaf,
}
