"""Reusable animal parts (ART-011): mesh primitives plus horns, antlers,
ears, tails, manes, wool, humps, trunk, tusks, glands and claws.

Every part is authored in a local frame and returns the objects it made;
the rig places them (rig.attach) on the body, head or tail bone. Frames
follow the model convention: -Y is forward (towards the nose), +X is the
animal's left, Z is up; paired parts take `side` = +1 (left) / -1
(right). Units are body lengths (the rig scales the finished model to
the species size).

Parts are deterministic: anything irregular (antler tines, tufts, wool
bumps) draws from a zlib.crc32 seed, never from Python's salted hash.
"""
import math
import random
import zlib

import bmesh
import bpy
from mathutils import Matrix, Vector

from ... import stage as S

TAU = 2 * math.pi


# ----------------------------------------------------------- primitives
def _mesh(name, bm, material, smooth=True):
    me = bpy.data.meshes.new(name)
    bm.to_mesh(me)
    bm.free()
    me.materials.append(S.mat(material))
    for p in me.polygons:
        p.use_smooth = smooth
    ob = bpy.data.objects.new(name, me)
    S.stage().collection.objects.link(ob)
    return ob


def rng(*key):
    """Deterministic RNG from any key parts (crc32, not hash())."""
    return random.Random(zlib.crc32(("/".join(str(k) for k in key)).encode()))


def rot(x=0.0, y=0.0, z=0.0):
    """4x4 rotation, degrees, applied X then Y then Z."""
    return (Matrix.Rotation(math.radians(z), 4, "Z") @ Matrix.Rotation(math.radians(y), 4, "Y")
            @ Matrix.Rotation(math.radians(x), 4, "X"))


def ell(name, c, r, material, m=None, segs=16, rings=10, smooth=True):
    """Ellipsoid at c with radii r=(rx, ry, rz), optionally rotated by m."""
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=rings, radius=1.0)
    M = Matrix.Translation(Vector(c)) @ (m if m is not None else Matrix.Identity(4)) @ Matrix.Diagonal(
        (r[0], r[1], r[2], 1.0))
    bmesh.ops.transform(bm, matrix=M, verts=bm.verts)
    return _mesh(name, bm, material, smooth)


def _rad(r):
    return (r, r) if isinstance(r, (int, float)) else (r[0], r[1])


def tube(name, pts, radii, material, segs=10, caps=(1.0, 1.0), side=(1, 0, 0), smooth=True):
    """Swept tube through pts with per-point radii (float, or (rs, ru) for an
    elliptic section: rs along the side vector, ru along tangent x side).
    Frames are parallel-transported from `side`. caps: dome depth factors
    at the start/end (0 = flat); a ~0 radius ends in a point."""
    P = [Vector(p) for p in pts]
    R = [_rad(r) for r in radii]
    n = len(P)
    T = []
    for i in range(n):
        d = P[min(i + 1, n - 1)] - P[max(i - 1, 0)]
        T.append(d.normalized() if d.length > 1e-9 else (T[-1] if T else Vector((0, -1, 0))))
    ref = Vector(side).normalized()
    if abs(ref.dot(T[0])) > 0.97:
        ref = Vector((0, 0, 1)) if abs(T[0].z) < 0.9 else Vector((1, 0, 0))
    s = (ref - T[0] * ref.dot(T[0])).normalized()
    sides = [s]
    for i in range(1, n):
        q = T[i - 1].rotation_difference(T[i])
        s = q @ sides[-1]
        s = (s - T[i] * s.dot(T[i])).normalized()
        sides.append(s)
    bm = bmesh.new()
    rings = []

    def ring(c, t, sv, rs, ru):
        if max(rs, ru) < 1e-5:
            return [bm.verts.new(c)]
        uv = t.cross(sv)
        return [bm.verts.new(c + sv * (rs * math.cos(a)) + uv * (ru * math.sin(a)))
                for a in (TAU * k / segs for k in range(segs))]

    # start dome (built outward, then reversed so rings run start -> end)
    start = []
    if caps[0] and min(R[0]) > 1e-5:
        depth = caps[0] * (R[0][0] + R[0][1]) / 2
        for phi in (90, 60, 30):
            k = math.cos(math.radians(phi))
            o = math.sin(math.radians(phi)) * depth
            if phi == 90:
                start.append([bm.verts.new(P[0] - T[0] * depth)])
            else:
                start.append(ring(P[0] - T[0] * o, T[0], sides[0], R[0][0] * k, R[0][1] * k))
    rings += start
    for i in range(n):
        rings.append(ring(P[i], T[i], sides[i], *R[i]))
    if caps[1] and min(R[-1]) > 1e-5:
        depth = caps[1] * (R[-1][0] + R[-1][1]) / 2
        for phi in (30, 60, 90):
            k = math.cos(math.radians(phi))
            o = math.sin(math.radians(phi)) * depth
            if phi == 90:
                rings.append([bm.verts.new(P[-1] + T[-1] * depth)])
            else:
                rings.append(ring(P[-1] + T[-1] * o, T[-1], sides[-1], R[-1][0] * k, R[-1][1] * k))
    for a, b in zip(rings, rings[1:]):
        if len(a) == 1 and len(b) == 1:
            continue
        if len(a) == 1:
            for k in range(segs):
                bm.faces.new((a[0], b[(k + 1) % segs], b[k]))
        elif len(b) == 1:
            for k in range(segs):
                bm.faces.new((a[k], a[(k + 1) % segs], b[0]))
        else:
            for k in range(segs):
                bm.faces.new((a[k], a[(k + 1) % segs], b[(k + 1) % segs], b[k]))
    if len(rings[0]) > 1:
        bm.faces.new(list(reversed(rings[0])))
    if len(rings[-1]) > 1:
        bm.faces.new(rings[-1])
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return _mesh(name, bm, material, smooth)


