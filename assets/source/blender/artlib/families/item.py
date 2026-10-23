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
              "stone_neutral" if k != 1 else "stone_neutral_dark", bevel=0.025)
    return S.render_asset("item", "stone_blocks", zmax=0.33, extra={"item": "stone_blocks", "stack": True})


def _stone_chunk():
    """A quarried chunk: same geometry as the terrain rock_chunk, tint-neutral."""
    r = 0.30
    S.blob("rock", 0.5, -0.52, 0.0, r, "stone_neutral", seed=11, lumps=0.25, squash=0.8)
    S.blob("rock2", 0.5 + r * 0.9, -0.62, 0.0, r * 0.45, "stone_neutral_dark", seed=12, lumps=0.3)
    return S.render_asset("item", "stone_chunk", zmax=r * 0.9, extra={"item": "stone_chunk", "stack": True})


def _silver():
    bars = [(0.20, -0.70, 0), (0.24, -0.50, 0), (0.30, -0.60, 1), (0.52, -0.62, 0)]
    for k, (x, y, layer) in enumerate(bars):
        z = layer * 0.08
        w = 0.34 if k == 3 else 0.46
        S.prism("bar%d" % k, [(x, y - 0.07), (x + w, y - 0.07), (x + w - 0.03, y + 0.07), (x + 0.03, y + 0.07)],
                z, z + 0.08, "silver", bevel=0.012)
    return S.render_asset("item", "silver", zmax=0.19, extra={"item": "silver", "stack": True})


def _gold():
    nuggets = [(0.36, -0.58, 0.0, 0.17, 3), (0.62, -0.50, 0.0, 0.14, 4), (0.52, -0.74, 0.0, 0.12, 5),
               (0.48, -0.56, 0.12, 0.13, 6)]
    for k, (x, y, z, r, seed) in enumerate(nuggets):
        S.blob("nugget%d" % k, x, y, z, r, "gold", seed=seed, lumps=0.30, squash=0.8)
    return S.render_asset("item", "gold", zmax=0.3, extra={"item": "gold", "stack": True})


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


def _knife_item():
    """A flint knife lying on the ground, angled."""
    from .gear import _knife_geo
    _knife_geo()
    S.rotate_z(S.objects(), -22)
    return S.render_asset("item", "knife", zmax=0.12, extra={"item": "knife"})


def _spear_item():
    """A stone-tipped spear lying on the ground, angled."""
    from .gear import _spear_geo
    _spear_geo()
    S.rotate_z(S.objects(), 24)
    return S.render_asset("item", "spear", zmax=0.12, extra={"item": "spear"})


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


def _herbal_medicine():
    """Herbal medicine: bundles of dried healroot bound with twine, a few
    knobbly roots beside them."""
    for k, (x, y, a) in enumerate([(0.42, -0.56, 0.0), (0.58, -0.50, 0.0)]):
        S.box("bundle%d" % k, x - 0.07, y - 0.20, 0.0, x + 0.07, y + 0.12, 0.09, "vegetation", bevel=0.035, segments=2)
        S.box("tips%d" % k, x - 0.08, y + 0.10, 0.01, x + 0.08, y + 0.20, 0.08, "leaf_light", bevel=0.03)
        S.box("twine%d" % k, x - 0.075, y - 0.06, 0.0, x + 0.075, y - 0.02, 0.10, "straw", bevel=0.01)
    for k, (x, y) in enumerate([(0.30, -0.72), (0.70, -0.74)]):
        S.blob("root%d" % k, x, y, 0.02, 0.055, "tuber", seed=120 + k, lumps=0.3, squash=0.6)
    return S.render_asset("item", "herbal_medicine", zmax=0.14, extra={"item": "herbal medicine", "stack": True})


