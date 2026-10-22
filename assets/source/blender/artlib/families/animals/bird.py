"""Birds (ART-011): cassowary, chicken (rooster + hen), duck, emu, goose,
ostrich and turkey, built on the shared rig (rig.py).

Every bird is a rig.Body with the rig's hooks:
- `legs=`   bird_legs: two digitigrade legs (short hidden thigh, long
            tarsus whose "knee" is the ankle, bending backwards);
- `model=`  bird_model: egg-shaped torso (rig.build_torso), S-curved neck,
            beaked head, folded wings, species tail and plumage extras;
- `poses=`  walk (two-legged, head bob), peck / kick, graze, flinch.

Bird specifics ride on the rig's Pose fields:
- `Pose.ears` opens the wings (degrees), since birds have no ears;
- the walk uses rig.trot_pose with the two legs half a cycle apart.

This module also holds helpers the reptile and insect modules reuse
(basis, oell, blade, beam, cubic, render).
"""
import math
from dataclasses import replace

from mathutils import Matrix, Vector

from .. import animal_roster as R
from ... import stage as S
from . import parts as P
from . import rig
from .rig import Body, Leg, Pose, Skel, attach, ik2

ARCHETYPES = ("bird",)

# outline width in @2x pixels by roster size class (default is 4)
OUTLINE = {"XS": 2, "S": 3}


# ------------------------------------------------------- shared helpers
def basis(axis, up=(0, 0, 1)):
    """4x4 matrix whose local +Y is `axis` and +Z is as close to `up` as
    possible (local X is the side)."""
    y = Vector(axis).normalized()
    x = y.cross(Vector(up))
    if x.length < 1e-4:
        x = y.cross(Vector((1, 0, 0)))
    x.normalize()
    z = x.cross(y)
    return Matrix(((x.x, y.x, z.x, 0), (x.y, y.y, z.y, 0), (x.z, y.z, z.z, 0), (0, 0, 0, 1)))


def oell(name, c, axis, radii, mat, up=(0, 0, 1), segs=12, rings=8):
    """Ellipsoid centred at c, radii=(side, along `axis`, normal)."""
    return P.ell(name, c, radii, mat, m=basis(axis, up), segs=segs, rings=rings)


def blade(name, base, direction, length, width, mat, up=(0, 0, 1), thick=0.006, segs=8, rings=5):
    """A flat feather: thin ellipsoid from `base` along `direction`."""
    d = Vector(direction).normalized()
    return oell(name, Vector(base) + d * length * 0.5, d, (width * 0.5, length * 0.5, thick), mat, up, segs, rings)


def beam(name, a, b, r, mat, segs=8, caps=(0.6, 0.6)):
    """A straight tube of radius r (float or (rs, ru)) from a to b."""
    return P.tube(name, [Vector(a), Vector(b)], [r, r], mat, segs=segs, caps=caps)


def cubic(a, b, c, d, t):
    u = 1 - t
    return a * (u * u * u) + b * (3 * u * u * t) + c * (3 * u * t * t) + d * (t * t * t)


def lerp(a, b, t):
    return a + (b - a) * t


def render(spec, static, size):
    """rig.render_species with this species' outline width (the rig does
    not pass one): a thin outline for XS and S sprites."""
    width = OUTLINE.get(size.rstrip("+-"))
    orig = S.render_asset

    def wrapped(*a, **k):
        if width is not None:
            k.setdefault("outline_width", width)
        return orig(*a, **k)

    S.render_asset = wrapped
    try:
        return rig.render_species(spec, static=static)
    finally:
        S.render_asset = orig


# --------------------------------------------------------------- legs
def cfg_for(spec):
    cfg = dict(CFG[spec.key])
    if spec.pal.get("sex") == "f":
        cfg.update(CFG.get(spec.key + "_f", {}))
    return cfg


def bird_legs(b):
    cfg = cfg_for(b)
    cw, cd = b.chest
    fake = type("S", (), {"b": b})()
    y, zc, _, _, rx, rz = Skel.section(fake, cfg["leg_u"])
    z0 = b.height - cd
    xh = rx * b.stance
    jz = zc - rz * 0.55
    r0, r1, r2, r3 = b.leg_r
    drop = z0 + jz - r3
    total = drop * cfg.get("leg_k", 1.1)
    l1 = total * cfg.get("thigh", 0.4)
    legs = []
    for name, sx, ph in (("hl", 1, 0.5), ("hr", -1, 0.0)):
        legs.append(Leg(name, (sx * xh * 0.9, y, zc + rz * 0.1), (sx * xh, y, jz), l1, total - l1, (0, 1, 0),
                        (sx * xh * cfg.get("foot_spread", 1.05), y + cfg.get("foot_dy", -0.02)),
                        (r0, r1, r2, r3), b.c("legs"), b.c("lower"), "bird", b.c("toe"), ph, False))
    return legs


DEAD_FEET = {0: (-0.04, 0.55), 1: (0.1, 0.38), 2: (-0.14, 0.9)}


def foot_target(sk, leg):
    p = sk.p
    if p.dead >= 0:
        j = sk.joint_world(leg)
        dy, k = DEAD_FEET[p.dead]
        L = (leg.l1 + leg.l2)
        return Vector((leg.foot[0] * (1.0 + 0.25 * p.dead), j.y + dy * (1 + L), j.z - L * k))
    return sk.foot_target(leg)


