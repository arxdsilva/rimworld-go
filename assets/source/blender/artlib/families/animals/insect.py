"""Hive insects (ART-011): ORIGINAL designs, not RimWorld's megascarab,
megaspider or spelopede.

- hive_beetle  (beetle):  squat, domed iridescent dark-teal carapace split
                          down the middle, one forward horn, six short legs;
                          tripod gait.
- hive_brute   (brute):   hulking rust-brown armour, front body raised on
                          four legs, two big scythe forelimbs (six limbs in
                          all), heavy banded abdomen, short mandibles.
- hive_crawler (crawler): nine repeating ochre-and-black armoured segments,
                          eighteen short legs, curved pincers; the body
                          undulates and the legs ripple front to back.

All three use `model=` and `poses=` on rig.Body and rig.Skel for the body
frame and two-bone leg IK; eyes are tiny emissive amber dots.
"""
import math
from dataclasses import dataclass, replace

from mathutils import Matrix, Vector

from .. import animal_roster as R
from ... import stage as S
from . import parts as P
from . import rig
from .bird import basis, lerp, oell, render
from .rig import Body, Leg, Pose, Skel, attach, ik2

ARCHETYPES = ("beetle", "brute", "crawler")

Z0 = 0.15      # body-frame origin height (Body.height - chest depth)


def smooth(t):
    t = max(0.0, min(1.0, t))
    return t * t * (3 - 2 * t)


def Z(z):
    """Standing-height z -> body-frame z."""
    return z - Z0


def flip_dead(p):
    """Dead insects lie on their backs: cancel the rig's side roll and turn
    the model nearly upside down, so the legs curl up."""
    roll = rig.CORPSE_ROLL[p.dead][0]
    S.transform(S.objects(), Matrix.Rotation(math.radians(172 - roll), 4, "Y"))


def dead_target(sk, leg):
    j = sk.joint_world(leg)
    sx = 1 if leg.joint[0] > 0 else -1
    k = {0: 0.55, 1: 0.4, 2: 0.7}[sk.p.dead]
    return Vector((j.x + sx * 0.05, j.y + (0.03 if leg.name[0] == "h" else -0.02), j.z - (leg.l1 + leg.l2) * k))


def insect_leg(sk, leg, mat_hi=None):
    joint = sk.B @ Vector(leg.joint)
    root = sk.B @ Vector(leg.root)
    tgt = dead_target(sk, leg) if sk.p.dead >= 0 else sk.foot_target(leg)
    knee, foot = ik2(joint, tgt, leg.l1, leg.l2, leg.bend)
    r0, r1, r2, r3 = leg.r
    P.tube("leg", [root, joint, knee], [r0, r1, r2], leg.mat, segs=7, caps=(1, 0.8))
    P.tube("tibia", [knee, foot], [r2 * 0.95, r3], leg.lower or leg.mat, segs=7, caps=(0.8, 0.3))
    d = (foot - knee).normalized()
    P.cone("tip", foot - d * r3, foot + d * r3 * 2.2, r3 * 1.05, "chitin_black", segs=5)


def eye(c, r, side=0):
    P.ell("eye", c, (r, r, r), "hive_glow", segs=6, rings=4)


def insect_legs(names, joints, feet, l1, l2, bend_up, r, mat, lower, phases, front=(False,) * 8):
    legs = []
    for i, n in enumerate(names):
        sx = 1 if n[1] == "l" else -1
        legs.append(Leg(n, (sx * joints[i][0] * 0.6, joints[i][1], joints[i][2]),
                        (sx * joints[i][0], joints[i][1], joints[i][2]), l1[i], l2[i],
                        (sx * bend_up[0], bend_up[1], bend_up[2]), (sx * feet[i][0], feet[i][1]), r[i], mat,
                        lower, "claw", "chitin_black", phases[i], front[i]))
    return legs


# ================================================================== beetle
def beetle_legs(b):
    ys = (-0.2, 0.0, 0.2)
    names = ("fl", "fr", "ml", "mr", "hl", "hr")
    ph = {"fl": 0.0, "mr": 0.0, "hl": 0.0, "fr": 0.5, "ml": 0.5, "hr": 0.5}
    rows = {"f": ys[0], "m": ys[1], "h": ys[2]}
    joints = [(0.15, rows[n[0]], Z(0.115)) for n in names]
    feet = [(0.27 + (0.03 if n[0] != "m" else 0.0), rows[n[0]] + {"f": -0.05, "m": 0.0, "h": 0.06}[n[0]]) for n in names]
    rr = (0.03, 0.026, 0.019, 0.013)
    return insect_legs(names, joints, feet, [0.09] * 6, [0.1] * 6, (0.5, 0.0, 1.0), [rr] * 6, "chitin_teal_dark",
                       "chitin_black", [ph[n] for n in names])


