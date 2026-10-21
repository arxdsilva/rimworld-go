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


# ------------------------------------------------------------------ rig
# There is no armature: every vertex is moved by the chain of joints that
# owns it, in the rest pose (south-facing authoring, forward is -Y):
#   legs   pelvis -> hip (pitch h, spread x) -> knee (mesh bend k)
#   trunk  pelvis -> drop -> twist -> waist (lean, bend; blended over the
#          waist so the coat and hips bend instead of breaking)
#   arms   trunk -> shoulder (pitch a, abduction s, twist t) -> elbow e
#          -> wrist w (the hand rides the wrist rigidly)
#   head   trunk -> neck (yaw hy, pitch hp)
# Hair and hats are not posed; the renderer moves them by `head`.
HIP_Z, SHOULDER_Z = 0.33, 0.60
KNEE_Z = 0.18
ELBOW_Z = 0.465
WRIST_Z = 0.345
WAIST_Z = 0.34
NECK_Z = 0.68
SHOULDER_X = 0.19
LEG_X = 0.075
THIGH, SHIN = HIP_Z - KNEE_Z, KNEE_Z
ARM_PARTS = ("arm", "hand", "sleeve", "psleeve", "pcuff", "dsleeve")
LEG_PARTS = ("leg", "pleg")
HEAD_PARTS = ("head", "eye", "nose")

# Attachment points in the rest pose (Blender coords before the facing):
# the front of each fist (grip) and the head centre. `hand` rides the r
# (+X) arm, `hand_l` the l arm, through shoulder -> elbow -> wrist.
HAND = {  # where held equipment's grip goes (kept for older callers)
    "south": (CX + 0.195, CY - 0.06, 0.34),
}
HAND_R = HAND["south"]
HAND_L = (CX - 0.195, CY - 0.06, 0.34)
HEAD_CENTER = (CX, CY, HEAD_Z)

# Pose parameters (degrees unless noted; r = the +X arm/leg, which holds
# the main item and owns the `hand` attachment; l = the -X side, `hand_l`):
#   hr hl   hip pitch, + = thigh forward      kr kl  knee flex, + = shin back
#   xr xl   hip spread, + = leg out
#   ar al   shoulder pitch, - = arm forward/up (the walk/work convention)
#   sr sl   shoulder abduction, + = arm out to the side
#   tr tl   upper-arm twist, + = inward (a bent forearm crosses the body)
#   er el   elbow flex, + = forearm forward/up
#   wr wl   wrist flex, + = hand forward
#   lr ll   arm stretch (fraction; set by the solver when a target is out
#           of the short chibi reach, up to STRETCH_MAX)
#   drop    upper body down (tiles; legs stay)   sink  pelvis down (tiles)
#   shift   pelvis forward (tiles)               twist trunk yaw, + = r shoulder back
#   lean    trunk pitch, + = forward             bend  trunk side bend, + = toward r
#   hy hp   head yaw (+ = turn toward r) and pitch (+ = nod down)
# Solver inputs, resolved per key pose:
#   fr fl   foot target (forward, lift) tiles from the rest spot -> h, k
#   Rw Lw   hand target (x, forward, z) tiles from the feet -> a, s, t, e
#   Rp Lp   elbow pole (x, forward, z) in the trunk frame (default out-back-down)
KEYS = ("hr hl kr kl xr xl ar al sr sl tr tl er el wr wl lr ll "
        "drop sink shift twist lean bend hy hp").split()
STRETCH_MAX = 0.3
ZERO = {k: 0.0 for k in KEYS}

# Lean is pitch toward/away from a camera that already sees the figure
# leaned back 55 deg, so seen from behind a forward lean barely changes
# the silhouette. North fakes it: the trunk tips toward the viewer instead,
# which drops the head between the shoulders the way a back view of
# someone bending over reads.
NORTH_LEAN = -0.45


def _smooth(t):
    t = min(max(t, 0.0), 1.0)
    return t * t * (3.0 - 2.0 * t)


def _M():
    from mathutils import Matrix
    return Matrix


def _R(deg, axis):
    return _M().Rotation(math.radians(deg), 4, axis)


def _about(pivot, m):
    Matrix = _M()
    return Matrix.Translation(pivot) @ m @ Matrix.Translation((-pivot[0], -pivot[1], -pivot[2]))


def _rot_x(deg, pz):
    return _about((0, CY, pz), _R(deg, "X"))


def _pelvis(p):
    return _M().Translation((0, -p["shift"], -p["sink"]))