def bezier(p0, p1, p2, n=6):
    p0, p1, p2 = Vector(p0), Vector(p1), Vector(p2)
    return [p0 * (1 - t) ** 2 + p1 * 2 * t * (1 - t) + p2 * t * t for t in (i / (n - 1) for i in range(n))]


def lerp(a, b, t):
    return a + (b - a) * t


def taper(r0, r1, n, power=1.0):
    return [lerp(r0, r1, (i / (n - 1)) ** power) for i in range(n)]


def turtle(start, d, length, n, axis, bend, twist=None, twist_deg=0.0, drift=None):
    """Polyline of n segments: start at `start` heading `d`, turning `bend`
    degrees in total about `axis` (optionally rotating the axis about
    `twist` as it goes), moving `drift` per step on top (helix)."""
    p = Vector(start)
    d = Vector(d).normalized()
    ax = Vector(axis).normalized()
    pts = [p.copy()]
    step = length / n
    for _ in range(n):
        d = (Matrix.Rotation(math.radians(bend / n), 3, ax) @ d).normalized()
        if twist is not None:
            ax = (Matrix.Rotation(math.radians(twist_deg / n), 3, Vector(twist)) @ ax).normalized()
        p = p + d * step + (Vector(drift) if drift else Vector())
        pts.append(p.copy())
    return pts


def cone(name, base, tip, r, material, segs=8, cap=0.6):
    return tube(name, [base, tip], [r, 0.0], material, segs=segs, caps=(cap, 0))


# -------------------------------------------------------------- horns
def horn(name, base, kind, size, material, side=1, ridged=False, segs=9):
    """One horn in head-local space, rooted at `base` on the skull's `side`
    (+1 left/+X, -1 right). size scales length. Kinds:
    nub, curved (out then up), short_curved (bison: out, up, in), swept
    (back-curving, thin), sweep_big (ibex: long arcs back), ram (curl down
    and forward by the cheek), out (yak: out and slightly up/forward),
    spike (straight cone up), cow (out to the side, tips up)."""
    sx = side
    b = Vector(base)
    if kind == "nub":
        return [cone(name, b, b + Vector((0.2 * sx, 0.05, 1)).normalized() * size * 0.06, size * 0.022, material)]
    if kind == "spike":
        return [cone(name, b, b + Vector((0.15 * sx, 0.25, 1)).normalized() * size * 0.12, size * 0.02, material)]
    if kind == "cow":
        pts = turtle(b, (sx, 0.05, 0.25), size * 0.09, 6, (0, 1, 0), -50 * sx)
        r = taper(size * 0.016, size * 0.003, len(pts), 1.2)
    elif kind == "curved":
        pts = turtle(b, (sx, 0, 0.35), size * 0.1, 7, (0, 1, 0), -70 * sx)
        r = taper(size * 0.017, size * 0.002, len(pts), 1.1)
    elif kind == "short_curved":
        pts = turtle(b, (sx, -0.05, 0.1), size * 0.1, 7, (0, 1, 0), -120 * sx)
        r = taper(size * 0.02, size * 0.003, len(pts), 1.0)
    elif kind == "out":
        pts = turtle(b, (sx, -0.25, 0.25), size * 0.12, 7, (0, 1, 0), -70 * sx)
        r = taper(size * 0.02, size * 0.003, len(pts), 1.1)
    elif kind == "swept":
        pts = turtle(b, (0.18 * sx, 0.1, 1), size * 0.16, 8, (1, 0, 0), -70)
        r = taper(size * 0.013, size * 0.002, len(pts), 1.0)
    elif kind == "sweep_big":
        pts = turtle(b, (0.22 * sx, -0.1, 1), size * 0.34, 12, (1, 0.1 * sx, 0), -170, drift=(0.003 * sx, 0, 0))
        r = taper(size * 0.026, size * 0.004, len(pts), 1.0)
    elif kind == "ram":
        # up and back, then down behind the eye and forward beside the cheek
        pts = turtle(b, (0.35 * sx, 0.55, 0.65), size * 0.28, 14, (1, 0, 0), -300,
                     drift=(0.0035 * sx, 0, 0))
        r = taper(size * 0.03, size * 0.006, len(pts), 0.9)
    else:
        raise ValueError("horn kind %r" % kind)
    if ridged:
        r = [ri * (1.18 if i % 2 else 0.95) for i, ri in enumerate(r)]
    return [tube(name, pts, r, material, segs=segs, caps=(0.5, 0))]


