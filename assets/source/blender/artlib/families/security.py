"""AX-A5 fire foam and security (AX-AR2; ART spec §12-§16): the foam
popper, the foam and warhead charges, the foam turret, the rocket battery,
the foam shells and antimatter warhead items, the foam stain, and the
rocket and foam projectiles, plus their Architect and item icons. Our own
designs: nothing traced or reconstructed. Fire-foam gear is painted a
safety teal (`foam_paint`) so it never reads as an incendiary.

Turrets follow fortify.py (a base and a gun baked in 16 aim directions,
frame 0 south, clockwise on screen at 22.5 degrees; every frame shares the
base's canvas, origin and zmax, so `mount` is the same pixel in all of
them; gun frames carry `mount` and `muzzle`; the base's status lamp is its
glow layer, drawn while powered).

- foam_popper (defense, 1x1): a squat teal foam bottle on a three-legged
  stand, a perforated burst dome on top, a red heat bulb on a stalk and a
  pressure gauge. Unpowered, no glow.
- ied_foam (defense, 1x1, "foam charge"): the charges' mound with two foam
  shells (teal bodies, white bands) wired to a pressure plate.
- ied_warhead (defense, 1x1, "warhead charge"): one fat soot-black warhead
  with violet bands and a domed nose, half sunk in the mound, wired to a
  plate.
- foam_turret_base (defense, 1x1) + foam_turret_gun_00..15: a pedestal
  with a chemfuel-fed foam tank on its flank (glow: status lamp); the gun
  is a short wide nozzle cannon with a teal foam hopper.
- rocket_battery_base (defense, 2x2) + rocket_battery_gun_00..15: a heavy
  pedestal with a rocket crate (glow: status lamp); the gun is a boxy
  launcher pod with twelve tube mouths (3 x 4) on its face.
- item/foam_shells: the shell crate in the foam colours (item "foam
  shells"); item/antimatter_warhead: one warhead on a padded cradle (item
  "antimatter warhead").
- overlay/foam_0..2: the foam stain, flat 1x1 decals of creamy foam lobes
  and bubbles, no outline, edges fading out (like ash_<n>).
- effect/rocket_projectile (96x32) and effect/foam_projectile (64x32):
  particles pointing east (the renderer turns them to the heading), origin
  at the nose; the rocket trails a flame, the foam glob drops a spray.

    make art FAMILIES=security
"""
import math
import os
import tempfile

import numpy as np
from mathutils import Matrix

from .. import contracts as C
from .. import stage as S
from . import effect as E
from . import fortify as F
from . import overlay as O
from . import power as P

FAMILY = "defense"
GUN_FRAMES = F.GUN_FRAMES
GUN_STEP = F.GUN_STEP
FOAM_CHARGE = ("foam_paint", "foam", "steel_dark")     # shell body, band, nose


# ---------------------------------------------------------------- popper
def _foam_popper():
    """1x1: tripod stand, the foam bottle, a burst dome, heat bulb, gauge."""
    cx, cy = 0.5, -0.52
    for k in range(3):
        a = math.radians(90 + 120 * k)
        P._rod("leg%d" % k, (cx + 0.10 * math.cos(a), cy + 0.10 * math.sin(a), 0.16),
               (cx + 0.30 * math.cos(a), cy + 0.30 * math.sin(a), 0.0), 0.022, "metal_dark", segs=6, bevel=0.0)
        S.cyl("foot%d" % k, cx + 0.30 * math.cos(a), cy + 0.30 * math.sin(a), 0.045, 0.0, 0.02, "metal_dark",
              segs=8, bevel=0.0)
    S.cyl("ring", cx, cy, 0.20, 0.12, 0.16, "steel_dark", segs=20, bevel=0.008)
    S.cyl("bottle", cx, cy, 0.18, 0.16, 0.50, "foam_paint", segs=22, bevel=0.03)
    S.cyl("band", cx, cy, 0.185, 0.24, 0.29, "foam", segs=22, bevel=0.0)
    S.ellipsoid("shoulder", cx, cy, 0.50, 0.18, 0.18, 0.08, "foam_paint", segs=22, rings=8)
    S.cyl("neck", cx, cy, 0.07, 0.54, 0.60, "steel_dark", segs=12, bevel=0.0)
    S.ellipsoid("dome", cx, cy, 0.60, 0.13, 0.13, 0.09, "steel", segs=18, rings=8)
    for k in range(8):                                               # burst holes round the dome
        a = 2 * math.pi * k / 8
        S.cyl("hole%d" % k, cx + 0.085 * math.cos(a), cy + 0.085 * math.sin(a), 0.018, 0.63, 0.66, "fur_black",
              segs=6, bevel=0.0)
    S.cyl("hole_top", cx, cy, 0.02, 0.68, 0.70, "fur_black", segs=6, bevel=0.0)
    P._rod("stalk", (cx + 0.12, cy - 0.04, 0.50), (cx + 0.24, cy - 0.10, 0.66), 0.014, "steel", segs=6, bevel=0.0)
    S.ellipsoid("bulb", cx + 0.25, cy - 0.105, 0.68, 0.035, 0.035, 0.045, "med_red", segs=10, rings=6)
    P._rod("gauge", (cx - 0.08, cy - 0.19, 0.38), (cx - 0.08, cy - 0.17, 0.38), 0.045, "ceramic", segs=14, bevel=0.0)
    S.box("needle", cx - 0.085, cy - 0.195, 0.38, cx - 0.05, cy - 0.19, 0.392, "med_red", bevel=0.0)
    return (1, 1), 0.74