def _torso_matrix(p, w=1.0):
    """Pelvis, drop and twist (everything above the legs), plus the waist
    lean/bend blended in by w (0 below the waist, 1 above it)."""
    Matrix = _M()
    tw = _about((CX, CY, 0), _R(p["twist"], "Z"))
    m = _pelvis(p) @ Matrix.Translation((0, 0, -p["drop"])) @ tw
    if w > 0 and (p["lean"] or p["bend"]):
        m = m @ _about((CX, CY, WAIST_Z), _R(p["bend"] * w, "Y") @ _R(p["lean"] * w, "X"))
    return m


def _head_matrix(p):
    return _torso_matrix(p) @ _about((CX, CY, NECK_Z), _R(p["hy"], "Z") @ _R(p["hp"], "X"))


def _shoulder(p, side):
    k = "r" if side > 0 else "l"
    return (_R(p["a" + k], "X") @ _R(-side * p["s" + k], "Y") @ _R(-side * p["t" + k], "Z"))


def _arm_matrix(p, side, fe=1.0, fw=1.0, rigid=True):
    """Trunk -> shoulder -> elbow (fe of the flex) -> wrist (fw of it).
    A stretched arm scales its sleeve/arm vertices along the bone (rigid=
    False) or slides the rigid hand down to the stretched wrist."""
    Matrix = _M()
    k = "r" if side > 0 else "l"
    sx = CX + side * SHOULDER_X
    L = 1.0 + p["l" + k]
    ez = SHOULDER_Z + L * (ELBOW_Z - SHOULDER_Z)
    wz = SHOULDER_Z + L * (WRIST_Z - SHOULDER_Z)
    if rigid:
        st = Matrix.Translation((0, 0, wz - WRIST_Z))
    else:
        st = _about((sx, CY, SHOULDER_Z), Matrix.Diagonal((1.0, 1.0, L, 1.0)))
    sh = _about((sx, CY, SHOULDER_Z), _shoulder(p, side))
    el = _about((sx, CY, ez), _R(-p["e" + k] * fe, "X"))
    wr = _about((sx, CY, wz), _R(-p["w" + k] * fw, "X"))
    return _torso_matrix(p) @ sh @ el @ wr @ st


def _set_shoulder(p, side, rot):
    """Store a shoulder rotation (quaternion or matrix) as a, s, t."""
    k = "r" if side > 0 else "l"
    m = rot.to_matrix() if hasattr(rot, "to_matrix") else rot.to_3x3()
    eu = m.to_euler("ZYX")                                  # M = Rx(x) Ry(y) Rz(z)
    p["a" + k] = math.degrees(eu.x)
    p["s" + k] = -side * math.degrees(eu.y)
    p["t" + k] = -side * math.degrees(eu.z)


def _leg_matrix(p, side):
    k = "r" if side > 0 else "l"
    hip = (CX + side * LEG_X, CY, HIP_Z)
    return _pelvis(p) @ _about(hip, _R(-side * p["x" + k], "Y") @ _R(-p["h" + k], "X"))


# -------------------------------------------------------------- solvers
def _leg_ik(sink, shift, foot):
    """Hip pitch and knee flex (deg) that put the foot (forward, lift)
    tiles from its rest spot, the knee bending forward."""
    f, lift = foot
    dy, dz = f - shift, (HIP_Z - sink) - lift
    d = min(math.hypot(dy, dz), THIGH + SHIN - 1e-6)
    c = (d * d - THIGH ** 2 - SHIN ** 2) / (2 * THIGH * SHIN)
    k = math.acos(min(max(c, -1.0), 1.0))
    alpha = math.atan2(SHIN * math.sin(k), THIGH + SHIN * math.cos(k))
    return math.degrees(math.atan2(dy, dz) + alpha), math.degrees(k)