def bird_leg(sk, leg, cfg):
    joint = sk.B @ Vector(leg.joint)
    root = sk.B @ Vector(leg.root)
    knee, foot = ik2(joint, foot_target(sk, leg), leg.l1, leg.l2, leg.bend)
    r0, r1, r2, r3 = leg.r
    P.tube("thigh", [root, joint, knee], [r0, r1, r2 * 1.15], leg.mat, segs=10, caps=(1, 0.8))
    P.tube("tarsus", [knee, (knee + foot) / 2, foot], [r2, (r2 + r3) / 2, r3], leg.lower or leg.mat, segs=8,
           caps=(0.8, 0.5))
    if cfg.get("spur"):
        d = (foot - knee).normalized()
        c = knee + (foot - knee) * 0.7
        P.cone("spur", c + Vector((0, r3 * 0.6, 0)), c + Vector((0, r3 * 3.2, 0.01)) - d * 0.005, r3 * 0.7, "horn_ivory",
               segs=6)
    mat = leg.foot_mat or leg.lower or leg.mat
    lift = max(0.0, foot.z - r3)
    droop = min(0.5, lift * 2.5)
    rt = cfg.get("toe_r", r3 * 0.85)
    sx = 1 if leg.joint[0] > 0 else -1
    for ang, ln in cfg["toes"]:
        a = math.radians(ang)
        d = Vector((math.sin(a) * (1 if ang else 0), -math.cos(a), 0))
        if ang and sx < 0:
            d.x = -d.x
        base = Vector((foot.x, foot.y, max(rt * 0.95, foot.z - r3 * 0.2)))
        tip = base + d * ln + Vector((0, 0, -ln * droop))
        tip.z = max(tip.z, rt * 0.7) if droop == 0 else tip.z
        P.tube("toe", [base, (base + tip) / 2, tip], [rt * 1.1, rt, rt * 0.7], mat, segs=7, caps=(0.8, 0.8))
        if cfg.get("claw") and ang == 0:
            cl = cfg["claw"]
            P.cone("claw", tip - d * rt, tip + d * cl + Vector((0, 0, -cl * 0.45)), rt * 0.75, "horn_dark", segs=6)
        if cfg.get("claw_all") and ang != 0:
            cl = cfg["claw_all"]
            P.cone("claw", tip - d * rt, tip + d * cl + Vector((0, 0, -cl * 0.45)), rt * 0.7, "horn_dark", segs=6)
    if cfg.get("hind"):
        base = Vector((foot.x, foot.y, max(rt, foot.z)))
        P.tube("toe", [base, base + Vector((0, cfg["hind"], -droop * cfg["hind"] * 0.5))], [rt, rt * 0.6], mat, segs=6,
               caps=(0.8, 0.8))
    if cfg.get("web"):
        wl = cfg["web"]
        P.ell("web", Vector((foot.x, foot.y - wl * 0.45, max(rt * 0.5, foot.z - r3 * 0.6))), (wl * 0.62, wl * 0.5, 0.0035),
              mat, segs=12, rings=4)


# ------------------------------------------------------------ neck, head
def neck_path(sk, cfg, n=11):
    b, p = sk.b, sk.p
    nb, poll = sk.neck_base(), sk.poll
    th = math.radians(b.neck_angle + p.neck)
    s = math.radians(cfg.get("neck_s", 0.0))
    d0 = Vector((0, -math.cos(th - s), math.sin(th - s)))
    d1 = Vector((0, -math.cos(th + s), math.sin(th + s)))
    k = 0.42 * b.neck_len * cfg.get("neck_k", 1.0)
    p1, p2 = nb + d0 * k, poll - d1 * k
    return [cubic(nb, p1, p2, poll, i / (n - 1)) for i in range(n)]


def bird_neck(sk, cfg):
    b = sk.b
    pts = neck_path(sk, cfg)
    n = len(pts)
    r0, r1 = b.neck_r
    rad = [lerp(r0, r1, (i / (n - 1)) ** cfg.get("neck_pow", 0.8)) for i in range(n)]
    pts = [pts[0] + (pts[0] - pts[2]) * 0.35] + pts
    rad = [rad[0]] + rad
    segs = cfg.get("neck_mats") or [(0.0, 1.0, b.c("coat"), 1.0)]
    with attach(sk.B):
        for t0, t1, mat, k in segs:
            a, z = int(round(t0 * (n - 1))), int(round(t1 * (n - 1)))
            sub, rs = pts[a:z + 2], rad[a:z + 2]
            if len(sub) < 2:
                continue
            P.tube("neck", sub, [r * k for r in rs], mat, segs=12, caps=(0.5 if a == 0 else 0.3, 0.8))
        for fn in cfg.get("neck_extra", ()):
            fn(sk, cfg, pts, rad)