# ------------------------------------------------------- the warhead charge
def _warhead(cx, cy, z, length, r):
    """A warhead lying east-west centred at (cx, cy, z): a soot-black body,
    two violet bands, a domed nose (east) and a finned tail cap (west)."""
    x0, x1 = cx - length / 2, cx + length / 2
    P._rod("wh_body", (x0, cy, z), (x1, cy, z), r, "mech_soot", segs=20, bevel=0.012)
    for k, t in enumerate((0.28, 0.62)):
        xa = x0 + length * t
        P._rod("wh_band%d" % k, (xa, cy, z), (xa + 0.05, cy, z), r + 0.006, "warhead_band", segs=20, bevel=0.0)
    S.ellipsoid("wh_nose", x1, cy, z, r * 0.95, r, r, "mech_soot", segs=20, rings=10)
    S.ellipsoid("wh_tip", x1 + r * 0.82, cy, z, 0.03, 0.03, 0.03, "warhead_band", segs=10, rings=6)
    P._rod("wh_tail", (x0 - 0.06, cy, z), (x0, cy, z), r * 0.7, "metal_dark", segs=16, bevel=0.0)
    for k in range(4):
        n0 = len(S.objects())
        S.box("wh_fin%d" % k, x0 - 0.08, cy - 0.006, z, x0 + 0.06, cy + 0.006, z + r + 0.04, "metal_dark", bevel=0.0)
        P._spin(S.objects()[n0:], (0, cy, z), "X", 45 + 90 * k)


def _ied_warhead():
    """1x1: the mound, one warhead half sunk in it, wired to the plate."""
    S.blob("mound", 0.48, -0.54, 0.0, 0.42, "soil", seed=23, lumps=0.12, squash=0.22)
    _warhead(0.42, -0.54, 0.09, 0.46, 0.12)
    S.box("plate", 0.70, -0.30, 0.06, 0.92, -0.10, 0.10, "metal_dark", bevel=0.01)
    S.box("plate_top", 0.72, -0.28, 0.10, 0.90, -0.12, 0.115, "steel_dark", bevel=0.005)
    S.box("fuse_box", 0.70, -0.62, 0.12, 0.80, -0.46, 0.22, "steel_dark", bevel=0.01)
    P._rod("wire", (0.75, -0.46, 0.20), (0.80, -0.22, 0.11), 0.012, "fur_black", segs=6, bevel=0.0)
    return (1, 1), 0.26


# ------------------------------------------------------------- foam turret
FT_MOUNT = (0.5, -0.5, 0.48)
FT_ZMAX = 0.82
FT_REACH = 0.44
FT_BORE = 0.13


