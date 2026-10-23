"""Trees (ART spec §9.2): oak, birch, pine (summer, autumn, bare, snow) and
jungle trees. Split out of plant.py in PLAY-010 so tree and ground-plant
art can change independently; plant.build() renders both.
"""
import math
import random

import bmesh
import bpy
import numpy as np

from mathutils import Vector

from .. import stage as S
from .plant_base import CX, CY, _Z, _ball, _cluster, _leaf, _limb, _top

# ------------------------------------------------------------------ trees
TREE_MARGINS = (0.36, 0.36, 0.15, 0.25)
SWAY = {"trunk": 0.02, "canopy": 0.06}

# canopy colour bands per season: (bottom, mid, top) layers, two tones each
# (ART 9.1 muted plant palette; autumn desaturated with dried/khaki tones)
_SUMMER = {
    "oak": (("pl_forest", "pl_dark"), ("pl_grass", "pl_forest"), ("pl_grass", "pl_sage")),
    "birch": (("pl_grass", "pl_dry"), ("pl_dry", "pl_grass"), ("pl_dry", "pl_khaki")),
    "jungle": (("pl_forest", "pl_dark"), ("pl_forest", "pl_grass"), ("pl_grass", "pl_forest")),
}
_AUTUMN = {
    "oak": (("leaf_autumn_red", "pl_dried"), ("leaf_autumn_amber", "leaf_autumn_red"), ("pl_khaki", "leaf_autumn_amber")),
    "birch": (("leaf_autumn_amber", "pl_dry"), ("pl_khaki", "pl_dry"), ("pl_khaki", "leaf_autumn_amber")),
}


def _band(kind, season, layer, k):
    table = (_AUTUMN if season == "autumn" else _SUMMER)[kind]
    return table[layer][k % 2]



# Parts (PLAY-010 2.8): every object is named "T_*" (trunk and lowest
# limbs) or "C_*" (leaves and upper limbs). The full sprite is rendered
# first to fix zmax; the parts reuse it so canvas and origin_px match.
def _tl(name, p0, p1, r0, r1, mat, segs=6):
    return _limb("T_" + name, p0, p1, r0, r1, mat, segs=segs)


def _cl(name, p0, p1, r0, r1, mat, segs=5):
    return _limb("C_" + name, p0, p1, r0, r1, mat, segs=segs)


def _cc(name, x, y, z, r, mat, seed, lumps=0.28, squash=0.85):
    """Smooth-shaded leaf cluster (subdivided, soft normals: no facets)."""
    rng = np.random.RandomState(seed)
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=3, radius=1.0)
    dirs = rng.normal(size=(5, 3))
    dirs /= np.linalg.norm(dirs, axis=1, keepdims=True)
    amp = rng.uniform(-lumps, lumps, size=5)
    for v in bm.verts:
        n = np.array(v.co[:]) / max(v.co.length, 1e-6)
        k = 1.0 + float(np.sum(amp * np.maximum(dirs @ n, 0) ** 2))
        v.co.x = x + n[0] * r * k
        v.co.y = y + n[1] * r * k
        v.co.z = max(0.0, z + n[2] * r * k * squash)
    _top(z + r * squash * 1.15)
    return S._obj("C_" + name, bm, mat, smooth=True)


def _oak_trunk(i, th, lean):
    top = (CX + lean[0], CY + lean[1], th)
    S.cyl("T_flare", CX, CY, 0.16, 0.0, 0.06, "bark", segs=10, r_top=0.125, bevel=0.0)
    _tl("trunk", (CX, CY, 0.05), top, 0.125, 0.085, "bark", segs=9)
    for k in range(4):  # bark ridges
        a = math.radians(70 + 90 * k + 17 * i)
        o = (math.cos(a) * 0.1, math.sin(a) * 0.1)
        _tl("ridge%d" % k, (CX + o[0], CY + o[1], 0.04),
            (CX + lean[0] * 0.85 + o[0] * 0.8, CY + lean[1] * 0.85 + o[1] * 0.8, th * 0.85),
            0.026, 0.014, "bark", segs=4)
    for k in range(2):  # surface roots
        a = math.radians(30 + 150 * k + 40 * i)
        _tl("root%d" % k, (CX + math.cos(a) * 0.1, CY + math.sin(a) * 0.1, 0.05),
            (CX + math.cos(a) * 0.21, CY + math.sin(a) * 0.21, 0.0), 0.03, 0.016, "bark", segs=5)
    return top


