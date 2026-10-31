"""FOOD programme family (ART spec §9, §12-§16): the new crops, the fruiting
agave, the new produce and meat items, the hydroponics basin and the sun
lamp. Our own designs: nothing traced or reconstructed.

    make art FAMILIES=food

- plant/<rice|strawberry|haygrass|hops>_<stage>_v0..3: crop units from
  crops_food.py, the crops_common stages (sown, sprout, young, mature,
  harvestable, wilted). No outline.
- plant/agave_harvestable_v0..3: the wild agave rosette (plant.py, same
  variant structure as agave_mature_v<n>) with a stout flower stalk
  carrying clusters of ochre fruit. No outline.
- item/<rice|strawberries|hay|hops|agave_fruit|human_meat|insect_meat>:
  ground stacks; `item:` is the economy kind name (ItemSprite's default
  "item/" + name with underscores, so no key mapping is needed).
- ui_icon/<rice|strawberries|hops|hay>: crop-picker icons cut from those
  item sprites.
- furniture/hydroponics_basin_<facing> (1x4, rotatable; construction Def
  Size {1, 4}): a steel trough on legs, four soil cells split by
  dividers, a nutrient-water channel down one long side and a pump box
  with a status lamp (glow layer hydroponics_basin_glow_<facing>, drawn
  while powered). Plants draw over the cells: `attachments` slot0..3 are
  the canvas px of each cell's soil centre (a crop unit's root), slot0 at
  the authored north end; `bed_height` is the soil surface height (m).
- furniture/sun_lamp (1x1; Def has no Size): a tall steel mast with a
  big caged glass lantern under a small reflector cap; the lantern is its
  glow layer (sun_lamp_glow, emissive sun_white), drawn while powered.
  light_radius 11 tiles (wiki: Sun lamp, light reaches 30 % to 11 tiles).
"""
import math
import random

from mathutils import Vector

from .. import stage as S
from . import crops_common
from . import overlay as O
from . import power as P
from .crops_food import BUILDERS as CROPS
from .plant_base import _leaf, _limb, _top

FACINGS = ("south", "east", "north", "west")


# ------------------------------------------------------------ agave fruit
def _agave_fruiting(v):
    """The mature agave rosette with a stout central stalk whose short
    branches hold clusters of plump ochre fruit."""
    from .plant import _agave_geo
    _agave_geo(v, "mature")
    rng = random.Random(2350 + v)
    h = (0.46, 0.4, 0.52, 0.42)[v]
    lean = (0.03, -0.04, 0.02, -0.02)[v]
    top = Vector((0.5 + lean, -0.5, h))
    _limb("stalk", (0.5, -0.5, 0.02), top, 0.03, 0.02, "pl_stem", segs=7)
    nb = (3, 3, 4, 3)[v]
    for k in range(nb):
        a = 2 * math.pi * k / nb + rng.uniform(-0.3, 0.3) + v
        z = h - 0.1 + 0.05 * (k % 2)
        p = Vector((0.5 + lean * z / h, -0.5, z))
        e = p + Vector((math.cos(a) * 0.08, math.sin(a) * 0.06, 0.05))
        _limb("br%d" % k, p, e, 0.014, 0.01, "pl_stem", segs=5)
        for j in range(3):
            fa = a + (j - 1) * 0.9
            c = e + Vector((math.cos(fa) * 0.028, math.sin(fa) * 0.024, 0.012 * (j % 2)))
            S.ellipsoid("fr%d_%d" % (k, j), c.x, c.y, c.z, 0.032, 0.03, 0.038, "pl_agave_fruit", segs=10, rings=6)
            _top(c.z + 0.04)
    S.ellipsoid("crown", top.x, top.y, top.z + 0.02, 0.03, 0.03, 0.035, "pl_agave_fruit", segs=10, rings=6)
    _top(top.z + 0.06)


def _agave_harvestable(v):
    from .plant import BUSH_M, _plant_asset
    return _plant_asset(lambda: _agave_fruiting(v), "agave_harvestable_v%d" % v,
                        {"species": "agave", "stage": "harvestable", "variant": v}, BUSH_M)


