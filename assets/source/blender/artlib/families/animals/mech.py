"""Mechanoid raiders (wave E, DEF-4): ORIGINAL machine designs of our own,
not RimWorld's mechanoids. One archetype ("mech"), one spec per roster key;
the binding briefs are the roster rows' features.

- harrow  (blade unit):  low lean hound-like chassis of overlapping
                         gunmetal plates on four digitigrade legs, a narrow
                         wedge sensor head with one amber slit, two long
                         hooked blades folded forward under the chest, a
                         short stabiliser spine at the rear. Attack `slash`.
- spindle (sniper):      a small bone-white drum on three tall stilt legs
                         (knees high), one big round cyan lens, a long twin
                         rail barrel along its back reaching far past the
                         front. Attack `shoot` (charge, then a cyan-white
                         muzzle flash and recoil).
- bulwark (heavy gunner, XL): a squat, very wide soot-black hull under a
                         low dome of overlapping olive slab plates, six
                         short thick legs, a six-barrel rotary gun in a
                         front turret ring, a row of red sensor lights.
                         Attack `shoot` (the barrels spin, flash).
- quill   (skirmisher):  a small boxy sand-brown body with rust-orange
                         stripes on four long thin spider legs, a raised
                         back launcher of eight needle tubes pointing
                         forward, one sensor stalk with a green lens.
                         Attack `shoot`.

Mechs are hard-surfaced: flat-shaded beveled boxes and convex plate hulls
over dark joints (mech_* materials, palettes/materials_wave_e.yaml). Legs
are the rig's two-bone IK chains (rig.Leg), so the rig's walk, gait and
canvases apply unchanged. Wrecks are the corpses (item/corpse_<key>_v0..v2):
lens dark, legs splayed and some snapped, lying slumped (v0), on a side
(v1) or upturned (v2), on a dark oil pool instead of blood (Body.pool).
"""
import math
from dataclasses import dataclass

import bmesh
from mathutils import Matrix, Vector

from .. import animal_roster as R
from ... import stage as S
from . import parts as P
from . import rig
from .bird import basis, lerp, render
from .rig import Body, Leg, Pose, Skel, attach, ik2

ARCHETYPES = ("mech",)

JOINT = "mech_joint"
OFF = JOINT                       # a wreck's dead lens
WRECK_TILT = {0: 10.0, 1: 78.0, 2: 166.0}   # degrees about the long axis: slumped, on a side, upturned
TILT_OVERRIDE = {"bulwark": {1: 16.0}}       # too wide and heavy to lie on a side: listing instead
WRECK_SPIN = {0: 12.0, 1: -18.0, 2: 168.0}  # heading on the ground: near east-west, so long hulls stay in visPad
BROKEN = {0: (), 1: (1,), 2: (0, -1)}       # leg indices snapped at the knee per wreck variant


@dataclass
class MechPose(Pose):
    arm: float = 0.0      # harrow blades: -1 cocked high .. 0 folded forward .. 1 swept forward and down
    aim: float = 0.0      # gun elevation, degrees (+ up)
    fire: float = 0.0     # muzzle flash 0..1
    spin: float = 0.0     # rotary barrels, degrees
    charge: float = 0.0   # rail coils lit 0..1


def clamp(v, lo, hi):
    return max(lo, min(hi, v))


# ---------------------------------------------------------------- shapes
def obox(name, a, b, w, h, mat, up=(0, 0, 1), bevel=0.006):
    """A flat-shaded box along the segment a->b: half width w (side), half
    height h (towards `up`)."""
    a, b = Vector(a), Vector(b)
    d = b - a
    L = max(d.length, 1e-4)
    ob = S.box(name, -w, -L / 2, -h, w, L / 2, h, mat, bevel=bevel)
    ob.matrix_world = Matrix.Translation((a + b) / 2) @ basis(d if d.length > 1e-6 else (0, -1, 0), up) @ ob.matrix_world
    return ob


def ocyl(name, a, b, r, mat, segs=12, r2=None, bevel=0.003):
    """A cylinder (or cone frustum to r2) along a->b."""
    a, b = Vector(a), Vector(b)
    d = b - a
    ob = S.cyl(name, 0, 0, r, 0, max(d.length, 1e-4), mat, segs=segs, r_top=r2, bevel=bevel)
    ob.matrix_world = (Matrix.Translation(a) @ basis(d if d.length > 1e-6 else (0, -1, 0))
                       @ Matrix.Rotation(-math.pi / 2, 4, "X") @ ob.matrix_world)
    return ob