def _birch_trunk(i, th, lean):
    top = (CX + lean[0], CY + lean[1], th)
    S.cyl("T_flare", CX, CY, 0.09, 0.0, 0.05, "bark_birch", segs=9, r_top=0.07, bevel=0.0)
    _tl("trunk", (CX, CY, 0.04), top, 0.068, 0.04, "bark_birch", segs=8)
    rng = random.Random(50 + i)
    for k in range(9):  # dark papery marks, facing the camera (south side)
        t = 0.1 + 0.8 * k / 8
        a = math.radians(rng.uniform(205, 335))
        r = 0.068 - 0.028 * t
        x = CX + lean[0] * t + math.cos(a) * r
        y = CY + lean[1] * t + math.sin(a) * r
        _ball("T_mark%d" % k, x, y, th * t, rng.uniform(0.022, 0.04), 0.012, rng.uniform(0.012, 0.02),
              "fur_dark", segs=6, rings=4)
    return top


# per kind: (trunk top z, canopy base offset below trunk top, layers)
# layer = (count, radial offset, z above canopy base, cluster radius)
_DECID = {
    "oak": dict(th=(1.32, 1.4, 1.25), lean=((0.0, 0.0), (0.05, 0.0), (-0.04, 0.02)), zb=(1.07, 1.15, 1.0),
                layers=((5, 0.4, 0.0, 0.21), (4, 0.22, 0.17, 0.2), (2, 0.08, 0.32, 0.17)),
                feather=(6, 0.5, 0.11)),
    "birch": dict(th=(1.38, 1.46, 1.3), lean=((0.05, 0.02), (-0.06, 0.0), (0.0, 0.05)), zb=(1.12, 1.2, 1.05),
                  layers=((5, 0.34, 0.0, 0.18), (4, 0.2, 0.17, 0.17), (2, 0.07, 0.32, 0.15)),
                  feather=(6, 0.42, 0.1)),
}


def _crown(kind, i, season, top, th):
    """Loose leaf clusters on limbs. No core blob: the trunk and limbs
    show through the gaps; satellites feather the outer edge."""
    cfg = _DECID[kind]
    rng = random.Random(100 * (kind == "birch") + 7 * i + 3)
    zb = cfg["zb"][i]
    tx, ty = top[0], top[1]
    bark = "bark" if kind == "oak" else "bark_birch"
    for layer, (n, rad0, dz, cr0) in enumerate(cfg["layers"]):
        for k in range(n):
            a = 2 * math.pi * k / n + rng.uniform(-0.4, 0.4) + layer * 0.95 + i * 0.7
            rad = rad0 * rng.uniform(0.8, 1.15)
            cr = cr0 * rng.uniform(0.85, 1.1)
            z = zb + dz + rng.uniform(-0.03, 0.04)
            x, y = tx + math.cos(a) * rad, ty + math.sin(a) * rad * 0.6
            _cc("leaf%d_%d" % (layer, k), x, y, z, cr, _band(kind, season, layer, k),
                seed=40 * i + 7 * layer + k + 11, lumps=0.3, squash=0.85)
            _cl("arm%d_%d" % (layer, k), (tx, ty, th - 0.12 + 0.1 * layer), (x, y, z - cr * 0.2),
                0.03 if kind == "oak" else 0.016, 0.014 if kind == "oak" else 0.008, bark, segs=4)
    n, rad0, cr0 = cfg["feather"]
    for k in range(n):  # small irregular satellites so neighbours blend
        a = 2 * math.pi * k / n + rng.uniform(-0.35, 0.35) + 0.4 * i
        rad = rad0 * rng.uniform(0.85, 1.1)
        z = zb + rng.uniform(0.0, 0.22)
        cr = cr0 * rng.uniform(0.8, 1.25)
        _cc("feather%d" % k, tx + math.cos(a) * rad, ty + math.sin(a) * rad * 0.5, z, cr,
            _band(kind, season, rng.randrange(3), k), seed=70 * i + k + 500, lumps=0.38, squash=0.8)
    # lowest limbs: two short forks off the upper trunk (trunk part)
    for k in range(2):
        a = math.radians(160 + 180 * k + 25 * i)
        _tl("fork%d" % k, (tx, ty, th - 0.3), (tx + math.cos(a) * 0.16, ty + math.sin(a) * 0.1, th - 0.1),
            0.04 if kind == "oak" else 0.02, 0.018 if kind == "oak" else 0.01, bark, segs=5)