def _bones(variant):
    """Bleached remains half sunk in the turf: a skull and a fan of ribs
    off a short spine, with grass tufts closing over the low parts."""
    import math
    y0 = -0.5
    sx = 1.0 if variant == 0 else -1.0
    x = lambda v: 0.5 + sx * v
    S.ellipsoid("turf", 0.5, y0, 0.0, 0.40, 0.30, 0.02, "vegetation", segs=18, rings=6)
    # spine: a row of small vertebrae
    for k in range(6):
        S.ellipsoid("vert", x(-0.18 + 0.065 * k), y0 + 0.02 - 0.012 * k, 0.03, 0.03, 0.026, 0.026, "horn",
                    segs=8, rings=5)
    # ribs: curved strips arching away from the spine, curling toward the skull end
    for k in range(5):
        bx = -0.14 + 0.065 * k
        for side in (-1, 1):
            r = 0.19 - 0.018 * abs(k - 2)
            cl = []
            for i in range(9):
                t = i / 8.0
                a = math.radians(160 * t)
                cl.append((x(bx + 0.05 * t * t * 1.4), y0 + 0.02 - 0.012 * k + side * r * math.sin(a * 0.95)))
            left, right = [], []
            for i, (px, py) in enumerate(cl):
                qx, qy = cl[min(i + 1, 8)][0] - cl[max(i - 1, 0)][0], cl[min(i + 1, 8)][1] - cl[max(i - 1, 0)][1]
                ln = math.hypot(qx, qy) or 1.0
                nx_, ny_ = -qy / ln, qx / ln
                w = 0.020 * (1 - 0.4 * i / 8)
                left.append((px + nx_ * w, py + ny_ * w))
                right.append((px - nx_ * w, py - ny_ * w))
            strip = left + right[::-1]
            if (side * sx) < 0:
                strip = strip[::-1]
            S.prism("rib", strip, 0.0, 0.04, "horn", bevel=0.006)
    # skull: cranium, jaw and dark sockets
    sk = x(0.27)
    S.ellipsoid("cranium", sk, y0 - 0.02, 0.05, 0.085, 0.10, 0.065, "ceramic", segs=16, rings=10)
    S.ellipsoid("jaw", x(0.35), y0 - 0.02, 0.03, 0.06, 0.055, 0.035, "horn", segs=12, rings=8)
    for sy in (-1, 1):
        S.ellipsoid("socket", x(0.31), y0 - 0.02 + sy * 0.04, 0.085, 0.026, 0.022, 0.02, "fur_dark", segs=8, rings=6)
    S.ellipsoid("horn_stub", sk - sx * 0.02, y0 - 0.1, 0.075, 0.025, 0.02, 0.02, "horn", segs=8, rings=5)
    # grass sunk over the remains
    for k, (gx, gy, gr) in enumerate([(-0.28, -0.07, 0.06), (-0.08, 0.1, 0.05), (0.08, -0.1, 0.05),
                                      (0.18, 0.1, 0.055), (0.40, 0.03, 0.06), (-0.12, -0.12, 0.05),
                                      (0.33, -0.16, 0.05), (0.0, 0.14, 0.045)]):
        S.ellipsoid("tuft", x(gx), y0 + gy, 0.03, gr, gr * 0.8, 0.07, "foliage" if k % 2 else "vegetation",
                    segs=8, rings=5)
    return S.render_asset("item", "bones_%02d" % variant, zmax=0.14, extra={"item": "bones", "variant": variant})


# --- cloth, straw and rolled carpets (ART 16.3) -----------------------------

def _roll_matrix(cx, cy, r, ln, yaw):
    """Place a +Z cylinder of length `ln` lying on the ground, centred on (cx, cy),
    its axis `yaw` degrees from +X (negative turns towards the viewer)."""
    import math
    from mathutils import Matrix
    a = math.radians(yaw)
    return (Matrix.Translation((cx - math.cos(a) * ln / 2, cy - math.sin(a) * ln / 2, r))
            @ Matrix.Rotation(a, 4, "Z") @ Matrix.Rotation(math.radians(90), 4, "Y"))


def _cloth_bolt():
    """Three off-white bolts of cloth rolled on wooden cores, two below and
    one on top, each with a burlap wrapper band."""
    import math
    yaw = -38
    r, ln = 0.105, 0.56
    n = (-math.sin(math.radians(yaw)), math.cos(math.radians(yaw)))   # sideways on the ground
    for k, (off, z) in enumerate(((-0.115, r), (0.115, r), (0.0, r * 2.55))):
        n0 = len(S.objects())
        S.cyl("bolt", 0, 0, r, 0.0, ln, "cloth_white", segs=18, bevel=0.01)
        S.cyl("core", 0, 0, 0.04, ln, ln + 0.012, "wood", segs=10, bevel=0.0)
        S.cyl("layer", 0, 0, 0.075, ln, ln + 0.006, "ceramic", segs=14, bevel=0.0)
        S.cyl("band", 0, 0, r + 0.006, ln * 0.45, ln * 0.45 + 0.07, "burlap", segs=18, bevel=0.0)
        S.cyl("edge", 0, 0, r + 0.003, ln * 0.12, ln * 0.12 + 0.02, "ceramic", segs=18, bevel=0.0)
        m = _roll_matrix(0.5 + n[0] * off, -0.52 + n[1] * off, z, ln, yaw)
        for ob in S.objects()[n0:]:
            ob.matrix_world = m
    return S.render_asset("item", "cloth_bolt", zmax=0.36, extra={"item": "cloth", "stack": True})


