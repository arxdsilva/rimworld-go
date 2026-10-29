"""Fortification family (wave D: DEF-3; ART spec §12-§14, §38): barricade,
the four charges, the autocannon, the slug turret, the mortar and the
powered door, plus their Architect icons. Our own designs: nothing traced or
reconstructed.

Sprites land in the `defense` output family (the powered door in
`structure`, beside the doors), @2x. Turrets follow the sentry turret
(families/defense.py): a base and a gun baked in 16 aim directions, frame 0
pointing south (toward the viewer), frames counting clockwise on screen at
22.5 degrees (4 west, 8 north, 12 east). Every frame shares the base's canvas,
origin and zmax, so `mount` is the same pixel in the base and in each frame:
draw the frame at the base's origin, after the base. Gun frames carry
`mount` and `muzzle` (where the flash and tracer start).

- barricade (1x1): a waist-high wall of staggered blocks tied with iron
  straps. The blocks are the `stone_tint` tint group (neutral body plus a
  _mask.png), so the renderer can multiply them by the stuff colour.
- ied_high_explosive / ied_incendiary / ied_emp / ied_smoke (1x1): two shells
  half buried in a dirt mound, wired to a pressure plate. The shells' band
  and body colours tell the charges apart (olive and yellow, red and orange,
  steel and blue, grey and cream).
- autocannon_base (2x2) + autocannon_gun_00..15: an armoured octagonal
  pedestal with an ammo crate; a heavy single-barrel gun with a belt box.
  Status lamp glow layer autocannon_base_glow (powered).
- slug_turret_base (2x2) + slug_turret_gun_00..15: a stepped round pedestal
  with coolant cylinders; a long twin-rail gun wound with copper coils.
  Status lamp glow layer slug_turret_base_glow (powered).
- mortar_base (1x1) + mortar_tube_00..15: a round baseplate with a shell
  crate; the tube, its bipod and sight, raised 55 degrees and baked in 16
  headings (unpowered, no glow). `muzzle` is the tube's mouth.
- autodoor_<ew|ns>_<closed|open> (structure, 1x1): a steel frame joining the
  walls like a door, two sliding leaves (closed: meeting at a seam; open:
  slid into the posts) and a status lamp on each post, glow layer
  autodoor_<axis>_<state>_glow (powered).
- item/mortar_shells, incendiary_shells, emp_shells, smoke_shells: a crate
  of three finned shells in the charges' colours (metadata `item:` the item
  kind's name).

    make art FAMILIES=fortify
"""
import math
import os
import tempfile

import numpy as np

from .. import stage as S
from . import overlay as O
from . import power as P
from .structure import EXT, WALL_H

FAMILY = "defense"
GUN_FRAMES = 16
GUN_STEP = 22.5


def _aim(frame):
    """Screen-clockwise from south: the heading in Blender degrees."""
    return -90.0 - GUN_STEP * frame


def _ring_lamp(x, y, z, lit="sentry_lamp_glow"):
    """A hooded status lamp facing south at (x, y, z)."""
    S.box("lamp_hood", x - 0.07, y - 0.03, z - 0.05, x + 0.07, y + 0.02, z + 0.05, "metal_dark", bevel=0.01)
    P.glow(P._rod("lamp", (x, y - 0.035, z), (x, y + 0.0, z), 0.035, "backdrop", segs=12, bevel=0.0), lit)
    P.glow(S.cyl("lamp_top", x, y - 0.01, 0.03, z + 0.05, z + 0.065, "backdrop", segs=10, bevel=0.0), lit)