def _bare(kind, i, top, th):
    """Winter branches only; light snow on the upper limbs."""
    rng = random.Random(300 + 11 * i + (kind == "birch") * 5)
    n = 10
    mat = "bark" if kind == "oak" else "bark_birch"
    r0 = 0.05 if kind == "oak" else 0.026
    for k in range(n):
        az = 2 * math.pi * k / n + rng.uniform(-0.3, 0.3) + i
        el = math.radians(rng.uniform(28, 52) if kind == "oak" else rng.uniform(50, 70))
        ln = rng.uniform(0.42, 0.6) * (1.0 if kind == "oak" else 1.2)
        sz = th - rng.uniform(0.0, 0.3)
        low = sz < th - 0.22
        mk = _tl if low else _cl
        pre = "T_" if low else "C_"
        p0 = (top[0], top[1], sz)
        p1 = (top[0] + math.cos(az) * math.cos(el) * ln, top[1] + math.sin(az) * math.cos(el) * ln,
              sz + math.sin(el) * ln)
        mk("limb%d" % k, p0, p1, r0, r0 * 0.35, mat, segs=5)
        high = p1[2] > th + 0.22
        if high:
            _leaf("C_snow%d" % k, p0[0], p0[1], p0[2] + r0 * 0.9, az, el, ln * 0.85, 0.06, "snow", thick=0.035)
        for s in range(2):  # twigs
            t = 0.5 + 0.22 * s
            q = [p0[j] + (p1[j] - p0[j]) * t for j in range(3)]
            az2 = az + (1 if s else -1) * rng.uniform(0.5, 0.9)
            el2 = el + rng.uniform(-0.1, 0.3)
            ln2 = ln * 0.45
            q2 = (q[0] + math.cos(az2) * math.cos(el2) * ln2, q[1] + math.sin(az2) * math.cos(el2) * ln2,
                  q[2] + math.sin(el2) * ln2)
            mk("twig%d_%d" % (k, s), q, q2, r0 * 0.5, r0 * 0.2, mat, segs=4)
            if high and s == 0:
                _leaf("C_snowt%d" % k, q[0], q[1], q[2] + 0.02, az2, el2, ln2 * 0.8, 0.04, "snow", thick=0.03)


def _deciduous(kind, i, season):
    cfg = _DECID[kind]
    th, lean = cfg["th"][i], cfg["lean"][i]
    top = _oak_trunk(i, th, lean) if kind == "oak" else _birch_trunk(i, th, lean)
    if season == "bare":
        _bare(kind, i, top, th)
    else:
        _crown(kind, i, season, top, th)


# pine: bare trunk up to z0 (~35-45% of the height), then a few tiers of
# irregular bough clusters with gaps; max radius ~0.45 tile.
_PINE = {"tiers": (4, 4, 3), "R0": (0.56, 0.54, 0.5), "z0": (0.66, 0.7, 0.62), "step": (0.22, 0.2, 0.24)}


