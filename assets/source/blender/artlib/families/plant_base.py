"""Shared plant primitives (PLAY-010 split): the geometry helpers both
the ground-plant family (plant.py) and the trees (trees.py) build with.
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



def _blade(name, x, y, h, lean_az, lean, mat, r=0.03):
    mid = (x + math.cos(lean_az) * lean * 0.35, y + math.sin(lean_az) * lean * 0.35, h * 0.55)
    tip = (x + math.cos(lean_az) * lean, y + math.sin(lean_az) * lean, h)
    _limb(name + "a", (x, y, 0.0), mid, r, r * 0.7, mat, segs=4)
    _limb(name + "b", mid, tip, r * 0.7, 0.004, mat, segs=4)
