"""Furniture family (ART spec §12-§14): bed, table, chair, workbench, stove,
butcher table, drying rack, campfire, lamp. (Sleeping spots are floor markings: overlay family.)
PLAY-021 adds the torch lamp and the electric stove, tailoring and research
benches; their emissive parts get glow layers (families/power.py).

Geometry is authored facing SOUTH (the side a user approaches from /
the sleeper's feet point to) and rotated with stage.face().
"""
from .. import stage as S
from . import power as P

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


def _tool_cabinet():
    """1x1 workshop tool cabinet: a tall steel locker with a pegboard top rack,
    an orange stripe, two doors and a wrench and mallet resting on top."""
    import math
    from mathutils import Matrix
    for sx in (-1, 1):
        S.box("foot%d" % sx, 0.5 + sx * 0.28 - 0.05, -0.74, 0.0, 0.5 + sx * 0.28 + 0.05, -0.64, 0.06, "metal_dark",
              bevel=0.01)
    S.box("body", 0.14, -0.80, 0.05, 0.86, -0.22, 0.80, "pod_hull", bevel=0.035, segments=2)
    S.box("top", 0.12, -0.82, 0.78, 0.88, -0.20, 0.84, "steel", bevel=0.02)
    S.box("seam", 0.495, -0.815, 0.09, 0.505, -0.79, 0.76, "metal_dark", bevel=0.0)
    S.box("stripe", 0.15, -0.815, 0.56, 0.85, -0.79, 0.70, "pod_paint", bevel=0.0)
    for sx in (-1, 1):
        S.box("handle%d" % sx, 0.5 + sx * 0.07 - 0.012, -0.835, 0.30, 0.5 + sx * 0.07 + 0.012, -0.80, 0.46, "metal_dark",
              bevel=0.004)
        S.box("vent%d" % sx, 0.5 + sx * 0.2 - 0.09, -0.805, 0.10, 0.5 + sx * 0.2 + 0.09, -0.795, 0.13, "metal_dark",
              bevel=0.0)
    # wrench lying on top, angled; mallet beside it
    n0 = len(S.objects())
    S.box("wrench", 0.24, -0.58, 0.84, 0.62, -0.52, 0.875, "metal_dark", bevel=0.01)
    S.cyl("wr_ring", 0.64, -0.55, 0.055, 0.84, 0.875, "metal_dark", segs=10, bevel=0.005)
    m = Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(24), 4, "Z") @ Matrix.Translation((-0.5, 0.5, 0))
    for ob in S.objects()[n0:]:
        ob.matrix_world = m @ ob.matrix_world
    S.box("mal_h", 0.30, -0.36, 0.84, 0.70, -0.31, 0.875, "wood", bevel=0.008)
    S.box("mal_head", 0.66, -0.43, 0.84, 0.78, -0.25, 0.94, "metal_dark", bevel=0.012)
    return (1, 1), 0.94


def _hospital_bed():
    """1x2 hospital bed: steel frame on casters, white mattress, teal sheet, raised
    side rails, a bed-head monitor and an IV stand. Authored facing south."""
    for sx in (-1, 1):
        for y in (-0.22, -1.78):
            S.cyl("caster", 0.5 + sx * 0.36, y, 0.045, 0.0, 0.07, "metal_dark", segs=10, bevel=0.005)
    S.box("frame", 0.09, -1.92, 0.07, 0.91, -0.08, 0.24, "steel", bevel=0.03)
    S.box("mattress", 0.13, -1.88, 0.22, 0.87, -0.12, 0.36, "med_white", bevel=0.05, segments=2)
    S.box("sheet", 0.12, -1.90, 0.30, 0.88, -0.78, 0.42, "sheet_teal", bevel=0.045, segments=2)
    S.box("fold", 0.12, -0.92, 0.34, 0.88, -0.74, 0.45, "sheet_teal", bevel=0.05)
    S.box("pillow", 0.22, -0.46, 0.34, 0.78, -0.16, 0.45, "med_white", bevel=0.07, segments=2)
    S.box("headboard", 0.08, -0.14, 0.07, 0.92, -0.06, 0.60, "steel", bevel=0.03)
    S.box("footboard", 0.10, -1.94, 0.07, 0.90, -1.88, 0.40, "steel", bevel=0.02)
    for sx in (-1, 1):
        x = 0.5 + sx * 0.405
        S.box("rail%d" % sx, x - 0.018, -1.42, 0.36, x + 0.018, -0.50, 0.40, "steel", bevel=0.006)
        for y in (-1.42, -0.50):
            S.box("rail_post", x - 0.025, y - 0.025, 0.24, x + 0.025, y + 0.025, 0.40, "steel", bevel=0.005)
    # monitor on the headboard, with a lit trace
    S.box("monitor", 0.60, -0.14, 0.60, 0.88, -0.04, 0.82, "metal_dark", bevel=0.015)
    S.box("screen", 0.63, -0.16, 0.65, 0.85, -0.14, 0.79, "indicator", bevel=0.0)
    # IV stand at the head corner: pole, crossbar, bag
    S.cyl("iv_base", 0.97, -0.32, 0.05, 0.0, 0.03, "metal_dark", segs=10, bevel=0.004)
    S.cyl("iv_pole", 0.97, -0.32, 0.014, 0.03, 0.98, "steel", segs=8, bevel=0.0)
    S.box("iv_arm", 0.88, -0.335, 0.94, 1.06, -0.305, 0.97, "steel", bevel=0.004)
    S.ellipsoid("iv_bag", 0.88, -0.32, 0.84, 0.045, 0.03, 0.09, "med_blue", segs=10, rings=6)
    return (1, 2), 0.98


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


def _bench_legs(mat, r=0.06, h=0.52, x0=0.12, x1=2.88, y0=-0.18, y1=-0.82, bevel=0.015):
    """Four corner legs of a 3 x 1 bench (STK-AR)."""
    for i, x in enumerate((x0, x1)):
        for j, y in enumerate((y0, y1)):
            S.box("leg%d%d" % (i, j), x - r, y - r, 0.0, x + r, y + r, h, mat, bevel=bevel)


def _workbench():
    # 3 x 1 (STK-AR): the worker stands south of the middle cell, where a
    # plank sits clamped in the vise. Toolbox west, hammer and saw east.
    _bench_legs("metal_dark")
    S.box("shelf", 0.10, -0.86, 0.14, 2.90, -0.14, 0.18, "wood", bevel=0.015)
    S.box("crate", 0.24, -0.76, 0.18, 0.80, -0.28, 0.44, "wood_plank", bevel=0.02)
    S.box("top", 0.04, -0.94, 0.52, 2.96, -0.06, 0.62, "steel", bevel=0.03)
    S.box("board", 0.06, -0.20, 0.62, 2.94, -0.06, 1.05, "wood_plank", bevel=0.03)
    # middle: the vise and its workpiece
    S.box("vise_base", 1.32, -0.62, 0.62, 1.68, -0.36, 0.72, "metal_dark", bevel=0.02)
    S.box("vise_jaw", 1.34, -0.84, 0.66, 1.66, -0.70, 0.80, "steel", bevel=0.02)
    S.box("vise_jaw_b", 1.34, -0.66, 0.66, 1.66, -0.58, 0.80, "steel", bevel=0.015)
    S.box("workpiece", 1.06, -0.70, 0.74, 1.94, -0.62, 0.84, "wood_plank", bevel=0.012)
    S.cyl("vise_screw", 1.50, -0.90, 0.025, 0.68, 0.76, "metal_dark", segs=8, ry=0.07, bevel=0.0)
    for k, (x, y) in enumerate(((1.10, -0.82), (1.88, -0.44), (1.22, -0.36))):
        S.blob("shaving%d" % k, x, y, 0.625, 0.035, "straw", seed=20 + k, lumps=0.3, squash=0.4)
    # west: the toolbox
    S.box("toolbox", 0.24, -0.66, 0.62, 0.74, -0.34, 0.80, "food_red", bevel=0.03)
    S.box("toolbox_lid", 0.22, -0.68, 0.80, 0.76, -0.32, 0.84, "metal_dark", bevel=0.015)
    S.box("toolbox_grip", 0.40, -0.52, 0.84, 0.58, -0.48, 0.88, "metal_dark", bevel=0.005)
    # east: hammer and hand saw
    S.cyl("hammer_handle", 2.40, -0.70, 0.03, 0.62, 0.66, "wood", segs=8, ry=0.22)
    S.box("hammer_head", 2.24, -0.80, 0.62, 2.32, -0.62, 0.70, "steel", bevel=0.015)
    S.box("saw_blade", 2.24, -0.44, 0.62, 2.74, -0.32, 0.64, "steel", bevel=0.004)
    S.box("saw_grip", 2.74, -0.46, 0.62, 2.86, -0.30, 0.68, "wood", bevel=0.01)
    # pegboard tools and the lamp over the middle
    for i, x in enumerate((0.40, 0.70, 1.00, 2.00, 2.30, 2.60)):
        S.box("peg%d" % i, x - 0.025, -0.24, 0.80, x + 0.025, -0.19, 0.98, "metal_dark", bevel=0.005)
    S.box("wrench", 0.66, -0.235, 0.70, 0.74, -0.205, 0.92, "steel", bevel=0.006)
    S.box("square", 2.24, -0.235, 0.74, 2.36, -0.205, 0.96, "brass", bevel=0.004)
    S.box("light", 1.40, -0.20, 0.96, 1.60, -0.15, 1.02, "indicator", bevel=0.0)
    return (3, 1), 1.05


