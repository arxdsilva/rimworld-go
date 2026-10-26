"""Compositional pawns (ART spec §10): a limbless figure, one standing
sprite per layer and facing, no animation frames.

The figure is a round head on a rounded body capsule, with no visible arms,
legs or hands, and dot eyes (two facing south, one east, none north). The
renderer moves the whole figure (bob, sway, lean, squash, rotation), draws
held items at the body's `hold` point (no hands, as the reference style
draws none), and mirrors east for west (ART §27.1-27.2).

Layers share one canvas and origin per facing and are drawn in this order:
    body (skin) -> pants -> shirt -> vest / parka / duster -> beard -> hair -> tuque
Every non-body layer is rendered with the body as a holdout, so hidden
surfaces are already removed and any subset of layers composites correctly.
Neutral greys + tint masks give skin, hair and cloth variety.
"""
import math
import os

import bmesh

from .. import contracts as C
from .. import stage as S

FACINGS = ("south", "east", "north")      # west is east mirrored by the renderer
CX, CY = 0.5, -0.52                       # feet, in the tile
MARGINS = (0.125, 0.125, 0.10, 0.125)
TOP = 0.56                                # top of the body capsule (shoulders)
HEAD_Z = 0.75                             # head centre; the head overlaps the shoulders
HEAD_R = (0.19, 0.175, 0.19)              # head radii at build head scale 1
HGT = HEAD_Z + 0.30                       # tallest point of any layer (tuque bobble, curls)
R_BOT, R_TOP = 0.10, 0.12                 # rounding of the capsule's bottom and shoulders
SEG, SQUARE = 32, 2.4                     # ring segments; superellipse exponent of a section
G = 0.015                                 # apparel shell offset over the body
HOLD_Z, BACK_Z = 0.26, 0.44               # heights of the hold and back attachments

# ------------------------------------------------------------- builds
# NG-1: six builds. A build only changes the capsule's section profile and
# the head scale; heights (and so the head attachment) are shared, so hair,
# beards and hats rendered once on m_avg fit every build. m_avg keeps the
# un-suffixed file names.
#   W      half-width at z = KNOT_Z (hip, waist, chest, shoulder), x WS, its
#          spread about the build's mean softened by SOFT (no limbs to hide it)
#   F, B   extra half-depth in front (-Y) and behind (+Y) at the same knots
#   d      base half-depth                head  head scale about its centre
KNOT_Z = (0.12, 0.28, 0.40, 0.52)
WS = 1.4
SOFT = 0.55
BUILDS = {
    "m_thin": dict(W=(0.128, 0.122, 0.136, 0.142), F=(0, 0, 0, 0), B=(0, 0, 0, 0), d=0.088, head=1.03),
    "m_avg": dict(W=(0.150, 0.150, 0.150, 0.150), F=(0, 0, 0, 0), B=(0, 0, 0, 0), d=0.100, head=1.0),
    "m_heavy": dict(W=(0.182, 0.196, 0.186, 0.170), F=(0, 0.030, 0.010, 0), B=(0, 0.004, 0, 0), d=0.118,
                    head=0.97),
    "f_thin": dict(W=(0.156, 0.104, 0.108, 0.108), F=(0, 0, 0.014, 0.005), B=(0.010, 0, 0, 0), d=0.080,
                   head=1.02),
    "f_avg": dict(W=(0.180, 0.122, 0.128, 0.122), F=(0, 0, 0.020, 0.008), B=(0.014, 0, 0, 0), d=0.092,
                  head=0.985),
    "f_heavy": dict(W=(0.206, 0.164, 0.164, 0.146), F=(0, 0.030, 0.030, 0.010), B=(0.018, 0, 0, 0), d=0.115,
                    head=0.97),
}
BUILD_KEYS = tuple(BUILDS)
LEGACY_BUILD = "m_avg"        # un-suffixed file names
BUILD_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility", "parka_leather", "duster_leather")
_build_key = LEGACY_BUILD
_build = BUILDS[_build_key]
_FACING = ["south"]