def hull(name, pts, mat, bevel=0.005):
    """A convex armour plate through pts (flat shaded, coplanar faces merged)."""
    bm = bmesh.new()
    vs = [bm.verts.new(Vector(p)) for p in pts]
    res = bmesh.ops.convex_hull(bm, input=vs, use_existing_faces=False)
    junk = [g for g in res["geom_interior"] + res["geom_unused"] if isinstance(g, bmesh.types.BMVert)]
    if junk:
        bmesh.ops.delete(bm, geom=junk, context="VERTS")
    bmesh.ops.dissolve_limit(bm, angle_limit=math.radians(1.0), verts=bm.verts[:], edges=bm.edges[:])
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    S._bevel(bm, bevel)
    return S._obj(name, bm, mat)


def slab(name, y0, y1, z0, z1, wb, wt, mat, inset=0.0, dz=(0.0, 0.0), x=0.0, bevel=0.006):
    """A plate along Y: bottom face y0..y1 (half widths wb, or (at y0, at
    y1)), top face inset in y with half widths wt, its height z1 shifted by
    dz at the y0 / y1 ends (slopes)."""
    wb0, wb1 = wb if isinstance(wb, tuple) else (wb, wb)
    wt0, wt1 = wt if isinstance(wt, tuple) else (wt, wt)
    pts = []
    for s in (-1, 1):
        pts += [(x + s * wb0, y0, z0), (x + s * wb1, y1, z0),
                (x + s * wt0, y0 + inset, z1 + dz[0]), (x + s * wt1, y1 - inset, z1 + dz[1])]
    return hull(name, pts, mat, bevel)


def lens(name, c, axis, r, mat, depth=0.35):
    """A round lens facing `axis` (flattened ellipsoid)."""
    from .bird import oell
    return oell(name, Vector(c), axis, (r, r * depth, r), mat, segs=14, rings=8)


def flash(c, axis, k, scale):
    """A muzzle flash at c along `axis` (k 0..1): a long core and a cross burst."""
    if k <= 0:
        return
    from .bird import oell
    c, d = Vector(c), Vector(axis).normalized()
    oell("flash", c + d * 0.06 * scale * k, d, (0.045 * scale * k, 0.11 * scale * k, 0.045 * scale * k), "mech_flash",
         segs=10, rings=6)
    oell("flash_x", c + d * 0.02 * scale, d, (0.09 * scale * k, 0.025 * scale * k, 0.03 * scale * k), "mech_flash",
         segs=10, rings=6)


def pivot(sk, piv, deg, axis="X"):
    """A body-frame rotation of `deg` about `axis` through body point piv."""
    piv = Vector(piv)
    return sk.B @ (Matrix.Translation(piv) @ Matrix.Rotation(math.radians(deg), 4, axis) @ Matrix.Translation(-piv))


# ------------------------------------------------------------------ legs
def mk_leg(name, side, root, joint, l1, l2, bend, foot, r, mat, kind, phase, front):
    return Leg(name, root, joint, l1, l2, bend, foot, r, mat, JOINT, kind, JOINT, phase, front)


def wreck_foot(sk, leg, i):
    """Splayed leg target on a wreck (body frame -> model space)."""
    j = sk.joint_world(leg)
    out = Vector((leg.foot[0], leg.foot[1], 0.0)) - Vector((j.x * 0.3, j.y * 0.6, 0.0))
    if out.length < 1e-4:
        out = Vector((0, 1, 0))
    out.normalize()
    reach = (leg.l1 + leg.l2) * (0.78 if i % 2 else 0.9)
    return j + out * reach * 0.85 + Vector((0, 0, -reach * (0.25 + 0.1 * sk.p.dead)))