def _stonecutter_table():
    # 3 x 1 (STK-AR): stone slab top. Middle: a block under the saw;
    # west: cut blocks stacked; east: chisel, mallet and dust.
    _bench_legs("stone_neutral_dark", r=0.08, h=0.50, x0=0.14, x1=2.86, y0=-0.20, y1=-0.80, bevel=0.02)
    S.box("shelf", 0.10, -0.86, 0.12, 2.90, -0.14, 0.17, "wood", bevel=0.015)
    S.box("slab", 0.04, -0.94, 0.50, 2.96, -0.06, 0.64, "stone_neutral", bevel=0.03)
    S.box("back", 0.06, -0.18, 0.64, 2.94, -0.06, 0.92, "wood_plank", bevel=0.03)
    # middle: the block being cut, the saw half way through it
    S.box("block", 1.18, -0.74, 0.64, 1.82, -0.32, 0.84, "stone_neutral_dark", bevel=0.025)
    S.box("kerf", 1.49, -0.75, 0.70, 1.51, -0.31, 0.845, "stone_neutral", bevel=0.0)
    S.box("saw_blade", 1.46, -0.92, 0.80, 1.54, -0.30, 0.96, "steel", bevel=0.005)
    S.box("saw_grip", 1.44, -1.02, 0.84, 1.56, -0.90, 0.98, "wood", bevel=0.01)
    S.box("dust_mid", 1.06, -0.86, 0.64, 1.34, -0.70, 0.67, "stone_neutral", bevel=0.01)
    # west: finished blocks
    S.box("block_a", 0.20, -0.74, 0.64, 0.66, -0.34, 0.80, "stone_neutral_dark", bevel=0.025)
    S.box("block_b", 0.26, -0.70, 0.80, 0.60, -0.40, 0.92, "stone_neutral", bevel=0.02)
    S.box("block_c", 0.72, -0.60, 0.64, 0.92, -0.36, 0.76, "stone_neutral", bevel=0.02)
    # east: chisel, mallet, dust
    S.box("chisel", 2.10, -0.50, 0.64, 2.52, -0.44, 0.68, "metal_dark", bevel=0.005)
    S.box("mallet_head", 2.20, -0.82, 0.64, 2.44, -0.66, 0.76, "wood", bevel=0.02)
    S.cyl("mallet_handle", 2.60, -0.74, 0.022, 0.68, 0.72, "bark", segs=8, ry=0.18, bevel=0.0)
    S.box("dust", 2.52, -0.62, 0.64, 2.80, -0.36, 0.67, "stone_neutral", bevel=0.01)
    return (3, 1), 0.98


def _stove():
    # 3 x 1 (STK-AR), the cook stands south of the middle cell: two burners
    # and the oven door there; the firebox and flue west, a prep board east.
    S.box("body", 0.06, -0.92, 0.0, 2.94, -0.10, 0.62, "metal_dark", bevel=0.04)
    S.box("top", 0.04, -0.94, 0.62, 2.96, -0.08, 0.68, "steel", bevel=0.03)
    for i, cx in enumerate((1.24, 1.76)):
        S.cyl("ring%d" % i, cx, -0.50, 0.17, 0.68, 0.71, "metal_dark", segs=20, bevel=0.0)
        S.cyl("glow%d" % i, cx, -0.50, 0.10, 0.70, 0.715, "lamp_warm", segs=20, bevel=0.0)
    S.cyl("pot", 1.24, -0.50, 0.15, 0.71, 0.86, "steel", r_top=0.16, segs=18)
    S.cyl("stew", 1.24, -0.50, 0.13, 0.855, 0.865, "crop_fruit", segs=18, bevel=0.0)
    S.cyl("pan", 1.76, -0.50, 0.14, 0.71, 0.76, "metal_dark", segs=18, bevel=0.01)
    S.box("pan_handle", 1.90, -0.53, 0.73, 2.12, -0.47, 0.76, "metal_dark", bevel=0.005)
    S.box("door", 1.08, -0.935, 0.12, 1.92, -0.90, 0.48, "steel", bevel=0.015)
    S.box("handle", 1.18, -0.95, 0.40, 1.82, -0.93, 0.43, "metal_dark", bevel=0.005)
    # west: the firebox with logs and its flue
    S.box("firedoor", 0.20, -0.935, 0.10, 0.80, -0.90, 0.42, "steel_dark", bevel=0.015)
    S.box("grate", 0.30, -0.95, 0.18, 0.70, -0.93, 0.26, "lamp_warm", bevel=0.0)
    for k, y in enumerate((-0.74, -0.58)):
        S.cyl("log%d" % k, 0.46, y, 0.06, 0.68, 0.74, "bark", segs=10, ry=0.06, bevel=0.0)
        S.box("log_body%d" % k, 0.20, y - 0.06, 0.68, 0.72, y + 0.06, 0.78, "bark", bevel=0.05)
    S.box("pipe", 0.30, -0.24, 0.68, 0.48, -0.10, 1.10, "metal_dark", bevel=0.02)
    S.box("pipe_cap", 0.26, -0.28, 1.06, 0.52, -0.06, 1.12, "steel_dark", bevel=0.015)
    # east: the prep board with a vegetable and a knife
    S.box("board", 2.20, -0.78, 0.68, 2.84, -0.30, 0.72, "wood_plank", bevel=0.015)
    S.ellipsoid("veg", 2.40, -0.54, 0.76, 0.07, 0.07, 0.05, "crop_fruit", segs=10, rings=6)
    S.box("knife", 2.56, -0.66, 0.72, 2.80, -0.62, 0.735, "steel", bevel=0.003)
    return (3, 1), 1.12


