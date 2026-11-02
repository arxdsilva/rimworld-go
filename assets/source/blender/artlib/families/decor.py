"""Decor family (AX, A2 rows; ART spec §12-§13): the vanilla structure,
furniture and decor pieces the Architect gained, plus their Architect
icons and the extra wall materials. Our own designs: nothing traced or
reconstructed.

Sprites land in the structure, furniture and overlay output families and
follow furniture.py: geometry authored facing SOUTH (the side a user
approaches from, a sleeper's feet), turned with stage.face(); footprints
come from the construction Def rows (wiki sizes). Wood parts are drawn in
`wood`/`wood_plank` and take the renderer's stuff colour shift like the
table and the shelf; the one tint group per sprite is `cloth` (upholstery,
the flap) or `stone_tint` (carved stone, as for the sculptures).

- structure/ornate_door_<ew|ns>_<open|closed> (2x1 / 1x2): a wide double
  door between two wall posts with gold finials, panelled leaves with
  gold strips, round gold bosses and gold copings. Axis and state follow
  the doors (`ew`: the door spans east-west, walls at both ends; open draws
  the leaves folded back against the posts). Clipped to join the walls.
- structure/animal_flap_<ew|ns>_<open|closed> (1x1): a low doorway in a
  short stretch of wall; the opening is hung with fabric strips (`cloth`
  tint group: the flap's stuff), rolled up under the lintel when open.
- furniture/column (1x1): a carved round shaft on a two-step base under a
  square capital; the shaft and capital are `stone_tint` (masked), the base
  and abacus a dark neutral.
- overlay/double_sleeping_spot_<facing> (2x2 flat marking): two dashed
  chalk outlines side by side, pillow marks at the head end (opposite the
  facing), like overlay/sleeping_spot.
- furniture/animal_sleeping_box (1x1): a low plank box with a lowered
  front and a straw bed.
- furniture/table_1x2_<south|east>, table_2x2, table_2x4_<south|east>,
  table_3x3: the table family at the wiki sizes (south: long side
  north-south; east: east-west; north and west draw the same as south and
  east). Planked top over an apron, legs at the corners and under long
  sides. Wood, coloured by stuff in the renderer like table_small.
- furniture/couch_<facing> (2x1): a two-seat upholstered couch (`cloth`
  tint group); attachments `seat`, `seat2` in tiles like the chair.
- furniture/plant_pot (1x1): an empty clay pot of soil; the potted plant is
  drawn on top at the `plant` attachment (canvas px, the soil centre).
- furniture/small_shelf_<facing> (1x1): the shelf's one-cell sibling.
- furniture/wall_torch_<facing>, wall_lamp_<facing> (1x1, wall-mounted):
  hung on the wall BEHIND the tile (north of it when facing south) at its
  shared edge. The torch is an iron cup on a bracket; its flame is drawn
  live at the `fire` attachment (canvas px) like the torch lamp. The lamp
  is a shaded bulb on an arm; wall_lamp_glow_<facing> lights the bulb
  while powered.
- furniture/flood_light (1x1): a tilted lamp head on a yoke and post; its
  lens is flood_light_glow (emissive flood_white) while powered.
- furniture/urn (1x1): a two-handled urn (`stone_tint` masked).
- furniture/large_stele (2x2), grand_stele (3x3): carved standing stones on
  stepped plinths (`stone_tint` masked; stone only).
- furniture/egg_box (1x1): a low crate lined with straw; stored eggs draw
  on top.
- structure/wall_<plasteel|uranium|jade|silver|gold>_<NN>[_c<h>]: the wall
  autotile (47 linked sprites per material, structure.py's wall recipe):
  plasteel and uranium panels, silver plates, jade and gold blocks.

Icons ui_icon/<name> are cut from the south sprites; the couch and flap
icons carry a cloth preset (the HUD draws icons untinted).

    make art FAMILIES=decor
"""
import math
import os
import tempfile

import numpy as np

from .. import contracts as C
from .. import meta
from .. import stage as S
from . import overlay as O
from . import power as P
from . import structure as ST
from .linked import _variants

FACINGS = ("south", "east", "north", "west")
EXT = ST.EXT
WALL_H = ST.WALL_H