def bird_head(sk, cfg):
    b, p = sk.b, sk.p
    g = sk.head_geo()
    c, m0, tip = g["c"], g["m0"], g["tip"]
    hw, hl, hh, ml, mw, mh = g["hw"], g["hl"], g["hh"], g["ml"], g["mw"], g["mh"]
    a = math.radians(b.muzzle_drop)
    fwd = Vector((0, -math.cos(a), -math.sin(a)))
    flat = cfg.get("beak") == "flat"
    hook = cfg.get("hook", 0.0)
    with attach(sk.B @ sk.H):
        P.ell("head", c, (hw, hl, hh), b.c("head"), segs=18, rings=12)
        root = m0 - fwd * hl * 0.45
        n = 4
        pts, rad = [], []
        for i in range(n):
            t = i / (n - 1)
            q = root + fwd * (ml + hl * 0.45) * t
            q.z -= hook * ml * t * t
            pts.append(q)
        if flat:
            rad = [(mw * 1.0, mh * 1.1), (mw * 1.08, mh * 1.0), (mw * 1.12, mh * 0.85), (mw * 1.02, mh * 0.55)]
            caps = (0.3, 0.6)
        else:
            rad = [(mw * 1.35, mh * 1.4), (mw * 1.05, mh * 1.05), (mw * 0.62, mh * 0.62), (mw * 0.12, mh * 0.12)]
            caps = (0.3, 0.3)
        P.tube("beak", pts, rad, b.c("nose"), segs=10, caps=caps)
        # lower beak, hinged under the cranium
        hinge = c + Vector((0, -hl * 0.1, -hh * 0.45))
        jt = pts[-1] * 0.92 + hinge * 0.08 + Vector((0, 0, -mh * 1.5))
        J = Matrix.Translation(hinge) @ Matrix.Rotation(math.radians(p.jaw), 4, "X") @ Matrix.Translation(-hinge)
        with attach(J):
            mid = (hinge + jt) / 2 + Vector((0, 0, mh * 0.2))
            lr = [(mw * 0.85, mh * 0.55), (mw * 0.8, mh * 0.5), (mw * (0.9 if flat else 0.35), mh * (0.35 if flat else 0.3))]
            P.tube("jaw", [hinge, mid, jt], lr, b.c("beak_low", b.c("nose")), segs=8, caps=(0.5, 0.5 if flat else 0.2))
        for sx in (-1, 1):
            e = c + Vector((sx * hw * 0.9, -hl * 0.18, hh * 0.22))
            if p.dead >= 0:
                P.ell("eye", e, (b.eye * 0.5, b.eye * 1.3, b.eye * 0.35), "fur_dark", segs=8, rings=5)
            else:
                P.ell("eye", e, (b.eye * 0.9, b.eye, b.eye), b.c("eye"), segs=8, rings=6)
        for fn in cfg.get("head_extra", ()):
            fn(sk, g, cfg)


# -------------------------------------------------------- wings, tails
def wings(sk, cfg):
    w = cfg.get("wing")
    if not w:
        return
    p = sk.p
    open_ = max(0.0, p.ears)
    y, zc, rx, rz, frx, frz = sk.section(w["u"])
    wl, wh = w["len"], w["h"]
    thick = w.get("thick", 0.016)
    for sx in (1, -1):
        sh = Vector((sx * (frx * w.get("out", 0.9)), y, zc + frz * w.get("up", 0.35)))
        M = sk.B @ Matrix.Translation(sh) @ Matrix.Rotation(math.radians(-sx * open_), 4, "Y")
        with attach(M):
            if w.get("puff"):
                P.puff("wingpuff", Vector((sx * thick * 2, wl * 0.45, -wh * 0.2)),
                       (thick * 2.5, wl * 0.55, wh * 0.6), w["mat"], sk.b.key + "w%d" % sx, n=18, bump=0.5)
            else:
                oell("wing", Vector((sx * thick * 0.5, wl * 0.5, -wh * 0.38)), (0, 1, -0.12),
                     (wh * 0.5, wl * 0.5, thick), w["mat"], up=(sx, 0, 0))
                tm = w.get("tip")
                if tm:
                    P.tube("primaries", [Vector((sx * thick * 0.6, wl * 0.62, -wh * 0.3)),
                                         Vector((sx * thick * 0.6, wl * 1.0, -wh * 0.5)),
                                         Vector((sx * thick * 0.6, wl * 1.28, -wh * 0.66))],
                           [(thick * 0.9, wh * 0.28), (thick * 0.8, wh * 0.2), (0.0, 0.0)], tm, segs=8,
                           caps=(0.3, 0), side=(1, 0, 0))


def tail_xform(sk, extra=0.0):
    base = sk.tail_base()
    return (sk.B @ Matrix.Translation(base) @ Matrix.Rotation(math.radians(sk.p.sway), 4, "Z")
            @ Matrix.Rotation(math.radians(sk.p.tail + extra), 4, "X") @ Matrix.Translation(-base)), base


def tail_fan(sk, cfg):
    """Turkey: a big fan of banded feathers, leaning back."""
    t = cfg["fan"]
    M, base = tail_xform(sk)
    ang = math.radians(t["lean"])
    a = Vector((0, math.cos(ang), math.sin(ang)))
    nrm = Vector((0, -math.sin(ang), math.cos(ang)))
    side = Vector((1, 0, 0))
    n = t["n"]
    with attach(M):
        for i in range(n):
            phi = math.radians(lerp(-t["spread"], t["spread"], i / (n - 1)))
            d = a * math.cos(phi) + side * math.sin(phi)
            ln = t["len"] * (1 - 0.1 * abs(math.sin(phi)))
            org = base + Vector((0, 0.01, 0.0))
            blade("fan", org, d, ln, t["w"], t["mat"], up=nrm, thick=0.007)
            blade("fanband", org + d * ln * 0.78 + nrm * 0.0035, d, ln * 0.22, t["w"] * 0.9, t["tip"], up=nrm,
                  thick=0.008)
            blade("fanband", org + d * ln * 0.7 + nrm * 0.003, d, ln * 0.06, t["w"] * 0.85, "fur_dark", up=nrm,
                  thick=0.0075)
        P.ell("fanbase", base + a * 0.04 + Vector((0, 0.02, 0.0)), (0.07, 0.06, 0.06), t["mat"], segs=10, rings=6)


