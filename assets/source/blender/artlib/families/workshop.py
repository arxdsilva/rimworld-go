"""Workshop family (wave D: PRD-3, BED-3; ART spec §12-§14): the production
benches, the crafting and butchering spots, the fermenting barrel, the three
sculptures, the stool and the armchair, plus their Architect icons. Our own
designs: nothing traced or reconstructed.

Sprites land in the furniture output family (the two spots in overlay, beside
the other floor markings) and follow furniture.py: geometry authored facing
SOUTH (the worker's side), rotated with stage.face(); footprints and the
worker's side come from the construction Def rows.

STK-AR (2026-11-01): the production benches take their wiki sizes, with the
work surface on the middle of the worker's side (the interaction spot is in
front of the middle cell): 3x1 for the forges, machining table, sculpting
bench and smelter, 5x2 for the fabrication bench, 3x2 for the fuel refinery.

- fueled_smithy_<facing> (3x1): a stone hearth with a coal bed, a hood and a
  flue in the middle, leather bellows west, an anvil on a stump east. The
  coals are its glow layer (fueled_smithy_glow_<facing>), drawn while it
  burns; metadata `fuel: wood`.
- electric_smithy_<facing> (3x1): a steel induction hearth (glowing coil) in
  the middle, a control cabinet (glow) west, an anvil east.
- machining_table_<facing> (3x1): a steel table with a lathe in the middle,
  a vise west, a drill press with a power lamp (glow) east.
- fabrication_bench_<facing> (5x2): a long steel console with five screens
  (glow), an articulated arm over the middle work mat, parts bins and a
  component rack. The worker stands at the middle of the south side.
- art_bench_<facing> (3x1): a heavy plank bench with a turntable holding a
  roughed-out block in the middle, spare blocks, mallet and chisels. Wood;
  the renderer's stuff tint covers stone and steel benches as it does for
  the dresser.
- electric_smelter_<facing> (3x1): a lined furnace with a glowing mouth
  (glow) in the middle, a control cabinet (glow) west, a crucible frame and
  an ingot mould east.
- biofuel_refinery_<facing> (3x2): a feed hopper in the middle of the south
  row (the worker's spot), two upright tanks joined by pipes at the back, a
  gauge box (glow) and a filled drum at the outlet.
- fermenting_barrel (1x1): a hooped barrel lying in a cradle with a tap.
- sculpture_small (1x1), sculpture_large (1x1), sculpture_grand (2x2):
  abstract pieces on plinths (the large and grand pieces are authored on
  2x2 and 3x3 and shrunk to the wiki footprints by _scaled). The carved
  body is the `stone_tint` tint
  group (a neutral body plus a _mask.png), so the renderer can multiply it
  by the piece's stuff colour; the plinth stays a dark neutral.
- stool (1x1): a round seat on three splayed legs.
- armchair_<facing> (1x1): a padded chair, the upholstery the `cloth` tint
  group (its fabric), on a wooden frame. Attachment `seat` like the chair.
- crafting_spot, butcher_spot (overlay, 1x1 flat markings): a chalk ring
  with a hammer, or a cleaver over a drain mark.

    make art FAMILIES=workshop
"""
import math

import numpy as np

from .. import contracts as C
from .. import meta
from .. import stage as S
from . import overlay as O
from . import power as P

FACINGS = ("south", "east", "north", "west")


def _legs(x0, x1, y0, y1, h, mat="metal_dark", r=0.055):
    for i, x in enumerate((x0, x1)):
        for j, y in enumerate((y0, y1)):
            S.box("leg%d%d" % (i, j), x - r, y - r, 0.0, x + r, y + r, h, mat, bevel=0.012)


def _anvil(cx, cy, z, s=1.0):
    """An anvil on its own (horn to the east), base at height z."""
    S.box("anvil_foot", cx - 0.13 * s, cy - 0.10 * s, z, cx + 0.13 * s, cy + 0.10 * s, z + 0.05 * s, "metal_dark",
          bevel=0.01)
    S.box("anvil_waist", cx - 0.07 * s, cy - 0.06 * s, z + 0.05 * s, cx + 0.07 * s, cy + 0.06 * s, z + 0.14 * s,
          "metal_dark", bevel=0.008)
    S.box("anvil_face", cx - 0.17 * s, cy - 0.08 * s, z + 0.14 * s, cx + 0.12 * s, cy + 0.08 * s, z + 0.21 * s,
          "steel_dark", bevel=0.012)
    P._rod("anvil_horn", (cx + 0.12 * s, cy, z + 0.185 * s), (cx + 0.27 * s, cy, z + 0.185 * s), 0.035 * s,
           "steel_dark", segs=10, bevel=0.0)