# ----------------------------------------------------------------- doors
def _ornate_door(open_):
    """2x1, spanning east-west (walls at x=0 and x=2), authored like the
    door: posts join the walls, the leaves stand on the centre line."""
    def build():
        S.box("post_w", -EXT, -0.96, 0.0, 0.14, -0.04, WALL_H, "wood", bevel=0.02)
        S.box("post_e", 1.86, -0.96, 0.0, 2 + EXT, -0.04, WALL_H, "wood", bevel=0.02)
        for k, x in enumerate((0.14, 1.80)):          # gold strip down each post's faces
            S.box("pstrip%d" % k, x, -0.975, 0.10, x + 0.06, -0.025, WALL_H - 0.14, "gold_trim", bevel=0.004)
        for k, x in enumerate((0.07, 1.93)):          # gold finials on the posts
            S.ellipsoid("finial%d" % k, x, -0.5, WALL_H + 0.01, 0.07, 0.12, 0.05, "gold", segs=14, rings=8)
        S.box("sill", 0.12, -0.55, 0.0, 1.88, -0.45, 0.025, "gold_trim", bevel=0.004)
        top = WALL_H - 0.08
        leaves = (((0.14, 0.32), (1.68, 1.86)) if open_ else ((0.14, 1.0), (1.0, 1.86)))
        for k, (x0, x1) in enumerate(leaves):
            S.box("leaf%d" % k, x0 + 0.005, -0.57, 0.03, x1 - 0.005, -0.43, top, "wood_plank", bevel=0.02)
            S.box("cope%d" % k, x0 + 0.01, -0.59, top - 0.01, x1 - 0.01, -0.41, top + 0.03, "gold_trim", bevel=0.008)
            if open_:
                continue
            for face, y0, y1 in (("s", -0.585, -0.565), ("n", -0.435, -0.415)):
                for j, (z0, z1) in enumerate(((0.10, 0.36), (0.44, top - 0.08))):
                    S.box("panel%d%s%d" % (k, face, j), x0 + 0.10, y0, z0, x1 - 0.10, y1, z1, "wood", bevel=0.01)
                    S.box("strip%d%s%d" % (k, face, j), x0 + 0.08, y0 - 0.006, z1, x1 - 0.08, y1 + 0.006,
                          z1 + 0.025, "gold_trim", bevel=0.003)
                S.cyl("boss%d%s" % (k, face), (x0 + x1) / 2, (y0 + y1) / 2, 0.075, 0.38, 0.42, "gold",
                      segs=16, bevel=0.0)
        if not open_:
            for k, x in enumerate((0.93, 1.03)):
                S.box("handle%d" % k, x, -0.61, 0.40, x + 0.04, -0.39, 0.56, "gold", bevel=0.008)
        return (2, 1), WALL_H + 0.08
    return build


def _animal_flap(open_):
    """1x1, spanning east-west: posts join the walls; a header board over a
    low opening, hung on its south face with fabric strips (cloth tint
    group) that read below the header from above."""
    def build():
        S.box("post_w", -EXT, -0.96, 0.0, 0.12, -0.04, WALL_H, "wood", bevel=0.02)
        S.box("post_e", 0.88, -0.96, 0.0, 1 + EXT, -0.04, WALL_H, "wood", bevel=0.02)
        S.box("header", 0.10, -0.62, 0.52, 0.90, -0.38, WALL_H - 0.05, "wood_plank", bevel=0.02)
        S.box("cap", 0.08, -0.64, WALL_H - 0.07, 0.92, -0.36, WALL_H, "wood", bevel=0.02)
        S.box("beam", 0.10, -0.66, 0.48, 0.90, -0.60, 0.56, "bark", bevel=0.008)
        S.box("sill", 0.12, -0.62, 0.0, 0.88, -0.38, 0.03, "bark", bevel=0.005)
        if open_:
            P._rod("roll", (0.13, -0.70, 0.44), (0.87, -0.70, 0.44), 0.07, "cloth", segs=14, bevel=0.0)
        else:
            for k in range(4):
                x = 0.135 + k * 0.185
                S.box("strip%d" % k, x, -0.69, 0.03, x + 0.175, -0.64, 0.50, "cloth", bevel=0.012)
        return (1, 1), WALL_H
    return build


# ----------------------------------------------------------- furniture
def _column():
    """A round fluted shaft between a square two-step base and a round
    capital under a small square abacus: a disc on a square from above."""
    S.box("base0", 0.12, -0.88, 0.0, 0.88, -0.12, 0.08, "stone_neutral_dark", bevel=0.015)
    S.box("base1", 0.20, -0.80, 0.08, 0.80, -0.20, 0.14, "stone_neutral_dark", bevel=0.015)
    S.cyl("torus_lo", 0.5, -0.5, 0.23, 0.14, 0.20, "stone_tint", segs=24, bevel=0.02)
    S.cyl("shaft", 0.5, -0.5, 0.19, 0.20, 0.84, "stone_tint", segs=12, r_top=0.17, bevel=0.0)
    for k in range(12):                 # carved flutes: shallow dark grooves down the facets
        a = (k + 0.5) / 12 * 2 * math.pi
        x, y = 0.5 + 0.183 * math.cos(a), -0.5 + 0.183 * math.sin(a)
        if y < -0.45:                   # only the faces the camera sees
            S.box("flute%d" % k, x - 0.008, y - 0.008, 0.24, x + 0.008, y + 0.008, 0.80, "stone_neutral_dark",
                  bevel=0.0)
    S.cyl("capital", 0.5, -0.5, 0.20, 0.82, 0.92, "stone_tint", segs=24, r_top=0.27, bevel=0.02)
    S.box("abacus", 0.25, -0.75, 0.92, 0.75, -0.25, 1.0, "stone_neutral_dark", bevel=0.015)
    S.cyl("boss", 0.5, -0.5, 0.12, 0.995, 1.02, "stone_tint", segs=20, bevel=0.008)
    return (1, 1), 1.02