def _arm_ik(p, side, target, pole=None):
    """Shoulder (a, s, t), stretch and elbow e that put the hand's grip
    point on a target (x, forward, z tiles from the feet), elbow towards
    the pole. The chibi arm is short: past its reach the arm stretches
    (up to STRETCH_MAX), which reads as a smear at sprite scale."""
    from mathutils import Quaternion, Vector
    k = "r" if side > 0 else "l"
    sx = CX + side * SHOULDER_X
    S = Vector((sx, CY, SHOULDER_Z))
    grip = Vector(HAND_R if side > 0 else HAND_L)
    world = Vector((CX + target[0], CY - target[1], target[2]))
    T = _torso_matrix(p).inverted() @ world - S          # trunk frame, from the shoulder
    q0 = dict(ZERO)

    def chain(e, ln):
        q0["e" + k], q0["l" + k] = e, ln
        return _arm_matrix(q0, side) @ grip - S

    reach = chain(0.0, 0.0).length
    ln = min(max(T.length / (0.99 * reach) - 1.0, 0.0), STRETCH_MAX)
    lo, hi = 0.0, 150.0                                    # |chain| shrinks as e grows
    if chain(lo, ln).length <= T.length:
        e = lo
    elif chain(hi, ln).length >= T.length:
        e = hi
    else:
        for _ in range(40):
            mid = (lo + hi) / 2
            lo, hi = (mid, hi) if chain(mid, ln).length > T.length else (lo, mid)
        e = (lo + hi) / 2
    q = chain(e, ln).rotation_difference(T)
    axis = T.normalized()
    pole = Vector((side * 0.7, 0.45, -0.55)) if pole is None else Vector((pole[0] * side, -pole[1], pole[2]))
    e1 = q @ Vector((0, 0, ELBOW_Z - SHOULDER_Z))
    a1 = e1 - axis * e1.dot(axis)
    a2 = pole - axis * pole.dot(axis)
    if a1.length > 1e-6 and a2.length > 1e-6:
        ang = a1.angle(a2)
        if a1.cross(a2).dot(axis) < 0:
            ang = -ang
        q = Quaternion(axis, ang) @ q
    out = {"e" + k: e, "l" + k: ln}
    _set_shoulder(out, side, q)
    return out


def _final(pose):
    """Fill defaults and run the foot and hand solvers."""
    p = dict(ZERO)
    p.update(pose)
    for s in "rl":
        foot = p.pop("f" + s, None)
        if foot is not None:
            p["h" + s], p["k" + s] = _leg_ik(p["sink"], p["shift"], foot)
    for s, side in (("R", 1), ("L", -1)):
        tgt = p.pop(s + "w", None)
        pole = p.pop(s + "p", None)
        if tgt is not None:
            p.update(_arm_ik(p, side, tgt, pole))
    return p


def _tween(a, b, t):
    """Smoothstep in-between of two key poses: feet planted in both blend
    their targets, shoulders slerp, everything else blends solved angles."""
    e = _smooth(t)
    A, B = dict(a), dict(b)
    feet = {}
    for s in "rl":
        if "f" + s in A and "f" + s in B:
            fa, fb = A.pop("f" + s), B.pop("f" + s)
            feet["f" + s] = tuple(x + (y - x) * e for x, y in zip(fa, fb))
    A, B = _final(A), _final(B)
    out = {k: A[k] + (B[k] - A[k]) * e for k in KEYS}
    for side in (1, -1):                                   # shoulders slerp, never Euler-lerp
        qa = _shoulder(A, side).to_3x3().to_quaternion()
        qb = _shoulder(B, side).to_3x3().to_quaternion()
        _set_shoulder(out, side, qa.slerp(qb, e))
    for s in "rl":
        if "f" + s in feet:
            out["h" + s], out["k" + s] = _leg_ik(out["sink"], out["shift"], feet["f" + s])
    return out


def _P(*parts, **kw):
    """Merge pose fragments left to right."""
    out = {}
    for d in parts:
        out.update(d)
    out.update(kw)
    return out


# ------------------------------------------------------------- the walk
# Walk cycle (ART spec §27.1): 12 unique poses per facing, walk01..walk12.
# Frame 01 is the right heel contact; the right leg then loads, passes
# mid-stance, terminal stance, pre-swing and swings through; 07..12 are the
# same poses for the left foot. Per-frame curves:
#   hip     +-26 deg (cosine, forward at contact)
#   knee    0-40 deg, peaking in swing
#   pelvis  drops 2-3 px at loading (everything above the legs)
#   arms    counter-swing at 0.8x the hip
#   torso   twists +-4 deg against the pelvis
WALK_DEG = 26.0
WALK_N = 12
WALK_FRAMES = {"walk%02d" % i: i for i in range(1, WALK_N + 1)}
POSE_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility", "parka_leather", "duster_leather")
WALK_LAYERS = WORK_LAYERS = POSE_LAYERS
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
WORK_ARM = {"w_start": -142.0, "w_peak": -162.0, "strike": -38.0, "recover": -78.0}