def nose_horns(base, size, material):
    """Rhino: two horns on the snout, the front one larger (head-local)."""
    b = Vector(base)
    return [cone("horn", b, b + Vector((0, -0.35, 1)).normalized() * 0.15 * size, 0.035 * size, material, segs=10),
            cone("horn", b + Vector((0, 0.07, 0.01)) * size,
                 b + Vector((0, 0.07, 0.01)) * size + Vector((0, -0.1, 1)).normalized() * 0.07 * size,
                 0.026 * size, material, segs=10)]


def antlers(key, base_y, base_z, size, material, kind="deer", tip_material=None, spread=0.05):
    """Procedural, deterministic antlers (head-local), mirrored left/right.
    kind: deer (forward-curving beam, 3-4 upward tines), elk (tall, wide,
    swept back, 6 tines), caribou (sweeping back then forward, irregular,
    a palmate brow tine), crystal (angular faceted, branching)."""
    out = []
    R = rng(key, kind)
    if kind == "crystal":
        for sx in (-1, 1):
            out += _crystal(Vector((sx * spread, base_y, base_z)), sx, size, material, tip_material, R)
        return out
    spec = {
        "deer": dict(beam=0.3, n=10, up=(0.6, 0.8, 1.0), bend=95, tines=[(0.35, 0.09), (0.6, 0.1), (0.82, 0.07)],
                     r0=0.014, brow=0.06),
        "elk": dict(beam=0.5, n=12, up=(0.85, 1.5, 0.9), bend=70, tines=[(0.25, 0.14), (0.42, 0.14), (0.6, 0.13),
                                                                          (0.76, 0.11), (0.9, 0.08)],
                    r0=0.017, brow=0.12),
        "caribou": dict(beam=0.52, n=12, up=(0.7, 1.6, 0.8), bend=115, tines=[(0.45, 0.1), (0.62, 0.12),
                                                                             (0.76, 0.1), (0.88, 0.09)],
                        r0=0.015, brow=0.11, palm=True),
    }[kind]
    for sx in (-1, 1):
        b = Vector((sx * spread, base_y, base_z))
        up = Vector((spec["up"][0] * sx, spec["up"][1], spec["up"][2]))
        # the beam rises, sweeps back, and curls its tip forward
        beam = turtle(b, up, spec["beam"] * size, spec["n"], (1, 0, 0), spec["bend"])
        # rotation about +X tilts a backward heading forward over the top
        r = taper(spec["r0"] * size, 0.004 * size, len(beam), 0.8)
        out.append(tube("antler", beam, r, material, segs=7, caps=(0.4, 0)))
        # brow tine: forward from near the base
        bp = beam[1]
        bdir = Vector((0.25 * sx, -1.0, 0.55)).normalized()
        if spec.get("palm"):
            pts = turtle(bp, bdir, spec["brow"] * size, 4, (1, 0, 0), 30)
            out.append(tube("antler", pts, [(0.012 * size, 0.004 * size), (0.026 * size, 0.004 * size),
                                            (0.024 * size, 0.004 * size), (0.016 * size, 0.003 * size),
                                            (0.004 * size, 0.002 * size)], material, segs=8, side=(0, 0, 1)))
        else:
            out.append(tube("antler", turtle(bp, bdir, spec["brow"] * size, 3, (1, 0, 0), -25),
                            taper(0.009 * size, 0.003 * size, 4), material, segs=6, caps=(0.4, 0)))
        for f, ln in spec["tines"]:
            i = min(len(beam) - 2, max(1, int(round(f * (len(beam) - 1)))))
            p = beam[i]
            fwd = (beam[i + 1] - beam[i - 1]).normalized()
            jitter = R.uniform(-0.15, 0.15)
            d = (Vector((0.2 * sx + jitter * 0.3, 0.1 + jitter, 1.0)).normalized() * 0.8 + fwd * 0.2).normalized()
            ln *= size * R.uniform(0.85, 1.12)
            pts = turtle(p, d, ln, 3, (1, 0, 0), -12)
            out.append(tube("antler", pts, taper(0.0095 * size, 0.003 * size, len(pts)), material, segs=6,
                            caps=(0.4, 0)))
    return out