def use_build(key):
    """Select the build every builder below reads from."""
    global _build, _build_key
    _build_key, _build = key, BUILDS[key]


def _knot(vals, z):
    """Smooth profile through the KNOT_Z knots, held flat beyond them."""
    zs = KNOT_Z
    if z <= zs[0]:
        return vals[0]
    for i in range(len(zs) - 1):
        if z <= zs[i + 1]:
            t = (z - zs[i]) / (zs[i + 1] - zs[i])
            t = t * t * (3.0 - 2.0 * t)
            return vals[i] + (vals[i + 1] - vals[i]) * t
    return vals[-1]


def _extent(z, g=0.0):
    """Unrounded (half-width, front half-depth, back half-depth) at z, grown by g."""
    b = _build
    mean = sum(b["W"]) / len(b["W"])
    return ((mean + SOFT * (_knot(b["W"], z) - mean)) * WS + g, (b["d"] + _knot(b["F"], z)) * WS + g,
            (b["d"] + _knot(b["B"], z)) * WS + g)


def _shrink(z, lo, hi, g):
    """How far (0..1 of the rounding radius) a section at z is pulled in by a
    rounded bottom ending at lo and/or rounded shoulders ending at hi."""
    s, r = 0.0, 0.0
    for rad, u in ((R_BOT + g, None if lo is None else (lo + R_BOT + g - z) / (R_BOT + g)),
                   (R_TOP + g, None if hi is None else (z - hi + R_TOP + g) / (R_TOP + g))):
        if u is not None and u > 0:
            k = 1.0 - math.sqrt(max(0.0, 1.0 - min(u, 1.0) ** 2))
            if k > s:
                s, r = k, rad
    return s, r


def _shell(name, z0, z1, g, mat, round_lo=False, round_hi=False, flare=None):
    """The body capsule between z0 and z1, grown by g (0 = the body itself).
    A rounded end closes the capsule (the bottom at -g, the shoulders at
    TOP + g); otherwise the end is a flat cut. flare=(z, extra) widens the
    shell linearly by up to `extra` below z (a coat's hem)."""
    lo = -g if round_lo else None
    hi = TOP + g if round_hi else None
    a, b = (lo if round_lo else z0), (hi if round_hi else z1)
    zs = {a, b}
    n = max(2, int((b - a) / 0.015))
    zs.update(a + (b - a) * i / n for i in range(n + 1))
    for end, rad, sgn in ((lo, R_BOT + g, 1), (hi, R_TOP + g, -1)):
        if end is not None:
            zs.update(end + sgn * rad * (1 - math.cos(math.pi / 2 * i / 8)) for i in range(9))
    zs = sorted({round(z, 5) for z in zs if a - 1e-9 <= z <= b + 1e-9})
    bm = bmesh.new()
    rings = []
    for z in zs:
        gz = g + (flare[1] * max(0.0, (flare[0] - z) / flare[0]) if flare else 0.0)
        wx, yf, yb = _extent(z, gz)
        s, rad = _shrink(z, lo, hi, g)
        wx, yf, yb = (e - min(rad, 0.9 * e) * s for e in (wx, yf, yb))
        ring = []
        for k in range(SEG):
            t = 2 * math.pi * k / SEG
            c, sn = math.cos(t), math.sin(t)
            x = math.copysign(abs(c) ** (2 / SQUARE), c) * wx
            y = math.copysign(abs(sn) ** (2 / SQUARE), sn) * (yb if sn > 0 else yf)
            ring.append(bm.verts.new((CX + x, CY + y, z)))
        rings.append(ring)
    for r0, r1 in zip(rings, rings[1:]):
        for k in range(SEG):
            bm.faces.new((r0[k], r0[(k + 1) % SEG], r1[(k + 1) % SEG], r1[k]))
    for ring, z in ((rings[0], zs[0]), (rings[-1], zs[-1])):
        c = bm.verts.new((CX, CY, z))
        for k in range(SEG):
            bm.faces.new((ring[k], ring[(k + 1) % SEG], c))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return S._obj(name, bm, mat, smooth=True)