# ---------------------------------------------------------------- barricade
def _barricade():
    """1x1: two courses of staggered blocks across the tile (to its east and
    west edges, so a row reads as one wall), two iron straps over them."""
    S.box("footing", 0.02, -0.74, 0.0, 0.98, -0.26, 0.04, "stone_neutral_dark", bevel=0.01)
    courses = (((0.02, 0.34), (0.36, 0.64), (0.66, 0.98)), ((0.02, 0.18), (0.20, 0.49), (0.51, 0.80),
                                                            (0.82, 0.98)))
    for row, blocks in enumerate(courses):
        z0 = 0.04 + 0.20 * row
        inset = 0.03 * row
        for k, (x0, x1) in enumerate(blocks):
            S.box("block%d_%d" % (row, k), x0, -0.72 + inset, z0, x1, -0.28 - inset, z0 + 0.19, "stone_tint",
                  bevel=0.03, segments=2)
    S.box("cap", 0.04, -0.66, 0.43, 0.96, -0.34, 0.47, "stone_tint", bevel=0.02)
    for k, x in enumerate((0.27, 0.73)):
        S.box("strap%d" % k, x - 0.03, -0.735, 0.04, x + 0.03, -0.265, 0.475, "metal_dark", bevel=0.004)
        S.cyl("rivet%d" % k, x, -0.745, 0.016, 0.30, 0.33, "steel", segs=6, bevel=0.0, ry=0.016)
    return (1, 1), 0.48


# ------------------------------------------------------------------ charges
CHARGES = {
    # name: (shell body, band, nose)
    "ied_high_explosive": ("armour_olive", "corn_yellow", "steel_dark"),
    "ied_incendiary": ("food_red", "pumpkin", "metal_dark"),
    "ied_emp": ("steel", "crystal_blue", "copper"),
    "ied_smoke": ("ash_grey", "wool_cream", "metal_dark"),
}


def _charge(body, band, nose):
    def build():
        """1x1: a low dirt mound, two shells lying half in it, a pressure
        plate on a short post and the wires between."""
        S.blob("mound", 0.5, -0.52, 0.0, 0.40, "soil", seed=17, lumps=0.12, squash=0.22)
        for k, y in enumerate((-0.40, -0.62)):
            x0, x1 = 0.22, 0.66
            P._rod("shell%d" % k, (x0, y, 0.07), (x1, y, 0.07), 0.08, body, segs=16, bevel=0.01)
            P._rod("band%d" % k, (x0 + 0.08, y, 0.07), (x0 + 0.13, y, 0.07), 0.083, band, segs=16, bevel=0.0)
            cone = S.cyl("nose%d" % k, 0, 0, 0.08, 0.0, 0.12, nose, segs=16, r_top=0.02, bevel=0.0)
            from mathutils import Matrix
            cone.matrix_world = Matrix.Translation((x1, y, 0.07)) @ Matrix.Rotation(math.radians(90), 4, "Y")
            S.cyl("fuse%d" % k, x1 + 0.12, y, 0.018, 0.06, 0.09, "brass", segs=6, bevel=0.0)
        S.box("plate", 0.70, -0.38, 0.06, 0.92, -0.16, 0.10, "metal_dark", bevel=0.01)
        S.box("plate_top", 0.72, -0.36, 0.10, 0.90, -0.18, 0.115, "steel_dark", bevel=0.005)
        for k, y in enumerate((-0.40, -0.62)):
            P._rod("wire%d" % k, (0.80, y, 0.11), (0.80, -0.30, 0.11), 0.012, "fur_black", segs=6, bevel=0.0)
        return (1, 1), 0.24
    return build


# ------------------------------------------------------------- shell items
# Mortar shell items (ground stacks), colour coded like the charges.
SHELLS = {
    # sprite: (item name, charge colours)
    "mortar_shells": ("mortar shells", CHARGES["ied_high_explosive"]),
    "incendiary_shells": ("incendiary shells", CHARGES["ied_incendiary"]),
    "emp_shells": ("EMP shells", CHARGES["ied_emp"]),
    "smoke_shells": ("smoke shells", CHARGES["ied_smoke"]),
}


def _shell_stack(body, band, nose):
    """A low open crate holding three finned shells lying east-west."""
    from mathutils import Matrix
    S.box("tray", 0.16, -0.80, 0.0, 0.84, -0.22, 0.05, "wood", bevel=0.015)
    for k, y in enumerate((-0.22, -0.77)):
        S.box("side%d" % k, 0.16, y - 0.03, 0.0, 0.84, y + 0.03, 0.12, "wood_plank", bevel=0.01)
    for k, y in enumerate((-0.36, -0.51, -0.66)):
        x0, x1, z = 0.22, 0.64, 0.12
        P._rod("body%d" % k, (x0, y, z), (x1, y, z), 0.068, body, segs=16, bevel=0.008)
        P._rod("band%d" % k, (x0 + 0.20, y, z), (x0 + 0.25, y, z), 0.071, band, segs=16, bevel=0.0)
        cone = S.cyl("nose%d" % k, 0, 0, 0.068, 0.0, 0.13, nose, segs=16, r_top=0.018, bevel=0.0)
        cone.matrix_world = Matrix.Translation((x1, y, z)) @ Matrix.Rotation(math.radians(90), 4, "Y")
        P._rod("tail%d" % k, (x0 - 0.10, y, z), (x0, y, z), 0.03, "metal_dark", segs=10, bevel=0.0)
        S.box("fin%d" % k, x0 - 0.10, y - 0.006, z - 0.065, x0 - 0.02, y + 0.006, z + 0.065, "metal_dark",
              bevel=0.0)
    return (1, 1), 0.22