def _animal_sleeping_box():
    S.box("floor", 0.12, -0.88, 0.0, 0.88, -0.12, 0.05, "wood", bevel=0.012)
    S.box("wall_n", 0.12, -0.20, 0.05, 0.88, -0.12, 0.32, "wood_plank", bevel=0.012)
    for k, x in enumerate((0.12, 0.80)):
        S.box("wall_side%d" % k, x, -0.88, 0.05, x + 0.08, -0.12, 0.32, "wood_plank", bevel=0.012)
    S.box("wall_s", 0.12, -0.88, 0.05, 0.88, -0.80, 0.15, "wood_plank", bevel=0.012)   # lowered front
    for k, (x, y) in enumerate(((0.12, -0.88), (0.80, -0.88), (0.12, -0.20), (0.80, -0.20))):
        S.box("post%d" % k, x, y, 0.0, x + 0.08, y + 0.08, 0.34, "wood", bevel=0.01)
    S.box("bed", 0.20, -0.80, 0.05, 0.80, -0.20, 0.13, "straw", bevel=0.03, segments=2)
    for k, (x, y, s) in enumerate(((0.36, -0.40, 0.10), (0.62, -0.56, 0.09), (0.44, -0.66, 0.08))):
        S.ellipsoid("tuft%d" % k, x, y, 0.13, s, s * 0.8, 0.03, "carpet_straw_dark", segs=10, rings=6)
    return (1, 1), 0.34


def _table(w, h):
    """w x h table authored with its long side north-south."""
    def build():
        xs = [0.12, w - 0.12] + ([w / 2] if w >= 3 else [])
        ys = [-0.12, -(h - 0.12)] + ([-h / 2] if h >= 3 else [])
        k = 0
        for x in xs:
            for y in ys:
                if x not in (0.12, w - 0.12) and y not in (-0.12, -(h - 0.12)):
                    continue            # no leg in the middle of the floor
                S.box("leg%d" % k, x - 0.05, y - 0.05, 0.0, x + 0.05, y + 0.05, 0.50, "wood", bevel=0.015)
                k += 1
        S.box("apron", 0.10, -(h - 0.10), 0.40, w - 0.10, -0.10, 0.50, "wood", bevel=0.02)
        S.box("rim", 0.04, -(h - 0.04), 0.50, w - 0.04, -0.04, 0.58, "wood", bevel=0.03)
        S.box("top", 0.12, -(h - 0.12), 0.55, w - 0.12, -0.12, 0.586, "wood_plank", bevel=0.01)
        n = max(2, int(round((w - 0.24) / 0.30)))
        for i in range(1, n):
            x = 0.12 + (w - 0.24) * i / n
            S.box("seam%d" % i, x - 0.008, -(h - 0.13), 0.584, x + 0.008, -0.13, 0.589, "bark", bevel=0.0)
        return (w, h), 0.59
    return build


def _couch():
    for k, (x, y) in enumerate(((0.12, -0.18), (1.88, -0.18), (0.12, -0.86), (1.88, -0.86))):
        S.box("foot%d" % k, x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.08, "bark", bevel=0.008)
    S.box("frame", 0.06, -0.92, 0.08, 1.94, -0.10, 0.16, "wood", bevel=0.02)
    S.box("base", 0.06, -0.92, 0.16, 1.94, -0.12, 0.32, "cloth", bevel=0.04, segments=2)
    for k, (x0, x1) in enumerate(((0.22, 0.99), (1.01, 1.78))):
        S.box("cushion%d" % k, x0, -0.90, 0.30, x1, -0.36, 0.43, "cloth", bevel=0.05, segments=2)
        S.box("back_cushion%d" % k, x0, -0.44, 0.40, x1, -0.28, 0.78, "cloth", bevel=0.06, segments=2)
    S.box("back", 0.08, -0.34, 0.30, 1.92, -0.10, 0.86, "cloth", bevel=0.06, segments=2)
    for k, x in enumerate((0.14, 1.86)):
        S.box("arm%d" % k, x - 0.08, -0.90, 0.30, x + 0.08, -0.16, 0.58, "cloth", bevel=0.06, segments=2)
        P._rod("roll%d" % k, (x, -0.92, 0.58), (x, -0.16, 0.58), 0.085, "cloth", segs=16, bevel=0.0)
    return (2, 1), 0.86