# --------------------------------------------------------------- layers
def _head_point(dx, dy, dz=0.0):
    """A point on the head surface: dx, dy are fractions of the head radii."""
    h = _build["head"]
    return (CX + dx * HEAD_R[0] * h, CY + dy * HEAD_R[1] * h, HEAD_Z + dz * h)


def _body():
    """Skin capsule, head sphere and dot eyes (two south, one east, none north)."""
    h = _build["head"]
    objs = [_shell("torso", 0.0, TOP, 0.0, "skin", round_lo=True, round_hi=True),
            S.ellipsoid("head", CX, CY, HEAD_Z, HEAD_R[0] * h, HEAD_R[1] * h, HEAD_R[2] * h, "skin",
                        segs=28, rings=16)]
    # East shows the authored -X side (the facing turns -X towards the camera).
    eyes = {"south": ((0.37, -0.93), (-0.37, -0.93)), "east": ((-0.64, -0.77),), "north": ()}
    for dx, dy in eyes.get(_FACING[0], eyes["south"]):
        x, y, z = _head_point(dx, dy, -0.02)
        objs.append(S.ellipsoid("eye", x, y, z, 0.024 * h, 0.018 * h, 0.03 * h, "eye_black", segs=10, rings=6))
    return objs


def _pants():
    _shell("pants", 0.0, 0.27, G, "cloth", round_lo=True)
    _shell("belt", 0.20, 0.25, G + 0.008, "leather")


def _shirt():
    _shell("shirt", 0.25, TOP, G + 0.004, "cloth", round_hi=True)
    S.cyl("collar", CX, CY, 0.10, TOP - 0.02, TOP + 0.035, "cloth", segs=16, ry=0.085)


def _vest():
    """Chest band with two shoulder straps and two chest pockets."""
    _shell("vest", 0.22, 0.47, G + 0.02, "cloth")
    for sx in (-1, 1):
        wx, yf, yb = _extent(0.42, G + 0.02)
        x = CX + sx * 0.5 * wx
        S.box("strap", x - 0.026, CY - yf - 0.004, 0.40, x + 0.026, CY + yb + 0.004, TOP + G + 0.02,
              "leather", bevel=0.01)
        wx, yf, _ = _extent(0.34, G + 0.02)
        x = CX + sx * 0.52 * wx
        S.box("pocket", x - 0.045, CY - yf - 0.022, 0.29, x + 0.045, CY - yf + 0.01, 0.38, "leather",
              bevel=0.012)


def _parka():
    """Padded leather parka to the hips: fur hem, fur ruff and the hood folded on the back."""
    _shell("parka", 0.10, TOP, G + 0.02, "leather", round_hi=True)
    _shell("hem", 0.10, 0.15, G + 0.032, "fur_cream")
    _, _, yb = _extent(TOP - 0.06, G + 0.02)
    S.ellipsoid("hood", CX, CY + yb - 0.02, TOP - 0.04, 0.16, 0.07, 0.10, "leather", segs=16, rings=8)
    S.ellipsoid("ruff", CX, CY + 0.03, TOP + 0.02, 0.2, 0.16, 0.065, "fur_cream", segs=20, rings=8)


def _duster():
    """Long leather duster to the ground with a flared hem, an open front, lapels and a belt."""
    _shell("duster", 0.0, TOP, G + 0.012, "leather", round_hi=True, flare=(0.32, 0.07))
    _shell("dbelt", 0.26, 0.30, G + 0.022, "bark")
    _, yf, _ = _extent(0.13, G + 0.012 + 0.04)
    S.box("opening", CX - 0.012, CY - yf - 0.01, 0.0, CX + 0.012, CY - yf + 0.03, 0.26, "bark", bevel=0.004)
    for sx in (-1, 1):
        _, yf, _ = _extent(0.42, G + 0.012)
        x = CX + sx * 0.055
        S.box("lapel", x - 0.04, CY - yf - 0.014, 0.33, x + 0.04, CY - yf + 0.01, TOP - 0.02, "bark", bevel=0.01)