# ------------------------------------------------------------------ items
def _ell(name, c, r, mat, yaw=0.0, pitch=0.0, segs=14, rings=8):
    from .item import _ell as ell
    return ell(name, c, r, mat, yaw=yaw, pitch=pitch, segs=segs, rings=rings)


def _item(name, kind, zmax):
    return S.render_asset("item", name, zmax=zmax, extra={"item": kind, "stack": True})


def _rice_item():
    """A heap of pale husked grain on a round woven mat, a few ripe panicle
    stalks laid against it, loose grains scattered."""
    rnd = random.Random(31)
    S.cyl("mat", 0.5, -0.55, 0.32, 0.0, 0.025, "pl_dried", segs=24, bevel=0.008)
    S.cyl("weave", 0.5, -0.55, 0.26, 0.02, 0.03, "pl_stem", segs=24, bevel=0.0)
    S.cyl("weave2", 0.5, -0.55, 0.2, 0.02, 0.032, "pl_dried", segs=24, bevel=0.0)
    S.cyl("heap", 0.5, -0.55, 0.23, 0.03, 0.17, "pl_crop_cotton", r_top=0.035, segs=22, bevel=0.02)
    _ell("cap", (0.5, -0.55, 0.17), (0.05, 0.05, 0.035), "pl_crop_cotton")
    for k in range(18):   # grains studding the heap
        a = rnd.uniform(0, 2 * math.pi)
        r = rnd.uniform(0.06, 0.2)
        z = 0.19 - 0.65 * r
        _ell("g%d" % k, (0.5 + math.cos(a) * r, -0.55 + math.sin(a) * r * 0.95, z), (0.026, 0.014, 0.012),
             "pl_crop_straw" if k % 4 == 0 else "pl_crop_cotton", yaw=math.degrees(a) + 30, segs=8, rings=5)
    for k, (x, y) in enumerate([(0.22, -0.82), (0.3, -0.86), (0.78, -0.8), (0.74, -0.27), (0.82, -0.66)]):
        _ell("lg%d" % k, (x, y, 0.012), (0.026, 0.014, 0.012), "pl_crop_cotton", yaw=k * 47, segs=8, rings=5)
    for k, a in enumerate((2.3, 2.6)):   # two ripe stalks lying across the back of the heap
        p0 = Vector((0.5 + math.cos(a) * 0.3, -0.55 + math.sin(a) * 0.12 + 0.12, 0.03))
        p1 = Vector((0.62 + 0.05 * k, -0.4, 0.12 + 0.02 * k))
        _limb("stk%d" % k, p0, p1, 0.008, 0.006, "pl_crop_straw", segs=5)
        for j in range(5):
            c = p1.lerp(p0, 0.05 + 0.12 * j)
            _leaf("pg%d_%d" % (k, j), c.x, c.y, c.z + 0.006, 0.3 + j * 0.9, -0.2, 0.034, 0.018,
                  "pl_crop_corn", thick=0.016, segs=6)
    return _item("rice", "rice", 0.22)