def beetle_model(spec, p):
    sk = Skel(spec, p)
    open_ = max(0.0, p.ears)
    with attach(sk.B):
        # elytra: two domed halves split down the middle
        cy, rx, ry, rz = 0.13, 0.108, 0.27, 0.16
        for sx in (-1, 1):
            M = Matrix.Translation((0, -0.08, Z(0.2))) @ Matrix.Rotation(math.radians(-sx * open_ * 0.5), 4, "Y")
            with attach(M):
                oell("elytron", Vector((sx * 0.092, cy, 0.0)), (0, 1, 0), (rx, ry, rz), "chitin_teal", segs=18, rings=11)
                for k, ur in enumerate((-0.55, 0.0, 0.55)):
                    pts = []
                    for i in range(11):
                        t = -0.9 + 1.8 * i / 10
                        h = 1 - ur * ur - t * t
                        if h < 0.03:
                            continue
                        pts.append(Vector((sx * 0.092 + rx * ur, cy + ry * t, rz * math.sqrt(h) + 0.004)))
                    if len(pts) > 2:
                        P.tube("ridge", pts, [0.0085] * len(pts), "chitin_violet" if k != 1 else "chitin_black", segs=6,
                               caps=(0.4, 0.4))
        if open_ < 6:
            pts = []
            for i in range(11):
                t = -0.5 + 1.0 * i / 10
                pts.append(Vector((0, -0.08 + cy + ry * t, Z(0.2) + rz * math.sqrt(max(0.0, 0.274 - t * t)) + 0.002)))
            P.tube("seam", pts, [0.0065] * 11, "chitin_black", segs=6, caps=(0.4, 0.4))
        # pronotum shield and underside
        oell("pronotum", Vector((0, -0.26, Z(0.19))), (0, 1, 0), (0.15, 0.1, 0.1), "chitin_teal", segs=14, rings=9)
        oell("pronotum_rim", Vector((0, -0.27, Z(0.16))), (0, 1, 0), (0.16, 0.11, 0.06), "chitin_violet", segs=14, rings=7)
        oell("under", Vector((0, 0.04, Z(0.1))), (0, 1, 0), (0.15, 0.3, 0.05), "chitin_black", segs=12, rings=6)
    # head with horn and mandibles, pitched by Pose.head about the thorax
    piv = Vector((0, -0.32, Z(0.15)))
    MH = sk.B @ (Matrix.Translation(piv) @ Matrix.Rotation(math.radians(p.head), 4, "X") @ Matrix.Translation(-piv))
    with attach(MH):
        c = Vector((0, -0.39, Z(0.15)))
        oell("head", c, (0, 1, 0), (0.075, 0.06, 0.058), "chitin_black", segs=12, rings=8)
        pts = P.turtle(c + Vector((0, -0.03, 0.04)), (0, -0.35, 0.95), 0.3, 9, (1, 0, 0), 60)
        rad = [lerp(0.034, 0.005, (i / 9) ** 0.9) for i in range(10)]
        P.tube("horn", pts[:6], rad[:6], "chitin_black", segs=8, caps=(0.5, 0.0))
        P.tube("horntip", pts[5:], rad[5:], "chitin_bone", segs=8, caps=(0.0, 0.3))
        oell("hornbase", c + Vector((0, 0.0, 0.05)), (0, -1, 0.5), (0.05, 0.04, 0.035), "chitin_violet", segs=10, rings=6)
        for sx in (-1, 1):
            eye(c + Vector((sx * 0.062, -0.02, 0.025)), 0.012)
            a = math.radians(14 + max(0.0, p.jaw) * 0.9)
            d = Vector((sx * math.sin(a), -math.cos(a), 0))
            b0 = c + Vector((sx * 0.03, -0.055, -0.02))
            pts = [b0, b0 + d * 0.04, b0 + d * 0.07 + Vector((-sx * 0.03, 0, 0)) * (1 - max(0.0, p.jaw) / 60)]
            P.tube("mandible", pts, [0.014, 0.011, 0.002], "chitin_black", segs=6, caps=(0.5, 0.3))
    for leg in sk.legs:
        insect_leg(sk, leg)
    if p.dead >= 0:
        flip_dead(p)
    return sk


