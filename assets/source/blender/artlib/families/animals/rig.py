"""Shared animal rig (ART-011): Body spec, skeleton, poses and rendering.

Model space: the animal is authored at unit body length (nose to rump
~1.0), standing on z=0, centred on the origin, nose towards -Y (south).
render_species() scales it to the species size, moves it to the tile
centre, turns it to each facing and leans it back (LEAN_DEG) like the old
animal family, then renders every frame through stage.render_asset.
Only south, east and north are rendered: the renderer draws west as east
mirrored about the tile centre, so every canvas is horizontally symmetric
around the footprint (left margin == right margin).

Posing happens at build time: a Pose (body shift/pitch, neck, head, jaw,
tail, per-leg foot targets) drives the skeleton (Skel), and every part is
built where the posed bones put it. Legs are two-bone IK chains from a
body-frame joint to a foot target, so feet stay planted while the body
moves.

PLAY-022: each facing gets its own tight canvas (the union of that
facing's frames) at a per-sprite density `ppt` (pixels per tile, written
to the metadata; large canvases render below contracts.PPT and the
renderer scales them up). The walk plants every foot for DUTY of the
cycle at one ground speed, and every animal sprite carries `gait`: the
tiles an adult travels per 8-frame walk cycle (gait_tiles).

Other archetypes (A2: bird, reptile, insect) reuse this module:
- give Body a `legs` callable (spec -> [Leg]) to change the leg layout,
  and/or a `model` callable (spec, pose) that builds the geometry with the
  helpers here (Skel, attach, build_legs, build_head, build_tail, ...);
- give Body `poses` to add or override frames (e.g. the peck set);
- call render_species(spec) from the archetype's build(row).
"""
import math
import os
from collections import namedtuple
from contextlib import contextmanager
from dataclasses import dataclass, field, replace

import numpy as np
from mathutils import Matrix, Vector

from ... import contracts as C
from ... import stage as S
from . import parts as P

FACINGS = ("south", "east", "north")   # west = east mirrored by the renderer (ART §11.1)
CX, CY = 0.5, -0.5
LEAN_DEG = C.ANIMAL_LEAN_DEG   # animals lean less than people (contracts.ANIMAL_LEAN_DEG): the body stays long
TROT_N = 8
WALK_FRAMES = tuple("walk%02d" % i for i in range(1, TROT_N + 1))
ATTACK_KINDS = ("bite", "claw", "kick", "gore", "headbutt", "peck", "slash", "shoot")   # slash/shoot: mech.py poses
GRAZE = ("graze1", "graze2")
FLINCH = ("flinch1", "flinch2")
CORPSE_VARIANTS = 3
PAD_PX = 3                # canvas pixels beyond the outline (its soft fringe, anti-aliasing, one spare)
CANVAS_CAP = 384          # px: a facing canvas longer than this at contracts.PPT renders at a lower ppt
MIN_PPT = 64              # never below half density

# Walk (ART §27.1, PLAY-022). Diagonal pairs (fl+hr, fr+hl); every foot is
# planted for DUTY of the cycle (5 of the 8 frames, then a toe-off and two
# swing frames) and slides back under the body at the same rate, so the
# body travels stride / DUTY per cycle with no foot skating.
DUTY = 0.6
STRIDE_K = 0.6            # quadrupeds: stance sweep / shoulder-hip height (about +-17 deg about the shoulder)
STRIDE_LEG = 0.42         # other leg layouts: stance sweep / leg length
LIFT_K = (0.14, 0.2)      # swing clearance / leg length: (quadruped, other layouts)
BOB_MIN = 0.012           # body lengths: the least body dip at contact (walk01/05) that reads at game scale
DROP_MAX = 0.04           # body lengths: the deepest dip allowed; a stride that needs more is narrowed
WALK_PITCH = 4.0          # degrees nose-down at contact, level at passing (walk03/07)
WALK_NOD = 9.0            # head nod (degrees, nose down at contact)
WALK_NECK = 4.0           # neck dips at contact (degrees)
WALK_ROLL = 1.5           # side-to-side weight shift, once per cycle (degrees)
NOSE_MAX = 0.16           # the nose rises and falls at most this share of the withers height per cycle:
                          # low-carried heads (dogs, cats, rodents) nod less than deer and horses
TRUNK_SWING = 35.0        # a trunk keeps hanging under the nodding head and swings forward at contact (degrees),
                          # so the dipping head never drives its tip into the ground


# ------------------------------------------------------------------ specs
@dataclass
class Leg:
    """One leg. Body-frame points move with the torso; the foot is a
    model-space target, solved by two-bone IK from `joint`."""
    name: str                 # fl, fr, hl, hr (others for A2's bodies)
    root: tuple               # body frame: top of shoulder / haunch (inside the torso)
    joint: tuple              # body frame: elbow / stifle, where IK starts
    l1: float                 # joint -> knee/hock
    l2: float                 # knee/hock -> foot
    bend: tuple               # model-space direction the knee/hock points
    foot: tuple               # rest foot (x, y) in model space
    r: tuple                  # radii: root, joint, knee, foot
    mat: str
    lower: str = None         # material below the knee (socks)
    foot_kind: str = "hoof"   # hoof, paw, pad, claw, hidden
    foot_mat: str = "fur_dark"
    phase: float = 0.0        # trot phase offset (0..1)
    front: bool = True


@dataclass
class Body:
    """A species, in body lengths. Defaults are a mid-size deer-like
    quadruped; species specs override what makes them themselves."""
    key: str
    length: float = 1.0                 # tiles: roster body length
    attack: str = "bite"
    archetype: str = "quadruped"
    # torso
    height: float = 0.55                # top of the withers above the ground
    torso: float = 0.62                 # rump to chest
    chest: tuple = (0.11, 0.14)         # half width, half depth
    waist: tuple = (0.10, 0.12)
    hips: tuple = (0.105, 0.125)
    rump_z: float = 0.0                 # back height at the rump minus at the withers
    arch: float = 0.0                   # back arch (+ up) mid-torso
    belly: float = 0.0                  # extra belly depth (sag) mid-torso
    torso_p: float = 2.6                # end roundness (lower = more pointed ends)
    # legs
    leg_r: tuple = (0.06, 0.04, 0.026, 0.022)
    hock: float = 0.9                   # hind reach: lower = deeper hock angle
    stance: float = 0.62                # leg spread as a fraction of the chest width
    foot: str = "hoof"
    hind_r: float = 1.15                # hind leg thickness factor
    muscle: float = 1.0                 # shoulder/haunch mass and forearm/thigh bulge (cervids <1, cats/dogs >1)
    # neck and head
    neck_len: float = 0.24
    neck_angle: float = 50.0            # above horizontal
    neck_r: tuple = (0.075, 0.045)      # base, top
    neck_bow: float = 0.0               # + bulges the top line
    neck_y: float = 0.1                 # neck base along the torso from the chest end (fraction)
    head: tuple = (0.055, 0.07, 0.055)  # cranium half width, half length, half height
    head_angle: float = 35.0            # nose down from horizontal
    muzzle: tuple = (0.09, 0.03, 0.03)  # length, half width, half height
    muzzle_drop: float = 12.0
    ears: tuple = ("pointed", 1.0, 0.0)  # kind, size, tilt back
    horns: tuple = None                  # (kind, size, ridged)
    antlers: tuple = None                # (kind, size)
    tail: tuple = ("short", 1.0, 0.0)    # kind, size, raise
    eye: float = 0.011                   # eye radius
    pal: dict = field(default_factory=dict)
    marks: tuple = ()                    # extra features, see MARKS
    fangs: bool = False                  # teeth show when the jaw opens
    female: "Body" = None                # dimorphic second set (_f)
    legs: object = None                  # callable(spec) -> [Leg] (default quadruped_legs)
    model: object = None                 # callable(spec, pose) builder (default quadruped)
    poses: dict = None                   # extra/override frames {name: Pose}
    corpse_scale: float = 1.0
    pool: str = "blood_dried"            # the corpse's dried pool material; None = no pool (mech wrecks: oil)
    outline: int = None                  # outline px at contracts.PPT; None = contracts default. Thinner for XS/S.
    stride: float = None                 # walk stance sweep (body lengths); None = from the legs (gait())
    travel: float = None                 # ground travel per walk cycle (body lengths) for walks without feet
    dip: bool = True                     # the walk may dip the body to keep planted feet in reach (gait())
    nod: float = 1.0                     # walk head/neck nod scale (low-carried heads nod less)

    def c(self, role, default=KeyError):
        """Palette role -> material key. Unset roles fall back along
        ROLE_FALLBACK; roles with no fallback return `default` (or None)."""
        if role in self.pal:
            return self.pal[role]
        if default is not KeyError:
            return default
        fb = ROLE_FALLBACK.get(role, "coat")
        if fb is None:
            return None
        if fb in ROLE_DEFAULT and fb not in self.pal:
            return ROLE_DEFAULT[fb]
        return self.c(fb)