def _brewing_bench():
    # 3 x 1 (STK-AR), the brewer stands south of the middle cell: the mash
    # tub there, two banded barrels west, a paddle, mugs and a sack east.
    _bench_legs("bark", h=0.50)
    S.box("shelf", 0.10, -0.86, 0.12, 2.90, -0.14, 0.16, "wood", bevel=0.015)
    S.box("top", 0.04, -0.94, 0.50, 2.96, -0.06, 0.58, "wood_plank", bevel=0.03)
    for k, cx in enumerate((0.30, 0.72)):
        S.cyl("barrel%d" % k, cx, -0.36, 0.17, 0.58, 0.98, "wood", r_top=0.17, segs=16, bevel=0.03)
        for z in (0.66, 0.90):
            S.cyl("hoop%d%d" % (k, int(z * 10)), cx, -0.36, 0.178, z, z + 0.05, "metal_dark", segs=16, bevel=0.0)
        S.cyl("lid%d" % k, cx, -0.36, 0.15, 0.98, 1.0, "wood_plank", segs=16, bevel=0.0)
    S.cyl("spigot", 0.51, -0.70, 0.03, 0.66, 0.72, "steel", segs=8, ry=0.03, bevel=0.0)
    S.cyl("jug", 0.50, -0.78, 0.07, 0.58, 0.72, "ceramic", segs=12, r_top=0.05, bevel=0.01)
    # middle: the mash tub
    S.cyl("tub", 1.50, -0.54, 0.32, 0.58, 0.84, "bark", r_top=0.35, segs=22, bevel=0.025)
    S.cyl("tub_rim", 1.50, -0.54, 0.355, 0.78, 0.82, "metal_dark", segs=22, bevel=0.0)
    S.cyl("mash", 1.50, -0.54, 0.31, 0.82, 0.87, "crop_fruit", segs=22, bevel=0.0)
    S.ellipsoid("foam", 1.58, -0.48, 0.88, 0.08, 0.07, 0.02, "snow", segs=10, rings=5)
    S.box("paddle", 1.62, -0.66, 0.84, 1.70, -0.58, 1.14, "wood", bevel=0.01)
    # east: mugs, a grain sack
    for k, (x, y) in enumerate(((2.16, -0.78), (2.34, -0.70))):
        S.cyl("mug%d" % k, x, y, 0.055, 0.58, 0.68, "ceramic", segs=12, bevel=0.01)
    S.blob("sack", 2.60, -0.42, 0.72, 0.22, "straw", seed=61, lumps=0.15, squash=0.8)
    S.cyl("sack_tie", 2.60, -0.42, 0.06, 0.86, 0.92, "bark", segs=10, bevel=0.0)
    S.ellipsoid("grain", 2.36, -0.40, 0.60, 0.08, 0.06, 0.02, "corn_yellow", segs=10, rings=5)
    return (3, 1), 1.14


def _drug_table():
    # 3 x 1 (STK-AR), the worker stands south of the middle cell: mortar,
    # pestle and a tray there; jars west, leaf bundles drying on a rail east.
    _bench_legs("wood", h=0.52, x0=0.14, x1=2.86, y0=-0.20, y1=-0.80)
    S.box("top", 0.04, -0.94, 0.52, 2.96, -0.06, 0.60, "wood_plank", bevel=0.03)
    S.box("brace", 0.10, -0.86, 0.16, 2.90, -0.14, 0.20, "bark", bevel=0.015)
    # middle: mortar, pestle and the tray of ground leaf
    S.cyl("mortar", 1.36, -0.55, 0.17, 0.60, 0.76, "stone_dark", r_top=0.2, segs=18, bevel=0.025)
    S.cyl("paste", 1.36, -0.55, 0.15, 0.755, 0.77, "vegetation", segs=18, bevel=0.0)
    S.cyl("pestle", 1.44, -0.50, 0.03, 0.70, 1.02, "stone", segs=8, r_top=0.045, bevel=0.01, ry=0.03)
    S.box("tray", 1.62, -0.84, 0.60, 1.94, -0.42, 0.64, "wood", bevel=0.01)
    S.ellipsoid("pile", 1.78, -0.63, 0.66, 0.12, 0.15, 0.04, "leaf_light", segs=12, rings=6)
    # west: jars and a folded paper
    for k, (x, y, mat) in enumerate(((0.30, -0.40, "glass"), (0.56, -0.36, "glass"), (0.40, -0.66, "ceramic"))):
        S.cyl("jar%d" % k, x, y, 0.08, 0.60, 0.76, mat, segs=12, bevel=0.01)
        S.cyl("jar_lid%d" % k, x, y, 0.085, 0.76, 0.80, "wood", segs=12, bevel=0.005)
    S.box("paper", 0.66, -0.84, 0.60, 0.92, -0.62, 0.605, "ceramic", bevel=0.0)
    # east: drying rail with leaf bundles
    S.box("rail_l", 2.08, -0.24, 0.60, 2.14, -0.18, 1.10, "bark", bevel=0.01)
    S.box("rail_r", 2.86, -0.24, 0.60, 2.92, -0.18, 1.10, "bark", bevel=0.01)
    S.box("rail", 2.08, -0.24, 1.04, 2.92, -0.18, 1.10, "bark", bevel=0.01)
    for k, x in enumerate((2.22, 2.42, 2.62, 2.80)):
        S.box("twine%d" % k, x - 0.01, -0.225, 0.90, x + 0.01, -0.195, 1.04, "straw", bevel=0.0)
        S.ellipsoid("leaves%d" % k, x, -0.21, 0.82, 0.07, 0.04, 0.12, "leaf_light" if k % 2 else "vegetation",
                    segs=10, rings=6)
    S.ellipsoid("loose", 2.50, -0.66, 0.62, 0.16, 0.10, 0.03, "vegetation", segs=12, rings=6)
    return (3, 1), 1.10