def beetle_poses(spec):
    out = {}
    for i in range(1, 9):
        out["walk%02d" % i] = replace(rig.trot_pose(spec, i), head=3 * math.sin(2 * math.pi * (i - 1) / 8),
                                      tail=0, sway=0, neck=0)
    out["bite1"] = Pose(ty=0.03, pitch=-5, head=-10, jaw=0)
    out["bite2"] = Pose(ty=-0.04, pitch=3, head=8, jaw=38, feet={"fl": (-0.05, 0.0), "fr": (-0.03, 0.0)})
    out["bite3"] = Pose(ty=-0.1, pitch=7, head=26, jaw=8, feet={"fl": (-0.09, 0.0), "fr": (-0.07, 0.01),
                                                                  "ml": (-0.03, 0.0), "mr": (-0.02, 0.0)})
    out["bite4"] = Pose(ty=-0.03, pitch=2, head=6, jaw=22)
    out["graze1"] = Pose(ty=-0.01, pitch=4, head=24)
    out["graze2"] = Pose(ty=-0.02, pitch=7, head=40, jaw=10)
    out["flinch1"] = Pose(ty=0.05, bz=0.01, pitch=-9, head=-14, ears=30,
                          feet={"fl": (0.05, 0.02), "fr": (0.04, 0.0), "ml": (0.03, 0), "mr": (0.03, 0.02)})
    out["flinch2"] = Pose(ty=0.025, pitch=-4, head=-6, ears=12)
    out["rest"] = Pose(bz=0.07, head=18, feet={n: (0.0, 0.0) for n in ("fl", "fr", "ml", "mr", "hl", "hr")})
    return out


# =================================================================== brute
@dataclass
class BrutePose(Pose):
    arm: float = 0.0     # scythe forelimbs: -1 cocked back .. 0 ready .. 1 slashed down and forward


def brute_legs(b):
    names = ("ml", "mr", "hl", "hr")
    ph = {"ml": 0.0, "hr": 0.0, "mr": 0.5, "hl": 0.5}
    ys = {"m": 0.02, "h": 0.2}
    joints = [(0.17, ys[n[0]], Z(0.2)) for n in names]
    feet = [(0.34, ys[n[0]] + (-0.04 if n[0] == "m" else 0.07)) for n in names]
    rr = (0.065, 0.058, 0.042, 0.03)
    return insect_legs(names, joints, feet, [0.17] * 4, [0.19] * 4, (0.5, 0.0, 1.0), [rr] * 4, "chitin_rust",
                       "chitin_rust_dark", [ph[n] for n in names])


def ring(cy, ry, rx, rz, cz, y, mat, k=1.04, w=0.012):
    """A thin band around an ellipsoid at body y (the cross-section's radius
    follows the ellipsoid)."""
    e = math.sqrt(max(0.0, 1 - ((y - cy) / ry) ** 2))
    if e < 0.2:
        return
    P.tube("plate", [(0, y - w, cz), (0, y + w, cz)], [(rx * e * k, rz * e * k)] * 2, mat, segs=18, caps=(0.5, 0.5))


def brute_arm(sk, sx, a, dead=False):
    s = Vector((sx * 0.18, -0.12, Z(0.4)))
    up = Vector((sx * 0.8, -0.5, 0.55)).normalized()
    if dead:
        up = Vector((sx * 0.7, -0.2, 0.4)).normalized()
    e = s + up * 0.22
    P.tube("humerus", [s, (s + e) / 2, e], [0.062, 0.056, 0.05], "chitin_rust", segs=9, caps=(0.8, 0.8))
    P.ell("elbow", e, (0.06, 0.06, 0.06), "chitin_rust_dark", segs=10, rings=7)
    a = max(-1.0, min(1.0, a))
    k0, k1, km = Vector((-sx * 0.35, -0.85, 0.15)), Vector((-sx * 0.6, -0.45, -0.75)), Vector((sx * 0.25, -0.1, 1.0))
    d = (k0.lerp(k1, a) if a >= 0 else k0.lerp(km, -a)).normalized()
    if dead:
        d = Vector((-sx * 0.2, -0.3, 0.9)).normalized()
    L = 0.54
    tip = e + d * L
    inward = Vector((-sx * 0.12, -0.02, -0.08 * (1 - a) - 0.04))
    ctrl = e + d * L * 0.5 + inward
    pts = P.bezier(e, ctrl, tip + inward * 0.6, 9)
    split = 3
    P.tube("forearm", pts[:split + 1], [0.056, 0.05, 0.045, 0.04], "chitin_rust", segs=8, caps=(0.4, 0.2))
    blade = pts[split:]
    n = len(blade)
    P.tube("scythe", blade, [(lerp(0.03, 0.004, i / (n - 1)), lerp(0.065, 0.005, (i / (n - 1)) ** 0.8)) for i in range(n)],
           "chitin_bone", segs=8, caps=(0.2, 0.0), side=(0, 0, 1))