# ------------------------------------------------------------------ benches
def _fueled_smithy():
    """3x1 wood-fired forge (STK-AR): the stone hearth with its coal bed
    (glow), hood and flue on the middle cell, where the smith stands;
    leather bellows west, an anvil on a stump east."""
    S.box("hearth", 0.98, -0.92, 0.0, 2.02, -0.12, 0.56, "stone_dark", bevel=0.04, segments=2)
    S.box("hearth_lip", 0.95, -0.95, 0.52, 2.05, -0.09, 0.60, "stone", bevel=0.03)
    S.box("firepit", 1.10, -0.80, 0.58, 1.90, -0.30, 0.61, "ash_char", bevel=0.01)
    for k, (x, y) in enumerate(((1.28, -0.62), (1.50, -0.50), (1.72, -0.64), (1.40, -0.70), (1.64, -0.42))):
        P.glow(S.blob("coal%d" % k, x, y, 0.62, 0.075, "ash_char", seed=40 + k, lumps=0.25, squash=0.55),
               "gland_ember")
    S.box("hood", 1.02, -0.30, 0.60, 1.98, -0.10, 0.92, "metal_dark", bevel=0.03)
    S.cyl("flue", 1.50, -0.20, 0.09, 0.92, 1.30, "metal_dark", segs=12, bevel=0.01)
    S.cyl("flue_cap", 1.50, -0.20, 0.13, 1.30, 1.34, "steel_dark", segs=12, bevel=0.01)
    # west: bellows on a trestle, nozzle into the hearth's west cheek; a log pile
    for k, x in enumerate((0.36, 0.84)):
        S.box("trestle%d" % k, x - 0.04, -0.46, 0.0, x + 0.04, -0.14, 0.30, "wood", bevel=0.01)
    S.box("bellows_lo", 0.30, -0.46, 0.30, 0.90, -0.14, 0.34, "wood", bevel=0.01)
    S.ellipsoid("bellows_bag", 0.60, -0.30, 0.42, 0.28, 0.16, 0.08, "leather", segs=16, rings=8)
    S.box("bellows_hi", 0.30, -0.46, 0.48, 0.90, -0.14, 0.52, "wood", bevel=0.01)
    S.box("bellows_grip", 0.18, -0.34, 0.48, 0.30, -0.26, 0.52, "wood", bevel=0.005)
    P._rod("nozzle", (0.90, -0.30, 0.38), (0.98, -0.30, 0.38), 0.025, "brass", segs=8, bevel=0.0)
    for k, (y, z) in enumerate(((-0.82, 0.06), (-0.66, 0.06), (-0.74, 0.17))):
        P._rod("log%d" % k, (0.20, y, z), (0.80, y, z), 0.06, "bark", segs=10, bevel=0.005)
    # tongs leaning on the hearth front
    P._rod("tongs_a", (1.96, -0.94, 0.62), (2.04, -0.97, 0.02), 0.012, "metal_dark", segs=6, bevel=0.0)
    P._rod("tongs_b", (1.94, -0.94, 0.62), (2.08, -0.96, 0.02), 0.012, "metal_dark", segs=6, bevel=0.0)
    # east: stump and anvil, a hammer at its foot
    S.cyl("stump", 2.50, -0.56, 0.22, 0.0, 0.40, "bark", segs=16, bevel=0.02)
    S.cyl("stump_top", 2.50, -0.56, 0.20, 0.40, 0.42, "wood_plank", segs=16, bevel=0.0)
    _anvil(2.48, -0.56, 0.42)
    S.box("hammer_head", 2.70, -0.90, 0.0, 2.82, -0.82, 0.07, "steel_dark", bevel=0.01)
    P._rod("hammer_handle", (2.76, -0.86, 0.035), (2.90, -0.60, 0.035), 0.018, "wood", segs=6, bevel=0.0)
    return (3, 1), 1.34


def _electric_smithy():
    """3x1 electric forge (STK-AR): the steel induction cabinet with its coil
    (glow) round a crucible pit on the middle cell; a control cabinet with a
    display (glow) west; an anvil on a steel stand and ingots east."""
    S.box("plinth", 0.08, -0.92, 0.0, 2.04, -0.10, 0.08, "steel_dark", bevel=0.01)
    S.box("cabinet", 0.96, -0.94, 0.08, 2.04, -0.08, 0.58, "steel", bevel=0.035)
    S.box("top", 0.94, -0.96, 0.58, 2.06, -0.06, 0.63, "metal_dark", bevel=0.02)
    S.cyl("pit", 1.50, -0.54, 0.26, 0.63, 0.66, "backdrop", segs=24, bevel=0.0)
    for k in range(3):
        P.glow(S.cyl("coil%d" % k, 1.50, -0.54, 0.23 - 0.05 * k, 0.655 + 0.004 * k, 0.665 + 0.004 * k,
                     "copper", segs=24, bevel=0.0), "gland_ember")
    S.cyl("crucible", 1.50, -0.54, 0.08, 0.66, 0.73, "ceramic", segs=14, bevel=0.01)
    S.box("vent", 1.10, -0.955, 0.18, 1.90, -0.94, 0.46, "metal_dark", bevel=0.006)
    for k in range(5):
        z = 0.22 + 0.05 * k
        S.box("slat%d" % k, 1.12, -0.965, z, 1.88, -0.955, z + 0.02, "steel_dark", bevel=0.0)
    # west: control cabinet with a display and dials, cable into the cabinet
    S.box("console", 0.10, -0.80, 0.08, 0.90, -0.14, 0.92, "steel_dark", bevel=0.03)
    S.box("console_lid", 0.08, -0.82, 0.92, 0.92, -0.12, 0.96, "metal_dark", bevel=0.015)
    P.glow(S.box("display", 0.22, -0.815, 0.62, 0.78, -0.80, 0.82, "backdrop", bevel=0.004), "indicator")
    for k, x in enumerate((0.30, 0.50, 0.70)):
        S.cyl("dial%d" % k, x, -0.48, 0.045, 0.96, 1.00, "metal_dark", segs=10, bevel=0.005)
    P._rod("cable", (0.90, -0.30, 0.40), (0.96, -0.30, 0.40), 0.03, "fur_black", segs=8, bevel=0.0)
    # east: anvil on a squat steel stand, an ingot stack
    S.cyl("stand", 2.50, -0.58, 0.18, 0.0, 0.40, "steel_dark", segs=8, r_top=0.12, bevel=0.015)
    _anvil(2.48, -0.58, 0.40)
    for k, (x, y, z) in enumerate(((2.66, -0.24, 0.0), (2.66, -0.12, 0.0), (2.66, -0.18, 0.05))):
        S.box("ingot%d" % k, x, y - 0.05, z, x + 0.22, y + 0.05, z + 0.05, "steel", bevel=0.01)
    return (3, 1), 1.0