def mech_leg(sk, leg, i, n):
    p = sk.p
    dead = p.dead >= 0
    root = sk.B @ Vector(leg.root)
    j = sk.joint_world(leg)
    tgt = wreck_foot(sk, leg, i) if dead else sk.foot_target(leg)
    knee, foot = ik2(j, tgt, leg.l1, leg.l2, leg.bend)
    r0, r1, r2, r3 = leg.r
    up = Vector(leg.bend)
    P.ell("hip", root, (r0, r0, r0), JOINT, segs=10, rings=6)
    if (j - root).length > 1e-3:
        obox("coxa", root, j, r1 * 0.7, r1 * 0.7, JOINT, up)
    P.ell("hipj", j, (r1 * 1.15,) * 3, JOINT, segs=10, rings=6)
    obox("femur", j, knee, r1 * 0.85, r1 * 1.3, leg.mat, up, bevel=r1 * 0.3)
    P.ell("knee", knee, (r2 * 1.3,) * 3, JOINT, segs=10, rings=6)
    snapped = dead and any(i == (b % n) for b in BROKEN[p.dead])
    if snapped:   # snapped below the knee: a short torn stub
        d = (foot - knee).normalized()
        P.cone("stub", knee, knee + d * leg.l2 * 0.35, r2 * 0.9, JOINT, segs=6, cap=0.3)
        return
    obox("tibia", knee, foot, r2 * 0.75, r2, leg.lower or leg.mat, up, bevel=r2 * 0.3)
    d = (foot - knee).normalized()
    if leg.foot_kind == "pad":       # wide flat pad square to the shin
        ocyl("pad", foot - d * r3 * 0.2, foot + d * r3 * 0.9, r3 * 1.9, JOINT, segs=12, r2=r3 * 2.1)
    elif leg.foot_kind == "claw":    # a pointed toe forward and a spur behind
        P.cone("toe", foot, foot + Vector((0, -1, -0.2)).normalized() * r3 * 3.2, r3 * 0.9, JOINT, segs=6)
        P.cone("spur", foot, foot + Vector((0, 0.8, -0.1)).normalized() * r3 * 1.8, r3 * 0.7, JOINT, segs=6)
        P.ell("ankle", foot, (r3,) * 3, JOINT, segs=8, rings=5)
    else:                            # spike: the shin ends in a point
        P.cone("tip", foot - d * r3, foot + d * r3 * 1.6, r3 * 0.9, JOINT, segs=6, cap=0.4)


def build_legs(sk):
    n = len(sk.legs)
    for i, leg in enumerate(sk.legs):
        mech_leg(sk, leg, i, n)


def wreck(p, key=""):
    """Cancel the rig's corpse roll and spin (rig._corpse applies spin @ roll
    after the model) and lie the wreck at its own tilt and heading."""
    roll, spin = rig.CORPSE_ROLL[p.dead]
    S.transform(S.objects(), Matrix.Rotation(math.radians(-roll), 4, "Y")
                @ Matrix.Rotation(math.radians(WRECK_SPIN[p.dead] - spin), 4, "Z")
                @ Matrix.Rotation(math.radians(TILT_OVERRIDE.get(key, {}).get(p.dead, WRECK_TILT[p.dead])), 4, "Y"))


def walk_set(spec, extra=None):
    """The rig's walk frames as MechPoses (no head nod or tail), plus extra(phase)."""
    out = {}
    for i in range(1, rig.TROT_N + 1):
        ph = (i - 1) / rig.TROT_N
        b = rig.trot_pose(spec, i)
        kw = {**b.__dict__, "head": 0.0, "neck": 0.0, "tail": 0.0, "sway": 0.0}
        kw.update(extra(ph) if extra else {})
        out["walk%02d" % i] = MechPose(**kw)
    return out


# ================================================================ harrow
H_Z0 = 0.30


def hz(z):
    return z - H_Z0


def harrow_legs(b):
    out = []
    for n, y, fy, by, ph in (("fl", -0.20, -0.25, 1.0, 0.0), ("fr", -0.20, -0.25, 1.0, 0.5),
                             ("hl", 0.21, 0.25, -1.0, 0.5), ("hr", 0.21, 0.25, -1.0, 0.0)):
        sx = 1 if n[1] == "l" else -1
        out.append(mk_leg(n, sx, (sx * 0.06, y, hz(0.36)), (sx * 0.1, y, hz(0.33)), 0.19, 0.2, (sx * 0.25, by, 0.25),
                          (sx * 0.17, fy), (0.042, 0.034, 0.027, 0.02), "mech_gunmetal", "claw", ph, n[0] == "f"))
    return out


