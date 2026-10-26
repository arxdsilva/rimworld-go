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
    """A shallow woven basket heaped with plum-red berries, a leaf or two."""
    import math
    from .plant_base import _leaf
    S.cyl("basket", 0.5, -0.55, 0.25, 0.0, 0.12, "pl_dried", r_top=0.28, segs=22, bevel=0.015)
    S.cyl("weave", 0.5, -0.55, 0.265, 0.05, 0.065, "pl_stem", r_top=0.268, segs=22, bevel=0.0)
    S.cyl("rim", 0.5, -0.55, 0.285, 0.115, 0.145, "pl_stem", segs=22, bevel=0.01)
    pts = [(0.0, 0.0, 0.245)]
    pts += [(0.115 * math.cos(a), 0.115 * math.sin(a) * 0.9, 0.215) for a in [i * 2 * math.pi / 6 + 0.3 for i in range(6)]]
    pts += [(0.2 * math.cos(a), 0.2 * math.sin(a) * 0.9, 0.165) for a in [i * 2 * math.pi / 9 + 0.1 for i in range(9)]]
    for k, (dx, dy, z) in enumerate(pts):
        _ell("b%d" % k, (0.5 + dx, -0.55 + dy, z), (0.056, 0.056, 0.052), "pl_crop_berry", segs=12, rings=8)
    for k, (x, y) in enumerate([(0.24, -0.80), (0.80, -0.32)]):
        _ell("fall%d" % k, (x, y, 0.045), (0.048, 0.048, 0.046), "pl_crop_berry", segs=12, rings=8)
    _leaf("lf0", 0.60, -0.50, 0.25, 0.4, 0.15, 0.17, 0.09, "pl_crop_leaf", thick=0.03)
    _leaf("lf1", 0.42, -0.46, 0.24, 2.2, 0.15, 0.15, 0.08, "pl_crop_deep", thick=0.03)
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


# --- farm produce (PLAY-016, ART spec 9.3) ---------------------------------
# Produce keeps the item outline rule and canvas but wears the muted
# pl_crop_* palette and softer, rounder, smooth-shaded forms (no facets).

def _ell(name, c, r, mat, yaw=0.0, pitch=0.0, segs=16, rings=10):
    """Smooth ellipsoid centred on `c` with radii `r`, tipped `pitch` degrees
    (positive lifts +x) and turned `yaw` degrees about Z."""
    import math
    from mathutils import Matrix
    ob = S.ellipsoid(name, 0, 0, 0, r[0], r[1], r[2], mat, segs=segs, rings=rings)
    ob.data.transform(Matrix.Translation(c) @ Matrix.Rotation(math.radians(yaw), 4, "Z")
                      @ Matrix.Rotation(math.radians(-pitch), 4, "Y"))
    return ob


def _lumpy(name, c, r, mat, seed, amp=0.1, yaw=0.0):
    """A smooth, slightly irregular ellipsoid (a tuber, a pebble of produce)."""
    import math
    import numpy as np
    from mathutils import Matrix
    ob = S.ellipsoid(name, 0, 0, 0, r[0], r[1], r[2], mat, segs=16, rings=10)
    rng = np.random.RandomState(seed)
    dirs = rng.normal(size=(4, 3))
    dirs /= np.linalg.norm(dirs, axis=1, keepdims=True)
    amps = rng.uniform(-amp, amp, 4)
    for v in ob.data.vertices:
        n = np.array(v.co[:]) / max(v.co.length, 1e-6)
        k = 1.0 + float(np.sum(amps * np.maximum(dirs @ n, 0) ** 2))
        v.co.x *= k
        v.co.y *= k
        v.co.z *= k
    ob.data.transform(Matrix.Translation(c) @ Matrix.Rotation(math.radians(yaw), 4, "Z"))
    return ob


def _potatoes():
    spec = [(0.35, -0.58, 0.0, 0.105, 0.09, 0.075, 10, "pl_crop_tuber"),
            (0.57, -0.61, 0.0, 0.115, 0.095, 0.08, -25, "pl_crop_bronze"),
            (0.68, -0.45, 0.0, 0.095, 0.08, 0.07, 40, "pl_crop_tuber"),
            (0.45, -0.42, 0.0, 0.1, 0.085, 0.072, -10, "pl_crop_bronze"),
            (0.30, -0.42, 0.0, 0.08, 0.07, 0.06, 60, "pl_crop_tuber"),
            (0.50, -0.52, 0.09, 0.095, 0.08, 0.07, 20, "pl_crop_tuber")]
    for k, (x, y, z, rx, ry, rz, yaw, m) in enumerate(spec):
        _lumpy("potato%d" % k, (x, y, z + rz), (rx, ry, rz), m, 120 + k, amp=0.14, yaw=yaw)
    return S.render_asset("item", "potatoes", zmax=0.24, extra={"item": "potatoes", "stack": True})


def _corn():
    import math
    from .plant_base import _leaf
    for k, (x, y, z, a) in enumerate([(0.32, -0.64, 0.0, 8), (0.30, -0.46, 0.0, -8), (0.38, -0.55, 0.11, 3)]):
        # a plump cob with a pale silk tuft at its tip, husk leaves folded back from its base
        _ell("cob%d" % k, (x + 0.2, y, z + 0.075), (0.2, 0.072, 0.07), "pl_crop_corn", yaw=a)
        _ell("tip%d" % k, (x + 0.40, y, z + 0.075), (0.06, 0.05, 0.048), "pl_crop_straw", yaw=a)
        for j, da in enumerate((-0.5, 0.0, 0.5)):
            az = math.pi + math.radians(a) + da
            _leaf("husk%d_%d" % (k, j), x + 0.14, y + 0.07 * (j - 1), z + 0.06, az, 0.22, 0.24, 0.12,
                  "pl_crop_leaf" if j % 2 == 0 else "pl_crop_deep", thick=0.045)
    return S.render_asset("item", "corn_pile", zmax=0.28, extra={"item": "corn", "stack": True})


