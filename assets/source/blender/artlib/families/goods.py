"""Wave E goods (ART spec §15-§16; PRD-4 and DEF-4): the new items on the
ground, plus their HUD icons (ui_icon/<kind with underscores>).

Same rules as item.py: one sprite per kind in the item family, centred on
its tile, material reads first, `item:` metadata holding the exact
economy kind name. Food categories stay apart at a glance: meals on
plates (a fine meal on one plate, a lavish meal on a platter), preserved
food packed (pemmican bars, the survival meal's foil pouch), animal feed
loose (a kibble heap). Drugs are small (a flake paper, a dust jar, a tonic
flask); chemicals and industrial goods are drums, racks, ingots, sheets,
a sealed module and a long banded barrel. Original designs.
"""
import math

from mathutils import Matrix

from .. import stage as S
from . import overlay as O
from .item import _ell, _spin, _tube
from .plant_base import _limb


def _chemfuel():
    """Chemfuel: a rust-orange fuel drum with dark rolling hoops and a brass
    bung, a flat jerrycan leaning beside it."""
    S.cyl("drum", 0.40, -0.52, 0.2, 0.0, 0.44, "fuel_drum", segs=22, bevel=0.02)
    for k, z in enumerate((0.1, 0.32)):
        S.cyl("hoop%d" % k, 0.40, -0.52, 0.207, z, z + 0.03, "metal_dark", segs=22, bevel=0.0)
    S.cyl("lid", 0.40, -0.52, 0.18, 0.44, 0.455, "metal_dark", segs=22, bevel=0.0)
    S.cyl("bung", 0.46, -0.58, 0.035, 0.455, 0.48, "brass", segs=10, bevel=0.004)
    n0 = len(S.objects())
    S.box("can", 0.62, -0.74, 0.0, 0.84, -0.62, 0.3, "fuel_drum", bevel=0.025)
    S.box("can_rib", 0.62, -0.745, 0.12, 0.84, -0.735, 0.18, "metal_dark", bevel=0.0)
    S.box("handle", 0.66, -0.71, 0.3, 0.78, -0.65, 0.34, "metal_dark", bevel=0.01)
    S.cyl("spout", 0.81, -0.68, 0.025, 0.3, 0.36, "metal_dark", segs=8, bevel=0.0)
    _spin(n0, 0.73, -0.68, -14)
    return S.render_asset("item", "chemfuel", zmax=0.5, extra={"item": "chemfuel", "stack": True})


def _wort():
    """Wort: a low open tub of coopered staves brimming with dark sweet wort,
    a wooden paddle across the rim."""
    S.cyl("tub", 0.5, -0.55, 0.27, 0.0, 0.22, "wood", r_top=0.3, segs=24, bevel=0.02)
    for k in range(12):
        a = 2 * math.pi * k / 12
        x, y = 0.5 + math.cos(a) * 0.285, -0.55 + math.sin(a) * 0.285
        S.box("stave%d" % k, x - 0.006, y - 0.006, 0.02, x + 0.006, y + 0.006, 0.21, "bark", bevel=0.0)
    for k, z in enumerate((0.05, 0.16)):
        S.cyl("band%d" % k, 0.5, -0.55, 0.29 + 0.01 * k, z, z + 0.025, "metal_dark", segs=24, bevel=0.0)
    S.cyl("wort", 0.5, -0.55, 0.27, 0.19, 0.205, "wort_brown", segs=24, bevel=0.0)
    S.ellipsoid("froth", 0.42, -0.5, 0.205, 0.08, 0.06, 0.01, "wool_cream", segs=12, rings=4)
    _limb("paddle", (0.24, -0.32, 0.25), (0.78, -0.74, 0.25), 0.016, 0.016, "wood_plank", segs=8)
    S.box("blade", 0.72, -0.80, 0.235, 0.84, -0.70, 0.255, "wood_plank", bevel=0.006)
    return S.render_asset("item", "wort", zmax=0.3, extra={"item": "wort", "stack": True})