# palette roles: where an unset role borrows its material from
ROLE_FALLBACK = {"coat": "_coat", "belly": "coat", "muzzle": "coat", "head": "coat", "ear": "coat", "legs": "coat",
                 "tail": "coat", "jaw": "muzzle", "throat": "belly", "mane": "dark", "dark": "_dark",
                 "hoof": "dark", "nose": "dark", "eye": "_eye", "horn": "_horn", "antler": "horn",
                 "lower": None, "ear_in": None}
ROLE_DEFAULT = {"_coat": "fur_tan", "_dark": "fur_dark", "_eye": "eye_black", "_horn": "horn"}


@dataclass
class Pose:
    ty: float = 0.0          # body shift along Y (negative = forward)
    bz: float = 0.0          # body drop (positive = lower)
    pitch: float = 0.0       # degrees, positive = nose down
    pivot: float = 0.0       # body-frame Y that the pitch turns about
    roll: float = 0.0        # degrees about the spine
    neck: float = 0.0        # added to neck_angle (positive raises)
    head: float = 0.0        # added to head_angle (positive = nose down)
    yaw: float = 0.0         # head turn, degrees
    jaw: float = 0.0         # jaw open, degrees
    tail: float = 0.0        # tail raise, degrees
    sway: float = 0.0        # tail swing, degrees
    ears: float = 0.0        # ears pinned back, degrees
    feet: dict = field(default_factory=dict)   # leg -> (dy, dz): model-space offset from the rest foot
    paws: dict = field(default_factory=dict)   # leg -> (dy, dz): foot relative to the posed joint
    trunk: float = 0.0       # trunk raise (elephant)
    dead: int = -1           # corpse variant, -1 alive
    swing: float = 0.0       # 0..1: how far the upper leg (root->joint) turns with the foot about the shoulder/hip


# --------------------------------------------------------------- skeleton
def _smooth(t):
    t = max(0.0, min(1.0, t))
    return t * t * (3 - 2 * t)


class Skel:
    """Posed bones for a Body (pure math; no geometry)."""

    def __init__(self, b, p):
        self.b, self.p = b, p
        cw, cd = b.chest
        self.z0 = b.height - cd
        pv = p.pivot
        self.B = (Matrix.Translation((0, p.ty, self.z0 - p.bz)) @ Matrix.Translation((0, pv, 0))
                  @ Matrix.Rotation(math.radians(p.pitch), 4, "X") @ Matrix.Rotation(math.radians(p.roll), 4, "Y")
                  @ Matrix.Translation((0, -pv, 0)))
        self.legs = (b.legs or quadruped_legs)(b)
        nb = self.neck_base()
        th = math.radians(b.neck_angle + p.neck)
        d = Vector((0, -math.cos(th), math.sin(th)))
        self.poll = nb + d * b.neck_len
        perp = Vector((0, math.sin(th), math.cos(th)))
        self.neck_mid = (nb + self.poll) / 2 + perp * b.neck_bow * b.neck_len
        self.H = (Matrix.Translation(self.poll) @ Matrix.Rotation(math.radians(p.yaw), 4, "Z")
                  @ Matrix.Rotation(math.radians(b.head_angle + p.head), 4, "X"))

    # torso profile, body frame. u: 0 rump end .. 1 chest end
    def section(self, u):
        b = self.b
        T = b.torso
        y = T / 2 - u * T
        hw, hd = b.hips
        ww, wd = b.waist
        cw, cd = b.chest
        if u < 0.5:
            k = _smooth((u - 0.2) / 0.3)
            rx, rz = P.lerp(hw, ww, k), P.lerp(hd, wd, k)
        else:
            k = _smooth((u - 0.5) / 0.3)
            rx, rz = P.lerp(ww, cw, k), P.lerp(wd, cd, k)
        back = P.lerp(cd + b.rump_z, cd, _smooth(u)) + b.arch * math.sin(math.pi * u)
        sag = b.belly * math.sin(math.pi * min(1.0, max(0.0, (u - 0.1) / 0.75)))
        bottom = back - 2 * rz - sag
        rz = (back - bottom) / 2
        zc = (back + bottom) / 2
        e = (1 - abs(2 * u - 1) ** b.torso_p) ** (1 / b.torso_p)
        return y, zc, rx * e, rz * e, rx, rz

    def surface(self, u, theta, lift=0.0):
        """Body-frame point and normal on the torso at u, angle theta
        (0 = left side +X, 90 = top)."""
        y, zc, rx, rz, _, _ = self.section(u)
        c, s = math.cos(math.radians(theta)), math.sin(math.radians(theta))
        n = Vector((c / max(rx, 1e-4), 0, s / max(rz, 1e-4))).normalized()
        return Vector((rx * c, y, zc + rz * s)) + n * lift, n

    def neck_base(self):
        b = self.b
        u = 1.0 - b.neck_y
        y, zc, rx, rz, _, _ = self.section(u)
        return Vector((0, y, zc + rz * 0.45))

    def tail_base(self):
        y, zc, rx, rz, _, _ = self.section(0.06)
        return Vector((0, y + 0.01, zc + rz * 0.55))

    def world(self, v):
        return self.B @ Vector(v)

    def head_world(self, v):
        return self.B @ self.H @ Vector(v)

    def head_geo(self):
        """Head-local landmarks."""
        b = self.b
        hw, hl, hh = b.head
        c = Vector((0, -hl * 0.85, -hh * 0.3))
        ml, mw, mh = b.muzzle
        a = math.radians(b.muzzle_drop)
        m0 = c + Vector((0, -hl * 0.55, -hh * 0.15))
        tip = m0 + Vector((0, -math.cos(a), -math.sin(a))) * ml
        return dict(c=c, m0=m0, tip=tip, hw=hw, hl=hl, hh=hh, ml=ml, mw=mw, mh=mh)

    def nose_world(self):
        return self.head_world(self.head_geo()["tip"])

    def joint_world(self, leg):
        """The elbow/stifle where the leg's IK starts. With Pose.swing the
        upper leg turns about the shoulder/hip (root) by that fraction of
        the root->foot angle change, so the whole leg strides like a strut
        instead of pivoting at the elbow (feet given by `paws` never swing)."""
        j = self.B @ Vector(leg.joint)
        p = self.p
        if not p.swing or leg.name in p.paws:
            return j
        r = self.B @ Vector(leg.root)
        t = self.foot_target(leg)
        a0 = math.atan2(leg.foot[1] - r.y, r.z - leg.r[3])
        a1 = math.atan2(t.y - r.y, r.z - t.z)
        return (Matrix.Translation(r) @ Matrix.Rotation((a1 - a0) * p.swing, 4, "X") @ Matrix.Translation(-r)) @ j

    def foot_target(self, leg):
        p = self.p
        if leg.name in p.paws:
            dy, dz = p.paws[leg.name]
            j = self.joint_world(leg)
            return Vector((leg.foot[0], j.y + dy, j.z + dz))
        dy, dz = p.feet.get(leg.name, (0.0, 0.0))
        return Vector((leg.foot[0], leg.foot[1] + dy, leg.r[3] + dz))


def ik2(a, t, l1, l2, bend):
    """Knee and (reachable) foot for a two-bone chain from a to target t,
    the knee pushed towards `bend`."""
    a, t = Vector(a), Vector(t)
    d = t - a
    dist = max(1e-4, min(d.length, (l1 + l2) * 0.999, ))
    u = d.normalized() if d.length > 1e-6 else Vector((0, 0, -1))
    foot = a + u * dist
    x = (l1 * l1 - l2 * l2 + dist * dist) / (2 * dist)
    h = math.sqrt(max(0.0, l1 * l1 - x * x))
    bv = Vector(bend)
    perp = bv - u * bv.dot(u)
    perp = perp.normalized() if perp.length > 1e-6 else Vector((0, -1, 0))
    return a + u * x + perp * h, foot


def quadruped_legs(b):
    T = b.torso
    cw, cd = b.chest
    hw, hd = b.hips
    sk_sections = Skel.section
    # front legs under the chest, hind legs under the hips
    fake = type("S", (), {"b": b})()
    yf, zf, _, _, _, _ = sk_sections(fake, 0.8)
    yh, zh, _, _, _, _ = sk_sections(fake, 0.2)
    z0 = b.height - cd
    xf = cw * b.stance
    xh = hw * b.stance
    r0, r1, r2, r3 = b.leg_r
    hr = b.hind_r
    legs = []
    jf = zf - cd * 0.72
    Lf = (z0 + jf - r3) / 0.985
    jh = zh - hd * 0.62
    Lh = (z0 + jh - r3) / b.hock
    foot_kind = b.foot
    for name, sx, ph in (("fl", 1, 0.0), ("fr", -1, 0.5)):
        legs.append(Leg(name, (sx * xf, yf - 0.01, zf + cd * 0.1), (sx * xf, yf + 0.015, jf), Lf * 0.5, Lf * 0.5,
                        (0, -1, 0), (sx * xf, yf + 0.015), (r0, r1, r2, r3), b.c("legs", b.c("coat")),
                        b.c("lower"), foot_kind, b.c("hoof"), ph, True))
    for name, sx, ph in (("hl", 1, 0.5), ("hr", -1, 0.0)):
        legs.append(Leg(name, (sx * xh, yh + 0.02, zh + hd * 0.2), (sx * xh, yh - 0.035, jh),
                        Lh * 0.52, Lh * 0.48, (0, 1, 0), (sx * xh, yh + 0.035),
                        (r0 * hr * 1.1, r1 * hr, r2 * 1.05, r3), b.c("legs", b.c("coat")), b.c("lower"),
                        foot_kind, b.c("hoof"), ph, False))
    return legs