def _foam_turret_base():
    """1x1: footing, pedestal, a foam tank on the west flank piped to the
    ring, a fuel inlet, the status lamp."""
    cx, cy = FT_MOUNT[0], FT_MOUNT[1]
    S.box("footing", 0.12, -0.88, 0.0, 0.88, -0.12, 0.06, "steel_dark", bevel=0.02)
    for x, y in ((0.18, -0.18), (0.82, -0.18), (0.18, -0.82), (0.82, -0.82)):
        S.cyl("anchor", x, y, 0.026, 0.06, 0.085, "metal_dark", segs=6, bevel=0.0)
    n0 = len(S.objects())
    S.cyl("pedestal", cx + 0.06, cy, 0.24, 0.06, 0.38, "steel", segs=8, r_top=0.21, bevel=0.02)
    P._spin(S.objects()[n0:], (cx + 0.06, cy, 0.0), "Z", 22.5)
    S.cyl("collar", cx, cy, 0.22, 0.38, 0.42, "metal_dark", segs=24, bevel=0.01)
    S.cyl("slew_ring", cx, cy, 0.18, 0.42, FT_MOUNT[2], "steel_dark", segs=24, bevel=0.01)
    # the foam tank lying north-south on the west flank
    P._rod("tank", (0.22, -0.82, 0.20), (0.22, -0.30, 0.20), 0.11, "foam_paint", segs=18, bevel=0.02)
    P._rod("tank_band", (0.22, -0.58, 0.20), (0.22, -0.54, 0.20), 0.115, "foam", segs=18, bevel=0.0)
    S.cyl("tank_cap", 0.22, -0.36, 0.04, 0.30, 0.34, "brass", segs=10, bevel=0.0)
    P._rod("feed", (0.30, -0.44, 0.26), (0.38, -0.46, 0.40), 0.025, "copper", segs=8)
    # fuel inlet and the status lamp (south-east)
    S.box("inlet", 0.70, -0.86, 0.06, 0.84, -0.74, 0.16, "fuel_drum", bevel=0.01)
    F._ring_lamp(0.66, -0.73, 0.28)
    return (1, 1), FT_ZMAX, {"mount": FT_MOUNT}


def _foam_turret_gun(frame):
    """The nozzle cannon along +X about the pivot, turned to its frame."""
    def build():
        px, py, pz = FT_MOUNT
        bz = pz + FT_BORE
        S.cyl("yoke", px, py, 0.16, pz, pz + 0.05, "metal_dark", segs=20, bevel=0.01)
        n0 = len(S.objects())
        S.box("body", px - 0.16, py - 0.11, pz + 0.05, px + 0.10, py + 0.11, pz + 0.22, "steel", bevel=0.03)
        S.box("roof", px - 0.13, py - 0.09, pz + 0.22, px + 0.07, py + 0.09, pz + 0.245, "steel_dark", bevel=0.01)
        # the foam hopper on top, its cap
        S.cyl("hopper", px - 0.05, py, 0.075, pz + 0.22, pz + 0.33, "foam_paint", segs=16, bevel=0.012)
        S.cyl("hopper_cap", px - 0.05, py, 0.06, pz + 0.33, pz + 0.35, "foam", segs=16, bevel=0.006)
        # pump block at the rear, the wide nozzle forward with a flared mouth
        S.box("pump", px - 0.24, py - 0.08, pz + 0.07, px - 0.15, py + 0.08, pz + 0.18, "metal_dark", bevel=0.015)
        P._rod("nozzle", (px + 0.10, py, bz), (px + 0.34, py, bz), 0.05, "foam_paint", segs=16, bevel=0.006)
        P._rod("band", (px + 0.20, py, bz), (px + 0.23, py, bz), 0.054, "foam", segs=16, bevel=0.0)
        cone = S.cyl("mouth", 0, 0, 0.05, 0.0, FT_REACH - 0.34, "steel_dark", segs=16, r_top=0.075, bevel=0.0)
        cone.matrix_world = Matrix.Translation((px + 0.34, py, bz)) @ Matrix.Rotation(math.radians(90), 4, "Y")
        P._spin(S.objects()[n0:], (px, py, 0.0), "Z", F._aim(frame))
        a = math.radians(F._aim(frame))
        return (1, 1), FT_ZMAX, {"mount": FT_MOUNT,
                                 "muzzle": (px + FT_REACH * math.cos(a), py + FT_REACH * math.sin(a), bz)}
    return build


# ------------------------------------------------------------ rocket battery
RB_MOUNT = (1.0, -1.0, 0.58)
RB_ZMAX = 1.20
RB_REACH = 0.56
RB_BORE = 0.24


def _rocket_battery_base():
    cx, cy = RB_MOUNT[0], RB_MOUNT[1]
    F._pedestal(cx, cy, 0.44, 0.48, "steel_dark", RB_MOUNT[2])
    # a rocket crate on the west flank: lid open, three rocket noses showing
    S.box("crate", 0.18, -1.70, 0.06, 0.56, -1.04, 0.36, "wood", bevel=0.02)
    S.box("crate_rim", 0.16, -1.72, 0.36, 0.58, -1.02, 0.39, "wood_plank", bevel=0.01)
    for k, y in enumerate((-1.56, -1.37, -1.18)):
        S.cyl("rocket_nose%d" % k, 0.37, y, 0.055, 0.30, 0.44, "med_red", segs=12, r_top=0.015, bevel=0.0)
    S.box("fuel_tank", 1.46, -0.40, 0.06, 1.84, -0.20, 0.34, "fuel_drum", bevel=0.02)
    F._ring_lamp(1.34, -1.48, 0.32)
    P._hazard(0.70, 1.10, -1.47, -1.455, 0.12, 0.20, n=3)
    return (2, 2), RB_ZMAX, {"mount": RB_MOUNT}


