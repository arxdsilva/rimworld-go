"""Grain and tall crops (PLAY-016, ART spec §9.3): corn, wheat, brewgrain,
cotton. Owned by the ART-GRAINS agent. BUILDERS maps species -> fn(stage,
v); species absent here still use the legacy builders in plant.py.

One builder call is ONE unit (crop_batch.go puts 2 corn stalks, 4 wheat or
brewgrain clumps, 3 cotton plants per tile): a single corn stalk, a clump
of 5-8 grain stems, one small cotton shrub. A variant keeps its structure
(leaf azimuths, stem count, lean) through every stage, so a unit grows
rather than swaps design. Sizes are at render scale (1 tile = 1 m, the
shear draws height at half): corn is the tallest (~0.8 tile on screen),
grain clumps sit just above tall grass, cotton is a small shrub.
"""
import math
import random

import bmesh
from mathutils import Vector

from .. import stage as S
from .plant_base import CX, CY, _ball, _cluster, _leaf, _limb, _top

STAGE_IDX = {"sown": 0, "sprout": 1, "young": 2, "mature": 3, "harvestable": 4, "wilted": 5}
SEED = {"corn": 4100, "wheat": 4200, "brewgrain": 4300, "cotton": 4400}
UP = Vector((0.0, 0.0, 1.0))


def _rng(sp, v, salt=0):
    """Structure RNG: the same per variant at every stage."""
    return random.Random(SEED[sp] + 17 * v + 1000 * salt)


# ------------------------------------------------------------ geometry
def _ribbon(name, pts, widths, mat, thick=0.012, az=0.0):
    """Strap leaf along a polyline: a flattened diamond tube whose width
    follows `widths` and ends in a point at pts[-1]. The low midrib ridge
    splits each leaf into a lit and a shaded half under the toon light."""
    pts = [Vector(p) for p in pts]
    ref = Vector((math.sin(az), -math.cos(az), 0.0))
    wmax = max(widths) or 1.0
    bm = bmesh.new()
    rings = []
    for i, p in enumerate(pts[:-1]):
        t = (pts[i + 1] - pts[max(i - 1, 0)]).normalized()
        s = t.cross(UP)
        if s.length < 0.25:
            s = ref.copy()
        s.normalize()
        if s.dot(ref) < 0:
            s = -s
        up = s.cross(t).normalized()
        if up.z < 0:
            up = -up
        w = widths[i] / 2
        h = thick * (0.35 + 0.65 * widths[i] / wmax)
        rings.append([bm.verts.new(p + s * w), bm.verts.new(p + up * h), bm.verts.new(p - s * w),
                      bm.verts.new(p - up * h * 0.5)])
        _top(p.z + h)
    tip = bm.verts.new(pts[-1])
    _top(pts[-1].z)
    for a, b in zip(rings, rings[1:]):
        for k in range(4):
            bm.faces.new((a[k], a[(k + 1) % 4], b[(k + 1) % 4], b[k]))
    for k in range(4):
        bm.faces.new((rings[-1][k], rings[-1][(k + 1) % 4], tip))
    bm.faces.new(list(reversed(rings[0])))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces)
    return S._obj(name, bm, mat, smooth=True)


def _bez(p0, p1, p2, t):
    a = 1.0 - t
    return p0 * (a * a) + p1 * (2 * a * t) + p2 * (t * t)


def _strap(name, x, y, z, az, length, width, rise, end, mat, n=7, thick=0.012, base=0.6, shoulder=0.25):
    """Arching strap leaf from (x, y, z) out along azimuth az: it climbs to
    about `rise` x length at its shoulder and ends `end` x length above
    its base (negative: the tip hangs below the attachment)."""
    d = Vector((math.cos(az), math.sin(az), 0.0))
    p0 = Vector((x, y, z))
    p1 = p0 + d * (length * 0.32) + UP * (length * rise)
    p2 = p0 + d * (length * 0.86) + UP * (length * end)
    pts = [_bez(p0, p1, p2, i / n) for i in range(n + 1)]
    ws = []
    for i in range(n):
        t = i / n
        ws.append(width * (1.0 - t ** 1.6) * (base + (1.0 - base) * min(1.0, t / shoulder)))
    return _ribbon(name, pts, ws, mat, thick=thick, az=az)