def harrow_arm(sx, a, dead):
    s = Vector((sx * 0.085, -0.22, hz(0.31)))
    a = clamp(a, -1.0, 1.0)
    fold, cock, strike = Vector((sx * 0.25, -0.55, -0.8)), Vector((sx * 0.35, -0.25, 0.9)), Vector((sx * 0.15, -1.0, -0.15))
    d1 = (fold.lerp(strike, a) if a >= 0 else fold.lerp(cock, -a)).normalized()
    bf, bc, bs = Vector((-sx * 0.06, -1.0, 0.18)), Vector((sx * 0.05, -0.45, 0.9)), Vector((-sx * 0.05, -0.8, -0.6))
    d2 = (bf.lerp(bs, a) if a >= 0 else bf.lerp(bc, -a)).normalized()
    if dead:
        d1, d2 = Vector((sx * 0.9, -0.3, -0.2)).normalized(), Vector((sx * 0.5, -0.8, 0.1)).normalized()
    e = s + d1 * 0.15
    P.ell("shoulder", s, (0.03, 0.03, 0.03), JOINT, segs=10, rings=6)
    obox("upper_arm", s, e, 0.018, 0.026, "mech_gunmetal", (sx, 0, 0))
    P.ell("elbow", e, (0.024, 0.024, 0.024), JOINT, segs=10, rings=6)
    obox("blade_root", e, e + d2 * 0.07, 0.016, 0.022, "mech_gunmetal", (sx, 0, 0))
    hook = Vector((0, 0, -1)) - d2 * d2.z
    hook = hook.normalized() if hook.length > 1e-4 else Vector((0, 1, 0))
    L = 0.3
    pts = P.bezier(e + d2 * 0.05, e + d2 * L * 0.6, e + d2 * L * 0.92 + hook * 0.075, 9)
    n = len(pts)
    P.tube("blade", pts, [(0.0055, lerp(0.026, 0.003, (k / (n - 1)) ** 0.8)) for k in range(n)], "steel", segs=8,
           caps=(0.3, 0.0), side=(1, 0, 0), smooth=False)


def harrow_model(spec, p):
    sk = Skel(spec, p)
    dead = p.dead >= 0
    with attach(sk.B):
        obox("spine", (0, 0.30, hz(0.34)), (0, -0.30, hz(0.36)), 0.06, 0.04, JOINT)
        slab("belly", -0.27, 0.27, hz(0.26), hz(0.34), 0.06, 0.085, JOINT)
        slab("plate_rear", 0.06, 0.32, hz(0.33), hz(0.43), (0.10, 0.085), (0.07, 0.055), "mech_gunmetal", inset=0.02,
             dz=(0.0, -0.025))
        slab("plate_mid", -0.15, 0.10, hz(0.34), hz(0.46), (0.115, 0.11), (0.08, 0.078), "mech_gunmetal", inset=0.02)
        slab("plate_chest", -0.34, -0.11, hz(0.29), hz(0.455), (0.095, 0.115), (0.055, 0.08), "mech_gunmetal",
             inset=0.015, dz=(-0.045, 0.0))
        for k, y in enumerate((-0.07, -0.02, 0.03)):    # vents across the mid plate
            obox("vent%d" % k, (-0.05, y, hz(0.462)), (0.05, y, hz(0.462)), 0.008, 0.006, JOINT)
        obox("seam", (-0.075, 0.095, hz(0.445)), (0.075, 0.095, hz(0.445)), 0.006, 0.01, JOINT)
        # stabiliser spine: a short raised tail of three plated vertebrae
        rise = math.radians(32 + p.tail)
        d = Vector((0, math.cos(rise), math.sin(rise)))
        t0 = Vector((0, 0.3, hz(0.4)))
        pts = [t0 + d * (0.05 * k) for k in range(4)]
        P.tube("tail", pts, [0.016, 0.014, 0.012, 0.008], JOINT, segs=8, caps=(0.5, 0.6))
        for k in range(1, 4):
            obox("vert%d" % k, pts[k] - d * 0.018, pts[k] + d * 0.018, 0.026 - 0.004 * k, 0.018, "mech_gunmetal")
        for sx in (-1, 1):
            harrow_arm(sx, getattr(p, "arm", 0.0), dead)
    piv = (0, -0.32, hz(0.42))
    with attach(pivot(sk, piv, p.head)):
        obox("neck", (0, -0.29, hz(0.4)), (0, -0.39, hz(0.43)), 0.035, 0.03, JOINT)
        hull("head", [(s * 0.06, -0.36, hz(0.38)) for s in (-1, 1)] + [(s * 0.05, -0.36, hz(0.48)) for s in (-1, 1)]
             + [(s * 0.036, -0.46, hz(0.47)) for s in (-1, 1)] + [(s * 0.012, -0.585, hz(0.4)) for s in (-1, 1)]
             + [(s * 0.01, -0.585, hz(0.425)) for s in (-1, 1)], "mech_gunmetal")
        hull("jaw", [(s * 0.045, -0.37, hz(0.365)) for s in (-1, 1)] + [(s * 0.04, -0.37, hz(0.4)) for s in (-1, 1)]
             + [(s * 0.01, -0.55, hz(0.39)) for s in (-1, 1)] + [(0, -0.55, hz(0.405))], JOINT)
        obox("slit", (-0.058, -0.44, hz(0.474)), (0.058, -0.44, hz(0.474)), 0.011, 0.0075,
             OFF if dead else "mech_lens_amber", bevel=0.002)
    build_legs(sk)
    if dead:
        wreck(p, spec.key)
    return sk


