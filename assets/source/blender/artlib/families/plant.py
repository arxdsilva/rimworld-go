"""Plants (ART spec §9): silhouette and growth state first.

Trees may exceed their tile but keep a clear ground origin (trunk base at
tile centre). Crops show four growth stages; wild berry bushes show a
harvestable state. Variation is authored per variant index, never RNG at
runtime.
"""
import math

from .. import stage as S


def _tree(i):
    seeds = (3, 8, 15)
    trunk_h = (0.9, 0.75, 1.05)[i]
    S.cyl("trunk", 0.5, -0.5, 0.09, 0.0, trunk_h + 0.3, "bark", segs=10, r_top=0.06)
    S.cyl("root", 0.5, -0.5, 0.15, 0.0, 0.08, "bark", segs=10, r_top=0.10)
    cz = trunk_h + 0.45
    r = (0.62, 0.52, 0.58)[i]
    S.blob("crown", 0.5, -0.45, cz, r, "foliage", seed=seeds[i], lumps=0.22, squash=0.85)
    for k in range(3):
        a = math.radians(40 + 120 * k + 25 * i)
        S.blob("lobe%d" % k, 0.5 + math.cos(a) * r * 0.62, -0.45 + math.sin(a) * r * 0.62,
               cz - 0.12, r * 0.55, "foliage", seed=seeds[i] * 10 + k, lumps=0.25, squash=0.8)
    zmax = cz + r * 0.95
    return S.render_asset("plant", "tree_%02d" % i, zmax=zmax,
                          margins=(0.3, 0.3, 0.15, 0.25),
                          extra={"species": "tree", "variant": i, "yields": "wood"})


def _bush(i, harvestable):
    S.blob("bush", 0.5, -0.52, 0.0, 0.34, "vegetation", seed=40 + i, lumps=0.25, squash=0.85)
    S.blob("bush2", 0.36, -0.42, 0.05, 0.2, "foliage", seed=50 + i, lumps=0.25, squash=0.9)
    if harvestable:
        for k, (x, y, z) in enumerate([(0.38, -0.58, 0.26), (0.58, -0.38, 0.30), (0.66, -0.66, 0.20),
                                        (0.48, -0.48, 0.34), (0.30, -0.40, 0.24), (0.62, -0.52, 0.31)]):
            S.ellipsoid("berry%d" % k, x, y, z, 0.05, 0.05, 0.05, "food_red", segs=10, rings=6)
    name = "bush_%02d%s" % (i, "_harvestable" if harvestable else "")
    return S.render_asset("plant", name, zmax=0.42,
                          extra={"species": "berry_bush", "variant": i, "harvestable": harvestable})


def _grass(i):
    import numpy as np
    rng = np.random.RandomState(70 + i)
    for k in range(7 + i * 2):
        x, y = rng.uniform(0.25, 0.75), rng.uniform(-0.75, -0.25)
        h = rng.uniform(0.12, 0.24)
        S.cyl("blade%d" % k, x, y, 0.035, 0.0, h, "vegetation" if k % 3 else "foliage",
              segs=5, r_top=0.005, bevel=0.0)
    return S.render_asset("plant", "grass_%02d" % i, zmax=0.25, outline=True, shadow=False,
                          extra={"species": "grass", "variant": i, "decoration": True})


def _leaf(name, x, y, ang, length, z, mat="vegetation", width=0.07):
    dx, dy = math.cos(ang), math.sin(ang)
    px, py = -dy * width, dx * width
    pts = [(x + px * 0.3, y + py * 0.3), (x + dx * length * 0.5 + px, y + dy * length * 0.5 + py),
           (x + dx * length, y + dy * length), (x + dx * length * 0.5 - px, y + dy * length * 0.5 - py),
           (x - px * 0.3, y - py * 0.3)]
    pts = pts[::-1] if (dx * py - dy * px) > 0 else pts
    S.prism(name, pts, z, z + 0.02, mat, bevel=0.0)


def _mound():
    S.cyl("mound", 0.5, -0.52, 0.30, 0.0, 0.05, "soil_rich", segs=20, r_top=0.24, bevel=0.0)


def _sown():
    """Shared first stage: a tilled mound with seedlings."""
    _mound()
    for k, (x, y) in enumerate([(0.38, -0.40), (0.62, -0.60), (0.58, -0.36)]):
        S.cyl("sprout%d" % k, x, y, 0.025, 0.05, 0.12, "vegetation", segs=6, r_top=0.01, bevel=0.0)
    return S.render_asset("plant", "crop_sown", zmax=0.12, extra={"growth": "sown"})


def _rosette(n, length, z, mats=("vegetation", "foliage"), width=None, cx=0.5, cy=-0.5, rot=0.3):
    for k in range(n):
        a = 2 * math.pi * k / n + rot
        _leaf("leaf%d" % k, cx, cy, a, length, z + 0.02 * (k % 2), mats[k % len(mats)],
              width=width or (0.07 + length * 0.12))


