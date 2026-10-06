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


def _crop(stage):
    """Original crop 'amberroot': rosette of leaves, amber bulbs when ripe."""
    S.cyl("mound", 0.5, -0.52, 0.30, 0.0, 0.05, "soil_rich", segs=20, r_top=0.24, bevel=0.0)
    if stage == "sown":
        for k, (x, y) in enumerate([(0.38, -0.40), (0.62, -0.60), (0.58, -0.36)]):
            S.cyl("sprout%d" % k, x, y, 0.025, 0.05, 0.12, "vegetation", segs=6, r_top=0.01, bevel=0.0)
        zmax = 0.12
    else:
        n, length, z = {"young": (5, 0.22, 0.08), "mature": (7, 0.36, 0.12), "harvestable": (8, 0.40, 0.13)}[stage]
        for k in range(n):
            a = 2 * math.pi * k / n + 0.3
            _leaf("leaf%d" % k, 0.5, -0.5, a, length, z + 0.02 * (k % 2),
                  "foliage" if k % 2 else "vegetation", width=0.07 + length * 0.12)
        S.cyl("heart", 0.5, -0.5, 0.06 + length * 0.1, 0.05, z + 0.08, "vegetation", segs=8, r_top=0.03)
        zmax = z + 0.10
        if stage == "harvestable":
            for k in range(3):
                a = 2 * math.pi * k / 3 + 1.0
                S.ellipsoid("bulb%d" % k, 0.5 + math.cos(a) * 0.17, -0.5 + math.sin(a) * 0.17, 0.12,
                            0.09, 0.09, 0.08, "crop_fruit", segs=12, rings=8)
            zmax = 0.25
    return S.render_asset("plant", "crop_%s" % stage, zmax=zmax,
                          extra={"species": "amberroot", "growth": stage})


def build():
    out = [_tree(i) for i in range(3)]
    out += [_bush(0, False), _bush(0, True), _bush(1, False), _bush(1, True)]
    out += [_grass(0), _grass(1)]
    out += [_crop(s) for s in ("sown", "young", "mature", "harvestable")]
    return out