def _rocket_battery_gun(frame):
    """A launcher pod on a yoke, twelve tube mouths (3 rows x 4) facing +X."""
    def build():
        px, py, pz = RB_MOUNT
        bz = pz + RB_BORE
        S.cyl("yoke", px, py, 0.26, pz, pz + 0.06, "metal_dark", segs=24, bevel=0.012)
        n0 = len(S.objects())
        for k, dy in enumerate((-0.30, 0.30)):                       # trunnion arms
            S.box("arm%d" % k, px - 0.10, py + dy - 0.04, pz + 0.06, px + 0.10, py + dy + 0.04, bz + 0.06,
                  "steel_dark", bevel=0.012)
        S.box("pod", px - 0.30, py - 0.26, bz - 0.17, px + RB_REACH - 0.04, py + 0.26, bz + 0.17, "armour_olive",
              bevel=0.03)
        S.box("pod_roof", px - 0.26, py - 0.22, bz + 0.17, px + RB_REACH - 0.10, py + 0.22, bz + 0.19, "steel_dark",
              bevel=0.01)
        S.box("face", px + RB_REACH - 0.06, py - 0.25, bz - 0.16, px + RB_REACH, py + 0.25, bz + 0.16, "metal_dark",
              bevel=0.012)
        for i, z in enumerate((bz - 0.10, bz, bz + 0.10)):
            for j, dy in enumerate((-0.18, -0.06, 0.06, 0.18)):
                P._rod("tube%d_%d" % (i, j), (px + RB_REACH - 0.02, py + dy, z), (px + RB_REACH + 0.005, py + dy, z),
                       0.038, "fur_black", segs=10, bevel=0.0)
                S.ellipsoid("tip%d_%d" % (i, j), px + RB_REACH - 0.025, py + dy, z, 0.012, 0.024, 0.024, "med_red",
                            segs=8, rings=4)
        S.box("sight", px - 0.10, py - 0.30, bz + 0.10, px + 0.06, py - 0.24, bz + 0.20, "steel", bevel=0.008)
        P._spin(S.objects()[n0:], (px, py, 0.0), "Z", F._aim(frame))
        a = math.radians(F._aim(frame))
        return (2, 2), RB_ZMAX, {"mount": RB_MOUNT,
                                 "muzzle": (px + RB_REACH * math.cos(a), py + RB_REACH * math.sin(a), bz)}
    return build


TURRETS = {
    # prefix: (base, gun, building name)
    "foam_turret": (_foam_turret_base, _foam_turret_gun, "foam turret"),
    "rocket_battery": (_rocket_battery_base, _rocket_battery_gun, "rocket battery"),
}


# ------------------------------------------------------------------- items
def _antimatter_warhead():
    """One warhead on a padded cradle (two felt-lined saddles on a plank)."""
    S.box("plank", 0.14, -0.70, 0.0, 0.86, -0.36, 0.04, "wood_plank", bevel=0.01)
    for k, x in enumerate((0.30, 0.66)):
        S.box("saddle%d" % k, x - 0.05, -0.68, 0.04, x + 0.05, -0.38, 0.12, "wood", bevel=0.01)
        S.box("pad%d" % k, x - 0.055, -0.62, 0.11, x + 0.055, -0.44, 0.14, "burlap", bevel=0.01)
    _warhead(0.48, -0.53, 0.25, 0.48, 0.12)
    return S.render_asset("item", "antimatter_warhead", zmax=0.40, extra={"item": "antimatter warhead"})


def _foam_shells():
    F._shell_stack(*FOAM_CHARGE)
    return S.render_asset("item", "foam_shells", zmax=0.22, extra={"item": "foam shells", "stack": True})


