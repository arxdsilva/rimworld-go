"""Furniture family (ART spec §12-§14): bed, table, chair, workbench, stove,
butcher table, drying rack, campfire, lamp. (Sleeping spots are floor markings: overlay family.)

Geometry is authored facing SOUTH (the side a user approaches from /
the sleeper's feet point to) and rotated with stage.face().
"""
from .. import stage as S

FACINGS = ("south", "east", "north", "west")


def _bed():
    S.box("frame", 0.07, -1.96, 0.0, 0.93, -0.04, 0.26, "wood", bevel=0.04)
    S.box("headboard", 0.05, -0.17, 0.0, 0.95, -0.02, 0.58, "wood", bevel=0.045)
    S.box("post_l", 0.05, -1.96, 0.0, 0.17, -1.84, 0.34, "wood", bevel=0.03)
    S.box("post_r", 0.83, -1.96, 0.0, 0.95, -1.84, 0.34, "wood", bevel=0.03)
    S.box("mattress", 0.12, -1.88, 0.20, 0.88, -0.15, 0.36, "ceramic", bevel=0.06, segments=2)
    S.box("pillow", 0.20, -0.52, 0.34, 0.80, -0.21, 0.46, "ceramic", bevel=0.07, segments=2)
    S.box("blanket", 0.10, -1.90, 0.30, 0.90, -0.72, 0.43, "cloth", bevel=0.05, segments=2)
    S.box("fold", 0.10, -0.84, 0.34, 0.90, -0.64, 0.47, "cloth", bevel=0.06, segments=2)
    return (1, 2), 0.58


def _bed_double():
    """2x2 bed for two: one wide frame, two pillows, a shared blanket."""
    S.box("frame", 0.07, -1.96, 0.0, 1.93, -0.04, 0.26, "wood", bevel=0.04)
    S.box("headboard", 0.05, -0.17, 0.0, 1.95, -0.02, 0.62, "wood", bevel=0.045)
    S.box("crest", 0.70, -0.15, 0.58, 1.30, -0.04, 0.70, "wood", bevel=0.04)
    S.box("post_l", 0.05, -1.96, 0.0, 0.17, -1.84, 0.34, "wood", bevel=0.03)
    S.box("post_r", 1.83, -1.96, 0.0, 1.95, -1.84, 0.34, "wood", bevel=0.03)
    S.box("mattress", 0.12, -1.88, 0.20, 1.88, -0.15, 0.36, "ceramic", bevel=0.06, segments=2)
    for k, x in enumerate((0.18, 1.06)):
        S.box("pillow%d" % k, x, -0.52, 0.34, x + 0.76, -0.21, 0.46, "ceramic", bevel=0.07, segments=2)
    S.box("blanket", 0.10, -1.90, 0.30, 1.90, -0.72, 0.43, "cloth", bevel=0.05, segments=2)
    S.box("fold", 0.10, -0.84, 0.34, 1.90, -0.64, 0.47, "cloth", bevel=0.06, segments=2)
    return (2, 2), 0.70


def _table():
    for i, (x, y) in enumerate([(0.12, -0.12), (0.88, -0.12), (0.12, -0.88), (0.88, -0.88)]):
        S.box("leg%d" % i, x - 0.05, y - 0.05, 0.0, x + 0.05, y + 0.05, 0.50, "wood", bevel=0.015)
    S.box("apron", 0.10, -0.90, 0.40, 0.90, -0.10, 0.50, "wood", bevel=0.02)
    S.box("top", 0.04, -0.96, 0.50, 0.96, -0.04, 0.58, "wood_plank", bevel=0.03)
    S.box("seam", 0.49, -0.95, 0.575, 0.51, -0.05, 0.582, "wood", bevel=0.0)
    return (1, 1), 0.58


def _chair():
    for i, (x, y) in enumerate([(0.27, -0.30), (0.73, -0.30), (0.27, -0.74), (0.73, -0.74)]):
        S.box("leg%d" % i, x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.32, "wood", bevel=0.01)
    S.box("seat", 0.20, -0.80, 0.30, 0.80, -0.24, 0.38, "wood_plank", bevel=0.03)
    S.box("back", 0.20, -0.30, 0.38, 0.80, -0.20, 0.78, "wood", bevel=0.03)
    S.box("rail", 0.24, -0.29, 0.62, 0.76, -0.21, 0.72, "wood_plank", bevel=0.02)
    return (1, 1), 0.78


def _trough(full):
    """1x1 feeding trough: a long open wooden box on two trestle ends. Full
    shows a mounded fodder heap (straw with a few greens) over the rim."""
    for i, x in enumerate([0.22, 0.78]):
        S.box("foot%d" % i, x - 0.05, -0.80, 0.0, x + 0.05, -0.20, 0.10, "wood", bevel=0.015)
        S.box("leg%d" % i, x - 0.05, -0.66, 0.08, x + 0.05, -0.34, 0.30, "wood", bevel=0.015)
    S.box("floor", 0.10, -0.74, 0.28, 0.90, -0.26, 0.34, "wood_plank", bevel=0.02)
    S.box("side_s", 0.08, -0.78, 0.30, 0.92, -0.70, 0.52, "wood", bevel=0.02)
    S.box("side_n", 0.08, -0.30, 0.30, 0.92, -0.22, 0.52, "wood", bevel=0.02)
    S.box("end_w", 0.06, -0.78, 0.30, 0.14, -0.22, 0.52, "wood_plank", bevel=0.02)
    S.box("end_e", 0.86, -0.78, 0.30, 0.94, -0.22, 0.52, "wood_plank", bevel=0.02)
    if full:
        S.blob("fodder", 0.5, -0.5, 0.34, 0.27, "straw", seed=88, lumps=0.2, squash=0.5)
        S.blob("fodder2", 0.34, -0.46, 0.4, 0.2, "corn_yellow", seed=89, lumps=0.25, squash=0.5)
        S.blob("greens", 0.68, -0.52, 0.42, 0.17, "vegetation", seed=90, lumps=0.3, squash=0.55)
        return (1, 1), 0.7
    S.box("scrap", 0.30, -0.62, 0.34, 0.52, -0.52, 0.37, "straw", bevel=0.01)
    return (1, 1), 0.52