COUCH_SEATS = {"seat": (0.6, -0.55, 0.0), "seat2": (1.4, -0.55, 0.0)}

POT_SOIL = (0.5, -0.5, 0.35)


def _plant_pot():
    S.cyl("foot", 0.5, -0.5, 0.21, 0.0, 0.04, "clay_dark", segs=24, bevel=0.01)
    S.cyl("pot", 0.5, -0.5, 0.24, 0.04, 0.31, "clay", segs=24, r_top=0.30, bevel=0.0)
    S.cyl("band", 0.5, -0.5, 0.285, 0.17, 0.20, "clay_dark", segs=24, r_top=0.29, bevel=0.0)
    S.cyl("rim", 0.5, -0.5, 0.335, 0.30, 0.36, "clay", segs=24, bevel=0.015)
    S.cyl("soil", 0.5, -0.5, 0.29, 0.33, POT_SOIL[2] + 0.003, "soil", segs=24, bevel=0.0)
    return (1, 1), 0.36


def _small_shelf():
    """1x1 shelf, the open front to the south (as storage.py's shelf)."""
    x0, x1, y0, y1 = 0.08, 0.92, -0.9, -0.12
    for k, x in enumerate((x0, x1 - 0.07)):
        for j, y in enumerate((y0, y1 - 0.07)):
            S.box("post%d%d" % (k, j), x, y, 0.0, x + 0.07, y + 0.07, 0.6, "wood", bevel=0.012)
        S.box("side%d" % k, x + 0.01, y0 + 0.07, 0.28, x + 0.06, y1 - 0.07, 0.33, "wood_plank", bevel=0.006)
    S.box("board_low", x0 + 0.02, y0 + 0.02, 0.1, x1 - 0.02, y1 - 0.02, 0.15, "bark", bevel=0.01)
    for k, xb in enumerate((x0 + 0.03, x1 - 0.1)):
        S.box("bearer%d" % k, xb, y0 + 0.02, 0.47, xb + 0.07, y1 - 0.02, 0.52, "wood", bevel=0.008)
    depth = (y1 - y0) - 0.04
    for k in range(3):
        py0 = y0 + 0.02 + depth * k / 3 + 0.025
        S.box("plank%d" % k, x0 + 0.02, py0, 0.52, x1 - 0.02, py0 + depth / 3 - 0.05, 0.57, "wood_plank", bevel=0.01)
    S.box("rail", x0, y1 - 0.06, 0.6, x1, y1, 0.66, "wood", bevel=0.01)
    S.box("brace", x0 + 0.07, y1 - 0.04, 0.2, x1 - 0.07, y1 - 0.01, 0.26, "wood_plank", bevel=0.006)
    S.box("kick", x0 + 0.04, y0 - 0.005, 0.02, x1 - 0.04, y0 + 0.03, 0.09, "bark", bevel=0.006)
    return (1, 1), 0.66


# Wall-mounted pieces hang on the wall face along the tile's north edge (the
# wall tile behind: its body stops ST.INSET short of the shared edge).
FACE_Y = ST.INSET
TORCH_FIRE = (0.5, -0.32, 0.80)


def _wall_torch():
    S.box("plate", 0.38, -0.03, 0.40, 0.62, FACE_Y, 0.84, "metal_dark", bevel=0.014)
    for k, z in enumerate((0.47, 0.77)):
        S.cyl("bolt%d" % k, 0.5, -0.035, 0.022, z - 0.014, z + 0.014, "steel_dark", segs=8, bevel=0.0)
    P._rod("arm", (0.5, -0.03, 0.60), (0.5, -0.28, 0.66), 0.03, "metal_dark", segs=10, bevel=0.0)
    P._rod("strut", (0.5, -0.03, 0.46), (0.5, -0.26, 0.64), 0.02, "metal_dark", segs=8, bevel=0.0)
    S.cyl("cup", 0.5, -0.32, 0.08, 0.62, 0.78, "metal_dark", segs=18, r_top=0.15, bevel=0.012)
    S.cyl("rim", 0.5, -0.32, 0.155, 0.76, 0.79, "steel_dark", segs=18, bevel=0.005)
    S.cyl("coals", 0.5, -0.32, 0.13, 0.77, 0.795, "ash_char", segs=18, bevel=0.0)
    for k, (dx, dy) in enumerate(((-0.05, 0.0), (0.03, 0.045), (0.045, -0.04))):
        S.box("stick%d" % k, 0.5 + dx - 0.07, -0.32 + dy - 0.018, 0.79, 0.5 + dx + 0.07, -0.32 + dy + 0.018, 0.83,
              "bark" if k % 2 else "wood", bevel=0.01)
    return (1, 1), 0.86, {"fire": TORCH_FIRE}