def tail_sickle(sk, cfg):
    t = cfg["sickle"]
    M, base = tail_xform(sk)
    with attach(M):
        for k, (x, ln, lat, mat) in enumerate(t["feathers"]):
            d = Vector((lat, 0.35, 0.94))
            pts = P.turtle(base + Vector((x, 0.0, 0.0)), d, ln, 9, (1, 0, 0), -t["bend"])
            rad = [(lerp(0.022, 0.006, (i / 9) ** 0.9), lerp(0.011, 0.004, i / 9)) for i in range(10)]
            P.tube("sickle", pts, rad, mat, segs=7, caps=(0.5, 0.3))


def tail_hen(sk, cfg):
    t = cfg["hentail"]
    M, base = tail_xform(sk)
    with attach(M):
        for lat, ln, mat in t["feathers"]:
            d = Vector((lat, 0.5, 0.86))
            blade("tailf", base, d, ln, t["w"], mat, up=(0, -0.86, 0.5), thick=0.008)


def tail_pointed(sk, cfg):
    """Duck / goose: a short wedge of feathers, upturned."""
    t = cfg["wedge"]
    M, base = tail_xform(sk)
    with attach(M):
        for lat in (-1, 0, 1):
            d = Vector((lat * t.get("fan", 0.25), 1.0, t["rise"]))
            blade("tailf", base, d, t["len"] * (1 - 0.15 * abs(lat)), t["w"], t["mat"], up=(0, -t["rise"], 1.0),
                  thick=0.009)
        if t.get("curl"):
            pts = P.turtle(base + Vector((0, 0.05, 0.01)), (0, 0.4, 1.0), 0.07, 4, (1, 0, 0), 90)
            P.tube("curl", pts, [0.012, 0.01, 0.008, 0.005, 0.003], t["curl"], segs=6, caps=(0.5, 0.3))


def tail_plume(sk, cfg):
    t = cfg["plume"]
    M, base = tail_xform(sk)
    with attach(M):
        P.puff("tailpuff", base + Vector((0, t["len"] * 0.4, t["rise"])), (t["w"], t["len"], t["h"]), t["mat"],
               sk.b.key + "tail", n=22, bump=0.42)


def tail_shag(sk, cfg):
    """Emu / cassowary: a droop of coarse feathers, no visible tail."""
    t = cfg["shagtail"]
    M, base = tail_xform(sk)
    with attach(M):
        P.ell("bustle", base + Vector((0, t["len"] * 0.15, -t["drop"] * 0.2)), (t["w"], t["len"], t["h"]), t["mat"],
              segs=14, rings=8)


# ---------------------------------------------------------- plumage, marks
def plumage(sk, cfg):
    """Shaggy body feathers: a ragged skirt and tufts (emu, cassowary)."""
    pl = cfg.get("shaggy")
    if not pl:
        return
    b = sk.b
    y0, zc, rx, rz, frx, frz = sk.section(pl["u"])
    with attach(sk.B):
        P.skirt("shag", (0, y0), frx * pl.get("k", 0.98), b.torso * pl["ry"], zc + frz * pl.get("top", 0.0),
                pl["z_bot"] - sk.z0 + sk.p.bz, pl["mat"], b.key, n=pl.get("n", 26), flare=pl.get("flare", 1.1),
                jag=pl.get("jag", 0.05))
        pts, nrm = [], []
        for i in range(pl.get("tufts", 8)):
            u = 0.08 + 0.6 * i / max(1, pl.get("tufts", 8) - 1)
            for th in (20, 160, 60, 120):
                p_, n_ = sk.surface(u, th)
                pts.append(p_)
                nrm.append(Vector((n_.x * 0.5, 0.7, -0.4)))
        P.tufts("shagt", pts, nrm, pl.get("tuft_len", 0.05), pl.get("tuft_r", 0.022), pl["mat"], b.key + "t")


def body_marks(sk, cfg):
    cw, cd = sk.b.chest
    for m in cfg.get("overlays", ()):
        mat, scale, dz, u0, u1 = m
        rig.build_torso(sk, mat, scale=scale, dz=dz * cd, name="mark", u0=u0, u1=u1)


# ---------------------------------------------------------- head extras
def comb(sk, g, cfg):
    c = cfg["comb"]
    pts = c["n"]
    hw, hl, hh = g["hw"], g["hl"], g["hh"]
    for i in range(pts):
        t = i / max(1, pts - 1)
        y = g["c"].y - hl * 0.55 + hl * 1.25 * t
        h = c["h"] * (0.65 + 0.7 * math.sin(math.pi * (0.15 + 0.7 * t)))
        P.ell("comb", Vector((0, y, g["c"].z + hh * 0.82 + h * 0.4)), (c["w"], hl * 0.28, h * 0.62),
              "skin_red", segs=8, rings=6)
    for sx in (-1, 1):
        w = c.get("wattle", 0)
        if w:
            P.ell("wattle", Vector((sx * hw * 0.2, g["tip"].y * 0.75 + g["c"].y * 0.25 + hl * 0.2, g["c"].z - hh * 1.15)),
                  (w * 0.4, w * 0.5, w * 1.0), "skin_red", segs=8, rings=6)
    if c.get("earlobe"):
        for sx in (-1, 1):
            P.ell("lobe", Vector((sx * hw * 0.95, g["c"].y + hl * 0.1, g["c"].z - hh * 0.1)),
                  (hw * 0.12, hl * 0.2, hh * 0.3), "feather_white", segs=8, rings=5)


def casque(sk, g, cfg):
    c = cfg["casque"]
    hw, hl, hh = g["hw"], g["hl"], g["hh"]
    ch = c["h"]
    oell("casque", g["c"] + Vector((0, -hl * 0.05, hh * 0.55 + ch * 0.5)), (0, -1, 0.5),
         (hw * 0.45, hl * 1.15, ch), "casque_brown", segs=14, rings=8)
    oell("casque", g["c"] + Vector((0, hl * 0.55, hh * 0.5 + ch * 0.2)), (0, -1, 0.1),
         (hw * 0.4, hl * 0.7, ch * 0.55), "casque_brown", segs=12, rings=6)