def _workbench():
    # 2 x 1, worker stands on the south side.
    for i, x in enumerate([0.12, 1.88]):
        for j, y in enumerate([-0.18, -0.82]):
            S.box("leg%d%d" % (i, j), x - 0.06, y - 0.06, 0.0, x + 0.06, y + 0.06, 0.52, "metal_dark", bevel=0.015)
    S.box("shelf", 0.10, -0.86, 0.14, 1.90, -0.14, 0.18, "wood", bevel=0.015)
    S.box("top", 0.04, -0.94, 0.52, 1.96, -0.06, 0.62, "steel", bevel=0.03)
    S.box("board", 0.06, -0.20, 0.62, 1.94, -0.06, 1.05, "wood_plank", bevel=0.03)
    S.box("vise_base", 0.20, -0.72, 0.62, 0.48, -0.42, 0.72, "metal_dark", bevel=0.02)
    S.box("vise_jaw", 0.24, -0.82, 0.66, 0.44, -0.70, 0.78, "steel", bevel=0.02)
    S.box("toolbox", 1.38, -0.62, 0.62, 1.80, -0.34, 0.80, "food_red", bevel=0.03)
    S.box("toolbox_lid", 1.36, -0.64, 0.80, 1.82, -0.32, 0.84, "metal_dark", bevel=0.015)
    S.cyl("hammer_handle", 0.95, -0.55, 0.03, 0.62, 0.66, "wood", segs=8, ry=0.25)
    S.box("hammer_head", 0.78, -0.64, 0.62, 0.86, -0.46, 0.70, "steel", bevel=0.015)
    for i, x in enumerate([0.70, 0.95, 1.20]):
        S.box("peg%d" % i, x - 0.025, -0.24, 0.80, x + 0.025, -0.19, 0.98, "metal_dark", bevel=0.005)
    S.box("light", 1.55, -0.20, 0.96, 1.70, -0.15, 1.02, "indicator", bevel=0.0)
    return (2, 1), 1.05


def _stonecutter_table():
    # 2 x 1, worker stands on the south side. Stone slab top, blocks on a shelf, a hand saw.
    for i, x in enumerate([0.14, 1.86]):
        for j, y in enumerate([-0.20, -0.80]):
            S.box("leg%d%d" % (i, j), x - 0.08, y - 0.08, 0.0, x + 0.08, y + 0.08, 0.50, "stone_neutral_dark", bevel=0.02)
    S.box("shelf", 0.10, -0.86, 0.12, 1.90, -0.14, 0.17, "wood", bevel=0.015)
    S.box("slab", 0.04, -0.94, 0.50, 1.96, -0.06, 0.64, "stone_neutral", bevel=0.03)
    S.box("back", 0.06, -0.18, 0.64, 1.94, -0.06, 0.92, "wood_plank", bevel=0.03)
    S.box("block_a", 0.22, -0.74, 0.64, 0.66, -0.34, 0.82, "stone_neutral_dark", bevel=0.025)
    S.box("block_b", 0.30, -0.70, 0.82, 0.58, -0.40, 0.92, "stone_neutral", bevel=0.02)
    S.box("saw_blade", 1.02, -0.70, 0.66, 1.62, -0.66, 0.78, "steel", bevel=0.005)
    S.box("saw_grip", 1.62, -0.72, 0.66, 1.76, -0.62, 0.78, "wood", bevel=0.01)
    S.box("chisel", 1.20, -0.46, 0.64, 1.56, -0.40, 0.68, "metal_dark", bevel=0.005)
    S.box("dust", 0.78, -0.62, 0.64, 0.98, -0.44, 0.67, "stone_neutral", bevel=0.01)
    return (2, 1), 0.92


def _stove():
    # 2 x 1, the cook stands on the south side.
    S.box("body", 0.06, -0.92, 0.0, 1.94, -0.10, 0.62, "metal_dark", bevel=0.04)
    S.box("top", 0.04, -0.94, 0.62, 1.96, -0.08, 0.68, "steel", bevel=0.03)
    for i, (cx, cy) in enumerate([(0.45, -0.48), (1.0, -0.48)]):
        S.cyl("ring%d" % i, cx, cy, 0.17, 0.68, 0.71, "metal_dark", segs=20, bevel=0.0)
        S.cyl("glow%d" % i, cx, cy, 0.10, 0.70, 0.715, "lamp_warm", segs=20, bevel=0.0)
    S.cyl("pot", 0.45, -0.48, 0.15, 0.71, 0.86, "steel", r_top=0.16, segs=18)
    S.cyl("stew", 0.45, -0.48, 0.13, 0.855, 0.865, "crop_fruit", segs=18, bevel=0.0)
    S.box("board", 1.40, -0.72, 0.68, 1.86, -0.26, 0.72, "wood_plank", bevel=0.015)
    S.ellipsoid("veg", 1.55, -0.45, 0.76, 0.06, 0.06, 0.05, "crop_fruit", segs=10, rings=6)
    S.box("door", 0.20, -0.935, 0.12, 1.00, -0.90, 0.48, "steel", bevel=0.015)
    S.box("handle", 0.30, -0.95, 0.40, 0.90, -0.93, 0.43, "metal_dark", bevel=0.005)
    S.box("pipe", 1.70, -0.20, 0.68, 1.84, -0.10, 1.10, "metal_dark", bevel=0.02)
    return (2, 1), 1.10


def _butcher_table():
    # 2 x 1, the butcher stands on the south side.
    for i, x in enumerate([0.14, 1.86]):
        for j, y in enumerate([-0.2, -0.8]):
            S.box("leg%d%d" % (i, j), x - 0.07, y - 0.07, 0.0, x + 0.07, y + 0.07, 0.5, "wood", bevel=0.02)
    S.box("stretcher", 0.12, -0.56, 0.12, 1.88, -0.44, 0.18, "wood", bevel=0.015)
    S.box("slab", 0.04, -0.94, 0.50, 1.96, -0.06, 0.66, "wood_plank", bevel=0.04, segments=2)
    S.box("block", 0.12, -0.86, 0.66, 0.86, -0.18, 0.74, "wood", bevel=0.03)
    for i, y in enumerate([-0.34, -0.52, -0.70]):
        S.box("groove%d" % i, 0.16, y - 0.008, 0.738, 0.82, y + 0.008, 0.744, "bark", bevel=0.0)
    S.box("blade", 1.05, -0.70, 0.66, 1.42, -0.48, 0.69, "steel", bevel=0.01)
    S.box("handle", 1.42, -0.62, 0.66, 1.62, -0.56, 0.70, "metal_dark", bevel=0.01)
    S.ellipsoid("cut", 1.62, -0.30, 0.71, 0.13, 0.09, 0.05, "food_red", segs=14, rings=8)
    S.ellipsoid("bone", 1.62, -0.30, 0.74, 0.05, 0.03, 0.02, "ceramic", segs=10, rings=6)
    S.box("rail", 0.06, -0.10, 0.66, 1.94, -0.04, 1.02, "wood", bevel=0.02)
    for i, x in enumerate([0.40, 0.80, 1.20]):
        S.cyl("hook%d" % i, x, -0.12, 0.012, 0.86, 1.0, "steel", segs=6, bevel=0.0)
    S.ellipsoid("hang", 1.20, -0.13, 0.78, 0.07, 0.04, 0.09, "food_red", segs=12, rings=8)
    return (2, 1), 1.02