def brute_model(spec, p):
    p = replace(p, pitch=p.pitch - (0 if p.dead >= 0 else 16), pivot=0.22 if p.dead < 0 else 0.0)
    sk = Skel(spec, p)
    arm = getattr(p, "arm", 0.0)
    with attach(sk.B):
        # heavy abdomen with plate bands, raised thorax, head
        ac = Vector((0, 0.24, Z(0.22)))
        oell("abdomen", ac, (0, 1, 0), (0.22, 0.27, 0.2), "chitin_rust", segs=20, rings=12)
        for y in (0.04, 0.12, 0.2, 0.28, 0.36, 0.44):
            ring(ac.y, 0.27, 0.22, 0.2, ac.z, y, "chitin_rust_dark")
        oell("dorsal", ac + Vector((0, -0.02, 0.1)), (0, 1, 0), (0.11, 0.2, 0.1), "chitin_rust_dark", segs=12, rings=7)
        tc = Vector((0, -0.06, Z(0.33)))
        oell("thorax", tc, (0, 1, 0), (0.17, 0.19, 0.17), "chitin_rust", segs=18, rings=10)
        for sx in (-1, 1):
            oell("pauldron", tc + Vector((sx * 0.15, -0.03, 0.08)), (0, 1, 0), (0.07, 0.11, 0.07), "chitin_rust_dark",
                 segs=10, rings=6)
        oell("plate", tc + Vector((0, 0.0, 0.1)), (0, 1, 0), (0.12, 0.15, 0.07), "chitin_rust_dark", segs=12, rings=6)
        hc = Vector((0, -0.28, Z(0.35)))
    piv = Vector((0, -0.2, Z(0.34)))
    MH = sk.B @ (Matrix.Translation(piv) @ Matrix.Rotation(math.radians(p.head), 4, "X") @ Matrix.Translation(-piv))
    with attach(MH):
        oell("head", hc, (0, 1, 0), (0.105, 0.09, 0.085), "chitin_rust_dark", segs=14, rings=9)
        oell("brow", hc + Vector((0, -0.03, 0.045)), (0, 1, 0), (0.1, 0.05, 0.03), "chitin_rust", segs=10, rings=6)
        for sx in (-1, 1):
            eye(hc + Vector((sx * 0.075, -0.045, 0.03)), 0.016)
            a = math.radians(18 + max(0.0, p.jaw))
            d = Vector((sx * math.sin(a), -math.cos(a), -0.15))
            b0 = hc + Vector((sx * 0.04, -0.075, -0.03))
            P.tube("mandible", [b0, b0 + d * 0.06, b0 + d * 0.1 + Vector((-sx * 0.04, 0, 0))],
                   [0.024, 0.02, 0.004], "chitin_bone", segs=7, caps=(0.5, 0.3))
    for leg in sk.legs:
        insect_leg(sk, leg)
    with attach(sk.B):
        dead = p.dead >= 0
        if p.dead < 0:
            brute_arm(sk, 1, arm)
            brute_arm(sk, -1, arm * 0.85 + (0.1 if arm > 0.5 else 0.0))
        else:
            brute_arm(sk, 1, 0.0, True)
            brute_arm(sk, -1, 0.0, True)
    if p.dead >= 0:
        flip_dead(p)
    return sk