def _crystal(b, sx, size, material, tip_material, R):
    """Angular crystal antler: straight faceted shards that branch."""
    out = []

    def shard(p, d, ln, r, depth):
        tip = p + d * ln
        out.append(tube("antler", [p, p + d * ln * 0.55, tip], [r, r * 0.8, 0.0], material, segs=5,
                        caps=(0.3, 0), smooth=False))
        if tip_material and depth >= 1:
            out.append(ell("antler_glow", p + d * ln * 0.78, (r * 0.55, r * 0.55, r * 0.55), tip_material,
                           segs=6, rings=4, smooth=False))
        if depth <= 0:
            return
        m = p + d * ln * 0.55
        for k in range(2):
            a = R.uniform(25, 45) * (1 if k == 0 else -1)
            nd = (Matrix.Rotation(math.radians(a), 3, Vector((1, 0, 0))) @ d)
            nd = (nd + Vector((0.25 * sx * R.uniform(0.3, 1.0), 0, 0))).normalized()
            shard(m if k == 0 else p + d * ln * 0.8, nd, ln * R.uniform(0.5, 0.65), r * 0.7, depth - 1)

    shard(b, Vector((0.45 * sx, 0.35, 1.0)).normalized(), 0.34 * size, 0.022 * size, 2)
    return out


