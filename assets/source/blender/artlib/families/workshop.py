"""Workshop family (wave D: PRD-3, BED-3; ART spec §12-§14): the production
benches, the crafting and butchering spots, the fermenting barrel, the three
sculptures, the stool and the armchair, plus their Architect icons. Our own
designs: nothing traced or reconstructed.

Sprites land in the furniture output family (the two spots in overlay, beside
the other floor markings) and follow furniture.py: geometry authored facing
SOUTH (the worker's side), rotated with stage.face(); footprints and the
worker's side come from the construction Def rows.

- fueled_smithy_<facing> (2x1): a stone hearth with a coal bed, a hood and a
  flue, leather bellows, an anvil on a stump. The coals are its glow layer
  (fueled_smithy_glow_<facing>), drawn while it burns; metadata `fuel: wood`.
- electric_smithy_<facing> (2x1): a steel induction hearth (glowing coil),
  a control box (glow) and an anvil.
- machining_table_<facing> (2x1): a steel table with a lathe, a drill press,
  a vise and a power lamp (glow).
- fabrication_bench_<facing> (3x2): a long steel console with screens
  (glow), an articulated arm over the work mat, parts bins and a circuit
  board. The worker stands at the middle of the south side.
- art_bench_<facing> (2x1): a heavy plank bench with a turntable holding a
  roughed-out block, mallet and chisels. Wood; the renderer's stuff tint
  covers stone and steel benches as it does for the dresser.
- electric_smelter_<facing> (2x1): a lined furnace with a glowing mouth
  (glow), a crucible, an ingot mould and a control panel (glow).
- biofuel_refinery_<facing> (2x2): a feed hopper, two upright tanks joined
  by pipes, a gauge box (glow) and a filled drum at the outlet. The worker
  stands south of the west column.
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
    """2x1 wood-fired forge: stone hearth with a coal bed (glow), a hood and
    flue at the back, bellows on its east flank, anvil on a stump."""
    S.box("hearth", 0.06, -0.92, 0.0, 1.06, -0.12, 0.56, "stone_dark", bevel=0.04, segments=2)
    S.box("hearth_lip", 0.03, -0.95, 0.52, 1.09, -0.09, 0.60, "stone", bevel=0.03)
    S.box("firepit", 0.18, -0.80, 0.58, 0.94, -0.30, 0.61, "ash_char", bevel=0.01)
    for k, (x, y) in enumerate(((0.34, -0.62), (0.56, -0.50), (0.78, -0.64), (0.46, -0.70), (0.70, -0.42))):
        P.glow(S.blob("coal%d" % k, x, y, 0.62, 0.075, "ash_char", seed=40 + k, lumps=0.25, squash=0.55),
               "gland_ember")
    S.box("hood", 0.10, -0.30, 0.60, 1.02, -0.10, 0.92, "metal_dark", bevel=0.03)
    S.cyl("flue", 0.56, -0.20, 0.09, 0.92, 1.30, "metal_dark", segs=12, bevel=0.01)
    S.cyl("flue_cap", 0.56, -0.20, 0.13, 1.30, 1.34, "steel_dark", segs=12, bevel=0.01)
    # bellows: two boards and a leather bag, nozzle into the hearth
    S.box("bellows_lo", 1.12, -0.46, 0.30, 1.40, -0.14, 0.34, "wood", bevel=0.01)
    S.ellipsoid("bellows_bag", 1.26, -0.30, 0.40, 0.14, 0.16, 0.07, "leather", segs=16, rings=8)
    S.box("bellows_hi", 1.12, -0.46, 0.46, 1.40, -0.14, 0.50, "wood", bevel=0.01)
    P._rod("nozzle", (1.12, -0.30, 0.38), (1.04, -0.30, 0.38), 0.025, "brass", segs=8, bevel=0.0)
    # stump and anvil at the front east
    S.cyl("stump", 1.52, -0.62, 0.20, 0.0, 0.40, "bark", segs=16, bevel=0.02)
    S.cyl("stump_top", 1.52, -0.62, 0.18, 0.40, 0.42, "wood_plank", segs=16, bevel=0.0)
    _anvil(1.50, -0.62, 0.42)
    # tongs leaning on the hearth, a hammer on the lip
    P._rod("tongs_a", (1.02, -0.94, 0.62), (1.10, -0.97, 0.02), 0.012, "metal_dark", segs=6, bevel=0.0)
    P._rod("tongs_b", (1.00, -0.94, 0.62), (1.14, -0.96, 0.02), 0.012, "metal_dark", segs=6, bevel=0.0)
    S.box("hammer_head", 1.74, -0.84, 0.0, 1.86, -0.76, 0.07, "steel_dark", bevel=0.01)
    P._rod("hammer_handle", (1.80, -0.80, 0.035), (1.92, -0.50, 0.035), 0.018, "wood", segs=6, bevel=0.0)
    return (2, 1), 1.34


def _electric_smithy():
    """2x1 electric forge: a steel cabinet with an induction coil (glow)
    round a crucible pit, a control box with a display (glow), an anvil on
    a steel stand."""
    S.box("plinth", 0.08, -0.92, 0.0, 1.10, -0.10, 0.08, "steel_dark", bevel=0.01)
    S.box("cabinet", 0.06, -0.94, 0.08, 1.12, -0.08, 0.58, "steel", bevel=0.035)
    S.box("top", 0.04, -0.96, 0.58, 1.14, -0.06, 0.63, "metal_dark", bevel=0.02)
    S.cyl("pit", 0.52, -0.56, 0.24, 0.63, 0.66, "backdrop", segs=24, bevel=0.0)
    for k in range(3):
        P.glow(S.cyl("coil%d" % k, 0.52, -0.56, 0.21 - 0.05 * k, 0.655 + 0.004 * k, 0.665 + 0.004 * k,
                     "copper", segs=24, bevel=0.0), "gland_ember")
    S.cyl("crucible", 0.52, -0.56, 0.07, 0.66, 0.72, "ceramic", segs=14, bevel=0.01)
    S.box("vent", 0.16, -0.955, 0.18, 0.96, -0.94, 0.46, "metal_dark", bevel=0.006)
    for k in range(5):
        z = 0.22 + 0.05 * k
        S.box("slat%d" % k, 0.18, -0.965, z, 0.94, -0.955, z + 0.02, "steel_dark", bevel=0.0)
    S.box("console", 0.10, -0.28, 0.63, 1.10, -0.08, 0.92, "steel_dark", bevel=0.025)
    P.glow(S.box("display", 0.62, -0.30, 0.92, 1.02, -0.14, 0.935, "backdrop", bevel=0.004), "indicator")
    for k, x in enumerate((0.22, 0.36)):
        S.cyl("dial%d" % k, x, -0.21, 0.04, 0.92, 0.96, "metal_dark", segs=10, bevel=0.005)
    S.box("cable", 1.10, -0.24, 0.10, 1.18, -0.16, 0.60, "fur_black", bevel=0.01)
    # anvil on a squat steel stand
    S.cyl("stand", 1.54, -0.60, 0.17, 0.0, 0.40, "steel_dark", segs=8, r_top=0.12, bevel=0.015)
    _anvil(1.52, -0.60, 0.40)
    S.box("ingot", 1.70, -0.32, 0.0, 1.88, -0.22, 0.05, "steel", bevel=0.01)
    return (2, 1), 0.96


def _machining_table():
    """2x1 machining table: a steel top on a cabinet, a lathe along the back
    (headstock, bed, tailstock), a drill press at the east end, a vise, gears
    and a power lamp (glow)."""
    _legs(0.12, 1.88, -0.18, -0.82, 0.50)
    S.box("drawers", 0.10, -0.86, 0.10, 0.80, -0.16, 0.48, "steel_dark", bevel=0.02)
    for k, z in enumerate((0.16, 0.32)):
        S.box("drawer%d" % k, 0.16, -0.875, z, 0.74, -0.86, z + 0.12, "steel", bevel=0.006)
    S.box("top", 0.04, -0.94, 0.50, 1.96, -0.06, 0.58, "steel", bevel=0.025)
    # lathe
    S.box("lathe_bed", 0.14, -0.40, 0.58, 1.26, -0.20, 0.66, "metal_dark", bevel=0.012)
    S.box("headstock", 0.12, -0.44, 0.58, 0.42, -0.14, 0.84, "armour_olive", bevel=0.025)
    P._rod("chuck", (0.42, -0.30, 0.75), (0.50, -0.30, 0.75), 0.07, "steel_dark", segs=14, bevel=0.006)
    P._rod("stock", (0.50, -0.30, 0.75), (0.92, -0.30, 0.75), 0.035, "brass", segs=12, bevel=0.0)
    S.box("tailstock", 0.96, -0.40, 0.66, 1.14, -0.20, 0.82, "armour_olive", bevel=0.02)
    P._rod("quill", (0.92, -0.30, 0.75), (0.96, -0.30, 0.75), 0.022, "steel", segs=8, bevel=0.0)
    # drill press
    S.box("press_base", 1.46, -0.44, 0.58, 1.86, -0.12, 0.62, "metal_dark", bevel=0.01)
    S.cyl("column", 1.76, -0.22, 0.04, 0.62, 1.18, "steel", segs=10, bevel=0.0)
    S.box("press_head", 1.48, -0.34, 0.98, 1.84, -0.12, 1.18, "armour_olive", bevel=0.025)
    S.cyl("spindle", 1.58, -0.30, 0.025, 0.80, 0.98, "steel_dark", segs=8, bevel=0.0)
    S.cyl("press_table", 1.60, -0.30, 0.10, 0.74, 0.77, "metal_dark", segs=16, bevel=0.005)
    P.glow(S.box("power_lamp", 1.60, -0.30, 1.18, 1.72, -0.18, 1.20, "backdrop", bevel=0.004), "indicator")
    # vise and loose parts on the front
    S.box("vise_base", 0.20, -0.84, 0.58, 0.50, -0.58, 0.66, "metal_dark", bevel=0.015)
    S.box("vise_jaw", 0.24, -0.90, 0.62, 0.46, -0.80, 0.74, "steel", bevel=0.015)
    for k, (x, y, r) in enumerate(((0.92, -0.70, 0.09), (1.14, -0.62, 0.06))):
        S.cyl("gear%d" % k, x, y, r, 0.58, 0.62, "brass", segs=12, bevel=0.006)
        S.cyl("gear_hub%d" % k, x, y, r * 0.35, 0.58, 0.63, "steel_dark", segs=8, bevel=0.0)
    S.box("file", 1.36, -0.82, 0.58, 1.76, -0.78, 0.60, "steel_dark", bevel=0.004)
    return (2, 1), 1.20


def _fabrication_bench():
    """3x2 fabrication bench: a long steel work surface, a back console with
    three screens (glow), an articulated arm over a dark mat, parts bins and
    a green circuit board. The worker stands at the middle of the south."""
    for i, x in enumerate((0.14, 1.50, 2.86)):
        for j, y in enumerate((-0.30, -1.80)):
            S.box("leg%d%d" % (i, j), x - 0.07, y - 0.07, 0.0, x + 0.07, y + 0.07, 0.52, "metal_dark", bevel=0.015)
    S.box("shelf", 0.10, -1.86, 0.14, 2.90, -0.24, 0.20, "steel_dark", bevel=0.015)
    for k, x in enumerate((0.30, 0.90, 2.10, 2.70)):
        S.box("crate%d" % k, x - 0.22, -1.50, 0.20, x + 0.22, -0.70, 0.42, "armour_olive" if k % 2 else "steel",
              bevel=0.02)
    S.box("top", 0.04, -1.94, 0.52, 2.96, -0.06, 0.60, "steel", bevel=0.03)
    S.box("console", 0.06, -0.62, 0.60, 2.94, -0.06, 1.10, "steel_dark", bevel=0.04)
    S.box("console_top", 0.04, -0.66, 1.10, 2.96, -0.04, 1.16, "metal_dark", bevel=0.02)
    for k, x in enumerate((0.30, 1.14, 1.98)):
        S.box("bezel%d" % k, x, -0.635, 0.70, x + 0.72, -0.60, 1.04, "metal_dark", bevel=0.012)
        P.glow(S.box("screen%d" % k, x + 0.05, -0.645, 0.75, x + 0.67, -0.63, 1.00, "backdrop", bevel=0.004),
               "indicator")
    for k, x in enumerate((0.40, 0.60, 2.40, 2.60)):
        P.glow(S.box("top_lamp%d" % k, x - 0.05, -0.40, 1.16, x + 0.05, -0.30, 1.175, "backdrop", bevel=0.003),
               "indicator")
    # work mat with a circuit board and an articulated arm reaching over it
    S.box("mat", 0.80, -1.70, 0.60, 2.20, -0.80, 0.615, "backdrop", bevel=0.004)
    S.box("board", 1.20, -1.40, 0.615, 1.70, -1.04, 0.64, "circuit", bevel=0.005)
    for k in range(3):
        S.box("chip%d" % k, 1.28 + 0.14 * k, -1.30, 0.64, 1.36 + 0.14 * k, -1.20, 0.665, "metal_dark", bevel=0.0)
    S.cyl("arm_base", 2.50, -1.00, 0.13, 0.60, 0.70, "metal_dark", segs=16, bevel=0.01)
    P._rod("arm_lo", (2.50, -1.00, 0.70), (2.30, -1.16, 1.12), 0.045, "corn_yellow", segs=10, bevel=0.006)
    S.ellipsoid("arm_joint", 2.30, -1.16, 1.12, 0.07, 0.07, 0.07, "metal_dark", segs=12, rings=8)
    P._rod("arm_hi", (2.30, -1.16, 1.12), (1.70, -1.24, 0.92), 0.04, "corn_yellow", segs=10, bevel=0.006)
    S.cyl("arm_tool", 1.66, -1.24, 0.035, 0.76, 0.94, "steel_dark", segs=10, r_top=0.05, bevel=0.0)
    # bins of parts at the west end of the top
    for k, (x, mat) in enumerate(((0.20, "copper"), (0.44, "steel"))):
        S.box("bin%d" % k, x, -1.60, 0.60, x + 0.20, -1.30, 0.70, "metal_dark", bevel=0.01)
        S.box("parts%d" % k, x + 0.02, -1.58, 0.68, x + 0.18, -1.32, 0.71, mat, bevel=0.0)
    S.box("cable", 2.90, -0.30, 0.10, 2.96, -0.20, 0.60, "fur_black", bevel=0.01)
    return (3, 2), 1.18


def _art_bench():
    """2x1 sculpting bench: a thick plank top, a turntable on a stand with a
    roughed-out block on it, a mallet, chisels and chips."""
    _legs(0.14, 1.86, -0.20, -0.80, 0.48, mat="wood", r=0.07)
    S.box("stretcher", 0.12, -0.56, 0.12, 1.88, -0.44, 0.18, "wood", bevel=0.015)
    S.box("top", 0.04, -0.94, 0.48, 1.96, -0.06, 0.60, "wood_plank", bevel=0.035, segments=2)
    S.cyl("turn_stand", 0.62, -0.52, 0.10, 0.60, 0.68, "metal_dark", segs=14, bevel=0.01)
    S.cyl("turntable", 0.62, -0.52, 0.30, 0.68, 0.72, "wood", segs=24, bevel=0.012)
    # the block, half carved: a squared base turning into a rounded head
    S.box("block", 0.44, -0.70, 0.72, 0.80, -0.34, 0.94, "stone_neutral", bevel=0.03)
    S.ellipsoid("roughed", 0.62, -0.52, 1.02, 0.15, 0.14, 0.12, "stone_neutral", segs=16, rings=10)
    S.box("cut", 0.70, -0.71, 0.80, 0.80, -0.62, 0.94, "stone_neutral_dark", bevel=0.01)
    # mallet and chisels laid on the east of the top
    S.box("mallet_head", 1.16, -0.74, 0.60, 1.36, -0.60, 0.72, "wood", bevel=0.02)
    P._rod("mallet_handle", (1.26, -0.60, 0.66), (1.30, -0.30, 0.64), 0.022, "bark", segs=8, bevel=0.0)
    for k, y in enumerate((-0.80, -0.72, -0.64)):
        P._rod("chisel%d" % k, (1.48, y, 0.62), (1.72, y + 0.02, 0.62), 0.012, "steel", segs=6, bevel=0.0)
        P._rod("chisel_grip%d" % k, (1.72, y + 0.02, 0.62), (1.84, y + 0.03, 0.62), 0.02, "wood", segs=6,
               bevel=0.0)
    S.box("rasp", 1.46, -0.40, 0.60, 1.84, -0.34, 0.62, "metal_dark", bevel=0.004)
    for k, (x, y) in enumerate(((0.30, -0.80), (0.98, -0.70), (0.40, -0.24), (0.92, -0.30))):
        S.blob("chip%d" % k, x, y, 0.61, 0.03, "stone_neutral", seed=70 + k, lumps=0.3, squash=0.5)
    return (2, 1), 1.14


def _electric_smelter():
    """2x1 electric smelter: a lined furnace box with a glowing mouth (glow),
    a heat hood, a crucible on a pouring frame, an ingot mould and a control
    panel (glow)."""
    S.box("plinth", 0.06, -0.94, 0.0, 1.20, -0.08, 0.08, "steel_dark", bevel=0.01)
    S.box("furnace", 0.08, -0.90, 0.08, 1.18, -0.10, 0.86, "steel", bevel=0.04, segments=2)
    S.box("lining", 0.24, -0.915, 0.30, 1.02, -0.88, 0.68, "stone_dark", bevel=0.02)
    P.glow(S.box("mouth", 0.32, -0.925, 0.38, 0.94, -0.90, 0.60, "ash_char", bevel=0.012), "gland_ember")
    S.box("hood", 0.04, -0.94, 0.86, 1.22, -0.06, 0.94, "metal_dark", bevel=0.02)
    P.glow(S.cyl("vent_glow", 0.63, -0.50, 0.16, 0.94, 0.955, "ash_char", segs=20, bevel=0.0), "gland_ember")
    S.cyl("vent_ring", 0.63, -0.50, 0.21, 0.94, 0.97, "steel_dark", segs=20, bevel=0.006)
    for k, x in enumerate((0.20, 1.06)):
        S.box("strap%d" % k, x - 0.03, -0.915, 0.08, x + 0.03, -0.895, 0.86, "metal_dark", bevel=0.0)
    # pouring frame with a tipped crucible, a mould of three ingots
    S.box("frame_w", 1.30, -0.70, 0.0, 1.36, -0.30, 0.56, "metal_dark", bevel=0.008)
    S.box("frame_e", 1.86, -0.70, 0.0, 1.92, -0.30, 0.56, "metal_dark", bevel=0.008)
    P._rod("axle", (1.36, -0.50, 0.50), (1.86, -0.50, 0.50), 0.02, "steel_dark", segs=8, bevel=0.0)
    S.cyl("crucible", 1.61, -0.50, 0.14, 0.38, 0.62, "ceramic", segs=16, r_top=0.16, bevel=0.012)
    S.cyl("melt", 1.61, -0.50, 0.12, 0.60, 0.615, "gold", segs=16, bevel=0.0)
    S.box("mould", 1.32, -0.96, 0.0, 1.94, -0.76, 0.10, "metal_dark", bevel=0.012)
    for k in range(3):
        x = 1.38 + 0.19 * k
        S.box("ingot%d" % k, x, -0.93, 0.08, x + 0.15, -0.79, 0.12, "steel", bevel=0.008)
    # control panel on the furnace's east cheek, its lamp on top
    S.box("panel", 1.00, -0.40, 0.94, 1.18, -0.14, 1.10, "steel_dark", bevel=0.012)
    P.glow(S.box("panel_lamp", 1.04, -0.36, 1.10, 1.14, -0.18, 1.115, "backdrop", bevel=0.003), "indicator")
    return (2, 1), 1.12


def _biofuel_refinery():
    """2x2 fuel refinery: a feed hopper at the west, two upright tanks joined
    by pipes, a gauge box (glow) and a red drum at the outlet. The worker
    stands south of the west column, at the hopper."""
    S.box("skid", 0.06, -1.94, 0.0, 1.94, -0.06, 0.08, "steel_dark", bevel=0.015)
    for k, x in enumerate((0.30, 1.70)):
        S.box("rail%d" % k, x - 0.06, -1.90, 0.08, x + 0.06, -0.10, 0.12, "metal_dark", bevel=0.0)
    # hopper: a funnel on legs over the feed screw
    for k, (x, y) in enumerate(((0.16, -1.10), (0.84, -1.10), (0.16, -1.76), (0.84, -1.76))):
        S.box("hleg%d" % k, x - 0.04, y - 0.04, 0.08, x + 0.04, y + 0.04, 0.70, "metal_dark", bevel=0.008)
    S.cyl("hopper", 0.50, -1.43, 0.16, 0.50, 1.02, "steel", segs=4, r_top=0.42, bevel=0.02)
    S.cyl("hopper_rim", 0.50, -1.43, 0.43, 1.02, 1.06, "metal_dark", segs=4, bevel=0.01)
    S.blob("feed", 0.50, -1.43, 1.00, 0.24, "vegetation", seed=31, lumps=0.25, squash=0.4)
    P._rod("screw", (0.50, -1.43, 0.40), (1.10, -1.30, 0.40), 0.06, "metal_dark", segs=12, bevel=0.006)
    # two tanks at the back, bands, domed tops, a bridge pipe
    for k, x in enumerate((0.62, 1.40)):
        S.cyl("tank%d" % k, x, -0.52, 0.34, 0.08, 1.20, "steel", segs=24, bevel=0.03)
        S.ellipsoid("dome%d" % k, x, -0.52, 1.20, 0.34, 0.34, 0.14, "steel", segs=24, rings=10)
        for j, z in enumerate((0.36, 0.84)):
            S.cyl("band%d%d" % (k, j), x, -0.52, 0.35, z, z + 0.05, "metal_dark", segs=24, bevel=0.0)
    P._rod("bridge", (0.62, -0.52, 1.30), (1.40, -0.52, 1.30), 0.045, "copper", segs=10, bevel=0.0)
    P._rod("drop", (1.40, -0.86, 0.60), (1.40, -1.30, 0.60), 0.04, "copper", segs=10, bevel=0.0)
    P._rod("feed_up", (1.10, -1.30, 0.40), (1.10, -0.86, 0.40), 0.05, "metal_dark", segs=10, bevel=0.0)
    # gauge box on the tanks' front, lamp on its top
    S.box("gauge_box", 0.86, -0.96, 0.60, 1.16, -0.82, 0.96, "steel_dark", bevel=0.015)
    S.cyl("gauge", 1.01, -0.97, 0.06, 0.74, 0.76, "ceramic", segs=12, bevel=0.0, ry=0.06)
    P.glow(S.box("gauge_lamp", 0.92, -0.94, 0.96, 1.10, -0.84, 0.975, "backdrop", bevel=0.003), "indicator")
    # outlet drum
    S.cyl("drum", 1.56, -1.52, 0.24, 0.08, 0.70, "food_red", segs=20, bevel=0.02)
    for j, z in enumerate((0.26, 0.52)):
        S.cyl("drum_band%d" % j, 1.56, -1.52, 0.245, z, z + 0.03, "metal_dark", segs=20, bevel=0.0)
    S.cyl("drum_cap", 1.62, -1.46, 0.05, 0.70, 0.73, "corn_yellow", segs=10, bevel=0.0)
    P._rod("spout", (1.40, -1.30, 0.60), (1.50, -1.44, 0.80), 0.03, "copper", segs=8, bevel=0.0)
    return (2, 2), 1.34


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
    "fueled_smithy": (_fueled_smithy, {"attachments": {"worker": [0.5, 1.15]}, "building": "wood-fired forge",
                                       "fuel": "wood"}),
    "electric_smithy": (_electric_smithy, {"attachments": {"worker": [0.5, 1.15]}, "building": "electric forge",
                                           "recipes": "wood-fired forge"}),
    "machining_table": (_machining_table, {"attachments": {"worker": [0.5, 1.15]}, "building": "machining table"}),
    "fabrication_bench": (_fabrication_bench, {"attachments": {"worker": [1.5, 2.15]},
                                               "building": "fabrication bench"}),
    "art_bench": (_art_bench, {"attachments": {"worker": [0.5, 1.15]}, "building": "sculpting bench"}),
    "electric_smelter": (_electric_smelter, {"attachments": {"worker": [0.5, 1.15]},
                                             "building": "electric smelter"}),
    "biofuel_refinery": (_biofuel_refinery, {"attachments": {"worker": [0.5, 2.15]}, "building": "fuel refinery"}),
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