def _wall_lamp():
    """A glass globe in a cup on an arm (open on top, so the lit globe reads
    from above in every facing), a small vent cap over it."""
    S.box("plate", 0.36, -0.03, 0.44, 0.64, FACE_Y, 0.80, "steel", bevel=0.016)
    P._rod("arm", (0.5, -0.03, 0.58), (0.5, -0.28, 0.62), 0.028, "metal_dark", segs=10, bevel=0.0)
    P._rod("strut", (0.5, -0.03, 0.47), (0.5, -0.26, 0.60), 0.018, "metal_dark", segs=8, bevel=0.0)
    S.cyl("cup", 0.5, -0.32, 0.08, 0.56, 0.66, "metal_dark", segs=18, r_top=0.115, bevel=0.01)
    # plain glass in the sprite; wall_lamp_glow_<facing> lights it while powered
    P.glow(S.ellipsoid("globe", 0.5, -0.32, 0.77, 0.13, 0.13, 0.135, "glass", segs=20, rings=12), "lamp_warm")
    S.cyl("cap", 0.5, -0.32, 0.045, 0.90, 0.93, "steel", segs=12, bevel=0.006)
    return (1, 1), 0.94


def _flood_light():
    S.cyl("base", 0.5, -0.5, 0.19, 0.0, 0.06, "metal_dark", segs=20, bevel=0.012)
    for k in range(4):                  # bolts round the foot
        a = (k + 0.5) / 4 * 2 * math.pi
        S.cyl("bolt%d" % k, 0.5 + 0.14 * math.cos(a), -0.5 + 0.14 * math.sin(a), 0.02, 0.06, 0.08,
              "steel", segs=8, bevel=0.0)
    S.cyl("post", 0.5, -0.5, 0.05, 0.06, 0.70, "steel", segs=12, bevel=0.0)
    S.box("yoke", 0.14, -0.55, 0.64, 0.86, -0.45, 0.70, "metal_dark", bevel=0.01)
    for k, x in enumerate((0.14, 0.81)):
        S.box("yoke_arm%d" % k, x, -0.55, 0.64, x + 0.05, -0.45, 0.96, "metal_dark", bevel=0.008)
    n0 = len(S.objects())
    S.box("head", 0.20, -0.66, 0.68, 0.80, -0.34, 1.04, "steel_dark", bevel=0.03)
    for k in range(3):                  # cooling fins on the back
        S.box("fin%d" % k, 0.28 + k * 0.20, -0.36, 0.70, 0.32 + k * 0.20, -0.31, 1.02, "metal_dark", bevel=0.004)
    P.glow(S.box("lens", 0.24, -0.675, 0.72, 0.76, -0.65, 1.00, "glass", bevel=0.012), "flood_white")
    for k in range(2):
        z = 0.81 + k * 0.10
        S.box("guard%d" % k, 0.22, -0.685, z - 0.007, 0.78, -0.675, z + 0.007, "metal_dark", bevel=0.0)
    P._spin(P._since(n0), (0.5, -0.5, 0.85), "X", -35)   # the head tips up, its lens to the south
    return (1, 1), 1.16


def _urn():
    S.cyl("foot", 0.5, -0.5, 0.14, 0.0, 0.06, "stone_neutral_dark", segs=20, bevel=0.012)
    S.cyl("stem", 0.5, -0.5, 0.09, 0.05, 0.12, "stone_tint", segs=20, bevel=0.0)
    S.ellipsoid("body", 0.5, -0.5, 0.32, 0.22, 0.22, 0.22, "stone_tint", segs=24, rings=14)
    S.cyl("band", 0.5, -0.5, 0.222, 0.30, 0.33, "stone_neutral_dark", segs=24, bevel=0.0)
    S.cyl("neck", 0.5, -0.5, 0.10, 0.48, 0.62, "stone_tint", segs=20, r_top=0.085, bevel=0.0)
    S.cyl("lip", 0.5, -0.5, 0.14, 0.60, 0.66, "stone_tint", segs=20, bevel=0.015)
    for k, sx in enumerate((-1, 1)):
        pts = [(0.5 + sx * 0.19, -0.5, 0.42), (0.5 + sx * 0.22, -0.5, 0.54), (0.5 + sx * 0.18, -0.5, 0.60),
               (0.5 + sx * 0.10, -0.5, 0.58)]
        for j in range(3):
            P._rod("handle%d_%d" % (k, j), pts[j], pts[j + 1], 0.024, "stone_tint", segs=10, bevel=0.0)
    return (1, 1), 0.68


def _plinth(cx, cy, hw, hd, h, steps):
    for k in range(steps):
        g = 0.10 * (steps - 1 - k)
        S.box("plinth%d" % k, cx - hw - g, cy - hd - g, h * k / steps, cx + hw + g, cy + hd + g,
              h * (k + 1) / steps, "stone_neutral_dark", bevel=0.02)