def _strawberries_item():
    """A shallow slatted wooden punnet heaped with red berries, green caps up."""
    S.box("floor", 0.24, -0.76, 0.0, 0.76, -0.34, 0.03, "wood", bevel=0.01)
    for k, (x0, y0, x1, y1) in enumerate([(0.22, -0.78, 0.78, -0.73), (0.22, -0.37, 0.78, -0.32),
                                          (0.22, -0.78, 0.27, -0.32), (0.73, -0.78, 0.78, -0.32)]):
        S.box("wall%d" % k, x0, y0, 0.0, x1, y1, 0.12, "wood_plank", bevel=0.008)
    S.box("band", 0.215, -0.785, 0.07, 0.785, -0.775, 0.095, "wood", bevel=0.0)
    spec = [(0.34, -0.66), (0.47, -0.67), (0.6, -0.66), (0.66, -0.52), (0.36, -0.48), (0.5, -0.45), (0.62, -0.42),
            (0.42, -0.57), (0.56, -0.56)]
    for k, (x, y) in enumerate(spec):
        z = 0.1 + (0.04 if k >= 7 else 0.0)
        a = k * 41
        _ell("b%d" % k, (x, y, z), (0.06, 0.055, 0.052), "pl_crop_strawberry", yaw=a, pitch=-15, segs=12, rings=8)
        for j in range(3):
            ca = math.radians(a) + j * 2.1
            _leaf("c%d_%d" % (k, j), x, y + 0.01, z + 0.045, ca, 0.2, 0.034, 0.016, "pl_crop_deep", thick=0.01,
                  segs=6)
    _leaf("lf", 0.68, -0.38, 0.14, 0.6, 0.15, 0.12, 0.08, "pl_crop_leaf", thick=0.025)
    for k, (x, y) in enumerate([(0.18, -0.84), (0.84, -0.8)]):
        _ell("fall%d" % k, (x, y, 0.045), (0.05, 0.046, 0.044), "pl_crop_strawberry", yaw=k * 70, segs=12, rings=8)
    return _item("strawberries", "strawberries", 0.22)


def _hay_item():
    """A tied bale of dried green-khaki grass, loose strands bristling on
    top and two cords round it (straw_bundle is loose golden stalks)."""
    rnd = random.Random(17)
    S.box("bale", 0.18, -0.74, 0.0, 0.82, -0.36, 0.3, "pl_dry", bevel=0.05, segments=2)
    for k in range(22):   # strands lying on the top and front faces
        x0 = rnd.uniform(0.2, 0.62)
        ln = rnd.uniform(0.12, 0.2)
        if k % 3 == 0:
            y = -0.745
            z = rnd.uniform(0.06, 0.26)
            p0, p1 = (x0, y, z), (x0 + ln, y, z + rnd.uniform(-0.03, 0.03))
        else:
            y = rnd.uniform(-0.7, -0.4)
            p0, p1 = (x0, y, 0.302), (x0 + ln, y + rnd.uniform(-0.05, 0.05), 0.302)
        _limb("s%d" % k, p0, p1, 0.007, 0.006, ("pl_khaki", "pl_dried", "pl_crop_straw", "pl_crop_leaf")[k % 4],
              segs=4)
    for k, x in enumerate((0.36, 0.64)):   # cords
        S.box("cord%d" % k, x - 0.018, -0.755, -0.002, x + 0.018, -0.345, 0.312, "pl_stem", bevel=0.008)
    for k, (x, y, a) in enumerate([(0.86, -0.78, 0.3), (0.14, -0.3, 2.6), (0.12, -0.8, 3.6)]):
        _limb("loose%d" % k, (x, y, 0.01), (x + math.cos(a) * 0.12, y + math.sin(a) * 0.08, 0.01), 0.007, 0.006,
              "pl_khaki", segs=4)
    return _item("hay", "hay", 0.34)


def _hop_cone(name, c, yaw, s=1.0, mat="pl_crop_hop"):
    """A hop cone lying on its side: three overlapping bract lobes."""
    d = (math.cos(math.radians(yaw)), math.sin(math.radians(yaw)))
    for j in range(3):
        r = (0.044, 0.038, 0.026)[j] * s
        off = (0.0, 0.04, 0.072)[j] * s
        _ell("%s%d" % (name, j), (c[0] + d[0] * off, c[1] + d[1] * off, c[2]), (r * 1.15, r, r), mat, yaw=yaw,
             segs=10, rings=6)


def _hops_item():
    """A small heap of pale green hop cones with a lobed leaf."""
    spec = [(0.3, -0.62, 0.05, 20), (0.48, -0.7, 0.05, -40), (0.64, -0.6, 0.05, 110), (0.36, -0.44, 0.05, 160),
            (0.58, -0.42, 0.05, 230), (0.42, -0.58, 0.13, 80), (0.54, -0.52, 0.14, -20)]
    for k, (x, y, z, yaw) in enumerate(spec):
        _hop_cone("cn%d" % k, (x, y, z), yaw, 1.3, "pl_crop_hop" if k % 3 else "pl_crop_jade")
    from .crops_grain import _palm
    _palm("lf", 0.7, -0.32, 0.04, 0.8, 0.1, 0.16, "pl_crop_leaf", "pl_crop_deep")
    _hop_cone("loose", (0.2, -0.82, 0.04), 35, 1.1)
    return _item("hops", "hops", 0.26)