def _butcher_table():
    import math
    from mathutils import Matrix
    from .item import _steak
    # 3 x 1 (STK-AR), the butcher stands south of the middle cell, at the
    # chopping block; a bucket west, cut meat east, a hook rail at the back.
    _bench_legs("wood", r=0.07, h=0.5, x0=0.14, x1=2.86, y0=-0.2, y1=-0.8, bevel=0.02)
    S.box("stretcher", 0.12, -0.56, 0.12, 2.88, -0.44, 0.18, "wood", bevel=0.015)
    S.box("slab", 0.04, -0.94, 0.50, 2.96, -0.06, 0.66, "wood_plank", bevel=0.04, segments=2)
    # middle: the block, its grooves, the cleaver and the steak just cut
    S.box("block", 1.08, -0.88, 0.66, 1.92, -0.20, 0.74, "wood", bevel=0.03)
    for i, y in enumerate((-0.36, -0.54, -0.72)):
        S.box("groove%d" % i, 1.12, y - 0.008, 0.738, 1.88, y + 0.008, 0.744, "bark", bevel=0.0)
    _steak("joint", 1.36, -0.54, 0.74, 10, "skin_red", "meat_fat", "meat_fat", size=0.85,
           bone=("ceramic", "food_red"), thick=0.07)
    S.box("blade", 1.56, -0.72, 0.74, 1.80, -0.50, 0.77, "steel", bevel=0.01)
    S.box("blade_grip", 1.80, -0.64, 0.74, 1.96, -0.58, 0.78, "metal_dark", bevel=0.01)
    # west: a pail
    S.cyl("pail", 0.44, -0.58, 0.17, 0.66, 0.90, "wood", r_top=0.19, segs=16, bevel=0.02)
    S.cyl("pail_band", 0.44, -0.58, 0.18, 0.80, 0.84, "metal_dark", segs=16, bevel=0.0)
    S.cyl("pail_in", 0.44, -0.58, 0.16, 0.86, 0.885, "food_red", segs=16, bevel=0.0)
    # east: steaks laid out and a knife
    for k, (x, y, yaw) in enumerate(((2.26, -0.67, 12), (2.64, -0.62, -14))):
        _steak("cut%d" % k, x, y, 0.66, yaw, "skin_red", "meat_fat", "meat_fat", pattern=k + 1, size=0.72, thick=0.06)
    S.box("knife", 2.20, -0.40, 0.66, 2.54, -0.36, 0.68, "steel", bevel=0.004)
    S.box("knife_grip", 2.54, -0.41, 0.66, 2.66, -0.35, 0.70, "metal_dark", bevel=0.005)
    # back rail with hooks
    S.box("rail", 0.06, -0.10, 0.66, 2.94, -0.04, 1.02, "wood", bevel=0.02)
    for i, x in enumerate((0.50, 1.00, 1.50, 2.00, 2.50)):
        S.cyl("hook%d" % i, x, -0.12, 0.012, 0.86, 1.0, "steel", segs=6, bevel=0.0)
    # steaks on two of the hooks, tipped towards the viewer (hung dead flat
    # against the rail, the camera would see them as a sliver)
    for k, x in enumerate((1.00, 2.50)):
        n0 = len(S.objects())
        _steak("hang%d" % k, 0.0, 0.0, 0.0, 90, "skin_red", "meat_fat", "meat_fat", pattern=k, size=0.5, thick=0.04)
        S.transform(S.objects()[n0:], Matrix.Translation((x, -0.2, 0.8)) @ Matrix.Rotation(math.radians(55), 4, "X"))
    return (3, 1), 1.02


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
    """3 x 1 tailor's table (STK-AR): the hide spread on the middle cell
    under the shears, a bolt of cloth west, spool, pins and a folded shirt
    east. The tailor stands south of the middle."""
    _bench_legs("wood", r=0.05)
    S.box("stretcher", 0.10, -0.55, 0.14, 2.90, -0.45, 0.19, "wood", bevel=0.01)
    S.box("top", 0.04, -0.94, 0.52, 2.96, -0.06, 0.60, "wood_plank", bevel=0.03)
    # middle: hide with a chalk line and the shears on it
    S.box("hide", 1.06, -0.86, 0.60, 1.94, -0.22, 0.615, "leather", bevel=0.01)
    S.box("chalk", 1.16, -0.56, 0.615, 1.84, -0.545, 0.618, "ceramic", bevel=0.0)
    S.box("shear_a", 1.30, -0.74, 0.615, 1.60, -0.71, 0.635, "steel", bevel=0.004)
    S.box("shear_b", 1.32, -0.66, 0.615, 1.60, -0.69, 0.635, "steel", bevel=0.004)
    S.cyl("ring", 1.62, -0.69, 0.03, 0.615, 0.64, "metal_dark", segs=10, bevel=0.0)
    # west: a bolt of cloth
    S.cyl("bolt", 0.50, -0.34, 0.09, 0.60, 0.62, "cloth", segs=14, ry=0.09)
    S.box("bolt_body", 0.16, -0.43, 0.60, 0.84, -0.25, 0.70, "cloth", bevel=0.05)
    S.box("bolt_tail", 0.30, -0.80, 0.60, 0.86, -0.43, 0.61, "cloth", bevel=0.005)
    # east: spool, pin cushion, folded shirt
    S.cyl("spool", 2.20, -0.70, 0.05, 0.60, 0.70, "wood", segs=12)
    S.cyl("thread", 2.20, -0.70, 0.045, 0.62, 0.68, "food_red", segs=12, bevel=0.0)
    S.ellipsoid("cushion", 2.40, -0.78, 0.64, 0.07, 0.07, 0.04, "food_red", segs=12, rings=6)
    S.box("folded", 2.36, -0.50, 0.60, 2.84, -0.20, 0.66, "cloth", bevel=0.03)
    S.box("folded_top", 2.40, -0.48, 0.66, 2.80, -0.22, 0.70, "cloth", bevel=0.03)
    return (3, 1), 0.72


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
    """3 x 2 research desk (RSCH-3): a broad plank table on six trestle legs,
    a book rack along the back, an open journal with quill and ink pot,
    a book stack, a rolled chart, a seedling in a clay pot, a flask on its
    stand and a candle. The researcher sits on the south side."""
    for i, x in enumerate([0.12, 1.50, 2.88]):
        for j, y in enumerate([-0.14, -1.86]):
            S.box("leg%d%d" % (i, j), x - 0.055, y - 0.055, 0.0, x + 0.055, y + 0.055, 0.52, "wood", bevel=0.015)
    S.box("stretcher", 0.10, -1.05, 0.14, 2.90, -0.95, 0.20, "wood", bevel=0.015)
    S.box("top", 0.04, -1.96, 0.52, 2.96, -0.04, 0.60, "wood_plank", bevel=0.03)
    for k, x in enumerate((0.99, 2.01)):
        S.box("seam%d" % k, x - 0.01, -1.95, 0.595, x + 0.01, -0.05, 0.602, "wood", bevel=0.0)
    # open book rack on the back edge: end boards and a back rail, a row of
    # standing books whose coloured tops read from above, one lying flat
    for k, x in enumerate((0.08, 1.46, 2.84)):
        S.box("rack_end%d" % k, x, -0.46, 0.60, x + 0.08, -0.06, 0.88, "wood", bevel=0.015)
    S.box("rack_back", 0.08, -0.12, 0.60, 2.92, -0.06, 0.92, "wood", bevel=0.012)
    mats = ("leather", "food_red", "bark", "jacket_navy", "sheet_teal", "pumpkin", "leather", "mug_blue", "bark")
    k = 0
    for bay, (x0, x1) in enumerate(((0.18, 1.42), (1.56, 2.24))):
        x = x0
        while x < x1 - 0.07:
            w = 0.07 + 0.02 * ((k * 7) % 3)
            h = 0.18 + 0.03 * ((k * 5) % 3)
            if k % 7 == 4:   # a gap in the row
                x += 0.12
                k += 1
                continue
            S.box("book%d" % k, x, -0.42 + 0.02 * (k % 2), 0.60, min(x + w, x1), -0.14, 0.60 + h,
                  mats[k % len(mats)], bevel=0.01)
            x += w + 0.01
            k += 1
    S.box("tome", 2.30, -0.44, 0.60, 2.74, -0.16, 0.66, "leather", bevel=0.012)
    # loose notes on the desk
    S.box("note0", 0.98, -1.80, 0.60, 1.16, -1.58, 0.605, "ceramic", bevel=0.0)
    S.box("note1", 2.14, -1.82, 0.60, 2.36, -1.62, 0.605, "ceramic", bevel=0.0)
    # work surface: open journal, quill, ink pot
    S.box("journal_l", 1.18, -1.62, 0.60, 1.48, -1.18, 0.625, "ceramic", bevel=0.008)
    S.box("journal_r", 1.50, -1.62, 0.60, 1.80, -1.18, 0.625, "ceramic", bevel=0.008)
    S.box("spine", 1.48, -1.63, 0.595, 1.50, -1.17, 0.632, "leather", bevel=0.004)
    for k in range(4):
        y = -1.28 - 0.07 * k
        S.box("ink_line%d" % k, 1.22, y - 0.008, 0.625, 1.42 - 0.05 * (k % 2), y + 0.008, 0.628, "metal_dark",
              bevel=0.0)
    S.cyl("ink_pot", 1.98, -1.30, 0.05, 0.60, 0.68, "glass", segs=12, bevel=0.005)
    S.cyl("ink", 1.98, -1.30, 0.035, 0.68, 0.69, "jacket_navy", segs=10, bevel=0.0)
    S.box("quill", 1.86, -1.48, 0.625, 2.14, -1.46, 0.64, "fur_cream", bevel=0.004)
    # left: stacked books and a rolled chart
    for k, (h, mat) in enumerate([(0.07, "leather"), (0.05, "food_red"), (0.06, "bark")]):
        z = 0.60 + sum((0.07, 0.05, 0.06)[:k])
        S.box("stack%d" % k, 0.22 + 0.015 * k, -1.06, z, 0.60 - 0.02 * k, -0.74, z + h, mat, bevel=0.012)
    S.box("chart", 0.20, -1.62, 0.60, 0.88, -1.48, 0.68, "ceramic", bevel=0.038, segments=3)
    S.box("chart_tie", 0.52, -1.63, 0.60, 0.56, -1.47, 0.685, "leather", bevel=0.01)
    # right: seedling in a pot, a flask on its stand, a candle
    S.cyl("pot", 2.30, -0.80, 0.09, 0.60, 0.72, "ceramic", r_top=0.11, segs=14)
    S.cyl("pot_soil", 2.30, -0.80, 0.095, 0.715, 0.725, "soil", segs=14, bevel=0.0)
    for k, (dx, dy, s) in enumerate(((0.0, 0.0, 0.06), (-0.05, 0.03, 0.045), (0.05, -0.02, 0.05))):
        S.ellipsoid("leaf%d" % k, 2.30 + dx, -0.80 + dy, 0.80 + 0.03 * k, s, s, s * 0.7, "vegetation", segs=10, rings=6)
    S.cyl("stand", 2.66, -1.10, 0.10, 0.60, 0.64, "metal_dark", segs=10)
    S.ellipsoid("flask", 2.66, -1.10, 0.74, 0.09, 0.09, 0.09, "glass", segs=14, rings=8)
    S.cyl("neck", 2.66, -1.10, 0.03, 0.80, 0.92, "glass", segs=8, bevel=0.0)
    S.cyl("candle", 2.70, -1.66, 0.035, 0.60, 0.72, "ceramic", segs=8)
    S.cyl("flame", 2.70, -1.66, 0.015, 0.72, 0.76, "lamp_warm", segs=6, r_top=0.002, bevel=0.0)
    return (3, 2), 0.96


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
    # the bulb is plain glass in the sprite; furniture/lamp_standing_glow lights
    # it only while the lamp has power (power_draw.go drawPowerParts)
    P.glow(S.cyl("bulb", 0.5, -0.5, 0.15, 1.38, 1.40, "glass", segs=20, bevel=0.0), "lamp_warm")
    return (1, 1), 1.40


