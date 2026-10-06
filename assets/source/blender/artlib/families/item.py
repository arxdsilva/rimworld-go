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


def _club():
    import math
    from mathutils import Matrix
    S.cyl("shaft", 0, 0, 0.045, 0.0, 0.55, "wood", segs=10, r_top=0.07, bevel=0.01)
    S.objects()[-1].matrix_world = Matrix.Translation((0.20, -0.55, 0.06)) @ Matrix.Rotation(math.radians(90), 4, "Y") @ Matrix.Rotation(math.radians(-20), 4, "X")
    S.ellipsoid("head", 0.78, -0.48, 0.09, 0.12, 0.09, 0.08, "bark", segs=12, rings=8)
    S.box("grip", 0.18, -0.60, 0.02, 0.32, -0.50, 0.10, "leather", bevel=0.02)
    return S.render_asset("item", "club_wood", zmax=0.2, extra={"item": "club"})


def _rifle_item():
    import math
    from mathutils import Matrix
    y = -0.5
    S.box("stock", 0.06, y - 0.045, 0.0, 0.30, y + 0.045, 0.06, "wood", bevel=0.02)
    S.box("receiver", 0.28, y - 0.04, 0.0, 0.58, y + 0.04, 0.07, "metal_dark", bevel=0.012)
    S.box("mag", 0.44, y - 0.13, 0.0, 0.51, y - 0.03, 0.045, "metal_dark", bevel=0.01)
    S.box("guard", 0.56, y - 0.035, 0.0, 0.74, y + 0.035, 0.055, "wood", bevel=0.012)
    S.cyl("barrel", 0, 0, 0.018, 0.0, 0.22, "steel", segs=8, bevel=0.0)
    S.objects()[-1].matrix_world = Matrix.Translation((0.72, y, 0.03)) @ Matrix.Rotation(math.radians(90), 4, "Y")
    return S.render_asset("item", "rifle_ground", zmax=0.12, extra={"item": "rifle"})


# --- farm produce ----------------------------------------------------------

def _potatoes():
    for k, (x, y, z, r) in enumerate([(0.36, -0.58, 0.0, 0.10), (0.56, -0.62, 0.0, 0.11), (0.66, -0.44, 0.0, 0.09),
                                       (0.44, -0.42, 0.0, 0.09), (0.50, -0.52, 0.12, 0.09)]):
        S.blob("potato%d" % k, x, y, z, r, "tuber", seed=120 + k, lumps=0.18, squash=0.8)
    return S.render_asset("item", "potatoes", zmax=0.24, extra={"item": "potatoes", "stack": True})


def _corn():
    import math
    from mathutils import Matrix
    for k, (x, y, z, a) in enumerate([(0.30, -0.62, 0.05, 8), (0.30, -0.46, 0.05, -6), (0.36, -0.54, 0.14, 2)]):
        S.ellipsoid("cob%d" % k, 0, 0, 0, 0.26, 0.065, 0.065, "corn_yellow", segs=14, rings=8)
        S.objects()[-1].matrix_world = Matrix.Translation((x + 0.2, y, z + 0.06)) @ Matrix.Rotation(math.radians(a), 4, "Z")
        S.ellipsoid("husk%d" % k, 0, 0, 0, 0.12, 0.07, 0.06, "leaf_light", segs=10, rings=6)
        S.objects()[-1].matrix_world = Matrix.Translation((x + 0.44, y, z + 0.06)) @ Matrix.Rotation(math.radians(a), 4, "Z")
    return S.render_asset("item", "corn_pile", zmax=0.28, extra={"item": "corn", "stack": True})


def _wheat():
    import random
    rnd = random.Random(5)
    for i in range(22):
        x, y = 0.5 + rnd.uniform(-0.12, 0.12), -0.5 + rnd.uniform(-0.1, 0.1)
        S.cyl("straw%d" % i, x, y, 0.02, 0.0, 0.46, "straw", segs=5, bevel=0.0)
        S.ellipsoid("ear%d" % i, x + rnd.uniform(-0.08, 0.08), y, 0.52, 0.04, 0.04, 0.09, "corn_yellow",
                    segs=6, rings=5)
    S.cyl("tie", 0.5, -0.5, 0.15, 0.18, 0.24, "leather", segs=16, bevel=0.005)
    return S.render_asset("item", "wheat_sheaf", zmax=0.64, extra={"item": "wheat", "stack": True})


def _beans():
    S.cyl("sack", 0.5, -0.55, 0.22, 0.0, 0.26, "leather", r_top=0.18, segs=16)
    S.cyl("mouth", 0.5, -0.55, 0.19, 0.25, 0.29, "bark", segs=16, bevel=0.01)
    for k, (x, y) in enumerate([(0.44, -0.52), (0.54, -0.50), (0.50, -0.60), (0.58, -0.58), (0.42, -0.60),
                                (0.48, -0.46)]):
        S.ellipsoid("bean%d" % k, x, y, 0.29, 0.035, 0.022, 0.02, "ceramic" if k % 2 else "tuber", segs=8, rings=5)
    return S.render_asset("item", "beans_sack", zmax=0.32, extra={"item": "beans", "stack": True})