def _drying_rack():
    """2 x 1 drying rack: two trestle ends carry a slatted tray at waist
    height, with a ridge pole and twine for hanging strips above it. The
    worker stands on the south side; the batch is drawn on the tray."""
    for i, x in enumerate([0.10, 1.90]):
        for j, (y0, y1) in enumerate([(-0.86, -0.50), (-0.14, -0.50)]):
            # splayed trestle legs meeting under the ridge
            S.box("leg%d%d" % (i, j), x - 0.05, min(y0, y1) + 0.02, 0.0, x + 0.05, max(y0, y1) - 0.02, 0.06,
                  "wood", bevel=0.01)
        S.box("post%d" % i, x - 0.05, -0.55, 0.0, x + 0.05, -0.45, 1.02, "wood", bevel=0.02)
        S.box("foot%d" % i, x - 0.07, -0.90, 0.0, x + 0.07, -0.10, 0.07, "wood", bevel=0.015)
    S.box("ridge", 0.04, -0.54, 0.98, 1.96, -0.46, 1.05, "wood", bevel=0.015)
    S.box("rail_n", 0.08, -0.16, 0.46, 1.92, -0.10, 0.52, "wood", bevel=0.01)
    S.box("rail_s", 0.08, -0.90, 0.46, 1.92, -0.84, 0.52, "wood", bevel=0.01)
    for k in range(9):
        x = 0.14 + k * 0.215
        S.box("slat%d" % k, x - 0.03, -0.88, 0.52, x + 0.03, -0.12, 0.55, "wood_plank", bevel=0.006)
    for k, y in enumerate([-0.70, -0.50, -0.30]):
        # woven cross-slats make the tray read as a mesh, not a fence
        S.box("weave%d" % k, 0.10, y - 0.025, 0.55, 1.90, y + 0.025, 0.575, "straw", bevel=0.004)
    for k, x in enumerate([0.45, 0.85, 1.25, 1.65]):
        S.cyl("twine%d" % k, x, -0.50, 0.008, 0.80, 0.98, "straw", segs=6, bevel=0.0)
        S.box("strip%d" % k, x - 0.035, -0.52, 0.66, x + 0.035, -0.48, 0.82, "food_red", bevel=0.01)
    return (2, 1), 1.05


def _tailoring_bench():
    """2 x 1 tailor's table: a cutting top with a spread hide, a bolt of
    cloth, shears and a spool. The tailor stands on the south side."""
    for i, x in enumerate([0.12, 1.88]):
        for j, y in enumerate([-0.18, -0.82]):
            S.box("leg%d%d" % (i, j), x - 0.05, y - 0.05, 0.0, x + 0.05, y + 0.05, 0.52, "wood", bevel=0.015)
    S.box("stretcher", 0.10, -0.55, 0.14, 1.90, -0.45, 0.19, "wood", bevel=0.01)
    S.box("top", 0.04, -0.94, 0.52, 1.96, -0.06, 0.60, "wood_plank", bevel=0.03)
    S.box("hide", 0.18, -0.80, 0.60, 0.98, -0.22, 0.615, "leather", bevel=0.01)
    S.cyl("bolt", 1.40, -0.30, 0.09, 0.60, 0.62, "cloth", segs=14, ry=0.09)
    S.box("bolt_body", 1.10, -0.39, 0.60, 1.70, -0.21, 0.70, "cloth", bevel=0.05)
    S.cyl("spool", 1.58, -0.70, 0.05, 0.60, 0.70, "wood", segs=12)
    S.cyl("thread", 1.58, -0.70, 0.045, 0.62, 0.68, "food_red", segs=12, bevel=0.0)
    S.box("shear_a", 1.10, -0.74, 0.60, 1.34, -0.71, 0.62, "steel", bevel=0.004)
    S.box("shear_b", 1.12, -0.66, 0.60, 1.34, -0.69, 0.62, "steel", bevel=0.004)
    S.cyl("ring", 1.36, -0.69, 0.03, 0.60, 0.625, "metal_dark", segs=10, bevel=0.0)
    return (2, 1), 0.72


def _horseshoe_pin():
    """1 x 1 horseshoe pitch: a raked sand patch, an iron stake and a few
    horseshoes thrown around it."""
    import math
    S.cyl("sand", 0.5, -0.5, 0.42, 0.0, 0.025, "straw", segs=24, bevel=0.0)
    S.cyl("stake", 0.5, -0.5, 0.04, 0.0, 0.34, "metal_dark", segs=10, bevel=0.01)
    S.cyl("hole", 0.5, -0.5, 0.10, 0.02, 0.03, "soil", segs=16, bevel=0.0)
    for k, (cx, cy, a) in enumerate([(0.30, -0.36, 0.4), (0.70, -0.66, 2.4), (0.50, -0.50, 1.2)]):
        z = 0.03 + (0.12 if k == 2 else 0.0)  # one ringer round the stake
        for seg in range(5):
            t = a + (seg / 4.0 - 0.5) * 4.4
            x, y = cx + 0.11 * math.cos(t), cy + 0.11 * math.sin(t)
            S.box("shoe%d_%d" % (k, seg), x - 0.04, y - 0.04, z, x + 0.04, y + 0.04, z + 0.035, "steel", bevel=0.012)
    return (1, 1), 0.32