def _wheat():
    import math
    import random
    from .plant_base import _limb
    rnd = random.Random(5)
    n = 24
    for i in range(n):
        a = i * 2 * math.pi / n + rnd.uniform(-0.2, 0.2)
        spread = rnd.uniform(0.08, 0.2)
        x0, y0 = 0.5 + rnd.uniform(-0.03, 0.03), -0.5 + rnd.uniform(-0.03, 0.03)
        top = (x0 + math.cos(a) * spread, y0 + math.sin(a) * spread * 0.8, 0.44 + rnd.uniform(0.0, 0.08))
        mid = (x0 + (top[0] - x0) * 0.25, y0 + (top[1] - y0) * 0.25, 0.2)
        _limb("stem%d" % i, (x0, y0, 0.0), mid, 0.017, 0.015, "pl_crop_straw", segs=5)
        _limb("stemu%d" % i, mid, top, 0.015, 0.011, "pl_crop_straw", segs=5)
        # the ear bows outward and down
        _ell("ear%d" % i, (top[0] + math.cos(a) * 0.028, top[1] + math.sin(a) * 0.022, top[2] + 0.055),
             (0.032, 0.032, 0.09), "pl_crop_corn" if i % 3 else "pl_crop_straw", yaw=math.degrees(a), pitch=-30, segs=10, rings=6)
    S.cyl("tie", 0.5, -0.5, 0.085, 0.13, 0.21, "pl_stem", segs=16, bevel=0.01)
    return S.render_asset("item", "wheat_sheaf", zmax=0.64, extra={"item": "wheat", "stack": True})


def _beans():
    S.cyl("sack", 0.5, -0.55, 0.22, 0.0, 0.24, "burlap", r_top=0.18, segs=20, bevel=0.03)
    S.cyl("collar", 0.5, -0.55, 0.2, 0.2, 0.27, "pl_dried", segs=20, bevel=0.025)
    # plump sage pods heaped to fill the mouth, a few spilled beside the sack
    spec = [(-0.07, -0.05, 0.30, 20, 6, "pl_crop_pod"), (0.06, -0.07, 0.30, -30, 6, "pl_crop_jade"),
            (-0.01, 0.06, 0.30, 80, 5, "pl_crop_pod"), (0.09, 0.04, 0.31, 5, 10, "pl_crop_jade"),
            (-0.08, 0.04, 0.31, -50, 8, "pl_crop_jade"), (0.0, -0.01, 0.34, 35, 10, "pl_crop_pod"),
            (0.05, -0.02, 0.33, 100, 12, "pl_crop_jade"), (-0.04, -0.07, 0.31, 140, 8, "pl_crop_jade"),
            (0.0, 0.09, 0.29, 10, 4, "pl_crop_pod")]
    for k, (dx, dy, z, yaw, pitch, m) in enumerate(spec):
        _ell("pod%d" % k, (0.5 + dx, -0.55 + dy, z), (0.1, 0.034, 0.03), m, yaw=yaw, pitch=pitch, segs=10, rings=6)
    for k, (x, y, yaw) in enumerate([(0.24, -0.72, 25), (0.32, -0.80, -15), (0.76, -0.74, 60)]):
        _ell("spill%d" % k, (x, y, 0.03), (0.08, 0.03, 0.03), "pl_crop_pod" if k % 2 else "pl_crop_jade", yaw=yaw, segs=10, rings=6)
    return S.render_asset("item", "beans_sack", zmax=0.32, extra={"item": "beans", "stack": True})


def _lettuce_head(k, hx, hy, r):
    import math
    from .plant_base import _leaf
    _ell("core%d" % k, (hx, hy, r * 0.62), (r * 0.82, r * 0.82, r * 0.7), "pl_crop_jade")
    for i in range(5):    # outer flare: broad leaves resting low
        a = i * 2 * math.pi / 5 + k * 0.6
        _leaf("fl%d_%d" % (k, i), hx + math.cos(a) * r * 0.3, hy + math.sin(a) * r * 0.3, 0.01, a,
              math.radians(14), r * 1.9, r * 1.5, "pl_crop_leaf", thick=0.05)
    for i in range(6):    # wrapper leaves cupping the head
        a = i * 2 * math.pi / 6 + k * 0.6 + 0.5
        _leaf("wr%d_%d" % (k, i), hx + math.cos(a) * r * 0.5, hy + math.sin(a) * r * 0.5, 0.03, a,
              math.radians(64), r * 1.7, r * 1.2, "pl_crop_jade", thick=0.05)


def _lettuce():
    _lettuce_head(0, 0.37, -0.56, 0.13)
    _lettuce_head(1, 0.63, -0.52, 0.12)
    return S.render_asset("item", "lettuce_heads", zmax=0.28, extra={"item": "lettuce", "stack": True})


def _radishes():
    import math
    from .plant_base import _leaf, _limb
    # four radishes fanned out, roots toward the viewer, leaves trailing away
    for k, (x, y, a) in enumerate([(0.40, -0.58, 225), (0.50, -0.55, 250), (0.60, -0.58, 290), (0.47, -0.50, 205)]):
        ar = math.radians(a)
        dx, dy = math.cos(ar), math.sin(ar)
        _ell("root%d" % k, (x + dx * 0.04, y + dy * 0.04, 0.055), (0.09, 0.056, 0.055), "pl_crop_radish", yaw=a)
        _limb("tail%d" % k, (x + dx * 0.11, y + dy * 0.11, 0.045), (x + dx * 0.19, y + dy * 0.19, 0.02), 0.026, 0.007,
              "pl_crop_radish", segs=6)
        for j, da in enumerate((-0.5, 0.0, 0.5)):
            _leaf("lf%d_%d" % (k, j), x - dx * 0.03, y - dy * 0.03, 0.07, ar + math.pi + da, math.radians(16 + 8 * j),
                  0.21, 0.09, "pl_crop_jade" if (j + k) % 2 else "pl_crop_leaf", thick=0.03)
    _ell("twine", (0.52, -0.52, 0.07), (0.035, 0.1, 0.045), "pl_stem", yaw=45, segs=10, rings=6)
    return S.render_asset("item", "radish_bunch", zmax=0.18, extra={"item": "radishes", "stack": True})


def _pumpkin(k, x, y, r, rot):
    import math
    from .plant_base import _limb
    rz = r * 0.74
    _ell("core%d" % k, (x, y, rz), (r * 0.75, r * 0.75, rz * 0.97), "pl_crop_pumpkin")
    for i in range(7):    # soft ribs: overlapping lobes round a core
        a = rot + i * 2 * math.pi / 7
        _ell("rib%d_%d" % (k, i), (x + math.cos(a) * r * 0.4, y + math.sin(a) * r * 0.4, rz),
             (r * 0.58, r * 0.58, rz), "pl_crop_pumpkin", segs=14, rings=9)
    _limb("stem%d" % k, (x, y, rz * 1.88), (x + 0.02, y + 0.01, rz * 1.88 + 0.07), 0.026, 0.016, "pl_crop_stem", segs=6)


