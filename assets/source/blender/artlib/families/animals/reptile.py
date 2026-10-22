"""Reptiles (ART-011): cobra (snake), iguana (lizard) and tortoise, on the
shared rig (rig.py).

- cobra:    a `model=` that sweeps one tube along an S-wave spine on the
            ground with the front third raised into a hooded rear-up;
            SnakePose adds the wave phase, raise, hood and lunge.
- iguana:   a sprawled quadruped: rig Skel torso, sideways-bending legs
            (`legs=`), crest spines, a long banded tail that swings.
- tortoise: a domed, scute-patterned shell over stumpy pillar legs, with a
            head that extends, bites and withdraws (TortoisePose).

Poses reuse the rig's walk (diagonal pairs), attack, graze and flinch
functions where they fit and override the rest through `poses=`.
"""
import math
from dataclasses import dataclass, replace

from mathutils import Matrix, Vector

from .. import animal_roster as R
from ... import stage as S
from . import parts as P
from . import rig
from .bird import basis, lerp, oell, render
from .rig import Body, Leg, Pose, Skel, attach

ARCHETYPES = ("snake", "lizard", "tortoise")


def smooth(t):
    t = max(0.0, min(1.0, t))
    return t * t * (3 - 2 * t)


def unroll(p):
    """Corpses are rolled onto their side by the rig; flat bodies cancel the
    roll so they lie as modelled."""
    roll = rig.CORPSE_ROLL[p.dead][0]
    S.transform(S.objects(), Matrix.Rotation(math.radians(-roll), 4, "Y"))


# ===================================================================== cobra
@dataclass
class SnakePose(Pose):
    phase: float = 0.0     # S-wave phase (radians)
    rear: float = 1.0      # 0 flat on the ground .. 1 fully reared up
    hood: float = 0.8      # hood spread 0..1
    lunge: float = 0.0     # strike: + forward and down, - recoil
    amp: float = 1.0       # S-wave amplitude


KEYS = [(0.0, 0.0), (0.22, 70.0), (0.45, 98.0), (0.75, 96.0), (1.0, 70.0)]


def _key(t):
    for (t0, a0), (t1, a1) in zip(KEYS, KEYS[1:]):
        if t <= t1:
            return lerp(a0, a1, smooth((t - t0) / (t1 - t0)))
    return KEYS[-1][1]


def snake_spine(p, ng=30, nr=16):
    ph, rear = getattr(p, "phase", 0.0), getattr(p, "rear", 1.0)
    lunge, amp = getattr(p, "lunge", 0.0), getattr(p, "amp", 1.0)
    y_f = -0.1 - 0.1 * lunge * rear
    pts, rad = [], []

    def radius(s):
        tab = [(0.0, 0.007), (0.12, 0.02), (0.4, 0.05), (0.6, 0.05), (0.85, 0.038), (1.0, 0.032)]
        for (s0, r0), (s1, r1) in zip(tab, tab[1:]):
            if s <= s1:
                return lerp(r0, r1, (s - s0) / (s1 - s0))
        return tab[-1][1]

    for i in range(ng):
        g = i / ng
        env = smooth(g / 0.2) * (1 - 0.8 * smooth((g - 0.78) / 0.22))
        x = amp * 0.15 * env * math.sin(2 * math.pi * 1.3 * g - ph)
        y = lerp(0.5, y_f, g)
        r = radius(g * 0.62)
        pts.append(Vector((x, y, r)))
        rad.append(r)
    x, y, z = pts[-1]
    Lr = 0.44 * (1 + 0.3 * max(0.0, lunge))
    ds = Lr / nr
    for j in range(1, nr + 1):
        t = j / nr
        a = rear * _key(t) - lunge * 85.0 * t ** 1.5 + (20.0 * -lunge * t if lunge < 0 else 0.0)
        al = math.radians(a)
        y -= math.cos(al) * ds
        z += math.sin(al) * ds
        x += getattr(p, "yaw", 0.0) * 0.0012
        z = max(z, 0.025)
        s = 0.62 + 0.38 * t
        r = radius(s)
        pts.append(Vector((x, y, z)))
        rad.append(r)
    return pts, rad, ng