def _agave_fruit_item():
    """Four plump ochre agave fruits, one halved to show its pale flesh."""
    for k, (x, y, yaw) in enumerate([(0.36, -0.6, 20), (0.56, -0.64, -30), (0.46, -0.45, 80), (0.62, -0.47, 40)]):
        _ell("f%d" % k, (x, y, 0.075), (0.1, 0.075, 0.072), "pl_agave_fruit", yaw=yaw)
        a = math.radians(yaw)
        _limb("tip%d" % k, (x + math.cos(a) * 0.09, y + math.sin(a) * 0.07, 0.08),
              (x + math.cos(a) * 0.12, y + math.sin(a) * 0.09, 0.09), 0.014, 0.008, "pl_stem", segs=5)
    # the halved fruit lying cut face up
    S.cyl("half", 0.3, -0.78, 0.08, 0.0, 0.05, "pl_agave_fruit", ry=0.06, segs=16, bevel=0.01)
    S.cyl("flesh", 0.3, -0.78, 0.064, 0.05, 0.056, "pl_crop_cotton", ry=0.046, segs=16, bevel=0.0)
    S.cyl("core", 0.3, -0.78, 0.022, 0.055, 0.06, "pl_crop_corn", ry=0.016, segs=10, bevel=0.0)
    return _item("agave_fruit", "agave fruit", 0.18)


def _human_meat_item():
    """Two flat pale-pink cuts edged in fat, stacked offset (the red meat
    pile is rounded chunks; this reads as slabs)."""
    for k, (x, y, z, rot) in enumerate([(0.44, -0.58, 0.0, -12), (0.56, -0.5, 0.075, 18)]):
        pts = [(-0.2, -0.12), (0.18, -0.14), (0.22, 0.06), (0.06, 0.15), (-0.18, 0.12)]
        a = math.radians(rot)
        P2 = [(x + px * math.cos(a) - py * math.sin(a), y + px * math.sin(a) + py * math.cos(a)) for px, py in pts]
        S.prism("slab%d" % k, P2, z, z + 0.07, "meat_pale", bevel=0.025)
        # fat rim along the slab's north edge
        fx0, fy0 = P2[2]
        fx1, fy1 = P2[4]
        _limb("fat%d" % k, (fx0, fy0, z + 0.04), (P2[3][0], P2[3][1], z + 0.045), 0.03, 0.03, "meat_fat", segs=8)
        _limb("fatb%d" % k, (P2[3][0], P2[3][1], z + 0.045), (fx1, fy1, z + 0.04), 0.03, 0.026, "meat_fat", segs=8)
        _ell("mb%d" % k, (x - 0.04, y - 0.02, z + 0.07), (0.05, 0.025, 0.006), "meat_fat", yaw=rot + 20, segs=10,
             rings=4)
    return _item("human_meat", "human meat", 0.2)


def _insect_meat_item():
    """Lumpy yellow-green insect meat with two dark chitin plates."""
    for k, (x, y, r) in enumerate([(0.42, -0.58, 0.15), (0.6, -0.54, 0.13), (0.5, -0.68, 0.11)]):
        S.blob("cut%d" % k, x, y, 0.0, r, "meat_insect", seed=90 + k, lumps=0.35, squash=0.5)
    for k, (x, y, yaw) in enumerate([(0.62, -0.4, 30), (0.3, -0.44, -40)]):
        _ell("plate%d" % k, (x, y, 0.07), (0.12, 0.07, 0.022), "chitin", yaw=yaw, pitch=25 if k else -20, segs=12,
             rings=6)
        _ell("ridge%d" % k, (x, y, 0.095), (0.1, 0.012, 0.012), "meat_insect", yaw=yaw, pitch=25 if k else -20,
             segs=8, rings=4)
    return _item("insect_meat", "insect meat", 0.2)