# ------------------------------------------------------------------ turrets
def _pedestal(cx, cy, r, h, plate_mat, top):
    """A footing plate with anchors, an armoured octagonal pedestal (a flat
    face to the south), a collar and the slewing ring up to `top`."""
    half = r + 0.30
    S.box("footing", cx - half, cy - half, 0.0, cx + half, cy + half, 0.06, "steel_dark", bevel=0.02)
    for sx in (-1, 1):
        for sy in (-1, 1):
            S.cyl("anchor", cx + sx * (half - 0.08), cy + sy * (half - 0.08), 0.035, 0.06, 0.09, "metal_dark",
                  segs=6, bevel=0.0)
    n0 = len(S.objects())
    S.cyl("pedestal", cx, cy, r, 0.06, h, "steel", segs=8, r_top=r * 0.86, bevel=0.025)
    S.cyl("plates", cx, cy, r + 0.006, 0.14, h - 0.08, plate_mat, segs=8, r_top=r * 0.9, bevel=0.0)
    P._spin(P._since(n0), (cx, cy, 0.0), "Z", 22.5)
    S.cyl("collar", cx, cy, r * 0.80, h, h + 0.05, "metal_dark", segs=28, bevel=0.012)
    S.cyl("slew_ring", cx, cy, r * 0.66, h + 0.05, top, "steel_dark", segs=28, bevel=0.012)
    P._bolts(cx, cy, r * 0.73, h + 0.05, h + 0.07, 10, rb=0.02)


AC_MOUNT = (1.0, -1.0, 0.62)
AC_ZMAX = 1.10
AC_REACH = 0.95
AC_BORE = 0.20


def _autocannon_base():
    cx, cy = 1.0, -1.0
    _pedestal(cx, cy, 0.46, 0.52, "armour_olive", AC_MOUNT[2])
    # ammo crate on the west flank, a feed chute up to the ring
    S.box("ammo", 0.20, -1.56, 0.06, 0.58, -0.98, 0.42, "armour_olive", bevel=0.025)
    S.box("ammo_lid", 0.18, -1.58, 0.42, 0.60, -0.96, 0.46, "steel_dark", bevel=0.012)
    for k, y in enumerate((-1.40, -1.14)):
        S.box("latch%d" % k, 0.17, y - 0.04, 0.30, 0.19, y + 0.04, 0.36, "brass", bevel=0.004)
    S.box("chute", 0.52, -1.10, 0.36, 0.66, -0.96, 0.58, "metal_dark", bevel=0.012)
    _ring_lamp(1.30, -1.44, 0.34)
    P._hazard(0.70, 1.10, -1.43, -1.415, 0.12, 0.20, n=3)
    return (2, 2), AC_ZMAX, {"mount": AC_MOUNT}