def _fine_meal():
    """Fine meal: one wide plate with a seared cut, a mound of greens, a
    halved root and a bread slice; a fork beside it."""
    S.cyl("plate", 0.5, -0.55, 0.32, 0.0, 0.035, "ceramic", segs=28, bevel=0.01)
    S.cyl("rim", 0.5, -0.55, 0.32, 0.035, 0.045, "med_white", segs=28, bevel=0.0)
    S.blob("cut", 0.44, -0.6, 0.03, 0.12, "food_red", seed=301, lumps=0.2, squash=0.45)
    _ell("greens", (0.62, -0.46, 0.07), (0.09, 0.08, 0.05), "vegetation")
    _ell("root", (0.6, -0.66, 0.06), (0.06, 0.045, 0.035), "crop_fruit", yaw=30)
    S.box("bread", 0.34, -0.44, 0.035, 0.46, -0.36, 0.08, "wool_cream", bevel=0.02)
    S.box("fork", 0.86, -0.78, 0.0, 0.89, -0.38, 0.015, "steel", bevel=0.004)
    return S.render_asset("item", "fine_meal", zmax=0.16, extra={"item": "fine meal", "stack": True})


def _lavish_meal():
    """Lavish meal: an oval brass-rimmed platter carrying a whole roast with
    a bone end, piles of greens and roots, two bread rolls and a fruit."""
    S.cyl("platter", 0.5, -0.55, 0.38, 0.0, 0.04, "ceramic", segs=32, bevel=0.012, ry=0.3)
    S.cyl("platter_rim", 0.5, -0.55, 0.385, 0.03, 0.05, "brass", segs=32, bevel=0.0, ry=0.305)
    _ell("roast", (0.48, -0.56, 0.12), (0.17, 0.11, 0.09), "food_red", yaw=-15)
    _ell("crust", (0.48, -0.55, 0.16), (0.14, 0.08, 0.05), "pemmican_brown", yaw=-15)
    _limb("bone", (0.62, -0.6, 0.12), (0.72, -0.64, 0.14), 0.02, 0.016, "ceramic", segs=8)
    for k, (x, y) in enumerate(((0.26, -0.48), (0.3, -0.66), (0.74, -0.44))):
        _ell("greens%d" % k, (x, y, 0.07), (0.07, 0.06, 0.045), "vegetation")
    for k, (x, y) in enumerate(((0.56, -0.38), (0.4, -0.76))):
        _ell("roll%d" % k, (x, y, 0.07), (0.05, 0.04, 0.035), "wool_cream")
    _ell("fruit", (0.7, -0.72, 0.08), (0.05, 0.05, 0.05), "crop_fruit")
    return S.render_asset("item", "lavish_meal", zmax=0.26, extra={"item": "lavish meal", "stack": True})


def _pemmican():
    """Pemmican: a stack of dark pressed bars flecked with berries, tied with
    a burlap band."""
    bars = ((0.28, -0.66, 0.0), (0.5, -0.66, 0.0), (0.28, -0.46, 0.0), (0.5, -0.46, 0.0), (0.38, -0.56, 0.09))
    for k, (x, y, z) in enumerate(bars):
        S.box("bar%d" % k, x, y - 0.08, z, x + 0.21, y + 0.08, z + 0.09, "pemmican_brown", bevel=0.02)
        for j in range(3):
            S.ellipsoid("berry%d_%d" % (k, j), x + 0.05 + 0.06 * j, y - 0.03 + 0.03 * (j % 2), z + 0.09, 0.014,
                        0.014, 0.008, "food_red", segs=8, rings=4)
    S.box("band", 0.44, -0.72, 0.0, 0.5, -0.4, 0.19, "burlap", bevel=0.005)
    return S.render_asset("item", "pemmican", zmax=0.22, extra={"item": "pemmican", "stack": True})