def snake_model(spec, p):
    if p.dead >= 0:
        p = SnakePose(phase=0.9 * p.dead, rear=0.0, hood=0.0, amp=1.0 + 0.2 * p.dead, jaw=6.0, dead=p.dead)
    pts, rad, ng = snake_spine(p)
    n = len(pts)
    tan = [(pts[min(i + 1, n - 1)] - pts[max(i - 1, 0)]).normalized() for i in range(n)]
    P.tube("snake", pts, rad, "scale_olive", segs=12, caps=(0.5, 0.2))
    # dark cross bands along the back
    for i in range(4, ng, 5):
        P.tube("band", [pts[i] - tan[i] * 0.005, pts[i] + tan[i] * 0.005], [rad[i] * 1.04, rad[i] * 1.04],
               "scale_dark", segs=12, caps=(0.2, 0.2))
    # pale belly shows on the raised front
    fwd = Vector((0, -1, 0))
    belly, br = [], []
    for i in range(ng - 2, n):
        off = (fwd - tan[i] * fwd.dot(tan[i])).normalized()
        belly.append(pts[i] + off * rad[i] * 0.62)
        br.append(rad[i] * 0.55)
    P.tube("belly", belly, br, "scale_cream", segs=8, caps=(0.5, 0.2))
    for k in range(2, len(belly) - 2, 2):
        q = belly[k]
        P.tube("bband", [q - tan[ng - 2 + k] * 0.006, q + tan[ng - 2 + k] * 0.006], [br[k] * 1.1, br[k] * 1.1],
               "scale_dark", segs=8, caps=(0.2, 0.2))
    # hood and head at the top
    hood = getattr(p, "hood", 0.8)
    ih = n - 3
    T = tan[ih]
    c = pts[ih]
    if hood > 0.05:
        hw, hh = 0.03 + 0.14 * hood, 0.06 + 0.1 * hood
        oell("hood", c, fwd, (hw, 0.045, hh * 1.1), "scale_olive", up=T, segs=18, rings=10)
        oell("hoodf", c + fwd * 0.03, fwd, (hw * 0.8, 0.024, hh * 0.9), "scale_cream", up=T, segs=16, rings=8)
        oell("hoodb", c + fwd * 0.052 + Vector((0, 0, -hh * 0.1)), fwd, (hw * 0.55, 0.01, hh * 0.12), "scale_dark",
             up=T, segs=12, rings=5)
    beta = math.radians(14 + max(0.0, getattr(p, "lunge", 0.0)) * 14)
    hd = Vector((0, -math.cos(beta), -math.sin(beta)))
    tip = pts[-1]
    hc = tip + hd * 0.03
    oell("head", hc, hd, (0.036, 0.058, 0.028), "scale_olive", segs=14, rings=9)
    jaw = max(0.0, p.jaw)
    hinge = hc - hd * 0.02 + Vector((0, 0, -0.012))
    jd = Matrix.Rotation(math.radians(jaw), 3, Vector((1, 0, 0))) @ hd
    oell("jaw", hinge + jd * 0.032, jd, (0.021, 0.04, 0.011), "scale_cream", segs=10, rings=6)
    for sx in (-1, 1):
        oell("eye", hc + Vector((sx * 0.024, -0.012, 0.012)), hd, (0.007, 0.007, 0.007), "eye_black", segs=6, rings=4)
        if jaw > 10:
            P.cone("fang", hc + hd * 0.045 + Vector((sx * 0.011, 0, -0.012)),
                   hc + hd * 0.05 + Vector((sx * 0.011, 0, -0.04)), 0.005, "horn_ivory", segs=5)
    if p.dead >= 0:
        unroll(p)


def snake_poses(spec):
    out = {}
    for i in range(1, 9):
        ph = 2 * math.pi * (i - 1) / 8
        out["walk%02d" % i] = SnakePose(phase=-ph, rear=0.55, hood=0.15, yaw=8 * math.sin(ph))
    out["bite1"] = SnakePose(phase=0.4, rear=1.0, hood=1.0, lunge=-0.35, jaw=0)
    out["bite2"] = SnakePose(phase=0.4, rear=1.0, hood=1.0, lunge=0.4, jaw=30)
    out["bite3"] = SnakePose(phase=0.4, rear=1.0, hood=0.9, lunge=1.0, jaw=50)
    out["bite4"] = SnakePose(phase=0.4, rear=0.9, hood=0.7, lunge=0.3, jaw=12)
    out["graze1"] = SnakePose(phase=0.0, rear=0.55, hood=0.0)
    out["graze2"] = SnakePose(phase=0.0, rear=0.25, hood=0.0, jaw=6)
    out["flinch1"] = SnakePose(phase=0.8, rear=1.0, hood=1.0, lunge=-0.6, yaw=10)
    out["flinch2"] = SnakePose(phase=0.6, rear=1.0, hood=0.9, lunge=-0.25, yaw=5)
    return out