def _research_bench():
    """2 x 1 research desk: a plank top on trestles, stacked books, an open
    journal, a glass flask on a stand and a candle. The researcher sits on
    the south side."""
    import math
    for i, x in enumerate([0.12, 1.88]):
        for j, y in enumerate([-0.18, -0.82]):
            S.box("leg%d%d" % (i, j), x - 0.05, y - 0.05, 0.0, x + 0.05, y + 0.05, 0.52, "wood", bevel=0.015)
    S.box("top", 0.04, -0.96, 0.52, 1.96, -0.04, 0.60, "wood_plank", bevel=0.03)
    S.box("shelf", 0.10, -0.90, 0.16, 1.90, -0.60, 0.20, "wood", bevel=0.015)
    for k, (h, mat) in enumerate([(0.06, "leather"), (0.05, "food_red"), (0.07, "bark")]):
        z = 0.60 + sum((0.06, 0.05, 0.07)[:k])
        S.box("book%d" % k, 0.16 + 0.01 * k, -0.86, z, 0.46 - 0.02 * k, -0.62, z + h, mat, bevel=0.012)
    S.box("journal_l", 0.70, -0.56, 0.60, 0.92, -0.26, 0.625, "ceramic", bevel=0.008)
    S.box("journal_r", 0.93, -0.56, 0.60, 1.15, -0.26, 0.625, "ceramic", bevel=0.008)
    S.box("spine", 0.915, -0.57, 0.595, 0.935, -0.25, 0.63, "leather", bevel=0.004)
    S.cyl("stand", 1.55, -0.70, 0.10, 0.60, 0.64, "metal_dark", segs=10)
    S.ellipsoid("flask", 1.55, -0.70, 0.74, 0.09, 0.09, 0.09, "glass", segs=14, rings=8)
    S.cyl("neck", 1.55, -0.70, 0.03, 0.80, 0.92, "glass", segs=8, bevel=0.0)
    S.cyl("candle", 1.78, -0.30, 0.035, 0.60, 0.72, "ceramic", segs=8)
    S.cyl("flame", 1.78, -0.30, 0.015, 0.72, 0.76, "lamp_warm", segs=6, r_top=0.002, bevel=0.0)
    S.cyl("quill", 1.20, -0.40, 0.008, 0.62, 0.64, "fur_cream", segs=4, bevel=0.0)
    return (2, 1), 0.92


def _sandbags():
    """1 x 1 waist-high bank of burlap sacks: two staggered courses of
    pillow-shaped bags, tied off at one end."""
    courses = [(0.0, (0.0, 0.50), 0.0), (0.15, (0.25,), -0.06), (0.15, (-0.25, 0.75), -0.06)]
    for row, (z, xs, dy) in enumerate(courses):
        for k, x0 in enumerate(xs):
            for depth, y in enumerate((-0.22, -0.56)):
                if row and depth:
                    continue
                a, b = max(0.0, x0 + 0.02), min(1.0, x0 + 0.48)
                if b - a < 0.2:
                    continue
                y0 = y + dy
                S.box("sack%d_%d_%d" % (row, k, depth), a, y0 - 0.30, z, b, y0, z + 0.17, "burlap",
                      bevel=0.075, segments=3)
                S.box("tie%d_%d_%d" % (row, k, depth), b - 0.05, y0 - 0.19, z + 0.05, b - 0.02, y0 - 0.11,
                      z + 0.13, "straw", bevel=0.01)
    return (1, 1), 0.34


def _spike_trap():
    """1 x 1 spike trap: a low timber frame in the dirt bristling with
    sharpened stakes leaning every which way."""
    import math
    S.box("pit", 0.08, -0.92, 0.0, 0.92, -0.08, 0.02, "soil", bevel=0.0)
    for i, (x0, y0, x1, y1) in enumerate([(0.08, -0.20, 0.92, -0.10), (0.08, -0.90, 0.92, -0.80),
                                          (0.08, -0.80, 0.18, -0.20), (0.82, -0.80, 0.92, -0.20)]):
        S.box("rail%d" % i, x0, y0, 0.0, x1, y1, 0.07, "wood", bevel=0.015)
    k = 0
    for gx in range(3):
        for gy in range(3):
            cx, cy = 0.28 + gx * 0.22, -0.30 - gy * 0.20
            lean = ((gx * 7 + gy * 3) % 5 - 2) * 6.0
            st = S.cyl("stake%d" % k, cx, cy, 0.045, 0.02, 0.44 + 0.05 * ((gx + gy) % 2), "wood",
                  r_top=0.004, segs=6, bevel=0.0)
            from mathutils import Matrix
            tilt = (Matrix.Translation((cx, cy, 0.02)) @ Matrix.Rotation(math.radians(lean), 4, "X")
                    @ Matrix.Rotation(math.radians(-lean * 0.7), 4, "Y") @ Matrix.Translation((-cx, -cy, -0.02)))
            S.transform([st], tilt)
            k += 1
    return (1, 1), 0.34


def _campfire():
    """1 x 1 fire pit: a ring of field stones around ash and a log-cabin
    stack of split logs. The flame itself is drawn live by the game."""
    import math
    S.cyl("ash", 0.5, -0.5, 0.30, 0.0, 0.03, "stone_dark", segs=20, bevel=0.0)
    for k in range(9):
        a = k / 9 * 2 * math.pi + 0.2
        r = 0.36 + 0.02 * ((k * 7) % 3)
        S.ellipsoid("stone%d" % k, 0.5 + r * math.cos(a), -0.5 + r * math.sin(a), 0.06,
                    0.085, 0.07, 0.07 + 0.01 * (k % 2), "stone" if k % 3 else "stone_dark", segs=10, rings=6)
    # two layers of logs, crosswise
    for i, x in enumerate([0.36, 0.64]):
        S.box("log_a%d" % i, x - 0.05, -0.76, 0.03, x + 0.05, -0.24, 0.12, "bark", bevel=0.03)
    for i, y in enumerate([-0.36, -0.64]):
        S.box("log_b%d" % i, 0.24, y - 0.05, 0.12, 0.76, y + 0.05, 0.21, "wood", bevel=0.03)
    S.ellipsoid("cinders", 0.5, -0.5, 0.05, 0.14, 0.14, 0.04, "stone_dark", segs=14, rings=6)
    return (1, 1), 0.30


def _lamp():
    S.cyl("base", 0.5, -0.5, 0.22, 0.0, 0.06, "metal_dark", segs=20)
    S.cyl("pole", 0.5, -0.5, 0.035, 0.06, 1.25, "steel", segs=10)
    S.cyl("shade", 0.5, -0.5, 0.26, 1.18, 1.38, "metal_dark", r_top=0.16, segs=20)
    S.cyl("bulb", 0.5, -0.5, 0.15, 1.38, 1.40, "lamp_warm", segs=20, bevel=0.0)
    return (1, 1), 1.40


