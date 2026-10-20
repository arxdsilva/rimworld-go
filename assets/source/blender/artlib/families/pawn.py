"""Compositional pawns (ART spec §10) and the first weapon (§15).

Layers share one canvas/origin per facing and are drawn in this order:
    body (skin) -> pants -> shirt -> vest -> hair -> held equipment
Every non-body layer is rendered with the body as a holdout, so hidden
surfaces are already removed and any subset of layers composites
correctly. Neutral greys + tint masks give skin/hair/cloth variety.

Original proportions: compact geometric torso, short legs, large head.
"""
import math

import bmesh

from .. import contracts as C
from .. import stage as S

FACINGS = ("south", "east", "north", "west")
CX, CY = 0.5, -0.52
ZMAX = 1.10
MARGINS = (0.125, 0.125, 0.10, 0.125)
HEAD_Z = 0.84


def _body():
    objs = []
    for sx in (-1, 1):
        objs.append(S.cyl("leg", CX + sx * 0.075, CY, 0.062, 0.0, 0.32, "skin", segs=10))
        objs.append(S.cyl("arm", CX + sx * 0.19, CY, 0.048, 0.33, 0.60, "skin", segs=10))
        objs.append(S.ellipsoid("hand", CX + sx * 0.195, CY - 0.01, 0.31, 0.055, 0.055, 0.05, "skin", segs=10, rings=6))
    objs.append(S.box("torso", CX - 0.15, CY - 0.10, 0.28, CX + 0.15, CY + 0.10, 0.63, "skin", bevel=0.06, segments=2))
    objs.append(S.cyl("neck", CX, CY, 0.06, 0.60, 0.70, "skin", segs=10))
    objs.append(S.ellipsoid("head", CX, CY, HEAD_Z, 0.19, 0.175, 0.19, "skin", segs=24, rings=14))
    for sx in (-1, 1):
        objs.append(S.ellipsoid("eye", CX + sx * 0.068, CY - 0.158, HEAD_Z - 0.01, 0.026, 0.02, 0.034,
                                "metal_dark", segs=10, rings=6))
    objs.append(S.ellipsoid("nose", CX, CY - 0.175, HEAD_Z - 0.06, 0.03, 0.02, 0.03, "skin", segs=8, rings=6))
    return objs


def _pants():
    for sx in (-1, 1):
        S.cyl("pleg", CX + sx * 0.075, CY, 0.074, 0.05, 0.34, "cloth", segs=12)
    S.box("hips", CX - 0.16, CY - 0.11, 0.26, CX + 0.16, CY + 0.11, 0.40, "cloth", bevel=0.05, segments=2)
    S.box("belt", CX - 0.165, CY - 0.115, 0.38, CX + 0.165, CY + 0.115, 0.42, "leather", bevel=0.02)


def _shirt():
    S.box("shirt", CX - 0.162, CY - 0.112, 0.36, CX + 0.162, CY + 0.112, 0.645, "cloth", bevel=0.07, segments=2)
    for sx in (-1, 1):
        S.cyl("sleeve", CX + sx * 0.19, CY, 0.06, 0.45, 0.62, "cloth", segs=12)
    S.cyl("collar", CX, CY, 0.075, 0.62, 0.67, "cloth", segs=12)


def _vest():
    S.box("vest", CX - 0.172, CY - 0.122, 0.40, CX + 0.172, CY + 0.122, 0.635, "cloth", bevel=0.06, segments=2)
    for sx in (-1, 1):
        S.box("pocket", CX + sx * 0.085 - 0.05, CY - 0.135, 0.43, CX + sx * 0.085 + 0.05, CY - 0.10, 0.52,
              "leather", bevel=0.012)
        S.box("strap", CX + sx * 0.09 - 0.025, CY - 0.127, 0.40, CX + sx * 0.09 + 0.025, CY + 0.127, 0.645,
              "leather", bevel=0.008)