# -------------------------------------------------------------------- stain
def _foam_stain(k):
    """128x128 foam decal: soft creamy lobes, darker under-edges, bubbles."""
    foam = E._mat("foam")
    n = C.PPT
    yy, xx = np.mgrid[0:n, 0:n].astype(np.float32) + 0.5
    rng = np.random.RandomState(2300 + k)
    img = np.zeros((n, n, 4), np.float32)
    cov = np.zeros((n, n), np.float32)
    for _ in range(9):                                               # lobes
        px, py = n / 2 + rng.uniform(-30, 30), n / 2 + rng.uniform(-30, 30)
        r = rng.uniform(13, 26)
        cov = np.maximum(cov, np.clip((r - np.hypot((xx - px) * 0.9, yy - py)) / 6.0, 0, 1))
    img[..., :3] = foam["base"]
    img[..., 3] = cov * 0.82
    # shaded under-edge: the south rim of each lobe
    edge = np.clip(cov - np.roll(cov, -4, axis=0), 0, 1)
    E._over(img, edge * (cov > 0.2), foam["shadow"], 0.7)
    for _ in range(28):                                              # bubbles: a light rim, a dark dot
        px, py = n / 2 + rng.uniform(-38, 38), n / 2 + rng.uniform(-38, 38)
        r = rng.uniform(1.8, 4.5)
        d = np.hypot(xx - px, yy - py)
        m = (cov > 0.6).astype(np.float32)
        E._over(img, np.clip(1.2 - np.abs(d - r) / 0.9, 0, 1) * m, foam["light"] * 1.03, 0.9)
        E._over(img, np.clip(1.0 - d / (r * 0.45), 0, 1) * m, foam["shadow"], 0.45)
    return O._save_flat("foam_%d" % k, n, n, img, {"overlay": "foam", "variant": k, "tileable": False,
                                                   "marking": True})


# -------------------------------------------------------------- projectiles
def _rocket_projectile():
    """96x32, pointing east, origin at the nose: red nose, steel body, dark
    fins and a flame tongue trailing west."""
    steel, red, dark = E._mat("steel"), E._mat("med_red"), E._mat("metal_dark")
    fire_b, fire_c = E._mat("fire_body"), E._mat("fire_core")
    w, h = 96, 32
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    cy, nose, tail = h / 2, 90.0, 46.0
    img = np.zeros((h, w, 4), np.float32)
    dy = np.abs(yy - cy)
    # flame: a tapering tongue from the tail west, core near the nozzle
    t = np.clip((tail - xx) / tail, 0, 1)
    flame = np.clip(1.0 - (dy - 6.0 * (1 - t) ** 0.6) / 2.5, 0, 1) * (xx < tail) * (1 - E._smooth(0.6, 1.0, t))
    E._over(img, flame, E._mix(fire_c["light"], fire_b["base"], t[..., None]), 0.95)
    # fins at the tail
    fins = (np.abs(dy - 6.5) < 2.5) & (xx > tail - 2) & (xx < tail + 9)
    E._over(img, fins.astype(np.float32), dark["base"])
    # body and nose cone
    body = np.clip(1.0 - (dy - 4.0) / 1.0, 0, 1) * (xx > tail) * (xx < nose - 10)
    E._over(img, body, steel["base"])
    E._over(img, (np.clip(1.0 - (dy - 1.2) / 1.0, 0, 1) * (yy < cy) * (xx > tail) * (xx < nose - 10)), steel["light"],
            0.8)
    cone = np.clip(1.0 - (dy - 4.0 * np.clip((nose - xx) / 10.0, 0, 1)) / 1.0, 0, 1) * (xx >= nose - 10) * (xx < nose)
    E._over(img, cone, red["base"])
    # a dark rim so it reads on any ground
    rim = np.clip(img[..., 3] * 3, 0, 1)
    halo = np.maximum.reduce([np.roll(np.roll(rim, a, 0), b, 1) for a in (-1, 0, 1) for b in (-1, 0, 1)])
    out = np.zeros_like(img)
    out[..., :3] = (0.08, 0.07, 0.06)
    out[..., 3] = halo * 0.75 * (1 - flame)
    a = img[..., 3:4]
    oa = a + out[..., 3:4] * (1 - a)
    out[..., :3] = (img[..., :3] * a + out[..., :3] * out[..., 3:4] * (1 - a)) / np.maximum(oa, 1e-6)
    out[..., 3:4] = oa
    return E._save("rocket_projectile", out, (int(nose), int(cy)), {"effect": "projectile", "projectile": "rocket",
                                                                   "points": "east"})


