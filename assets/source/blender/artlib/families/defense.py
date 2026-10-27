"""Defense family (PLAY-024; ART spec §12, §14, §38): the sentry turret.
Our own design: nothing traced or reconstructed.

Sprites go to the `defense` family (@2x).

- sentry_turret_base (1x1, not rotatable): a bolted footing plate, a squat
  eight-sided armoured pedestal with olive plates, an ammo box on its west
  flank and a slewing ring on top. A red status lamp on the south-east of
  the pedestal is its glow layer, sentry_turret_base_glow, drawn only while
  powered. Attachment `mount`: where the gun's pivot sits (the top of the
  slewing ring).
- sentry_turret_gun_00..15: the gun head alone (yoke, armoured receiver,
  drum magazine, sight and a shrouded barrel with a muzzle brake), baked in
  16 aim directions like the wind turbine's rotor frames. Frame 0 points
  south (toward the viewer) and frames count clockwise on screen, 22.5
  degrees each: 4 west, 8 north, 12 east. Every frame shares the base's
  canvas, origin and zmax, so `mount` is the same pixel in all of them and
  in the base: draw the frame at the base's origin, after the base.
  Attachments `mount` (the pivot) and `muzzle` (where the flash and the
  tracer start).

Attachments are canvas pixels (@2x), like the power family's.

    make art FAMILIES=defense
"""
import math
import os
import tempfile

import numpy as np

from .. import stage as S
from . import power as P

FAMILY = "defense"
MOUNT = (0.5, -0.5, 0.46)      # the gun's pivot: centre of the slewing ring's top
ZMAX = 0.80                    # base and gun share one canvas
GUN_FRAMES = 16
GUN_STEP = 22.5                # degrees per frame, clockwise on screen
MUZZLE_REACH = 0.47            # pivot to the muzzle brake's face, tiles
BORE_Z = 0.145                 # barrel axis above the pivot


def _base():
    """Footing plate, armoured pedestal, ammo box, slewing ring, lamp."""
    cx, cy = 0.5, -0.5
    S.box("footing", 0.14, -0.86, 0.0, 0.86, -0.14, 0.06, "steel_dark", bevel=0.02)
    for x, y in ((0.20, -0.20), (0.80, -0.20), (0.20, -0.80), (0.80, -0.80)):
        S.cyl("anchor", x, y, 0.028, 0.06, 0.085, "metal_dark", segs=6, bevel=0.0)
    n0 = len(S.objects())
    S.cyl("pedestal", cx, cy, 0.30, 0.06, 0.36, "steel", segs=8, r_top=0.26, bevel=0.02)
    S.cyl("plates", cx, cy, 0.305, 0.12, 0.28, "armour_olive", segs=8, r_top=0.28, bevel=0.0)
    P._spin(P._since(n0), (cx, cy, 0.0), "Z", 22.5)            # a flat face to the south
    S.cyl("collar", cx, cy, 0.25, 0.36, 0.40, "metal_dark", segs=24, bevel=0.01)
    S.cyl("slew_ring", cx, cy, 0.21, 0.40, MOUNT[2], "steel_dark", segs=24, bevel=0.01)
    P._bolts(cx, cy, 0.23, 0.40, 0.415, 8, rb=0.016)
    # ammo box on the west flank, a feed chute up to the ring
    S.box("ammo_box", 0.10, -0.78, 0.06, 0.30, -0.48, 0.30, "armour_olive", bevel=0.02)
    S.box("ammo_lid", 0.09, -0.79, 0.30, 0.31, -0.47, 0.33, "steel_dark", bevel=0.01)
    S.box("ammo_latch", 0.16, -0.795, 0.22, 0.24, -0.78, 0.27, "brass", bevel=0.004)
    S.box("feed", 0.26, -0.60, 0.26, 0.34, -0.52, 0.40, "metal_dark", bevel=0.01)
    # status lamp on the south-east face, a hood over it
    S.box("lamp_hood", 0.62, -0.80, 0.24, 0.76, -0.75, 0.33, "metal_dark", bevel=0.01)
    P.glow(P._rod("lamp", (0.69, -0.815, 0.275), (0.69, -0.78, 0.275), 0.035, "backdrop", segs=12, bevel=0.0),
           "sentry_lamp_glow")
    P._hazard(0.30, 0.56, -0.79, -0.775, 0.08, 0.13, n=3)
    return (1, 1), ZMAX, {"mount": MOUNT}


def _aim(frame):
    """Screen-clockwise from south: the gun's heading in Blender degrees
    (counter-clockwise from +X, +Y north)."""
    return -90.0 - GUN_STEP * frame