def _tuque():
    """Knitted tuque: a snug cap with a rolled cuff and a bobble on top."""
    _hair_shell("tuque", 0.04, [_plane(HEAD_Z + 0.05, HEAD_Z + 0.05)], mat="cloth")
    S.cyl("tcuff", CX, CY, 0.232, HEAD_Z + 0.02, HEAD_Z + 0.08, "fur_tan", segs=24, ry=0.218)
    S.ellipsoid("bobble", CX, CY, HEAD_Z + 0.24, 0.05, 0.05, 0.05, "fur_cream", segs=10, rings=6)


def _plane(front, back, above=True):
    """A cut plane through the head's profile: height `front` at the face and
    `back` at the back of the skull (a straight, sloped hairline or jawline).
    Keeps what lies above it (or below, above=False)."""
    ry = HEAD_R[1] + 0.03
    n = (0.0, -(back - front), 2 * ry)
    return (CX, CY - ry, front), n if above else tuple(-c for c in n)


def _vplane(y, front_side=True):
    """A vertical cut at depth y keeping the face side (y below) or the back."""
    return (CX, y, 0.0), (0.0, -1.0, 0.0) if front_side else (0.0, 1.0, 0.0)


def _hair_shell(name, grow, planes, mat="hair"):
    """A shell over the head trimmed by clean plane cuts (no stepped hairlines),
    solidified so the outline has a body to follow."""
    h = _build["head"]
    ob = S.ellipsoid(name, CX, CY, HEAD_Z, HEAD_R[0] * h + grow, HEAD_R[1] * h + grow, HEAD_R[2] * h + grow,
                     mat, segs=40, rings=24)
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    for co, no in planes:
        bmesh.ops.bisect_plane(bm, geom=bm.verts[:] + bm.edges[:] + bm.faces[:], dist=1e-6,
                               plane_co=co, plane_no=no, clear_inner=True)
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()
    sol = ob.modifiers.new("solid", "SOLIDIFY")
    sol.thickness = 0.02
    sol.offset = -1
    return ob


def _fringe(dx, w, dz, depth=0.05):
    """A rounded lock lying on the forehead (never a box standing proud of it)."""
    z = HEAD_Z + dz
    y = CY - HEAD_R[1] * math.sqrt(max(0.0, 1 - (dz / HEAD_R[2]) ** 2)) + 0.01
    S.ellipsoid("fringe", CX + dx, y, z, w, 0.042, depth, "hair", segs=16, rings=8)