def turkey_head(sk, g, cfg):
    hw, hl, hh = g["hw"], g["hl"], g["hh"]
    c = g["c"]
    # snood: a fleshy flap over the beak root; wattle under the chin
    P.tube("snood", [c + Vector((0, -hl * 0.9, hh * 0.35)), c + Vector((0, -hl * 1.1, hh * 0.1)),
                     c + Vector((0, -hl * 1.15, -hh * 0.7))], [(hw * 0.2, hh * 0.1), (hw * 0.24, hh * 0.1), (hw * 0.18, hh * 0.1)],
           "skin_red", segs=8, caps=(0.5, 0.8))
    P.ell("wattle", c + Vector((0, -hl * 0.2, -hh * 1.5)), (hw * 0.55, hl * 0.35, hh * 0.9), "skin_red", segs=10, rings=7)
    for sx in (-1, 1):
        P.ell("caruncle", c + Vector((sx * hw * 0.85, hl * 0.15, hh * 0.1)), (hw * 0.2, hl * 0.4, hh * 0.5), "skin_red",
              segs=8, rings=5)


def cassowary_wattle(sk, cfg, pts, rad):
    n = len(pts)
    for t, side in ((0.55, 1), (0.55, -1)):
        i = int(t * (n - 2))
        q = pts[i]
        P.ell("wattle", q + Vector((side * rad[i] * 0.55, -rad[i] * 0.8, -rad[i] * 1.2)),
              (rad[i] * 0.55, rad[i] * 0.55, rad[i] * 1.5), "skin_red", segs=8, rings=6)
    q = pts[int(0.8 * (n - 2))]
    P.ell("nape", q + Vector((0, rad[-2] * 0.6, rad[-2] * 0.6)), (rad[-2] * 0.9, rad[-2] * 1.3, rad[-2] * 1.0),
          "skin_red", segs=8, rings=6)


def duck_ring(sk, cfg, pts, rad):
    i = 3
    q, q2 = pts[i], pts[i + 1]
    d = (q2 - q).normalized()
    P.tube("ring", [q - d * 0.012, q + d * 0.012], [rad[i] * 1.18, rad[i] * 1.18], "feather_white", segs=12,
           caps=(0.4, 0.4))


def ruff(sk, cfg, pts, rad):
    r = cfg["ruff"]
    q = pts[r["i"]]
    P.ell("ruff", q, (rad[r["i"]] * r["k"],) * 3, r["mat"], segs=14, rings=8)
    q = pts[0]
    P.ell("hackle", q + Vector((0, 0.02, 0.0)), (rad[0] * r["k"] * 1.1, rad[0] * r["k"], rad[0] * r["k"]), r["mat"],
          segs=14, rings=8)


# --------------------------------------------------------------- model
def bird_model(spec, pose):
    cfg = cfg_for(spec)
    p = pose
    if p.dead >= 0:
        p = replace(p, pitch=0, neck=p.neck + cfg.get("dead_neck", 0))
    p = replace(p, pitch=p.pitch + (0 if p.dead >= 0 else cfg["pitch"]))
    sk = Skel(spec, p)
    rig.build_torso(sk)
    body_marks(sk, cfg)
    for leg in sk.legs:
        bird_leg(sk, leg, cfg)
    bird_neck(sk, cfg)
    bird_head(sk, cfg)
    wings(sk, cfg)
    plumage(sk, cfg)
    cfg["tail"](sk, cfg)
    return sk


# --------------------------------------------------------------- poses
def _reach(spec, z_target, head, pitch, bz, ty=0.0, lo=-150, hi=20, **kw):
    """Neck delta (degrees) that puts the beak tip at z_target."""
    cfg = cfg_for(spec)
    best, bd = None, 1e9
    for dn in range(hi, lo - 1, -3):
        p = Pose(neck=dn, head=head, pitch=pitch + cfg["pitch"], bz=bz, ty=ty)
        z = Skel(spec, p).nose_world().z
        if abs(z - z_target) < bd:
            best, bd = dn, abs(z - z_target)
    return best


def walk_pose(spec, i):
    cfg = cfg_for(spec)
    ph = (i - 1) / rig.TROT_N
    base = rig.trot_pose(spec, i)
    bob = cfg.get("bob", 7)
    feet = {n: (dy * 1.7, dz * 1.5) for n, (dy, dz) in base.feet.items()}
    return replace(base, feet=feet, neck=bob * math.sin(2 * math.pi * ph), head=-bob * 0.9 * math.sin(2 * math.pi * ph),
                   tail=cfg.get("tail_bob", 3) * math.sin(4 * math.pi * ph))


def graze_set(spec):
    cfg = cfg_for(spec)
    out = []
    for frac, head, pitch, bz in ((0.35, 50, 4, 0.0), (0.0, 62, 8, 0.01)):
        z = 0.02 + frac * spec.height * 0.5
        dn = _reach(spec, z, head, pitch, bz, ty=-0.01)
        out.append(Pose(neck=dn, head=head, pitch=pitch, bz=bz, ty=-0.01, tail=-4))
    return out