# ------------------------------------------------------ electricity (PLAY-021)
# Emissive parts are marked with P.glow(): dark in the sprite, lit in the
# <name>_glow_<facing> layer the renderer draws while the bench has power
# (families/power.py). Each electric bench keeps its base bench's footprint
# and approach side but reads as steel and electronics.
TORCH_ZMAX = 1.16
TORCH_FIRE = (0.5, -0.5, 1.06)     # where drawFlame sits: the basket's mouth


def _torch_lamp():
    """1 x 1 wood-burning standing torch: field stones round a pole, an
    iron basket of split sticks on top. The flame is drawn live by the game
    at the `fire` attachment; unlit, the basket holds cold sticks."""
    import math
    S.cyl("footing", 0.5, -0.5, 0.22, 0.0, 0.035, "stone_dark", segs=18, bevel=0.01)
    for k in range(6):
        a = k / 6 * 2 * math.pi + 0.3
        S.ellipsoid("stone%d" % k, 0.5 + 0.17 * math.cos(a), -0.5 + 0.15 * math.sin(a), 0.03,
                    0.07, 0.06, 0.05, "stone" if k % 2 else "stone_dark", segs=10, rings=6)
    S.cyl("pole", 0.5, -0.5, 0.06, 0.0, 0.90, "bark", r_top=0.05, segs=10)
    for z in (0.28, 0.62):
        S.cyl("binding", 0.5, -0.5, 0.066, z, z + 0.05, "metal_dark", segs=10, bevel=0.0)
    S.cyl("basket", 0.5, -0.5, 0.09, 0.86, 1.04, "metal_dark", r_top=0.19, segs=18, bevel=0.012)
    S.cyl("rim", 0.5, -0.5, 0.20, 1.02, 1.05, "steel_dark", segs=18, bevel=0.005)
    S.cyl("coals", 0.5, -0.5, 0.165, 1.03, 1.052, "ash_char", segs=18, bevel=0.0)
    for k in range(4):
        a = k / 4 * 2 * math.pi + 0.785
        x, y = 0.5 + 0.185 * math.cos(a), -0.5 + 0.185 * math.sin(a)
        S.box("prong%d" % k, x - 0.014, y - 0.014, 1.0, x + 0.014, y + 0.014, 1.13, "metal_dark", bevel=0.0)
    for k, (dx, dy) in enumerate(((-0.08, 0.0), (0.0, 0.07), (0.07, -0.04))):
        S.box("stick%d" % k, 0.5 + dx - 0.09, -0.5 + dy - 0.022, 1.05, 0.5 + dx + 0.09, -0.5 + dy + 0.022, 1.09,
              "bark" if k % 2 else "wood", bevel=0.012)
    return (1, 1), TORCH_ZMAX


def _electric_stove():
    # 3 x 1 (STK-AR), the cook stands south of the middle cell. Enamel body,
    # a black glass cooktop with three rings (glow) on the middle, an oven
    # window (glow) under it, a back console with knobs and a display (glow),
    # a drawer cabinet west, a prep board east. No flue: it runs on power.
    S.box("plinth", 0.08, -0.92, 0.0, 2.92, -0.10, 0.08, "steel_dark", bevel=0.01)
    S.box("body", 0.06, -0.92, 0.08, 2.94, -0.08, 0.62, "ceramic", bevel=0.04)
    S.box("top", 0.04, -0.94, 0.62, 2.96, -0.06, 0.66, "steel", bevel=0.02)
    S.box("glass", 1.02, -0.88, 0.66, 1.98, -0.30, 0.672, "backdrop", bevel=0.008)
    for k, (x, y, r) in enumerate(((1.26, -0.66, 0.12), (1.74, -0.66, 0.12), (1.50, -0.44, 0.09))):
        P.glow(S.cyl("ring%d" % k, x, y, r, 0.672, 0.68, "steel_dark", segs=24, bevel=0.0), "gland_ember")
        S.cyl("ring_in%d" % k, x, y, r * 0.56, 0.672, 0.682, "backdrop", segs=20, bevel=0.0)
    S.cyl("pot", 1.26, -0.66, 0.11, 0.68, 0.80, "steel", r_top=0.115, segs=18)
    S.cyl("stew", 1.26, -0.66, 0.095, 0.795, 0.805, "crop_fruit", segs=18, bevel=0.0)
    S.box("console", 0.06, -0.26, 0.66, 2.94, -0.08, 0.92, "steel_dark", bevel=0.03)
    for k, x in enumerate((1.14, 1.34, 1.66, 1.86)):
        S.cyl("knob%d" % k, x, -0.27, 0.035, 0.78, 0.84, "metal_dark", segs=10, bevel=0.005)
    P.glow(S.box("display", 1.40, -0.275, 0.74, 1.60, -0.255, 0.86, "backdrop", bevel=0.005), "indicator")
    S.box("door", 1.06, -0.945, 0.14, 1.94, -0.91, 0.54, "steel", bevel=0.015)
    P.glow(S.box("oven_window", 1.16, -0.955, 0.22, 1.84, -0.94, 0.42, "backdrop", bevel=0.006), "lamp_warm")
    S.box("handle", 1.14, -0.965, 0.47, 1.86, -0.945, 0.50, "steel_dark", bevel=0.005)
    # west: drawers and a canister set
    for k, z in enumerate((0.14, 0.36)):
        S.box("drawer%d" % k, 0.16, -0.935, z, 0.86, -0.91, z + 0.18, "ceramic", bevel=0.015)
        S.box("pull%d" % k, 0.42, -0.95, z + 0.12, 0.60, -0.93, z + 0.15, "steel_dark", bevel=0.005)
    for k, x in enumerate((0.30, 0.52)):
        S.cyl("canister%d" % k, x, -0.48, 0.08, 0.66, 0.84 - 0.04 * k, "steel", segs=12, bevel=0.01)
    # east: board, vegetable, cable
    S.box("board", 2.18, -0.80, 0.66, 2.84, -0.36, 0.70, "wood_plank", bevel=0.015)
    S.ellipsoid("veg", 2.40, -0.56, 0.74, 0.06, 0.06, 0.05, "vegetation", segs=10, rings=6)
    S.box("cabinet_door", 2.16, -0.935, 0.14, 2.84, -0.91, 0.54, "ceramic", bevel=0.015)
    S.box("cabinet_pull", 2.24, -0.95, 0.30, 2.28, -0.93, 0.42, "steel_dark", bevel=0.005)
    S.box("cable", 2.84, -0.10, 0.0, 2.90, -0.02, 0.66, "fur_black", bevel=0.01)
    return (3, 1), 0.94