def _hair(style):
    H = HEAD_Z
    if style == "hair_short":
        _hair_shell("hair", 0.025, [_plane(H + 0.06, H - 0.18)])
        _fringe(-0.03, 0.10, 0.10)
    elif style == "hair_crop":
        _hair_shell("hair", 0.012, [_plane(H + 0.10, H - 0.10)])
    elif style == "hair_bun":
        _hair_shell("hair", 0.022, [_plane(H + 0.05, H - 0.21)])
        S.ellipsoid("bun", CX, CY + 0.17, H + 0.12, 0.085, 0.08, 0.085, "hair", segs=14, rings=8)
        S.cyl("tie", CX, CY + 0.13, 0.05, H + 0.08, H + 0.10, "food_red", segs=10, ry=0.04)
    elif style == "hair_long":
        _hair_shell("hair", 0.026, [_plane(H + 0.06, H - 0.24)])
        _fringe(-0.02, 0.11, 0.10)
        # a curtain behind the head and a lock down each side of the face, to the shoulders
        S.ellipsoid("hair_back", CX, CY + 0.10, H - 0.11, 0.19, 0.11, 0.19, "hair", segs=24, rings=12)
        for sx in (-1, 1):
            S.ellipsoid("lock", CX + sx * 0.18, CY + 0.02, H - 0.10, 0.04, 0.09, 0.14, "hair", segs=14, rings=8)
    elif style == "hair_braid":
        _hair_shell("hair", 0.024, [_plane(H + 0.06, H - 0.19)])
        _fringe(0.03, 0.09, 0.10)
        # one braid over the +X shoulder, down the chest, tied at the end
        n = 6
        for i in range(n):
            t = i / (n - 1)
            r = 0.044 - 0.014 * t
            S.ellipsoid("braid", CX + 0.17 - 0.03 * t, CY + 0.02 - 0.21 * t, H - 0.09 - 0.30 * t,
                        r, r, 0.052, "hair", segs=10, rings=6)
        S.cyl("tie", CX + 0.14, CY - 0.19, 0.03, H - 0.42, H - 0.40, "food_red", segs=10)
    elif style == "hair_shaved":
        # stubble: a thin skin-tight cap
        _hair_shell("hair", 0.008, [_plane(H + 0.08, H - 0.09)])
    elif style == "hair_curls":
        _hair_shell("hair", 0.035, [_plane(H + 0.02, H - 0.21)])
        # a ring of curls round the head and two on the crown
        for k in range(9):
            a = 2 * math.pi * k / 9 + 0.3
            x, y = CX + 0.2 * math.cos(a), CY + 0.185 * math.sin(a)
            if y > CY - 0.09 or abs(x - CX) > 0.1:
                S.ellipsoid("curl", x, y, H + 0.02, 0.05, 0.05, 0.05, "hair", segs=10, rings=6)
        for k in range(8):
            a = 2 * math.pi * k / 8 + 0.1
            S.ellipsoid("curl", CX + 0.13 * math.cos(a), CY + 0.12 * math.sin(a), H + 0.13,
                        0.055, 0.055, 0.05, "hair", segs=10, rings=6)
        for x, y in ((0.0, 0.0), (0.06, 0.04), (-0.06, 0.03)):
            S.ellipsoid("curl", CX + x, CY + y, H + 0.2, 0.055, 0.055, 0.05, "hair", segs=10, rings=6)
    elif style == "hair_tail":
        _hair_shell("hair", 0.02, [_plane(H + 0.06, H - 0.12)])
        _fringe(-0.02, 0.09, 0.10)
        # a high ponytail hanging behind the head
        S.cyl("tie", CX, CY + 0.17, 0.045, H + 0.06, H + 0.085, "food_red", segs=10, ry=0.04)
        for i, (dy, dz, r, rz) in enumerate(((0.20, 0.03, 0.05, 0.07), (0.225, -0.07, 0.055, 0.08),
                                             (0.235, -0.18, 0.05, 0.08), (0.24, -0.28, 0.036, 0.07))):
            S.ellipsoid("tail", CX + 0.008 * i, CY + dy, H + dz, r, r, rz, "hair", segs=10, rings=6)
    else:
        raise KeyError(style)


def _beard(style):
    """Beards (males): shells and masses on the lower face, tinted like hair."""
    H = HEAD_Z
    if style == "beard_short":
        _hair_shell("beard", 0.02, [_plane(H - 0.06, H + 0.0, above=False), _plane(H - 0.21, H - 0.16),
                                    _vplane(CY - 0.02)])
    elif style == "beard_full":
        _hair_shell("beard", 0.03, [_plane(H - 0.05, H + 0.04, above=False), _vplane(CY - 0.01)])
        S.ellipsoid("chin", CX, CY - 0.07, H - 0.2, 0.095, 0.075, 0.085, "hair", segs=14, rings=8)
        for sx in (-1, 1):
            S.ellipsoid("mous", CX + sx * 0.045, CY - 0.16, H - 0.105, 0.058, 0.035, 0.024, "hair", segs=10, rings=6)
    elif style == "beard_moustache":
        # one smooth band over the upper lip, drooping at the ends; the ends show past
        # the head from behind
        _hair_shell("beard", 0.024, [_plane(H - 0.125, H - 0.125), _plane(H - 0.075, H - 0.075, above=False),
                                     _vplane(CY - 0.10)])
        for sx in (-1, 1):
            S.ellipsoid("mous", CX + sx * 0.13, CY - 0.11, H - 0.13, 0.035, 0.03, 0.05, "hair", segs=12, rings=8)
            S.ellipsoid("mous", CX + sx * 0.155, CY - 0.07, H - 0.155, 0.03, 0.03, 0.04, "hair", segs=12, rings=8)
    else:
        raise KeyError(style)