def _pumpkins():
    _pumpkin(1, 0.66, -0.46, 0.13, 0.3)
    _pumpkin(0, 0.40, -0.60, 0.17, 0.0)
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
    """Herbal medicine: two bundles of dried healroot, a little cluster of pale
    blue-white blooms at each tip, bound with twine; a few knobbly roots."""
    import math
    from .plant_base import _leaf
    for k, (bx, by, az0) in enumerate([(0.18, -0.64, 0.1), (0.26, -0.40, 0.3)]):
        for j, da in enumerate((-0.35, -0.17, 0.0, 0.17, 0.35)):
            az = az0 + da
            ln = 0.36 - 0.04 * (j % 2)
            _leaf("hb%d_%d" % (k, j), bx, by, 0.02 + 0.012 * (j % 2), az, 0.06, ln, 0.065,
                  "pl_crop_pod" if j % 2 else "pl_crop_silver", thick=0.035)
        tx, ty = bx + math.cos(az0) * 0.36, by + math.sin(az0) * 0.36
        for j, (ox, oy) in enumerate([(0.0, 0.0), (-0.035, 0.03), (-0.03, -0.035)]):
            _ell("fl%d_%d" % (k, j), (tx + ox, ty + oy, 0.05 + 0.01 * j), (0.032, 0.03, 0.028), "pl_crop_bloom", segs=10, rings=6)
        _ell("tw%d" % k, (bx + math.cos(az0) * 0.08, by + math.sin(az0) * 0.08, 0.04), (0.035, 0.08, 0.05), "pl_stem",
             yaw=math.degrees(az0), segs=12, rings=6)
    for k, (x, y) in enumerate([(0.62, -0.74), (0.76, -0.62)]):
        _lumpy("root%d" % k, (x, y, 0.04), (0.07, 0.05, 0.04), "pl_crop_tuber", 120 + k, amp=0.25, yaw=30 * k)
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
    """Three warm-cream cotton bolts rolled on wooden cores, two below and
    one on top, each with a straw-coloured wrapper band."""
    import math
    yaw = -38
    r, ln = 0.105, 0.56
    n = (-math.sin(math.radians(yaw)), math.cos(math.radians(yaw)))   # sideways on the ground
    for k, (off, z) in enumerate(((-0.115, r), (0.115, r), (0.0, r * 2.55))):
        n0 = len(S.objects())
        S.cyl("bolt", 0, 0, r, 0.0, ln, "pl_crop_cotton", segs=24, bevel=0.03)
        S.cyl("core", 0, 0, 0.04, ln, ln + 0.012, "pl_stem", segs=10, bevel=0.0)
        S.cyl("layer", 0, 0, 0.075, ln, ln + 0.006, "pl_crop_bloom", segs=16, bevel=0.0)
        S.cyl("band", 0, 0, r + 0.006, ln * 0.45, ln * 0.45 + 0.07, "pl_crop_straw", segs=24, bevel=0.0)
        S.cyl("edge", 0, 0, r + 0.003, ln * 0.12, ln * 0.12 + 0.02, "pl_crop_bloom", segs=24, bevel=0.0)
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


# ---- social & host systems items (ENG-003, ART §37)
def _brewgrain_item():
    """A squat burlap sack of malted grain, heaped with bronze kernels, a few
    loose kernels beside it."""
    import math
    S.cyl("sack", 0.5, -0.55, 0.25, 0.0, 0.2, "burlap", r_top=0.2, segs=22, bevel=0.03)
    S.cyl("collar", 0.5, -0.55, 0.2, 0.17, 0.235, "pl_dried", segs=22, bevel=0.025)
    _ell("grain", (0.5, -0.55, 0.225), (0.17, 0.16, 0.085), "pl_crop_bronze")
    for k in range(7):    # kernels studding the heap
        a = k * 2 * math.pi / 7
        _ell("hk%d" % k, (0.5 + math.cos(a) * 0.09, -0.55 + math.sin(a) * 0.08, 0.3 - 0.01 * (k % 2)),
             (0.04, 0.026, 0.024), "pl_crop_straw" if k % 2 else "pl_crop_bronze", yaw=math.degrees(a) + 40, segs=8, rings=5)
    for k, (x, y) in enumerate([(0.26, -0.78), (0.34, -0.83), (0.72, -0.80), (0.80, -0.72), (0.76, -0.86)]):
        _ell("kern%d" % k, (x, y, 0.025), (0.034, 0.02, 0.02), "pl_crop_straw" if k % 2 else "pl_crop_bronze",
             yaw=k * 37, segs=8, rings=5)
    return S.render_asset("item", "brewgrain", zmax=0.34, extra={"item": "brewgrain", "stack": True})


def _ashleaf_item():
    """A loose bundle of broad silver-sage leaves tied with twine."""
    import math
    from .plant_base import _leaf
    for k, (da, ln, pitch, z) in enumerate([(-0.85, 0.34, 0.10, 0.02), (-0.52, 0.42, 0.14, 0.045), (-0.18, 0.37, 0.08, 0.03),
                                             (0.16, 0.43, 0.16, 0.055), (0.5, 0.36, 0.1, 0.035), (0.85, 0.38, 0.12, 0.02),
                                             (0.0, 0.4, 0.3, 0.1)]):
        _leaf("lf%d" % k, 0.26, -0.58, z, 0.35 + da, pitch, ln, 0.17, "pl_crop_jade" if k % 2 else "pl_crop_silver", thick=0.045)
    _ell("twine", (0.31, -0.57, 0.06), (0.035, 0.1, 0.06), "pl_stem", yaw=20, segs=12, rings=6)
    return S.render_asset("item", "ashleaf", zmax=0.2, extra={"item": "ashleaf", "stack": True})


def _beer():
    """A wooden keg tapped for a foaming ceramic tankard."""
    S.cyl("keg", 0.38, -0.5, 0.2, 0.0, 0.34, "wood", r_top=0.2, segs=18, bevel=0.03)
    for z in (0.06, 0.26):
        S.cyl("hoop%d" % int(z * 10), 0.38, -0.5, 0.207, z, z + 0.04, "metal_dark", segs=18, bevel=0.0)
    S.cyl("tap", 0.38, -0.72, 0.025, 0.14, 0.2, "steel", segs=8, ry=0.07, bevel=0.0)
    S.cyl("mug", 0.72, -0.6, 0.1, 0.0, 0.22, "ceramic", segs=16, bevel=0.015)
    S.cyl("ale", 0.72, -0.6, 0.085, 0.2, 0.215, "crop_fruit", segs=16, bevel=0.0)
    S.ellipsoid("foam", 0.72, -0.6, 0.225, 0.09, 0.09, 0.04, "snow", segs=12, rings=6)
    S.box("handle", 0.82, -0.62, 0.06, 0.88, -0.58, 0.18, "ceramic", bevel=0.01)
    return S.render_asset("item", "beer", zmax=0.4, extra={"item": "beer", "stack": True})