def brute_poses(spec):
    out = {}
    for i in range(1, 9):
        ph = (i - 1) / 8
        b = rig.trot_pose(spec, i)
        out["walk%02d" % i] = BrutePose(**{**b.__dict__, "head": 3 * math.sin(2 * math.pi * ph),
                                           "tail": 0, "sway": 0, "neck": 0,
                                           "arm": 0.12 * math.sin(2 * math.pi * ph)})
    out["claw1"] = BrutePose(ty=0.03, pitch=-5, arm=-0.35, head=-6)
    out["claw2"] = BrutePose(ty=0.04, pitch=-9, arm=-0.9, head=-10, jaw=14)
    out["claw3"] = BrutePose(ty=-0.07, pitch=6, arm=1.0, head=14, jaw=26,
                             feet={"ml": (-0.05, 0.0), "mr": (-0.03, 0.0)})
    out["claw4"] = BrutePose(ty=-0.03, pitch=2, arm=0.35, head=4, jaw=8)
    out["graze1"] = BrutePose(ty=-0.02, pitch=6, head=18, arm=0.4)
    out["graze2"] = BrutePose(ty=-0.03, pitch=10, head=30, arm=0.6, jaw=10)
    out["flinch1"] = BrutePose(ty=0.05, pitch=-8, head=-12, arm=-0.6, feet={"ml": (0.04, 0.02), "mr": (0.03, 0)})
    out["flinch2"] = BrutePose(ty=0.025, pitch=-4, head=-6, arm=-0.3)
    out["rest"] = BrutePose(bz=0.09, pitch=3, head=16, arm=-0.6)   # crouched, scythes folded in
    return out


# ================================================================ crawler
@dataclass
class CrawlerPose(Pose):
    phase: float = 0.0
    rise: float = 0.0      # head end lifted (0..1)
    amp: float = 1.0       # sideways undulation
    curl: float = 0.0      # body curled up


NSEG = 9
SEG_Y = 0.105
CRAWL_STEP = 0.055                 # half a leg's stance sweep (body lengths)
CRAWL_TRAVEL = 4 * CRAWL_STEP      # ground travel per cycle: the full sweep over half a cycle


def crawler_axis(p):
    """Segment centres (x, y, z) head first, in body frame, and heading."""
    ph = getattr(p, "phase", 0.0)
    rise = getattr(p, "rise", 0.0)
    amp = getattr(p, "amp", 1.0)
    curl = getattr(p, "curl", 0.0)
    pts = []
    for k in range(NSEG + 1):          # 0 = head, 1..9 segments
        t = k / NSEG
        x = amp * 0.055 * math.sin(2 * math.pi * 0.9 * t - ph) * (0.35 + t)
        y = -0.46 + k * SEG_Y
        z = Z(0.1)
        if rise:
            z += 0.2 * rise * smooth(1 - t * 2.2) ** 1.2
            y -= 0.05 * rise * smooth(1 - t * 2.0)
        if curl:
            x += curl * 0.1 * math.sin(math.pi * t) * (1 if k % 2 else 1)
            y -= curl * 0.2 * t
        pts.append(Vector((x, y, z)))
    return pts


def crawler_legs(b):
    return []