def _stem(name, pts, r0, r1, mat, segs=5):
    """Tapered stem through a polyline."""
    n = len(pts) - 1
    for k in range(n):
        ra = r0 + (r1 - r0) * k / n
        rb = r0 + (r1 - r0) * (k + 1) / n
        _limb("%s%d" % (name, k), pts[k], pts[k + 1], ra, rb, mat, segs=segs)


def _curve(p0, d, h, lean, bend=0.0, n=3):
    """Points of a stem rising h from p0 that leans `lean` along d,
    bending over by `bend` (tip drop) at the top."""
    out = []
    for i in range(n + 1):
        t = i / n
        out.append(Vector((p0[0] + d[0] * lean * t * t, p0[1] + d[1] * lean * t * t,
                           p0[2] + h * t - bend * t ** 3)))
    return out


# ---------------------------------------------------------------- sown
SOWN = {  # seed material, length, width
    "corn": ("pl_crop_corn", 0.034, 0.026),
    "wheat": ("pl_crop_straw", 0.03, 0.019),
    "brewgrain": ("pl_crop_bronze", 0.032, 0.021),
    "cotton": ("pl_khaki", 0.03, 0.022),
}


def _sown(sp, v):
    """A few seed specks pressed into bare ground: no mound, no plant."""
    rng = _rng(sp, v, 9)
    mat, ln, wd = SOWN[sp]
    n = (3, 4, 2, 5)[v]
    row = rng.uniform(-0.4, 0.4)
    for k in range(n):
        t = (k - (n - 1) / 2) * 0.05 + rng.uniform(-0.015, 0.015)
        x = CX + math.cos(row) * t + rng.uniform(-0.012, 0.012)
        y = CY + math.sin(row) * t * 0.6 + rng.uniform(-0.02, 0.02)
        s = rng.uniform(0.8, 1.1)
        _leaf("seed%d" % k, x, y, 0.0, rng.uniform(0, math.pi), 0.0, ln * s, wd * s, mat,
              thick=wd * 0.7 * s, segs=6)


# -------------------------------------------------------------- sprouts
def _blade_sprouts(sp, v, n, length, width, mats, rise=1.0, end=0.75):
    rng = _rng(sp, v, 8)
    a0 = rng.uniform(0, 2 * math.pi)
    for k in range(n):
        a = a0 + 2 * math.pi * k / n + rng.uniform(-0.4, 0.4)
        _strap("sp%d" % k, CX + math.cos(a) * 0.006, CY + math.sin(a) * 0.006, 0.0, a,
               length * rng.uniform(0.8, 1.15), width, rise + rng.uniform(-0.15, 0.15),
               end + rng.uniform(-0.25, 0.1), mats[k % len(mats)], n=5, thick=0.008)


# ------------------------------------------------------------------ corn
CORN_H = {"young": 0.46, "mature": 1.2, "harvestable": 1.26, "wilted": 0.92}
CORN_V = ((7, 0.15, 1.04), (8, -0.3, 1.1), (6, 0.45, 0.94), (7, -0.1, 1.0))  # leaves, azimuth, height


def _corn_leaves(rng, v, n):
    """Distichous leaves (alternate sides), with jitter and a variant turn."""
    _, az0, _ = CORN_V[v]
    out = []
    for k in range(n):
        side = 0.0 if (k + v) % 2 == 0 else math.pi
        a = az0 + side + rng.uniform(-0.5, 0.5)
        # nudge a few leaves toward / away from the viewer so the fan is not flat
        if k % 3 == 2:
            a += (0.6 if side == 0.0 else -0.6) * (1 if v % 2 else -1)
        out.append((a, rng.uniform(0.85, 1.12), rng.uniform(-0.1, 0.1)))
    return out