# ------------------------------------------------------------ burial (W6)
def _log(name, p0, p1, r, mat, end_mat="wood", z=None):
    """A round log from p0 to p1 (x,y,z each), lying free, end grain lit."""
    import math
    from mathutils import Matrix, Vector
    a, b = Vector(p0), Vector(p1)
    d = b - a
    ln = d.length
    M = Matrix.Translation(a) @ d.to_track_quat("Z", "Y").to_matrix().to_4x4()
    S.cyl(name, 0, 0, r, 0.0, ln, mat, segs=10, bevel=0.012)
    S.objects()[-1].matrix_world = M
    if end_mat:
        S.cyl(name + "_end", 0, 0, r * 0.82, ln, ln + 0.012, end_mat, segs=10, bevel=0.0)
        S.objects()[-1].matrix_world = M
        S.cyl(name + "_beg", 0, 0, r * 0.82, -0.012, 0.0, end_mat, segs=10, bevel=0.0)
        S.objects()[-1].matrix_world = M


def _grass_edge(cx, cy, rx, ry, n, seed=0):
    """Soft tufts ringed around an ellipse."""
    import math
    for k in range(n):
        a = k / n * 2 * math.pi + 0.3 * seed
        j = 1.0 + 0.10 * (((k * 5 + seed) % 4) - 1.5)
        S.ellipsoid("tuft", cx + rx * j * math.cos(a), cy + ry * j * math.sin(a), 0.02,
                    0.075 + 0.015 * (k % 3), 0.06 + 0.01 * (k % 2), 0.07 + 0.015 * ((k + seed) % 3),
                    "foliage" if k % 3 else "vegetation", segs=8, rings=5)


def _grave_dug():
    """1 x 2 (head north): an open pit ringed by raised earth, a heap at the
    foot with a shovel standing in it."""
    import math
    from mathutils import Matrix
    # raised rim
    S.box("rim_n", 0.2, -0.28, 0.0, 0.8, -0.14, 0.09, "soil", bevel=0.04)
    S.box("rim_w", 0.16, -1.52, 0.0, 0.30, -0.20, 0.09, "soil", bevel=0.04)
    S.box("rim_e", 0.70, -1.52, 0.0, 0.84, -0.20, 0.09, "soil", bevel=0.04)
    S.box("rim_s", 0.2, -1.60, 0.0, 0.8, -1.46, 0.09, "soil", bevel=0.04)
    # the hole: dark floor inside the rim
    S.box("floor", 0.29, -1.47, 0.0, 0.71, -0.29, 0.012, "soil_rich", bevel=0.0)
    S.box("floor_deep", 0.34, -1.42, 0.0, 0.66, -0.34, 0.014, "soil_rich", bevel=0.0)
    S.box("wall_lip", 0.29, -1.47, 0.0, 0.34, -0.29, 0.06, "soil_rich", bevel=0.0)
    # the heap
    S.ellipsoid("heap", 0.5, -1.78, 0.0, 0.34, 0.20, 0.26, "soil", segs=18, rings=10)
    S.ellipsoid("heap_hi", 0.43, -1.74, 0.12, 0.17, 0.10, 0.14, "soil", segs=14, rings=8)
    for k, (cx, cy, r) in enumerate([(0.2, -1.62, 0.05), (0.78, -1.64, 0.045), (0.64, -1.92, 0.05)]):
        S.ellipsoid("clod", cx, cy, 0.03, r, r * 0.8, r * 0.7, "soil_rich", segs=8, rings=5)
    # shovel stuck blade-down in the heap, leaning toward the viewer-right
    base = (0.52, -1.78, 0.16)
    S.cyl("shaft", 0, 0, 0.03, 0.0, 0.80, "wood", r_top=0.026, segs=8, bevel=0.0)
    S.box("d_grip", -0.09, -0.04, 0.78, 0.09, 0.04, 0.86, "wood", bevel=0.015)
    S.box("blade", -0.10, -0.02, -0.22, 0.10, 0.02, 0.0, "steel", bevel=0.008)
    for ob in S.objects()[-3:]:
        ob.matrix_world = (Matrix.Translation(base) @ Matrix.Rotation(math.radians(-12), 4, "Y")
                           @ Matrix.Rotation(math.radians(6), 4, "X")) @ ob.matrix_world
    _grass_edge(0.5, -1.0, 0.46, 0.92, 14)
    return (1, 2), 1.02


def _grave_filled():
    """1 x 2 (head north): a mounded plot of settled earth edged with grass."""
    S.ellipsoid("mound_base", 0.5, -1.0, 0.0, 0.36, 0.78, 0.17, "soil", segs=24, rings=10)
    S.ellipsoid("mound_top", 0.47, -1.0, 0.06, 0.25, 0.60, 0.15, "soil", segs=22, rings=10)
    S.ellipsoid("mound_hi", 0.44, -0.86, 0.12, 0.11, 0.30, 0.08, "soil", segs=14, rings=8)
    for k, (cx, cy, r) in enumerate([(0.62, -1.4, 0.04), (0.38, -0.6, 0.035), (0.6, -0.75, 0.03)]):
        S.ellipsoid("clod", cx, cy, 0.1, r, r * 0.8, r * 0.7, "soil_rich", segs=8, rings=5)
    _grass_edge(0.5, -1.0, 0.40, 0.82, 16, seed=1)
    _grass_edge(0.5, -1.0, 0.30, 0.70, 10, seed=3)
    return (1, 2), 0.34


def _stone_cap(name, cx, y, w, z0, z1, mat):
    """A tall slab with a rounded top (half cylinder lying along Y)."""
    import math
    from mathutils import Matrix
    S.box(name + "_slab", cx - w / 2, y - 0.045, z0, cx + w / 2, y + 0.045, z1 - w / 2, mat, bevel=0.015)
    S.cyl(name + "_arch", 0, 0, w / 2, -0.045, 0.045, mat, segs=16, bevel=0.012)
    S.objects()[-1].matrix_world = Matrix.Translation((cx, y, z1 - w / 2)) @ Matrix.Rotation(math.radians(-90), 4, "X")


def _headstone_00():
    S.ellipsoid("moss", 0.5, -0.5, 0.0, 0.2, 0.1, 0.03, "vegetation", segs=12, rings=5)
    S.box("plinth", 0.30, -0.58, 0.0, 0.70, -0.42, 0.06, "stone_dark", bevel=0.015)
    _stone_cap("rounded", 0.5, -0.5, 0.30, 0.05, 0.50, "stone")
    S.box("etch", 0.45, -0.552, 0.27, 0.55, -0.545, 0.285, "stone_dark", bevel=0.0)
    S.box("etch2", 0.42, -0.552, 0.22, 0.58, -0.545, 0.235, "stone_dark", bevel=0.0)
    return (1, 1), 0.50