def _machining_table():
    """3x1 machining table (STK-AR): a steel top on a drawer cabinet; the
    lathe on the middle cell, where the machinist stands (headstock, bed,
    tailstock); a vise west; a drill press with its power lamp (glow) east."""
    _legs(0.12, 2.88, -0.18, -0.82, 0.50)
    S.box("drawers", 0.10, -0.86, 0.10, 0.90, -0.16, 0.48, "steel_dark", bevel=0.02)
    for k, z in enumerate((0.16, 0.32)):
        S.box("drawer%d" % k, 0.16, -0.875, z, 0.84, -0.86, z + 0.12, "steel", bevel=0.006)
    S.box("top", 0.04, -0.94, 0.50, 2.96, -0.06, 0.58, "steel", bevel=0.025)
    # middle: the lathe with a part in the chuck, a chip tray in front
    S.box("lathe_bed", 1.06, -0.58, 0.58, 1.98, -0.30, 0.66, "metal_dark", bevel=0.012)
    S.box("headstock", 1.02, -0.62, 0.58, 1.32, -0.24, 0.86, "armour_olive", bevel=0.025)
    P._rod("chuck", (1.32, -0.44, 0.76), (1.40, -0.44, 0.76), 0.08, "steel_dark", segs=14, bevel=0.006)
    P._rod("stock", (1.40, -0.44, 0.76), (1.76, -0.44, 0.76), 0.04, "brass", segs=12, bevel=0.0)
    S.box("tailstock", 1.78, -0.58, 0.66, 1.96, -0.30, 0.84, "armour_olive", bevel=0.02)
    P._rod("quill", (1.76, -0.44, 0.76), (1.78, -0.44, 0.76), 0.022, "steel", segs=8, bevel=0.0)
    S.box("tool_post", 1.52, -0.72, 0.66, 1.64, -0.58, 0.74, "steel_dark", bevel=0.008)
    S.box("chip_tray", 1.10, -0.90, 0.58, 1.90, -0.70, 0.61, "metal_dark", bevel=0.006)
    for k, x in enumerate((1.30, 1.48, 1.66)):
        S.blob("chip%d" % k, x, -0.80, 0.615, 0.03, "brass", seed=80 + k, lumps=0.35, squash=0.4)
    # west: vise and gears
    S.box("vise_base", 0.20, -0.84, 0.58, 0.50, -0.58, 0.66, "metal_dark", bevel=0.015)
    S.box("vise_jaw", 0.24, -0.90, 0.62, 0.46, -0.80, 0.74, "steel", bevel=0.015)
    for k, (x, y, r) in enumerate(((0.62, -0.36, 0.09), (0.80, -0.50, 0.06))):
        S.cyl("gear%d" % k, x, y, r, 0.58, 0.62, "brass", segs=12, bevel=0.006)
        S.cyl("gear_hub%d" % k, x, y, r * 0.35, 0.58, 0.63, "steel_dark", segs=8, bevel=0.0)
    # east: the drill press and a file
    S.box("press_base", 2.36, -0.44, 0.58, 2.86, -0.12, 0.62, "metal_dark", bevel=0.01)
    S.cyl("column", 2.70, -0.22, 0.04, 0.62, 1.18, "steel", segs=10, bevel=0.0)
    S.box("press_head", 2.38, -0.34, 0.98, 2.84, -0.12, 1.18, "armour_olive", bevel=0.025)
    S.cyl("spindle", 2.50, -0.30, 0.025, 0.80, 0.98, "steel_dark", segs=8, bevel=0.0)
    S.cyl("press_table", 2.52, -0.30, 0.10, 0.74, 0.77, "metal_dark", segs=16, bevel=0.005)
    P.glow(S.box("power_lamp", 2.54, -0.30, 1.18, 2.66, -0.18, 1.20, "backdrop", bevel=0.004), "indicator")
    S.box("file", 2.20, -0.84, 0.58, 2.70, -0.80, 0.60, "steel_dark", bevel=0.004)
    return (3, 1), 1.20


def _fabrication_bench():
    """5x2 fabrication bench (STK-AR, wiki size): a long steel work surface,
    a back console with five screens (glow), an articulated arm over a dark
    mat on the middle column, where the worker stands, parts bins west and
    a component rack east."""
    for i, x in enumerate((0.14, 1.70, 3.30, 4.86)):
        for j, y in enumerate((-0.30, -1.80)):
            S.box("leg%d%d" % (i, j), x - 0.07, y - 0.07, 0.0, x + 0.07, y + 0.07, 0.52, "metal_dark", bevel=0.015)
    S.box("shelf", 0.10, -1.86, 0.14, 4.90, -0.24, 0.20, "steel_dark", bevel=0.015)
    for k, x in enumerate((0.40, 1.00, 1.60, 3.40, 4.00, 4.60)):
        S.box("crate%d" % k, x - 0.22, -1.50, 0.20, x + 0.22, -0.70, 0.42, "armour_olive" if k % 2 else "steel",
              bevel=0.02)
    S.box("top", 0.04, -1.94, 0.52, 4.96, -0.06, 0.60, "steel", bevel=0.03)
    S.box("console", 0.06, -0.62, 0.60, 4.94, -0.06, 1.10, "steel_dark", bevel=0.04)
    S.box("console_top", 0.04, -0.66, 1.10, 4.96, -0.04, 1.16, "metal_dark", bevel=0.02)
    for k, x in enumerate((0.26, 1.22, 2.14, 3.06, 4.02)):
        S.box("bezel%d" % k, x, -0.635, 0.70, x + 0.72, -0.60, 1.04, "metal_dark", bevel=0.012)
        P.glow(S.box("screen%d" % k, x + 0.05, -0.645, 0.75, x + 0.67, -0.63, 1.00, "backdrop", bevel=0.004),
               "indicator")
    for k, x in enumerate((0.40, 0.60, 4.40, 4.60)):
        P.glow(S.box("top_lamp%d" % k, x - 0.05, -0.40, 1.16, x + 0.05, -0.30, 1.175, "backdrop", bevel=0.003),
               "indicator")
    # middle: work mat, circuit board and the articulated arm reaching over it
    S.box("mat", 1.80, -1.74, 0.60, 3.20, -0.80, 0.615, "backdrop", bevel=0.004)
    S.box("board", 2.24, -1.42, 0.615, 2.76, -1.04, 0.64, "circuit", bevel=0.005)
    for k in range(3):
        S.box("chip%d" % k, 2.30 + 0.15 * k, -1.32, 0.64, 2.38 + 0.15 * k, -1.22, 0.665, "metal_dark", bevel=0.0)
    S.cyl("arm_base", 3.50, -1.00, 0.13, 0.60, 0.70, "metal_dark", segs=16, bevel=0.01)
    P._rod("arm_lo", (3.50, -1.00, 0.70), (3.30, -1.16, 1.12), 0.045, "corn_yellow", segs=10, bevel=0.006)
    S.ellipsoid("arm_joint", 3.30, -1.16, 1.12, 0.07, 0.07, 0.07, "metal_dark", segs=12, rings=8)
    P._rod("arm_hi", (3.30, -1.16, 1.12), (2.70, -1.24, 0.92), 0.04, "corn_yellow", segs=10, bevel=0.006)
    S.cyl("arm_tool", 2.66, -1.24, 0.035, 0.76, 0.94, "steel_dark", segs=10, r_top=0.05, bevel=0.0)
    # west: bins of parts and a spare board
    for k, (x, mat) in enumerate(((0.24, "copper"), (0.50, "steel"), (0.76, "brass"))):
        S.box("bin%d" % k, x, -1.64, 0.60, x + 0.22, -1.30, 0.70, "metal_dark", bevel=0.01)
        S.box("parts%d" % k, x + 0.02, -1.62, 0.68, x + 0.20, -1.32, 0.71, mat, bevel=0.0)
    S.box("spare_board", 1.06, -1.70, 0.60, 1.50, -1.34, 0.625, "circuit", bevel=0.005)
    S.box("solder", 0.30, -1.10, 0.60, 0.80, -0.94, 0.66, "steel_dark", bevel=0.01)
    # east: a component rack of small drawers and a scope probe
    S.box("rack", 4.06, -1.70, 0.60, 4.84, -1.10, 0.86, "steel_dark", bevel=0.02)
    for r in range(2):
        for c in range(3):
            S.box("cell%d%d" % (r, c), 4.10 + 0.25 * c, -1.715, 0.64 + 0.11 * r, 4.30 + 0.25 * c, -1.70,
                  0.72 + 0.11 * r, "pod_hull", bevel=0.004)
    S.box("probe", 3.50, -1.66, 0.60, 3.86, -1.62, 0.63, "fur_black", bevel=0.004)
    S.box("cable", 4.90, -0.30, 0.10, 4.96, -0.20, 0.60, "fur_black", bevel=0.01)
    return (5, 2), 1.18