# ------------------------------------------------------------ build helpers
@contextmanager
def attach(M):
    """Objects created inside the block are moved by M (a bone's matrix)."""
    before = {o.name for o in S.objects()}
    yield
    new = [o for o in S.objects() if o.name not in before]
    S.transform(new, M)


def build_torso(sk, material=None, scale=(1.0, 1.0), dz=0.0, name="body", n=15, segs=20, u0=0.0, u1=1.0):
    pts, radii = [], []
    for i in range(n):
        u = P.lerp(u0, u1, i / (n - 1))
        y, zc, rx, rz, _, _ = sk.section(u)
        pts.append((0, y, zc + dz))
        radii.append((rx * scale[0], rz * scale[1]))
    with attach(sk.B):
        P.tube(name, pts, radii, material or sk.b.c("coat"), segs=segs, caps=(0, 0))


def build_legs(sk):
    for leg in sk.legs:
        build_leg(sk, leg)


def _muscle(sk, leg, a, b, k):
    """Shoulder / haunch mass from root a to joint b (model space), blending
    the leg into the torso: wide at the top, tapering to the joint."""
    body = sk.b
    w, d = (body.chest if leg.front else body.hips)
    ax = (b - a)
    ln = ax.length
    if ln < 1e-5:
        return
    m = Vector((0, 0, 1)).rotation_difference(ax.normalized()).to_matrix().to_4x4()
    c = a + ax * 0.42
    rx = w * 0.42 * k
    ry = d * (0.42 if leg.front else 0.55) * k
    P.ell("haunch" if not leg.front else "shoulder", c, (rx, ry, ln * 0.62), leg.mat, m=m, segs=14, rings=10)


def _leg_path(joint, knee, foot, r1, r2, r3, k):
    """A tapered, organic limb: muscled upper segment (bulge k), a rounded
    knee/hock, a slim cannon and a slight fetlock swell above the foot."""
    up, lo = knee - joint, foot - knee
    pts, rad = [], []
    for t in (0.0, 0.18, 0.4, 0.62):
        bulge = math.sin(math.pi * min(1.0, t / 0.75)) * (k - 1.0)
        pts.append(joint + up * t)
        rad.append(P.lerp(r1, r2, (t / 0.85) ** 1.3) * (1.0 + bulge))
    a, c = knee - up * 0.2, knee + lo * 0.16
    for t in (0.0, 0.5, 1.0):
        pts.append(a * (1 - t) ** 2 + knee * 2 * t * (1 - t) + c * t * t)
        rad.append(r2 * (1.0 + 0.12 * math.sin(math.pi * t)))
    for t, r in ((0.42, P.lerp(r2, r3, 0.75)), (0.7, r3 * 0.95), (0.86, r3 * 1.1), (1.0, r3)):
        pts.append(knee + lo * t)
        rad.append(r)
    return pts, rad


def build_leg(sk, leg):
    b = sk.b
    root = sk.B @ Vector(leg.root)
    joint = sk.joint_world(leg)
    knee, foot = ik2(joint, sk.foot_target(leg), leg.l1, leg.l2, leg.bend)
    r0, r1, r2, r3 = leg.r
    k = b.muscle
    _muscle(sk, leg, root, joint, k)
    P.tube("leg", [root, joint], [r0 * 0.9, r1], leg.mat, segs=12, caps=(1, 0))
    pts, rad = _leg_path(joint, knee, foot, r1, r2, r3, 1.0 + 0.22 * k)
    if leg.foot_kind == "hidden":
        P.tube("leg", pts, rad, leg.mat, segs=12, caps=(1, 1))
        return
    if leg.lower:
        n = 6
        P.tube("leg", pts[:n + 1], rad[:n + 1], leg.mat, segs=12, caps=(1, 1))
        P.tube("leg", pts[n - 1:], [x * 1.02 for x in rad[n - 1:]], leg.lower, segs=12, caps=(1, 0.8))
    else:
        P.tube("leg", pts, rad, leg.mat, segs=12, caps=(1, 0.8))
    P.ell("knee", knee, (r2 * 1.12,) * 3, leg.mat, segs=10, rings=7)
    d = (foot - knee).normalized()
    fwd = Vector((0, -1, 0))
    k = leg.foot_kind
    if k == "hoof":
        h = r3 * 1.5
        P.tube("hoof", [foot - d * h * 0.2, foot + d * h], [r3 * 1.12, r3 * 1.3], leg.foot_mat, segs=10,
               caps=(0, 0.15))
    elif k in ("paw", "claw"):
        c = foot + d * r3 * 0.55 + fwd * r3 * 0.55
        P.ell("paw", c, (r3 * 1.2, r3 * 1.6, r3 * 0.85), leg.lower or leg.mat, segs=12, rings=8)
        if k == "claw":
            P.claws(c + fwd * r3 * 1.3 + Vector((0, 0, -r3 * 0.2)), fwd, 1.0, leg.foot_mat, n=3,
                    length=r3 * 2.4, spread=r3 * 0.55, curve=70)
    elif k == "bigclaw":
        c = foot + d * r3 * 0.55 + fwd * r3 * 0.4
        P.ell("paw", c, (r3 * 1.2, r3 * 1.5, r3 * 0.9), leg.lower or leg.mat, segs=12, rings=8)
        if leg.front:
            P.claws(c + fwd * r3 * 1.0 + Vector((0, 0, r3 * 0.1)), fwd, 1.0, leg.foot_mat, n=3,
                    length=r3 * 4.2, spread=r3 * 0.6, curve=100)
    elif k == "pad":
        if sk.p.swing and abs(foot.z - r3) < 1e-3:
            d = Vector((0, 0, -1))   # a planted pad (walk) stands flat on its sole, not tilted with the leg
        P.tube("pad", [foot - d * r3 * 0.3, foot + d * r3 * 0.9], [r3 * 1.15, r3 * 1.2], leg.lower or leg.mat,
               segs=14, caps=(0, 0.1))
        for sx in (-1, 0, 1):
            P.ell("nail", foot + d * r3 * 0.7 + Vector((sx * r3 * 0.6, -r3 * 1.05, 0)),
                  (r3 * 0.28, r3 * 0.18, r3 * 0.22), leg.foot_mat, segs=8, rings=5)


def build_neck(sk, material=None, scale=1.0, name="neck"):
    b = sk.b
    nb = sk.neck_base()
    pts = P.bezier(nb, sk.neck_mid, sk.poll, 7)
    r0, r1 = b.neck_r
    radii = [(P.lerp(r0, r1, (i / 6) ** 0.8) * scale) for i in range(7)]
    # pull the base back into the chest so the joint never shows a seam
    pts = [nb + (nb - pts[1]) * 0.6] + pts
    radii = [radii[0]] + radii
    with attach(sk.B):
        P.tube(name, pts, radii, material or b.c("neck", b.c("coat")), segs=14, caps=(0.6, 0.8))