def _lettuce():
    for k, (x, y, r) in enumerate([(0.38, -0.56, 0.15), (0.62, -0.54, 0.14)]):
        S.ellipsoid("head%d" % k, x, y, r * 0.8, r, r, r * 0.8, "leaf_light", segs=16, rings=10)
        S.ellipsoid("outer%d" % k, x, y - 0.02, r * 0.5, r * 1.15, r * 1.1, r * 0.55, "vegetation", segs=16, rings=8)
    return S.render_asset("item", "lettuce_heads", zmax=0.28, extra={"item": "lettuce", "stack": True})


def _radishes():
    for k, (x, y) in enumerate([(0.40, -0.60), (0.52, -0.56), (0.62, -0.62), (0.48, -0.68)]):
        S.ellipsoid("radish%d" % k, x, y, 0.06, 0.065, 0.065, 0.06, "food_red", segs=12, rings=8)
        S.cyl("leaves%d" % k, x - 0.02, y + 0.10, 0.03, 0.04, 0.10, "vegetation", segs=6, r_top=0.05, bevel=0.0)
    return S.render_asset("item", "radish_bunch", zmax=0.18, extra={"item": "radishes", "stack": True})


def _pumpkins():
    for k, (x, y, r) in enumerate([(0.40, -0.58, 0.17), (0.64, -0.50, 0.13)]):
        S.ellipsoid("pumpkin%d" % k, x, y, r * 0.75, r, r * 0.95, r * 0.75, "pumpkin", segs=18, rings=10)
        S.cyl("stem%d" % k, x, y, 0.018, r * 1.4, r * 1.7, "bark", segs=6, bevel=0.0)
    return S.render_asset("item", "pumpkins", zmax=0.32, extra={"item": "pumpkins", "stack": True})


def _leather():
    """A stack of tanned hides, edges uneven."""
    for k, (x, y, z, w) in enumerate([(0.22, -0.74, 0.0, 0.56), (0.26, -0.70, 0.045, 0.50), (0.20, -0.72, 0.09, 0.54)]):
        S.box("hide%d" % k, x, y, z, x + w, y + 0.40, z + 0.045, "leather", bevel=0.02)
    S.box("cord", 0.46, -0.76, 0.0, 0.52, -0.28, 0.15, "straw", bevel=0.01)
    return S.render_asset("item", "leather_hides", zmax=0.18, extra={"item": "leather", "stack": True})


def _parka():
    """A folded leather parka with a fur-lined hood."""
    S.box("body", 0.18, -0.78, 0.0, 0.82, -0.30, 0.12, "leather", bevel=0.05, segments=2)
    S.box("fold", 0.22, -0.74, 0.12, 0.78, -0.40, 0.18, "leather", bevel=0.04)
    S.ellipsoid("hood", 0.50, -0.30, 0.14, 0.22, 0.10, 0.08, "fur_cream", segs=14, rings=8)
    for k, x in enumerate([0.36, 0.50, 0.64]):
        S.cyl("toggle%d" % k, x, -0.60, 0.018, 0.17, 0.20, "horn", segs=8, bevel=0.0)
    return S.render_asset("item", "parka_folded", zmax=0.24, extra={"item": "parka"})


def _tuque():
    """A knitted wool tuque with a rolled cuff and a bobble."""
    S.ellipsoid("crown", 0.5, -0.55, 0.06, 0.20, 0.20, 0.17, "cloth", segs=18, rings=10)
    S.cyl("cuff", 0.5, -0.55, 0.205, 0.0, 0.07, "fur_tan", segs=18)
    S.ellipsoid("bobble", 0.5, -0.55, 0.24, 0.06, 0.06, 0.06, "fur_cream", segs=10, rings=6)
    return S.render_asset("item", "tuque", zmax=0.30, extra={"item": "tuque"})


def _duster():
    """A long leather duster, folded lengthwise."""
    S.box("coat", 0.16, -0.86, 0.0, 0.84, -0.24, 0.09, "leather", bevel=0.04, segments=2)
    S.box("lapel", 0.30, -0.36, 0.09, 0.70, -0.26, 0.13, "bark", bevel=0.02)
    S.box("belt", 0.16, -0.62, 0.09, 0.84, -0.56, 0.11, "bark", bevel=0.01)
    return S.render_asset("item", "duster_folded", zmax=0.16, extra={"item": "duster"})


def build():
    return [_wood(), _food(), _steel(), _stone(), _berries(), _ration(), _meat(), _meal(), _club(), _rifle_item(),
            _potatoes(), _corn(), _wheat(), _beans(), _lettuce(), _radishes(), _pumpkins(),
            _leather(), _parka(), _tuque(), _duster()]