def _radish(k):
    """Radish: a low rosette of leaves; red roots crown at the soil when ripe."""
    _mound()
    size = (0.5, 0.85, 1.0)[k]
    _rosette(6, 0.26 * size, 0.07, ("vegetation", "foliage"))
    S.cyl("heart", 0.5, -0.5, 0.05, 0.05, 0.12 * size + 0.05, "vegetation", segs=8, r_top=0.02)
    if k == 2:
        for i, (x, y) in enumerate([(0.38, -0.60), (0.62, -0.58), (0.50, -0.36)]):
            S.ellipsoid("root%d" % i, x, y, 0.07, 0.07, 0.07, 0.065, "food_red", segs=12, rings=8)
    return 0.22 * size + 0.08


def _lettuce(k):
    """Lettuce: broad pale leaves closing into a round head."""
    _mound()
    size = (0.5, 0.85, 1.05)[k]
    _rosette(7, 0.30 * size, 0.06, ("leaf_light", "vegetation"), width=0.11 * size + 0.03)
    S.ellipsoid("head", 0.5, -0.5, 0.10 * size + 0.04, 0.16 * size, 0.16 * size, 0.11 * size,
                "leaf_light", segs=16, rings=10)
    if k >= 1:
        S.ellipsoid("heart", 0.5, -0.5, 0.16 * size + 0.04, 0.08 * size, 0.08 * size, 0.06 * size,
                    "vegetation", segs=12, rings=8)
    return 0.30 * size + 0.06


def _beans(k):
    """Bush beans: a leafy little shrub; pods hang from it when ripe."""
    _mound()
    size = (0.5, 0.9, 1.0)[k]
    S.blob("bush", 0.5, -0.5, 0.05, 0.26 * size, "foliage", seed=81, lumps=0.28, squash=0.9)
    S.blob("bush2", 0.40, -0.42, 0.10, 0.16 * size, "vegetation", seed=82, lumps=0.3, squash=0.9)
    S.blob("bush3", 0.62, -0.58, 0.08, 0.15 * size, "vegetation", seed=83, lumps=0.3, squash=0.9)
    if k == 2:
        # ripe dry pods (tan) hang all round the bush, on top of the leaves
        for i in range(8):
            a = 2 * math.pi * i / 8 + 0.2
            x, y = 0.5 + math.cos(a) * 0.22, -0.5 + math.sin(a) * 0.22
            S.ellipsoid("pod%d" % i, x, y, 0.22 + 0.04 * (i % 2), 0.035, 0.035, 0.09, "straw", segs=8, rings=6)
    return 0.30 * size + 0.12


def _potato(k):
    """Potato: a sprawling leafy plant that flowers; tubers show when dug."""
    _mound()
    size = (0.5, 0.9, 1.0)[k]
    S.blob("haulm", 0.5, -0.5, 0.06, 0.28 * size, "vegetation", seed=91, lumps=0.25, squash=0.8)
    S.blob("haulm2", 0.38, -0.40, 0.08, 0.18 * size, "foliage", seed=92, lumps=0.28, squash=0.85)
    if k >= 1:
        for i, (x, y) in enumerate([(0.45, -0.45), (0.58, -0.55), (0.52, -0.38)]):
            S.ellipsoid("flower%d" % i, x, y, 0.30 * size + 0.04, 0.035, 0.035, 0.02, "blossom", segs=8, rings=5)
    if k == 2:
        for i, (x, y) in enumerate([(0.26, -0.70), (0.72, -0.72), (0.76, -0.40)]):
            S.ellipsoid("tuber%d" % i, x, y, 0.05, 0.075, 0.06, 0.05, "tuber", segs=12, rings=8)
    return 0.30 * size + 0.10


def _corn(k):
    """Corn: tall stalks with long arching leaves, yellow cobs and tassels."""
    _mound()
    h = (0.35, 0.95, 1.10)[k]
    for i, (x, y) in enumerate([(0.42, -0.46), (0.60, -0.56)]):
        S.cyl("stalk%d" % i, x, y, 0.035, 0.03, h, "vegetation", segs=8, r_top=0.022, bevel=0.0)
        for j in range(3 if k else 2):
            z = h * (0.35 + 0.22 * j)
            a = 0.8 + 2.2 * j + 1.4 * i
            _leaf("cleaf%d%d" % (i, j), x, y, a, 0.32 * (h / 1.1) + 0.06, z, "foliage" if j % 2 else "vegetation",
                  width=0.05)
        if k == 2:
            S.ellipsoid("cob%d" % i, x + 0.06, y - 0.03, h * 0.55, 0.045, 0.045, 0.11, "corn_yellow", segs=10, rings=8)
            S.ellipsoid("husk%d" % i, x + 0.07, y - 0.035, h * 0.50, 0.05, 0.05, 0.08, "leaf_light", segs=10, rings=6)
            S.cyl("tassel%d" % i, x, y, 0.03, h, h + 0.12, "straw", segs=6, r_top=0.004, bevel=0.0)
    return h + (0.14 if k == 2 else 0.04)