def _headstone_01():
    S.box("plinth", 0.32, -0.58, 0.0, 0.68, -0.42, 0.06, "stone_dark", bevel=0.015)
    S.box("post", 0.455, -0.55, 0.05, 0.545, -0.45, 0.62, "stone", bevel=0.02)
    S.box("arm", 0.32, -0.55, 0.38, 0.68, -0.45, 0.46, "stone", bevel=0.02)
    S.ellipsoid("moss", 0.54, -0.5, 0.05, 0.07, 0.06, 0.03, "vegetation", segs=8, rings=5)
    return (1, 1), 0.62


def _headstone_02():
    import math
    from mathutils import Matrix
    S.ellipsoid("mound", 0.5, -0.5, 0.0, 0.17, 0.12, 0.04, "soil", segs=12, rings=5)
    S.cyl("stake", 0.5, -0.5, 0.032, 0.0, 0.42, "wood", r_top=0.03, segs=8, bevel=0.0)
    S.box("board", 0.30, -0.535, 0.28, 0.70, -0.465, 0.40, "wood_plank", bevel=0.015)
    S.box("tie", 0.485, -0.54, 0.12, 0.515, -0.46, 0.20, "leather", bevel=0.006)
    S.box("scratch", 0.38, -0.54, 0.335, 0.62, -0.535, 0.345, "bark", bevel=0.0)
    return (1, 1), 0.42


def _headstone_03():
    S.box("slab", 0.20, -0.66, 0.0, 0.80, -0.34, 0.07, "stone", bevel=0.02)
    S.box("slab_top", 0.24, -0.62, 0.065, 0.76, -0.38, 0.09, "stone_dark", bevel=0.01)
    S.ellipsoid("leaves", 0.5, -0.5, 0.10, 0.20, 0.10, 0.04, "foliage", segs=12, rings=5)
    for k, (fx, fy, m, h) in enumerate([(0.38, -0.5, "food_red", 0.20), (0.50, -0.46, "blossom", 0.24),
                                        (0.60, -0.52, "crop_fruit", 0.21), (0.46, -0.56, "blossom", 0.18),
                                        (0.55, -0.42, "food_red", 0.17)]):
        S.cyl("stem", fx, fy, 0.008, 0.08, h, "vegetation", segs=5, bevel=0.0)
        S.ellipsoid("bloom", fx, fy, h + 0.01, 0.036, 0.036, 0.03, m, segs=8, rings=5)
    return (1, 1), 0.30


def _pyre():
    """2 x 1 funeral pyre: a crossed stack of logs on a bed of kindling."""
    S.ellipsoid("bed", 1.0, -0.5, 0.0, 0.88, 0.42, 0.015, "straw", segs=22, rings=6)
    # bottom layer runs north-south, then east-west, then north-south again
    for i, x in enumerate((0.34, 0.62, 0.90, 1.18, 1.46, 1.70)):
        _log("l0_%d" % i, (x, -0.92, 0.09), (x, -0.08, 0.09), 0.085, "bark")
    for i, y in enumerate((-0.30, -0.52, -0.74)):
        _log("l1_%d" % i, (0.22, y, 0.26), (1.78, y, 0.26), 0.08, "wood" if i == 1 else "bark")
    for i, x in enumerate((0.55, 1.0, 1.45)):
        _log("l2_%d" % i, (x, -0.84, 0.41), (x, -0.16, 0.41), 0.07, "bark")
    _log("l3_0", (0.45, -0.40, 0.54), (1.55, -0.40, 0.54), 0.06, "wood")
    _log("l3_1", (0.45, -0.60, 0.54), (1.55, -0.60, 0.54), 0.06, "bark")
    # kindling: straw bundles and thin sticks poking out
    for k, (x, y, z) in enumerate([(0.2, -0.12, 0.05), (1.82, -0.86, 0.06), (0.12, -0.80, 0.05), (1.9, -0.2, 0.05)]):
        S.ellipsoid("bundle", x, y, z, 0.11, 0.07, 0.05, "straw", segs=10, rings=5)
    for k in range(6):
        x = 0.35 + 0.25 * k
        _log("stick%d" % k, (x, -0.08, 0.20), (x + 0.12, 0.08, 0.26), 0.014, "bark", end_mat=None)
    return (2, 1), 0.64


def _pyre_burning():
    """The same stack burnt down: charred, cracked logs and glowing embers.
    The renderer draws the flames."""
    S.ellipsoid("ash", 1.0, -0.5, 0.0, 0.92, 0.46, 0.02, "stone_dark", segs=22, rings=6)
    for i, x in enumerate((0.34, 0.62, 0.90, 1.18, 1.46, 1.70)):
        _log("l0_%d" % i, (x, -0.92, 0.08), (x, -0.08, 0.08), 0.082, "fur_dark", end_mat="stone_dark")
    for i, y in enumerate((-0.30, -0.52, -0.74)):
        _log("l1_%d" % i, (0.24, y, 0.24), (1.76, y, 0.24), 0.078, "fur_dark", end_mat="metal_dark")
    for i, x in enumerate((0.55, 1.0, 1.45)):
        _log("l2_%d" % i, (x, -0.82, 0.38), (x, -0.18, 0.38), 0.066, "fur_dark", end_mat="stone_dark")
    _log("l3_0", (0.5, -0.42, 0.5), (1.5, -0.42, 0.5), 0.055, "metal_dark", end_mat="stone_dark")
    # glowing cracks and embers between and on the logs
    for k, (x, y) in enumerate([(0.48, -0.30), (0.76, -0.6), (1.04, -0.38), (1.32, -0.7), (1.60, -0.45),
                                (0.9, -0.78), (1.2, -0.2), (0.62, -0.5)]):
        S.ellipsoid("ember", x, y, 0.5 if k % 2 else 0.33, 0.09, 0.05, 0.03, "lamp_warm" if k % 3 else "pumpkin",
                    segs=8, rings=5)
    for k, x in enumerate((0.3, 0.8, 1.3, 1.75)):
        S.ellipsoid("coal", x, -0.12 - 0.04 * (k % 2), 0.1, 0.08, 0.05, 0.04, "food_red", segs=8, rings=5)
    return (2, 1), 0.60


# --- carpets (ART 16.3): flat 1x1 floor mats, ~0.8 tile wide, no facing ----
CP0, CP1 = 0.10, 0.90          # mat extent on each axis (x; y is -CP1..-CP0)
CP_EDGE = 0.03                 # 2 px darker border at game scale