def build_head(sk, ears=True, horns=True):
    """Cranium, muzzle, nose, eyes, jaw, ears, horns/antlers (head-local)."""
    b, p = sk.b, sk.p
    g = sk.head_geo()
    c, m0, tip = g["c"], g["m0"], g["tip"]
    hw, hl, hh, ml, mw, mh = g["hw"], g["hl"], g["hh"], g["ml"], g["mw"], g["mh"]
    coat = b.c("coat")
    with attach(sk.B @ sk.H):
        P.ell("head", c, (hw, hl, hh), b.c("head", coat), segs=18, rings=12)
        if ml > 0.005:
            mid = (m0 + tip) / 2 + Vector((0, 0, mh * 0.08))
            P.tube("muzzle", [c + Vector((0, -hl * 0.2, 0)), m0, mid, tip],
                   [(hw * 0.85, hh * 0.85), (mw * 1.15, mh * 1.15), (mw * 1.03, mh), (mw * 0.95, mh * 0.88)],
                   b.c("muzzle"), segs=14, caps=(0, 0.9))
        nose = b.c("nose")
        if nose and ml > 0.005:
            a = math.radians(b.muzzle_drop)
            fwd = Vector((0, -math.cos(a), -math.sin(a)))
            P.ell("nose", tip + fwd * mh * 0.72 + Vector((0, 0, mh * 0.25)), (mw * 0.62, mw * 0.36, mh * 0.42),
                  nose, segs=10, rings=6)
        # jaw (opens about a hinge under the cranium)
        if ml > 0.005:
            hinge = c + Vector((0, -hl * 0.25, -hh * 0.55))
            jt = tip + Vector((0, mh * 0.25, -mh * 0.75))
            J = (Matrix.Translation(hinge) @ Matrix.Rotation(math.radians(p.jaw), 4, "X")
                 @ Matrix.Translation(-hinge))
            with attach(J):
                P.tube("jaw", [hinge, (hinge + jt) / 2, jt], [(mw * 0.8, mh * 0.55), (mw * 0.8, mh * 0.5),
                                                              (mw * 0.6, mh * 0.35)],
                       b.c("jaw", b.c("muzzle")), segs=10, caps=(0.6, 0.8))
                if b.fangs and p.jaw > 8:
                    for sx in (-1, 1):
                        P.cone("fang", jt + Vector((sx * mw * 0.45, mh * 0.5, mh * 0.2)),
                               jt + Vector((sx * mw * 0.45, mh * 0.5, mh * 1.0)), mw * 0.16, "horn_ivory", segs=6)
            if b.fangs and p.jaw > 8:
                for sx in (-1, 1):
                    P.cone("fang", tip + Vector((sx * mw * 0.5, mh * 0.6, -mh * 0.5)),
                           tip + Vector((sx * mw * 0.5, mh * 0.6, -mh * 1.35)), mw * 0.18, "horn_ivory", segs=6)
        # eyes: tiny; closed slits on corpses
        for sx in (-1, 1):
            e = c + Vector((sx * hw * 0.8, -hl * 0.55, hh * 0.22))
            if p.dead >= 0:
                P.ell("eye", e, (b.eye * 0.8, b.eye * 1.3, b.eye * 0.35), "fur_dark", segs=8, rings=5)
            else:
                P.ell("eye", e, (b.eye, b.eye, b.eye * 1.1), b.c("eye"), segs=8, rings=6)
        if ears and b.ears:
            kind, size, tilt = b.ears
            for sx in (-1, 1):
                base = c + Vector((sx * hw * 0.62, hl * 0.45, hh * 0.72))
                P.ear(base, kind, size, b.c("ear", coat), sx, inner=b.c("ear_in"), tip=b.c("ear_tip", None)
                      if "ear_tip" in b.pal else None, angle=tilt + p.ears)
        if horns and b.horns:
            kind, size, ridged = (tuple(b.horns) + (False,))[:3]
            for sx in (-1, 1):
                base = c + Vector((sx * hw * 0.42, hl * 0.4, hh * 0.85))
                P.horn("horn", base, kind, size, b.c("horn"), side=sx, ridged=ridged)
        if horns and b.antlers:
            kind, size = b.antlers
            P.antlers(b.key, c.y + hl * 0.35, c.z + hh * 0.85, size, b.c("antler", b.c("horn")), kind=kind,
                      tip_material=b.c("antler_tip", None) if "antler_tip" in b.pal else None, spread=hw * 0.45)


def build_tail(sk):
    b, p = sk.b, sk.p
    if not b.tail:
        return
    kind, size, up = b.tail
    with attach(sk.B):
        P.tail(kind, sk.tail_base(), size, b.c("tail"), tip=b.c("tail_tip", None) if "tail_tip" in b.pal else None,
               rings=b.pal.get("rings"), raise_deg=up + p.tail, sway=p.sway, key=b.key)


# ---------------------------------------------------------------- marks
# Feature tuples on Body.marks: (name, *args). Each draws in the posed frame.
def _m_belly(sk, mat, depth=0.65, width=0.86):
    build_torso(sk, mat, scale=(width, 0.9), dz=-sk.b.chest[1] * depth * 0.45, name="belly", u0=0.08, u1=0.92)


def _m_saddle(sk, mat, depth=0.4, width=0.8, u0=0.12, u1=1.0):
    build_torso(sk, mat, scale=(width, 0.8), dz=sk.b.chest[1] * depth * 0.5, name="saddle", u0=u0, u1=u1)


def _m_throat(sk, mat, side=-1, k=0.28, name="throat"):
    """A lighter (or darker, side=+1: top) band along the neck."""
    b = sk.b
    nb = sk.neck_base()
    pts = P.bezier(nb, sk.neck_mid, sk.poll, 6)
    th = math.radians(b.neck_angle + sk.p.neck)
    down = Vector((0, -math.sin(th), -math.cos(th))) * -side
    r0, r1 = b.neck_r
    with attach(sk.B):
        P.tube(name, [q + down * P.lerp(r0, r1, i / 5) * k for i, q in enumerate(pts)],
               [P.lerp(r0, r1, i / 5) * 0.8 for i in range(6)], mat, segs=12, caps=(0.6, 0.6))


def _m_neck_top(sk, mat):
    _m_throat(sk, mat, side=1, k=0.3, name="neck_top")


def _m_eye_patch(sk, mat, sides=(1,)):
    """A dark patch over one eye and ear (cow)."""
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        for sx in sides:
            P.ell("patch", g["c"] + Vector((sx * g["hw"] * 0.45, -g["hl"] * 0.2, g["hh"] * 0.3)),
                  (g["hw"] * 0.62, g["hl"] * 0.75, g["hh"] * 0.75), mat, segs=14, rings=8)


def _m_rump(sk, mat, size=1.0):
    y, zc, rx, rz, _, _ = sk.section(0.1)
    with attach(sk.B):
        P.ell("rump", (0, y + 0.012, zc + rz * 0.1), (rx * 0.75, rx * 0.5 * size, rz * 0.7 * size), mat)


def _m_patches(sk, mat, spots):
    """spots: [(u, theta, ru, rv)] on the torso surface; both sides mirror
    unless theta is given as a (left, right) pair."""
    with attach(sk.B):
        for u, th, ru, rv in spots:
            for t in ((th, 180 - th) if not isinstance(th, tuple) else th):
                p, n = sk.surface(u, t)
                m = Vector((0, 0, 1)).rotation_difference(n).to_matrix().to_4x4()
                P.ell("patch", p - n * 0.0095, (ru, rv, 0.012), mat, m=m @ P.rot(z=90), segs=14, rings=8)


def _m_hump(sk, mat, u=0.78, w=0.8, h=0.09, ln=0.16, lift=0.0):
    p, n = sk.surface(u, 90)
    y, zc, rx, rz, _, _ = sk.section(u)
    with attach(sk.B):
        P.ell("hump", (0, p.y, p.z - h * 0.35 + lift), (rx * w, ln, h), mat, segs=18, rings=10)


def _m_skirt(sk, mat, z_bot=0.08, top=0.0, flare=1.16, n=30, jag=0.05, width=1.0, wave=0, top_k=1.0):
    """Long hanging coat (body frame), z_bot in model space above ground."""
    b = sk.b
    y0, zc, rx, rz, _, _ = sk.section(0.5)
    zb = z_bot - sk.z0 + sk.p.bz
    with attach(sk.B):
        P.skirt("coat", (0, y0), max(b.chest[0], b.hips[0]) * 0.98 * width, b.torso * 0.44, zc + top, zb, mat, b.key,
                n=n, flare=flare, jag=jag, wave=wave, top_k=top_k)


def _m_tufts_side(sk, mat, n=10, length=0.05, r=0.025, z=-0.6):
    """A ragged fringe along both flanks, pointing down."""
    pts, nrm = [], []
    for i in range(n):
        u = 0.12 + 0.76 * i / (n - 1)
        for th in (-35, 215):
            p, nn = sk.surface(u, th)
            pts.append(p)
            nrm.append(Vector((nn.x * 0.4, 0, -1)))
    with attach(sk.B):
        P.tufts("fringe", pts, nrm, length, r, mat, sk.b.key)


def _m_shag(sk, mat, size=1.0, u=0.86):
    """A shaggy mass over the shoulders, neck and head base (bison)."""
    b = sk.b
    y, zc, rx, rz, _, _ = sk.section(u)
    nb = sk.neck_base()
    with attach(sk.B):
        c = Vector((0, (y + nb.y) / 2 - 0.01, zc + rz * 0.3))
        P.ell("shag", c, (rx * 1.12 * size, b.torso * 0.24 * size, rz * 1.18 * size), mat, segs=20, rings=12)
        pts, nrm = [], []
        for i in range(9):
            a = math.radians(-60 + 300 * i / 8)
            nv = Vector((math.cos(a) * 0.9, -0.4, math.sin(a) * 0.3 - 0.8)).normalized()
            pts.append(c + Vector((math.cos(a) * rx * 1.05 * size, -b.torso * 0.08, -rz * 0.85 * size)))
            nrm.append(nv)
        P.tufts("shag", pts, nrm, 0.05 * size, 0.03 * size, mat, b.key)


def _m_ruff(sk, mat, size=1.0):
    b = sk.b
    nb = sk.neck_base()
    q = P.bezier(nb, sk.neck_mid, sk.poll, 5)[1]
    r0, r1 = b.neck_r
    with attach(sk.B):
        P.ell("ruff", q, (r0 * 1.35 * size, r0 * 1.0 * size, r0 * 1.3 * size), mat, segs=18, rings=10)