def _wheat(k):
    """Wheat: a dense clump of thin stalks, green, then golden with heads."""
    _mound()
    h = (0.28, 0.62, 0.70)[k]
    stalk = "straw" if k == 2 else "vegetation"
    import random
    rnd = random.Random(11)
    for i in range(14):
        x, y = 0.5 + rnd.uniform(-0.2, 0.2), -0.5 + rnd.uniform(-0.2, 0.2)
        hh = h * rnd.uniform(0.85, 1.05)
        S.cyl("stem%d" % i, x, y, 0.014, 0.03, hh, stalk if i % 3 else ("foliage" if k < 2 else "straw"),
              segs=5, r_top=0.008, bevel=0.0)
        if k >= 1:
            S.ellipsoid("ear%d" % i, x, y, hh + 0.05, 0.022, 0.022, 0.06,
                        "straw" if k == 2 else "leaf_light", segs=6, rings=5)
    return h + 0.12


def _pumpkin(k):
    """Pumpkin: big flat leaves on a sprawling vine; orange fruit when ripe."""
    _mound()
    size = (0.5, 0.95, 1.0)[k]
    for i in range(5):
        a = 2 * math.pi * i / 5 + 0.5
        _leaf("pleaf%d" % i, 0.5, -0.5, a, 0.36 * size, 0.05 + 0.02 * (i % 2),
              "foliage" if i % 2 else "vegetation", width=0.15 * size)
    S.cyl("vine", 0.5, -0.5, 0.04, 0.04, 0.10, "vegetation", segs=8)
    if k >= 1:
        S.ellipsoid("bloom", 0.62, -0.40, 0.12, 0.04, 0.04, 0.03, "corn_yellow", segs=8, rings=5)
    if k == 2:
        for i, (x, y, r) in enumerate([(0.36, -0.62, 0.15), (0.66, -0.66, 0.12)]):
            S.ellipsoid("pumpkin%d" % i, x, y, r * 0.75, r, r * 0.95, r * 0.75, "pumpkin", segs=16, rings=10)
            S.cyl("stem%d" % i, x, y, 0.015, r * 1.4, r * 1.65, "bark", segs=6, bevel=0.0)
        return 0.30
    return 0.16 * size + 0.08


def _healroot(k):
    """Healroot: a low rosette of pointed, pale-veined leaves; tiny white
    flowers and the knobbly root pushing up through the soil when ripe."""
    _mound()
    size = (0.55, 0.9, 1.0)[k]
    for i in range(7):
        a = 2 * math.pi * i / 7 + 0.3
        _leaf("hleaf%d" % i, 0.5, -0.5, a, 0.30 * size, 0.04 + 0.03 * (i % 3),
              "leaf_light" if i % 2 else "foliage", width=0.06 * size)
    S.cyl("heart", 0.5, -0.5, 0.05, 0.03, 0.12 * size, "vegetation", segs=8, r_top=0.02)
    if k >= 1:
        for i in range(4):
            a = 2 * math.pi * i / 4 + 0.9
            x, y = 0.5 + 0.09 * math.cos(a), -0.5 + 0.09 * math.sin(a)
            S.cyl("stalk%d" % i, x, y, 0.01, 0.05, 0.22, "vegetation", segs=5, bevel=0.0)
            S.ellipsoid("flower%d" % i, x, y, 0.23, 0.035, 0.035, 0.025, "blossom", segs=8, rings=5)
    if k == 2:
        for i, (x, y) in enumerate([(0.28, -0.70), (0.72, -0.68), (0.30, -0.30)]):
            S.blob("root%d" % i, x, y, 0.03, 0.095, "tuber", seed=90 + i, lumps=0.3, squash=0.7)
        return 0.28
    return 0.12 * size + 0.12 * (k >= 1)


CROPS = {"radish": _radish, "lettuce": _lettuce, "beans": _beans, "potato": _potato,
         "corn": _corn, "wheat": _wheat, "pumpkin": _pumpkin, "healroot": _healroot}
STAGES = ("young", "mature", "harvestable")


def _crop(species, k):
    zmax = CROPS[species](k)
    return S.render_asset("plant", "%s_%s" % (species, STAGES[k]), zmax=zmax,
                          margins=(0.125, 0.125, 0.12, 0.125),
                          extra={"species": species, "growth": STAGES[k]})


def build():
    out = [_tree(i) for i in range(3)]
    out += [_bush(0, False), _bush(0, True), _bush(1, False), _bush(1, True)]
    out += [_grass(0), _grass(1)]
    out.append(_sown())
    out += [_crop(sp, k) for sp in CROPS for k in range(3)]
    return out