def _art_bench():
    """3x1 sculpting bench (STK-AR): a thick plank top; the turntable with a
    roughed-out block on the middle cell, where the sculptor stands; spare
    blocks west; mallet, chisels and a rasp east."""
    _legs(0.14, 2.86, -0.20, -0.80, 0.48, mat="wood", r=0.07)
    S.box("stretcher", 0.12, -0.56, 0.12, 2.88, -0.44, 0.18, "wood", bevel=0.015)
    S.box("top", 0.04, -0.94, 0.48, 2.96, -0.06, 0.60, "wood_plank", bevel=0.035, segments=2)
    # middle: the turntable and the half-carved block
    S.cyl("turn_stand", 1.50, -0.52, 0.10, 0.60, 0.68, "metal_dark", segs=14, bevel=0.01)
    S.cyl("turntable", 1.50, -0.52, 0.32, 0.68, 0.72, "wood", segs=24, bevel=0.012)
    S.box("block", 1.32, -0.70, 0.72, 1.68, -0.34, 0.94, "stone_neutral", bevel=0.03)
    S.ellipsoid("roughed", 1.50, -0.52, 1.02, 0.15, 0.14, 0.12, "stone_neutral", segs=16, rings=10)
    S.box("cut", 1.58, -0.71, 0.80, 1.68, -0.62, 0.94, "stone_neutral_dark", bevel=0.01)
    # west: spare blocks
    S.box("spare_a", 0.20, -0.70, 0.60, 0.62, -0.30, 0.86, "stone_neutral_dark", bevel=0.03)
    S.box("spare_b", 0.66, -0.62, 0.60, 0.90, -0.38, 0.76, "stone_neutral", bevel=0.025)
    # east: mallet, chisels, rasp
    S.box("mallet_head", 2.10, -0.74, 0.60, 2.30, -0.60, 0.72, "wood", bevel=0.02)
    P._rod("mallet_handle", (2.20, -0.60, 0.66), (2.24, -0.30, 0.64), 0.022, "bark", segs=8, bevel=0.0)
    for k, y in enumerate((-0.80, -0.72, -0.64)):
        P._rod("chisel%d" % k, (2.42, y, 0.62), (2.66, y + 0.02, 0.62), 0.012, "steel", segs=6, bevel=0.0)
        P._rod("chisel_grip%d" % k, (2.66, y + 0.02, 0.62), (2.78, y + 0.03, 0.62), 0.02, "wood", segs=6,
               bevel=0.0)
    S.box("rasp", 2.40, -0.40, 0.60, 2.82, -0.34, 0.62, "metal_dark", bevel=0.004)
    for k, (x, y) in enumerate(((1.12, -0.82), (1.88, -0.74), (1.20, -0.24), (1.84, -0.28))):
        S.blob("chip%d" % k, x, y, 0.61, 0.03, "stone_neutral", seed=70 + k, lumps=0.3, squash=0.5)
    return (3, 1), 1.14