def _gun(frame):
    """The gun head, authored along +X about the pivot, then turned to its
    frame's heading."""
    def build():
        px, py, pz = MOUNT
        bz = pz + BORE_Z
        S.cyl("yoke", px, py, 0.17, pz, pz + 0.05, "metal_dark", segs=20, bevel=0.01)
        n0 = len(S.objects())
        # receiver: an armoured box with a darker roof and a sloped rear block
        S.box("receiver", px - 0.17, py - 0.12, pz + 0.05, px + 0.12, py + 0.12, pz + 0.23, "armour_olive",
              bevel=0.03)
        S.box("roof", px - 0.14, py - 0.10, pz + 0.23, px + 0.09, py + 0.10, pz + 0.255, "steel_dark",
              bevel=0.01)
        S.box("rear", px - 0.24, py - 0.09, pz + 0.07, px - 0.16, py + 0.09, pz + 0.19, "steel_dark",
              bevel=0.02)
        S.box("mantlet", px + 0.11, py - 0.08, pz + 0.075, px + 0.17, py + 0.08, pz + 0.215, "steel",
              bevel=0.015)
        # drum magazine on the left (north when aimed east)
        P._rod("drum", (px - 0.08, py + 0.12, pz + 0.13), (px - 0.08, py + 0.20, pz + 0.13), 0.075,
               "brass", segs=18, bevel=0.01)
        P._rod("drum_cap", (px - 0.08, py + 0.20, pz + 0.13), (px - 0.08, py + 0.215, pz + 0.13), 0.05,
               "metal_dark", segs=14, bevel=0.0)
        # sight box on the roof, its lens forward
        S.box("sight", px - 0.06, py - 0.10, pz + 0.255, px + 0.06, py - 0.03, pz + 0.31, "steel", bevel=0.01)
        P._rod("sight_lens", (px + 0.06, py - 0.065, pz + 0.283), (px + 0.075, py - 0.065, pz + 0.283), 0.022,
               "backdrop", segs=10, bevel=0.0)
        # barrel: perforated shroud, bare tube, muzzle brake
        P._rod("shroud", (px + 0.17, py, bz), (px + 0.32, py, bz), 0.045, "metal_dark", segs=14, bevel=0.006)
        for x in (0.21, 0.25, 0.29):
            P._rod("vent", (px + x, py, bz), (px + x + 0.015, py, bz), 0.048, "steel_dark", segs=14, bevel=0.0)
        P._rod("barrel", (px + 0.32, py, bz), (px + 0.42, py, bz), 0.026, "steel_dark", segs=12, bevel=0.0)
        P._rod("brake", (px + 0.41, py, bz), (px + MUZZLE_REACH, py, bz), 0.038, "metal_dark", segs=12,
               bevel=0.006)
        P._spin(P._since(n0), (px, py, 0.0), "Z", _aim(frame))
        a = math.radians(_aim(frame))
        muzzle = (px + MUZZLE_REACH * math.cos(a), py + MUZZLE_REACH * math.sin(a), bz)
        return (1, 1), ZMAX, {"mount": MOUNT, "muzzle": muzzle}
    return build


def build():
    out = P.render("sentry_turret_base", _base, family=FAMILY,
                   extra={"building": "sentry turret", "gun_layer": "defense/sentry_turret_gun",
                          "frames": GUN_FRAMES})
    for i in range(GUN_FRAMES):
        out += P.render("sentry_turret_gun_%02d" % i, _gun(i), family=FAMILY, shadow_kind="none",
                        extra={"layer": "gun", "of": "defense/sentry_turret_base", "frame": i,
                               "frames": GUN_FRAMES, "step_deg": GUN_STEP})
    return out


# ------------------------------------------------------------ previews
def turret_composite(frame=14):
    """The base with one gun frame on it (same canvas; a temp PNG for the
    Architect icon); None until both are rendered."""
    base, gun = S.out_path(FAMILY, "sentry_turret_base"), S.out_path(FAMILY, "sentry_turret_gun_%02d" % frame)
    if not (os.path.exists(base) and os.path.exists(gun)):
        return None
    out, g = S._read_png(base).copy(), S._read_png(gun)
    a = g[..., 3:4]
    oa = a + out[..., 3:4] * (1 - a)
    out[..., :3] = (g[..., :3] * a + out[..., :3] * out[..., 3:4] * (1 - a)) / np.maximum(oa, 1e-6)
    out[..., 3:4] = oa
    path = os.path.join(tempfile.gettempdir(), "rwg_sentry_turret_preview.png")
    S.write_png(out, path)
    return path