def _m_mane(sk, mat, height=0.04, width=0.012, forelock=True):
    """Upright mane along the top of the neck (equines)."""
    b = sk.b
    nb = sk.neck_base()
    pts = P.bezier(nb, sk.neck_mid, sk.poll, 7)
    th = math.radians(b.neck_angle + sk.p.neck)
    up = Vector((0, math.sin(th), math.cos(th)))
    r0, r1 = b.neck_r
    top = [q + up * (P.lerp(r0, r1, i / 6) * 0.8 + height * 0.3) for i, q in enumerate(pts)]
    hs = [height * (0.5 + 0.5 * math.sin(math.pi * min(1, (i + 1) / 6))) for i in range(7)]
    hs[-1] = height * 0.6
    with attach(sk.B):
        P.fin("mane", top, hs, width, mat)


def _m_blaze(sk, mat, width=0.5):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        top_c = g["c"] + Vector((0, -g["hl"] * 0.45, g["hh"] * 0.62))
        top_t = g["tip"] + Vector((0, g["mh"] * 0.4, g["mh"] * 0.75))
        P.tube("blaze", [top_c, (top_c + top_t) / 2, top_t],
               [(g["hw"] * width * 0.7, g["hh"] * 0.25), (g["mw"] * width * 1.1, g["mh"] * 0.3),
                (g["mw"] * width * 0.9, g["mh"] * 0.3)], mat, segs=10, caps=(0.6, 0.6))


def _m_mask(sk, mat, kind="raccoon"):
    g = sk.head_geo()
    c, hw, hl, hh = g["c"], g["hw"], g["hl"], g["hh"]
    with attach(sk.B @ sk.H):
        for sx in (-1, 1):
            P.ell("mask", c + Vector((sx * hw * 0.62, -hl * 0.48, hh * 0.18)), (hw * 0.42, hl * 0.4, hh * 0.32), mat,
                  m=P.rot(y=-20 * sx), segs=12, rings=8)


def _m_glands(sk, mat, n=6, r=0.018, u0=0.2, u1=0.85):
    with attach(sk.B):
        for i in range(n):
            u = P.lerp(u0, u1, i / max(1, n - 1))
            p, nn = sk.surface(u, 90)
            P.gland(p + Vector((0, 0, r * 0.2)), r * (0.85 + 0.3 * math.sin(math.pi * i / max(1, n - 1))), mat)


def _m_throat_gland(sk, mat, r=0.03, wide=1.0, at=2):
    """A glowing gland on the throat; wide > 1 stretches it round the neck
    sides so it shows from above and from the side. at="jaw" seats it under
    the back of the jaw (short-necked bodies, where the neck is hidden)."""
    b = sk.b
    if at == "jaw":
        g = sk.head_geo()
        q = g["c"] + Vector((0, g["hl"] * 0.45, -g["hh"] * 1.15))
        with attach(sk.B @ sk.H):
            P.ell("gland_socket", q + Vector((0, 0, r * 0.45)), (r * 0.8 * wide, r * 1.35, r * 0.75), "fur_dark",
                  segs=12)
            P.ell("gland", q, (r * 0.95 * wide, r * 1.6, r * 0.9), mat, segs=12)
        return
    nb = sk.neck_base()
    q = P.bezier(nb, sk.neck_mid, sk.poll, 5)[at]
    th = math.radians(b.neck_angle + sk.p.neck)
    down = Vector((0, -math.sin(th), -math.cos(th)))
    with attach(sk.B):
        P.ell("gland_socket", q + down * b.neck_r[0] * 0.55, (r * 1.2 * wide, r * 1.5, r * 1.1), "fur_dark", segs=12)
        P.ell("gland", q + down * b.neck_r[0] * 0.75, (r * 0.95 * wide, r * 1.3, r * 0.9), mat, segs=12)


def _m_beard(sk, mat, size=1.0):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        b0 = g["tip"] + Vector((0, g["mh"] * 2.2, -g["mh"] * 1.1))
        P.cone("beard", b0 + Vector((0, 0, g["mh"] * 0.5)), b0 + Vector((0, 0.01, -0.06 * size)), 0.017 * size, mat,
               segs=8)


def _m_trunk(sk, mat):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        P.trunk(g["m0"] + Vector((0, -g["ml"] * 0.4, -g["mh"] * 0.2)), 1.0, mat, curl=sk.p.trunk * 0.5,
                lift=sk.p.trunk)


def _m_tusks(sk, mat, kind="elephant", size=1.0):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        for sx in (-1, 1):
            if kind == "elephant":
                base = g["m0"] + Vector((sx * g["mw"] * 0.95, -g["ml"] * 0.15, -g["mh"] * 0.8))
            else:
                base = g["tip"] + Vector((sx * g["mw"] * 0.8, g["mh"] * 0.9, -g["mh"] * 0.2))
            P.tusk(base, sx, size, mat, kind=kind)


def _m_folds(sk, mat, us=(0.78, 0.25), k=1.04):
    with attach(sk.B):
        for u in us:
            y, zc, rx, rz, _, _ = sk.section(u)
            P.tube("fold", [(0, y + 0.012, zc), (0, y - 0.012, zc)], [(rx * k, rz * k), (rx * k, rz * k)], mat,
                   segs=18, caps=(0.3, 0.3))


def _m_crest(sk, mat, n=9, length=0.05, r=0.02, u0=0.25, u1=0.95):
    pts, nrm = [], []
    for i in range(n):
        u = u0 + (u1 - u0) * i / (n - 1)
        p, nn = sk.surface(u, 90)
        pts.append(p)
        nrm.append(Vector((0, 0.35, 1)))
    with attach(sk.B):
        P.tufts("crest", pts, nrm, length * 1.0, r, mat, sk.b.key + "crest")


def _m_stripe(sk, mat):
    with attach(sk.B):
        for th in (-12, 192):
            pts = []
            for i in range(6):
                u = 0.22 + 0.58 * i / 5
                p, n = sk.surface(u, th, lift=-0.004)
                pts.append(p)
            P.tube("stripe", pts, [0.01] * 6, mat, segs=8, caps=(0.6, 0.6))


def _m_chest(sk, mat, size=1.0):
    """Pale bib on the chest and throat (red fox, husky)."""
    y, zc, rx, rz, _, _ = sk.section(0.93)
    with attach(sk.B):
        P.ell("bib", (0, y + 0.02, zc - rz * 0.1), (rx * 0.62 * size, 0.05 * size, rz * 0.8 * size), mat)


def _m_incisors(sk, mat):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        for sx in (-1, 1):
            b0 = g["tip"] + Vector((sx * g["mw"] * 0.22, g["mh"] * 0.1, -g["mh"] * 0.45))
            P.tube("incisor", [b0, b0 + Vector((0, -0.002, -0.035))], [(0.011, 0.006), (0.011, 0.005)], mat, segs=6,
                   caps=(0, 0.3), side=(1, 0, 0))


def _m_face_disc(sk, mat):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        P.ell("face", g["c"] + Vector((0, -g["hl"] * 0.62, -g["hh"] * 0.05)),
              (g["hw"] * 0.78, g["hl"] * 0.42, g["hh"] * 0.78), mat, segs=16, rings=10)


def _m_cheeks(sk, mat, size=1.0):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        for sx in (-1, 1):
            P.cone("cheek", g["c"] + Vector((sx * g["hw"] * 0.6, -g["hl"] * 0.1, -g["hh"] * 0.45)),
                   g["c"] + Vector((sx * g["hw"] * 1.55 * size, g["hl"] * 0.25, -g["hh"] * 0.95)),
                   0.03 * size, mat, segs=8)


def _m_nose_horns(sk, mat, size=1.0):
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        P.nose_horns(g["tip"] + Vector((0, g["mh"] * 0.9, g["mh"] * 0.7)), size, mat)


def _m_bands(sk, mat, us=(0.25, 0.37, 0.49, 0.61, 0.73), w=0.012):
    """Tabby stripes: thin dark bands over the back and flanks."""
    with attach(sk.B):
        for u in us:
            pts = []
            for th in range(-20, 201, 20):
                p, n = sk.surface(u, th, lift=-0.002)
                pts.append(p)
            P.tube("band", pts, [w] * len(pts), mat, segs=6, caps=(0.6, 0.6))


def _m_spots(sk, mat, n=10, r=0.014):
    R = P.rng(sk.b.key, "spots")
    with attach(sk.B):
        for i in range(n):
            u = R.uniform(0.15, 0.85)
            th = R.uniform(-30, 80)
            for t in (th, 180 - th):
                p, nn = sk.surface(u, t)
                P.ell("spot", p, (r, r, r * 0.6), mat, segs=8, rings=5)


def _m_wool(sk, mat, size=1.0, bump=0.3, n=30):
    """Wool cloud over the torso (sheep, alpaca)."""
    y, zc, rx, rz, _, _ = sk.section(0.5)
    b = sk.b
    with attach(sk.B):
        P.puff("wool", (0, y, zc), (max(b.chest[0], b.hips[0]) * 1.12 * size, b.torso * 0.5 * size, rz * 1.12 * size),
               mat, b.key, n=n, bump=bump)


def _m_neck_wool(sk, mat, size=1.0):
    b = sk.b
    nb = sk.neck_base()
    pts = P.bezier(nb, sk.neck_mid, sk.poll, 6)
    r0, r1 = b.neck_r
    with attach(sk.B):
        for i, q in enumerate(pts[:-1]):
            P.ell("neck_wool", q, (P.lerp(r0, r1, i / 5) * 1.25 * size,) * 3, mat, segs=12, rings=8)