def _autocannon_gun(frame):
    def build():
        px, py, pz = AC_MOUNT
        bz = pz + AC_BORE
        S.cyl("yoke", px, py, 0.28, pz, pz + 0.06, "metal_dark", segs=24, bevel=0.012)
        n0 = len(S.objects())
        S.box("cheek_l", px - 0.20, py + 0.16, pz + 0.06, px + 0.14, py + 0.22, pz + 0.30, "steel_dark", bevel=0.02)
        S.box("cheek_r", px - 0.20, py - 0.22, pz + 0.06, px + 0.14, py - 0.16, pz + 0.30, "steel_dark", bevel=0.02)
        S.box("receiver", px - 0.32, py - 0.16, pz + 0.08, px + 0.20, py + 0.16, pz + 0.34, "armour_olive",
              bevel=0.035)
        S.box("roof", px - 0.28, py - 0.13, pz + 0.34, px + 0.14, py + 0.13, pz + 0.37, "steel_dark", bevel=0.012)
        S.box("breech", px - 0.42, py - 0.11, pz + 0.12, px - 0.30, py + 0.11, pz + 0.30, "steel_dark", bevel=0.02)
        S.box("mantlet", px + 0.18, py - 0.12, pz + 0.12, px + 0.26, py + 0.12, pz + 0.30, "steel", bevel=0.02)
        # belt box on the left (north when aimed east), a belt into the receiver
        S.box("belt_box", px - 0.22, py + 0.22, pz + 0.10, px + 0.04, py + 0.38, pz + 0.30, "armour_olive",
              bevel=0.02)
        S.box("belt", px - 0.16, py + 0.15, pz + 0.26, px - 0.02, py + 0.23, pz + 0.30, "brass", bevel=0.004)
        # sight on the roof, its lens forward
        S.box("sight", px - 0.08, py - 0.14, pz + 0.37, px + 0.08, py - 0.05, pz + 0.44, "steel", bevel=0.01)
        P._rod("sight_lens", (px + 0.08, py - 0.095, pz + 0.405), (px + 0.095, py - 0.095, pz + 0.405), 0.026,
               "backdrop", segs=10, bevel=0.0)
        # barrel: recoil sleeve, tube, muzzle brake
        P._rod("sleeve", (px + 0.26, py, bz), (px + 0.52, py, bz), 0.075, "metal_dark", segs=16, bevel=0.008)
        for x in (0.32, 0.40, 0.48):
            P._rod("rib", (px + x, py, bz), (px + x + 0.02, py, bz), 0.08, "steel_dark", segs=16, bevel=0.0)
        P._rod("barrel", (px + 0.52, py, bz), (px + 0.86, py, bz), 0.042, "steel_dark", segs=14, bevel=0.0)
        P._rod("brake", (px + 0.85, py, bz), (px + AC_REACH, py, bz), 0.065, "metal_dark", segs=8, bevel=0.008)
        P._spin(P._since(n0), (px, py, 0.0), "Z", _aim(frame))
        a = math.radians(_aim(frame))
        return (2, 2), AC_ZMAX, {"mount": AC_MOUNT,
                                 "muzzle": (px + AC_REACH * math.cos(a), py + AC_REACH * math.sin(a), bz)}
    return build


SL_MOUNT = (1.0, -1.0, 0.58)
SL_ZMAX = 1.00
SL_REACH = 0.93
SL_BORE = 0.16


def _slug_base():
    cx, cy = 1.0, -1.0
    S.box("footing", 0.16, -1.84, 0.0, 1.84, -0.16, 0.06, "steel_dark", bevel=0.02)
    for sx in (-1, 1):
        for sy in (-1, 1):
            S.cyl("anchor", cx + sx * 0.76, cy + sy * 0.76, 0.035, 0.06, 0.09, "metal_dark", segs=6, bevel=0.0)
    S.cyl("tier0", cx, cy, 0.56, 0.06, 0.20, "steel", segs=32, bevel=0.025)
    S.cyl("tier1", cx, cy, 0.46, 0.20, 0.40, "metal_dark", segs=32, r_top=0.40, bevel=0.02)
    S.cyl("tier2", cx, cy, 0.38, 0.40, 0.50, "steel", segs=32, bevel=0.015)
    S.cyl("slew_ring", cx, cy, 0.30, 0.50, SL_MOUNT[2], "steel_dark", segs=28, bevel=0.012)
    P._bolts(cx, cy, 0.50, 0.20, 0.22, 12, rb=0.018)
    # coolant cylinders on the east and west flanks, piped into the pedestal
    for k, x in enumerate((0.30, 1.70)):
        S.cyl("coolant%d" % k, x, -1.30, 0.12, 0.06, 0.52, "crystal_blue", segs=16, bevel=0.02)
        S.cyl("coolant_cap%d" % k, x, -1.30, 0.13, 0.52, 0.56, "steel_dark", segs=16, bevel=0.008)
        P._rod("pipe%d" % k, (x, -1.30, 0.44), (cx + (0.36 if x > cx else -0.36), -1.20, 0.30), 0.025, "copper",
               segs=8, bevel=0.0)
    _ring_lamp(1.0, -1.57, 0.14)
    return (2, 2), SL_ZMAX, {"mount": SL_MOUNT}