# ------------------------------------------------------ hydroponics basin
BED_Z = 0.33        # soil surface height
SOIL_X = (0.12, 0.74)


def _basin():
    """1x4 authored facing south: x 0..1, y 0..-4."""
    rnd = random.Random(4100)
    for x in (0.12, 0.88):   # six steel legs
        for y in (-0.16, -2.0, -3.84):
            S.box("leg", x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.14, "metal_dark", bevel=0.006)
    S.box("floor", 0.05, -3.95, 0.12, 0.95, -0.05, 0.28, "steel", bevel=0.02)
    # walls round the trough (open top so the soil and water show)
    S.box("wall_w", 0.05, -3.95, 0.12, 0.12, -0.05, 0.38, "steel", bevel=0.012)
    S.box("wall_e", 0.88, -3.95, 0.12, 0.95, -0.05, 0.38, "steel", bevel=0.012)
    S.box("wall_n", 0.05, -0.12, 0.12, 0.95, -0.05, 0.38, "steel", bevel=0.012)
    S.box("wall_s", 0.05, -3.95, 0.12, 0.95, -3.88, 0.38, "steel", bevel=0.012)
    S.box("skirt", 0.05, -3.955, 0.16, 0.95, -3.945, 0.22, "steel_dark", bevel=0.0)
    # the channel: a nutrient-water strip along the east side behind a low lip
    S.box("lip", SOIL_X[1], -3.88, 0.28, SOIL_X[1] + 0.025, -0.12, 0.365, "steel_dark", bevel=0.006)
    S.box("water", SOIL_X[1] + 0.025, -3.88, 0.28, 0.88, -0.12, 0.32, "water_shallow", bevel=0.0)
    # four soil cells split by dividers; plants root at each cell's centre
    for k in range(4):
        y1, y0 = -0.12 - k * 0.94, -0.12 - (k + 1) * 0.94
        S.box("soil%d" % k, SOIL_X[0], y0 + 0.012, 0.28, SOIL_X[1], y1 - 0.012, BED_Z, "soil_rich", bevel=0.01)
        for j in range(7):   # pale perlite flecks in the medium
            x = rnd.uniform(SOIL_X[0] + 0.05, SOIL_X[1] - 0.05)
            y = rnd.uniform(y0 + 0.08, y1 - 0.08)
            S.ellipsoid("fleck", x, y, BED_Z, 0.014, 0.012, 0.006, "pl_khaki", segs=6, rings=3)
        if k:
            S.box("div%d" % k, SOIL_X[0], y1 - 0.012, 0.28, SOIL_X[1], y1 + 0.012, 0.37, "steel_dark", bevel=0.004)
        # a drip nozzle from the channel into each cell
        P._rod("drip%d" % k, (0.83, (y0 + y1) / 2, 0.35), (0.7, (y0 + y1) / 2, 0.36), 0.012, "metal_dark", segs=8,
               bevel=0.0)
    # the pump box at the south end of the channel, with a status lamp on top
    S.box("pump", 0.74, -3.96, 0.12, 0.96, -3.62, 0.5, "steel_dark", bevel=0.02)
    S.box("pump_vent", 0.77, -3.965, 0.24, 0.93, -3.955, 0.42, "metal_dark", bevel=0.0)
    P._rod("pipe", (0.85, -3.62, 0.42), (0.85, -3.4, 0.33), 0.022, "metal_dark", segs=10, bevel=0.0)
    P.glow(S.cyl("status", 0.85, -3.79, 0.035, 0.5, 0.53, "backdrop", segs=12, bevel=0.0), "indicator")
    slots = {"slot%d" % k: ((SOIL_X[0] + SOIL_X[1]) / 2, -0.12 - (k + 0.5) * 0.94, BED_Z) for k in range(4)}
    return (1, 4), 0.53, slots


# ---------------------------------------------------------------- sun lamp
SUN_ZMAX = 2.0