def harrow_poses(spec):
    out = walk_set(spec, lambda ph: {"head": 2 * math.sin(2 * math.pi * ph), "tail": 4 * math.sin(4 * math.pi * ph),
                                     "arm": 0.06 * math.sin(2 * math.pi * ph)})
    out["slash1"] = MechPose(ty=0.02, pitch=-5, head=-6, arm=-0.5)
    out["slash2"] = MechPose(ty=0.04, bz=0.015, pitch=-9, head=-10, arm=-1.0, feet={"fl": (0.03, 0.0)})
    out["slash3"] = MechPose(ty=-0.09, pitch=7, head=6, arm=1.0,
                             feet={"fl": (-0.08, 0.0), "fr": (-0.05, 0.0), "hl": (0.03, 0.0), "hr": (0.04, 0.0)})
    out["slash4"] = MechPose(ty=-0.04, pitch=3, head=2, arm=0.45, feet={"fl": (-0.03, 0.0)})
    out["graze1"] = MechPose(ty=-0.01, pitch=4, head=18, arm=0.15)
    out["graze2"] = MechPose(ty=-0.02, bz=0.02, pitch=7, head=30, arm=0.2)
    out["flinch1"] = MechPose(ty=0.05, bz=0.01, pitch=-9, head=-14, roll=5, arm=-0.45, feet={"fl": (0.04, 0.02)})
    out["flinch2"] = MechPose(ty=0.025, pitch=-4, head=-6, arm=-0.2)
    out["rest"] = MechPose(bz=0.15, head=14, tail=-14)      # powered down: crouched, blades folded
    return out


# =============================================================== spindle
SP_Z0 = 0.62


def sz(z):
    return z - SP_Z0


def spindle_legs(b):
    out = []
    for n, (dx, dy), ph in (("fl", (0.8, -0.6), 0.0), ("fr", (-0.8, -0.6), 1 / 3), ("hm", (0.0, 1.0), 2 / 3)):
        d = Vector((dx, dy, 0)).normalized()
        out.append(mk_leg(n, 1, (d.x * 0.04, d.y * 0.04, sz(0.6)), (d.x * 0.085, d.y * 0.085, sz(0.6)), 0.4, 0.42,
                          (d.x * 0.55, d.y * 0.55, 1.0), (d.x * 0.36, d.y * 0.36), (0.032, 0.027, 0.021, 0.016),
                          "mech_bone", "spike", ph, dy < 0))
    return out


def spindle_model(spec, p):
    sk = Skel(spec, p)
    dead = p.dead >= 0
    with attach(sk.B):
        ocyl("hub", (0, 0, sz(0.55)), (0, 0, sz(0.62)), 0.07, JOINT, segs=16, r2=0.08)
        ocyl("drum", (0, 0, sz(0.6)), (0, 0, sz(0.72)), 0.115, "mech_bone", segs=22)
        ocyl("band", (0, 0, sz(0.645)), (0, 0, sz(0.668)), 0.119, JOINT, segs=22, bevel=0.0)
        ocyl("cap", (0, 0, sz(0.72)), (0, 0, sz(0.745)), 0.095, "mech_bone", segs=20, r2=0.075)
        ocyl("bezel", (0, -0.09, sz(0.668)), (0, -0.122, sz(0.668)), 0.058, JOINT, segs=18)
        lens("lens", (0, -0.124, sz(0.668)), (0, -1, 0), 0.047, OFF if dead else "mech_lens_cyan")
        obox("pylon", (0, 0.02, sz(0.74)), (0, 0.0, sz(0.795)), 0.03, 0.026, JOINT, up=(0, -1, 0))
    with attach(pivot(sk, (0, 0.0, sz(0.8)), -getattr(p, "aim", 0.0))):
        z = sz(0.805)
        obox("shroud", (0, 0.24, z), (0, -0.2, z), 0.036, 0.03, "mech_bone")
        obox("battery", (0, 0.24, z - 0.005), (0, 0.34, z - 0.005), 0.042, 0.04, JOINT)
        for sx in (-1, 1):
            obox("rail", (sx * 0.015, -0.18, z), (sx * 0.015, -0.8, z), 0.008, 0.014, JOINT, bevel=0.003)
        lit = getattr(p, "charge", 0.0) > 0.5 and not dead
        for k, y in enumerate((-0.32, -0.48, -0.64)):
            ocyl("coil%d" % k, (0, y + 0.012, z), (0, y - 0.012, z), 0.031, "mech_lens_cyan" if lit else "mech_gunmetal",
                 segs=14)
        obox("muzzle", (0, -0.78, z), (0, -0.835, z), 0.03, 0.022, "mech_bone")
        flash((0, -0.84, z), (0, -1, 0), getattr(p, "fire", 0.0), 1.0)
    build_legs(sk)
    if dead:
        wreck(p, spec.key)
    return sk