def _pine(i, snow=False):
    tiers, R0, z0, step = (_PINE[k][i] for k in ("tiers", "R0", "z0", "step"))
    S.cyl("T_flare", CX, CY, 0.11, 0.0, 0.06, "bark", segs=10, r_top=0.09, bevel=0.0)
    S.cyl("T_trunk", CX, CY, 0.085, 0.0, z0 + tiers * step * 0.8, "bark", segs=10, r_top=0.045)
    for k in range(3):
        a = math.radians(60 + 120 * k + 30 * i)
        _tl("ridge%d" % k, (CX + math.cos(a) * 0.075, CY + math.sin(a) * 0.075, 0.05),
            (CX + math.cos(a) * 0.06, CY + math.sin(a) * 0.06, z0 - 0.1), 0.02, 0.012, "bark", segs=4)
    rng = random.Random(600 + i)
    for j in range(tiers):
        f = j / float(tiers)
        reach = R0 * (1.0 - 0.72 * f)
        z = z0 + j * step
        n = 5 if j < tiers - 1 else 3
        off = rng.uniform(0, 6.28)
        for k in range(n):
            az = off + 2 * math.pi * k / n + rng.uniform(-0.45, 0.45) + 0.8 * j
            ln = reach * rng.uniform(0.7, 1.05)
            mat = "pl_dark" if False else ("leaf_pine" if (j + k) % 2 == 0 else "leaf_pine_hi")
            _leaf("C_bough%d_%d" % (j, k), CX, CY, z + rng.uniform(-0.02, 0.03), az, math.radians(-14), ln,
                  0.22 + 0.04 * (tiers - j) / tiers, mat, thick=0.11, segs=10)
            if snow:
                _leaf("C_snow%d_%d" % (j, k), CX, CY, z + 0.05, az, math.radians(-8), ln * 0.82, 0.12, "snow",
                      thick=0.07, segs=10)
        _top(z + 0.1)
    ztip = z0 + tiers * step
    S.cyl("C_tip", CX, CY, 0.09, ztip - 0.1, ztip + 0.16, "snow" if snow else "leaf_pine", segs=10, r_top=0.005,
          bevel=0.0)
    _top(ztip + 0.16)


def _jungle(i):
    th = (1.5, 1.4, 1.6)[i]
    lean = ((0.0, 0.0), (0.06, 0.02), (-0.05, 0.03))[i]
    top = (CX + lean[0], CY + lean[1], th)
    S.cyl("T_flare", CX, CY, 0.16, 0.0, 0.06, "bark", segs=10, r_top=0.115, bevel=0.0)
    _tl("trunk", (CX, CY, 0.05), top, 0.115, 0.07, "bark", segs=9)
    for k in range(3):  # small buttress spurs
        a = math.radians(40 + 120 * k + 25 * i)
        _tl("butt%d" % k, (CX + math.cos(a) * 0.08, CY + math.sin(a) * 0.08, 0.1),
            (CX + math.cos(a) * 0.22, CY + math.sin(a) * 0.22, 0.0), 0.035, 0.018, "bark", segs=5)
    rng = random.Random(1200 + i)
    zb = th - 0.15
    # broad, flat crown of loose clusters (gaps between them)
    rings = ((5, 0.38, 0.0, 0.22), (3, 0.18, 0.1, 0.2), (1, 0.0, 0.18, 0.18))
    for layer, (n, rad, dz, cr0) in enumerate(rings):
        for k in range(n):
            a = 2 * math.pi * k / max(n, 1) + rng.uniform(-0.35, 0.35) + layer * 1.1
            x = top[0] + math.cos(a) * rad * rng.uniform(0.85, 1.1)
            y = top[1] + math.sin(a) * rad * 0.5
            z = zb + dz + rng.uniform(0.0, 0.05)
            cr = cr0 * rng.uniform(0.9, 1.1)
            _cc("jl%d_%d" % (layer, k), x, y, z, cr, _band("jungle", "summer", layer, k),
                seed=1210 + 17 * i + 5 * layer + k, lumps=0.3, squash=0.5)
            if layer < 2:
                _cl("arm%d_%d" % (layer, k), (top[0], top[1], th - 0.2), (x, y, z - 0.05), 0.03, 0.014,
                    "bark", segs=4)
    for k in range(4):  # feathered satellites
        a = 2 * math.pi * k / 4 + rng.uniform(-0.3, 0.3) + i
        _cc("jf%d" % k, top[0] + math.cos(a) * 0.44, top[1] + math.sin(a) * 0.28, zb + rng.uniform(0.0, 0.1),
            rng.uniform(0.08, 0.12), _band("jungle", "summer", 0, k + 1), seed=1300 + 11 * i + k,
            lumps=0.38, squash=0.6)
    for k in range(3):  # hanging vine
        a = rng.uniform(0, 2 * math.pi)
        x, y = top[0] + math.cos(a) * 0.35, top[1] + math.sin(a) * 0.2
        _cl("vine%d" % k, (x, y, zb + 0.02), (x + 0.02, y, zb - 0.22), 0.014, 0.008, "pl_forest", segs=4)