def _corn(st, v):
    rng = _rng("corn", v)
    if st == "sown":
        return _sown("corn", v)
    if st == "sprout":
        return _blade_sprouts("corn", v, (2, 3, 2, 3)[v], 0.17, 0.038, ("pl_crop_jade", "pl_crop_leaf"),
                              rise=0.85, end=0.45)
    n, az0, hs = CORN_V[v]
    leaves = _corn_leaves(rng, v, n)
    wilt = st == "wilted"
    H = CORN_H[st] * hs
    lean_az = az0 + (math.pi / 2 if v % 2 else -math.pi / 2) * 0.6
    ld = (math.cos(lean_az), math.sin(lean_az))
    if st == "young":
        # half size: a fountain of leaves round a short hidden stalk
        _stem("stalk", _curve((CX, CY, 0.0), ld, H * 0.5, 0.0), 0.02, 0.014, "pl_crop_stem")
        for k, (a, ls, j) in enumerate(leaves[:5]):
            t = k / 4.0
            _strap("lf%d" % k, CX, CY, H * (0.04 + 0.4 * t), a, (0.3 - 0.06 * t) * ls, 0.045,
                   0.65 + 0.4 * t + j, 0.2 + 0.35 * t, ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf",
                                                        "pl_crop_leaf", "pl_crop_jade")[k], thick=0.012)
        return
    lean = (0.03 + 0.02 * (v % 2)) if not wilt else 0.24
    bend = 0.0 if not wilt else H * 0.28
    stalk = _curve((CX, CY, 0.0), ld, H, lean, bend, n=4)
    _stem("stalk", stalk, 0.026, 0.012, "pl_crop_wilt" if wilt else "pl_crop_stem", segs=6)

    def at(t):  # point on the stalk at fraction t
        f = t * 4
        i = min(int(f), 3)
        return stalk[i].lerp(stalk[i + 1], f - i)

    for k, (a, ls, j) in enumerate(leaves):
        t = k / (n - 1)
        p = at(0.07 + 0.7 * t)
        L = 0.42 * ls * (0.78 + 0.3 * math.sin(math.pi * (0.25 + 0.75 * t)))
        if wilt:
            mat = WILT[(k + v) % len(WILT)]
            _strap("lf%d" % k, p.x, p.y, p.z, a, L * 0.85, 0.042, 0.12 + j * 0.5, -0.55 - 0.2 * (1 - t), mat,
                   thick=0.01)
            continue
        mat = "pl_crop_deep" if k < 2 else ("pl_crop_leaf" if k < n - 1 else "pl_crop_jade")
        if st == "harvestable" and k == 0:
            mat = "pl_dry"   # the lowest leaf dries as the ears ripen
        _strap("lf%d" % k, p.x, p.y, p.z, a, L, 0.056, 0.42 + 0.45 * t + j, -0.32 + 0.55 * t + j, mat,
               thick=0.014)
    top = stalk[-1]
    if wilt:
        # the crown collapses into a limp brown curl
        _strap("crown", top.x, top.y, top.z, lean_az, 0.18, 0.035, 0.2, -0.5, "pl_dried", thick=0.01)
        return
    # crown: two narrow upright leaves wrapping the growing tip
    for k in range(2):
        a = az0 + k * math.pi + rng.uniform(-0.3, 0.3)
        _strap("cr%d" % k, top.x, top.y, top.z - 0.05, a, 0.2 - 0.04 * k, 0.04, 1.1, 0.75,
               "pl_crop_jade" if k == 0 else "pl_crop_leaf", thick=0.01)
    if st != "harvestable":
        return
    # tassel: a straw spike with drooping side branches
    tz = top.z + 0.02
    _limb("tsp", (top.x, top.y, tz), (top.x + ld[0] * 0.01, top.y + ld[1] * 0.01, tz + 0.16), 0.008, 0.003,
          "pl_crop_straw", segs=4)
    nb = (5, 6, 4, 5)[v]
    for k in range(nb):
        a = az0 + 2 * math.pi * k / nb + rng.uniform(-0.3, 0.3)
        z0 = tz + 0.05 + 0.025 * (k % 3)
        _stem("ts%d_" % k, [Vector((top.x, top.y, z0)),
                            Vector((top.x + math.cos(a) * 0.05, top.y + math.sin(a) * 0.04, z0 + 0.05)),
                            Vector((top.x + math.cos(a) * 0.1, top.y + math.sin(a) * 0.08, z0 + 0.01))],
              0.009, 0.004, "pl_crop_straw" if k % 2 else "pl_crop_bronze", segs=4)
    # the cob(s): a gold ear half out of its pale husk, leaning off the stalk
    cobs = ((0.48, -math.pi / 2 + 0.35 * (1 if v % 2 else -1)),)
    if v == 1:
        cobs += ((0.6, math.pi / 2 - 0.4),)
    for k, (t, ca) in enumerate(cobs):
        p = at(t)
        pitch = 0.75
        d = Vector((math.cos(ca) * math.cos(pitch), math.sin(ca) * math.cos(pitch), math.sin(pitch)))
        base = p + d * 0.015
        _leaf("husk%d" % k, base.x, base.y, base.z, ca, pitch, 0.13, 0.06, "pl_crop_jade", thick=0.055, segs=8)
        g = base + d * 0.05
        _leaf("cob%d" % k, g.x, g.y, g.z, ca, pitch - 0.05, 0.17, 0.068, "pl_crop_corn", thick=0.064, segs=10)
        for sgn in (-1, 1):  # husk leaves peeling back either side of the cob
            _strap("hk%d%d" % (k, sgn), base.x, base.y, base.z + 0.01, ca + sgn * 0.7, 0.14, 0.042, 0.8, 0.4,
                   "pl_crop_jade" if sgn < 0 else "pl_crop_leaf", thick=0.01)
        tip = g + d * 0.16
        for j in range(3):  # silk
            _limb("silk%d%d" % (k, j), tip, tip + Vector((math.cos(ca + j - 1) * 0.03, math.sin(ca + j - 1) * 0.03,
                                                          -0.03)), 0.006, 0.002, "pl_crop_bronze", segs=3)