def _glyphs(x0, x1, y, z0, z1, rows, tag):
    """Carved lines on a south face at y: short dark bars in rows."""
    for r in range(rows):
        z = z1 - (z1 - z0) * (r + 0.5) / rows
        n = 3 + (r % 2)
        for k in range(n):
            w = (x1 - x0) / n
            a = x0 + w * k + w * 0.18
            b = a + w * (0.45 if (r + k) % 3 else 0.64)
            S.box("%s%d_%d" % (tag, r, k), a, y - 0.006, z - 0.012, b, y + 0.004, z + 0.012, "stone_neutral_dark",
                  bevel=0.0)


def _large_stele():
    _plinth(1.0, -1.0, 0.62, 0.36, 0.16, 1)
    S.box("slab", 0.50, -1.17, 0.16, 1.50, -0.83, 1.66, "stone_tint", bevel=0.03)
    _arch("crown", 1.0, -1.0, 1.66, 0.50, 0.34)
    S.box("band", 0.48, -1.19, 1.32, 1.52, -0.81, 1.40, "stone_neutral_dark", bevel=0.008)
    _glyphs(0.60, 1.40, -1.17, 0.32, 1.24, 5, "glyph")
    return (2, 2), 1.96


def _grand_stele():
    _plinth(1.5, -1.5, 0.92, 0.52, 0.30, 2)
    S.box("slab_lo", 1.00, -1.74, 0.30, 2.00, -1.26, 1.50, "stone_tint", bevel=0.03)
    S.box("slab_hi", 1.06, -1.70, 1.50, 1.94, -1.30, 2.70, "stone_tint", bevel=0.03)
    S.box("band_lo", 0.98, -1.76, 1.44, 2.02, -1.24, 1.52, "stone_neutral_dark", bevel=0.01)
    S.box("band_hi", 1.04, -1.72, 2.66, 1.96, -1.28, 2.74, "stone_neutral_dark", bevel=0.01)
    n0 = len(S.objects())
    S.cyl("cap", 1.5, -1.5, 0.62, 2.74, 3.14, "stone_tint", segs=4, r_top=0.02, bevel=0.0)
    P._spin(P._since(n0), (1.5, -1.5, 2.9), "Z", 45)
    S.transform(P._since(n0), _scale_y_about(-1.5, 0.66))
    for k, x in enumerate((0.40, 2.16)):            # flanking stones
        S.box("flank%d" % k, x, -1.66, 0.30, x + 0.44, -1.34, 1.36, "stone_tint", bevel=0.03)
        _arch("flank_top%d" % k, x + 0.22, -1.50, 1.36, 0.22, 0.32)
    _glyphs(1.12, 1.88, -1.74, 0.44, 1.38, 5, "glyph_lo")
    _glyphs(1.16, 1.84, -1.70, 1.62, 2.58, 4, "glyph_hi")
    return (3, 3), 3.16


def _arch(name, cx, cy, cz, r, depth):
    """A round-topped slab end: a stone_tint disc of radius r standing in the
    x-z plane, centred (cx, cy, cz), depth thick along y (its lower half
    sinks into the slab it crowns)."""
    from mathutils import Matrix
    n0 = len(S.objects())
    S.cyl(name, cx, cz, r, -depth / 2, depth / 2, "stone_tint", segs=28, bevel=0.02)
    P._spin(P._since(n0), (0.0, 0.0, 0.0), "X", 90)
    S.transform(P._since(n0), Matrix.Translation((0.0, cy, 0.0)))


def _scale_y_about(cy, k):
    from mathutils import Matrix
    return Matrix.Translation((0, cy, 0)) @ Matrix.Diagonal((1.0, k, 1.0, 1.0)) @ Matrix.Translation((0, -cy, 0))


def _egg_box():
    S.box("floor", 0.14, -0.86, 0.0, 0.86, -0.14, 0.05, "wood", bevel=0.012)
    for k, (x0, y0, x1, y1) in enumerate(((0.14, -0.86, 0.86, -0.80), (0.14, -0.20, 0.86, -0.14),
                                          (0.14, -0.86, 0.20, -0.14), (0.80, -0.86, 0.86, -0.14))):
        S.box("side%d" % k, x0, y0, 0.03, x1, y1, 0.24, "wood_plank", bevel=0.01)
    for k, z in enumerate((0.10, 0.18)):    # board gaps on the visible front
        S.box("gap%d" % k, 0.16, -0.865, z, 0.84, -0.855, z + 0.012, "bark", bevel=0.0)
    for k, (x, y) in enumerate(((0.14, -0.86), (0.80, -0.86), (0.14, -0.20), (0.80, -0.20))):
        S.box("post%d" % k, x, y, 0.0, x + 0.06, y + 0.06, 0.26, "wood", bevel=0.008)
    S.box("straw", 0.20, -0.80, 0.05, 0.80, -0.20, 0.18, "straw", bevel=0.03, segments=2)
    for k, (x, y) in enumerate(((0.32, -0.34), (0.50, -0.50), (0.68, -0.38), (0.36, -0.66), (0.66, -0.66))):
        S.ellipsoid("nest%d" % k, x, y, 0.18, 0.09, 0.09, 0.02, "carpet_straw_dark", segs=12, rings=6)
    return (1, 1), 0.26