def spindle_poses(spec):
    out = walk_set(spec)
    out["shoot1"] = MechPose(bz=0.05, pitch=2, aim=2)
    out["shoot2"] = MechPose(bz=0.06, pitch=2, aim=2, charge=1)
    out["shoot3"] = MechPose(bz=0.05, ty=0.04, pitch=-4, aim=6, charge=1, fire=1)
    out["shoot4"] = MechPose(bz=0.04, ty=0.02, pitch=-1, aim=3)
    out["graze1"] = MechPose(bz=0.03, pitch=6, aim=-2)
    out["graze2"] = MechPose(bz=0.06, pitch=10, aim=-4)
    out["flinch1"] = MechPose(ty=0.05, pitch=-8, roll=6, aim=8, feet={"fl": (0.04, 0.03)})
    out["flinch2"] = MechPose(ty=0.025, pitch=-4, roll=3, aim=4)
    out["rest"] = MechPose(bz=0.32, aim=-2)                  # folded down on its stilts
    return out


# =============================================================== bulwark
BW_Z0 = 0.18
BW_LIFT = 0.05      # the hull rides this much higher than authored (legs stretch), for the XL drawn area


def bz_(z):
    return z + BW_LIFT - BW_Z0


def bulwark_legs(b):
    out = []
    ph = {"fl": 0.0, "mr": 0.0, "hl": 0.0, "fr": 0.5, "ml": 0.5, "hr": 0.5}
    for row, y, fy in (("f", -0.26, -0.29), ("m", 0.0, 0.0), ("h", 0.26, 0.29)):
        for sx in (1, -1):
            n = row + ("l" if sx > 0 else "r")
            out.append(mk_leg(n, sx, (sx * 0.27, y, bz_(0.19)), (sx * 0.33, y, bz_(0.17)), 0.15, 0.17, (sx * 1.0, 0.0, 0.9),
                              (sx * 0.47, fy), (0.07, 0.06, 0.05, 0.035), "mech_soot", "pad", ph[n], row == "f"))
    return out


BW_ROWS = ((0.18, 0.44, 0.38, 0.34, 0.27, (0.0, -0.06)),    # (y0, y1, top, bottom half width, top half width, dz)
           (-0.03, 0.25, 0.47, 0.36, 0.29, (0.0, 0.0)),
           (-0.23, 0.05, 0.49, 0.36, 0.29, (0.0, 0.0)),
           (-0.41, -0.16, 0.42, 0.34, 0.26, (-0.07, 0.0)))