def _parka():
    """Leather parka over the shirt: padded body to the hips, sleeves, and
    a fur ruff around the neck."""
    S.box("parka", CX - 0.182, CY - 0.132, 0.26, CX + 0.182, CY + 0.132, 0.655, "leather", bevel=0.075, segments=2)
    for sx in (-1, 1):
        S.cyl("psleeve", CX + sx * 0.195, CY, 0.068, 0.33, 0.63, "leather", segs=12)
        S.cyl("pcuff", CX + sx * 0.195, CY, 0.072, 0.33, 0.37, "fur_cream", segs=12)
    S.cyl("ruff", CX, CY, 0.11, 0.62, 0.70, "fur_cream", segs=14)
    S.box("hem", CX - 0.186, CY - 0.136, 0.26, CX + 0.186, CY + 0.136, 0.30, "fur_cream", bevel=0.03)


def _duster():
    """Long leather duster: open coat down past the knees, wide lapels."""
    S.box("duster", CX - 0.176, CY - 0.126, 0.12, CX + 0.176, CY + 0.126, 0.645, "leather", bevel=0.06, segments=2)
    for sx in (-1, 1):
        S.cyl("dsleeve", CX + sx * 0.193, CY, 0.064, 0.36, 0.63, "leather", segs=12)
        S.box("lapel", CX + sx * 0.06 - 0.04, CY - 0.14, 0.44, CX + sx * 0.06 + 0.04, CY - 0.12, 0.64, "bark",
              bevel=0.01)
    S.box("dbelt", CX - 0.18, CY - 0.13, 0.36, CX + 0.18, CY + 0.13, 0.40, "bark", bevel=0.015)


def _tuque():
    """Knitted tuque: a snug cap with a rolled cuff and a bobble on top."""
    _hair_shell("tuque", 0.04, lambda x, y, z: z > HEAD_Z + 0.02, mat="cloth")
    S.cyl("tcuff", CX, CY, 0.215, HEAD_Z + 0.01, HEAD_Z + 0.07, "fur_tan", segs=20)
    S.ellipsoid("bobble", CX, CY, HEAD_Z + 0.25, 0.055, 0.055, 0.055, "fur_cream", segs=10, rings=6)


def _cut(ob, keep):
    """Delete vertices of a mesh for which keep(x, y, z) is False."""
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    kill = [v for v in bm.verts if not keep(v.co.x, v.co.y, v.co.z)]
    bmesh.ops.delete(bm, geom=kill, context="VERTS")
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()


def _hair_shell(name, grow, keep, mat="hair"):
    ob = S.ellipsoid(name, CX, CY, HEAD_Z, 0.19 + grow, 0.175 + grow, 0.19 + grow, mat, segs=28, rings=16)
    _cut(ob, keep)
    # give the shell thickness so the outline has a body to follow
    sol = ob.modifiers.new("solid", "SOLIDIFY")
    sol.thickness = 0.02
    sol.offset = -1
    return ob


def _hair(style):
    front = CY - 0.06  # y below this is "face side"
    if style == "hair_short":
        _hair_shell("hair", 0.025, lambda x, y, z: z > HEAD_Z + 0.05 or (y > front and z > HEAD_Z - 0.12))
        S.box("fringe", CX - 0.12, CY - 0.19, HEAD_Z + 0.07, CX + 0.06, CY - 0.13, HEAD_Z + 0.15, "hair",
              bevel=0.025)
    elif style == "hair_crop":
        _hair_shell("hair", 0.012, lambda x, y, z: z > HEAD_Z + 0.09 or (y > CY + 0.02 and z > HEAD_Z - 0.04))
    elif style == "hair_bun":
        _hair_shell("hair", 0.022, lambda x, y, z: z > HEAD_Z + 0.03 or (y > front and z > HEAD_Z - 0.16))
        S.ellipsoid("bun", CX, CY + 0.17, HEAD_Z + 0.12, 0.085, 0.08, 0.085, "hair", segs=14, rings=8)
        S.cyl("tie", CX, CY + 0.13, 0.05, HEAD_Z + 0.08, HEAD_Z + 0.10, "food_red", segs=10, ry=0.04)
    else:
        raise KeyError(style)


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