def peck_set(spec):
    cfg = cfg_for(spec)
    pk = cfg.get("peck", "stab")
    th = spec.neck_angle
    if pk == "lunge":
        return [Pose(ty=0.03, pitch=-4, neck=8, head=-14, ears=25, tail=6),
                Pose(ty=-0.03, pitch=2, neck=-(th - 35), head=6, jaw=30, ears=35, tail=8),
                Pose(ty=-0.09, pitch=5, neck=-(th - 8), head=14, jaw=38, ears=45, tail=10),
                Pose(ty=-0.03, pitch=0, neck=-10, head=-4, jaw=8, ears=15)]
    strike = _reach(spec, 0.05, 55, 8, 0.01, ty=-0.07)
    return [Pose(ty=0.03, pitch=-7, neck=14, head=-14, ears=15, tail=6),
            Pose(ty=-0.03, pitch=3, neck=-28, head=34, jaw=24, ears=20),
            Pose(ty=-0.07, bz=0.01, pitch=8, neck=strike, head=55, jaw=4, ears=25, tail=8),
            Pose(ty=-0.02, pitch=0, neck=-8, head=12, ears=8)]


def kick_set(spec):
    legs = (spec.legs or bird_legs)(spec)
    L = legs[0].l1 + legs[0].l2
    kl = "hl"
    return [Pose(ty=0.03, pitch=-8, neck=10, head=-12, ears=12, tail=6, feet={"hl": (0.05, 0.0), "hr": (0.0, 0.0)}),
            Pose(ty=0.02, pitch=-13, neck=14, head=-16, ears=25, tail=10,
                 feet={kl: (-0.25 * L, 0.45 * L), "hr": (0.03, 0.0)}),
            Pose(ty=0.0, bz=-0.01, pitch=-20, neck=10, head=-10, ears=45, tail=18,
                 feet={kl: (-0.95 * L, 0.55 * L), "hr": (0.06, 0.0)}),
            Pose(ty=0.02, pitch=-6, neck=4, head=-4, ears=15, feet={kl: (-0.2 * L, 0.2 * L)})]


def flinch_set(spec):
    return [Pose(ty=0.06, bz=0.01, pitch=-9, neck=16, head=-20, yaw=18, ears=42, tail=14,
                 feet={"hl": (0.05, 0.02), "hr": (0.03, 0.0)}),
            Pose(ty=0.03, pitch=-4, neck=8, head=-10, yaw=8, ears=20, tail=6)]


def rest_set(spec):
    """Sitting: belly on the ground, legs folded away, neck drawn in with the
    head sunk low on the shoulders."""
    cd = spec.chest[1]
    drop = max(0.0, spec.height - 2 * cd - 0.0 + spec.belly * 0.5)
    return Pose(bz=drop, neck=-(spec.neck_angle * 0.3), head=30, tail=-6,
                feet={"hl": (-0.03, 0.0), "hr": (-0.03, 0.0)})


def bird_poses(spec):
    out = {}
    for i in range(1, rig.TROT_N + 1):
        out["walk%02d" % i] = walk_pose(spec, i)
    kind = spec.attack
    atk = peck_set(spec) if kind == "peck" else kick_set(spec)
    for i, p in enumerate(atk, 1):
        out["%s%d" % (kind, i)] = p
    for name, p in zip(rig.GRAZE, graze_set(spec)):
        out[name] = p
    for name, p in zip(rig.FLINCH, flinch_set(spec)):
        out[name] = p
    out["rest"] = rest_set(spec)
    return out


# --------------------------------------------------------------- species
def toes3(l=0.05, spread=30):
    return [(-spread, l * 0.85), (0, l), (spread, l * 0.85)]