def _carpet_straw():
    """Woven straw mat: a basket-weave checker of two straw tones."""
    S.box("edge", CP0, -CP1, 0.0, CP1, -CP0, 0.03, "carpet_straw_edge", bevel=0.008)
    n = 8
    lo, hi = CP0 + CP_EDGE, CP1 - CP_EDGE
    step = (hi - lo) / n
    for i in range(n):
        for j in range(n):
            x0, y0 = lo + i * step, -hi + j * step
            horiz = (i + j) % 2 == 0
            # each cell holds three strips; orientation alternates like a weave
            for k in range(3):
                f0, f1 = (k + 0.08) / 3, (k + 0.92) / 3
                if horiz:
                    S.box("w", x0 + 0.004, y0 + f0 * step, 0.022, x0 + step - 0.004, y0 + f1 * step, 0.04,
                          "carpet_straw" if k != 1 else "carpet_straw_dark", bevel=0.004)
                else:
                    S.box("w", x0 + f0 * step, y0 + 0.004, 0.026, x0 + f1 * step, y0 + step - 0.004, 0.044,
                          "carpet_straw_dark" if k != 1 else "carpet_straw", bevel=0.004)
    return (1, 1), 0.06


def _carpet_cloth():
    """Dyed cloth rug: thick pile, a cream border stripe and fringe."""
    S.box("edge", CP0, -CP1, 0.0, CP1, -CP0, 0.03, "carpet_cloth_edge", bevel=0.01)
    e = CP_EDGE
    S.box("pile", CP0 + e, -CP1 + e, 0.02, CP1 - e, -CP0 - e, 0.05, "carpet_cloth", bevel=0.012, segments=2)
    # border stripe, inset from the edge
    a, b, w = CP0 + 0.08, CP1 - 0.08, 0.035
    z0, z1 = 0.045, 0.056
    S.box("stripe_n", a, -CP0 - 0.08 - w, z0, b, -CP0 - 0.08, z1, "carpet_cloth_stripe", bevel=0.004)
    S.box("stripe_s", a, -CP1 + 0.08, z0, b, -CP1 + 0.08 + w, z1, "carpet_cloth_stripe", bevel=0.004)
    S.box("stripe_w", a, -CP1 + 0.08, z0, a + w, -CP0 - 0.08, z1, "carpet_cloth_stripe", bevel=0.004)
    S.box("stripe_e", b - w, -CP1 + 0.08, z0, b, -CP0 - 0.08, z1, "carpet_cloth_stripe", bevel=0.004)
    # centre lozenge
    S.prism("lozenge", [(0.5, -0.34), (0.62, -0.5), (0.5, -0.66), (0.38, -0.5)], 0.045, 0.056,
            "carpet_cloth_stripe", bevel=0.004)
    # fringe tassels on the short ends
    for k in range(9):
        x = CP0 + 0.04 + k * 0.088
        S.box("fr_n", x, -CP0, 0.0, x + 0.03, -CP0 + 0.05, 0.022, "carpet_cloth_stripe", bevel=0.004)
        S.box("fr_s", x, -CP1 - 0.05, 0.0, x + 0.03, -CP1, 0.022, "carpet_cloth_stripe", bevel=0.004)
    return (1, 1), 0.06


def _carpet_leather():
    """Stitched brown hide: slightly ragged outline, a dark rim, stitching."""
    import math
    outer = [(0.10, -0.22), (0.16, -0.11), (0.34, -0.09), (0.50, -0.12), (0.68, -0.09), (0.85, -0.12),
             (0.91, -0.28), (0.89, -0.50), (0.92, -0.72), (0.86, -0.88), (0.66, -0.91), (0.48, -0.88),
             (0.30, -0.91), (0.14, -0.87), (0.09, -0.70), (0.11, -0.46)]
    cx, cy = 0.5, -0.5
    inner = [(cx + (x - cx) * 0.93, cy + (y - cy) * 0.93) for x, y in outer]
    S.prism("rim", outer, 0.0, 0.03, "carpet_leather_edge", bevel=0.008)
    S.prism("hide", inner, 0.0, 0.042, "carpet_leather", bevel=0.01)
    # patches of darker, worn hide
    S.blob("patch0", 0.34, -0.36, 0.036, 0.12, "carpet_leather_edge", seed=31, lumps=0.2, squash=0.07)
    S.blob("patch1", 0.66, -0.64, 0.036, 0.10, "carpet_leather_edge", seed=32, lumps=0.2, squash=0.07)
    # stitching: short light dashes around the inside of the rim
    pts = [(cx + (x - cx) * 0.84, cy + (y - cy) * 0.84) for x, y in outer]
    for i in range(len(pts)):
        (x0, y0), (x1, y1) = pts[i], pts[(i + 1) % len(pts)]
        L = math.hypot(x1 - x0, y1 - y0)
        for k in range(max(2, int(L / 0.07))):
            t = (k + 0.5) / max(2, int(L / 0.07))
            sx, sy = x0 + (x1 - x0) * t, y0 + (y1 - y0) * t
            S.box("st", sx - 0.012, sy - 0.012, 0.04, sx + 0.012, sy + 0.012, 0.05, "straw", bevel=0.002)
    # a centre seam where two hides are joined
    S.box("seam", 0.20, -0.515, 0.04, 0.80, -0.485, 0.048, "carpet_leather_edge", bevel=0.002)
    for k in range(10):
        x = 0.22 + k * 0.062
        S.box("sx", x, -0.532, 0.045, x + 0.02, -0.468, 0.052, "straw", bevel=0.002)
    return (1, 1), 0.06


def _carpet_wood():
    """Wooden slat mat: six slats with dark gaps, two cross battens."""
    S.box("base", CP0, -CP1, 0.0, CP1, -CP0, 0.02, "carpet_wood_edge", bevel=0.006)
    for y, in ((-0.72,), (-0.28,)):
        S.box("batten", CP0, y - 0.035, 0.015, CP1, y + 0.035, 0.03, "carpet_wood_edge", bevel=0.004)
    n = 6
    gap = 0.014
    w = (CP1 - CP0 - gap * (n + 1)) / n
    for i in range(n):
        x0 = CP0 + gap + i * (w + gap)
        S.box("slat%d" % i, x0, -CP1 + CP_EDGE * 0.5, 0.02, x0 + w, -CP0 - CP_EDGE * 0.5, 0.05,
              "carpet_wood" if i % 2 == 0 else "carpet_wood_alt", bevel=0.01)
        # grain line and a nail head per batten
        S.box("grain%d" % i, x0 + w * 0.45, -0.84, 0.048, x0 + w * 0.55, -0.55, 0.054, "carpet_wood_edge", bevel=0.0)
        for y in (-0.72, -0.28):
            S.box("nail%d" % i, x0 + w * 0.4, y - 0.012, 0.048, x0 + w * 0.6, y + 0.012, 0.056, "metal_dark",
                  bevel=0.002)
    return (1, 1), 0.06