# Walk cycle (ART spec §27.1): 12 unique poses per facing, walk01..walk12.
# Frame 01 is the right heel contact; the right leg then loads, passes
# mid-stance, terminal stance, pre-swing and swings through; 07..12 are the
# same poses for the left foot. Per-frame curves:
#   hip     +-26 deg (cosine, forward at contact)
#   knee    0-40 deg, peaking in swing
#   pelvis  drops 2-3 px at loading (everything above the legs)
#   arms    counter-swing at 0.8x the hip
#   torso   twists +-4 deg against the pelvis
# Only layers that cover limbs get frames; hair rides the head (bobbed in
# the renderer).
WALK_DEG = 26.0
WALK_N = 12
WALK_FRAMES = {"walk%02d" % i: i for i in range(1, WALK_N + 1)}
WALK_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility", "parka_leather", "duster_leather")
HIP_Z, SHOULDER_Z = 0.33, 0.60
KNEE_Z = 0.18
# right-knee flexion per frame (deg); the left knee reads the table 6 frames on
KNEE_R = (3, 14, 8, 5, 6, 16, 34, 40, 36, 22, 8, 3)
# pelvis drop weight per half-cycle frame (loading is the deepest)
DROP_W = (0.5, 1.0, 0.55, 0.1, 0.0, 0.15)
DROP_MAX = 0.022

# Work swing (ART spec §27.1): both hands grip the tool and swing it
#   work1 wind-up start, work2 wind-up peak, work3 swing, work4 late swing,
#   work5 strike, work6 recover.
# Shoulder rotations in degrees (negative swings the hand forward and up);
# smoothstep eases from the wind-up peak to the strike. Legs are braced.
WORK_N = 6
WORK_FRAMES = {"work%d" % i: i for i in range(1, WORK_N + 1)}
WORK_LAYERS = ("body", "shirt_basic", "pants_basic", "vest_utility", "parka_leather", "duster_leather")
WORK_ARM = {"w_start": -142.0, "w_peak": -162.0, "strike": -38.0, "recover": -78.0}


def _smooth(t):
    t = min(max(t, 0.0), 1.0)
    return t * t * (3.0 - 2.0 * t)


def _params(frame):
    """Pose parameters for a frame: forward hip/knee flexion per leg (deg),
    shoulder rotations (deg, negative = forward), pelvis drop (tiles) and
    torso twist (deg)."""
    zero = dict(hr=0.0, hl=0.0, kr=0.0, kl=0.0, ar=0.0, al=0.0, drop=0.0, twist=0.0)
    if frame in WALK_FRAMES:
        i = WALK_FRAMES[frame]
        hr = WALK_DEG * math.cos(2 * math.pi * (i - 1) / WALK_N)
        return dict(hr=hr, hl=-hr, kr=KNEE_R[i - 1], kl=KNEE_R[(i - 1 + 6) % 12],
                    ar=0.8 * hr, al=-0.8 * hr, drop=DROP_MAX * DROP_W[(i - 1) % 6],
                    twist=4.0 * hr / WALK_DEG)
    if frame in WORK_FRAMES:
        i = WORK_FRAMES[frame]
        w = WORK_ARM
        if i == 1:
            r, e = w["w_start"], 0.0
        elif i == 2:
            r, e = w["w_peak"], 0.0
        elif i == 6:
            r, e = w["recover"], 1.0
        else:  # 3, 4, 5 ease from the peak into the strike
            e = _smooth((i - 2) / 3.0)
            r = w["w_peak"] + (w["strike"] - w["w_peak"]) * e
        return dict(hr=11.0, hl=-9.0, kr=14.0, kl=14.0, ar=r, al=r + 7.0 - 3.0 * e,
                    drop=0.012 + 0.012 * e, twist=-3.0 + 6.0 * e)
    return zero