def _slug_gun(frame):
    def build():
        px, py, pz = SL_MOUNT
        bz = pz + SL_BORE
        S.cyl("yoke", px, py, 0.24, pz, pz + 0.05, "metal_dark", segs=24, bevel=0.012)
        n0 = len(S.objects())
        # a long low receiver, tapered to the rear
        S.box("receiver", px - 0.30, py - 0.13, pz + 0.05, px + 0.22, py + 0.13, pz + 0.27, "steel", bevel=0.04)
        S.box("spine", px - 0.26, py - 0.06, pz + 0.27, px + 0.18, py + 0.06, pz + 0.31, "steel_dark", bevel=0.012)
        S.box("tail", px - 0.40, py - 0.09, pz + 0.08, px - 0.28, py + 0.09, pz + 0.22, "metal_dark", bevel=0.02)
        S.box("cell", px - 0.20, py + 0.13, pz + 0.08, px + 0.06, py + 0.22, pz + 0.24, "crystal_blue", bevel=0.02)
        # twin rails with copper coils wound round both, a muzzle collar
        for k, dy in enumerate((-0.055, 0.055)):
            P._rod("rail%d" % k, (px + 0.22, py + dy, bz), (px + SL_REACH - 0.02, py + dy, bz), 0.028,
                   "steel_dark", segs=10, bevel=0.0)
        for k in range(5):
            x = px + 0.30 + 0.10 * k
            S.box("coil%d" % k, x, py - 0.10, bz - 0.05, x + 0.05, py + 0.10, bz + 0.05, "copper", bevel=0.02)
        S.box("collar", px + SL_REACH - 0.06, py - 0.10, bz - 0.055, px + SL_REACH, py + 0.10, bz + 0.055,
              "metal_dark", bevel=0.015)
        P._spin(P._since(n0), (px, py, 0.0), "Z", _aim(frame))
        a = math.radians(_aim(frame))
        return (2, 2), SL_ZMAX, {"mount": SL_MOUNT,
                                 "muzzle": (px + SL_REACH * math.cos(a), py + SL_REACH * math.sin(a), bz)}
    return build


MO_MOUNT = (0.5, -0.5, 0.08)    # the tube's ball joint on the baseplate
MO_ZMAX = 0.82
MO_LEN = 0.74
MO_ELEV = 55.0                  # degrees above the ground


def _mortar_base():
    """1x1: a round ribbed baseplate with spades, a socket for the tube and a
    crate of shells at the north-east corner."""
    cx, cy = MO_MOUNT[0], MO_MOUNT[1]
    S.cyl("plate", cx, cy, 0.30, 0.0, 0.05, "metal_dark", segs=24, bevel=0.012)
    for k in range(6):
        a = math.radians(k * 60 + 30)
        P._rod("rib%d" % k, (cx + 0.08 * math.cos(a), cy + 0.08 * math.sin(a), 0.055),
               (cx + 0.28 * math.cos(a), cy + 0.28 * math.sin(a), 0.055), 0.016, "steel_dark", segs=6, bevel=0.0)
    S.cyl("socket", cx, cy, 0.07, 0.05, MO_MOUNT[2], "steel_dark", segs=14, bevel=0.01)
    S.box("crate", 0.72, -0.36, 0.0, 0.96, -0.06, 0.16, "wood", bevel=0.015)
    for k, x in enumerate((0.78, 0.90)):
        S.cyl("shell_top%d" % k, x, -0.21, 0.045, 0.16, 0.20, "armour_olive", segs=10, r_top=0.02, bevel=0.0)
    return (1, 1), MO_ZMAX, {"mount": MO_MOUNT}


