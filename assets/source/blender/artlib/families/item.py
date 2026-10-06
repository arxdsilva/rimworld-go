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


def build():
    return [_wood(), _food(), _steel(), _stone()]