def _survival_meal():
    """Packaged survival meal: two sealed olive foil pouches with crimped
    ends and a white label band, one lying across the other."""
    for k, (cx, cy, z, deg) in enumerate(((0.48, -0.58, 0.0, 10), (0.54, -0.5, 0.07, -24))):
        n0 = len(S.objects())
        S.box("pouch%d" % k, cx - 0.22, cy - 0.12, z, cx + 0.22, cy + 0.12, z + 0.07, "armour_olive", bevel=0.03,
              segments=2)
        for s in (-1, 1):
            S.box("crimp%d%d" % (k, s), cx + s * 0.25 - 0.03, cy - 0.12, z + 0.02, cx + s * 0.25 + 0.03, cy + 0.12,
                  z + 0.04, "steel_dark", bevel=0.0)
        S.box("label%d" % k, cx - 0.08, cy - 0.1, z + 0.065, cx + 0.08, cy + 0.1, z + 0.075, "med_white", bevel=0.0)
        S.box("mark%d" % k, cx - 0.05, cy - 0.015, z + 0.075, cx + 0.05, cy + 0.015, z + 0.08, "med_red", bevel=0.0)
        _spin(n0, cx, cy, deg)
    return S.render_asset("item", "survival_meal", zmax=0.18, extra={"item": "packaged survival meal", "stack": True})


def _kibble():
    """Kibble: a shallow steel feed bowl heaped with tan pellets, a few
    pellets spilled on the ground in front."""
    S.cyl("bowl", 0.5, -0.52, 0.24, 0.0, 0.1, "steel", r_top=0.29, segs=24, bevel=0.015)
    S.cyl("bowl_in", 0.5, -0.52, 0.25, 0.08, 0.1, "steel_dark", segs=24, bevel=0.0)
    S.blob("heap", 0.5, -0.52, 0.06, 0.2, "kibble_tan", seed=413, lumps=0.12, squash=0.4)
    for k in range(22):
        a = 2.4 * k
        r = 0.03 + 0.0085 * k
        x, y = 0.5 + math.cos(a) * r, -0.52 + math.sin(a) * r
        z = 0.06 + 0.075 * max(0.0, 1 - (r / 0.21) ** 2)
        S.ellipsoid("pellet%d" % k, x, y, z, 0.034, 0.028, 0.02, "pemmican_brown" if k % 3 == 0 else "kibble_tan",
                    segs=8, rings=4)
    for k, (x, y) in enumerate(((0.3, -0.82), (0.42, -0.86), (0.68, -0.84))):
        S.ellipsoid("spill%d" % k, x, y, 0.015, 0.032, 0.026, 0.018, "kibble_tan", segs=8, rings=4)
    return S.render_asset("item", "kibble", zmax=0.2, extra={"item": "kibble", "stack": True})


def _ash_flake():
    """Ash flake: a folded paper packet opened on a pale grey drift of
    flakes, a few flakes scattered in front."""
    n0 = len(S.objects())
    S.box("paper", 0.28, -0.72, 0.0, 0.72, -0.38, 0.012, "numbroot_label", bevel=0.004)
    S.box("fold", 0.28, -0.42, 0.012, 0.72, -0.36, 0.05, "numbroot_label", bevel=0.01)
    S.blob("drift", 0.5, -0.58, 0.01, 0.13, "ash_flake", seed=421, lumps=0.3, squash=0.35)
    for k in range(6):
        x, y = 0.3 + 0.08 * k, -0.78 + 0.03 * (k % 2)
        S.box("flake%d" % k, x - 0.02, y - 0.012, 0.0, x + 0.02, y + 0.012, 0.012, "ash_flake", bevel=0.003)
    _spin(n0, 0.5, -0.56, 8)
    return S.render_asset("item", "ash_flake", zmax=0.1, extra={"item": "ash flake", "stack": True})


def _silverdust():
    """Silverdust: a squat corked glass jar of shimmering silver powder, a
    little heap of it poured beside."""
    S.cyl("dust", 0.42, -0.52, 0.13, 0.0, 0.2, "silverdust", segs=20, bevel=0.0)
    _tube("jar", 0.42, -0.52, 0.15, 0.135, 0.0, 0.27, "glass", segs=20)
    S.cyl("neck", 0.42, -0.52, 0.09, 0.27, 0.3, "glass", segs=16, bevel=0.0)
    S.cyl("cork", 0.42, -0.52, 0.075, 0.3, 0.35, "wood", segs=14, bevel=0.01)
    S.blob("heap", 0.66, -0.66, 0.0, 0.11, "silverdust", seed=431, lumps=0.2, squash=0.45)
    return S.render_asset("item", "silverdust", zmax=0.38, extra={"item": "silverdust", "stack": True})