def _walk(i):
    hr = WALK_DEG * math.cos(2 * math.pi * (i - 1) / WALK_N)
    return dict(hr=hr, hl=-hr, kr=KNEE_R[i - 1], kl=KNEE_R[(i - 1 + 6) % 12],
                ar=0.8 * hr, al=-0.8 * hr, drop=DROP_MAX * DROP_W[(i - 1) % 6],
                twist=4.0 * hr / WALK_DEG)


def _work(i):
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


# ---------------------------------------------------------- action poses
# ART spec §27.2. Named key poses; frames are keys or smoothstep
# in-betweens of two keys. Hand targets (Rw/Lw) are (x, forward, z) tiles
# from the feet. Two-handed items: `hand` (r) holds the grip and `hand_l`
# sits further along the item, towards the muzzle, blade or head.
# Strike frames (the renderer's impact tick): fire1, stab4, thrust4,
# swing4, punch2, punch3, chop4, mine4.
STAND = dict(fr=(0.0, 0.0), fl=(0.0, 0.0))
BRACE = dict(fl=(0.07, 0.0), fr=(-0.05, 0.0), sink=0.015)            # left foot leads
FENCE = dict(fr=(0.07, 0.0), fl=(-0.05, 0.0), sink=0.015)            # right foot leads
LUNGE_L = dict(fl=(0.17, 0.0), fr=(-0.11, 0.0), sink=0.05, shift=0.03)
LUNGE_R = dict(fr=(0.17, 0.0), fl=(-0.11, 0.0), sink=0.05, shift=0.03)
WIDE = dict(fl=(0.04, 0.0), fr=(-0.03, 0.0), xr=7.0, xl=7.0, sink=0.03)
SQUAT = dict(fl=(0.07, 0.0), fr=(-0.02, 0.0), xr=10.0, xl=10.0, sink=0.11)
KNEEL = dict(hr=-6.0, kr=94.0, fl=(0.11, 0.0), sink=0.15, shift=-0.06)  # right knee down; hips back
# (deep leans sit the hips back so the head stays on the frozen canvas)
KNEEL2 = dict(hr=-4.0, kr=95.0, hl=-4.0, kl=95.0, sink=0.15, shift=-0.08)  # both knees down

ELBOW_OUT = (1.0, 0.0, 0.1)
ELBOW_UP = (1.0, 0.3, 0.6)

READY = _P(BRACE, twist=12.0, lean=4.0, hp=4.0, Rw=(0.15, 0.08, 0.47), Lw=(-0.04, 0.18, 0.44))
AIM = _P(BRACE, twist=30.0, lean=8.0, hy=-24.0, hp=6.0, bend=-3.0,
         Rw=(0.07, 0.08, 0.56), Rp=ELBOW_OUT, Lw=(0.03, 0.25, 0.57))
KICK = _P(AIM, twist=36.0, lean=2.0, hp=0.0, Rw=(0.07, 0.03, 0.58), Lw=(0.03, 0.20, 0.62))
RISE = _P(AIM, twist=33.0, lean=4.0, hp=-3.0, Rw=(0.07, 0.05, 0.59), Lw=(0.03, 0.22, 0.66))

GUARD_K = _P(FENCE, twist=-8.0, lean=6.0, Rw=(0.12, 0.17, 0.48), Lw=(-0.06, 0.15, 0.56))
CHAMBER_K = _P(FENCE, sink=0.03, twist=12.0, Rw=(0.21, -0.03, 0.44), Lw=(-0.04, 0.17, 0.55))
EXTEND_K = _P(LUNGE_R, twist=-22.0, lean=14.0, Rw=(0.06, 0.42, 0.50), Lw=(-0.19, 0.0, 0.48))

SPEAR = _P(BRACE, twist=20.0, lean=4.0, Rw=(0.14, 0.02, 0.44), Lw=(-0.02, 0.20, 0.50))
DRAW_S = _P(BRACE, sink=0.03, twist=30.0, lean=-2.0, Rw=(0.17, -0.06, 0.45), Lw=(-0.04, 0.12, 0.50))
REACH_S = _P(LUNGE_L, twist=25.0, lean=15.0, Rw=(0.05, 0.22, 0.48), Lw=(-0.03, 0.42, 0.50))

CLUB = _P(BRACE, lean=4.0, Rw=(0.15, 0.14, 0.46), Lw=(-0.14, 0.10, 0.45))
COCK = _P(BRACE, twist=22.0, bend=-6.0, lean=-4.0, Rw=(0.20, -0.08, 0.84), Rp=ELBOW_UP,
          Lw=(-0.06, 0.22, 0.52))