def _electric_smelter():
    """3x1 electric smelter (STK-AR): the lined furnace with its glowing
    mouth (glow) and heat hood on the middle cell, where the worker stands;
    a control cabinet (glow) and slag bin west; a pouring frame with a
    crucible and an ingot mould east."""
    S.box("plinth", 0.06, -0.94, 0.0, 2.10, -0.08, 0.08, "steel_dark", bevel=0.01)
    S.box("furnace", 0.96, -0.90, 0.08, 2.04, -0.10, 0.86, "steel", bevel=0.04, segments=2)
    S.box("lining", 1.10, -0.915, 0.30, 1.90, -0.88, 0.68, "stone_dark", bevel=0.02)
    P.glow(S.box("mouth", 1.18, -0.925, 0.38, 1.82, -0.90, 0.60, "ash_char", bevel=0.012), "gland_ember")
    S.box("hood", 0.92, -0.94, 0.86, 2.08, -0.06, 0.94, "metal_dark", bevel=0.02)
    S.cyl("vent_ring", 1.50, -0.50, 0.21, 0.94, 0.97, "steel_dark", segs=20, bevel=0.006)
    P.glow(S.cyl("vent_glow", 1.50, -0.50, 0.15, 0.97, 0.98, "ash_char", segs=20, bevel=0.0), "gland_ember")
    for k, x in enumerate((1.06, 1.94)):
        S.box("strap%d" % k, x - 0.03, -0.915, 0.08, x + 0.03, -0.895, 0.86, "metal_dark", bevel=0.0)
    # west: control cabinet with lamp and dials, a slag bin
    S.box("panel", 0.12, -0.40, 0.08, 0.84, -0.12, 1.00, "steel_dark", bevel=0.02)
    P.glow(S.box("panel_lamp", 0.30, -0.415, 0.78, 0.66, -0.40, 0.92, "backdrop", bevel=0.003), "indicator")
    P.glow(S.box("panel_top_lamp", 0.30, -0.32, 1.00, 0.66, -0.20, 1.015, "backdrop", bevel=0.003), "indicator")
    for k, x in enumerate((0.30, 0.48, 0.66)):
        S.cyl("dial%d" % k, x, -0.41, 0.04, 0.56, 0.60, "metal_dark", segs=10, bevel=0.0, ry=0.04)
    S.box("slag_bin", 0.16, -0.92, 0.08, 0.80, -0.52, 0.34, "metal_dark", bevel=0.02)
    S.blob("slag", 0.48, -0.72, 0.34, 0.16, "ash_char", seed=55, lumps=0.3, squash=0.5)
    # east: pouring frame with a crucible, a mould of three ingots
    S.box("frame_w", 2.22, -0.70, 0.0, 2.28, -0.30, 0.56, "metal_dark", bevel=0.008)
    S.box("frame_e", 2.78, -0.70, 0.0, 2.84, -0.30, 0.56, "metal_dark", bevel=0.008)
    P._rod("axle", (2.28, -0.50, 0.50), (2.78, -0.50, 0.50), 0.02, "steel_dark", segs=8, bevel=0.0)
    S.cyl("crucible", 2.53, -0.50, 0.14, 0.38, 0.62, "ceramic", segs=16, r_top=0.16, bevel=0.012)
    S.cyl("melt", 2.53, -0.50, 0.12, 0.60, 0.615, "gold", segs=16, bevel=0.0)
    S.box("mould", 2.22, -0.96, 0.0, 2.86, -0.76, 0.10, "metal_dark", bevel=0.012)
    for k in range(3):
        x = 2.28 + 0.19 * k
        S.box("ingot%d" % k, x, -0.93, 0.08, x + 0.15, -0.79, 0.12, "steel", bevel=0.008)
    return (3, 1), 1.12


def _biofuel_refinery():
    """3x2 fuel refinery (STK-AR, wiki size): the feed hopper on the middle
    of the south row, where the worker loads it; two upright tanks at the
    back corners joined by a bridge pipe; a gauge box (glow) east and a
    filled drum at the outlet west."""
    S.box("skid", 0.06, -1.94, 0.0, 2.94, -0.06, 0.08, "steel_dark", bevel=0.015)
    for k, x in enumerate((0.30, 1.50, 2.70)):
        S.box("rail%d" % k, x - 0.06, -1.90, 0.08, x + 0.06, -0.10, 0.12, "metal_dark", bevel=0.0)
    # middle: hopper on legs over the feed screw
    for k, (x, y) in enumerate(((1.16, -1.10), (1.84, -1.10), (1.16, -1.76), (1.84, -1.76))):
        S.box("hleg%d" % k, x - 0.04, y - 0.04, 0.08, x + 0.04, y + 0.04, 0.70, "metal_dark", bevel=0.008)
    S.cyl("hopper", 1.50, -1.43, 0.16, 0.50, 1.02, "steel", segs=4, r_top=0.42, bevel=0.02)
    S.cyl("hopper_rim", 1.50, -1.43, 0.43, 1.02, 1.06, "metal_dark", segs=4, bevel=0.01)
    S.blob("feed", 1.50, -1.43, 1.00, 0.24, "vegetation", seed=31, lumps=0.25, squash=0.4)
    P._rod("screw", (1.50, -1.43, 0.40), (1.50, -0.86, 0.40), 0.06, "metal_dark", segs=12, bevel=0.006)
    # back: two tanks, bands, domed tops, the bridge and the drop to the outlet
    for k, x in enumerate((0.56, 2.44)):
        S.cyl("tank%d" % k, x, -0.52, 0.36, 0.08, 1.20, "steel", segs=24, bevel=0.03)
        S.ellipsoid("dome%d" % k, x, -0.52, 1.20, 0.36, 0.36, 0.14, "steel", segs=24, rings=10)
        for j, z in enumerate((0.36, 0.84)):
            S.cyl("band%d%d" % (k, j), x, -0.52, 0.37, z, z + 0.05, "metal_dark", segs=24, bevel=0.0)
    P._rod("bridge", (0.56, -0.52, 1.30), (2.44, -0.52, 1.30), 0.045, "copper", segs=10, bevel=0.0)
    P._rod("feed_up", (1.50, -0.86, 0.40), (1.50, -0.60, 0.40), 0.05, "metal_dark", segs=10, bevel=0.0)
    S.box("mixer", 1.26, -0.74, 0.08, 1.74, -0.30, 0.66, "steel_dark", bevel=0.03)
    P._rod("mixer_up", (1.50, -0.52, 0.66), (1.50, -0.52, 1.30), 0.04, "copper", segs=10, bevel=0.0)
    P._rod("drop", (0.56, -0.88, 0.60), (0.56, -1.30, 0.60), 0.04, "copper", segs=10, bevel=0.0)
    # east: gauge box on a post, lamp on its top
    S.box("gauge_post", 2.42, -1.30, 0.08, 2.50, -1.22, 0.60, "metal_dark", bevel=0.005)
    S.box("gauge_box", 2.30, -1.40, 0.60, 2.62, -1.20, 0.96, "steel_dark", bevel=0.015)
    S.cyl("gauge", 2.46, -1.41, 0.06, 0.74, 0.76, "ceramic", segs=12, bevel=0.0, ry=0.06)
    P.glow(S.box("gauge_lamp", 2.36, -1.38, 0.96, 2.56, -1.22, 0.975, "backdrop", bevel=0.003), "indicator")
    # west: the outlet drum
    S.cyl("drum", 0.52, -1.52, 0.26, 0.08, 0.70, "food_red", segs=20, bevel=0.02)
    for j, z in enumerate((0.26, 0.52)):
        S.cyl("drum_band%d" % j, 0.52, -1.52, 0.265, z, z + 0.03, "metal_dark", segs=20, bevel=0.0)
    S.cyl("drum_cap", 0.58, -1.46, 0.05, 0.70, 0.73, "corn_yellow", segs=10, bevel=0.0)
    P._rod("spout", (0.56, -1.30, 0.60), (0.56, -1.40, 0.80), 0.03, "copper", segs=8, bevel=0.0)
    return (3, 2), 1.34