def _ashleaf_tea():
    """A steaming cup of pale tea on a saucer, one leaf floating beside it."""
    from .plant_base import _leaf
    S.cyl("saucer", 0.5, -0.55, 0.26, 0.0, 0.03, "ceramic", segs=22, bevel=0.005)
    S.cyl("cup", 0.5, -0.55, 0.14, 0.03, 0.17, "ceramic", r_top=0.17, segs=18, bevel=0.012)
    S.cyl("tea", 0.5, -0.55, 0.15, 0.165, 0.175, "leaf_light", segs=18, bevel=0.0)
    S.box("handle", 0.66, -0.57, 0.07, 0.74, -0.53, 0.14, "ceramic", bevel=0.01)
    S.ellipsoid("steam", 0.5, -0.55, 0.26, 0.05, 0.04, 0.07, "snow", segs=8, rings=5)
    S.ellipsoid("steam2", 0.54, -0.52, 0.36, 0.035, 0.03, 0.05, "snow", segs=8, rings=5)
    _leaf("lf", 0.22, -0.74, 0.02, 0.5, 0.1, 0.2, 0.09, "leaf_pine_hi", thick=0.03)
    return S.render_asset("item", "ashleaf_tea", zmax=0.42, extra={"item": "ashleaf tea", "stack": True})


def _spark_tablet():
    """A foil strip of chalky tablets, each stamped with a small ember bolt."""
    S.box("strip", 0.24, -0.72, 0.0, 0.76, -0.38, 0.03, "steel", bevel=0.012)
    for k, (x, y) in enumerate([(0.34, -0.64), (0.50, -0.64), (0.66, -0.64), (0.34, -0.46), (0.50, -0.46), (0.66, -0.46)]):
        S.cyl("tab%d" % k, x, y, 0.062, 0.03, 0.075, "ceramic" if k < 5 else "glass", segs=12, bevel=0.01)
        S.box("bolt%d" % k, x - 0.012, y - 0.03, 0.075, x + 0.012, y + 0.03, 0.082, "pumpkin", bevel=0.0)
    return S.render_asset("item", "spark_tablet", zmax=0.12, extra={"item": "spark tablet", "stack": True})


def _peg_leg():
    """A rough wooden leg: tapered shaft, iron-capped foot, leather strap cuff."""
    from .plant_base import _limb
    _limb("shaft", (0.28, -0.62, 0.07), (0.72, -0.44, 0.1), 0.05, 0.035, "wood", segs=10)
    _limb("foot", (0.24, -0.64, 0.05), (0.3, -0.61, 0.06), 0.055, 0.05, "metal_dark", segs=10)
    S.cyl("cuff", 0.72, -0.44, 0.07, 0.07, 0.14, "leather", r_top=0.085, segs=12, bevel=0.01)
    for k, f in enumerate((0.5, 0.64)):
        S.box("strap%d" % k, 0.28 + 0.44 * f - 0.02, -0.62 + 0.18 * f - 0.07, 0.05, 0.28 + 0.44 * f + 0.02, -0.62 + 0.18 * f + 0.07, 0.13, "leather", bevel=0.008)
    return S.render_asset("item", "peg_leg", zmax=0.2, extra={"item": "peg leg"})


def _prosthetic_leg():
    """A metal prosthetic leg: steel shin and thigh sockets, a hinge, a cushioned foot."""
    from .plant_base import _limb
    _limb("shin", (0.26, -0.66, 0.08), (0.52, -0.54, 0.1), 0.04, 0.034, "steel", segs=10)
    _limb("thigh", (0.54, -0.53, 0.1), (0.78, -0.42, 0.12), 0.05, 0.058, "steel", segs=10)
    S.cyl("hinge", 0.53, -0.535, 0.055, 0.04, 0.15, "metal_dark", segs=12, bevel=0.008)
    S.box("foot", 0.14, -0.78, 0.0, 0.34, -0.58, 0.07, "metal_dark", bevel=0.02)
    S.box("pad", 0.16, -0.76, 0.07, 0.32, -0.62, 0.095, "leather", bevel=0.01)
    S.cyl("socket", 0.78, -0.42, 0.075, 0.06, 0.18, "leather", r_top=0.09, segs=12, bevel=0.01)
    return S.render_asset("item", "prosthetic_leg", zmax=0.22, extra={"item": "prosthetic leg"})


def _prosthetic_arm():
    """A metal prosthetic arm: socket, steel forearm, hinged elbow and a three-finger grip."""
    from .plant_base import _limb
    S.cyl("socket", 0.76, -0.4, 0.08, 0.05, 0.17, "leather", r_top=0.1, segs=12, bevel=0.01)
    _limb("upper", (0.74, -0.42, 0.1), (0.54, -0.52, 0.1), 0.05, 0.04, "steel", segs=10)
    S.cyl("elbow", 0.53, -0.525, 0.05, 0.04, 0.15, "metal_dark", segs=12, bevel=0.008)
    _limb("fore", (0.52, -0.53, 0.1), (0.3, -0.62, 0.09), 0.04, 0.034, "steel", segs=10)
    S.box("palm", 0.18, -0.7, 0.04, 0.32, -0.55, 0.13, "metal_dark", bevel=0.02)
    for k, dy in enumerate((-0.68, -0.62, -0.56)):
        _limb("fing%d" % k, (0.2, dy, 0.07), (0.1, dy - 0.04, 0.06), 0.02, 0.014, "steel", segs=6)
    return S.render_asset("item", "prosthetic_arm", zmax=0.2, extra={"item": "prosthetic arm"})