def _carpet_wool():
    """Thick woollen rug: cream pile, two dark stripes, fringe at both ends."""
    S.box("base", CP0, -CP1, 0.0, CP1, -CP0, 0.04, "carpet_wool_stripe", bevel=0.01)
    S.box("pile", CP0 + CP_EDGE, -CP1 + CP_EDGE, 0.03, CP1 - CP_EDGE, -CP0 - CP_EDGE, 0.06, "carpet_wool",
          bevel=0.015, segments=2)
    for k, x in enumerate((0.26, 0.74)):
        S.box("stripe%d" % k, x - 0.03, -CP1 + CP_EDGE, 0.058, x + 0.03, -CP0 - CP_EDGE, 0.066,
              "carpet_wool_stripe", bevel=0.004)
    for k in range(10):
        y = -0.88 + k * 0.08
        for sx in (-1, 1):
            xe = CP0 if sx < 0 else CP1
            S.box("fr%d%d" % (k, sx), xe - 0.05 if sx < 0 else xe, y, 0.0, xe if sx < 0 else xe + 0.05, y + 0.035,
                  0.03, "wool_cream", bevel=0.006)
    return (1, 1), 0.07


BUILDERS = {
    "bed_basic": (_bed, {"attachments": {"sleeper": [0.5, 0.42]}}),
    "bed_double": (_bed_double, {"attachments": {"sleeper": [0.5, 0.42], "sleeper2": [1.5, 0.42]}}),
    "table_small": (_table, {}),
    "chair_basic": (_chair, {"attachments": {"seat": [0.5, 0.52]}}),
    "workbench_basic": (_workbench, {"attachments": {"worker": [0.5, 1.15]}}),
    "lamp_standing": (_lamp, {"emissive": "lamp_warm", "light_radius": 6}),
    "stonecutter_table": (_stonecutter_table, {"attachments": {"worker": [0.5, 1.15]}}),
    "stove_basic": (_stove, {"attachments": {"cook": [0.25, 1.5]}}),
    "butcher_table": (_butcher_table, {"attachments": {"worker": [0.5, 1.15]}}),
    "campfire": (_campfire, {}),
    "feeding_trough_empty": (lambda: _trough(False), {"building": "feeding trough", "state": "empty"}),
    "feeding_trough_full": (lambda: _trough(True), {"building": "feeding trough", "state": "full"}),
    "horseshoe_pin": (_horseshoe_pin, {}),
    "sandbags": (_sandbags, {}),
    "research_bench": (_research_bench, {"attachments": {"worker": [0.5, 1.15]}}),
    "spike_trap": (_spike_trap, {}),
    "tailoring_bench": (_tailoring_bench, {"attachments": {"worker": [0.5, 1.15]}}),
    "drying_rack": (_drying_rack, {"attachments": {"worker": [0.5, 1.15]}}),
    "grave_dug": (_grave_dug, {"building": "grave", "state": "dug"}),
    "grave_filled": (_grave_filled, {"building": "grave", "state": "filled"}),
    "headstone_00": (_headstone_00, {"item": "headstone", "style": "rounded"}),
    "headstone_01": (_headstone_01, {"item": "headstone", "style": "cross"}),
    "headstone_02": (_headstone_02, {"item": "headstone", "style": "wooden"}),
    "headstone_03": (_headstone_03, {"item": "headstone", "style": "slab"}),
    "pyre": (_pyre, {"building": "funeral pyre", "state": "stacked"}),
    "pyre_burning": (_pyre_burning, {"building": "funeral pyre", "state": "burning"}),
    "carpet_straw": (_carpet_straw, {"carpet": "straw", "flat": True}),
    "carpet_cloth": (_carpet_cloth, {"carpet": "cloth", "flat": True}),
    "carpet_leather": (_carpet_leather, {"carpet": "leather", "flat": True}),
    "carpet_wood": (_carpet_wood, {"carpet": "wood", "flat": True}),
    "carpet_wool": (_carpet_wool, {"carpet": "wool", "flat": True}),
}
FACINGS_ONLY = {"grave_dug": ("south", "east"), "grave_filled": ("south", "east")}
ROTATABLE = {"grave_dug", "grave_filled", "bed_basic", "bed_double", "chair_basic", "workbench_basic", "stonecutter_table", "stove_basic", "butcher_table",
             "drying_rack", "tailoring_bench", "research_bench", "feeding_trough_empty", "feeding_trough_full"}


# Blanket overlays: drawn over a sleeper so they read as tucked in. The
# rest of the bed is a holdout, so the overlay lines up pixel for pixel.
BLANKETS = {"bed_basic": (_bed, {"blanket", "fold", "post_l", "post_r"}),
            "bed_double": (_bed_double, {"blanket", "fold", "post_l", "post_r"})}


def _blankets():
    out = []
    for name, (fn, keep) in BLANKETS.items():
        for f in FACINGS:
            fp, zmax = fn()
            hold = [o for o in S.objects() if o.name.split(".")[0] not in keep]
            S.holdout(hold)
            fp = S.face(S.objects(), f, fp)
            out.append(S.render_asset("furniture", "%s_blanket_%s" % (name, f), footprint=fp, zmax=zmax,
                                      shadow=False, extra={"overlay": "blanket", "facing": f}))
    return out


def build():
    out = _blankets()
    for name, (fn, extra) in BUILDERS.items():
        facings = FACINGS_ONLY.get(name) or (FACINGS if name in ROTATABLE else ("south",))
        for f in facings:
            fp, zmax = fn()
            fp = S.face(S.objects(), f, fp)
            ex = dict(extra, rotatable=name in ROTATABLE, facing=f)
            suffix = "_" + f if name in ROTATABLE else ""
            # flat floor mats lie on the ground: no sun shadow
            out.append(S.render_asset("furniture", name + suffix, footprint=fp, zmax=zmax, extra=ex,
                                      shadow_kind="none" if extra.get("flat") else None))
    return out