def _fermenting_barrel():
    """1x1 fermenting barrel: a fat hooped barrel lying north-south on two
    chocks, a bung on top and a brass tap on its south head."""
    for k, y in enumerate((-0.26, -0.74)):
        S.box("chock%d" % k, 0.14, y - 0.07, 0.0, 0.86, y + 0.07, 0.16, "bark", bevel=0.015)
    n0 = len(S.objects())
    S.cyl("barrel", 0.5, -0.5, 0.33, 0.12, 0.88, "wood", segs=24, bevel=0.04)
    S.ellipsoid("belly", 0.5, -0.5, 0.50, 0.37, 0.37, 0.30, "wood", segs=24, rings=12)
    for k, z in enumerate((0.18, 0.36, 0.64, 0.82)):
        r = 0.345 if k in (0, 3) else 0.375
        S.cyl("hoop%d" % k, 0.5, -0.5, r, z - 0.02, z + 0.02, "metal_dark", segs=24, bevel=0.0)
    S.cyl("head_n", 0.5, -0.5, 0.30, 0.86, 0.885, "wood_plank", segs=24, bevel=0.0)
    S.cyl("head_s", 0.5, -0.5, 0.30, 0.115, 0.14, "wood_plank", segs=24, bevel=0.0)
    # lay it down (axis south-north) and settle it into the chocks
    P._spin(P._since(n0), (0.5, -0.5, 0.5), "X", 90)
    for ob in P._since(n0):
        ob.matrix_world.translation.z -= 0.10
    S.cyl("bung", 0.5, -0.5, 0.045, 0.74, 0.79, "bark", segs=10, bevel=0.006)
    P._rod("tap", (0.5, -0.87, 0.28), (0.5, -0.97, 0.28), 0.03, "brass", segs=8, bevel=0.0)
    S.box("tap_handle", 0.47, -0.96, 0.28, 0.53, -0.94, 0.37, "brass", bevel=0.004)
    return (1, 1), 0.80


# --------------------------------------------------------------- sculptures
def _arc(name, cx, cy, z0, r, a0, a1, n, rod, mat, axis="X"):
    """Rod segments along a vertical arc of radius r centred (cx, cy, z0),
    in the x-z plane (axis "X") or y-z plane (axis "Y")."""
    pts = []
    for k in range(n + 1):
        a = math.radians(a0 + (a1 - a0) * k / n)
        u, w = r * math.cos(a), r * math.sin(a)
        pts.append((cx + u, cy, z0 + w) if axis == "X" else (cx, cy + u, z0 + w))
    for k in range(n):
        P._rod("%s%d" % (name, k), pts[k], pts[k + 1], rod, mat, segs=12, bevel=0.0)
        S.ellipsoid("%s_j%d" % (name, k), *pts[k + 1], rod, rod, rod, mat, segs=12, rings=8)
    return pts


def _plinth(cx, cy, hw, hd, h, steps=1):
    for k in range(steps):
        g = 0.06 * (steps - 1 - k)
        S.box("plinth%d" % k, cx - hw - g, cy - hd - g, h * k / steps, cx + hw + g, cy + hd + g, h * (k + 1) / steps,
              "stone_neutral_dark", bevel=0.02)


def _sculpture_small():
    """1x1: a square plinth and a column of offset, turning lozenges that
    narrow as they rise (a twisting cairn)."""
    _plinth(0.5, -0.5, 0.28, 0.28, 0.22)
    z = 0.22
    for k in range(5):
        a = k * 1.15
        r = 0.20 - 0.025 * k
        dx, dy = 0.05 * math.cos(a), 0.05 * math.sin(a)
        h = 0.11 - 0.008 * k
        S.ellipsoid("stone%d" % k, 0.5 + dx, -0.5 + dy, z + h, r, r * 0.72, h, "stone_tint", segs=20, rings=10)
        z += h * 1.75
    S.ellipsoid("cap", 0.5, -0.5, z + 0.06, 0.06, 0.06, 0.09, "stone_tint", segs=14, rings=8)
    return (1, 1), z + 0.16


def _sculpture_large():
    """2x2: a round plinth and two interlocked hoops, each tilted off the
    upright so both read from above, round a sphere on a low saddle."""
    S.cyl("plinth", 1.0, -1.0, 0.70, 0.0, 0.14, "stone_neutral_dark", segs=32, bevel=0.03)
    S.cyl("plinth_top", 1.0, -1.0, 0.62, 0.14, 0.22, "stone_neutral_dark", segs=32, bevel=0.02)
    S.ellipsoid("saddle", 1.0, -1.0, 0.26, 0.26, 0.26, 0.07, "stone_tint", segs=20, rings=8)
    c = (1.0, -1.0, 0.72)
    for k, (axis, spin_axis, deg) in enumerate((("X", "X", -42), ("Y", "Y", 42))):
        n0 = len(S.objects())
        _arc("hoop%d_" % k, c[0], c[1], c[2], 0.44, 0, 360, 24, 0.07, "stone_tint", axis=axis)
        P._spin(P._since(n0), c, spin_axis, deg)
    S.ellipsoid("orb", c[0], c[1], c[2], 0.20, 0.20, 0.20, "stone_tint", segs=24, rings=14)
    return (2, 2), 1.24