# ===================================================================== iguana
def lizard_legs(b):
    cw, cd = b.chest
    fake = type("S", (), {"b": b})()
    r0, r1, r2, r3 = b.leg_r
    z0 = b.height - cd
    legs = []
    for name, sx, ph, front in (("fl", 1, 0.0, True), ("fr", -1, 0.5, True), ("hl", 1, 0.5, False),
                                ("hr", -1, 0.0, False)):
        y, zc, _, _, rx, rz = Skel.section(fake, 0.8 if front else 0.22)
        jz = zc - rz * 0.1
        out = 0.13 if front else 0.14
        drop = z0 + jz - r3
        dist = math.hypot(out, drop)
        L = dist * 1.28
        legs.append(Leg(name, (sx * rx * 0.5, y, zc), (sx * rx * 0.95, y, jz), L * 0.5, L * 0.5,
                        (sx * 0.8, 0.9 if front else -0.9, 0.7), (sx * (rx + out), y + (-0.03 if front else 0.04)),
                        (r0, r1, r2, r3), b.c("legs"), b.c("lower"), "claw", b.c("toe"), ph, front))
    return legs


def lizard_tail(sk, pal_a="scale_green", pal_b="scale_green_dark"):
    p = sk.p
    tb = sk.tail_base()
    zg = -(sk.z0 - p.bz)
    n = 24
    L = 0.58
    pts, rad = [], []
    for i in range(n + 1):
        s = i / n
        z = lerp(tb.z, zg + 0.012, smooth(s * 2.2))
        z += math.sin(math.radians(p.tail)) * s * 0.3
        x = 0.0042 * p.sway * s ** 1.4 + 0.02 * s * math.sin(5 * s)
        pts.append(Vector((x, tb.y + 0.02 + s * L, max(z, zg + 0.012))))
        rad.append(lerp(0.05, 0.005, s ** 0.8))
    with attach(sk.B):
        for k in range(0, n, 3):
            a, b2 = k, min(n, k + 3)
            mat = pal_b if (k // 3) % 2 else pal_a
            P.tube("tail", pts[a:b2 + 1], rad[a:b2 + 1], mat, segs=10,
                   caps=(0.5 if k == 0 else 0.0, 0.8 if b2 == n else 0.0))


def lizard_model(spec, pose):
    sk = Skel(spec, pose)
    rig.build_torso(sk)
    rig._m_folds(sk, "scale_green_dark", us=(0.3, 0.45, 0.6, 0.75), k=1.035)
    rig.build_legs(sk)
    rig.build_neck(sk)
    rig.build_head(sk, ears=False, horns=False)
    lizard_tail(sk)
    # dorsal spines down the neck and back
    with attach(sk.B):
        nb = sk.neck_base()
        nk = P.bezier(nb, sk.neck_mid, sk.poll, 6)
        th = math.radians(sk.b.neck_angle + pose.neck)
        up = Vector((0, math.sin(th), math.cos(th)))
        r0, r1 = sk.b.neck_r
        for i, q in enumerate(nk[:-1]):
            base = q + up * lerp(r0, r1, i / 5) * 0.85
            P.cone("spine", base, base + Vector((0, 0.012, 0.05)) - Vector((0, 0.0, 0.0)), 0.015, "scale_spine", segs=6)
        for i in range(10):
            u = 0.96 - 0.9 * i / 9
            q, nn = sk.surface(u, 90)
            h = 0.045 * (1 - 0.3 * i / 9)
            P.cone("spine", q - Vector((0, 0, 0.004)), q + Vector((0, 0.012, h * 1.2)), 0.015, "scale_spine", segs=6)
    g = sk.head_geo()
    with attach(sk.B @ sk.H):
        P.ell("dewlap", g["c"] + Vector((0, g["hl"] * 0.2, -g["hh"] * 1.0)),
              (g["hw"] * 0.12, g["hl"] * 0.6, g["hh"] * 0.85), "scale_pale", segs=10, rings=6)
        for sx in (-1, 1):
            P.ell("cheek", g["c"] + Vector((sx * g["hw"] * 0.85, g["hl"] * 0.25, -g["hh"] * 0.25)),
                  (g["hw"] * 0.12, g["hl"] * 0.18, g["hh"] * 0.3), "scale_pale", segs=8, rings=5)
    return sk


def lizard_walk(spec, i):
    ph = (i - 1) / rig.TROT_N
    p = rig.trot_pose(spec, i)
    s = math.sin(2 * math.pi * ph)
    return replace(p, yaw=-14 * s, sway=30 * s, roll=2.0 * s, neck=0.0, head=0.0, bz=p.bz * 0.5)


# ================================================================== tortoise
@dataclass
class TortoisePose(Pose):
    retract: float = 0.0    # 1 = head and legs withdrawn
    ext: float = 0.0        # head thrust forward (bite)


SHELL = dict(rx=0.3, ry=0.38, rz=0.27, cz=0.04)


def tortoise_legs(b):
    r0, r1, r2, r3 = b.leg_r
    z0 = b.height - b.chest[1]
    legs = []
    for name, sx, y, ph, front in (("fl", 1, -0.2, 0.0, True), ("fr", -1, -0.2, 0.5, True), ("hl", 1, 0.2, 0.5, False),
                                   ("hr", -1, 0.2, 0.0, False)):
        jz = -0.015
        drop = z0 + jz - r3
        L = drop * 1.2
        legs.append(Leg(name, (sx * 0.17, y, 0.0), (sx * 0.22, y, jz), L * 0.5, L * 0.5,
                        (sx * 0.8, -0.5 if front else 0.5, 0.2), (sx * 0.25, y + (-0.02 if front else 0.0)),
                        (r0, r1, r2, r3), b.c("legs"), None, "pad", b.c("toe"), ph, front))
    return legs


def scutes(sk):
    rx, ry, rz, cz = SHELL["rx"], SHELL["ry"], SHELL["rz"], SHELL["cz"]
    C = Vector((0, 0, cz))

    def place(d, su, sv, mat):
        d = Vector(d).normalized()
        pt = C + Vector((rx * d.x, ry * d.y, rz * d.z))
        n = Vector((d.x / rx, d.y / ry, d.z / rz)).normalized()
        tang = Vector((0, 1, 0)) - n * n.y
        m = basis(tang, n)
        P.ell("seam", pt - n * 0.0115, (su + 0.012, sv + 0.012, 0.028), "shell_seam", m=m, segs=10, rings=5)
        P.ell("scute", pt - n * 0.01, (su, sv, 0.03), mat, m=m, segs=10, rings=5)

    for k, b in enumerate((-0.66, -0.33, 0.0, 0.33, 0.66)):
        place((0, b, math.sqrt(1 - b * b)), rx * 0.3, ry * 0.22, "shell_olive")
    for a in (-0.6, 0.6):
        for k, b in enumerate((-0.57, -0.19, 0.19, 0.57)):
            place((a, b, math.sqrt(max(0.05, 1 - a * a - b * b))), rx * 0.23, ry * 0.22,
                  "shell_sand" if (k + (a > 0)) % 2 else "shell_olive")
    for k in range(14):
        th = 2 * math.pi * (k + 0.5) / 14
        if abs(math.sin(th + math.pi / 2)) > 0.93 and math.cos(th) * 0 == 0 and -math.sin(th) > 0.9:
            continue
        c = 0.2
        s = math.sqrt(1 - c * c)
        place((math.cos(th) * s, math.sin(th) * s, c), rx * 0.15, ry * 0.12, "shell_sand" if k % 2 else "shell_olive")


def flatten(ob, zmin):
    for v in ob.data.vertices:
        v.co.z = max(v.co.z, zmin)
    ob.data.update()


def tortoise_model(spec, p):
    rt = getattr(p, "retract", 0.0)
    ext = getattr(p, "ext", 0.0)
    sk = Skel(spec, p)
    rx, ry, rz, cz = SHELL["rx"], SHELL["ry"], SHELL["rz"], SHELL["cz"]
    with attach(sk.B):
        sh = P.ell("shell", (0, 0, cz), (rx, ry, rz), "shell_olive", segs=28, rings=18)
        flatten(sh, -0.07)
        rim = P.ell("plastron", (0, 0, -0.06), (rx * 0.97, ry * 0.97, 0.03), "shell_sand", segs=24, rings=6)
        scutes(sk)
    for leg in sk.legs:
        if rt > 0.5:
            continue
        rig.build_leg(sk, leg)
    with attach(sk.B):
        a = math.radians(6 + p.neck * 0.6)
        d = Vector((0, -math.cos(a), math.sin(a)))
        b0 = Vector((0, -ry * 0.9, -0.005))
        ln = lerp(0.14 + 0.1 * ext, 0.0, rt)
        hc = b0 + d * (ln + 0.02) + Vector((0, -0.02 * (1 - rt), -p.head * 0.0015))
        if rt < 0.9:
            P.tube("neck", [b0 - d * 0.06, (b0 + hc) / 2, hc], [0.062, 0.05, 0.046], "skin_tortoise", segs=10,
                   caps=(0.6, 0.8))
            P.ell("head", hc + d * 0.03, (0.047, 0.07, 0.044), "skin_tortoise", segs=14, rings=9)
            P.cone("beak", hc + d * 0.085 + Vector((0, 0, 0.004)), hc + d * 0.07 + Vector((0, 0, -0.012)), 0.027,
                   "shell_sand", segs=8)
            hinge = hc + d * 0.0 + Vector((0, 0, -0.025))
            J = Matrix.Translation(hinge) @ Matrix.Rotation(math.radians(max(0.0, p.jaw)), 4, "X") @ Matrix.Translation(-hinge)
            with attach(J):
                P.ell("jaw", hc + d * 0.055 + Vector((0, 0, -0.03)), (0.03, 0.045, 0.014), "skin_tortoise", segs=10,
                      rings=6)
            for sx in (-1, 1):
                P.ell("eye", hc + d * 0.035 + Vector((sx * 0.04, 0, 0.016)), (0.01, 0.01, 0.011), "eye_black", segs=6,
                      rings=4)
        P.cone("tail", Vector((0, ry * 0.95, 0.0)), Vector((0, ry * 1.12, -0.02)), 0.03, "skin_tortoise", segs=8)
    if p.dead >= 0:
        pass
    return sk


def tortoise_poses(spec):
    out = {}
    for i in range(1, 9):
        ph = (i - 1) / 8
        base = rig.trot_pose(spec, i)
        out["walk%02d" % i] = TortoisePose(**{**base.__dict__, "bz": base.bz * 0.3, "pitch": 1.0 * math.sin(2 * math.pi * ph),
                                              "neck": 4 * math.sin(2 * math.pi * ph), "head": 0.0, "tail": 0.0,
                                              "sway": 0.0, "retract": 0.0})
    out["bite1"] = TortoisePose(ty=0.01, retract=0.35, jaw=0, neck=4)
    out["bite2"] = TortoisePose(ty=-0.02, ext=0.5, jaw=28, neck=6)
    out["bite3"] = TortoisePose(ty=-0.05, ext=1.0, jaw=45, neck=8)
    out["bite4"] = TortoisePose(ty=-0.02, ext=0.4, jaw=10, neck=4)
    out["graze1"] = TortoisePose(neck=-14, ext=0.3, head=20)
    out["graze2"] = TortoisePose(neck=-26, ext=0.2, head=30)
    out["flinch1"] = TortoisePose(retract=1.0, ty=0.02, bz=0.06)
    out["flinch2"] = TortoisePose(retract=0.6, ty=0.01, bz=0.03)
    return out


# ------------------------------------------------------------------ specs
SPECS = {
    "cobra": dict(model=snake_model, legs=lambda b: [], height=0.3, torso=0.6, chest=(0.05, 0.05), pal=dict(coat="scale_olive")),
    "iguana": dict(
        height=0.19, torso=0.46, chest=(0.095, 0.082), waist=(0.1, 0.085), hips=(0.11, 0.09), rump_z=-0.004,
        torso_p=2.4, leg_r=(0.042, 0.035, 0.026, 0.021), neck_len=0.1, neck_angle=14, neck_r=(0.065, 0.046),
        neck_y=0.04, head=(0.05, 0.07, 0.042), head_angle=8, muzzle=(0.062, 0.03, 0.026), muzzle_drop=6,
        ears=None, tail=None, eye=0.011, legs=lizard_legs, model=lizard_model,
        pal=dict(coat="scale_green", belly="scale_pale", head="scale_green", muzzle="scale_green", nose=None,
                 legs="scale_green", lower="scale_green_dark", toe="scale_pale", eye="eye_black")),
    "tortoise": dict(
        height=0.19, torso=0.76, chest=(0.3, 0.05), waist=(0.3, 0.05), hips=(0.3, 0.05), leg_r=(0.05, 0.047, 0.043, 0.04),
        ears=None, tail=None, legs=tortoise_legs, model=tortoise_model,
        pal=dict(coat="shell_olive", legs="skin_tortoise", toe="shell_sand", lower=None)),
}
OUT_W = {}


def _spec(row):
    kw = dict(SPECS[row.key])
    L = R.body_length(row.size)
    kw.setdefault("eye", 0.011)
    b = Body(key=row.key, length=L, attack=row.attack, archetype=row.archetype, **kw)
    if row.key == "cobra":
        b.poses = snake_poses(b)
    elif row.key == "iguana":
        b.poses = {"walk%02d" % i: lizard_walk(b, i) for i in range(1, 9)}
    else:
        b.poses = tortoise_poses(b)
    return b


def build(row, static=False):
    if row.key not in SPECS:
        print("reptile: no spec for %s yet; skipped" % row.key)
        return []
    return render(_spec(row), static, row.size)