def _mean_x(ob):
    vs = ob.data.vertices
    return sum((ob.matrix_world @ v.co).x for v in vs) / max(len(vs), 1)


def _rot_x(deg, pz):
    from mathutils import Matrix
    return (Matrix.Translation((0, CY, pz)) @ Matrix.Rotation(math.radians(deg), 4, "X")
            @ Matrix.Translation((0, -CY, -pz)))


def _torso_matrix(p):
    """Pelvis drop + torso twist, applied to everything above the legs."""
    from mathutils import Matrix
    tw = (Matrix.Translation((CX, CY, 0)) @ Matrix.Rotation(math.radians(p["twist"]), 4, "Z")
          @ Matrix.Translation((-CX, -CY, 0)))
    return Matrix.Translation((0, 0, -p["drop"])) @ tw


def _bend_knee(ob, flex):
    """Bend a leg mesh at the knee: cut loops around KNEE_Z, then rotate the
    shank (feet go backwards) with a smooth ramp over the cut loops."""
    ob.data.transform(ob.matrix_world)
    from mathutils import Matrix
    ob.matrix_world = Matrix.Identity(4)
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    for z in (KNEE_Z + 0.03, KNEE_Z, KNEE_Z - 0.03):
        bmesh.ops.bisect_plane(bm, geom=list(bm.verts) + list(bm.edges) + list(bm.faces),
                               plane_co=(0, 0, z), plane_no=(0, 0, 1), dist=1e-5)
    for v in bm.verts:
        w = min(max((KNEE_Z + 0.03 - v.co.z) / 0.06, 0.0), 1.0)
        if w <= 0.0:
            continue
        a = math.radians(flex * w)
        dy, dz = v.co.y - CY, v.co.z - KNEE_Z
        v.co.y = CY + dy * math.cos(a) - dz * math.sin(a)
        v.co.z = KNEE_Z + dy * math.sin(a) + dz * math.cos(a)
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()


def _pose(frame):
    """Pose a walk or work frame. Forward is -Y; a forward swing is a
    negative rotation about X. Applied to every object (holdout body
    included) before the facing."""
    p = _params(frame)
    torso = _torso_matrix(p)
    for ob in S.objects():
        n = ob.name.split(".")[0]
        right = _mean_x(ob) >= CX
        if n in ("leg", "pleg"):
            flex = p["kr"] if right else p["kl"]
            if flex > 0:
                _bend_knee(ob, flex)
            hip = p["hr"] if right else p["hl"]
            ob.matrix_world = _rot_x(-hip, HIP_Z) @ ob.matrix_world
        else:
            m = torso
            if n in ("arm", "hand", "sleeve", "psleeve", "pcuff", "dsleeve"):
                m = torso @ _rot_x(p["ar"] if right else p["al"], SHOULDER_Z)
            ob.matrix_world = m @ ob.matrix_world


def _lean_matrix():
    import math
    from mathutils import Matrix
    t = math.radians(C.FIGURE_LEAN_DEG)
    feet = Matrix.Translation((0.5, CY, 0))
    return (feet @ Matrix.Rotation(-t, 4, "X") @ Matrix.Scale(C.FIGURE_SCALE, 4)
            @ Matrix.Translation((-0.5, -CY, 0)))


def _figure(facing):
    """Facing rotation, then the figure lean (always towards the viewer)."""
    S.face(S.objects(), facing, (1, 1))
    S.transform(S.objects(), _lean_matrix())


def _zmax_screen():
    """Equivalent sheared height that covers the leaned figure."""
    import math
    t = math.radians(C.FIGURE_LEAN_DEG)
    hgt = 1.05 * C.FIGURE_SCALE
    return hgt * (math.sin(t) / C.SHEAR_K + math.cos(t)) + 0.05


