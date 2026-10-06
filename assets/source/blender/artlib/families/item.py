"""Items on the ground (ART spec §15-§16): compact silhouettes, material
reads first. Each item occupies the centre of its tile."""
from .. import stage as S


def _wood():
    """Logs run north-south so their light end grain faces the viewer."""
    import math
    from mathutils import Matrix
    r = 0.1
    logs = [(0.29, 0.0, 0.70), (0.50, 0.0, 0.64), (0.71, 0.0, 0.72), (0.40, 0.17, 0.66), (0.60, 0.17, 0.68)]
    for k, (x, z, length) in enumerate(logs):
        y0 = -0.16 - (k % 2) * 0.03
        M = Matrix.Translation((x, y0, z + r)) @ Matrix.Rotation(math.radians(90), 4, "X")
        S.cyl("log%d" % k, 0, 0, r, 0.0, length, "bark", segs=12, bevel=0.015)
        S.objects()[-1].matrix_world = M
        S.cyl("end%d" % k, 0, 0, r * 0.8, length, length + 0.012, "wood", segs=12, bevel=0.0)
        S.objects()[-1].matrix_world = M
        S.cyl("ring%d" % k, 0, 0, r * 0.32, length + 0.012, length + 0.02, "wood_plank", segs=10, bevel=0.0)
        S.objects()[-1].matrix_world = M
    return S.render_asset("item", "wood_pile", zmax=0.48, extra={"item": "wood", "stack": True})


def _food():
    S.box("crate", 0.20, -0.78, 0.0, 0.80, -0.30, 0.22, "wood", bevel=0.025)
    S.box("inner", 0.25, -0.73, 0.18, 0.75, -0.35, 0.225, "bark", bevel=0.0)
    for k, (x, y) in enumerate([(0.36, -0.45), (0.56, -0.47), (0.44, -0.62), (0.64, -0.62), (0.50, -0.40)]):
        S.ellipsoid("bulb%d" % k, x, y, 0.25, 0.1, 0.09, 0.08, "crop_fruit", segs=12, rings=8)
    S.box("slat", 0.20, -0.80, 0.08, 0.80, -0.76, 0.13, "wood_plank", bevel=0.005)
    return S.render_asset("item", "food_crate", zmax=0.34, extra={"item": "food", "stack": True})


def _steel():
    bars = [(0.22, -0.66, 0), (0.22, -0.46, 0), (0.30, -0.56, 1)]
    for k, (x, y, layer) in enumerate(bars):
        z = layer * 0.09
        S.prism("bar%d" % k, [(x, y - 0.08), (x + 0.5, y - 0.08), (x + 0.47, y + 0.08), (x + 0.03, y + 0.08)],
                z, z + 0.09, "steel", bevel=0.012)
    return S.render_asset("item", "steel_bars", zmax=0.2, extra={"item": "steel", "stack": True})


def _stone():
    blocks = [(0.18, -0.74, 0.0), (0.50, -0.74, 0.0), (0.18, -0.44, 0.0), (0.34, -0.60, 0.16)]
    for k, (x, y, z) in enumerate(blocks):
        S.box("block%d" % k, x, y, z, x + 0.30, y + 0.28, z + 0.16,
              "stone" if k != 1 else "stone_dark", bevel=0.025)
    return S.render_asset("item", "stone_blocks", zmax=0.33, extra={"item": "stone_blocks", "stack": True})


def _berries():
    S.cyl("basket", 0.5, -0.55, 0.26, 0.0, 0.16, "wood", r_top=0.30, segs=18)
    S.cyl("rim", 0.5, -0.55, 0.31, 0.15, 0.19, "bark", segs=18, bevel=0.01)
    for k, (x, y) in enumerate([(0.40, -0.50), (0.52, -0.46), (0.62, -0.54), (0.46, -0.62),
                                (0.58, -0.64), (0.50, -0.56), (0.38, -0.60), (0.62, -0.44)]):
        S.ellipsoid("b%d" % k, x, y, 0.20 + 0.03 * (k % 3), 0.055, 0.055, 0.05, "food_red", segs=10, rings=6)
    S.ellipsoid("leaf", 0.66, -0.40, 0.24, 0.07, 0.035, 0.015, "foliage", segs=8, rings=4)
    return S.render_asset("item", "berries_basket", zmax=0.32, extra={"item": "berries", "stack": True})


def _ration():
    for k, (x, y, z) in enumerate([(0.24, -0.68, 0.0), (0.52, -0.68, 0.0), (0.30, -0.42, 0.0), (0.38, -0.56, 0.11)]):
        S.box("pack%d" % k, x, y, z, x + 0.26, y + 0.20, z + 0.11, "ceramic" if k % 2 == 0 else "glass", bevel=0.03)
        S.box("band%d" % k, x + 0.10, y - 0.005, z, x + 0.15, y + 0.205, z + 0.115, "food_red", bevel=0.0)
    return S.render_asset("item", "ration_pack", zmax=0.24, extra={"item": "food", "stack": True})


def _meat():
    for k, (x, y, r) in enumerate([(0.40, -0.56, 0.17), (0.60, -0.50, 0.15), (0.52, -0.66, 0.13)]):
        S.blob("cut%d" % k, x, y, 0.0, r, "food_red", seed=60 + k, lumps=0.25, squash=0.55)
    S.cyl("bone", 0.62, -0.60, 0.025, 0.08, 0.10, "ceramic", segs=8, ry=0.12, bevel=0.0)
    return S.render_asset("item", "meat_pile", zmax=0.2, extra={"item": "meat", "stack": True})


def _meal():
    S.cyl("plate", 0.5, -0.55, 0.30, 0.0, 0.04, "ceramic", segs=24, bevel=0.01)
    S.cyl("bowl", 0.5, -0.55, 0.20, 0.03, 0.15, "ceramic", r_top=0.24, segs=20)
    S.cyl("stew", 0.5, -0.55, 0.19, 0.14, 0.155, "crop_fruit", segs=20, bevel=0.0)
    for k, (x, y) in enumerate([(0.44, -0.52), (0.56, -0.58), (0.50, -0.48)]):
        S.ellipsoid("chunk%d" % k, x, y, 0.16, 0.04, 0.04, 0.03, "food_red" if k else "vegetation", segs=8, rings=4)
    S.box("spoon", 0.68, -0.70, 0.04, 0.84, -0.66, 0.06, "steel", bevel=0.005)
    return S.render_asset("item", "meal_simple", zmax=0.2, extra={"item": "meal", "stack": True})


def build():
    return [_wood(), _food(), _steel(), _stone(), _berries(), _ration(), _meat(), _meal()]