def _sun_lamp():
    """1x1: a weighted plinth with a ballast box, a tall steel mast and a
    big caged glass lantern (the glow) under a small reflector cap."""
    S.box("plinth", 0.22, -0.78, 0.0, 0.78, -0.22, 0.07, "metal_dark", bevel=0.015)
    S.box("ballast", 0.28, -0.74, 0.07, 0.5, -0.56, 0.27, "steel_dark", bevel=0.015)
    for k in range(3):
        S.box("vent%d" % k, 0.31, -0.745, 0.12 + 0.045 * k, 0.47, -0.735, 0.14 + 0.045 * k, "metal_dark", bevel=0.0)
    P._rod("cable", (0.45, -0.56, 0.2), (0.5, -0.5, 0.36), 0.012, "backdrop", segs=6, bevel=0.0)
    S.cyl("mast", 0.5, -0.5, 0.042, 0.07, 1.6, "steel", segs=12, bevel=0.0)
    S.cyl("foot", 0.5, -0.5, 0.075, 0.07, 0.16, "metal_dark", segs=12, bevel=0.01)
    S.cyl("collar", 0.5, -0.5, 0.17, 1.52, 1.6, "metal_dark", segs=20, bevel=0.01)
    P.glow(S.cyl("lantern", 0.5, -0.5, 0.26, 1.6, 1.86, "glass", segs=24, bevel=0.0), "sun_white")
    for k in range(8):   # cage bars over the lantern and its top
        a = math.radians(22.5 + 45 * k)
        x, y = 0.5 + 0.262 * math.cos(a), -0.5 + 0.262 * math.sin(a)
        S.box("bar%d" % k, x - 0.012, y - 0.012, 1.6, x + 0.012, y + 0.012, 1.86, "metal_dark", bevel=0.0)
        P._rod("rib%d" % k, (0.5 + 0.13 * math.cos(a), -0.5 + 0.13 * math.sin(a), 1.865), (x, y, 1.865), 0.011,
               "metal_dark", segs=6, bevel=0.0)
    S.cyl("cap", 0.5, -0.5, 0.14, 1.86, 1.9, "metal_dark", segs=20, bevel=0.01)
    S.cyl("hood", 0.5, -0.5, 0.13, 1.9, 1.96, "steel", r_top=0.06, segs=20, bevel=0.01)
    S.cyl("knob", 0.5, -0.5, 0.03, 1.96, 2.0, "metal_dark", segs=10, bevel=0.0)
    return (1, 1), SUN_ZMAX


# ------------------------------------------------------------------ build
ITEMS = (_rice_item, _strawberries_item, _hay_item, _hops_item, _agave_fruit_item, _human_meat_item,
         _insect_meat_item)


# crop-picker icons cut from the item sprites (as overlay.py does for the older crops)
ICONS = {"rice": ("item", "rice"), "strawberries": ("item", "strawberries"), "hops": ("item", "hops"),
         "hay": ("item", "hay")}


def build(only=None):
    """Every FOOD sprite; `only` (any of "crops", "agave", "items",
    "basin", "lamp") renders just those groups."""
    out = []
    want = (lambda g: True) if not only else (lambda g: g in only)
    if want("crops"):
        for sp, fn in CROPS.items():
            for st in crops_common.STAGES:
                for v in range(crops_common.VARIANTS):
                    out.append(crops_common.unit(sp, st, v, fn))
    if want("agave"):
        for v in range(4):
            out.append(_agave_harvestable(v))
    if want("items"):
        for fn in ITEMS:
            out.append(fn())
        for icon, (fam, spr) in ICONS.items():
            p = O._icon(icon, fam, spr)
            if p:
                out.append(p)
    if want("basin"):
        for f in FACINGS:
            out += P.render("hydroponics_basin_" + f, _basin, facing=f, family="furniture",
                            extra={"building": "hydroponics basin", "bed_height": BED_Z, "slots": 4})
    if want("lamp"):
        out += P.render("sun_lamp", _sun_lamp, family="furniture",
                        extra={"building": "sun lamp", "light_radius": 11})
    return out