def _project(x, y, z, origin):
    """Blender point -> image px on the pawn canvas (sheared)."""
    return [round(origin[0] + x * C.PPT, 1), round(origin[1] + (-y - C.SHEAR_K * z) * C.PPT, 1)]


HAND = {  # where held equipment's grip goes, per facing (Blender coords before facing rotation)
    "south": (CX + 0.195, CY - 0.06, 0.34),
}
HEAD_CENTER = (CX, CY, HEAD_Z)


def _attach(facing, frame, point):
    """Project a rest-pose body point (right-arm attachments ride the right
    arm) through the pose, facing and lean to canvas px."""
    from mathutils import Matrix, Vector
    deg = S.FACINGS[facing]
    m = (Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-0.5, 0.5, 0)))
    p = _params(frame)
    return _lean_matrix() @ m @ point_matrix(p, point) @ Vector(point[0])


def point_matrix(p, point):
    return _torso_matrix(p) @ (_rot_x(p["ar"], SHOULDER_Z) if point[1] == "arm" else _rot_x(0, SHOULDER_Z))


def _layer(name, facing, builder, body_holdout=True, frame=None):
    if body_holdout:
        objs = _body()
        S.holdout(objs)
    builder()
    _apply_modifiers()
    if frame:
        _pose(frame)
    _figure(facing)
    zm = _zmax_screen()
    top = int(-(-((C.SHEAR_K * zm + MARGINS[2]) * C.PPT) // 1))
    origin = (int(round(MARGINS[0] * C.PPT)), top)
    h = _attach(facing, frame, (HAND["south"], "arm"))
    att = {"hand": _project(h.x, h.y, h.z, origin)}
    if name == "body":
        hd = _attach(facing, frame, (HEAD_CENTER, "head"))
        att["head"] = _project(hd.x, hd.y, hd.z, origin)
    sprite = "%s_%s" % (name, facing) + ("_" + frame if frame else "")
    extra = {"layer": name, "facing": facing, "attachments": att}
    if frame:
        extra["frame"] = frame
    return S.render_asset("pawn", sprite, zmax=zm, margins=MARGINS, shadow=(name == "body"), extra=extra)


LAYERS = {
    "pants_basic": _pants,
    "shirt_basic": _shirt,
    "vest_utility": _vest,
    "parka_leather": _parka,
    "duster_leather": _duster,
    "tuque_knit": _tuque,
    "hair_short": lambda: _hair("hair_short"),
    "hair_crop": lambda: _hair("hair_crop"),
    "hair_bun": lambda: _hair("hair_bun"),
}


def _rifle():
    """Lying flat, muzzle pointing east (+X). Grip/muzzle in image px."""
    y = -0.5
    S.box("stock", 0.06, y - 0.045, 0.0, 0.30, y + 0.045, 0.06, "wood", bevel=0.02)
    S.box("receiver", 0.28, y - 0.04, 0.0, 0.58, y + 0.04, 0.07, "metal_dark", bevel=0.012)
    S.box("grip", 0.30, y - 0.10, 0.0, 0.36, y - 0.03, 0.05, "wood", bevel=0.012)
    S.box("mag", 0.44, y - 0.13, 0.0, 0.51, y - 0.03, 0.045, "metal_dark", bevel=0.01)
    S.box("guard", 0.56, y - 0.035, 0.0, 0.74, y + 0.035, 0.055, "wood", bevel=0.012)
    S.cyl("barrel", 0, 0, 0.018, 0.0, 0.24, "steel", segs=8, bevel=0.0)
    import math
    from mathutils import Matrix
    S.objects()[-1].matrix_world = Matrix.Translation((0.72, y, 0.03)) @ Matrix.Rotation(math.radians(90), 4, "Y")
    S.box("sight", 0.40, y - 0.015, 0.07, 0.46, y + 0.015, 0.10, "metal_dark", bevel=0.005)
    S.transform(S.objects(), Matrix.Translation((0.5, y, 0)) @ Matrix.Scale(0.8, 4) @ Matrix.Translation((-0.5, -y, 0)))
    m = (int(round(0.125 * C.PPT)), int(-(-((C.SHEAR_K * 0.11 + 0.125) * C.PPT) // 1)))
    return S.render_asset("weapon", "rifle_basic", zmax=0.11,
                          extra={"weapon": "rifle", "rotatable": True, "points": "east",
                                 "attachments": {"grip": _project(0.5 + (0.33 - 0.5) * 0.8, y - 0.056, 0.016, m),
                                                 "muzzle": _project(0.5 + (0.96 - 0.5) * 0.8, y, 0.024, m)}})


def build(walk_only=False, work_only=False):
    out = []
    for f in FACINGS:
        frames = [None] + list(WALK_FRAMES) + list(WORK_FRAMES)
        if walk_only:
            frames = list(WALK_FRAMES)
        if work_only:
            frames = list(WORK_FRAMES)
        for fr in frames:
            out.append(_body_sprite(f, fr))
        for name, fn in LAYERS.items():
            for fr in frames:
                if fr is None or (fr in WALK_FRAMES and name in WALK_LAYERS) or (fr in WORK_FRAMES and name in WORK_LAYERS):
                    out.append(_layer(name, f, fn, frame=fr))
    if not walk_only and not work_only:
        out.append(_rifle())
    if not walk_only:
        out += _tools()
    return out


def _tool_render(name, length, grip_x):
    """Render the staged tool lying flat, pointing east (head at +X)."""
    m = (int(round(0.125 * C.PPT)), int(-(-((C.SHEAR_K * 0.12 + 0.125) * C.PPT) // 1)))
    return S.render_asset("tool", name, zmax=0.12,
                          extra={"tool": name, "rotatable": True, "points": "east",
                                 "attachments": {"grip": _project(grip_x, -0.5, 0.02, m),
                                                 "head": _project(grip_x + length, -0.5, 0.04, m)}})


def _tools():
    """Hand tools held during work: hammer (building), axe (chopping),
    pickaxe (mining). Lying flat, pointing east, like the rifle."""
    import math
    from mathutils import Matrix
    y = -0.5
    out = []

    def handle(x0, x1):
        S.cyl("handle", 0, 0, 0.022, 0.0, x1 - x0, "wood", segs=8, bevel=0.0)
        S.objects()[-1].matrix_world = Matrix.Translation((x0, y, 0.03)) @ Matrix.Rotation(math.radians(90), 4, "Y")

    handle(0.18, 0.74)
    S.box("head", 0.70, y - 0.10, 0.0, 0.80, y + 0.07, 0.07, "steel", bevel=0.012)
    S.box("claw", 0.72, y + 0.07, 0.01, 0.78, y + 0.13, 0.05, "metal_dark", bevel=0.008)
    out.append(_tool_render("hammer", 0.56, 0.26))

    handle(0.14, 0.82)
    S.prism("blade", [(0.70, y - 0.03), (0.86, y - 0.03), (0.90, y - 0.17), (0.66, y - 0.15)], 0.0, 0.05,
            "steel", bevel=0.01)
    S.box("poll", 0.74, y - 0.03, 0.0, 0.84, y + 0.05, 0.06, "metal_dark", bevel=0.01)
    out.append(_tool_render("axe", 0.64, 0.22))

    handle(0.16, 0.80)
    S.prism("pick", [(0.78, y + 0.20), (0.84, y + 0.02), (0.84, y - 0.02), (0.78, y - 0.20),
                     (0.74, y - 0.04), (0.74, y + 0.04)], 0.0, 0.05, "steel", bevel=0.008)
    out.append(_tool_render("pickaxe", 0.62, 0.24))
    return out


def _body_sprite(facing, frame):
    """The skin layer: the body itself (no holdout), with a shadow."""
    return _layer("body", facing, _body, body_holdout=False, frame=frame)