def bulwark_model(spec, p):
    sk = Skel(spec, p)
    dead = p.dead >= 0
    with attach(sk.B):
        slab("hull", -0.4, 0.42, bz_(0.1), bz_(0.22), 0.33, 0.38, "mech_soot", inset=0.01)
        for sx in (-1, 1):
            obox("skirt", (sx * 0.385, -0.36, bz_(0.215)), (sx * 0.385, 0.38, bz_(0.215)), 0.03, 0.06, "mech_soot")
            obox("skirt_band", (sx * 0.417, -0.34, bz_(0.24)), (sx * 0.417, 0.36, bz_(0.24)), 0.004, 0.014, "mech_olive",
                 bevel=0.002)
        for k, (y0, y1, top, wb, wt, dz) in enumerate(BW_ROWS):
            slab("dome%d" % k, y0, y1, bz_(0.2), bz_(top), wb, wt, "mech_olive", inset=0.025, dz=dz, bevel=0.012)
            if k:
                ys = y1 - 0.006
                obox("seam%d" % k, (-wt, ys, bz_(top + 0.004)), (wt, ys, bz_(top + 0.004)), 0.007, 0.006, "mech_soot")
        ocyl("hatch", (0, 0.1, bz_(0.465)), (0, 0.1, bz_(0.495)), 0.075, "mech_soot", segs=18)
        for k, y in enumerate((0.26, 0.31, 0.36)):
            obox("vent%d" % k, (-0.14, y, bz_(0.375 - 0.13 * (y - 0.18))), (0.14, y, bz_(0.375 - 0.13 * (y - 0.18))),
                 0.01, 0.008, "mech_soot")
        ocyl("ring", (0, -0.36, bz_(0.17)), (0, -0.36, bz_(0.3)), 0.125, JOINT, segs=20)
        for k in range(5):
            x = -0.12 + 0.06 * k
            P.ell("light%d" % k, (x, -0.338, bz_(0.375)), (0.016, 0.016, 0.016), OFF if dead else "mech_lens_red",
                  segs=8, rings=5)
    with attach(pivot(sk, (0, -0.4, bz_(0.25)), -getattr(p, "aim", 0.0))):
        z = bz_(0.25)
        obox("housing", (0, -0.33, z), (0, -0.53, z), 0.078, 0.062, "mech_soot", bevel=0.012)
        obox("housing_band", (0, -0.43, z + 0.062), (0, -0.5, z + 0.062), 0.05, 0.006, "mech_olive", bevel=0.002)
        ocyl("collar_rear", (0, -0.52, z), (0, -0.555, z), 0.056, JOINT, segs=16)
        spin = math.radians(getattr(p, "spin", 0.0))
        for k in range(6):
            a = spin + k * math.pi / 3
            off = Vector((math.cos(a) * 0.03, 0, math.sin(a) * 0.03))
            ocyl("barrel%d" % k, Vector((0, -0.55, z)) + off, Vector((0, -0.77, z)) + off, 0.011, "steel", segs=8)
        ocyl("collar_mid", (0, -0.66, z), (0, -0.685, z), 0.048, JOINT, segs=16)
        ocyl("collar_tip", (0, -0.75, z), (0, -0.77, z), 0.046, JOINT, segs=16)
        flash((0, -0.79, z), (0, -1, 0), getattr(p, "fire", 0.0), 0.9)
    build_legs(sk)
    if dead:
        wreck(p, spec.key)
    return sk


def bulwark_poses(spec):
    out = walk_set(spec, lambda ph: {"roll": 1.2 * math.sin(2 * math.pi * ph)})
    out["shoot1"] = MechPose(bz=0.01, aim=2)
    out["shoot2"] = MechPose(bz=0.012, ty=0.01, aim=2, spin=20, fire=0.6)
    out["shoot3"] = MechPose(bz=0.012, ty=0.025, pitch=-2, aim=3, spin=40, fire=1.0)
    out["shoot4"] = MechPose(bz=0.01, ty=0.01, aim=2, spin=55)
    out["graze1"] = MechPose(bz=0.02, aim=-6)
    out["graze2"] = MechPose(bz=0.03, pitch=3, aim=-10)
    out["flinch1"] = MechPose(ty=0.03, pitch=-5, roll=4, aim=6, feet={"fl": (0.03, 0.02)})
    out["flinch2"] = MechPose(ty=0.015, pitch=-2, roll=2, aim=3)
    out["rest"] = MechPose(bz=0.09 + BW_LIFT, aim=-8)                  # hull settled on the ground, legs folded
    return out


# ================================================================= quill
Q_Z0 = 0.5


def qz(z):
    return z - Q_Z0


def quill_legs(b):
    out = []
    for n, y, fy, by, ph in (("fl", -0.12, -0.28, -0.3, 0.0), ("fr", -0.12, -0.28, -0.3, 0.5),
                             ("hl", 0.12, 0.28, 0.3, 0.5), ("hr", 0.12, 0.28, 0.3, 0.0)):
        sx = 1 if n[1] == "l" else -1
        out.append(mk_leg(n, sx, (sx * 0.05, y * 0.8, qz(0.5)), (sx * 0.08, y, qz(0.47)), 0.32, 0.36, (sx * 0.8, by, 1.0),
                          (sx * 0.3, fy), (0.026, 0.022, 0.018, 0.014), "mech_sand", "spike", ph, n[0] == "f"))
    return out