def _face_scar():
    """A thin healed scar slanting across the brow and cheek (ENG-003, ART §37):
    a pink ridge with three dark stitch marks lying on the side of the head the
    facing shows (the back of the skull facing north). No outline, no tint."""
    from mathutils import Matrix, Vector
    h = _build["head"]
    dx, dy = {"south": (0.45, -0.88), "east": (-0.58, -0.80), "north": (0.45, 0.88)}[_FACING[0]]
    cx, cy, cz = _head_point(dx, dy, 0.005)
    normal = Vector((dx / HEAD_R[0], dy / HEAD_R[1], 0.0)).normalized()
    ridge = S.ellipsoid("scar", cx, cy, cz, 0.016 * h, 0.016 * h, 0.095 * h, "food_red", segs=10, rings=6)
    marks = [ridge]
    across = math.atan2(normal.y, normal.x) + math.pi / 2     # horizontal tangent of the head
    for k, dz in enumerate((-0.05, 0.0, 0.05)):
        st = S.ellipsoid("stitch%d" % k, 0, 0, 0, 0.036 * h, 0.012 * h, 0.009 * h, "blood_dried", segs=8, rings=4)
        st.matrix_world = (Matrix.Translation((cx + normal.x * 0.004, cy + normal.y * 0.004, cz + dz * h))
                           @ Matrix.Rotation(across, 4, "Z"))
        marks.append(st)
    pivot = Matrix.Translation((cx, cy, cz))
    S.transform(marks, pivot @ Matrix.Rotation(math.radians(-28), 4, normal) @ pivot.inverted())


def _apply_modifiers():
    import bpy
    for ob in S.objects():
        if ob.modifiers:
            dg = bpy.context.evaluated_depsgraph_get()
            me = bpy.data.meshes.new_from_object(ob.evaluated_get(dg))
            old = ob.data
            ob.modifiers.clear()
            ob.data = me
            bpy.data.meshes.remove(old)


# --------------------------------------------------- facing, lean, canvas
def _lean_matrix():
    from mathutils import Matrix
    t = math.radians(C.FIGURE_LEAN_DEG)
    feet = Matrix.Translation((0.5, CY, 0))
    return (feet @ Matrix.Rotation(-t, 4, "X") @ Matrix.Scale(C.FIGURE_SCALE, 4)
            @ Matrix.Translation((-0.5, -CY, 0)))


def _facing_matrix(facing):
    from mathutils import Matrix
    return (Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(S.FACINGS[facing]), 4, "Z")
            @ Matrix.Translation((-0.5, 0.5, 0)))


def _figure(facing):
    """Facing rotation, then the figure lean (always towards the viewer)."""
    S.face(S.objects(), facing, (1, 1))
    S.transform(S.objects(), _lean_matrix())


def _zmax_screen():
    """Equivalent sheared height that covers the leaned figure."""
    t = math.radians(C.FIGURE_LEAN_DEG)
    return HGT * C.FIGURE_SCALE * (math.sin(t) / C.SHEAR_K + math.cos(t)) + 0.05


def _origin(zm, margins=MARGINS):
    """The canvas origin render_asset derives for zmax zm and these margins."""
    return (int(round(margins[0] * C.PPT)), int(math.ceil((C.SHEAR_K * zm + margins[2]) * C.PPT)))