MARKS = {
    "belly": _m_belly, "saddle": _m_saddle, "throat": _m_throat, "rump": _m_rump, "patches": _m_patches,
    "hump": _m_hump, "skirt": _m_skirt, "fringe": _m_tufts_side, "shag": _m_shag, "ruff": _m_ruff,
    "mane": _m_mane, "blaze": _m_blaze, "mask": _m_mask, "glands": _m_glands, "throat_gland": _m_throat_gland,
    "beard": _m_beard, "trunk": _m_trunk, "tusks": _m_tusks, "folds": _m_folds, "crest": _m_crest,
    "stripe": _m_stripe, "chest": _m_chest, "incisors": _m_incisors, "face_disc": _m_face_disc,
    "cheeks": _m_cheeks, "spots": _m_spots, "wool": _m_wool, "neck_wool": _m_neck_wool,
    "neck_top": _m_neck_top, "eye_patch": _m_eye_patch, "nose_horns": _m_nose_horns, "bands": _m_bands,
}


def build_marks(sk):
    for m in sk.b.marks:
        MARKS[m[0]](sk, *m[1:])


def quadruped(spec, pose):
    """The default model: torso, legs, neck, head, tail, then marks."""
    sk = Skel(spec, pose)
    build_torso(sk)
    build_legs(sk)
    build_neck(sk)
    build_head(sk)
    build_tail(sk)
    build_marks(sk)
    return sk


# ------------------------------------------------------------------ poses
Gait = namedtuple("Gait", "stride lift drop swing base nod")
_GAIT = {}


def _walk(spec, i, g):
    ph = (i - 1) / TROT_N
    feet = {}
    for leg in (spec.legs or quadruped_legs)(spec):
        q = (ph + leg.phase) % 1.0
        if q < DUTY:      # planted: slides back under the body at stride / DUTY per cycle
            feet[leg.name] = (P.lerp(-g.stride / 2, g.stride / 2, q / DUTY), 0.0)
        else:
            s = (q - DUTY) / (1 - DUTY)
            feet[leg.name] = (P.lerp(g.stride / 2, -g.stride / 2, _smooth(s)),
                              g.lift * (leg.l1 + leg.l2) * math.sin(math.pi * s))
    c2 = math.cos(4 * math.pi * ph)        # +1 contact (walk01/05), -1 passing (walk03/07)
    k = (1 + c2) / 2                       # 1 at contact, 0 at passing
    s1 = math.sin(2 * math.pi * ph)
    rock = 1.0 if g.swing else 0.0         # body pitch and roll: quadrupeds (other layouts add their own)
    pitch, head = WALK_PITCH * k * rock, WALK_NOD * c2 * g.nod
    # a trunk hangs under the nodding head like a pendulum instead of turning with it into the ground
    trunk = pitch + head + TRUNK_SWING * k if any(m[0] == "trunk" for m in spec.marks) else 0.0
    return Pose(bz=g.base + g.drop * k, pitch=pitch, neck=-WALK_NECK * c2 * g.nod, head=head, trunk=trunk,
                tail=5 * math.sin(4 * math.pi * ph), sway=8 * s1, roll=WALK_ROLL * s1 * rock, swing=g.swing,
                feet=feet)


def _reach(sk, leg):
    """How far (body lengths) the leg's foot target lies beyond its reach."""
    return (sk.foot_target(leg) - sk.joint_world(leg)).length - 0.999 * (leg.l1 + leg.l2)


HANG = 0.01   # body lengths: a leg that misses the ground standing by more than this is built short (hidden legs)


def _shortfall(spec, pose, allow):
    """How far (body lengths) the worst planted foot falls short of its
    target, past what `allow` (per leg) lets it hang."""
    sk = Skel(spec, pose)
    worst = 0.0
    for leg in sk.legs:
        if pose.feet.get(leg.name, (0.0, 0.0))[1] > 1e-9:
            continue
        worst = max(worst, _reach(sk, leg) - allow[leg.name])
    return worst


def gait(spec):
    """The walk's stride, swing lift (fraction of leg length), body dip at
    contact and upper-leg swing for one Body (cached per spec object).

    Quadrupeds stride STRIDE_K x their lowest shoulder/hip height, the
    whole leg turning about the shoulder. Every planted foot must stay in
    reach: the body sits `base` lower at passing if it has to, and dips at
    contact just enough (at least BOB_MIN); a stride that would need more
    than DROP_MAX is narrowed."""
    hit = _GAIT.get(id(spec))
    if hit and hit[0] is spec:
        return hit[1]
    legs = (spec.legs or quadruped_legs)(spec)
    quad = spec.legs is None
    if not legs:
        g = Gait(0.0, 0.0, 0.0, 0.0, 0.0, spec.nod)
    else:
        if spec.stride is not None:
            stride = spec.stride
        elif quad:
            z0 = spec.height - spec.chest[1]
            stride = STRIDE_K * min(z0 + leg.root[2] - leg.r[3] for leg in legs)
        else:
            stride = STRIDE_LEG * min(leg.l1 + leg.l2 for leg in legs)
        g = _fit(spec, Gait(stride, LIFT_K[0] if quad else LIFT_K[1], BOB_MIN, 1.0 if quad else 0.0, 0.0, spec.nod),
                 spec.dip)
        if quad:
            g = _calm(spec, g)
    _GAIT[id(spec)] = (spec, g)
    return g


def _nose_span(spec, g):
    nz = [Skel(spec, _walk(spec, i, g)).nose_world().z for i in range(1, TROT_N + 1)]
    return max(nz) - min(nz)


def _calm(spec, g):
    """Scale the nod down until the nose travels at most NOSE_MAX of the
    withers height per cycle (the nod moves no foot: the fit holds)."""
    while g.nod > 0.2 and _nose_span(spec, g) > NOSE_MAX * spec.height:
        g = g._replace(nod=g.nod - 0.05)
    return g


def _fit(spec, g, dip=True):
    """The widest stride (from g.stride down) whose planted feet all stay
    in reach with a passing dip and a contact dip <= DROP_MAX (dip=False:
    other leg layouts keep the BOB_MIN dip and only narrow the stride)."""
    sk = Skel(spec, Pose())
    rest = {leg.name: _reach(sk, leg) for leg in sk.legs}
    # A leg that misses the ground standing hangs: a quadruped's only when
    # it is built short (by more than HANG; a small miss is fixed by the
    # dip), other layouts' always (they cannot dip).
    allow = {n: (r if r > (HANG if dip else 0.0) else 0.0) for n, r in rest.items()}

    def worst(gg, frames):
        return max(_shortfall(spec, _walk(spec, i, gg), allow) for i in frames)

    if not dip:
        for _ in range(12):
            if worst(g, range(1, TROT_N + 1)) <= 1e-4:
                break
            g = g._replace(stride=g.stride * 0.92)
        return g
    for _ in range(12):
        base = 0.0
        while base < DROP_MAX and worst(g._replace(base=base, drop=0.0), (3, 7)) > 1e-4:
            base += 0.001
        drop = BOB_MIN
        while drop <= DROP_MAX and worst(g._replace(base=base, drop=drop), range(1, TROT_N + 1)) > 1e-4:
            drop += 0.001
        if drop <= DROP_MAX and base < DROP_MAX:
            return g._replace(base=base, drop=drop)
        g = g._replace(stride=g.stride * 0.92)
    return g._replace(base=base, drop=min(drop, DROP_MAX))


def trot_pose(spec, i):
    """walk01..08 (ART §27.1, PLAY-022): diagonal pairs (fl+hr, fr+hl).
    walk01/05 contact: the leading pair lands, the other pushes off; all
    feet down, body at its lowest, nose and head dipped. walk03/07
    passing: the stance pair upright, the other pair swinging under the
    body; body back at its standing height (its rise peaks), head up.
    Planted feet move back under the body at one rate (gait_tiles)."""
    return _walk(spec, i, gait(spec))


def gait_tiles(spec, poses):
    """C3 `gait`: tiles an adult travels per 8-frame walk cycle, i.e. how
    far a planted foot moves back under the body in one cycle (the same in
    every facing). Measured from the walk frames' planted feet; walks with
    no feet (snake, crawler) give Body.travel. None without walk frames."""
    per = spec.travel
    if per is None:
        steps = []
        walk = [poses.get(n) for n in WALK_FRAMES]
        if any(p is None for p in walk):
            return None
        for n in walk[0].feet:
            for i in range(TROT_N):
                a, b = walk[i].feet.get(n), walk[(i + 1) % TROT_N].feet.get(n)
                if a and b and a[1] <= 1e-9 and b[1] <= 1e-9:
                    steps.append(b[0] - a[0])
        if not steps:
            return None
        per = TROT_N * float(np.median(steps))
    return round(per * spec.length * C.FIGURE_SCALE, 4)