def quill_model(spec, p):
    sk = Skel(spec, p)
    dead = p.dead >= 0
    with attach(sk.B):
        slab("frame", -0.15, 0.15, qz(0.42), qz(0.48), 0.06, 0.07, JOINT)
        slab("body", -0.18, 0.17, qz(0.46), qz(0.58), 0.09, 0.08, "mech_sand", inset=0.015)
        for k, y in enumerate((-0.065, 0.055)):
            slab("stripe%d" % k, y - 0.018, y + 0.018, qz(0.455), qz(0.587), 0.094, 0.084, "mech_stripe", bevel=0.003)
        obox("stalk", (0, -0.14, qz(0.57)), (0, -0.18, qz(0.77)), 0.008, 0.008, JOINT, up=(0, -1, 0), bevel=0.002)
        obox("sensor", (0, -0.15, qz(0.775)), (0, -0.225, qz(0.775)), 0.026, 0.02, "mech_sand")
        lens("eye", (0, -0.227, qz(0.775)), (0, -1, 0), 0.015, OFF if dead else "mech_lens_green", depth=0.5)
        obox("pylon", (0, 0.08, qz(0.57)), (0, 0.05, qz(0.665)), 0.022, 0.022, JOINT, up=(0, -1, 0))
    with attach(pivot(sk, (0, 0.05, qz(0.67)), -(6 + getattr(p, "aim", 0.0)))):
        slab("pod", -0.06, 0.12, qz(0.64), qz(0.745), 0.07, 0.064, "mech_sand", inset=0.01)
        slab("pod_stripe", 0.06, 0.09, qz(0.635), qz(0.75), 0.073, 0.067, "mech_stripe", bevel=0.003)
        for zr in (0.667, 0.717):
            for x in (-0.045, -0.015, 0.015, 0.045):
                ocyl("tube", (x, -0.05, qz(zr)), (x, -0.24, qz(zr)), 0.0115, JOINT, segs=8)
                ocyl("tip", (x, -0.235, qz(zr)), (x, -0.252, qz(zr)), 0.0135, "mech_stripe", segs=8)
        flash((0, -0.27, qz(0.692)), (0, -1, 0), getattr(p, "fire", 0.0), 0.8)
    build_legs(sk)
    if dead:
        wreck(p, spec.key)
    return sk


def quill_poses(spec):
    out = walk_set(spec, lambda ph: {"aim": 1.5 * math.sin(2 * math.pi * ph)})
    out["shoot1"] = MechPose(bz=0.03, aim=8)
    out["shoot2"] = MechPose(bz=0.04, aim=12)
    out["shoot3"] = MechPose(bz=0.04, ty=0.03, pitch=-3, aim=12, fire=1)
    out["shoot4"] = MechPose(bz=0.03, ty=0.015, aim=8)
    out["graze1"] = MechPose(bz=0.04, pitch=6)
    out["graze2"] = MechPose(bz=0.08, pitch=10, aim=-3)
    out["flinch1"] = MechPose(ty=0.05, pitch=-8, roll=6, aim=-4, feet={"fl": (0.04, 0.03)})
    out["flinch2"] = MechPose(ty=0.025, pitch=-4, roll=3, aim=-2)
    out["rest"] = MechPose(bz=0.3, aim=-6)                   # folded low on its spider legs
    return out


# ----------------------------------------------------------------- specs
SPECS = {
    "harrow": dict(height=H_Z0 + 0.1, legs=harrow_legs, model=harrow_model, poses_fn=harrow_poses, coat="mech_gunmetal"),
    "spindle": dict(height=SP_Z0 + 0.1, legs=spindle_legs, model=spindle_model, poses_fn=spindle_poses,
                    coat="mech_bone"),
    "bulwark": dict(height=BW_Z0 + 0.1, legs=bulwark_legs, model=bulwark_model, poses_fn=bulwark_poses,
                    coat="mech_olive"),
    "quill": dict(height=Q_Z0 + 0.1, legs=quill_legs, model=quill_model, poses_fn=quill_poses, coat="mech_sand"),
}


def _spec(row):
    kw = dict(SPECS[row.key])
    fn = kw.pop("poses_fn")
    coat = kw.pop("coat")
    b = Body(key=row.key, length=R.body_length(row.size), attack=row.attack, archetype=row.archetype, ears=None,
             tail=None, chest=(0.2, 0.1), pal=dict(coat=coat), pool="mech_oil", **kw)
    b.poses = fn(b)
    return b


def build(row, static=False):
    if row.key not in SPECS:
        return []
    return render(_spec(row), static, row.size)