# --------------------------------------------------------------- ears
def ear(base, kind, size, material, side, inner=None, tip=None, angle=0.0):
    """One ear at head-local `base` on `side`. Kinds: pointed, round, long,
    floppy, tufted, fan, tiny, side, huge (fennec). angle tilts it back."""
    sx = side
    b = Vector(base)
    out = []
    rb = Matrix.Rotation(math.radians(angle), 3, Vector((1, 0, 0)))
    if kind in ("pointed", "tufted", "long", "huge"):
        ln, w, lean = {"pointed": (0.075, 0.03, 0.35), "tufted": (0.08, 0.032, 0.3),
                       "long": (0.2, 0.027, 0.34), "huge": (0.17, 0.065, 0.55)}[kind]
        ln, w = ln * size, w * size
        d = rb @ Vector((lean * sx, 0.2, 1.0)).normalized()
        pts = [b, b + d * ln * 0.4, b + d * ln]
        out.append(tube("ear", pts, [(w, w * 0.35), (w * 0.9, w * 0.3), (0.0, 0.0)], material, segs=8,
                        caps=(0.5, 0), side=(1 * sx, -0.6, 0)))
        if inner:
            fwd = Vector((0, -1, 0)) * w * 0.22
            out.append(tube("ear_in", [p + fwd for p in pts[:2]] + [pts[2] * 0.85 + b * 0.15 + fwd],
                            [(w * 0.62, w * 0.18), (w * 0.55, w * 0.16), (0.0, 0.0)], inner, segs=8,
                            caps=(0.5, 0), side=(1 * sx, -0.6, 0)))
        if kind == "tufted":
            out.append(tube("ear_tuft", [b + d * ln * 0.95, b + d * ln * 1.45],
                            [0.008 * size, 0.0], tip or "fur_black", segs=6))
        elif tip:
            out.append(tube("ear_tip", [b + d * ln * 0.62, b + d * ln * 0.82, b + d * ln * 1.001],
                            [(w * 0.5, w * 0.36), (w * 0.3, w * 0.3), (0.0, 0.0)], tip, segs=8,
                            side=(1 * sx, -0.6, 0)))
        return out
    if kind in ("round", "tiny"):
        r = (0.03 if kind == "round" else 0.016) * size
        c = b + Vector((0.2 * sx, 0.1, 0.9)).normalized() * r * 0.8
        out.append(ell("ear", c, (r, r * 0.42, r * 1.0), material, m=rot(x=-15 + angle, z=-25 * sx), segs=12,
                       rings=8))
        if inner:
            out.append(ell("ear_in", c + Vector((0, -r * 0.28, -r * 0.08)), (r * 0.62, r * 0.25, r * 0.62), inner,
                           m=rot(x=-15 + angle, z=-25 * sx), segs=10, rings=6))
        return out
    if kind == "floppy":
        ln, w = 0.085 * size, 0.032 * size
        pts = [b, b + Vector((0.035 * sx, 0.0, 0.005)) * size, b + Vector((0.05 * sx, -0.012, -0.05)) * size,
               b + Vector((0.052 * sx, -0.018, -0.08)) * size]
        out.append(tube("ear", pts, [(w * 0.6, w * 0.3), (w, w * 0.35), (w * 0.95, w * 0.32), (w * 0.5, w * 0.25)],
                        material, segs=8, side=(0, -1, 0)))
        return out
    if kind == "side":
        ln, w = 0.075 * size, 0.026 * size
        d = Vector((sx, 0.15, 0.05)).normalized()
        pts = [b, b + d * ln * 0.5, b + d * ln]
        out.append(tube("ear", pts, [(w * 0.6, w * 0.3), (w, w * 0.35), (0.0, 0.0)], material, segs=8,
                        side=(0, -1, 0), caps=(0.5, 0)))
        if inner:
            out.append(tube("ear_in", [p + Vector((0, -w * 0.18, 0)) for p in pts[:2]] + [pts[2] * 0.85 + b * 0.15],
                            [(w * 0.4, w * 0.2), (w * 0.62, w * 0.2), (0.0, 0.0)], inner, segs=8, side=(0, -1, 0)))
        return out
    if kind == "fan":
        # elephant: a big flat ear lying back along the side of the head
        r = 0.17 * size
        c = b + Vector((0.09 * sx, 0.05, -0.08)) * size
        out.append(ell("ear", c, (r * 0.12, r * 0.85, r), material, m=rot(z=-52 * sx, x=-6, y=8 * sx), segs=20,
                       rings=12))
        if inner:
            out.append(ell("ear_in", c + Vector((-0.012 * sx, -0.012, 0)) * size, (r * 0.08, r * 0.7, r * 0.82), inner,
                           m=rot(z=-52 * sx, x=-6, y=8 * sx), segs=18, rings=10))
        return out
    raise ValueError("ear kind %r" % kind)