# --------------------------------------------------------- floor marking
def _double_sleeping_spot(facing):
    """2x2: two sleeping places side by side, chalk on the floor; the
    pillows at the head end, opposite the facing (as the sleeping spot)."""
    W = H = 2 * C.PPT
    yy, xx = np.mgrid[0:H, 0:W].astype(np.float32) + 0.5
    img = np.zeros((H, W, 4), np.float32)
    chalk, shade = (0.93, 0.89, 0.80), (0.10, 0.09, 0.08)
    inset = 12.0
    side = facing in ("east", "west")
    for k in range(2):
        if side:
            cx, cy, hw, hh = W / 2, H / 4 + k * H / 2, W / 2 - inset, H / 4 - inset
        else:
            cx, cy, hw, hh = W / 4 + k * W / 2, H / 2, W / 4 - inset, H / 2 - inset
        qx, qy = np.abs(xx - cx) - (hw - 16), np.abs(yy - cy) - (hh - 16)
        outside = np.hypot(np.maximum(qx, 0), np.maximum(qy, 0)) + np.minimum(np.maximum(qx, qy), 0) - 16
        ring = np.abs(outside)
        dash = (np.floor(np.where(qy > qx, xx, yy) / 12.0) % 2) == 0
        O._paint(img, O._cov(outside), shade, 0.16)
        O._paint(img, O._cov(ring - 2.6) * dash, shade, 0.45)
        O._paint(img, O._cov(ring - 1.8) * dash, chalk, 0.9)
        hx, hy = {"south": (cx, 34), "north": (cx, H - 34), "east": (34, cy), "west": (W - 34, cy)}[facing]
        pw, ph = (15, 26) if side else (26, 15)
        pill = O._rbox(xx, yy, hx, hy, pw, ph, 8)
        O._paint(img, O._cov(pill), chalk, 0.5)
        O._paint(img, O._cov(np.abs(pill) - 1.4), chalk, 0.9)
    name = "double_sleeping_spot_" + facing
    path = S.out_path("overlay", name)
    S.write_png(img, path)
    meta.write("overlay", name, {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [W, H], "footprint": [2, 2], "origin_px": [0, 0],
                                 "overlay": "double_sleeping_spot", "building": "double sleeping spot",
                                 "marking": True, "facing": facing, "rotatable": True})
    return path


# ------------------------------------------------------------------ walls
# (material, body, cap, trim, front detail) as structure.WALLS
WALLS = (("plasteel", "plasteel_wall", "plasteel_wall", "plasteel_trim", "steel"),
         ("uranium", "uranium_wall", "uranium_wall", "uranium_trim", "steel"),
         ("silver", "silver", "silver", "silver_trim", "steel"),
         ("jade", "jade", "jade", "jade_dark", "stone"),
         ("gold", "gold", "gold", "gold_trim", "stone"))


def _walls():
    out = []
    for mat, body, cap, trim, kind in WALLS:
        for mask in range(16):
            for filled, suffix in _variants(mask):
                con = ST._wall(mask, filled, body, cap, trim, kind)
                bits = int(suffix[2:], 16) if suffix else 0
                out.append(S.render_asset("structure", "wall_%s_%02d%s" % (mat, mask, suffix), zmax=WALL_H,
                                          clip=tuple(con), extra={"autotile": "wall", "mask": mask,
                                                                  "corners": bits, "material": mat}))
    return out


# ------------------------------------------------------------------ build
# name: (builder, extra, facings); builder returns (footprint, zmax[, pts])
TWO = ("south", "east")
PIECES = {
    "column": (_column, {"building": "column", "stuff_tint": "stone_tint"}, None),
    "animal_sleeping_box": (_animal_sleeping_box, {"building": "animal sleeping box",
                                                   "attachments": {"sleeper": [0.5, 0.5]}}, None),
    "table_1x2": (_table(1, 2), {"building": "table"}, TWO),
    "table_2x2": (_table(2, 2), {"building": "table 2x2"}, None),
    "table_2x4": (_table(2, 4), {"building": "table 2x4"}, TWO),
    "table_3x3": (_table(3, 3), {"building": "table 3x3"}, None),
    "couch": (_couch, {"building": "couch"}, FACINGS),
    "plant_pot": (lambda: _plant_pot() + ({"plant": POT_SOIL},), {"building": "plant pot"}, None),
    "small_shelf": (_small_shelf, {"building": "small shelf"}, FACINGS),
    "wall_torch": (_wall_torch, {"building": "wall torch", "emissive": "lamp_warm", "light_radius": 6,
                                 "fuel": "wood", "flame_scale": 0.4, "wall_mounted": True}, FACINGS),
    "wall_lamp": (_wall_lamp, {"building": "wall lamp", "light_radius": 8, "wall_mounted": True}, FACINGS),
    "flood_light": (_flood_light, {"building": "flood light", "light_radius": 17}, None),
    "urn": (_urn, {"building": "urn", "stuff_tint": "stone_tint"}, None),
    "large_stele": (_large_stele, {"building": "large stele", "stuff_tint": "stone_tint"}, None),
    "grand_stele": (_grand_stele, {"building": "grand stele", "stuff_tint": "stone_tint"}, None),
    "egg_box": (_egg_box, {"building": "egg box"}, None),
}