def _straw_bundle():
    """Golden straw lying in a bundle, tied with twine."""
    import math
    import random
    from mathutils import Matrix
    rnd = random.Random(11)
    for i in range(26):
        y = -0.5 + rnd.uniform(-0.13, 0.13)
        z = 0.04 + rnd.uniform(0.0, 0.15)
        yaw = rnd.uniform(-9, 9)
        length = rnd.uniform(0.62, 0.76)
        x0 = 0.5 - length / 2 + rnd.uniform(-0.03, 0.03)
        S.cyl("stalk%d" % i, 0, 0, 0.02, 0.0, length, "straw" if i % 3 else "corn_yellow", segs=5, bevel=0.0)
        S.objects()[-1].matrix_world = Matrix.Translation((x0, y, z)) @ Matrix.Rotation(math.radians(yaw), 4, "Z") \
            @ Matrix.Rotation(math.radians(90), 4, "Y")
    for k, x in enumerate((0.40, 0.60)):
        S.box("tie%d" % k, x - 0.02, -0.66, 0.0, x + 0.02, -0.34, 0.23, "leather", bevel=0.01)
    return S.render_asset("item", "straw_bundle", zmax=0.26, extra={"item": "straw", "stack": True})


_ROLL = {
    # material -> (body, ring, core, tie)
    "straw": ("carpet_straw", "carpet_straw_dark", "carpet_straw_edge", "leather"),
    "cloth": ("carpet_cloth", "carpet_cloth_stripe", "carpet_cloth_edge", "straw"),
    "leather": ("carpet_leather", "carpet_leather_edge", "carpet_leather_edge", "straw"),
    "wood": ("carpet_wood", "carpet_wood_alt", "carpet_wood_edge", "leather"),
    "wool": ("carpet_wool", "carpet_wool_stripe", "carpet_wool_stripe", "straw"),
}


def _carpet_rolled(kind):
    """A rug rolled up lengthwise, spiral end facing the viewer, two ties."""
    import math
    from mathutils import Matrix
    body, ring, core, tie = _ROLL[kind]
    r, ln = 0.15, 0.62
    n0 = len(S.objects())
    S.cyl("roll", 0, 0, r, 0.0, ln, body, segs=20, bevel=0.01)
    # spiral end: concentric rings
    S.cyl("e1", 0, 0, r * 0.82, ln, ln + 0.008, ring, segs=18, bevel=0.0)
    S.cyl("e2", 0, 0, r * 0.62, ln, ln + 0.014, body, segs=16, bevel=0.0)
    S.cyl("e3", 0, 0, r * 0.42, ln, ln + 0.02, ring, segs=14, bevel=0.0)
    S.cyl("e4", 0, 0, r * 0.2, ln, ln + 0.026, core, segs=10, bevel=0.0)
    if kind == "cloth":   # border stripe shows as bands around the roll
        for k, t in enumerate((0.1, 0.84)):
            S.cyl("stripe%d" % k, 0, 0, r + 0.005, ln * t, ln * t + 0.05, ring, segs=20, bevel=0.0)
    elif kind == "wood":  # slat joints run round the roll
        for k in range(5):
            S.cyl("joint%d" % k, 0, 0, r + 0.004, ln * (0.12 + 0.19 * k), ln * (0.12 + 0.19 * k) + 0.014, core,
                  segs=20, bevel=0.0)
    elif kind == "straw":  # weave ridges
        for k in range(6):
            S.cyl("weave%d" % k, 0, 0, r + 0.004, ln * (0.1 + 0.15 * k), ln * (0.1 + 0.15 * k) + 0.03, ring,
                  segs=20, bevel=0.0)
    elif kind == "wool":  # dark stripe bands round the roll
        for k, t in enumerate((0.1, 0.84)):
            S.cyl("stripe%d" % k, 0, 0, r + 0.005, ln * t, ln * t + 0.05, ring, segs=20, bevel=0.0)
    else:                 # darker hide wrapping round near each end
        for k, t in enumerate((0.04, 0.9)):
            S.cyl("wrap%d" % k, 0, 0, r + 0.004, ln * t, ln * t + 0.05, ring, segs=20, bevel=0.0)
    for k, t in enumerate((0.22, 0.72)):
        S.cyl("tie%d" % k, 0, 0, r + 0.014, ln * t, ln * t + 0.045, tie, segs=20, bevel=0.0)
    for ob in S.objects()[n0:]:
        ob.matrix_world = _roll_matrix(0.5, -0.5, r, ln, -38)
    return S.render_asset("item", "carpet_%s_rolled" % kind, zmax=0.32,
                          extra={"item": "%s carpet" % kind, "stack": True})