def graze_poses(spec):
    """graze1 (reaching down) and graze2 (nose at the grass): the neck angle
    is solved so the nose reaches the ground."""
    out = []
    for frac, head, pitch, bz in ((0.45, 30, 3, 0.0), (0.0, 48, 6, 0.01)):
        best, bd = None, 1e9
        target = 0.03 + frac * spec.height * 0.6
        for dn in range(0, -121, -3):
            p = Pose(neck=dn, head=head, pitch=pitch, bz=bz, ty=-0.01, ears=10, tail=-6)
            z = Skel(spec, p).nose_world().z
            if abs(z - target) < bd:
                best, bd = p, abs(z - target)
        if best.neck <= -110 and bd > 0.04:
            best = replace(best, pitch=best.pitch + 10, bz=best.bz + 0.02, head=best.head + 15)
        out.append(best)
    return out


def flinch_poses(spec):
    return [Pose(ty=0.05, bz=0.02, pitch=-7, neck=10, head=-18, yaw=18, ears=40, tail=-15,
                 feet={"fl": (0.04, 0.03), "fr": (0.02, 0.0), "hl": (0.03, 0.0), "hr": (0.04, 0.02)}),
            Pose(ty=0.025, bz=0.01, pitch=-3, neck=5, head=-8, yaw=8, ears=20, tail=-8)]


def _scaled(p, k):
    """Shrink a pose's travel for short-legged bodies (k = height / 0.55)."""
    if k >= 0.999:
        return p
    return replace(p, ty=p.ty * k, bz=p.bz * k, feet={n: (a * k, b * k) for n, (a, b) in p.feet.items()},
                   paws={n: (a * k, b * k) for n, (a, b) in p.paws.items()})


def attack_poses(spec, kind):
    """Four keyframes per natural weapon (ART §27.2). 3 is the strike.
    Travel is authored for a 0.55-tall body and shrinks for shorter legs."""
    k = min(1.0, spec.height / 0.55)
    return [_scaled(p, k) for p in _attack_poses(spec, kind)]


def _attack_poses(spec, kind):
    T = spec.torso
    hind_y = T / 2 - T * 0.2
    if kind == "bite":
        return [Pose(ty=0.03, bz=0.05, pitch=6, neck=-12, head=4, ears=35, tail=8,
                     feet={"fl": (0.02, 0), "fr": (0.03, 0)}),
                Pose(ty=-0.08, bz=0.02, pitch=-2, neck=-6, head=-14, jaw=42, ears=40, tail=12,
                     feet={"fl": (-0.1, 0.05), "fr": (-0.06, 0.0), "hl": (0.06, 0), "hr": (0.04, 0)}),
                Pose(ty=-0.12, bz=0.03, pitch=4, neck=-10, head=8, jaw=6, ears=40, tail=10,
                     feet={"fl": (-0.12, 0), "fr": (-0.1, 0.0), "hl": (0.07, 0), "hr": (0.06, 0.02)}),
                Pose(ty=-0.04, bz=0.02, pitch=2, neck=-4, head=2, jaw=10, ears=20,
                     feet={"fl": (-0.04, 0), "fr": (-0.03, 0)})]
    if kind == "kick":
        return [Pose(ty=-0.02, bz=0.02, pitch=7, neck=-8, head=10, ears=30,
                     feet={"fl": (-0.03, 0), "fr": (-0.04, 0), "hl": (0.0, 0), "hr": (0.02, 0)}),
                Pose(ty=-0.02, pitch=12, neck=-10, head=12, ears=30, tail=15,
                     feet={"fl": (-0.03, 0), "fr": (-0.04, 0), "hl": (0.1, 0.07), "hr": (0.12, 0.08)}),
                Pose(ty=-0.01, bz=-0.01, pitch=17, neck=-4, head=-6, ears=40, tail=25,
                     feet={"fl": (-0.04, 0), "fr": (-0.05, 0), "hl": (0.24, 0.17), "hr": (0.26, 0.19)}),
                Pose(ty=0.0, bz=0.015, pitch=4, neck=-2, head=4, ears=15,
                     feet={"hl": (0.05, 0.02), "hr": (0.06, 0.02)})]
    if kind == "gore":
        return [Pose(ty=0.04, bz=0.03, pitch=2, neck=-25, head=30, ears=25, tail=10,
                     feet={"fl": (0.02, 0), "fr": (0.03, 0), "hl": (0.0, 0), "hr": (0.01, 0)}),
                Pose(ty=-0.07, bz=0.04, pitch=7, neck=-32, head=42, ears=30, tail=20,
                     feet={"fl": (-0.1, 0.05), "fr": (-0.05, 0), "hl": (0.05, 0), "hr": (0.07, 0.03)}),
                Pose(ty=-0.12, bz=0.0, pitch=-2, neck=-12, head=18, ears=30, tail=22,
                     feet={"fl": (-0.13, 0.0), "fr": (-0.11, 0.04), "hl": (0.07, 0), "hr": (0.06, 0)}),
                Pose(ty=-0.05, bz=0.02, pitch=2, neck=-8, head=10, ears=10,
                     feet={"fl": (-0.05, 0), "fr": (-0.04, 0)})]
    if kind == "headbutt":
        return [Pose(ty=0.05, bz=0.0, pitch=-8, pivot=hind_y, neck=12, head=-12, ears=20,
                     feet={"fl": (0.0, 0.06), "fr": (0.02, 0.03)}),
                Pose(ty=0.0, bz=0.03, pitch=4, neck=-18, head=38, ears=25, tail=10,
                     feet={"fl": (-0.04, 0.02)}),
                Pose(ty=-0.09, bz=0.025, pitch=6, neck=-22, head=50, ears=30, tail=15,
                     feet={"fl": (-0.11, 0), "fr": (-0.08, 0.03), "hl": (-0.02, 0), "hr": (0.0, 0)}),
                Pose(ty=-0.05, bz=0.02, pitch=3, neck=-8, head=15, ears=10,
                     feet={"fl": (-0.05, 0), "fr": (-0.04, 0)})]
    if kind == "claw":
        return [Pose(ty=0.02, bz=0.03, pitch=-8, pivot=hind_y, neck=-5, head=0, ears=35, tail=8,
                     paws={"fl": (-0.03, -0.14)}),
                Pose(ty=0.0, bz=0.0, pitch=-24, pivot=hind_y, neck=0, head=-8, jaw=20, ears=40, tail=12,
                     paws={"fl": (-0.06, -0.04), "fr": (-0.04, -0.1)}),
                Pose(ty=-0.07, bz=0.01, pitch=-14, pivot=hind_y, neck=-8, head=10, jaw=30, ears=40, tail=10,
                     paws={"fl": (-0.2, -0.13), "fr": (-0.06, -0.08)}),
                Pose(ty=-0.04, bz=0.02, pitch=-3, pivot=hind_y, neck=-4, head=4, ears=20,
                     feet={"fl": (-0.06, 0.02)})]
    if kind == "peck":
        return [Pose(ty=0.02, neck=10, head=-10), Pose(ty=-0.04, neck=-30, head=40, pitch=6),
                Pose(ty=-0.06, neck=-45, head=60, pitch=10), Pose(ty=-0.02, neck=-10, head=10)]
    raise ValueError("attack kind %r" % kind)


def corpse_pose(spec, v):
    """Lying dead: v0 legs relaxed, v1 curled with the head turned in,
    v2 stiff with the legs stretched."""
    if v == 1:
        return Pose(neck=-25, head=20, yaw=45, jaw=6, tail=-20, ears=50, dead=1,
                    paws={"fl": (0.05, -0.08), "fr": (0.1, -0.12), "hl": (-0.14, -0.06), "hr": (-0.1, -0.12)})
    if v == 2:
        return Pose(neck=-30, head=35, yaw=-30, jaw=8, tail=-25, ears=50, dead=2, roll=0,
                    paws={"fl": (0.08, -0.1), "fr": (0.02, -0.16), "hl": (-0.16, -0.12), "hr": (-0.08, -0.18)})
    return Pose(neck=-35, head=5, jaw=4, ears=40, tail=-5, dead=0,
                paws={"fl": (-0.06, -0.3), "fr": (0.02, -0.28), "hl": (0.08, -0.3), "hr": (0.0, -0.29)})


def rest_pose(spec):
    """Lying down (resting / sleeping): the belly on the ground, legs folded
    under (the IK knees bend out at ground level), neck lowered and the head
    carried low, ears relaxed, tail slack. Archetypes with a different
    anatomy override it through Body.poses["rest"]."""
    cd = spec.chest[1]
    drop = max(0.0, spec.height - 2 * cd - 0.012 + spec.belly * 0.55)
    return Pose(bz=drop, ty=0.0, pitch=0.0, neck=-22, head=14, ears=26, tail=-14, sway=0)


def only_frames():
    """ANIMAL_FRAMES=rest renders just the rest frame of each set (canvas
    margins still come from the full pose set, so it matches the others)."""
    v = os.environ.get("ANIMAL_FRAMES", "").strip()
    return v if v and v != "static" else None