def _wooden_hand():
    """A carved wooden hand strapped to a leather cuff."""
    from .plant_base import _limb
    S.cyl("cuff", 0.7, -0.48, 0.09, 0.0, 0.13, "leather", r_top=0.1, segs=14, bevel=0.012)
    _limb("wrist", (0.7, -0.48, 0.06), (0.52, -0.54, 0.06), 0.055, 0.05, "wood", segs=10)
    S.box("palm", 0.36, -0.64, 0.0, 0.54, -0.44, 0.08, "wood", bevel=0.025)
    for k, dy in enumerate((-0.62, -0.57, -0.52, -0.47)):
        _limb("fing%d" % k, (0.38, dy, 0.045), (0.2, dy - 0.03 * (k - 1.5), 0.04), 0.022, 0.016, "wood_plank", segs=6)
    _limb("thumb", (0.44, -0.45, 0.045), (0.34, -0.3, 0.04), 0.024, 0.017, "wood_plank", segs=6)
    S.box("strap", 0.6, -0.56, 0.0, 0.64, -0.4, 0.12, "bark", bevel=0.008)
    return S.render_asset("item", "wooden_hand", zmax=0.16, extra={"item": "wooden hand"})


def _denture():
    """A set of dentures: pink gum arches with rows of ceramic teeth, one over the other."""
    S.box("lower_gum", 0.28, -0.68, 0.0, 0.72, -0.44, 0.06, "food_red", bevel=0.04, segments=2)
    S.box("upper_gum", 0.3, -0.58, 0.1, 0.7, -0.36, 0.17, "food_red", bevel=0.04, segments=2)
    for k in range(7):
        x = 0.31 + 0.063 * k
        S.box("lt%d" % k, x, -0.69, 0.06, x + 0.05, -0.64, 0.12, "ceramic", bevel=0.012)
        S.box("ut%d" % k, x + 0.01, -0.59, 0.06, x + 0.06, -0.54, 0.1, "ceramic", bevel=0.012)
    return S.render_asset("item", "denture", zmax=0.2, extra={"item": "denture"})


def _hearing_trumpet():
    """A flared horn trumpet with a leather-wrapped mouthpiece."""
    from .plant_base import _limb
    _limb("bell", (0.2, -0.7, 0.14), (0.52, -0.54, 0.1), 0.15, 0.05, "horn", segs=16)
    _limb("tube", (0.52, -0.54, 0.1), (0.76, -0.42, 0.08), 0.05, 0.03, "horn", segs=12)
    S.cyl("rim", 0.2, -0.7, 0.155, 0.13, 0.15, "leather", segs=16, bevel=0.0)
    S.cyl("tip", 0.78, -0.41, 0.04, 0.06, 0.1, "leather", segs=10, bevel=0.005)
    S.box("wrap", 0.56, -0.56, 0.04, 0.6, -0.48, 0.16, "leather", bevel=0.008)
    return S.render_asset("item", "hearing_trumpet", zmax=0.3, extra={"item": "hearing trumpet"})


def _prosthetic_foot():
    """A carved wooden foot: heel block, toe wedge and a leather ankle strap."""
    from .plant_base import _limb
    S.box("sole", 0.2, -0.7, 0.0, 0.5, -0.5, 0.07, "wood", bevel=0.03)
    S.box("toe", 0.14, -0.67, 0.0, 0.24, -0.53, 0.05, "wood_plank", bevel=0.025)
    _limb("ankle", (0.46, -0.6, 0.07), (0.62, -0.54, 0.1), 0.055, 0.045, "wood", segs=10)
    S.cyl("cuff", 0.62, -0.54, 0.07, 0.09, 0.15, "leather", r_top=0.08, segs=12, bevel=0.01)
    S.box("strap", 0.38, -0.69, 0.07, 0.43, -0.51, 0.1, "leather", bevel=0.008)
    return S.render_asset("item", "prosthetic_foot", zmax=0.18, extra={"item": "prosthetic foot"})


def _glass_eye():
    """A small glass eyeball: white sphere, blue iris and a dark pupil."""
    S.ellipsoid("ball", 0.5, -0.55, 0.1, 0.11, 0.11, 0.1, "ceramic", segs=16, rings=10)
    S.ellipsoid("iris", 0.5, -0.55, 0.19, 0.055, 0.055, 0.02, "glass", segs=12, rings=6)
    S.ellipsoid("pupil", 0.5, -0.55, 0.205, 0.025, 0.025, 0.012, "metal_dark", segs=8, rings=4)
    return S.render_asset("item", "glass_eye", zmax=0.22, extra={"item": "glass eye"})


def _wooden_nose():
    """A small carved wooden nose with two nostrils and a leather tie."""
    S.ellipsoid("bridge", 0.5, -0.5, 0.06, 0.07, 0.15, 0.06, "wood", segs=12, rings=8)
    S.ellipsoid("tip", 0.5, -0.62, 0.07, 0.1, 0.07, 0.07, "wood", segs=12, rings=8)
    for k, dx in enumerate((-0.04, 0.04)):
        S.ellipsoid("nos%d" % k, 0.5 + dx, -0.66, 0.1, 0.022, 0.02, 0.015, "metal_dark", segs=6, rings=4)
    S.box("tie", 0.3, -0.52, 0.0, 0.7, -0.49, 0.025, "leather", bevel=0.006)
    return S.render_asset("item", "wooden_nose", zmax=0.16, extra={"item": "wooden nose"})


# ---- MED-05 medical tier (ART-026): nanomed kit, servo bionics, organs in sterile jars.
# Bionics share one look: dark plated metal_dark bodies, steel joints and a
# single med_teal accent; natural organs sit in a squat glass jar of sterile
# fluid with the lid set beside it, so they read as stored goods, not gore.
def _tube(name, cx, cy, r_out, r_in, z0, z1, mat, segs=24):
    """An open-topped ring wall (a jar's glass side), smooth shaded."""
    import math
    import bmesh
    bm = bmesh.new()
    rings = []
    for r, z in ((r_out, z0), (r_out, z1), (r_in, z1), (r_in, z0)):
        rings.append([bm.verts.new((cx + r * math.cos(2 * math.pi * k / segs),
                                    cy + r * math.sin(2 * math.pi * k / segs), z)) for k in range(segs)])
    for a in range(4):
        A, B = rings[a], rings[(a + 1) % 4]
        for k in range(segs):
            k2 = (k + 1) % segs
            bm.faces.new((A[k], A[k2], B[k2], B[k]))
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    return S._obj(name, bm, mat, smooth=True)