def _foam_projectile():
    """64x32, pointing east, origin at the glob's front: a white foam glob
    with a teal-shaded underside and a spray of droplets trailing west."""
    foam, paint = E._mat("foam"), E._mat("foam_paint")
    w, h = 64, 32
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    cy, front = h / 2, 60.0
    rng = np.random.RandomState(77)
    img = np.zeros((h, w, 4), np.float32)
    glob = np.hypot((xx - (front - 9)) / 10.0, (yy - cy) / 8.0)
    E._over(img, np.clip((1.0 - glob) * 6, 0, 1), foam["base"])
    E._over(img, np.clip((1.0 - glob) * 6, 0, 1) * (yy > cy + 2), paint["light"], 0.45)
    E._over(img, np.clip((0.45 - np.hypot((xx - (front - 12)) / 4.0, (yy - (cy - 3)) / 3.0)) * 4, 0, 1),
            foam["light"], 1.0)
    for k in range(9):                                               # trailing droplets, smaller further back
        t = k / 8.0
        px = front - 22 - t * 34 + rng.uniform(-2, 2)
        py = cy + rng.uniform(-6, 6) * (0.4 + t)
        r = 3.6 * (1 - 0.7 * t)
        E._over(img, np.clip((r - np.hypot(xx - px, yy - py)) / 0.9, 0, 1), foam["base"], 0.9 - 0.4 * t)
    rim = np.clip(img[..., 3] * 3, 0, 1)
    halo = np.maximum.reduce([np.roll(np.roll(rim, a, 0), b, 1) for a in (-1, 0, 1) for b in (-1, 0, 1)])
    out = np.zeros_like(img)
    out[..., :3] = (0.08, 0.07, 0.06)
    out[..., 3] = halo * 0.6
    a = img[..., 3:4]
    oa = a + out[..., 3:4] * (1 - a)
    out[..., :3] = (img[..., :3] * a + out[..., :3] * out[..., 3:4] * (1 - a)) / np.maximum(oa, 1e-6)
    out[..., 3:4] = oa
    return E._save("foam_projectile", out, (int(front), int(cy)), {"effect": "projectile", "projectile": "foam",
                                                                  "points": "east"})


# ------------------------------------------------------------------- icons
ICONS = {
    "foam_popper": ("defense", "foam_popper"),
    "ied_foam": ("defense", "ied_foam"),
    "ied_warhead": ("defense", "ied_warhead"),
    "foam_shells": ("item", "foam_shells"),
    "antimatter_warhead": ("item", "antimatter_warhead"),
}


def icons():
    out = [O._icon(k, fam, spr) for k, (fam, spr) in ICONS.items()]
    for prefix in TURRETS:
        out.append(O._cut_icon(prefix, F._composite(S.out_path(FAMILY, prefix + "_base"),
                                                    S.out_path(FAMILY, "%s_gun_14" % prefix)),
                               "%s:defense/%s_base+%s_gun_14" % (C.NAMESPACE, prefix, prefix)))
    return [p for p in out if p]


# ------------------------------------------------------------------- build
def build(only=None):
    """Every A5 sprite and icon; `only` (popper, charges, turret prefixes,
    items, stain, projectiles) renders just those, without icons."""
    def want(n):
        return not only or n in only
    out = []
    if want("popper"):
        out += P.render("foam_popper", _foam_popper, family=FAMILY, extra={"building": "foam popper"})
    if want("charges"):
        out += P.render("ied_foam", F._charge(*FOAM_CHARGE), family=FAMILY,
                        extra={"building": "foam charge", "charge": "foam"})
        out += P.render("ied_warhead", _ied_warhead, family=FAMILY,
                        extra={"building": "warhead charge", "charge": "warhead"})
    for prefix, (base, gun, building) in TURRETS.items():
        if not want(prefix):
            continue
        out += P.render(prefix + "_base", base, family=FAMILY,
                        extra={"building": building, "gun_layer": "%s/%s_gun" % (FAMILY, prefix),
                               "frames": GUN_FRAMES})
        for i in range(GUN_FRAMES):
            out += P.render("%s_gun_%02d" % (prefix, i), gun(i), family=FAMILY,
                            extra={"layer": "gun", "of": "%s/%s_base" % (FAMILY, prefix), "frame": i,
                                   "frames": GUN_FRAMES, "step_deg": GUN_STEP})
    if want("items"):
        out += [_foam_shells(), _antimatter_warhead()]
    if want("stain"):
        out += [_foam_stain(k) for k in range(3)]
    if want("projectiles"):
        out += [_rocket_projectile(), _foam_projectile()]
    if not only:
        out += icons()
    return out