SMASH = _P(BRACE, fl=(0.12, 0.0), fr=(-0.08, 0.0), sink=0.04, shift=0.02, twist=-22.0, lean=14.0,
           bend=4.0, Rw=(0.02, 0.36, 0.42), Lw=(-0.20, -0.02, 0.48))
FOLLOW = _P(SMASH, twist=-30.0, lean=18.0, Rw=(-0.16, 0.18, 0.32), Lw=(-0.22, -0.06, 0.46))

GUARD_P = _P(BRACE, sink=0.025, twist=10.0, lean=6.0, hp=6.0, Rw=(0.08, 0.14, 0.62), Lw=(-0.06, 0.20, 0.64))
JAB = _P(GUARD_P, twist=22.0, lean=10.0, Lw=(0.0, 0.42, 0.64))
CROSS = _P(GUARD_P, twist=-20.0, lean=12.0, shift=0.02, Rw=(0.0, 0.42, 0.64), Lw=(-0.08, 0.16, 0.64))

LOW2 = _P(WIDE, lean=6.0, Rw=(0.06, 0.18, 0.44), Lw=(0.0, 0.26, 0.42))
RAISE_AXE = _P(WIDE, bend=12.0, twist=10.0, lean=-6.0, hp=-4.0, Rw=(0.10, 0.10, 0.88), Rp=ELBOW_OUT,
               Lw=(0.0, 0.04, 0.96))
IMPACT_AXE = _P(WIDE, sink=0.05, bend=-10.0, twist=-20.0, lean=16.0, hp=10.0,
                Rw=(-0.02, 0.24, 0.40), Lw=(-0.10, 0.34, 0.36))
REBOUND_AXE = _tween(IMPACT_AXE, RAISE_AXE, 0.2)

RAISE_PICK = _P(WIDE, lean=-8.0, hp=-10.0, Rw=(0.05, 0.06, 0.96), Rp=ELBOW_UP,
                Lw=(-0.04, 0.02, 1.0), Lp=ELBOW_UP)
IMPACT_PICK = _P(WIDE, sink=0.08, lean=28.0, hp=15.0, Rw=(0.03, 0.26, 0.30), Lw=(-0.03, 0.34, 0.26))
PRY_PICK = _P(WIDE, sink=0.06, lean=14.0, hp=8.0, Rw=(0.06, 0.12, 0.40), Lw=(-0.02, 0.26, 0.34))

HAMMER_L = (-0.04, 0.28, 0.34)
HAMMER_UP = _P(WIDE, sink=0.04, lean=18.0, hp=14.0, Rw=(0.12, 0.16, 0.66), Rp=ELBOW_UP, Lw=HAMMER_L)
HAMMER_DOWN = _P(HAMMER_UP, Rw=(0.04, 0.28, 0.38), Rp=None)

KNEEL_REST = _P(KNEEL, lean=18.0, hp=10.0, Rw=(0.18, 0.06, 0.26), Lw=(-0.08, 0.10, 0.27))
DIG = _P(KNEEL, lean=46.0, hp=14.0, Rw=(0.10, 0.24, 0.06), Lw=(-0.08, 0.10, 0.24))
SCOOP = _P(DIG, Rw=(0.14, 0.14, 0.14))
SEED = _P(DIG, lean=42.0, Rw=(0.04, 0.26, 0.10), wr=35.0)
PRESS = _P(DIG, lean=48.0, Rw=(0.04, 0.24, 0.05), Lw=(-0.06, 0.24, 0.06))

PICK_REACH = _P(SQUAT, lean=32.0, hp=15.0, Rw=(0.06, 0.34, 0.20), Lw=(-0.06, 0.34, 0.22))
PICK_GRAB = _P(PICK_REACH, Rw=(0.03, 0.32, 0.18), Lw=(-0.03, 0.32, 0.19), wr=30.0, wl=30.0)
PICK_PULL = _P(SQUAT, sink=0.07, lean=18.0, hp=8.0, Rw=(0.04, 0.20, 0.42), Lw=(-0.04, 0.22, 0.44))
PICK_BAG = _P(SQUAT, sink=0.06, twist=20.0, lean=14.0, Rw=(0.24, -0.02, 0.36), Lw=(-0.08, 0.16, 0.40))