CFG = {
    "chicken": dict(
        pitch=-12, leg_u=0.5, neck_s=-14, bob=9, beak="cone", hook=0.2, dead_neck=-20,
        toes=toes3(0.05), hind=0.028, claw=0.014, spur=True, toe_r=0.0105,
        wing=dict(u=0.55, len=0.2, h=0.13, mat="feather_rust", tip="feather_green", thick=0.018),
        overlays=((("feather_black"), (0.92, 0.86), -0.2, 0.45, 0.98),),
        neck_mats=[(0.0, 1.0, "feather_rust", 1.0)],
        neck_extra=(ruff,), ruff=dict(i=2, k=1.25, mat="feather_rust"),
        head_extra=(comb,), comb=dict(n=5, h=0.05, w=0.012, wattle=0.022, earlobe=True),
        tail=tail_sickle,
        sickle=dict(bend=118, feathers=[(0.0, 0.42, 0.0, "feather_green"), (-0.018, 0.34, -0.2, "feather_green"),
                                        (0.018, 0.34, 0.2, "feather_green"), (0.0, 0.2, 0.0, "feather_rust"),
                                        (-0.02, 0.15, -0.35, "feather_rust"), (0.02, 0.15, 0.35, "feather_rust")])),
    "chicken_f": dict(
        pitch=-8, neck_s=-8, ruff=dict(i=2, k=1.12, mat="feather_hen"), spur=False,
        wing=dict(u=0.55, len=0.2, h=0.13, mat="feather_hen", tip="feather_cream", thick=0.02),
        overlays=((("feather_cream"), (0.9, 0.85), -0.2, 0.45, 0.98),),
        neck_mats=[(0.0, 1.0, "feather_hen", 1.0)],
        comb=dict(n=3, h=0.026, w=0.01, wattle=0.012, earlobe=False),
        tail=tail_hen,
        hentail=dict(w=0.07, feathers=[(-0.25, 0.14, "feather_hen"), (0.0, 0.17, "feather_hen"),
                                       (0.25, 0.14, "feather_hen"), (0.0, 0.1, "feather_cream")])),
    "duck": dict(
        pitch=-5, leg_u=0.3, neck_s=12, bob=6, beak="flat", dead_neck=-10, foot_dy=0.0, foot_spread=0.9,
        toes=toes3(0.05, 26), web=0.05, toe_r=0.009,
        wing=dict(u=0.5, len=0.26, h=0.11, mat="feather_duck", tip="feather_black", thick=0.016, out=0.93),
        overlays=((("feather_hen"), (0.9, 0.85), -0.2, 0.5, 0.97),),
        neck_mats=[(0.0, 0.22, "feather_hen", 1.0), (0.2, 1.0, "feather_mallard", 1.0)],
        neck_extra=(duck_ring,), tail=tail_pointed,
        wedge=dict(len=0.13, w=0.06, rise=0.45, mat="feather_duck")),
    "goose": dict(
        pitch=-9, leg_u=0.4, neck_s=14, neck_pow=0.7, bob=6, beak="cone", dead_neck=-15,
        toes=toes3(0.06, 26), web=0.06, toe_r=0.0095,
        wing=dict(u=0.5, len=0.28, h=0.12, mat="feather_grey", tip="feather_white", thick=0.017, out=0.94),
        neck_mats=[(0.0, 1.0, "feather_white", 1.0)], peck="lunge",
        tail=tail_pointed,
        wedge=dict(len=0.16, w=0.06, rise=0.35, mat="feather_white")),
    "turkey": dict(
        pitch=-8, leg_u=0.5, neck_s=-6, bob=8, beak="cone", hook=0.25, dead_neck=-15,
        toes=toes3(0.06), hind=0.03, claw=0.014, toe_r=0.0125,
        wing=dict(u=0.5, len=0.24, h=0.15, mat="feather_bronze", tip="feather_band", thick=0.02),
        overlays=((("feather_black"), (0.9, 0.82), -0.22, 0.4, 0.98),),
        neck_mats=[(0.0, 0.3, "feather_bronze", 1.1), (0.25, 1.0, "skin_bluegrey", 1.0)],
        head_extra=(turkey_head,), tail=tail_fan,
        fan=dict(n=13, spread=82, len=0.42, w=0.082, lean=58, mat="feather_bronze", tip="feather_band")),
    "cassowary": dict(
        pitch=0, leg_u=0.45, neck_s=10, bob=7, beak="cone", hook=0.12, dead_neck=-20,
        toes=[(-30, 0.085), (0, 0.105), (30, 0.085)], claw=0.03, claw_all=0.012, toe_r=0.022,
        wing=dict(u=0.5, len=0.22, h=0.14, mat="feather_black", thick=0.02),
        neck_mats=[(0.0, 0.18, "feather_black", 1.2), (0.12, 1.0, "skin_blue", 1.0)],
        neck_extra=(cassowary_wattle,), head_extra=(casque,), casque=dict(h=0.11),
        shaggy=dict(u=0.3, ry=0.36, z_bot=0.3, mat="feather_black", k=1.05, tufts=7, tuft_len=0.07, tuft_r=0.03),
        tail=tail_shag, shagtail=dict(len=0.17, w=0.13, h=0.11, drop=0.1, mat="feather_black")),
    "emu": dict(
        pitch=-2, leg_u=0.45, neck_s=10, bob=8, beak="cone", dead_neck=-20,
        toes=[(-26, 0.065), (0, 0.085), (26, 0.065)], claw=0.016, toe_r=0.0135,
        wing=dict(u=0.5, len=0.1, h=0.07, mat="feather_emu_dark", thick=0.012, out=1.0),
        neck_mats=[(0.0, 0.25, "feather_emu", 1.1), (0.2, 1.0, "skin_bluegrey", 1.0)],
        shaggy=dict(u=0.35, ry=0.38, z_bot=0.27, mat="feather_emu", k=1.08, tufts=9, tuft_len=0.075, tuft_r=0.03, jag=0.07),
        tail=tail_shag, shagtail=dict(len=0.2, w=0.12, h=0.1, drop=0.08, mat="feather_emu")),
    "ostrich": dict(
        pitch=-2, leg_u=0.5, neck_s=8, neck_pow=0.55, bob=6, beak="flat", dead_neck=-20,
        toes=[(0, 0.13), (26, 0.075)], claw=0.03, toe_r=0.028,
        wing=dict(u=0.55, len=0.26, h=0.16, mat="feather_white", thick=0.026, puff=True, out=0.96, up=0.55),
        neck_mats=[(0.0, 0.1, "feather_white", 1.25), (0.06, 1.0, "skin_pinkgrey", 1.0)],
        tail=tail_plume, plume=dict(len=0.24, w=0.15, h=0.13, rise=0.1, mat="feather_white")),
}


def spec_for(row):
    b = BODIES[row.key]
    kw = dict(b)
    L = R.body_length(row.size)
    kw.setdefault("eye", min(0.03, max(0.009, 0.01 / L)))
    return Body(key=row.key, length=L, attack=row.attack, archetype="bird", legs=bird_legs, model=bird_model, **kw)