def _electric_tailoring_bench():
    """3 x 1 electric tailoring bench (STK-AR): a steel table, the sewing
    machine on the middle cell with its work lamp and power lamp (glow),
    a cloth bolt and shears west, spools and folded work east. The tailor
    stands south of the middle."""
    _bench_legs("steel_dark", r=0.045, bevel=0.012)
    S.box("stretcher", 0.10, -0.55, 0.14, 2.90, -0.45, 0.19, "steel_dark", bevel=0.01)
    S.box("top", 0.04, -0.94, 0.52, 2.96, -0.06, 0.58, "steel", bevel=0.03)
    # west: cloth and shears
    S.box("cloth", 0.12, -0.84, 0.58, 0.88, -0.36, 0.595, "cloth", bevel=0.01)
    S.box("bolt", 0.14, -0.30, 0.58, 0.86, -0.10, 0.70, "cloth", bevel=0.05)
    S.box("shear_a", 0.46, -0.74, 0.595, 0.70, -0.71, 0.615, "steel", bevel=0.004)
    S.box("shear_b", 0.48, -0.66, 0.595, 0.70, -0.69, 0.615, "steel", bevel=0.004)
    S.cyl("shear_ring", 0.72, -0.69, 0.03, 0.595, 0.62, "metal_dark", segs=10, bevel=0.0)
    # middle: the sewing machine (bed, pillar, arm, head; hand wheel east)
    S.box("bed", 1.08, -0.82, 0.58, 1.92, -0.34, 0.64, "mug_blue", bevel=0.02)
    S.box("fabric", 1.06, -0.78, 0.64, 1.48, -0.40, 0.65, "cloth", bevel=0.005)
    S.box("pillar", 1.70, -0.66, 0.64, 1.88, -0.44, 0.98, "mug_blue", bevel=0.03)
    S.box("arm", 1.20, -0.64, 0.86, 1.88, -0.46, 0.99, "mug_blue", bevel=0.04)
    S.box("head", 1.18, -0.67, 0.70, 1.34, -0.43, 0.99, "mug_blue", bevel=0.03)
    S.box("needle", 1.25, -0.565, 0.64, 1.27, -0.545, 0.72, "steel", bevel=0.0)
    P.glow(S.box("work_lamp", 1.19, -0.685, 0.70, 1.33, -0.66, 0.76, "backdrop", bevel=0.004), "lamp_warm")
    P.glow(S.cyl("power_lamp", 1.80, -0.55, 0.035, 0.99, 1.015, "backdrop", segs=10, bevel=0.0), "indicator")
    S.box("hand_wheel", 1.88, -0.64, 0.80, 1.93, -0.46, 0.98, "steel", bevel=0.03)
    S.cyl("spool", 1.56, -0.55, 0.045, 0.99, 1.07, "food_red", segs=12, bevel=0.005)
    S.cyl("spindle", 1.56, -0.55, 0.012, 0.99, 1.10, "steel", segs=6, bevel=0.0)
    # east: spool rack and folded work
    S.box("rack", 2.12, -0.30, 0.58, 2.84, -0.12, 0.64, "steel_dark", bevel=0.01)
    for k, (x, mat) in enumerate(((2.24, "food_red"), (2.42, "jacket_navy"), (2.60, "corn_yellow"),
                                   (2.76, "sheet_teal"))):
        S.cyl("rspool%d" % k, x, -0.21, 0.045, 0.64, 0.76, mat, segs=12, bevel=0.005)
    S.box("folded", 2.16, -0.84, 0.58, 2.78, -0.44, 0.64, "cloth", bevel=0.03)
    S.box("folded_top", 2.20, -0.80, 0.64, 2.74, -0.48, 0.69, "jacket_navy", bevel=0.03)
    S.box("cable", 2.84, -0.40, 0.0, 2.90, -0.34, 0.58, "fur_black", bevel=0.01)
    return (3, 1), 1.10


def _electric_research_bench():
    """5 x 2 hi-tech research bench (RSCH-3; kind and key stay the electric
    research bench): a long steel desk on two drawer pedestals. Along the
    back a server tower, a twin-screen terminal and an analyzer cabinet
    (screens and lamps glow); in front a keyboard, a holo pad, a sample
    rack, a data slate, a centrifuge and a microscope. The researcher
    sits south."""
    for k, x0 in enumerate((0.10, 4.20)):   # drawer pedestals
        S.box("pedestal%d" % k, x0, -1.90, 0.0, x0 + 0.70, -0.10, 0.52, "steel_dark", bevel=0.02)
        for d in range(3):
            z = 0.06 + d * 0.15
            S.box("drawer%d_%d" % (k, d), x0 + 0.06, -1.915, z, x0 + 0.64, -1.89, z + 0.12, "pod_hull", bevel=0.008)
            S.box("pull%d_%d" % (k, d), x0 + 0.27, -1.93, z + 0.08, x0 + 0.43, -1.91, z + 0.10, "metal_dark",
                  bevel=0.003)
    for k, x in enumerate((1.80, 3.20)):
        S.box("leg%d" % k, x - 0.05, -1.86, 0.0, x + 0.05, -1.76, 0.52, "steel_dark", bevel=0.012)
    S.box("modesty", 0.80, -0.20, 0.10, 4.20, -0.12, 0.50, "steel_dark", bevel=0.01)
    S.box("top", 0.04, -1.96, 0.52, 4.96, -0.04, 0.58, "steel", bevel=0.03)
    S.box("trim", 0.04, -1.97, 0.53, 4.96, -1.94, 0.57, "pod_paint", bevel=0.005)
    # terminal: two monitors on a shared arm, screens on their south faces
    S.box("arm_base", 2.36, -0.36, 0.58, 2.64, -0.16, 0.62, "metal_dark", bevel=0.01)
    S.box("arm_post", 2.46, -0.28, 0.62, 2.54, -0.20, 0.80, "metal_dark", bevel=0.008)
    for k, (x0, x1) in enumerate(((1.30, 2.46), (2.54, 3.70))):
        S.box("monitor%d" % k, x0, -0.34, 0.74, x1, -0.18, 1.22, "steel_dark", bevel=0.03)
        P.glow(S.box("screen%d" % k, x0 + 0.06, -0.355, 0.80, x1 - 0.06, -0.335, 1.16, "backdrop", bevel=0.005),
               "indicator")
    S.box("keyboard", 2.10, -0.92, 0.58, 2.90, -0.66, 0.605, "metal_dark", bevel=0.01)
    S.box("keys", 2.13, -0.895, 0.605, 2.87, -0.685, 0.612, "pod_hull", bevel=0.0)
    S.box("mouse", 3.02, -0.86, 0.58, 3.10, -0.74, 0.62, "metal_dark", bevel=0.02)
    # server tower on the back left: vents, a lamp column and a roof lamp
    S.box("tower", 0.14, -0.66, 0.58, 0.96, -0.10, 1.30, "steel_dark", bevel=0.03)
    for k in range(5):
        z = 0.66 + 0.09 * k
        S.box("vent%d" % k, 0.22, -0.675, z, 0.70, -0.655, z + 0.04, "metal_dark", bevel=0.0)
    for k in range(4):
        z = 0.70 + 0.12 * k
        P.glow(S.box("rack_lamp%d" % k, 0.78, -0.675, z, 0.86, -0.655, z + 0.05, "backdrop", bevel=0.003), "indicator")
    P.glow(S.box("tower_lamp", 0.30, -0.44, 1.30, 0.80, -0.32, 1.315, "backdrop", bevel=0.004), "indicator")
    # analyzer cabinet on the back right, a sample slot and status lamps
    S.box("analyzer", 4.00, -0.58, 0.58, 4.86, -0.10, 1.02, "ceramic", bevel=0.03)
    S.box("slot", 4.08, -0.595, 0.70, 4.40, -0.575, 0.76, "backdrop", bevel=0.004)
    for k, x in enumerate((4.52, 4.62, 4.72)):
        P.glow(S.box("lamp%d" % k, x - 0.025, -0.595, 0.88, x + 0.025, -0.575, 0.93, "backdrop", bevel=0.003),
               "indicator")
    P.glow(S.box("top_lamp", 4.14, -0.40, 1.02, 4.50, -0.30, 1.035, "backdrop", bevel=0.004), "indicator")
    # holo pad (front left of the keyboard): a dark disc ringed in teal light
    S.cyl("holo_base", 1.40, -1.30, 0.22, 0.58, 0.62, "metal_dark", segs=20, bevel=0.01)
    P.glow(S.cyl("holo_ring", 1.40, -1.30, 0.17, 0.62, 0.635, "backdrop", segs=20, bevel=0.0), "med_teal_glow")
    S.cyl("holo_core", 1.40, -1.30, 0.11, 0.62, 0.64, "steel_dark", segs=16, bevel=0.0)
    # sample rack (front left)
    S.box("rack", 0.30, -1.66, 0.58, 0.90, -1.40, 0.64, "steel_dark", bevel=0.01)
    for k, (x, mat) in enumerate(((0.38, "med_blue"), (0.50, "circuit"), (0.62, "med_red"), (0.74, "reagent_violet"),
                                  (0.84, "med_blue"))):
        S.cyl("tube%d" % k, x, -1.53, 0.026, 0.60, 0.78, "glass", segs=10, bevel=0.0)
        S.cyl("liquid%d" % k, x, -1.53, 0.022, 0.62, 0.70, mat, segs=10, bevel=0.0)
    # data slate and a stylus beside the keyboard
    S.box("slate", 2.30, -1.66, 0.58, 2.76, -1.32, 0.60, "steel_dark", bevel=0.01)
    S.box("slate_face", 2.34, -1.62, 0.60, 2.72, -1.36, 0.605, "pod_hull", bevel=0.0)
    S.box("stylus", 2.82, -1.60, 0.58, 2.85, -1.38, 0.60, "steel", bevel=0.005)
    # bench centrifuge (mid right): a squat drum with a lid and a lamp
    S.cyl("centrifuge", 3.40, -1.10, 0.20, 0.58, 0.76, "ceramic", segs=20, bevel=0.02)
    S.cyl("centrifuge_lid", 3.40, -1.10, 0.16, 0.76, 0.79, "pod_hull", segs=20, bevel=0.01)
    P.glow(S.box("centrifuge_lamp", 3.34, -1.315, 0.64, 3.46, -1.295, 0.68, "backdrop", bevel=0.003), "indicator")
    # microscope (front right): foot, pillar, stage, head and tube
    S.box("scope_foot", 3.70, -1.70, 0.58, 3.98, -1.44, 0.62, "metal_dark", bevel=0.012)
    S.box("scope_pillar", 3.86, -1.54, 0.62, 3.93, -1.47, 0.90, "metal_dark", bevel=0.008)
    S.box("scope_stage", 3.72, -1.66, 0.70, 3.90, -1.50, 0.72, "steel", bevel=0.005)
    S.box("scope_head", 3.74, -1.58, 0.86, 3.93, -1.49, 0.93, "ceramic", bevel=0.012)
    S.cyl("scope_tube", 3.81, -1.56, 0.032, 0.72, 1.00, "steel", segs=10, bevel=0.0)
    S.box("cable", 2.50, -0.12, 0.0, 2.56, -0.04, 0.66, "fur_black", bevel=0.01)
    return (5, 2), 1.32