BENCH = _P(STAND, sink=0.01, lean=10.0, hp=18.0)
TEND = _P(KNEEL, lean=36.0, hp=20.0)
EAT = _P(STAND, lean=4.0, hp=10.0, Lw=(-0.04, 0.20, 0.52))
EAT_BOWL = _P(EAT, Rw=(0.02, 0.22, 0.56))
EAT_MOUTH = _P(EAT, hp=2.0, Rw=(0.02, 0.22, 0.73), Rp=ELBOW_OUT)

CARRY = _P(STAND, lean=-3.0, Rw=(0.16, 0.20, 0.46), Lw=(-0.16, 0.20, 0.46))
CARRY_ARMS = ("ar", "sr", "tr", "er", "lr", "al", "sl", "tl", "el", "ll")

SNAP = _P(STAND, sink=0.02, shift=-0.02, lean=-16.0, twist=12.0, hy=28.0, hp=-12.0,
          Rw=(0.22, 0.14, 0.74), Rp=ELBOW_OUT, Lw=(-0.18, 0.16, 0.76), Lp=ELBOW_OUT)
BUCKLE = _P(STAND, sink=0.08, lean=10.0, hp=10.0, Rw=(0.30, 0.10, 0.48), Lw=(-0.30, 0.10, 0.48))
SLUMP = _P(STAND, sink=0.14, lean=24.0, hp=12.0, Rw=(0.26, 0.24, 0.36), Lw=(-0.26, 0.24, 0.36))
FOLD = _P(KNEEL2, lean=40.0, hp=16.0, Rw=(0.18, 0.28, 0.18), Lw=(-0.18, 0.28, 0.18))
DOWN = _P(KNEEL2, lean=55.0, hp=16.0, Rw=(0.16, 0.30, 0.06), Lw=(-0.16, 0.30, 0.06))

CROUCH = dict(fl=(0.10, 0.0), fr=(-0.04, 0.0), sink=0.09)
OFFER_IN = _P(CROUCH, lean=22.0, hp=10.0, Rw=(0.10, 0.26, 0.30), Lw=(-0.08, 0.14, 0.24))
OFFER_OUT = _P(OFFER_IN, Rw=(0.08, 0.38, 0.24), wr=-20.0)

T = _tween
ACTION = {
    "ready": READY,
    "aim": AIM,
    "fire1": KICK, "fire2": RISE, "fire3": T(RISE, AIM, 0.6),
    "stab1": GUARD_K, "stab2": CHAMBER_K, "stab3": T(CHAMBER_K, EXTEND_K, 0.5), "stab4": EXTEND_K,
    "stab5": T(EXTEND_K, GUARD_K, 0.5), "stab6": T(EXTEND_K, GUARD_K, 0.9),
    "thrust1": SPEAR, "thrust2": DRAW_S, "thrust3": T(DRAW_S, REACH_S, 0.5), "thrust4": REACH_S,
    "thrust5": T(REACH_S, SPEAR, 0.5), "thrust6": T(REACH_S, SPEAR, 0.9),
    "swing1": T(CLUB, COCK, 0.55), "swing2": COCK, "swing3": T(COCK, SMASH, 0.45), "swing4": SMASH,
    "swing5": FOLLOW, "swing6": T(FOLLOW, CLUB, 0.6),
    "punch1": GUARD_P, "punch2": JAB, "punch3": CROSS, "punch4": T(CROSS, GUARD_P, 0.85),
    "chop1": T(LOW2, RAISE_AXE, 0.5), "chop2": RAISE_AXE, "chop3": T(RAISE_AXE, IMPACT_AXE, 0.5),
    "chop4": IMPACT_AXE, "chop5": REBOUND_AXE, "chop6": T(IMPACT_AXE, LOW2, 0.6),
    "mine1": T(PRY_PICK, RAISE_PICK, 0.55), "mine2": RAISE_PICK, "mine3": T(RAISE_PICK, IMPACT_PICK, 0.5),
    "mine4": IMPACT_PICK, "mine5": PRY_PICK, "mine6": T(PRY_PICK, RAISE_PICK, 0.25),
    "hammer1": T(HAMMER_DOWN, HAMMER_UP, 0.6), "hammer2": HAMMER_UP, "hammer3": HAMMER_DOWN,
    "hammer4": T(HAMMER_DOWN, HAMMER_UP, 0.25),
    "kneel": KNEEL_REST, "plant1": DIG, "plant2": SCOOP, "plant3": SEED, "plant4": PRESS,
    "pick1": PICK_REACH, "pick2": PICK_GRAB, "pick3": PICK_PULL, "pick4": PICK_BAG,
    "bench1": _P(BENCH, Rw=(0.10, 0.24, 0.42), Lw=(-0.10, 0.24, 0.42)),
    "bench2": _P(BENCH, Rw=(0.04, 0.30, 0.44), Lw=(-0.12, 0.20, 0.42)),
    "bench3": _P(BENCH, Rw=(0.12, 0.22, 0.46), Lw=(-0.06, 0.28, 0.42)),
    "bench4": _P(BENCH, Rw=(0.08, 0.26, 0.40), Lw=(-0.08, 0.26, 0.44)),
    "tend1": _P(TEND, Rw=(0.06, 0.25, 0.14), Lw=(-0.06, 0.25, 0.14)),
    "tend2": _P(TEND, Rw=(0.10, 0.27, 0.22), Lw=(-0.04, 0.23, 0.12)),
    "tend3": _P(TEND, Rw=(0.0, 0.25, 0.12), Lw=(-0.10, 0.27, 0.20)),
    "tend4": _P(TEND, Rw=(0.04, 0.29, 0.12), Lw=(-0.04, 0.21, 0.12)),
    "eat1": EAT_BOWL, "eat2": T(EAT_BOWL, EAT_MOUTH, 0.5), "eat3": EAT_MOUTH,
    "eat4": _P(T(EAT_MOUTH, EAT_BOWL, 0.6), hp=14.0),
    "carry": CARRY,
    "flinch1": T(STAND, SNAP, 0.7), "flinch2": SNAP, "flinch3": T(SNAP, STAND, 0.55),
    "fall1": BUCKLE, "fall2": SLUMP, "fall3": FOLD, "fall4": DOWN,
    "offer1": OFFER_IN, "offer2": OFFER_OUT,
}
CARRY_WALK = {"carry_walk%02d" % i: i for i in range(1, WALK_N + 1)}
ACTION_FRAMES = list(ACTION) + list(CARRY_WALK)
FRAMES = list(WALK_FRAMES) + list(WORK_FRAMES) + ACTION_FRAMES