# Body proportions (unit = body length). torso: rump to chest; height: top of the back.
BODIES = {
    "chicken": dict(
        height=0.43, torso=0.46, chest=(0.13, 0.15), waist=(0.135, 0.16), hips=(0.115, 0.145), rump_z=-0.03,
        torso_p=2.0, belly=0.01, leg_r=(0.05, 0.04, 0.0165, 0.013), stance=0.72, neck_len=0.19, neck_angle=70,
        neck_r=(0.068, 0.036), neck_y=0.1, head=(0.046, 0.056, 0.05), head_angle=22, muzzle=(0.055, 0.02, 0.02),
        muzzle_drop=22, ears=None, tail=None, eye=0.0125,
        pal=dict(coat="feather_rust", head="feather_rust", nose="beak_yellow", legs="feather_rust",
                 lower="scale_legs_yellow", toe="scale_legs_yellow", eye="eye_black")),
    "duck": dict(
        height=0.27, torso=0.58, chest=(0.115, 0.11), waist=(0.125, 0.115), hips=(0.1, 0.1), rump_z=0.03,
        torso_p=2.3, belly=0.01, leg_r=(0.04, 0.034, 0.014, 0.011), stance=0.7, neck_len=0.17, neck_angle=56,
        neck_r=(0.058, 0.04), neck_y=0.08, head=(0.05, 0.058, 0.05), head_angle=14, muzzle=(0.075, 0.034, 0.011),
        muzzle_drop=10, ears=None, tail=None, eye=0.012,
        pal=dict(coat="feather_duck", head="feather_mallard", nose="beak_yellow", legs="feather_duck",
                 lower="beak_orange", toe="beak_orange")),
    "goose": dict(
        height=0.38, torso=0.6, chest=(0.11, 0.115), waist=(0.12, 0.12), hips=(0.1, 0.115), rump_z=0.03,
        torso_p=2.3, belly=0.008, leg_r=(0.04, 0.034, 0.015, 0.012), stance=0.72, neck_len=0.4, neck_angle=66,
        neck_r=(0.052, 0.03), neck_y=0.07, head=(0.036, 0.05, 0.036), head_angle=16, muzzle=(0.065, 0.024, 0.017),
        muzzle_drop=12, ears=None, tail=None, eye=0.0095,
        pal=dict(coat="feather_white", head="feather_white", nose="beak_orange", legs="feather_white",
                 lower="beak_orange", toe="beak_orange")),
    "turkey": dict(
        height=0.46, torso=0.5, chest=(0.15, 0.165), waist=(0.15, 0.17), hips=(0.13, 0.155), rump_z=-0.02,
        torso_p=2.0, belly=0.012, leg_r=(0.055, 0.045, 0.019, 0.015), stance=0.72, neck_len=0.2, neck_angle=60,
        neck_r=(0.058, 0.034), neck_y=0.09, head=(0.034, 0.046, 0.04), head_angle=24, muzzle=(0.05, 0.018, 0.018),
        muzzle_drop=22, ears=None, tail=None, eye=0.011,
        pal=dict(coat="feather_bronze", head="skin_bluegrey", nose="beak_dark", legs="feather_bronze",
                 lower="skin_pinkgrey", toe="skin_pinkgrey")),
    "cassowary": dict(
        height=0.7, torso=0.6, chest=(0.17, 0.19), waist=(0.2, 0.21), hips=(0.2, 0.22), rump_z=-0.04,
        torso_p=2.2, belly=0.01, leg_r=(0.075, 0.058, 0.03, 0.025), stance=0.7, neck_len=0.42, neck_angle=66,
        neck_r=(0.065, 0.034), neck_y=0.1, head=(0.036, 0.052, 0.042), head_angle=18, muzzle=(0.08, 0.021, 0.019),
        muzzle_drop=18, ears=None, tail=None, eye=0.011,
        pal=dict(coat="feather_black", head="skin_blue", nose="beak_dark", legs="scale_legs", lower="scale_legs",
                 toe="scale_legs")),
    "emu": dict(
        height=0.66, torso=0.54, chest=(0.14, 0.17), waist=(0.16, 0.18), hips=(0.16, 0.19), rump_z=-0.04,
        torso_p=2.2, belly=0.008, leg_r=(0.06, 0.045, 0.024, 0.02), stance=0.7, neck_len=0.46, neck_angle=74,
        neck_r=(0.052, 0.028), neck_y=0.1, head=(0.03, 0.046, 0.034), head_angle=16, muzzle=(0.052, 0.018, 0.015),
        muzzle_drop=16, ears=None, tail=None, eye=0.0095,
        pal=dict(coat="feather_emu", head="feather_emu_dark", nose="beak_dark", legs="scale_legs", lower="scale_legs",
                 toe="scale_legs")),
    "ostrich": dict(
        height=0.76, torso=0.46, chest=(0.19, 0.2), waist=(0.21, 0.215), hips=(0.19, 0.205), rump_z=-0.03,
        torso_p=2.1, belly=0.01, leg_r=(0.07, 0.052, 0.027, 0.023), stance=0.62, neck_len=0.4, neck_angle=80,
        neck_r=(0.05, 0.026), neck_y=0.1, head=(0.026, 0.042, 0.03), head_angle=14, muzzle=(0.05, 0.02, 0.01),
        muzzle_drop=10, ears=None, tail=None, eye=0.011,
        pal=dict(coat="feather_black", head="skin_pinkgrey", nose="skin_pinkgrey", legs="skin_pinkgrey",
                 lower="skin_pinkgrey", toe="skin_pinkgrey")),
}


def _spec(row):
    b = spec_for(row)
    b.poses = bird_poses(b)
    if row.dimorphic and row.key == "chicken":
        f = replace(b, pal=dict(coat="feather_hen", head="feather_hen", nose="beak_yellow", legs="feather_hen",
                                lower="scale_legs_yellow", toe="scale_legs_yellow", sex="f"),
                    torso=0.46, chest=(0.14, 0.16), hips=(0.125, 0.15), female=None)
        f.poses = bird_poses(f)
        b.female = f
    return b


def build(row, static=False):
    if row.key not in BODIES:
        print("bird: no spec for %s yet; skipped" % row.key)
        return []
    return render(_spec(row), static, row.size)