def crawler_model(spec, p):
    sk = Skel(spec, p)
    ph = getattr(p, "phase", 0.0)
    pts = crawler_axis(p)
    dead = p.dead >= 0
    with attach(sk.B):
        for k in range(1, NSEG + 1):
            c = pts[k]
            t = (pts[min(k + 1, NSEG)] - pts[k - 1]).normalized() if k < NSEG else (pts[k] - pts[k - 1]).normalized()
            w = 0.1 * (0.85 + 0.15 * math.sin(math.pi * k / NSEG * 0.9))
            oell("seg", c, t, (w, SEG_Y * 0.62, 0.068), "chitin_ochre", segs=14, rings=8)
            oell("segband", c + t * SEG_Y * 0.5, t, (w * 0.97, 0.022, 0.066), "chitin_black", segs=12, rings=6)
            oell("segridge", c + Vector((0, 0, 0.05)), t, (0.03, SEG_Y * 0.5, 0.03), "chitin_ochre", segs=8, rings=5)
            # leg pair: a wave of steps ripples from the head to the tail;
            # each foot is planted half the cycle, sliding back under the
            # body at the ground speed (CRAWL_TRAVEL), then swings forward
            for sx in (-1, 1):
                q = (ph / (2 * math.pi) - k * 0.12 + (0 if sx > 0 else 0.5)) % 1.0
                if q < 0.5:
                    step, lift = lerp(-CRAWL_STEP, CRAWL_STEP, q / 0.5), 0.0
                else:
                    s = (q - 0.5) / 0.5
                    step = lerp(CRAWL_STEP, -CRAWL_STEP, smooth(s))
                    lift = 0.03 * math.sin(math.pi * s)
                root = c + Vector((sx * w * 0.8, 0, -0.02))
                if dead:
                    foot = root + Vector((sx * 0.06, 0.02 * (1 if k % 2 else -1), -0.05))
                else:
                    foot = Vector((c.x + sx * (w + 0.07), c.y + step, Z(0.012 + lift)))
                knee = (root + foot) / 2 + Vector((sx * 0.025, 0, 0.035))
                P.tube("leg", [root, knee, foot], [0.016, 0.013, 0.008], "chitin_black", segs=6, caps=(0.8, 0.4))
        # head: smaller dark-ochre capsule with pincers and feelers
        hc = pts[0] + Vector((0, -0.01, 0.0))
        oell("head", hc, (0, 1, 0), (0.078, 0.06, 0.058), "chitin_ochre", segs=14, rings=8)
        oell("headplate", hc + Vector((0, -0.005, 0.03)), (0, 1, 0), (0.06, 0.045, 0.03), "chitin_black", segs=10, rings=6)
        jaw = max(0.0, p.jaw)
        for sx in (-1, 1):
            eye(hc + Vector((sx * 0.06, -0.03, 0.03)), 0.011)
            spread = 0.25 + jaw * 0.014
            d = Vector((sx * spread, -1.0, -0.05))
            b0 = hc + Vector((sx * 0.035, -0.055, -0.012))
            pts2 = P.turtle(b0, d, 0.22, 6, (0, 0, 1), -sx * (70 - jaw * 0.9))
            P.tube("pincer", pts2, [lerp(0.03, 0.004, (i / 6) ** 0.8) for i in range(7)], "chitin_black", segs=7,
                   caps=(0.5, 0.0))
            f = P.turtle(hc + Vector((sx * 0.025, -0.05, 0.035)), Vector((sx * 0.4, -1, 0.35)), 0.15, 5,
                         (0, 0, 1), sx * 25)
            P.tube("feeler", f, [lerp(0.006, 0.002, i / 5) for i in range(6)], "chitin_ochre", segs=5, caps=(0.5, 0))
    if dead:
        flip_dead(p)
    return sk


def crawler_poses(spec):
    out = {}
    for i in range(1, 9):
        ph = 2 * math.pi * (i - 1) / 8
        out["walk%02d" % i] = CrawlerPose(phase=ph)
    out["bite1"] = CrawlerPose(phase=0.3, rise=0.5, jaw=4, amp=0.6)
    out["bite2"] = CrawlerPose(phase=0.3, rise=0.9, jaw=34, amp=0.6)
    out["bite3"] = CrawlerPose(phase=0.3, rise=0.4, jaw=8, amp=0.6, ty=-0.08)
    out["bite4"] = CrawlerPose(phase=0.3, rise=0.2, jaw=20, amp=0.7, ty=-0.03)
    out["graze1"] = CrawlerPose(phase=0.0, rise=0.0, amp=0.5, jaw=10)
    out["graze2"] = CrawlerPose(phase=0.5, rise=0.0, amp=0.7, jaw=0)
    out["flinch1"] = CrawlerPose(phase=1.2, rise=0.3, amp=1.7, curl=0.5, jaw=22, ty=0.04)
    out["flinch2"] = CrawlerPose(phase=0.9, rise=0.1, amp=1.3, curl=0.25, jaw=10)
    out["rest"] = CrawlerPose(phase=0.4, rise=0.0, amp=0.35, curl=0.7, jaw=0)   # loosely curled, legs tucked
    return out


# ----------------------------------------------------------------- specs
SPECS = {
    "hive_beetle": dict(height=Z0 + 0.1, chest=(0.2, 0.1), legs=beetle_legs, model=beetle_model, poses_fn=beetle_poses,
                        ),
    "hive_brute": dict(height=Z0 + 0.1, chest=(0.2, 0.1), legs=brute_legs, model=brute_model, poses_fn=brute_poses),
    "hive_crawler": dict(height=Z0 + 0.1, chest=(0.1, 0.1), legs=crawler_legs, model=crawler_model,
                         poses_fn=crawler_poses),
}


def _spec(row):
    kw = dict(SPECS[row.key])
    fn = kw.pop("poses_fn")
    b = Body(key=row.key, length=R.body_length(row.size), attack=row.attack, archetype=row.archetype, ears=None,
             tail=None, pal=dict(coat="chitin_teal"), **kw)
    if row.key == "hive_crawler":
        b.travel = CRAWL_TRAVEL
    b.poses = fn(b)
    return b


def build(row, static=False):
    if row.key not in SPECS:
        return []
    return render(_spec(row), static, row.size)