def _params(frame, facing=None):
    """Joint angles for a frame (rest when None)."""
    if frame in WALK_FRAMES:
        p = _final(_walk(WALK_FRAMES[frame]))
    elif frame in WORK_FRAMES:
        p = _final(_work(WORK_FRAMES[frame]))
    elif frame in CARRY_WALK:
        c = _final(CARRY)
        p = _final(_walk(CARRY_WALK[frame]))
        p["twist"] *= 0.5
        for k in CARRY_ARMS:
            p[k] = c[k]
        p["lean"] = c["lean"]
    elif frame in ACTION:
        p = _final(ACTION[frame])
    else:
        p = dict(ZERO)
    if facing == "north":
        p["lean"] *= NORTH_LEAN
    return p


# ------------------------------------------------------------- skinning
def _mean_x(ob):
    vs = ob.data.vertices
    return sum((ob.matrix_world @ v.co).x for v in vs) / max(len(vs), 1)


def _bake(ob):
    from mathutils import Matrix
    ob.data.transform(ob.matrix_world)
    ob.matrix_world = Matrix.Identity(4)


def _bisect(bm, zs):
    for z in zs:
        bmesh.ops.bisect_plane(bm, geom=list(bm.verts) + list(bm.edges) + list(bm.faces),
                               plane_co=(0, 0, z), plane_no=(0, 0, 1), dist=1e-5)


def _bend_knee(ob, flex):
    """Bend a leg mesh at the knee: cut loops around KNEE_Z, then rotate the
    shank (feet go backwards) with a smooth ramp over the cut loops."""
    _bake(ob)
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    _bisect(bm, (KNEE_Z + 0.03, KNEE_Z, KNEE_Z - 0.03))
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


def _skin(ob, cuts, weights, matrix):
    """Move each vertex by matrix(*weights(z)), after cutting loops at the
    given heights so joints bend smoothly."""
    _bake(ob)
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    _bisect(bm, cuts)
    cache = {}
    for v in bm.verts:
        w = tuple(round(x, 3) for x in weights(v.co.z))
        m = cache.get(w)
        if m is None:
            m = cache[w] = matrix(*w)
        v.co = m @ v.co
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()


def _ramp(z, top, width):
    return min(max((top - z) / width, 0.0), 1.0)


ELBOW_CUTS = tuple(ELBOW_Z + d for d in (0.03, 0.015, 0.0, -0.015, -0.03))
WRIST_CUTS = tuple(WRIST_Z + d for d in (0.015, 0.0, -0.015))
WAIST_CUTS = tuple(WAIST_Z + d for d in (-0.04, -0.02, 0.0, 0.02, 0.04))


