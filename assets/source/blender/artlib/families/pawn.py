"""Compositional pawns (ART spec §10) and the first weapon (§15).

Layers share one canvas/origin per facing and are drawn in this order:
    body (skin) -> pants -> shirt -> vest -> hair -> held equipment
Every non-body layer is rendered with the body as a holdout, so hidden
surfaces are already removed and any subset of layers composites
correctly. Neutral greys + tint masks give skin/hair/cloth variety.

Original proportions: compact geometric torso, short legs, large head.
"""
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


def _cut(ob, keep):
    """Delete vertices of a mesh for which keep(x, y, z) is False."""
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    kill = [v for v in bm.verts if not keep(v.co.x, v.co.y, v.co.z)]
    bmesh.ops.delete(bm, geom=kill, context="VERTS")
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()


def _hair_shell(name, grow, keep):
    ob = S.ellipsoid(name, CX, CY, HEAD_Z, 0.19 + grow, 0.175 + grow, 0.19 + grow, "hair", segs=28, rings=16)
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


# Walk cycle (ART spec §27): 8 presentation frames per facing,
#   neutral, w1, w2, w1, neutral, w3, w4, w3
# where w1/w2 swing the left leg forward by WALK_DEG/2 and WALK_DEG (right
# arm forward with it), and w3/w4 mirror that. Only layers that cover
# limbs get frames; hair rides the head (bobbed in the renderer).
WALK_DEG = 26.0
WALK_FRAMES = {"walk1": 0.5, "walk2": 1.0, "walk3": -0.5, "walk4": -1.0}
WALK_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility")
HIP_Z, SHOULDER_Z = 0.33, 0.60

# Work swing (ART spec §27): arm poses for hands-on work (hammering,
# chopping, digging), as (right arm, left arm) shoulder rotations in
# degrees; negative swings the hand forward (towards the facing) and up.
#   work1: wind-up, tool raised overhead
#   work2: mid-swing
#   work3: strike, arm forward and low at the worked tile
# Both hands grip the tool (a two-handed swing reads in every facing);
# the "hand" attachment is the tool hand. Only layers
# that cover the arms get these frames.
WORK_FRAMES = {"work1": (-155.0, -148.0), "work2": (-95.0, -90.0), "work3": (-38.0, -34.0)}
WORK_LAYERS = ("body", "shirt_basic")


def _mean_x(ob):
    vs = ob.data.vertices
    return sum((ob.matrix_world @ v.co).x for v in vs) / max(len(vs), 1)


def _rot_x(deg, pz):
    import math
    from mathutils import Matrix
    return (Matrix.Translation((0, CY, pz)) @ Matrix.Rotation(math.radians(deg), 4, "X")
            @ Matrix.Translation((0, -CY, -pz)))


def _pose_work(frame):
    """Arms for a work frame (legs stay planted)."""
    rdeg, ldeg = WORK_FRAMES[frame]
    for ob in S.objects():
        if ob.name.split(".")[0] in ("arm", "hand", "sleeve"):
            deg = rdeg if _mean_x(ob) >= CX else ldeg
            ob.matrix_world = _rot_x(deg, SHOULDER_Z) @ ob.matrix_world


def _pose(swing):
    """Swing limbs for a walk frame: swing in [-1, 1]. Positive moves the
    left (-X) leg forward (towards -Y, the facing) and the right arm with
    it. Applied to every object (holdout body included) before facing."""
    if swing == 0:
        return
    a = WALK_DEG * swing
    for ob in S.objects():
        n = ob.name.split(".")[0]
        side = -1 if _mean_x(ob) < CX else 1
        if n in ("leg", "pleg"):
            # left leg (side -1) forward: rotate so the foot moves to -Y
            ob.matrix_world = _rot_x(a * side, HIP_Z) @ ob.matrix_world
        elif n in ("arm", "hand", "sleeve"):
            ob.matrix_world = _rot_x(-a * side * 0.8, SHOULDER_Z) @ ob.matrix_world


def _arm_angle(swing):
    """Right-arm (hand attachment side) swing in degrees for a frame."""
    return -WALK_DEG * swing * 0.8


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


def _hand(facing, swing=0.0, arm=None):
    import math
    from mathutils import Matrix, Vector
    deg = S.FACINGS[facing]
    m = (Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-0.5, 0.5, 0)))
    a = _arm_angle(swing) if arm is None else arm
    return _lean_matrix() @ m @ _rot_x(a, SHOULDER_Z) @ Vector(HAND["south"])


def _layer(name, facing, builder, body_holdout=True, frame=None):
    if body_holdout:
        objs = _body()
        S.holdout(objs)
    builder()
    _apply_modifiers()
    swing = WALK_FRAMES.get(frame, 0.0)
    arm = None
    if frame in WORK_FRAMES:
        _pose_work(frame)
        arm = WORK_FRAMES[frame][0]
    else:
        _pose(swing)
    _figure(facing)
    zm = _zmax_screen()
    top = int(-(-((C.SHEAR_K * zm + MARGINS[2]) * C.PPT) // 1))
    origin = (int(round(MARGINS[0] * C.PPT)), top)
    h = _hand(facing, swing, arm)
    sprite = "%s_%s" % (name, facing) + ("_" + frame if frame else "")
    extra = {"layer": name, "facing": facing, "attachments": {"hand": _project(h.x, h.y, h.z, origin)}}
    if frame:
        extra["frame"] = frame
    return S.render_asset("pawn", sprite, zmax=zm, margins=MARGINS, shadow=(name == "body"), extra=extra)


LAYERS = {
    "pants_basic": _pants,
    "shirt_basic": _shirt,
    "vest_utility": _vest,
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