def _battle_tonic():
    """Battle tonic: two red tonic flasks with steel caps in a small leather
    holster, a white cross band on each."""
    S.box("holster", 0.3, -0.66, 0.0, 0.7, -0.46, 0.14, "leather", bevel=0.025)
    for k, x in enumerate((0.4, 0.6)):
        S.cyl("flask%d" % k, x, -0.56, 0.08, 0.04, 0.32, "tonic_red", segs=16, bevel=0.02)
        S.cyl("band%d" % k, x, -0.56, 0.083, 0.2, 0.24, "med_white", segs=16, bevel=0.0)
        S.cyl("cap%d" % k, x, -0.56, 0.05, 0.32, 0.38, "steel", segs=12, bevel=0.01)
    return S.render_asset("item", "battle_tonic", zmax=0.4, extra={"item": "battle tonic", "stack": True})


def _reagent():
    """Reagent: a wooden rack of three stoppered glass tubes of violet
    reagent, one more tube lying in front."""
    S.box("rack_base", 0.24, -0.62, 0.0, 0.76, -0.46, 0.05, "wood", bevel=0.01)
    S.box("rack_top", 0.24, -0.6, 0.18, 0.76, -0.48, 0.22, "wood_plank", bevel=0.01)
    for k, x in enumerate((0.34, 0.5, 0.66)):
        S.cyl("fluid%d" % k, x, -0.54, 0.045, 0.02, 0.22, "reagent_violet", segs=12, bevel=0.0)
        _tube("tube%d" % k, x, -0.54, 0.05, 0.04, 0.02, 0.32, "glass", segs=12)
        S.cyl("stopper%d" % k, x, -0.54, 0.042, 0.32, 0.36, "bark", segs=10, bevel=0.006)
    _limb("loose", (0.3, -0.76, 0.045), (0.62, -0.8, 0.045), 0.045, 0.045, "reagent_violet", segs=12)
    _limb("loose_cap", (0.62, -0.8, 0.045), (0.68, -0.81, 0.045), 0.042, 0.042, "bark", segs=10)
    return S.render_asset("item", "reagent", zmax=0.4, extra={"item": "reagent", "stack": True})


def _advanced_components():
    """Advanced components: two sealed pale-blue alloy modules with dark
    faceplates and teal-lit circuit slots, a coil of copper fibre."""
    for k, (cx, cy, z, deg) in enumerate(((0.44, -0.6, 0.0, -10), (0.56, -0.48, 0.08, 18))):
        n0 = len(S.objects())
        S.box("case%d" % k, cx - 0.17, cy - 0.11, z, cx + 0.17, cy + 0.11, z + 0.08, "plasteel_blue", bevel=0.02)
        S.box("face%d" % k, cx - 0.13, cy - 0.08, z + 0.08, cx + 0.13, cy + 0.08, z + 0.095, "metal_dark", bevel=0.006)
        for j in range(3):
            S.box("slot%d%d" % (k, j), cx - 0.1, cy - 0.055 + 0.045 * j, z + 0.095, cx + 0.06,
                  cy - 0.04 + 0.045 * j, z + 0.1, "med_teal_glow", bevel=0.0)
        S.box("pins%d" % k, cx + 0.17, cy - 0.06, z + 0.02, cx + 0.2, cy + 0.06, z + 0.05, "brass", bevel=0.004)
        _spin(n0, cx, cy, deg)
    S.cyl("coil", 0.78, -0.74, 0.07, 0.0, 0.04, "copper", segs=16, bevel=0.01)
    S.cyl("coil_hole", 0.78, -0.74, 0.03, 0.04, 0.045, "metal_dark", segs=12, bevel=0.0)
    return S.render_asset("item", "advanced_components", zmax=0.22,
                          extra={"item": "advanced components", "stack": True})