def _pose(frame, facing=None):
    """Pose a frame (forward is -Y). Applied to every object (holdout body
    included) before the facing."""
    p = _params(frame, facing)
    torso = _torso_matrix(p)
    head = _head_matrix(p)
    bends = p["lean"] or p["bend"]
    for ob in S.objects():
        n = ob.name.split(".")[0]
        side = 1 if _mean_x(ob) >= CX else -1
        k = "r" if side > 0 else "l"
        if n in LEG_PARTS:
            if p["k" + k] > 0:
                _bend_knee(ob, p["k" + k])
            ob.matrix_world = _leg_matrix(p, side) @ ob.matrix_world
        elif n in ARM_PARTS:
            if n == "hand" or not (p["e" + k] or p["w" + k] or p["l" + k]):
                ob.matrix_world = _arm_matrix(p, side) @ ob.matrix_world
            else:
                _skin(ob, ELBOW_CUTS + WRIST_CUTS,
                      lambda z: (_ramp(z, ELBOW_Z + 0.03, 0.06), _ramp(z, WRIST_Z + 0.015, 0.03)),
                      lambda fe, fw, s=side: _arm_matrix(p, s, fe, fw, rigid=False))
        elif n in HEAD_PARTS:
            ob.matrix_world = head @ ob.matrix_world
        elif bends:
            _skin(ob, WAIST_CUTS, lambda z: (1.0 - _ramp(z, WAIST_Z + 0.04, 0.08),),
                  lambda w: _torso_matrix(p, w))
        else:
            ob.matrix_world = torso @ ob.matrix_world


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




def _attach(facing, frame, point):
    """Project a rest-pose body point through its joint chain, the facing
    and the figure lean (Blender coords; _project turns them into px)."""
    from mathutils import Matrix, Vector
    deg = S.FACINGS[facing]
    m = (Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-0.5, 0.5, 0)))
    p = _params(frame, facing)
    return _lean_matrix() @ m @ point_matrix(p, point) @ Vector(point[0])


def point_matrix(p, point):
    kind = point[1]
    if kind in ("arm", "arm_r"):
        return _arm_matrix(p, 1)
    if kind == "arm_l":
        return _arm_matrix(p, -1)
    if kind == "head":
        return _head_matrix(p)
    return _torso_matrix(p)


def _layer(name, facing, builder, body_holdout=True, frame=None):
    if body_holdout:
        objs = _body()
        S.holdout(objs)
    builder()
    _apply_modifiers()
    if frame:
        _pose(frame, facing)
    _figure(facing)
    zm = _zmax_screen()
    top = int(-(-((C.SHEAR_K * zm + MARGINS[2]) * C.PPT) // 1))
    origin = (int(round(MARGINS[0] * C.PPT)), top)
    att = {}
    for key, point in (("hand", (HAND_R, "arm_r")), ("hand_l", (HAND_L, "arm_l"))):
        h = _attach(facing, frame, point)
        att[key] = _project(h.x, h.y, h.z, origin)
    if name == "body":
        hd = _attach(facing, frame, (HEAD_CENTER, "head"))
        att["head"] = _project(hd.x, hd.y, hd.z, origin)
    sprite = "%s_%s" % (name, facing) + ("_" + frame if frame else "")
    extra = {"layer": name, "facing": facing, "attachments": att}
    if frame:
        extra["frame"] = frame
    return S.render_asset("pawn", sprite, zmax=zm, margins=MARGINS, extra=extra)


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


def build(walk_only=False, work_only=False, frames=None, facings=None, layers=None):
    """Every layer x facing x frame. frames/facings/layers narrow a
    review build (e.g. frames=["aim"], layers=["body", "shirt_basic"])."""
    out = []
    todo = [None] + FRAMES
    if walk_only:
        todo = list(WALK_FRAMES)
    if work_only:
        todo = list(WORK_FRAMES)
    if frames is not None:
        todo = list(frames)
    for f in facings or FACINGS:
        for name in ["body"] + list(LAYERS):
            if layers is not None and name not in layers:
                continue
            for fr in todo:
                if fr is not None and name not in POSE_LAYERS:
                    continue
                if name == "body":
                    out.append(_layer("body", f, _body, body_holdout=False, frame=fr))
                else:
                    out.append(_layer(name, f, LAYERS[name], frame=fr))
    return out