# ---------------------------------------------------------- wheat family
GRAIN = {
    # height per stage, stems per variant, head (length, width), head colour green / ripe,
    # ripe stem colours, how far ripe heads bow (rad), awn length and count
    "wheat": {"H": {"young": 0.34, "mature": 0.6, "harvestable": 0.62, "wilted": 0.46},
              "n": (7, 8, 6, 8), "head": (0.13, 0.04), "green": "pl_crop_jade", "ripe": "pl_crop_straw",
              "stem": ("pl_crop_straw", "pl_khaki", "pl_crop_straw"), "droop": 1.0, "awn": (0.035, 2)},
    "brewgrain": {"H": {"young": 0.3, "mature": 0.52, "harvestable": 0.54, "wilted": 0.42},
                  "n": (7, 6, 8, 7), "head": (0.12, 0.05), "green": "pl_crop_jade", "ripe": "pl_crop_bronze",
                  "stem": ("pl_crop_straw", "pl_crop_straw", "pl_khaki"), "droop": 1.75, "awn": (0.09, 3)},
}
# dying foliage: browned, with a little dull green left, lighter than the soil
WILT = ("pl_crop_wilt", "pl_dried", "pl_khaki", "pl_crop_wilt", "pl_dry")


def _grain(sp, st, v):
    """A clump of 5-8 fine stems from a narrow base with grassy leaves at
    the foot and one on each stem; heads green and upright when mature,
    golden and bowed when ripe (brewgrain: heavier bronze heads nodding
    on long awns)."""
    if st == "sown":
        return _sown(sp, v)
    g = GRAIN[sp]
    if st == "sprout":
        return _blade_sprouts(sp, v, (3, 2, 3, 3)[v], 0.14, 0.022, ("pl_crop_leaf", "pl_crop_jade", "pl_crop_deep"),
                              rise=1.1, end=0.8)
    rng = _rng(sp, v)
    n = g["n"][v]
    H = g["H"][st] * (0.95, 1.04, 0.9, 1.0)[v]
    ripe = st == "harvestable"
    wilt = st == "wilted"
    stems = []
    a0 = rng.uniform(0, 2 * math.pi)
    for k in range(n):
        a = a0 + 2 * math.pi * k / n + rng.uniform(-0.35, 0.35)
        stems.append((a, rng.uniform(0.005, 0.035), rng.uniform(0.84, 1.06), rng.uniform(0.06, 0.15),
                      rng.uniform(0.9, 1.1)))
    # foot leaves: grassy blades arching out of the clump
    nl = (5, 6, 5, 6)[v] + (2 if st == "young" else 0)
    for k in range(nl):
        a = a0 + 0.4 + 2 * math.pi * k / nl + rng.uniform(-0.4, 0.4)
        if wilt:
            mat = WILT[k % len(WILT)]
        elif ripe:
            mat = ("pl_crop_straw", "pl_dry", "pl_crop_stem")[k % 3]
        else:
            mat = ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf", "pl_crop_jade")[k % 4]
        L = 0.26 * rng.uniform(0.8, 1.15) * (1.0 if st == "young" else H / 0.55)
        rise, end = (1.1, 0.75) if st == "young" else (0.85, 0.3)
        if wilt or ripe:
            rise, end = (0.45, -0.2) if wilt else (0.7, 0.12)
        _strap("bl%d" % k, CX + math.cos(a) * 0.01, CY + math.sin(a) * 0.01, 0.0, a, L, 0.026,
               rise + rng.uniform(-0.15, 0.15), end + rng.uniform(-0.15, 0.1), mat, n=5, thick=0.009)
    if st == "young":
        return
    hl0, hw0 = g["head"]
    for k, (a, rr, hs, lean, droop) in enumerate(stems):
        d = (math.cos(a), math.sin(a))
        x, y = CX + d[0] * rr, CY + d[1] * rr
        h = H * hs
        bend = 0.0
        if wilt:
            lean, bend = lean * 2.4, h * 0.32
        elif ripe:
            bend = 0.03 * g["droop"]
        pts = _curve((x, y, 0.0), d, h, lean, bend, n=3)
        if wilt:
            smat = WILT[(k + 1) % len(WILT)]
        elif ripe:
            smat = g["stem"][k % 3] if k % 4 else "pl_crop_stem"
        else:
            smat = "pl_crop_stem" if k % 3 else "pl_crop_leaf"
        _stem("st%d_" % k, pts, 0.012, 0.008, smat, segs=4)
        # one leaf off every other stem, low down, arching away from the clump
        if k % 2 == 0:
            p = pts[1]
            la = a + (0.8 if k % 4 else -0.8)
            lmat = WILT[k % len(WILT)] if wilt else (("pl_dry", "pl_crop_straw")[k % 2] if ripe else
                                                      ("pl_crop_leaf", "pl_crop_jade")[(k // 2) % 2])
            _strap("sl%d" % k, p.x, p.y, p.z, la, 0.17 * H / 0.55, 0.024, 0.15 if wilt else 0.5,
                   -0.45 if wilt else 0.0, lmat, n=5, thick=0.008)
        top = pts[-1]
        hl, hw = hl0, hw0
        if ripe:
            pitch = 1.25 - g["droop"] * droop
            hmat = g["ripe"]
        elif wilt:
            pitch = -0.9
            hmat = ("pl_crop_wilt", "pl_dried")[k % 2]
            hl, hw = hl * 0.8, hw * 0.75
        else:
            pitch = 1.35 - g["droop"] * 0.2
            hmat = g["green"]
            hl, hw = hl * 0.88, hw * 0.85
        hd = Vector((d[0] * math.cos(pitch), d[1] * math.cos(pitch), math.sin(pitch)))
        neck = top + hd * 0.01
        _leaf("hd%d" % k, neck.x, neck.y, neck.z, a, pitch, hl, hw, hmat, thick=hw * 0.92, segs=8)
        awn, na = g["awn"]
        if not wilt:  # whiskers fanning off the head
            tip = neck + hd * hl * 0.85
            amat = "pl_crop_straw" if ripe else "pl_crop_jade"
            for j in range(na):
                aa = a + (j - (na - 1) / 2) * 0.6
                ad = Vector((math.cos(aa) * math.cos(pitch - 0.1), math.sin(aa) * math.cos(pitch - 0.1),
                             math.sin(pitch - 0.1)))
                _limb("aw%d_%d" % (k, j), tip - hd * 0.02, tip + ad * awn, 0.005, 0.0018, amat, segs=3)


# ---------------------------------------------------------------- cotton
COTTON_H = {"young": 0.36, "mature": 0.66, "harvestable": 0.68, "wilted": 0.5}
COTTON_V = ((3, 0.3, 1.0), (4, -0.4, 1.06), (3, 1.2, 0.92), (4, 2.2, 1.0))  # branches, azimuth, height


def _palm(name, x, y, z, az, pitch, size, mat, mat2=None):
    """Lobed cotton leaf: a broad centre lobe with two side lobes."""
    _leaf(name, x, y, z, az, pitch, size, size * 0.62, mat, thick=0.022, segs=7)
    for sgn in (-1, 1):
        _leaf(name + "%d" % sgn, x, y, z - 0.004, az + sgn * 0.62, pitch - 0.08, size * 0.74, size * 0.48,
              mat2 or mat, thick=0.02, segs=6)


def _boll(name, x, y, z, r, seed):
    """An open cotton boll: a soft lumpy cream puff over a dark bract cup."""
    _ball(name + "c", x, y + r * 0.2, z - r * 0.5, r * 0.8, r * 0.7, r * 0.4, "pl_stem", segs=7, rings=4)
    _cluster(name + "a", x, y, z, r, "pl_crop_cotton", seed, lumps=0.3, squash=0.85)
    _cluster(name + "b", x + r * 0.45, y - r * 0.3, z - r * 0.2, r * 0.7, "pl_crop_cotton", seed + 1, lumps=0.25,
             squash=0.85)


def _cotton(st, v):
    """One small branching shrub: a woody stem, a few rising side branches
    tipped with lobed leaves; green bolls at maturity, cream bolls bursting
    out at harvest while the oldest leaves yellow."""
    if st == "sown":
        return _sown("cotton", v)
    rng = _rng("cotton", v)
    if st == "sprout":
        # two rounded seed leaves and a first true leaf on a short stem
        a = rng.uniform(0, math.pi)
        _limb("sst", (CX, CY, 0.0), (CX, CY, 0.06), 0.009, 0.007, "pl_crop_stem", segs=4)
        for sgn in (0, 1):
            _leaf("cot%d" % sgn, CX, CY, 0.06, a + sgn * math.pi, 0.18, 0.08, 0.058,
                  ("pl_crop_leaf", "pl_crop_jade")[sgn], thick=0.016, segs=7)
        if v % 2:
            _leaf("tl", CX, CY, 0.065, a + math.pi / 2, 0.7, 0.065, 0.05, "pl_crop_jade", thick=0.014, segs=6)
        return
    nb, az0, hs = COTTON_V[v]
    H = COTTON_H[st] * hs
    wilt = st == "wilted"
    ripe = st == "harvestable"
    lean_az = az0 + 1.3
    ld = (math.cos(lean_az), math.sin(lean_az))
    stem = _curve((CX, CY, 0.0), ld, H, 0.03 if not wilt else 0.12, H * 0.15 if wilt else 0.0, n=3)
    _stem("stem", stem, 0.024, 0.012, "pl_stem" if not wilt else "pl_dried", segs=6)
    if st == "young":
        for k in range(5):
            t = 0.35 + 0.65 * k / 4
            p = stem[0].lerp(stem[-1], t)
            a = az0 + 2.4 * k + rng.uniform(-0.3, 0.3)
            _palm("yl%d" % k, p.x, p.y, p.z, a, 0.3 + 0.15 * k / 4, 0.125,
                  ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf", "pl_crop_jade", "pl_crop_leaf")[k])
        return
    # low leaves on the main stem keep the shrub full to the ground
    for j in range(2):
        p = stem[0].lerp(stem[-1], 0.14 + 0.1 * j)
        la = az0 + math.pi * (j + 0.5) + rng.uniform(-0.3, 0.3)
        if wilt:
            mat, pitch = WILT[(j + v + 2) % len(WILT)], -0.5
        else:
            mat, pitch = ("pl_crop_deep", "pl_crop_leaf")[j], 0.15
        _palm("ll%d" % j, p.x, p.y, p.z, la, pitch, 0.13 * (0.85 if wilt else 1.0), mat)
    tips = []
    for k in range(nb):
        t = 0.24 + 0.46 * k / max(nb - 1, 1)
        p = stem[0].lerp(stem[-1], t)
        a = az0 + 2 * math.pi * k / nb + rng.uniform(-0.35, 0.35)
        reach = rng.uniform(0.15, 0.2) * (1.15 - 0.4 * t)
        rise = rng.uniform(0.08, 0.13) if not wilt else -0.08
        e = Vector((p.x + math.cos(a) * reach, p.y + math.sin(a) * reach * 0.85, p.z + rise))
        mid = p.lerp(e, 0.5) + Vector((0, 0, 0.03 if not wilt else 0.0))
        _stem("br%d_" % k, [p, mid, e], 0.014, 0.008, "pl_stem" if not wilt else "pl_dried", segs=5)
        tips.append((e, a))
    tips.append((stem[-1], lean_az))
    for k, (e, a) in enumerate(tips):
        # a rosette of lobed leaves at every tip (fewer once the bolls open)
        cnt = 3
        for j in range(cnt):
            la = a + (j - (cnt - 1) / 2) * 1.25 + rng.uniform(-0.25, 0.25)
            size = rng.uniform(0.12, 0.145) * (0.85 if wilt else 1.0)
            if wilt:
                mat = WILT[(k + j + v) % len(WILT)]
                pitch = -0.6
            else:
                mat = ("pl_crop_deep", "pl_crop_leaf", "pl_crop_leaf")[(k + j + v) % 3]
                if ripe and (k + j) % 4 == 3:
                    mat = "pl_dry"   # the oldest leaves yellow as the bolls open
                pitch = (0.1 if ripe else 0.25) + rng.uniform(-0.1, 0.15)
            _palm("lf%d_%d" % (k, j), e.x, e.y, e.z - 0.01, la, pitch, size, mat)
    if st == "mature":  # green bolls swelling and one pale flower
        for k, (e, a) in enumerate(tips[:3]):
            _ball("gb%d" % k, e.x + math.cos(a) * 0.035, e.y + math.sin(a) * 0.03, e.z + 0.04, 0.028, 0.028, 0.032,
                  "pl_crop_pod", segs=8, rings=5)
        e, a = tips[-1]
        _ball("fl", e.x, e.y - 0.01, e.z + 0.06, 0.034, 0.034, 0.02, "pl_crop_cotton", segs=8, rings=4)
    if ripe:
        for k, (e, a) in enumerate(tips):
            _boll("bo%d" % k, e.x + math.cos(a) * 0.04, e.y + math.sin(a) * 0.035 - 0.01, e.z + 0.06,
                  0.036 + 0.005 * (k % 2), 4500 + 10 * v + k)
        if v % 2 == 0:  # one more boll lower on the stem
            p = stem[0].lerp(stem[-1], 0.5)
            _boll("bm", p.x - 0.035, p.y - 0.05, p.z, 0.032, 4590 + v)


BUILDERS = {
    "corn": _corn,
    "wheat": lambda st, v: _grain("wheat", st, v),
    "brewgrain": lambda st, v: _grain("brewgrain", st, v),
    "cotton": _cotton,
}