def _nanomed_kit():
    """A compact sealed med case: dark rounded shell, a steel lid split by a
    glowing teal seam, a white panel with a teal plus and a carry grip."""
    S.box("shell", 0.22, -0.76, 0.0, 0.78, -0.36, 0.15, "metal_dark", bevel=0.045, segments=2)
    S.box("seam", 0.215, -0.765, 0.135, 0.785, -0.355, 0.158, "med_teal_glow", bevel=0.01)
    S.box("lid", 0.23, -0.75, 0.15, 0.77, -0.37, 0.205, "steel", bevel=0.035, segments=2)
    S.box("panel", 0.38, -0.66, 0.2, 0.62, -0.46, 0.215, "med_white", bevel=0.02)
    S.box("plus_h", 0.44, -0.578, 0.215, 0.56, -0.542, 0.222, "med_teal_glow", bevel=0.0)
    S.box("plus_v", 0.482, -0.62, 0.215, 0.518, -0.50, 0.222, "med_teal_glow", bevel=0.0)
    for k, x in enumerate((0.30, 0.70)):
        S.box("latch%d" % k, x - 0.035, -0.775, 0.06, x + 0.035, -0.745, 0.17, "steel_dark", bevel=0.008)
    S.box("grip", 0.40, -0.40, 0.205, 0.60, -0.36, 0.235, "steel_dark", bevel=0.012)
    return S.render_asset("item", "nanomed_kit", zmax=0.24, extra={"item": "nanomed kit", "stack": True})


def _servo_arm():
    """A servo arm: a teal-ringed shoulder cap, dark plated upper arm and
    forearm on a steel elbow, a teal accent strip and a jointed steel hand."""
    from .plant_base import _limb
    S.cyl("shoulder", 0.80, -0.36, 0.085, 0.04, 0.17, "metal_dark", segs=16, bevel=0.015)
    S.cyl("ring", 0.80, -0.36, 0.09, 0.15, 0.185, "med_teal", segs=16, bevel=0.0)
    _limb("upper", (0.78, -0.38, 0.1), (0.55, -0.50, 0.1), 0.06, 0.05, "metal_dark", segs=12)
    S.ellipsoid("elbow", 0.54, -0.505, 0.1, 0.06, 0.06, 0.06, "steel", segs=14, rings=8)
    _limb("fore", (0.53, -0.51, 0.1), (0.30, -0.62, 0.085), 0.05, 0.036, "metal_dark", segs=12)
    _limb("strip", (0.50, -0.52, 0.145), (0.34, -0.60, 0.122), 0.014, 0.012, "med_teal", segs=6)
    S.ellipsoid("wrist", 0.29, -0.625, 0.085, 0.04, 0.04, 0.04, "steel", segs=12, rings=6)
    S.box("palm", 0.16, -0.72, 0.03, 0.29, -0.57, 0.11, "metal_dark", bevel=0.025)
    for k, dy in enumerate((-0.70, -0.65, -0.60)):
        _limb("fing%d" % k, (0.17, dy + 0.005, 0.07), (0.07, dy - 0.025, 0.055), 0.018, 0.013, "steel", segs=6)
    _limb("thumb", (0.24, -0.57, 0.07), (0.20, -0.48, 0.06), 0.018, 0.013, "steel", segs=6)
    return S.render_asset("item", "servo_arm", zmax=0.2, extra={"item": "servo arm"})


def _servo_leg():
    """A servo leg: dark plated thigh and shin on a steel knee with a teal
    ring, a sprung steel ankle and a flat dark foot blade."""
    from .plant_base import _limb
    S.cyl("hip", 0.80, -0.36, 0.08, 0.05, 0.18, "metal_dark", segs=16, bevel=0.015)
    S.cyl("hip_ring", 0.80, -0.36, 0.085, 0.16, 0.19, "med_teal", segs=16, bevel=0.0)
    _limb("thigh", (0.78, -0.38, 0.11), (0.55, -0.50, 0.11), 0.07, 0.055, "metal_dark", segs=12)
    S.cyl("knee", 0.54, -0.505, 0.06, 0.05, 0.17, "steel", segs=14, bevel=0.012)
    S.cyl("knee_ring", 0.54, -0.505, 0.064, 0.15, 0.172, "med_teal", segs=14, bevel=0.0)
    _limb("shin", (0.53, -0.51, 0.1), (0.31, -0.63, 0.08), 0.05, 0.034, "metal_dark", segs=12)
    _limb("spring", (0.36, -0.60, 0.11), (0.30, -0.635, 0.1), 0.026, 0.026, "steel", segs=8)
    S.box("foot", 0.12, -0.80, 0.0, 0.33, -0.60, 0.06, "metal_dark", bevel=0.025)
    S.box("toe", 0.12, -0.80, 0.055, 0.20, -0.62, 0.075, "med_teal", bevel=0.01)
    return S.render_asset("item", "servo_leg", zmax=0.22, extra={"item": "servo leg"})


def _optic_implant():
    """An optic implant: a dark lens module with a big glass lens, a teal
    glowing ring and a dark aperture, two copper leads trailing off."""
    from .plant_base import _limb
    S.cyl("housing", 0.46, -0.56, 0.17, 0.0, 0.11, "metal_dark", r_top=0.15, segs=24, bevel=0.025)
    S.cyl("bezel", 0.46, -0.56, 0.135, 0.11, 0.14, "steel", segs=24, bevel=0.01)
    S.cyl("ring", 0.46, -0.56, 0.11, 0.14, 0.15, "med_teal_glow", segs=24, bevel=0.0)
    S.ellipsoid("lens", 0.46, -0.56, 0.15, 0.09, 0.09, 0.04, "glass", segs=20, rings=8)
    S.ellipsoid("pupil", 0.46, -0.56, 0.185, 0.035, 0.035, 0.012, "backdrop", segs=12, rings=4)
    for k, (dy, ex, ey) in enumerate(((-0.05, 0.82, -0.62), (0.03, 0.80, -0.48))):
        _limb("lead%d" % k, (0.6, -0.56 + dy, 0.04), (ex, ey, 0.02), 0.016, 0.014, "copper", segs=6)
        S.box("pin%d" % k, ex - 0.02, ey - 0.02, 0.0, ex + 0.04, ey + 0.02, 0.035, "brass", bevel=0.006)
    return S.render_asset("item", "optic_implant", zmax=0.2, extra={"item": "optic implant"})