def _sculpture_grand():
    """3x3: a stepped square plinth; three tapering blades rise from its
    corners, lean in and meet under a sphere, with a hoop round their waist."""
    _plinth(1.5, -1.5, 1.10, 1.10, 0.30, steps=2)
    cx, cy, top = 1.5, -1.5, 2.10
    feet = [(cx + 0.70 * math.cos(math.radians(a)), cy + 0.70 * math.sin(math.radians(a)))
            for a in (-90, 30, 150)]
    for k, (fx, fy) in enumerate(feet):
        # a blade: three rods narrowing toward the apex, bowed outward
        mx, my = cx + (fx - cx) * 0.82, cy + (fy - cy) * 0.82
        P._rod("blade_lo%d" % k, (fx, fy, 0.30), (mx, my, 1.10), 0.15, "stone_tint", segs=14, bevel=0.0)
        S.ellipsoid("knee%d" % k, mx, my, 1.10, 0.14, 0.14, 0.14, "stone_tint", segs=14, rings=8)
        P._rod("blade_hi%d" % k, (mx, my, 1.10), (cx + (fx - cx) * 0.18, cy + (fy - cy) * 0.18, top - 0.10), 0.10,
               "stone_tint", segs=14, bevel=0.0)
        S.ellipsoid("foot%d" % k, fx, fy, 0.34, 0.20, 0.20, 0.10, "stone_tint", segs=16, rings=8)
    S.cyl("hoop", cx, cy, 0.66, 1.02, 1.16, "stone_tint", segs=32, bevel=0.03)
    S.cyl("hoop_in", cx, cy, 0.56, 1.03, 1.15, "stone_neutral_dark", segs=32, bevel=0.0)
    S.ellipsoid("orb", cx, cy, top + 0.08, 0.26, 0.26, 0.26, "stone_tint", segs=24, rings=14)
    return (3, 3), top + 0.36


def _scaled(fn, sxy, sz):
    """fn's piece shrunk to a smaller footprint (x, y by sxy about the
    footprint's top-left corner, height by sz), so the design keeps its
    look on the wiki footprint."""
    def build():
        from mathutils import Matrix
        fp, zmax = fn()
        S.transform(S.objects(), Matrix.Diagonal((sxy, sxy, sz, 1.0)))
        return (round(fp[0] * sxy), round(fp[1] * sxy)), zmax * sz
    return build


# ---------------------------------------------------------------- furniture
def _stool():
    """1x1 stool: a round seat on three splayed legs with a ring rail."""
    for k in range(3):
        a = math.radians(90 + 120 * k)
        P._rod("leg%d" % k, (0.5 + 0.13 * math.cos(a), -0.5 + 0.13 * math.sin(a), 0.40),
               (0.5 + 0.24 * math.cos(a), -0.5 + 0.24 * math.sin(a), 0.0), 0.035, "wood", segs=8, bevel=0.0)
    S.cyl("rail", 0.5, -0.5, 0.20, 0.15, 0.18, "wood", segs=20, bevel=0.0)
    S.cyl("rail_hole", 0.5, -0.5, 0.17, 0.149, 0.181, "wood", segs=20, bevel=0.0)
    S.cyl("seat", 0.5, -0.5, 0.23, 0.38, 0.46, "wood_plank", segs=24, bevel=0.025)
    return (1, 1), 0.46


def _armchair():
    """1x1 armchair facing south: wooden plinth and feet, a deep padded seat,
    rolled arms and a tall back, all upholstered (cloth tint)."""
    for k, (x, y) in enumerate(((0.18, -0.20), (0.82, -0.20), (0.18, -0.84), (0.82, -0.84))):
        S.box("foot%d" % k, x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.08, "bark", bevel=0.008)
    S.box("frame", 0.12, -0.90, 0.08, 0.88, -0.12, 0.16, "wood", bevel=0.02)
    S.box("base", 0.12, -0.90, 0.16, 0.88, -0.14, 0.34, "cloth", bevel=0.04, segments=2)
    S.box("cushion", 0.26, -0.88, 0.32, 0.74, -0.34, 0.44, "cloth", bevel=0.05, segments=2)
    S.box("back", 0.14, -0.36, 0.30, 0.86, -0.12, 0.92, "cloth", bevel=0.06, segments=2)
    for k, x in enumerate((0.19, 0.81)):
        S.box("arm%d" % k, x - 0.08, -0.88, 0.30, x + 0.08, -0.22, 0.58, "cloth", bevel=0.06, segments=2)
        P._rod("roll%d" % k, (x, -0.90, 0.58), (x, -0.22, 0.58), 0.085, "cloth", segs=16, bevel=0.0)
    for k, x in enumerate((0.30, 0.50, 0.70)):
        S.cyl("button%d" % k, x, -0.37, 0.018, 0.70, 0.72, "cloth", segs=8, bevel=0.0, ry=0.018)
    return (1, 1), 0.92