def _uranium():
    """Uranium: three heavy rounded slugs of dull green-grey metal stacked
    two and one, each with a darker cast seam."""
    slugs = ((0.34, -0.6, 0.0), (0.62, -0.62, 0.0), (0.48, -0.56, 0.15))
    for k, (x, y, z) in enumerate(slugs):
        _limb("slug%d" % k, (x - 0.12, y, z + 0.08), (x + 0.12, y, z + 0.08), 0.08, 0.08, "uranium_green", segs=16)
        S.cyl("seam%d" % k, x, y, 0.082, z + 0.075, z + 0.085, "steel_dark", segs=16, bevel=0.0)
        S.objects()[-1].matrix_world = (Matrix.Translation((x, y, z + 0.08)) @ Matrix.Rotation(math.pi / 2, 4, "Y")
                                        @ Matrix.Translation((-x, -y, -(z + 0.08))))
    return S.render_asset("item", "uranium", zmax=0.32, extra={"item": "uranium", "stack": True})


def _plasteel():
    """Plasteel: a neat stack of thin pale-blue alloy sheets, offset so each
    edge reads, a dark strap round them."""
    for k in range(5):
        dx, dy = 0.015 * (k % 2), -0.012 * (k % 3)
        S.box("sheet%d" % k, 0.2 + dx, -0.74 + dy, 0.035 * k, 0.8 + dx, -0.36 + dy, 0.035 * k + 0.03,
              "plasteel_blue", bevel=0.008)
    S.box("strap", 0.47, -0.76, 0.0, 0.53, -0.34, 0.185, "metal_dark", bevel=0.004)
    return S.render_asset("item", "plasteel", zmax=0.2, extra={"item": "plasteel", "stack": True})


def _reinforced_barrel():
    """Reinforced barrel: a long dark gun barrel lying diagonally, banded by
    three brass reinforcing rings, with a flared muzzle and a breech block."""
    a, b = (0.14, -0.84, 0.07), (0.86, -0.26, 0.07)
    _limb("barrel", a, b, 0.05, 0.04, "steel_dark", segs=16)
    for k, t in enumerate((0.25, 0.5, 0.75)):
        p = tuple(a[i] + (b[i] - a[i]) * t for i in range(3))
        q = tuple(a[i] + (b[i] - a[i]) * (t + 0.035) for i in range(3))
        _limb("ring%d" % k, p, q, 0.06, 0.06, "brass", segs=16)
    _limb("muzzle", b, (0.9, -0.23, 0.07), 0.052, 0.058, "metal_dark", segs=16)
    S.box("breech", 0.08, -0.92, 0.0, 0.22, -0.78, 0.14, "metal_dark", bevel=0.02)
    return S.render_asset("item", "reinforced_barrel", zmax=0.16, extra={"item": "reinforced barrel", "stack": True})


# kind -> (builder, sprite name); the icon is ui_icon/<kind with underscores>
GOODS = {
    "chemfuel": (_chemfuel, "chemfuel"),
    "wort": (_wort, "wort"),
    "fine meal": (_fine_meal, "fine_meal"),
    "lavish meal": (_lavish_meal, "lavish_meal"),
    "pemmican": (_pemmican, "pemmican"),
    "packaged survival meal": (_survival_meal, "survival_meal"),
    "kibble": (_kibble, "kibble"),
    "ash flake": (_ash_flake, "ash_flake"),
    "silverdust": (_silverdust, "silverdust"),
    "battle tonic": (_battle_tonic, "battle_tonic"),
    "reagent": (_reagent, "reagent"),
    "advanced components": (_advanced_components, "advanced_components"),
    "uranium": (_uranium, "uranium"),
    "plasteel": (_plasteel, "plasteel"),
    "reinforced barrel": (_reinforced_barrel, "reinforced_barrel"),
}


def build(only=None):
    """Every wave E item sprite and its icon; `only` (kind names) renders just those."""
    out = []
    for kind, (fn, name) in GOODS.items():
        if only and kind not in only:
            continue
        out.append(fn())
        p = O._icon(kind.replace(" ", "_"), "item", name)
        if p:
            out.append(p)
    return out