def frame_poses(spec, static=False):
    """Ordered {frame name or None: Pose} for one sprite set."""
    out = {None: Pose()}
    if static:
        return out
    for i in range(1, TROT_N + 1):
        out["walk%02d" % i] = trot_pose(spec, i)
    if not (spec.poses and "%s1" % spec.attack in spec.poses):   # archetypes may author their own set
        for i, p in enumerate(attack_poses(spec, spec.attack), 1):
            out["%s%d" % (spec.attack, i)] = p
    for name, p in zip(GRAZE, graze_poses(spec)):
        out[name] = p
    for name, p in zip(FLINCH, flinch_poses(spec)):
        out[name] = p
    out["rest"] = rest_pose(spec)
    if spec.poses:
        out.update(spec.poses)
    return out


# ------------------------------------------------------------- rendering
def build(spec, pose):
    """Build one posed model at unit size, then scale it to the species
    and stand it on the tile centre (still facing south, unleaned)."""
    (spec.model or quadruped)(spec, pose)
    objs = S.objects()
    S.transform(objs, Matrix.Translation((CX, CY, 0)) @ Matrix.Scale(spec.length, 4))
    return objs


def lean_matrix():
    t = math.radians(LEAN_DEG)
    feet = Matrix.Translation((0.5, CY, 0))
    return feet @ Matrix.Rotation(-t, 4, "X") @ Matrix.Scale(C.FIGURE_SCALE, 4) @ Matrix.Translation((-0.5, -CY, 0))


def _points():
    """Every mesh vertex on the stage, world space (N x 3)."""
    out = []
    for ob in S.objects():
        n = len(ob.data.vertices)
        if not n:
            continue
        co = np.empty(n * 3, np.float32)
        ob.data.vertices.foreach_get("co", co)
        m = np.array(ob.matrix_world)
        out.append(co.reshape(n, 3) @ m[:3, :3].T + m[:3, 3])
    return np.concatenate(out) if out else np.zeros((1, 3))


def _screen_box(v):
    """Sheared screen extents (lo, hi) of world points, in tiles."""
    s = np.stack([v[:, 0], v[:, 1] + C.SHEAR_K * v[:, 2]], axis=1)
    return s.min(axis=0), s.max(axis=0)


def _extents():
    return _screen_box(_points())


def _facing_matrix(f):
    """stage.face for a 1x1 footprint, then the figure lean."""
    m = (Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(S.FACINGS[f]), 4, "Z")
         @ Matrix.Translation((-0.5, 0.5, 0)))
    return np.array(lean_matrix() @ m)


def outline_px(spec, ppt):
    """The baked outline width at this density: Body.outline (or the
    contracts default) is in pixels at contracts.PPT, scaled by ppt so it
    keeps its on-screen width."""
    base = C.OUTLINE_PX if spec.outline is None else spec.outline
    return max(1, int(round(base * ppt / C.PPT)))


def _canvas(spec, lo, hi, symmetric):
    """Margins (tiles, left/right/top/bottom around the 1x1 footprint; each
    an even pixel count at the returned ppt, and negative where the canvas
    is tighter than the tile) and ppt (C2) for screen extents lo..hi.

    ppt is contracts.PPT unless the canvas's long side would exceed
    CANVAS_CAP px there; then the even integer <= PPT * CAP / long side
    (never below MIN_PPT). symmetric: equal left and right margins, so the
    east sprite mirrored about its canvas centre is the west sprite."""
    def margins(ppt):
        pad = (outline_px(spec, ppt) + PAD_PX) / ppt
        l, r = -lo[0] + pad, hi[0] - 1 + pad
        if symmetric:
            l = r = max(l, r)
        return [int(math.ceil(v * ppt / 2 - 1e-6)) * 2 for v in (l, r, hi[1] + pad, -1 - lo[1] + pad)]

    ppt = C.PPT
    m = margins(ppt)
    long_px = max(m[0] + ppt + m[1], m[2] + ppt + m[3])
    if long_px > CANVAS_CAP:
        ppt = max(MIN_PPT, int(C.PPT * CANVAS_CAP / long_px) // 2 * 2)
        m = margins(ppt)
    return tuple(v / ppt for v in m), ppt


def _probe(spec, poses):
    """Per facing, the screen extents (lo, hi) of every pose: each pose is
    built once and turned to each facing in numpy."""
    ext = {f: (np.full(2, 1e9), np.full(2, -1e9)) for f in FACINGS}
    mats = {f: _facing_matrix(f) for f in FACINGS}
    for pose in poses.values():
        build(spec, pose)
        v = _points()
        S.clear()
        for f, M in mats.items():
            a, b = _screen_box(v @ M[:3, :3].T + M[:3, 3])
            ext[f] = (np.minimum(ext[f][0], a), np.maximum(ext[f][1], b))
    return ext


def canvases(spec):
    """{facing: (margins, ppt)}: one canvas per facing that holds all of
    that facing's frames (C2); east is symmetric about the tile."""
    return {f: _canvas(spec, lo, hi, f == "east") for f, (lo, hi) in _probe(spec, frame_poses(spec)).items()}


def render_species(spec, static=False, suffix="", corpses=True):
    """Render the full sprite set for one Body (and its female set).
    The canvases always come from the full frame set, so a static or
    single-frame render matches the full one."""
    out = []
    full = frame_poses(spec)
    boxes = canvases(spec)
    walk = gait_tiles(spec, full)
    poses = frame_poses(spec, static)
    only = only_frames()
    if only:
        poses = {n: p for n, p in poses.items() if n in only.split(",")}
        corpses = False
    base = spec.key + suffix
    for f in FACINGS:
        margins, ppt = boxes[f]
        for fr, pose in poses.items():
            build(spec, pose)
            S.face(S.objects(), f, (1, 1))
            S.transform(S.objects(), lean_matrix())
            extra = {"species": spec.key, "facing": f}
            if suffix:
                extra["sex"] = "female"
            if fr:
                extra["frame"] = fr
            if walk:
                extra["gait"] = walk
            name = "%s_%s" % (base, f) + ("_" + fr if fr else "")
            out.append(S.render_asset("animal", name, zmax=0.0, margins=margins, extra=extra,
                                      outline_width=outline_px(spec, ppt), ppt=ppt))
    if corpses and not static and not suffix:
        out += render_corpses(spec)
    if spec.female is not None:
        out += render_species(spec.female, static, suffix="_f", corpses=False)
    return out


CORPSE_ROLL = {0: (100.0, -90.0), 1: (96.0, 72.0), 2: (-84.0, 122.0)}
POOL_ALPHA = 0.7


def _translucent(name, alpha):
    """The palette material at a fixed alpha (blended), for the dried pool."""
    import bpy
    key = "rwg_%s@a%02d" % (name, int(alpha * 100))
    m = bpy.data.materials.get(key)
    if m:
        return m
    m = S.mat(name).copy()
    m.name = key
    nt = m.node_tree
    out = next(n for n in nt.nodes if n.type == "OUTPUT_MATERIAL")
    em = out.inputs["Surface"].links[0].from_node
    tr = nt.nodes.new("ShaderNodeBsdfTransparent")
    mix = nt.nodes.new("ShaderNodeMixShader")
    mix.inputs["Fac"].default_value = alpha
    nt.links.new(tr.outputs[0], mix.inputs[1])
    nt.links.new(em.outputs[0], mix.inputs[2])
    nt.links.new(mix.outputs[0], out.inputs["Surface"])
    if hasattr(m, "surface_render_method"):
        m.surface_render_method = "BLENDED"
    return m


def _corpse(spec, v):
    pose = corpse_pose(spec, v)
    (spec.model or quadruped)(spec, pose)
    objs = S.objects()
    roll, spin = CORPSE_ROLL[v]
    k = spec.length * C.FIGURE_SCALE * spec.corpse_scale
    S.transform(objs, Matrix.Rotation(math.radians(spin), 4, "Z") @ Matrix.Rotation(math.radians(roll), 4, "Y")
                @ Matrix.Scale(k, 4))
    pts = np.concatenate([np.array([ob.matrix_world @ vv.co for vv in ob.data.vertices]) for ob in objs])
    lo, hi = pts.min(axis=0), pts.max(axis=0)
    S.transform(objs, Matrix.Translation((0.5 - (lo[0] + hi[0]) / 2, -0.5 - (lo[1] + hi[1]) / 2, -lo[2])))
    w, d = hi[0] - lo[0], hi[1] - lo[1]
    s = spec.length
    if spec.pool:
        pool = P.ell("pool", (0.5 + 0.04 * s * (v - 1), -0.5 + 0.02 * s, 0.0),
                     (0.1 * s + 0.024, 0.066 * s + 0.018, 0.004), spec.pool, segs=22, rings=6)
        pool.material_slots[0].material = _translucent(spec.pool, POOL_ALPHA)
    return _extents()


def render_corpses(spec):
    """Corpses v0..v2: each its own tight canvas and ppt (C2)."""
    out = []
    for v in range(CORPSE_VARIANTS):
        margins, ppt = _canvas(spec, *_corpse(spec, v), False)
        out.append(S.render_asset("item", "corpse_%s_v%d" % (spec.key, v), zmax=0.0, margins=margins,
                                  outline_width=outline_px(spec, ppt), ppt=ppt,
                                  extra={"item": "%s corpse" % spec.key.replace("_", " "), "species": spec.key,
                                         "variant": v}))
    return out