BUILDERS = {
    "fueled_smithy": (_fueled_smithy, {"attachments": {"worker": [1.5, 1.15]}, "building": "wood-fired forge",
                                       "fuel": "wood"}),
    "electric_smithy": (_electric_smithy, {"attachments": {"worker": [1.5, 1.15]}, "building": "electric forge",
                                           "recipes": "wood-fired forge"}),
    "machining_table": (_machining_table, {"attachments": {"worker": [1.5, 1.15]}, "building": "machining table"}),
    "fabrication_bench": (_fabrication_bench, {"attachments": {"worker": [2.5, 2.15]},
                                               "building": "fabrication bench"}),
    "art_bench": (_art_bench, {"attachments": {"worker": [1.5, 1.15]}, "building": "sculpting bench"}),
    "electric_smelter": (_electric_smelter, {"attachments": {"worker": [1.5, 1.15]},
                                             "building": "electric smelter"}),
    "biofuel_refinery": (_biofuel_refinery, {"attachments": {"worker": [1.5, 2.15]}, "building": "fuel refinery"}),
    "fermenting_barrel": (_fermenting_barrel, {"attachments": {"worker": [0.5, 1.15]},
                                               "building": "fermenting barrel"}),
    "sculpture_small": (_sculpture_small, {"building": "small sculpture", "stuff_tint": "stone_tint"}),
    "sculpture_large": (_scaled(_sculpture_large, 0.5, 0.85),
                        {"building": "large sculpture", "stuff_tint": "stone_tint"}),
    "sculpture_grand": (_scaled(_sculpture_grand, 2 / 3, 0.85),
                        {"building": "grand sculpture", "stuff_tint": "stone_tint"}),
    "stool": (_stool, {"attachments": {"seat": [0.5, 0.52]}, "building": "stool"}),
    "armchair": (_armchair, {"attachments": {"seat": [0.5, 0.52]}, "building": "armchair"}),
}
ROTATABLE = {"fueled_smithy", "electric_smithy", "machining_table", "fabrication_bench", "art_bench",
             "electric_smelter", "biofuel_refinery", "armchair"}

# Architect icons, cut from the south sprite (ui_icon/<name>).
ICONS = {
    "crafting_spot": ("overlay", "crafting_spot"),
    "butcher_spot": ("overlay", "butcher_spot"),
    "fueled_smithy": ("furniture", "fueled_smithy_south"),
    "electric_smithy": ("furniture", "electric_smithy_south"),
    "machining_table": ("furniture", "machining_table_south"),
    "fabrication_bench": ("furniture", "fabrication_bench_south"),
    "art_bench": ("furniture", "art_bench_south"),
    "electric_smelter": ("furniture", "electric_smelter_south"),
    "biofuel_refinery": ("furniture", "biofuel_refinery_south"),
    "fermenting_barrel": ("furniture", "fermenting_barrel"),
    "sculpture_small": ("furniture", "sculpture_small"),
    "sculpture_large": ("furniture", "sculpture_large"),
    "sculpture_grand": ("furniture", "sculpture_grand"),
    "stool": ("furniture", "stool"),
    "armchair": ("furniture", "armchair_south"),
}


# ------------------------------------------------------------ floor marks
CHALK = (0.93, 0.89, 0.80)
SHADE = (0.10, 0.09, 0.08)


def _spot(name, glyph, building):
    """A 1x1 flat chalk marking: a dashed ring (as the animal sleeping spot)
    round a glyph (a signed distance field in canvas px)."""
    N = C.PPT
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32) + 0.5
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    R = N / 2 - 12.0
    r = np.hypot(xx - c, yy - c)
    ring = np.abs(r - R)
    dash = (np.floor((np.arctan2(yy - c, xx - c) + np.pi) * R / 11.0) % 2) == 0
    O._paint(img, O._cov(r - R), SHADE, 0.16)
    O._paint(img, O._cov(ring - 2.6) * dash, SHADE, 0.45)
    O._paint(img, O._cov(ring - 1.8) * dash, CHALK, 0.9)
    d = glyph(xx, yy, c)
    O._paint(img, O._cov(d - 1.6), SHADE, 0.35)
    O._paint(img, O._cov(d), CHALK, 0.85)
    path = S.out_path("overlay", name)
    S.write_png(img, path)
    meta.write("overlay", name, {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0],
                                 "overlay": name, "building": building, "marking": True, "rotatable": False})
    return path


def _g_hammer(xx, yy, c):
    """A claw hammer standing on its handle, a saw blade laid under it."""
    handle = O._seg(xx, yy, c, c + 20, c, c - 6) - 3.4
    head = O._rbox(xx, yy, c - 2, c - 12, 15, 5.5, 2.0)
    claw = O._seg(xx, yy, c + 12, c - 12, c + 19, c - 4) - 2.6
    saw = O._rbox(xx, yy, c, c + 25, 22, 2.6, 1.0)
    return np.minimum(np.minimum(handle, head), np.minimum(claw, saw))


def _g_cleaver(xx, yy, c):
    """A cleaver (broad blade, short grip) over a cross drain grate."""
    blade = O._rbox(xx, yy, c - 4, c - 6, 17, 11, 3.0)
    hole = np.hypot(xx - (c - 14), yy - (c - 11)) - 3.0
    blade = np.maximum(blade, -hole)
    grip = O._seg(xx, yy, c + 13, c - 6, c + 26, c - 6) - 3.6
    grate = np.minimum(O._seg(xx, yy, c - 18, c + 17, c + 18, c + 17),
                       O._seg(xx, yy, c, c + 10, c, c + 24)) - 1.6
    return np.minimum(np.minimum(blade, grip), grate)


# ------------------------------------------------------------------ build
def build(only=None):
    """Every workshop sprite and icon; `only` (names from BUILDERS) renders just those."""
    out = [] if only else [_spot("crafting_spot", _g_hammer, "crafting spot"),
                           _spot("butcher_spot", _g_cleaver, "butchering spot")]
    for name, (fn, extra) in BUILDERS.items():
        if only and name not in only:
            continue
        for f in FACINGS if name in ROTATABLE else ("south",):
            fp0, zmax = fn()
            lits = P.glow_colours()
            fp = S.face(S.objects(), f, fp0)
            suffix = "_" + f if name in ROTATABLE else ""
            ex = dict(extra, rotatable=name in ROTATABLE, facing=f)
            gname = P.glow_name(name + suffix)
            if lits:   # emissive parts get a glow layer, drawn while powered or burning
                ex.update(emissive=lits[0], glow="furniture/" + gname)
            out.append(S.render_asset("furniture", name + suffix, footprint=fp, zmax=zmax, extra=ex))
            if lits:
                fn()
                S.face(S.objects(), f, fp0)
                out.append(P.glow_layer("furniture", gname, fp, zmax, "furniture/" + name + suffix, lits[0]))
    if not only:
        for icon, (fam, spr) in ICONS.items():
            p = O._icon(icon, fam, spr)
            if p:
                out.append(p)
    return out