# --- wildlife products (biome / animal husbandry) ---------------------------

def _milk():
    """A wooden pail brimming with milk, with a ceramic jug beside it."""
    S.cyl("pail", 0.40, -0.52, 0.20, 0.0, 0.26, "wood", r_top=0.24, segs=18)
    S.cyl("hoop", 0.40, -0.52, 0.215, 0.10, 0.14, "bark", segs=18, bevel=0.0)
    S.cyl("milk", 0.40, -0.52, 0.215, 0.24, 0.265, "milk", segs=18, bevel=0.0)
    S.cyl("jug", 0.70, -0.40, 0.10, 0.0, 0.26, "ceramic", r_top=0.07, segs=14)
    S.cyl("neck", 0.70, -0.40, 0.055, 0.25, 0.32, "ceramic", segs=12)
    S.cyl("jugmilk", 0.70, -0.40, 0.045, 0.31, 0.325, "milk", segs=12, bevel=0.0)
    return S.render_asset("item", "milk", zmax=0.36, extra={"item": "milk", "stack": True})


def _wool():
    """Two soft fleece bundles, tied with twine. Neutral cream: the renderer
    tints it per species."""
    for k, (x, y, z, r, sd) in enumerate([(0.38, -0.54, 0.0, 0.22, 301), (0.62, -0.50, 0.0, 0.2, 302),
                                          (0.50, -0.56, 0.16, 0.18, 303), (0.30, -0.40, 0.02, 0.12, 304)]):
        S.blob("fleece%d" % k, x, y, z, r, "wool_cream", seed=sd, lumps=0.3, squash=0.7)
    S.box("twine", 0.30, -0.54, 0.10, 0.70, -0.50, 0.14, "straw", bevel=0.01)
    return S.render_asset("item", "wool", zmax=0.34, extra={"item": "wool", "stack": True})


def _eggs(fertile):
    """A straw nest with three eggs; fertile ones are warmer, with a dark speck."""
    S.cyl("nest", 0.5, -0.52, 0.27, 0.0, 0.07, "straw", r_top=0.30, segs=18)
    S.cyl("nest_in", 0.5, -0.52, 0.17, 0.06, 0.075, "burlap", segs=16, bevel=0.0)
    mat = "egg_fertile" if fertile else "egg_shell"
    for k, (x, y) in enumerate([(0.41, -0.48), (0.60, -0.48), (0.50, -0.64)]):
        S.ellipsoid("egg%d" % k, x, y, 0.17, 0.1, 0.1, 0.13, mat, segs=14, rings=9)
        if fertile:
            S.ellipsoid("speck%d" % k, x - 0.03, y - 0.06, 0.2, 0.018, 0.012, 0.018, "wood", segs=6, rings=4)
    return S.render_asset("item", "fertile_egg" if fertile else "egg", zmax=0.32,
                          extra={"item": "fertile egg" if fertile else "egg", "stack": True})


def _ember_oil():
    """A stoppered glass flask of glowing amber oil."""
    S.ellipsoid("flask", 0.5, -0.52, 0.17, 0.18, 0.18, 0.17, "oil_amber", segs=16, rings=10)
    S.ellipsoid("glow", 0.5, -0.56, 0.2, 0.1, 0.07, 0.09, "gland_ember", segs=12, rings=8)
    S.ellipsoid("glint", 0.40, -0.66, 0.26, 0.04, 0.02, 0.05, "glass", segs=8, rings=5)
    S.cyl("neck", 0.5, -0.52, 0.06, 0.28, 0.42, "oil_amber", segs=12)
    S.cyl("cork", 0.5, -0.52, 0.055, 0.42, 0.5, "wood", segs=10, bevel=0.005)
    S.box("label", 0.34, -0.71, 0.1, 0.66, -0.69, 0.2, "ceramic", bevel=0.004)
    return S.render_asset("item", "ember_oil", zmax=0.52, extra={"item": "ember oil", "stack": True,
                                                                "emissive": "gland_ember"})