def _cochlear_implant():
    """A cochlear implant: a small dark module curved like the back of an
    ear, a steel receiver disc with a teal light, and a coiled copper lead."""
    import math
    from .plant_base import _limb
    pts = [(0.5 + 0.2 * math.cos(math.radians(a)), -0.56 + 0.2 * math.sin(math.radians(a))) for a in range(110, 291, 30)]
    for k in range(len(pts) - 1):
        r = 0.055 - 0.004 * k
        _limb("arc%d" % k, (pts[k][0], pts[k][1], 0.06), (pts[k + 1][0], pts[k + 1][1], 0.06), r, r - 0.004,
              "metal_dark", segs=12)
    S.cyl("disc", 0.62, -0.48, 0.1, 0.0, 0.07, "steel", segs=20, bevel=0.015)
    S.cyl("coil", 0.62, -0.48, 0.07, 0.07, 0.085, "metal_dark", segs=20, bevel=0.0)
    S.cyl("led", 0.62, -0.48, 0.03, 0.085, 0.095, "med_teal_glow", segs=12, bevel=0.0)
    lead = [(0.6, -0.72), (0.66, -0.76), (0.72, -0.72), (0.76, -0.78), (0.82, -0.74)]
    for k in range(len(lead) - 1):
        _limb("lead%d" % k, (lead[k][0], lead[k][1], 0.02), (lead[k + 1][0], lead[k + 1][1], 0.02), 0.015, 0.015,
              "copper", segs=6)
    return S.render_asset("item", "cochlear_implant", zmax=0.14, extra={"item": "cochlear implant"})