def _mortar_tube(frame):
    def build():
        px, py, pz = MO_MOUNT
        e = math.radians(MO_ELEV)
        top = (px + MO_LEN * math.cos(e), py, pz + MO_LEN * math.sin(e))
        n0 = len(S.objects())
        S.ellipsoid("ball", px, py, pz, 0.07, 0.07, 0.07, "steel_dark", segs=12, rings=8)
        P._rod("tube", (px, py, pz), top, 0.075, "armour_olive", segs=16, bevel=0.006)
        P._rod("muzzle_ring", (px + (MO_LEN - 0.04) * math.cos(e), py, pz + (MO_LEN - 0.04) * math.sin(e)), top,
               0.068, "metal_dark", segs=16, bevel=0.0)
        t = 0.55
        col = (px + MO_LEN * t * math.cos(e), py, pz + MO_LEN * t * math.sin(e))
        P._rod("collar", (col[0] - 0.03 * math.cos(e), py, col[2] - 0.03 * math.sin(e)),
               (col[0] + 0.03 * math.cos(e), py, col[2] + 0.03 * math.sin(e)), 0.09, "metal_dark", segs=14,
               bevel=0.0)
        for k, dy in enumerate((-0.20, 0.20)):
            foot = (col[0] + 0.10, py + dy, 0.0)
            P._rod("leg%d" % k, col, foot, 0.018, "steel_dark", segs=6, bevel=0.0)
            S.box("shoe%d" % k, foot[0] - 0.03, foot[1] - 0.03, 0.0, foot[0] + 0.03, foot[1] + 0.03, 0.02,
                  "metal_dark", bevel=0.004)
        P._rod("screw", (col[0], py, col[2]), (col[0] + 0.07, py, 0.10), 0.014, "steel", segs=6, bevel=0.0)
        S.box("sight", col[0] - 0.04, py + 0.07, col[2] + 0.02, col[0] + 0.04, py + 0.12, col[2] + 0.08, "steel",
              bevel=0.006)
        P._spin(P._since(n0), (px, py, 0.0), "Z", _aim(frame))
        a = math.radians(_aim(frame))
        r = MO_LEN * math.cos(e)
        return (1, 1), MO_ZMAX, {"mount": MO_MOUNT, "muzzle": (px + r * math.cos(a), py + r * math.sin(a), top[2])}
    return build


TURRETS = {
    # prefix: (base, gun, gun layer word, building name)
    "autocannon": (_autocannon_base, _autocannon_gun, "gun", "autocannon"),
    "slug_turret": (_slug_base, _slug_gun, "gun", "slug turret"),
    "mortar": (_mortar_base, _mortar_tube, "tube", "mortar"),
}


# ------------------------------------------------------------- powered door
def _autodoor(axis, open_):
    """axis 'ew': spans east-west (walls left/right), as structure._door."""
    def build():
        def b(name, x0, y0, z0, x1, y1, z1, m, bevel=0.02):
            if axis == "ew":
                return S.box(name, x0, y0, z0, x1, y1, z1, m, bevel=bevel)
            return S.box(name, y0 + 1, -x1, z0, y1 + 1, -x0, z1, m, bevel=bevel)
        b("post_w", -EXT, -0.96, 0.0, 0.14, -0.04, WALL_H, "metal_dark")
        b("post_e", 0.86, -0.96, 0.0, 1 + EXT, -0.04, WALL_H, "metal_dark")
        b("track", 0.10, -0.60, 0.0, 0.90, -0.40, 0.025, "steel_dark", bevel=0.004)
        for k, x in enumerate((0.03, 0.89)):   # a status lamp on top of each post
            P.glow(b("lamp%d" % k, x, -0.56, WALL_H, x + 0.08, -0.44, WALL_H + 0.015, "backdrop", bevel=0.004),
                   "indicator")
        if open_:
            b("leaf_w", 0.12, -0.56, 0.025, 0.20, -0.44, WALL_H - 0.06, "steel")
            b("leaf_e", 0.80, -0.56, 0.025, 0.88, -0.44, WALL_H - 0.06, "steel")
        else:
            b("leaf_w", 0.14, -0.56, 0.025, 0.495, -0.44, WALL_H - 0.06, "steel")
            b("leaf_e", 0.505, -0.56, 0.025, 0.86, -0.44, WALL_H - 0.06, "steel")
            for k, (x0, x1) in enumerate(((0.20, 0.44), (0.56, 0.80))):
                b("window%d" % k, x0, -0.575, 0.52, x1, -0.425, 0.70, "glass", bevel=0.006)
            b("stripe_w", 0.14, -0.575, 0.12, 0.495, -0.425, 0.17, "corn_yellow", bevel=0.0)
            b("stripe_e", 0.505, -0.575, 0.12, 0.86, -0.425, 0.17, "corn_yellow", bevel=0.0)
        return (1, 1), WALL_H
    return build