# ------------------------------------------------------------ rendering
def _prune(keep_prefix):
    for ob in list(S.stage().collection.all_objects):
        if not ob.name.startswith(keep_prefix):
            data = ob.data
            bpy.data.objects.remove(ob, do_unlink=True)
            if data is not None and data.users == 0:
                bpy.data.meshes.remove(data)


def _render(name, extra, part, zmax):
    kw = dict(zmax=zmax, margins=TREE_MARGINS, extra=extra, outline=False, outline_width=0)
    if part == "trunk":
        kw["shadow_kind"] = "none"
    return S.render_asset("plant", name + ("_" + part if part else ""), **kw)


def _tree_set(build, name, extra):
    """Full sprite, then _trunk and _canopy on the same canvas/origin."""
    S.clear()
    _Z[0] = 0.0
    build()
    zmax = max(_Z[0] + 0.03, 0.12)
    out = [_render(name, extra, None, zmax)]
    for part, pre in (("trunk", "T_"), ("canopy", "C_")):
        S.clear()
        build()
        _prune(pre)
        out.append(_render(name, extra, part, zmax))
    return out


def _tree(kind, i, season, name):
    """Build one full tree (the menu diorama calls this under place())."""
    extra = {"species": "tree", "kind": kind, "variant": i, "season": season, "yields": "wood", "sway": SWAY}
    S.clear()
    _Z[0] = 0.0
    (lambda: _pine(i) if kind == "pine" else _deciduous(kind, i, season))()
    zmax = max(_Z[0] + 0.03, 0.12)
    return _render(name, extra, None, zmax)


def _tree_parts(kind, i, season, name):
    extra = {"species": "tree", "kind": kind, "variant": i, "season": season, "yields": "wood", "sway": SWAY}
    return _tree_set(lambda: _pine(i) if kind == "pine" else _deciduous(kind, i, season), name, extra)


def build():
    """Every tree sprite and its parts (build.py family `trees`; keys plant/tree_*)."""
    out = []
    for kind in ("oak", "birch", "pine"):
        for i in range(3):
            out += _tree_parts(kind, i, "summer", "tree_%s_%02d" % (kind, i))
            if kind != "pine":
                out += _tree_parts(kind, i, "autumn", "tree_%s_%02d_autumn" % (kind, i))
                out += _tree_parts(kind, i, "bare", "tree_%s_%02d_bare" % (kind, i))
    out += build_biome()
    return out


def build_biome():
    """The snowy pines and jungle trees."""
    out = []
    for i in range(3):
        ex = {"species": "tree", "kind": "pine", "variant": i, "season": "snow", "yields": "wood", "sway": SWAY}
        out += _tree_set(lambda i=i: _pine(i, snow=True), "tree_pine_%02d_snow" % i, ex)
        ex = {"species": "tree", "kind": "jungle", "variant": i, "season": "summer", "yields": "wood",
              "sway": SWAY}
        out += _tree_set(lambda i=i: _jungle(i), "tree_jungle_%02d" % i, ex)
    return out