def _spinal_brace():
    """A spinal brace: a steel rod rail threaded through seven dark plated
    vertebra blocks with steel side clamps, a teal-capped top block."""
    from .plant_base import _limb
    p0, p1 = (0.22, -0.78), (0.78, -0.30)
    _limb("rod", (p0[0], p0[1], 0.07), (p1[0], p1[1], 0.07), 0.025, 0.025, "steel", segs=8)
    n0 = len(S.objects())
    for k in range(7):
        t = k / 6.0
        x, y = p0[0] + (p1[0] - p0[0]) * t, p0[1] + (p1[1] - p0[1]) * t
        w = 0.075 - 0.012 * t
        mat = "med_teal" if k == 6 else "metal_dark"
        S.box("vert%d" % k, x - w, y - 0.03, 0.02, x + w, y + 0.03, 0.12, mat, bevel=0.018)
        S.box("clamp%d" % k, x - w - 0.03, y - 0.018, 0.04, x + w + 0.03, y + 0.018, 0.09, "steel", bevel=0.01)
    import math
    from mathutils import Matrix
    deg = math.degrees(math.atan2(p1[1] - p0[1], p1[0] - p0[0])) - 90
    # turn each block across the rail about its own centre
    for k, ob in enumerate(S.objects()[n0:]):
        t = (k // 2) / 6.0
        x, y = p0[0] + (p1[0] - p0[0]) * t, p0[1] + (p1[1] - p0[1]) * t
        ob.matrix_world = (Matrix.Translation((x, y, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
                           @ Matrix.Translation((-x, -y, 0))) @ ob.matrix_world
    return S.render_asset("item", "spinal_brace", zmax=0.14, extra={"item": "spinal brace"})


def _pump_heart():
    """A pump heart: a squat dark canister with steel end caps, a glowing teal
    band and two steel vessel ports angled out of the top."""
    from .plant_base import _limb
    S.cyl("body", 0.48, -0.58, 0.16, 0.0, 0.2, "metal_dark", segs=24, bevel=0.03)
    S.cyl("base", 0.48, -0.58, 0.17, 0.0, 0.04, "steel", segs=24, bevel=0.012)
    S.cyl("band", 0.48, -0.58, 0.165, 0.1, 0.125, "med_teal_glow", segs=24, bevel=0.0)
    S.cyl("cap", 0.48, -0.58, 0.14, 0.2, 0.24, "steel", r_top=0.11, segs=24, bevel=0.015)
    _tube("cap_ring", 0.48, -0.58, 0.1, 0.07, 0.24, 0.25, "med_teal_glow", segs=24)
    _limb("port_a", (0.44, -0.58, 0.23), (0.34, -0.50, 0.33), 0.04, 0.035, "steel", segs=10)
    _limb("port_b", (0.53, -0.56, 0.23), (0.62, -0.46, 0.31), 0.035, 0.03, "steel", segs=10)
    S.cyl("rim_a", 0.34, -0.50, 0.04, 0.32, 0.345, "metal_dark", segs=10, bevel=0.0)
    for k, x in enumerate((0.36, 0.48, 0.60)):
        S.box("rib%d" % k, x - 0.012, -0.745, 0.05, x + 0.012, -0.725, 0.18, "steel_dark", bevel=0.0)
    return S.render_asset("item", "pump_heart", zmax=0.36, extra={"item": "pump heart"})


def _synth_stomach():
    """A synth stomach: a pale pouch-shaped synthetic organ with teal seam
    ribs and steel port collars at both ends."""
    import math
    from .plant_base import _limb
    _ell("pouch", (0.48, -0.56, 0.1), (0.22, 0.14, 0.1), "med_white", yaw=-25, segs=20, rings=12)
    _ell("lobe", (0.34, -0.50, 0.11), (0.1, 0.09, 0.08), "med_white", yaw=-25, segs=16, rings=10)
    for k, f in enumerate((-0.5, 0.0, 0.5)):
        a = math.radians(-25)
        x, y = 0.48 + math.cos(a) * 0.2 * f, -0.56 + math.sin(a) * 0.2 * f
        _ell("rib%d" % k, (x, y, 0.1 + 0.005 * (1 - abs(f))), (0.03, 0.145 - 0.04 * abs(f), 0.11 - 0.025 * abs(f)),
             "med_teal", yaw=-25, segs=12, rings=8)
    _limb("port_in", (0.28, -0.44, 0.15), (0.22, -0.36, 0.2), 0.04, 0.035, "steel", segs=10)
    _limb("port_out", (0.68, -0.66, 0.08), (0.80, -0.70, 0.07), 0.035, 0.03, "steel", segs=10)
    return S.render_asset("item", "synth_stomach", zmax=0.24, extra={"item": "synth stomach"})


def _heart_body(name, cx, cy, z, s, mat):
    """A plain heart form: two rounded lobes over a tapering tip (pointing south-east)."""
    _ell(name + "_l", (cx - 0.06 * s, cy + 0.03 * s, z), (0.1 * s, 0.09 * s, 0.085 * s), mat, segs=16, rings=10)
    _ell(name + "_r", (cx + 0.06 * s, cy + 0.04 * s, z), (0.1 * s, 0.09 * s, 0.085 * s), mat, segs=16, rings=10)
    _ell(name + "_tip", (cx + 0.02 * s, cy - 0.06 * s, z - 0.01 * s), (0.1 * s, 0.11 * s, 0.07 * s), mat,
         yaw=-20, segs=16, rings=10)


def _valve_heart():
    """A valve heart: a plain steel heart with a brass valve set in its top,
    a dark slit across the valve, and a short steel stub."""
    from .plant_base import _limb
    _heart_body("hb", 0.5, -0.56, 0.1, 1.3, "steel")
    S.cyl("valve", 0.47, -0.52, 0.065, 0.16, 0.2, "brass", segs=18, bevel=0.01)
    S.box("slit", 0.42, -0.528, 0.2, 0.52, -0.512, 0.206, "metal_dark", bevel=0.0)
    _limb("stub", (0.6, -0.48, 0.15), (0.68, -0.40, 0.2), 0.035, 0.03, "steel", segs=10)
    S.cyl("stub_rim", 0.68, -0.40, 0.036, 0.19, 0.215, "metal_dark", segs=10, bevel=0.0)
    return S.render_asset("item", "valve_heart", zmax=0.24, extra={"item": "valve heart"})


JAR = (0.5, -0.56, 0.25)   # centre x, y and outer radius of a specimen jar


def _jar(organ):
    """A squat glass specimen jar of sterile fluid with a teal seal ring
    round its open rim and a white label on the front."""
    import math
    x, y, r = JAR
    S.cyl("fluid", x, y, r - 0.012, 0.0, 0.1, "specimen_fluid", segs=28, bevel=0.0)
    _tube("glass", x, y, r, r - 0.018, 0.0, 0.12, "glass", segs=28)
    _tube("seal", x, y, r + 0.008, r - 0.022, 0.12, 0.14, "med_teal", segs=28)
    S.cyl("foot", x, y, r + 0.005, 0.0, 0.02, "glass", segs=28, bevel=0.005)
    a = math.radians(-90)
    lx, ly = x + math.cos(a) * r, y + math.sin(a) * r
    S.box("label", lx - 0.08, ly - 0.012, 0.03, lx + 0.08, ly + 0.01, 0.09, "med_white", bevel=0.006)
    return S.render_asset("item", organ, zmax=0.3, extra={"item": organ})


def _kidney():
    """A kidney: a plump bean curled round a pale notch with a vessel stub."""
    import math
    from .plant_base import _limb
    x, y, _ = JAR
    for k, a in enumerate((165, 125, 90, 55, 15)):
        ar = math.radians(a)
        _ell("lobe%d" % k, (x + 0.1 * math.cos(ar), y - 0.06 + 0.1 * math.sin(ar), 0.13),
             (0.075, 0.07, 0.065), "organ_kidney", yaw=a - 90, segs=16, rings=10)
    _ell("hilum", (x, y - 0.03, 0.15), (0.04, 0.03, 0.03), "skin_rose", segs=12, rings=8)
    _limb("vessel", (x, y - 0.03, 0.16), (x + 0.03, y - 0.12, 0.2), 0.024, 0.02, "skin_rose", segs=8)
    return _jar("kidney")


def _lung():
    """One lung: a tall soft pink lobe with a fissure, an airway stub on top."""
    from .plant_base import _limb
    x, y, _ = JAR
    _lumpy("lobe", (x + 0.01, y - 0.02, 0.13), (0.14, 0.17, 0.085), "organ_lung", 211, amp=0.12, yaw=-10)
    _ell("upper", (x - 0.02, y + 0.09, 0.16), (0.09, 0.08, 0.06), "organ_lung", segs=14, rings=8)
    _limb("fissure", (x - 0.1, y - 0.03, 0.205), (x + 0.09, y + 0.03, 0.205), 0.009, 0.009, "skin_rose", segs=6)
    _limb("airway", (x - 0.03, y + 0.12, 0.2), (x - 0.05, y + 0.2, 0.25), 0.03, 0.026, "ceramic", segs=8)
    return _jar("lung")


def _organ_heart():
    """A natural heart: red lobes and two pale vessel stubs rising out of the jar."""
    from .plant_base import _limb
    x, y, _ = JAR
    _heart_body("hb", x, y - 0.01, 0.14, 1.35, "organ_heart")
    _limb("aorta", (x - 0.04, y + 0.06, 0.2), (x - 0.08, y + 0.14, 0.27), 0.04, 0.035, "skin_rose", segs=10)
    _limb("vein", (x + 0.06, y + 0.07, 0.19), (x + 0.1, y + 0.14, 0.25), 0.03, 0.026, "skin_rose", segs=8)
    return _jar("heart")


def _liver():
    """A liver: a broad dark wedge of a right lobe beside a smaller left lobe."""
    x, y, _ = JAR
    _lumpy("right", (x - 0.04, y + 0.0, 0.13), (0.16, 0.12, 0.065), "organ_liver", 307, amp=0.1, yaw=15)
    _lumpy("left", (x + 0.11, y - 0.05, 0.125), (0.1, 0.085, 0.055), "organ_liver", 311, amp=0.1, yaw=-20)
    _ell("gall", (x - 0.01, y - 0.1, 0.15), (0.035, 0.03, 0.03), "pl_crop_jade", segs=10, rings=6)
    return _jar("liver")

def build():
    return [_wood(), _food(), _steel(), _stone(), _stone_chunk(), _silver(), _gold(), _berries(), _ration(), _meat(), _meal(), _club(), _rifle_item(), _knife_item(), _spear_item(),
            _potatoes(), _corn(), _wheat(), _beans(), _lettuce(), _radishes(), _pumpkins(),
            _leather(), _parka(), _tuque(), _duster(), _herbal_medicine(), _bones(0), _bones(1),
            _cloth_bolt(), _straw_bundle(), _milk(), _wool(), _eggs(False), _eggs(True), _ember_oil(), _parka_wool()] + [_carpet_rolled(k) for k in _ROLL] + build_ng1() + build_social() + build_med()


def build_ng1():
    return [_components(z) for z in "sml"] + [_industrial_medicine(z) for z in "sml"]


def build_social():
    return [_brewgrain_item(), _ashleaf_item(), _beer(), _ashleaf_tea(), _spark_tablet(), _peg_leg(), _prosthetic_leg(),
            _prosthetic_arm(), _wooden_hand(), _denture(), _hearing_trumpet(),
            _prosthetic_foot(), _glass_eye(), _wooden_nose()]


def build_med():
    """MED-05 medical tier (ART-026)."""
    return [_nanomed_kit(), _servo_arm(), _servo_leg(), _optic_implant(), _cochlear_implant(), _spinal_brace(),
            _pump_heart(), _synth_stomach(), _valve_heart(), _kidney(), _lung(), _organ_heart(), _liver()]