# Architect icons: ui_icon/<name> from (family, sprite[, cloth tint for the masked sprite]).
ICONS = {
    "ornate_door": ("structure", "ornate_door_ew_closed"),
    "animal_flap": ("structure", "animal_flap_ew_closed", "#cbb48c"),
    "column": ("furniture", "column"),
    "double_sleeping_spot": ("overlay", "double_sleeping_spot_south"),
    "animal_sleeping_box": ("furniture", "animal_sleeping_box"),
    "table_1x2": ("furniture", "table_1x2_east"),
    "table_2x2": ("furniture", "table_2x2"),
    "table_2x4": ("furniture", "table_2x4_east"),
    "table_3x3": ("furniture", "table_3x3"),
    "couch": ("furniture", "couch_south", "#a2593a"),
    "plant_pot": ("furniture", "plant_pot"),
    "small_shelf": ("furniture", "small_shelf_south"),
    "wall_torch": ("furniture", "wall_torch_south"),
    "wall_lamp": ("furniture", "wall_lamp_south"),
    "flood_light": ("furniture", "flood_light"),
    "urn": ("furniture", "urn"),
    "large_stele": ("furniture", "large_stele"),
    "grand_stele": ("furniture", "grand_stele"),
    "egg_box": ("furniture", "egg_box"),
}


def tinted_icon(name, family, sprite, hex_, masked=True):
    """ui_icon/<name> cut from family/sprite with hex_ multiplied in: through
    the sprite's tint mask (masked), else over the whole sprite (floors)."""
    from ..palette import hex_to_srgb
    src = S.out_path(family, sprite)
    if not os.path.exists(src):
        return None
    img = S._read_png(src)
    k = S._read_png(S.out_path(family, sprite + "_mask"))[..., :1] if masked else np.ones_like(img[..., :1])
    rgb = np.array(hex_to_srgb(hex_), np.float32)
    img[..., :3] = img[..., :3] * (1 - k) + img[..., :3] * rgb * k
    tmp = os.path.join(tempfile.gettempdir(), "rwg_icon_%s_%d.png" % (name, os.getpid()))
    S.write_png(img, tmp)
    return O._cut_icon(name, tmp, "%s:%s/%s" % (C.NAMESPACE, family, sprite))


def _icons():
    out = []
    for name, spec in ICONS.items():
        fam, spr = spec[0], spec[1]
        p = tinted_icon(name, fam, spr, spec[2]) if len(spec) > 2 else O._icon(name, fam, spr)
        if p:
            out.append(p)
    return out


def _doors():
    out = []
    for name, fn, fp0 in (("ornate_door", _ornate_door, (2, 1)), ("animal_flap", _animal_flap, (1, 1))):
        for axis in ("ew", "ns"):
            for open_ in (False, True):
                state = "open" if open_ else "closed"
                _, zmax = fn(open_)()
                fp = S.face(S.objects(), "south" if axis == "ew" else "east", fp0)
                out.append(S.render_asset("structure", "%s_%s_%s" % (name, axis, state), footprint=fp,
                                          zmax=zmax, clip=("W", "E") if axis == "ew" else ("N", "S"),
                                          extra={"building": name.replace("_", " "), "axis": axis,
                                                 "state": state}))
    return out


def _pieces(only=None):
    out = []
    for name, (fn, extra, facings) in PIECES.items():
        if only and name not in only:
            continue
        for f in facings or (None,):
            ex = dict(extra)
            if name == "couch":
                ex["attachments"] = {k: [round(x, 2), round(-y, 2)]
                                     for k, (x, y, _) in ((k, P.turn(p, f, (2, 1))) for k, p in COUCH_SEATS.items())}
            out += P.render(name + ("_" + f if f else ""), fn, facing=f, extra=ex, family="furniture")
    return out


def build(only=None):
    """Every decor sprite, wall material and icon; `only` (PIECES names)
    renders just those pieces."""
    if only:
        return _pieces(only)
    out = _doors() + _pieces()
    out += [_double_sleeping_spot(f) for f in FACINGS]
    out += _walls()
    out += _icons()
    return out