def _parka_wool():
    """A folded wool parka: cream knit with grey trim and a woolly ruff."""
    S.box("body", 0.18, -0.78, 0.0, 0.82, -0.30, 0.12, "carpet_wool", bevel=0.05, segments=2)
    S.box("fold", 0.22, -0.74, 0.12, 0.78, -0.40, 0.18, "carpet_wool", bevel=0.04)
    S.box("stripe", 0.18, -0.58, 0.0, 0.82, -0.52, 0.125, "carpet_wool_stripe", bevel=0.02)
    S.ellipsoid("hood", 0.50, -0.30, 0.14, 0.22, 0.10, 0.08, "wool_cream", segs=14, rings=8)
    for k, x in enumerate([0.36, 0.50, 0.64]):
        S.cyl("toggle%d" % k, x, -0.64, 0.018, 0.17, 0.20, "wood", segs=8, bevel=0.0)
    return S.render_asset("item", "parka_wool", zmax=0.24, extra={"item": "wool parka"})


# ---- NG-1 salvage: components and industrial medicine (small / medium / large stacks)
def _spin(n0, cx, cy, deg):
    """Rotate the objects made since index n0 about the vertical axis through (cx, cy)."""
    import math
    from mathutils import Matrix
    m = (Matrix.Translation((cx, cy, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-cx, -cy, 0)))
    for ob in S.objects()[n0:]:
        ob.matrix_world = m @ ob.matrix_world


def _board(cx, cy, z, deg, w=0.34, d=0.24):
    """A circuit board: green plate, copper traces, a dark chip, brass contacts."""
    n0 = len(S.objects())
    x0, y0 = cx - w / 2, cy - d / 2
    S.box("pcb", x0, y0, z, x0 + w, y0 + d, z + 0.03, "circuit", bevel=0.008)
    S.box("trace_a", x0 + 0.03, y0 + 0.07, z + 0.03, x0 + w * 0.62, y0 + 0.09, z + 0.036, "copper", bevel=0.0)
    S.box("trace_b", x0 + w * 0.62, y0 + 0.07, z + 0.03, x0 + w * 0.65, y0 + d - 0.04, z + 0.036, "copper", bevel=0.0)
    S.box("chip", x0 + w * 0.2, y0 + 0.11, z + 0.03, x0 + w * 0.5, y0 + d - 0.05, z + 0.07, "metal_dark", bevel=0.01)
    S.box("cap", x0 + w * 0.72, y0 + 0.05, z + 0.03, x0 + w * 0.84, y0 + 0.13, z + 0.085, "steel", bevel=0.01)
    for k in range(4):
        S.box("pad%d" % k, x0 + 0.02 + k * 0.045, y0 - 0.02, z + 0.005, x0 + 0.045 + k * 0.045, y0 + 0.02,
              z + 0.034, "brass", bevel=0.0)
    _spin(n0, cx, cy, deg)


def _gear(cx, cy, z, r=0.09, teeth=8):
    import math
    S.cyl("gear", cx, cy, r, z, z + 0.04, "steel", segs=teeth * 2, bevel=0.006)
    S.cyl("gear_hub", cx, cy, r * 0.38, z + 0.04, z + 0.06, "metal_dark", segs=10, bevel=0.004)
    for k in range(teeth):
        a = 2 * math.pi * k / teeth
        S.box("tooth%d" % k, cx + math.cos(a) * r * 1.02 - 0.014, cy + math.sin(a) * r * 1.02 - 0.014, z,
              cx + math.cos(a) * r * 1.02 + 0.014, cy + math.sin(a) * r * 1.02 + 0.014, z + 0.04, "steel",
              bevel=0.003)


def _components(size):
    """Fabricated components: circuit boards and a gear. More of each as the stack grows."""
    if size == "s":
        _board(0.50, -0.55, 0.0, 12)
        _gear(0.70, -0.40, 0.0, r=0.08)
        z = 0.12
    elif size == "m":
        _board(0.42, -0.62, 0.0, -8)
        _board(0.58, -0.50, 0.03, 20)
        _board(0.46, -0.42, 0.06, 4, w=0.30, d=0.22)
        _gear(0.76, -0.68, 0.0)
        _gear(0.24, -0.30, 0.0, r=0.07, teeth=7)
        z = 0.2
    else:
        for k, (x, y, d) in enumerate([(0.38, -0.66, -10), (0.62, -0.62, 10), (0.34, -0.40, 6), (0.60, -0.36, -14)]):
            _board(x, y, 0.0, d)
        for k, (x, y, d) in enumerate([(0.46, -0.52, 18), (0.54, -0.50, -4), (0.44, -0.46, 8)]):
            _board(x, y, 0.035 * (k + 1), d, w=0.32, d=0.23)
        _gear(0.80, -0.76, 0.0, r=0.1)
        _gear(0.22, -0.82, 0.0, r=0.08, teeth=7)
        _gear(0.82, -0.22, 0.0, r=0.07, teeth=7)
        S.cyl("spool", 0.5, -0.80, 0.07, 0.0, 0.05, "copper", segs=14, bevel=0.008)
        z = 0.3
    return S.render_asset("item", "components_" + size, zmax=z + 0.08,
                          extra={"item": "components", "stack": True, "stack_size": size})