def _multi_analyzer():
    """2 x 2 multi-analyzer (RSCH-3): a facility a hi-tech research bench
    links to. A white enamel base on a plinth carries an instrument column
    with a read-out screen, a slim signal tower and a glass-domed sample
    carousel; the screen, lamps and carousel ring glow while powered."""
    import math
    S.box("plinth", 0.06, -1.94, 0.0, 1.94, -0.06, 0.10, "steel_dark", bevel=0.015)
    S.box("base", 0.12, -1.88, 0.10, 1.88, -0.12, 0.62, "ceramic", bevel=0.04, segments=2)
    S.box("base_top", 0.10, -1.90, 0.60, 1.90, -0.10, 0.64, "steel", bevel=0.02)
    S.box("stripe", 0.13, -1.895, 0.46, 1.87, -1.875, 0.52, "pod_paint", bevel=0.0)
    # front panel: sample drawer, keypad and vents
    S.box("drawer", 0.26, -1.905, 0.18, 0.98, -1.88, 0.40, "pod_hull", bevel=0.01)
    S.box("drawer_pull", 0.50, -1.925, 0.33, 0.74, -1.90, 0.36, "metal_dark", bevel=0.004)
    for k in range(3):
        for j in range(2):
            S.box("key%d%d" % (k, j), 1.18 + 0.09 * k, -1.90, 0.24 + 0.08 * j, 1.25 + 0.09 * k, -1.88,
                  0.30 + 0.08 * j, "metal_dark", bevel=0.003)
    for k in range(3):
        z = 0.16 + 0.05 * k
        S.box("vent%d" % k, 1.50, -1.895, z, 1.80, -1.88, z + 0.025, "metal_dark", bevel=0.0)
    P.glow(S.box("panel_lamp", 1.52, -1.905, 0.36, 1.78, -1.885, 0.40, "backdrop", bevel=0.003), "indicator")
    # instrument column (back left) with a read-out on its south face
    S.box("column", 0.18, -0.92, 0.64, 1.10, -0.18, 1.28, "pod_hull", bevel=0.035, segments=2)
    S.box("bezel", 0.24, -0.935, 0.76, 1.04, -0.915, 1.16, "steel_dark", bevel=0.01)
    P.glow(S.box("readout", 0.28, -0.95, 0.80, 1.00, -0.93, 1.12, "backdrop", bevel=0.004), "indicator")
    S.box("column_cap", 0.16, -0.94, 1.26, 1.12, -0.16, 1.31, "steel", bevel=0.015)
    P.glow(S.box("cap_lamp", 0.34, -0.62, 1.31, 0.94, -0.48, 1.325, "backdrop", bevel=0.004), "indicator")
    # slim signal tower (back right): a lamp strip south, vents on top
    S.box("tower", 1.40, -0.62, 0.64, 1.80, -0.20, 1.16, "steel_dark", bevel=0.025)
    for k in range(4):
        z = 0.72 + 0.10 * k
        P.glow(S.box("tower_lamp%d" % k, 1.54, -0.635, z, 1.66, -0.615, z + 0.05, "backdrop", bevel=0.003),
               "med_teal_glow")
    for k in range(3):
        x = 1.47 + 0.10 * k
        S.box("roof_vent%d" % k, x, -0.56, 1.16, x + 0.06, -0.26, 1.175, "metal_dark", bevel=0.0)
    # open sample carousel (front right) on a ringed turntable
    cx, cy = 1.30, -1.28
    S.cyl("turntable", cx, cy, 0.42, 0.64, 0.70, "metal_dark", segs=28, bevel=0.01)
    P.glow(S.cyl("ring", cx, cy, 0.40, 0.70, 0.712, "backdrop", segs=28, bevel=0.0), "med_teal_glow")
    S.cyl("deck", cx, cy, 0.34, 0.70, 0.73, "steel", segs=24, bevel=0.005)
    liquids = ("med_blue", "med_red", "circuit", "reagent_violet", "med_amber", "med_blue")
    for k, mat in enumerate(liquids):
        a = k / len(liquids) * 2 * math.pi + 0.3
        x, y = cx + 0.22 * math.cos(a), cy + 0.22 * math.sin(a)
        S.cyl("vial%d" % k, x, y, 0.04, 0.73, 0.94, "glass", segs=10, bevel=0.0)
        S.cyl("vial_fill%d" % k, x, y, 0.034, 0.75, 0.86, mat, segs=10, bevel=0.0)
    S.cyl("spindle", cx, cy, 0.05, 0.73, 1.00, "steel", segs=12, bevel=0.005)
    S.cyl("hub", cx, cy, 0.08, 1.00, 1.04, "metal_dark", segs=12, bevel=0.01)
    # power trunk into the back left corner
    S.box("cable", 0.14, -0.14, 0.0, 0.22, -0.06, 0.64, "fur_black", bevel=0.01)
    return (2, 2), 1.33