# -------------------------------------------------------------- tails
def tail(kind, base, size, material, tip=None, rings=None, raise_deg=0.0, sway=0.0, key=""):
    """Tail in body-frame space from `base` (rear of the torso).
    raise_deg lifts it (degrees), sway swings it sideways. Kinds:
    bushy, thin, tufted, paddle, ringed, curled (over the back),
    squirrel (huge plume curling up over the back), bob, flowing (horse),
    short (deer flag), arc (monkey: long curled arc), stub, none."""
    b = Vector(base)
    up = raise_deg
    out = []
    ax_side = Matrix.Rotation(math.radians(sway), 3, Vector((0, 0, 1)))

    def path(d, length, n, bend, axis=(1, 0, 0)):
        d = ax_side @ Vector(d)
        pts = turtle(b, d, length, n, ax_side @ Vector(axis), bend)
        return pts

    def droop(deg):
        a = math.radians(deg + up)
        return Vector((0, math.cos(a), math.sin(a)))

    if kind in ("none", None):
        return out
    if kind == "stub":
        out.append(ell("tail", b + droop(10) * 0.02 * size, (0.022 * size, 0.03 * size, 0.022 * size), material))
        return out
    if kind == "short":
        pts = path(droop(35), 0.07 * size, 4, 0)
        out.append(tube("tail", pts, [(0.028 * size, 0.018 * size), (0.03 * size, 0.02 * size),
                                      (0.024 * size, 0.016 * size), (0.012 * size, 0.01 * size), (0.0, 0.0)],
                        tip or material, segs=8))
        return out
    if kind == "bob":
        pts = path(droop(10), 0.06 * size, 3, 0)
        out.append(tube("tail", pts, taper(0.022 * size, 0.018 * size, len(pts)), material, segs=8))
        if tip:
            out.append(ell("tail_tip", pts[-1], (0.018 * size, 0.02 * size, 0.018 * size), tip, segs=10, rings=6))
        return out
    if kind == "thin":
        # long and hairless, trailing back in a low curve
        pts = path(droop(-28), 0.5 * size, 10, 40)
        if not tip:
            out.append(tube("tail", pts, taper(0.013 * size, 0.004 * size, len(pts)), material, segs=7))
            return out
        # glowing tip: the tail thickens into a bead at its end
        r = taper(0.015 * size, 0.008 * size, len(pts))
        k = len(pts) - 3
        out.append(tube("tail", pts[:k + 1], r[:k + 1], material, segs=8, caps=(0.6, 0)))
        bead = pts[k:] + [pts[-1] + (pts[-1] - pts[-2]) * 0.6]
        out.append(tube("tail_tip", bead, [r[k] * 1.05, 0.013 * size, 0.019 * size, 0.0], tip, segs=10,
                        caps=(0.3, 0)))
        return out
    if kind == "curly":
        # pig: a short tight corkscrew
        pts = path(droop(20), 0.09 * size, 10, 330)
        pts = [p + Vector((0.004 * size * math.sin(i * 0.9), 0, 0)) for i, p in enumerate(pts)]
        out.append(tube("tail", pts, taper(0.009 * size, 0.005 * size, len(pts)), material, segs=7))
        return out
    if kind == "taper":
        # thick at the root, straight, tapering (labrador)
        pts = path(droop(-30), 0.3 * size, 6, -12)
        out.append(tube("tail", pts, taper(0.03 * size, 0.004 * size, len(pts), 0.9), material, segs=10))
        return out
    if kind == "cat":
        pts = path(droop(-60), 0.42 * size, 10, 150)
        r = taper(0.017 * size, 0.011 * size, len(pts))
        out.append(tube("tail", pts, r, material, segs=8, caps=(0.6, 1.0)))
        if tip:
            k = int(len(pts) * 0.72)
            out.append(tube("tail_tip", pts[k:], taper(r[k] * 1.08, r[-1] * 1.08, len(pts) - k), tip, segs=8,
                            caps=(0.6, 1.0)))
        return out
    if kind == "tufted":
        pts = path(droop(-70), 0.3 * size, 6, -10)
        out.append(tube("tail", pts, taper(0.012 * size, 0.008 * size, len(pts)), material, segs=7))
        out.append(tube("tail_tip", [pts[-2], pts[-1], pts[-1] + (pts[-1] - pts[-2]) * 1.3],
                        [0.016 * size, 0.026 * size, 0.0], tip or "fur_black", segs=8))
        return out
    if kind == "flowing":
        pts = path(droop(-35), 0.4 * size, 8, -40)
        n = len(pts)
        r = [(0.032 * size, 0.045 * size)] + [(lerp(0.04, 0.026, i / (n - 2)) * size,
                                               lerp(0.052, 0.03, i / (n - 2)) * size) for i in range(n - 1)]
        out.append(tube("tail", pts + [pts[-1] + (pts[-1] - pts[-2]) * 0.6], r + [(0.0, 0.0)], material, segs=9))
        return out
    if kind == "bushy":
        pts = path(droop(-40), 0.36 * size, 8, -30)
        r = [lerp(0.022, 0.05, math.sin(math.pi * min(1, i / 6.0) * 0.5)) * size * (1 - (i / 9.0) ** 3)
             for i in range(len(pts))]
        r[-1] = 0.0
        out.append(tube("tail", pts, r, material, segs=10))
        if tip:
            k = len(pts) - 3
            out.append(tube("tail_tip", pts[k:], [r[k] * 1.05, r[k + 1] * 1.08, 0.0], tip, segs=10, caps=(0.5, 0)))
        return out
    if kind == "fox":
        pts = path(droop(-22), 0.44 * size, 9, -18)
        r = [lerp(0.024, 0.07, math.sin(math.pi * min(1, i / 6.5) * 0.5)) * size * (1 - (i / 9.6) ** 4)
             for i in range(len(pts))]
        r[-1] = 0.0
        out.append(tube("tail", pts, r, material, segs=10))
        if tip:
            k = len(pts) - 3
            out.append(tube("tail_tip", pts[k:], [r[k] * 1.05, r[k + 1] * 1.08, 0.0], tip, segs=10, caps=(0.5, 0)))
        return out
    if kind == "curled":
        # husky: rises and curls over the back towards the head
        pts = path(droop(75), 0.3 * size, 9, 200)
        r = [0.03 * size * (1 - (i / 10.0) ** 2) + 0.012 * size for i in range(len(pts))]
        r[-1] = 0.0
        out.append(tube("tail", pts, r, material, segs=10))
        if tip:
            k = len(pts) - 3
            out.append(tube("tail_tip", pts[k:], [r[k] * 1.05, r[k + 1] * 1.08, 0.0], tip, segs=10))
        return out
    if kind == "squirrel":
        # a plume as big as the body, rising from the rump and curling over the back
        # rises steeply behind the rump, then the plume tip curls back
        pts = path(droop(84), 0.36 * size, 7, 12)
        pts += turtle(pts[-1], pts[-1] - pts[-2], 0.4 * size, 7, ax_side @ Vector((1, 0, 0)), -150)[1:]
        n = len(pts)
        r = [(0.022 + 0.072 * math.sin(math.pi * min(1.0, (i + 2) / (n + 1)) * 0.95)) * size for i in range(n)]
        r[-1] = 0.03 * size
        out.append(tube("tail", pts, r, material, segs=14))
        return out
    if kind == "arc":
        # monkey: long, thin, raised in a question-mark arc
        pts = path(droop(30), 0.75 * size, 14, 260)
        out.append(tube("tail", pts, taper(0.014 * size, 0.008 * size, len(pts)), material, segs=7))
        return out
    if kind == "paddle":
        pts = path(droop(-12), 0.08 * size, 3, 0)
        out.append(tube("tail", pts, taper(0.03 * size, 0.022 * size, len(pts)), material, segs=8))
        d = (pts[-1] - pts[-2]).normalized()
        c = pts[-1] + d * 0.12 * size
        out.append(ell("tail_paddle", c, (0.075 * size, 0.14 * size, 0.018 * size), tip or material,
                       m=Matrix.Rotation(math.atan2(d.z, d.y), 4, "X"), segs=16, rings=8))
        return out
    if kind == "ringed":
        pts = path(droop(-30), 0.36 * size, 12, -35)
        n = len(pts)
        r = [lerp(0.03, 0.042, math.sin(math.pi * min(1, i / 8.0) * 0.5)) * size * (1 - (i / (n - 0.5)) ** 4)
             for i in range(n)]
        r[-1] = 0.0
        mats = rings or (material, "fur_black")
        for k in range(0, n - 1, 2):
            seg = pts[k:k + 3]
            rr = r[k:k + 3]
            if len(seg) < 2:
                break
            out.append(tube("tail", seg, [x * (1.0 if (k // 2) % 2 else 1.03) for x in rr], mats[(k // 2) % 2],
                            segs=10, caps=(0.3 if k else 0.6, 0.3 if k + 3 < n else 0)))
        return out
    raise ValueError("tail kind %r" % kind)


# --------------------------------------------- coats, manes and fur shapes
def tufts(name, pts, normals, length, r, material, key, jitter=0.3, segs=6):
    """Short cones along pts pointing along normals: shaggy edges."""
    R = rng(key, name, len(pts))
    out = []
    for p, n in zip(pts, normals):
        n = Vector(n).normalized()
        ln = length * R.uniform(1 - jitter, 1 + jitter)
        out.append(cone(name, Vector(p) - n * r * 0.4, Vector(p) + n * ln, r * R.uniform(0.85, 1.15), material,
                        segs=segs))
    return out


def skirt(name, c, rx, ry, z_top, z_bot, material, key, n=22, flare=1.12, jag=0.04, top_dome=True, wave=0,
          top_k=1.0):
    """Hanging coat around a body (body frame): an elliptic curtain from
    z_top down to a jagged bottom edge at about z_bot, closed underneath."""
    R = rng(key, name)
    bm = bmesh.new()
    cx, cy = c
    rows = 4
    ringsv = []
    for j in range(rows + 1):
        t = j / rows
        z = lerp(z_top, z_bot, t)
        k = lerp(1.0, flare, t ** 1.2)
        ky = lerp(top_k, flare, t ** 0.8) if top_k < 1 else k   # top_k narrows front-to-back only
        ring = []
        for i in range(n):
            a = TAU * i / n
            dz = 0.0
            kk = k * (1 + (0.05 * t * R.uniform(-1, 1) if j and not wave else 0))
            if j == rows:
                if wave:   # soft, continuous waves (silky coats)
                    dz = jag * 0.5 * (1 + math.sin(wave * a))
                else:
                    dz = jag * (1.0 if i % 2 == 0 else -0.35) * R.uniform(0.5, 1.3)
            ring.append(bm.verts.new((cx + rx * kk * math.cos(a), cy + ry * kk * (ky / k) * math.sin(a), z + dz)))
        ringsv.append(ring)
    if top_dome:
        top = bm.verts.new((cx, cy, z_top + 0.4 * min(rx, ry)))
        for i in range(n):
            bm.faces.new((top, ringsv[0][i], ringsv[0][(i + 1) % n]))
    for a, b in zip(ringsv, ringsv[1:]):
        for i in range(n):
            bm.faces.new((a[i], b[i], b[(i + 1) % n], a[(i + 1) % n]))
    bot = bm.verts.new((cx, cy, z_bot + jag))
    for i in range(n):
        bm.faces.new((ringsv[-1][(i + 1) % n], ringsv[-1][i], bot))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return [_mesh(name, bm, material, smooth=True)]


def puff(name, c, r, material, key, n=26, bump=0.32, under=True):
    """Wool cloud: an ellipsoid core with bumps spread over its surface by a
    golden-angle spiral (deterministic): a bumpy outline."""
    R = rng(key, name)
    out = [ell(name, c, r, material, segs=20, rings=12)]
    c = Vector(c)
    ga = math.pi * (3 - math.sqrt(5))
    for i in range(n):
        zf = 1 - 2 * (i + 0.5) / n
        if not under and zf < -0.55:
            continue
        rr = math.sqrt(max(0.0, 1 - zf * zf))
        a = ga * i
        dirv = Vector((math.cos(a) * rr, math.sin(a) * rr, zf))
        p = c + Vector((dirv.x * r[0], dirv.y * r[1], dirv.z * r[2])) * 0.92
        s = min(r) * bump * R.uniform(0.8, 1.2)
        out.append(ell(name, p, (s, s, s), material, segs=10, rings=7))
    return out


def fin(name, pts, heights, width, material, side=(1, 0, 0)):
    """Mane-like fin: a thin, tall tube (thin along `side`, tall along up)."""
    return tube(name, pts, [(width, h) for h in heights], material, segs=8, side=side)


def gland(c, r, material, socket="fur_dark"):
    """Glowing gland nodule in a dark socket (emissive material)."""
    c = Vector(c)
    return [ell("gland_socket", c - Vector((0, 0, r * 0.35)), (r * 1.3, r * 1.3, r * 0.9), socket, segs=10, rings=6),
            ell("gland", c + Vector((0, 0, r * 0.15)), (r, r, r * 0.9), material, segs=10, rings=7)]


def tusk(base, side, size, material, kind="elephant"):
    """Tusk from a head-local base: elephant (long, forward and up) or boar
    (short, curving up and out)."""
    b = Vector(base)
    sx = side
    if kind == "elephant":
        pts = turtle(b, (0.18 * sx, -0.55, -0.85), 0.28 * size, 8, (1, 0, 0), -95)
        return [tube("tusk", pts, taper(0.022 * size, 0.004 * size, len(pts)), material, segs=9, caps=(0.5, 0))]
    pts = turtle(b, (0.5 * sx, -0.3, 0.6), 0.06 * size, 5, (0, 1, 0), 80 * sx)
    return [tube("tusk", pts, taper(0.009 * size, 0.0015 * size, len(pts)), material, segs=7, caps=(0.5, 0))]


def trunk(base, size, material, curl=0.0, lift=0.0):
    """Elephant trunk from the upper muzzle (head-local): hangs down, the
    tip curving forward. curl/lift (degrees) raise it for attack poses."""
    b = Vector(base)
    pts = turtle(b, Vector((0, -0.2, -1.0)).normalized(), 0.3 * size, 7, (1, 0, 0), 12)
    pts += turtle(pts[-1], (pts[-1] - pts[-2]), 0.14 * size, 6, (1, 0, 0), -(55 + curl))[1:]
    if lift:
        m = Matrix.Rotation(math.radians(-lift), 3, Vector((1, 0, 0)))
        pts = [b + m @ (p - b) for p in pts]
    n = len(pts)
    r = [(lerp(0.06, 0.022, (i / (n - 1)) ** 0.8) * size) for i in range(n)]
    out = [tube("trunk", pts, r, material, segs=12, caps=(0.5, 0.35))]
    return out


def claws(foot, fwd, size, material, n=3, length=0.05, spread=0.018, curve=60):
    """Curved claws from a paw tip, pointing along fwd and curling down."""
    out = []
    f = Vector(fwd).normalized()
    side = f.cross(Vector((0, 0, 1))).normalized()
    for k in range(n):
        o = (k - (n - 1) / 2) * spread * size
        b = Vector(foot) + side * o
        pts = turtle(b, f + Vector((0, 0, 0.2)), length * size, 4, side, curve)
        out.append(tube("claw", pts, taper(0.007 * size, 0.001 * size, len(pts)), material, segs=6))
    return out