def _project(x, y, z, origin):
    """Blender point -> image px on a canvas with this origin (sheared)."""
    return [round(origin[0] + x * C.PPT, 1), round(origin[1] + (-y - C.SHEAR_K * z) * C.PPT, 1)]


def _attachments(facing, origin):
    """Body attachments in canvas px: `head` (head centre), `hold` (where a held
    item's grip sits: right-front of the torso facing south, in front of the
    chest facing east, behind the body facing north) and `back` (between the
    shoulders, for slung items)."""
    from mathutils import Vector
    lean, turn = _lean_matrix(), _facing_matrix(facing)
    wx, yf, yb = _extent(HOLD_Z)
    hold = {"south": (CX + 0.75 * wx, CY - yf - 0.02), "east": (CX + yf + 0.03, CY - 0.04),
            "north": (CX - 0.75 * wx, CY + yf + 0.02)}[facing]
    pts = {"head": turn @ Vector((CX, CY, HEAD_Z)),
           "hold": Vector((hold[0], hold[1], HOLD_Z)),
           "back": turn @ Vector((CX, CY + _extent(BACK_Z)[2], BACK_Z))}
    return {k: _project(*(lean @ p), origin) for k, p in pts.items()}


def _layer(name, facing, builder, body_holdout=True, outline=True):
    _FACING[0] = facing
    if body_holdout:
        S.holdout(_body())
    builder()
    _apply_modifiers()
    _figure(facing)
    zm = _zmax_screen()
    extra = {"layer": name, "facing": facing}
    if name == "body":
        extra["attachments"] = _attachments(facing, _origin(zm))
    tag = name if _build_key == LEGACY_BUILD or name not in BUILD_LAYERS else "%s_%s" % (name, _build_key)
    if tag != name:
        extra["build"] = _build_key
    return S.render_asset("pawn", "%s_%s" % (tag, facing), zmax=zm, margins=MARGINS, extra=extra, outline=outline)


LAYERS = {
    "pants_basic": _pants,
    "shirt_basic": _shirt,
    "vest_utility": _vest,
    "parka_leather": _parka,
    "duster_leather": _duster,
    "tuque_knit": _tuque,
    **{s: (lambda s=s: _hair(s)) for s in ("hair_short", "hair_crop", "hair_bun", "hair_long", "hair_braid",
                                           "hair_shaved", "hair_curls", "hair_tail")},
    **{s: (lambda s=s: _beard(s)) for s in ("beard_short", "beard_full", "beard_moustache")},
    "face_scar": _face_scar,
}


def _env_list(name):
    v = os.environ.get(name, "").strip()
    return [x for x in v.split(",") if x] or None


def build(facings=None, layers=None, builds=None):
    """Every build x layer x facing, one standing sprite each.
    facings/layers/builds narrow a review build (e.g. layers=["body",
    "shirt_basic"], builds=["f_avg"]); RWG_PAWN_BUILDS / RWG_PAWN_LAYERS (comma
    lists) do the same for headless runs. Only BUILD_LAYERS depend on the build;
    hair, beards, hats and the scar are shared and rendered once, on m_avg."""
    builds = builds or _env_list("RWG_PAWN_BUILDS") or list(BUILD_KEYS)
    layers = layers or _env_list("RWG_PAWN_LAYERS")
    out = []
    try:
        for bk in builds:
            use_build(bk)
            for f in facings or FACINGS:
                for name in ["body"] + list(LAYERS):
                    if layers is not None and name not in layers:
                        continue
                    if name not in BUILD_LAYERS and bk != LEGACY_BUILD:
                        continue          # shared layers: one render, on m_avg
                    if name == "body":
                        out.append(_layer("body", f, _body, body_holdout=False))
                    else:
                        out.append(_layer(name, f, LAYERS[name], outline=name != "face_scar"))
    finally:
        use_build(LEGACY_BUILD)
        _FACING[0] = "south"
    return out