def _vitals_monitor():
    """1x1 vitals monitor (MED-06, ART-026): a wheeled five-spoke stand, a
    steel pole with a cable tray, and a dark monitor tilted toward its
    approach side, its screen tracing a teal heartbeat (glow) beside a
    status lamp on top (glow, so every facing shows a lit part)."""
    import math
    from mathutils import Matrix
    from .plant_base import _limb
    cx, cy = 0.5, -0.5
    for k in range(5):
        a = math.radians(90 + 72 * k)
        ex, ey = cx + 0.27 * math.cos(a), cy + 0.27 * math.sin(a)
        _limb("spoke%d" % k, (cx, cy, 0.07), (ex, ey, 0.06), 0.03, 0.022, "metal_dark", segs=8)
        S.cyl("caster%d" % k, ex, ey, 0.04, 0.0, 0.06, "steel_dark", segs=10, bevel=0.008)
    S.cyl("hub", cx, cy, 0.07, 0.03, 0.11, "metal_dark", segs=14, bevel=0.012)
    S.cyl("pole", cx, cy, 0.03, 0.1, 0.66, "steel", segs=10, bevel=0.0)
    S.cyl("tray", cx, cy - 0.04, 0.15, 0.38, 0.41, "steel_dark", segs=18, bevel=0.008)
    S.box("pad", cx - 0.08, cy - 0.12, 0.41, cx + 0.06, cy + 0.0, 0.44, "med_white", bevel=0.01)
    S.box("mount", cx - 0.06, cy - 0.04, 0.62, cx + 0.06, cy + 0.06, 0.72, "metal_dark", bevel=0.012)
    # the monitor, authored upright with its screen on the south face, then tilted back
    n0 = len(S.objects())
    S.box("housing", 0.19, -0.60, 0.66, 0.81, -0.44, 1.04, "steel_dark", bevel=0.03)
    S.box("screen", 0.23, -0.612, 0.71, 0.77, -0.59, 1.0, "backdrop", bevel=0.006)
    trace = [(0.24, 0.84), (0.38, 0.84), (0.42, 0.80), (0.46, 0.97), (0.51, 0.75), (0.55, 0.86), (0.60, 0.84),
             (0.76, 0.84)]
    for k in range(len(trace) - 1):
        (x0, z0), (x1, z1) = trace[k], trace[k + 1]
        P.glow(_limb("beat%d" % k, (x0, -0.618, z0), (x1, -0.618, z1), 0.014, 0.014, "med_teal", segs=6),
               "med_teal_glow")
    for k, x in enumerate((0.30, 0.40)):
        S.box("readout%d" % k, x - 0.04, -0.62, 0.92, x + 0.04, -0.60, 0.95, "med_teal", bevel=0.0)
    P.glow(S.cyl("status", 0.72, -0.52, 0.03, 1.04, 1.06, "med_teal", segs=10, bevel=0.0), "med_teal_glow")
    m = (Matrix.Translation((0, -0.5, 0.72)) @ Matrix.Rotation(math.radians(-38), 4, "X")
         @ Matrix.Translation((0, 0.5, -0.72)))
    for ob in S.objects()[n0:]:
        ob.matrix_world = m @ ob.matrix_world
    S.box("cable", cx + 0.02, cy - 0.01, 0.1, cx + 0.05, cy + 0.02, 0.66, "fur_black", bevel=0.0)
    return (1, 1), 1.02


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
    "hospital_bed": (_hospital_bed, {"attachments": {"sleeper": [0.5, 0.42]}, "building": "hospital bed"}),
    "tool_cabinet": (_tool_cabinet, {"building": "tool cabinet"}),
    "chair_basic": (_chair, {"attachments": {"seat": [0.5, 0.52]}}),
    "workbench_basic": (_workbench, {"attachments": {"worker": [1.5, 1.15]}}),
    "lamp_standing": (_lamp, {"emissive": "lamp_warm", "light_radius": 6}),
    "stonecutter_table": (_stonecutter_table, {"attachments": {"worker": [1.5, 1.15]}}),
    "stove_basic": (_stove, {"attachments": {"cook": [1.5, 1.15]}}),
    "butcher_table": (_butcher_table, {"attachments": {"worker": [1.5, 1.15]}}),
    "brewing_bench": (_brewing_bench, {"attachments": {"worker": [1.5, 1.15]}, "building": "brewing bench"}),
    "drug_table": (_drug_table, {"attachments": {"worker": [1.5, 1.15]}, "building": "drug table"}),
    "campfire": (_campfire, {}),
    "feeding_trough_empty": (lambda: _trough(False), {"building": "feeding trough", "state": "empty"}),
    "feeding_trough_full": (lambda: _trough(True), {"building": "feeding trough", "state": "full"}),
    "horseshoe_pin": (_horseshoe_pin, {}),
    "sandbags": (_sandbags, {}),
    "research_bench": (_research_bench, {"attachments": {"worker": [1.5, 2.15]}}),
    "spike_trap": (_spike_trap, {}),
    "tailoring_bench": (_tailoring_bench, {"attachments": {"worker": [1.5, 1.15]}}),
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
    # PLAY-021: the torch's flame is drawn at `fire` (canvas px); the electric
    # benches keep their base bench's approach point.
    "torch_lamp": (_torch_lamp, {"building": "torch lamp", "emissive": "lamp_warm", "light_radius": 4,
                                 "fuel": "wood", "flame_scale": 0.55,
                                 "attachments": {"fire": P.project(TORCH_FIRE, TORCH_ZMAX)}}),
    "electric_stove": (_electric_stove, {"attachments": {"cook": [1.5, 1.15]}, "building": "electric stove",
                                         "recipes": "stove"}),
    "electric_tailoring_bench": (_electric_tailoring_bench, {"attachments": {"worker": [1.5, 1.15]},
                                                             "building": "electric tailoring bench",
                                                             "recipes": "tailoring bench"}),
    "electric_research_bench": (_electric_research_bench, {"attachments": {"worker": [2.5, 2.15]},
                                                           "building": "electric research bench",
                                                           "recipes": "research bench"}),
    "vitals_monitor": (_vitals_monitor, {"building": "vitals monitor"}),
    # RSCH-3: the multi-analyzer facility (one facing; its glow is the on state)
    "multi_analyzer": (_multi_analyzer, {"building": "multi-analyzer"}),
}
FACINGS_ONLY = {"grave_dug": ("south", "east"), "grave_filled": ("south", "east")}
ROTATABLE = {"hospital_bed", "grave_dug", "grave_filled", "bed_basic", "bed_double", "chair_basic", "workbench_basic", "stonecutter_table", "stove_basic", "butcher_table",
             "drying_rack", "tailoring_bench", "research_bench", "brewing_bench", "drug_table", "feeding_trough_empty", "feeding_trough_full",
             "electric_stove", "electric_tailoring_bench", "electric_research_bench", "vitals_monitor"}


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


def build(only=None):
    """Every furniture sprite; `only` (names from BUILDERS) renders just those."""
    out = [] if only else _blankets()
    for name, (fn, extra) in BUILDERS.items():
        if only and name not in only:
            continue
        facings = FACINGS_ONLY.get(name) or (FACINGS if name in ROTATABLE else ("south",))
        for f in facings:
            fp0, zmax = fn()
            lits = P.glow_colours()
            fp = S.face(S.objects(), f, fp0)
            ex = dict(extra, rotatable=name in ROTATABLE, facing=f)
            suffix = "_" + f if name in ROTATABLE else ""
            gname = P.glow_name(name + suffix)
            if lits:   # PLAY-021: emissive parts get a glow layer
                ex.update(emissive=lits[0], glow="furniture/" + gname)
            out.append(S.render_asset("furniture", name + suffix, footprint=fp, zmax=zmax, extra=ex))
            if lits:
                fn()
                S.face(S.objects(), f, fp0)
                out.append(P.glow_layer("furniture", gname, fp, zmax, "furniture/" + name + suffix, lits[0]))
    return out