def _vial(x, y, z, cap="med_blue"):
    S.cyl("vial", x, y, 0.045, z, z + 0.15, "glass", segs=10, bevel=0.006)
    S.cyl("fill", x, y, 0.036, z + 0.01, z + 0.09, "med_blue", segs=10, bevel=0.0)
    S.cyl("cap", x, y, 0.05, z + 0.15, z + 0.19, cap, segs=10, bevel=0.006)


def _med_box(x, y, z, deg, w=0.30, d=0.20):
    """White carton with a blue band and a plus mark (a generic plus, no cross emblem)."""
    n0 = len(S.objects())
    S.box("carton", x - w / 2, y - d / 2, z, x + w / 2, y + d / 2, z + 0.10, "med_white", bevel=0.02)
    S.box("band", x - w / 2 - 0.004, y - d / 2 - 0.004, z + 0.03, x + w / 2 + 0.004, y + d / 2 + 0.004, z + 0.055,
          "med_blue", bevel=0.0)
    S.box("plus_h", x - 0.06, y - 0.014, z + 0.10, x + 0.06, y + 0.014, z + 0.108, "med_blue", bevel=0.0)
    S.box("plus_v", x - 0.014, y - 0.06, z + 0.10, x + 0.014, y + 0.06, z + 0.108, "med_blue", bevel=0.0)
    _spin(n0, x, y, deg)


def _industrial_medicine(size):
    """Industrial medicine: sealed cartons and glass vials with blue caps."""
    if size == "s":
        _med_box(0.50, -0.58, 0.0, 8)
        _vial(0.36, -0.36, 0.0)
        _vial(0.62, -0.36, 0.0)
        z = 0.2
    elif size == "m":
        _med_box(0.40, -0.62, 0.0, -6)
        _med_box(0.60, -0.58, 0.0, 12)
        _med_box(0.50, -0.60, 0.10, 2, w=0.26, d=0.18)
        for k, x in enumerate((0.30, 0.50, 0.70)):
            _vial(x, -0.32 + 0.02 * (k % 2), 0.0)
        z = 0.3
    else:
        for k, (x, y, d) in enumerate([(0.32, -0.70, -8), (0.62, -0.72, 8), (0.34, -0.48, 10), (0.64, -0.50, -6)]):
            _med_box(x, y, 0.0, d)
        for k, (x, y, d) in enumerate([(0.46, -0.62, 6), (0.50, -0.50, -10), (0.44, -0.56, 2)]):
            _med_box(x, y, 0.10 * (k + 1), d, w=0.26, d=0.18)
        for k, x in enumerate((0.24, 0.40, 0.56, 0.72)):
            _vial(x, -0.26 + 0.02 * (k % 2), 0.0)
        z = 0.42
    return S.render_asset("item", "industrial_medicine_" + size, zmax=z + 0.05,
                          extra={"item": "industrial medicine", "stack": True, "stack_size": size})


def build():
    return [_wood(), _food(), _steel(), _stone(), _stone_chunk(), _silver(), _gold(), _berries(), _ration(), _meat(), _meal(), _club(), _rifle_item(), _knife_item(), _spear_item(),
            _potatoes(), _corn(), _wheat(), _beans(), _lettuce(), _radishes(), _pumpkins(),
            _leather(), _parka(), _tuque(), _duster(), _herbal_medicine(), _bones(0), _bones(1),
            _cloth_bolt(), _straw_bundle(), _milk(), _wool(), _eggs(False), _eggs(True), _ember_oil(), _parka_wool()] + [_carpet_rolled(k) for k in _ROLL] + build_ng1()


def build_ng1():
    return [_components(z) for z in "sml"] + [_industrial_medicine(z) for z in "sml"]