# ----------------------------------------------------------------- icons
def _composite(base, gun):
    """Base with one gun frame over it (same canvas): a temp PNG for the
    icon, or None until both are rendered."""
    if not (os.path.exists(base) and os.path.exists(gun)):
        return None
    out, g = S._read_png(base).copy(), S._read_png(gun)
    a = g[..., 3:4]
    oa = a + out[..., 3:4] * (1 - a)
    out[..., :3] = (g[..., :3] * a + out[..., :3] * out[..., 3:4] * (1 - a)) / np.maximum(oa, 1e-6)
    out[..., 3:4] = oa
    path = os.path.join(tempfile.gettempdir(), "rwg_%s_preview.png" % os.path.basename(base)[:-4])
    S.write_png(out, path)
    return path


ICONS = {
    "barricade": ("defense", "barricade"),
    "ied_high_explosive": ("defense", "ied_high_explosive"),
    "ied_incendiary": ("defense", "ied_incendiary"),
    "ied_emp": ("defense", "ied_emp"),
    "ied_smoke": ("defense", "ied_smoke"),
    "autodoor": ("structure", "autodoor_ew_closed"),
    "mortar_shells": ("item", "mortar_shells"),
    "incendiary_shells": ("item", "incendiary_shells"),
    "emp_shells": ("item", "emp_shells"),
    "smoke_shells": ("item", "smoke_shells"),
}


def _icons():
    out = [O._icon(icon, fam, spr) for icon, (fam, spr) in ICONS.items()]
    for prefix, (_, _, layer, _) in TURRETS.items():
        frame = 14 if layer == "gun" else 13
        out.append(O._cut_icon(prefix, _composite(S.out_path(FAMILY, prefix + "_base"),
                                                  S.out_path(FAMILY, "%s_%s_%02d" % (prefix, layer, frame))),
                               "core:defense/%s_base+%s_%s_%02d" % (prefix, prefix, layer, frame)))
    return [p for p in out if p]


# ------------------------------------------------------------------ build
def build(only=None):
    """Every fortification sprite and icon; `only` (barricade, charges,
    turret prefixes, autodoor) renders just those."""
    def want(n):
        return not only or n in only
    out = []
    if want("barricade"):
        out += P.render("barricade", _barricade, family=FAMILY,
                        extra={"building": "barricade", "stuff_tint": "stone_tint"})
    for name, mats in CHARGES.items():
        if want(name):
            out += P.render(name, _charge(*mats), family=FAMILY, extra={"building": "charge", "charge": name[4:]})
    for prefix, (base, gun, layer, building) in TURRETS.items():
        if not want(prefix):
            continue
        out += P.render(prefix + "_base", base, family=FAMILY,
                        extra={"building": building, "gun_layer": "%s/%s_%s" % (FAMILY, prefix, layer),
                               "frames": GUN_FRAMES})
        for i in range(GUN_FRAMES):
            out += P.render("%s_%s_%02d" % (prefix, layer, i), gun(i), family=FAMILY, shadow_kind="none",
                            extra={"layer": layer, "of": "%s/%s_base" % (FAMILY, prefix), "frame": i,
                                   "frames": GUN_FRAMES, "step_deg": GUN_STEP})
    for name, (item, mats) in SHELLS.items():
        if want(name):
            _shell_stack(*mats)
            out.append(S.render_asset("item", name, zmax=0.22, extra={"item": item, "stack": True}))
    if want("autodoor"):
        for axis in ("ew", "ns"):
            for open_ in (False, True):
                state = "open" if open_ else "closed"
                out += P.render("autodoor_%s_%s" % (axis, state), _autodoor(axis, open_), family="structure",
                                shadow_kind=None, clip=("W", "E") if axis == "ew" else ("N", "S"),
                                extra={"building": "powered door", "axis": axis, "state": state})
    if not only:
        out += _icons()
    return out
