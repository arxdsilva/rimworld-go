"""Nutrient paste family (STO-2/3; ART spec §12-§16): the hopper, the
nutrient paste dispenser and the paste meal, plus their icons. Our own
designs: nothing traced or reconstructed.

Footprints come from the construction Def rows; neither building rotates.

- hopper (1x1, furniture): a riveted steel bin on four short legs, open at
  the top over a dark floor so the raw food the renderer draws in it reads,
  a discharge chute under it. Steel only (the Def row is metal-only).
- nutrient_paste_dispenser (3x4, furniture): a steel machine. Two paste
  vats sit on a long housing at the back, joined by a manifold that runs
  forward into the serving hood; feed ducts stick out of both long sides
  where the hoppers stand. The serving counter fills the south row, its
  nozzle over a tray at the middle, where a colonist takes a meal (the
  Def row's spot is the cell south of the middle). The display, the
  nozzle lamp and the ready lamps are its glow layer
  (nutrient_paste_dispenser_glow), drawn while it has power.
- nutrient_paste_meal (item): a moulded steel tray with three wells of
  pale paste, the big one piled in a piped swirl. Unlike the simple meal's
  bowl on a plate, it reads as dispensed, not cooked.

    make art FAMILIES=paste
"""
import math

from .. import stage as S
from . import overlay as O
from . import power as P


# ------------------------------------------------------------------ hopper
def _hopper():
    """1x1: a square bin tapering to a chute; inside the rim walls, squares
    darkening toward the throat read as the funnel sloping down."""
    for i, x in enumerate((0.17, 0.83)):
        for j, y in enumerate((-0.83, -0.17)):
            S.box("leg%d%d" % (i, j), x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.30, "metal_dark", bevel=0.008)
    S.box("chute", 0.40, -0.66, 0.06, 0.60, -0.46, 0.22, "steel_dark", bevel=0.01)
    S.box("chute_lip", 0.38, -0.70, 0.04, 0.62, -0.62, 0.10, "metal_dark", bevel=0.006)
    # the tapered body: a square frustum (a four-sided cylinder turned 45 degrees)
    n0 = len(S.objects())
    S.cyl("body", 0.5, -0.5, 0.21 * math.sqrt(2), 0.16, 0.50, "steel", segs=4, r_top=0.39 * math.sqrt(2), bevel=0.01)
    S.rotate_z(P._since(n0), 45)
    lo, hi = 0.48, 0.62
    S.box("floor", 0.14, -0.86, 0.505, 0.86, -0.14, 0.515, "steel_dark", bevel=0.0)
    S.box("slope", 0.24, -0.76, 0.515, 0.76, -0.24, 0.522, "metal_dark", bevel=0.0)
    S.box("throat", 0.36, -0.64, 0.522, 0.64, -0.36, 0.528, "backdrop", bevel=0.0)
    S.box("wall_n", 0.10, -0.15, lo, 0.90, -0.10, hi, "steel", bevel=0.01)
    S.box("wall_s", 0.10, -0.90, lo, 0.90, -0.85, hi, "steel", bevel=0.01)
    S.box("wall_w", 0.10, -0.90, lo, 0.15, -0.10, hi, "steel", bevel=0.01)
    S.box("wall_e", 0.85, -0.90, lo, 0.90, -0.10, hi, "steel", bevel=0.01)
    S.box("band", 0.095, -0.905, lo + 0.02, 0.905, -0.86, lo + 0.06, "steel_dark", bevel=0.0)
    for k, (x, y) in enumerate(((0.125, -0.125), (0.875, -0.125), (0.125, -0.875), (0.875, -0.875))):
        S.cyl("rivet%d" % k, x, y, 0.018, hi, hi + 0.012, "metal_dark", segs=8, bevel=0.0)
    return (1, 1), hi


# --------------------------------------------------------------- dispenser
def _dispenser():
    """3x4 (the south row is the counter): housing, vats, manifold, side
    ducts, serving hood with nozzle and tray, console screen and ready
    lamps. Every emissive part faces up, so the glow layer reads from above."""
    S.box("plinth", 0.05, -3.95, 0.0, 2.95, -0.05, 0.08, "steel_dark", bevel=0.015)
    # the housing over the north three rows
    S.box("housing", 0.16, -2.70, 0.08, 2.84, -0.14, 1.02, "pod_hull", bevel=0.05, segments=2)
    S.box("housing_top", 0.12, -2.74, 1.0, 2.88, -0.10, 1.06, "pod_hull", bevel=0.02)
    S.box("enamel", 0.14, -2.72, 1.06, 2.86, -2.58, 1.072, "pod_paint", bevel=0.0)
    for k, (x, y) in enumerate(((0.22, -0.20), (2.78, -0.20), (0.22, -2.46), (2.78, -2.46))):
        S.cyl("corner_bolt%d" % k, x, y, 0.03, 1.06, 1.08, "metal_dark", segs=8, bevel=0.0)
    # feed ducts out of both long sides, where the hoppers stand
    for k, (x0, x1) in enumerate(((0.0, 0.18), (2.82, 3.0))):
        for j, y in enumerate((-0.95, -1.95)):
            S.box("duct%d%d" % (k, j), x0, y - 0.20, 0.52, x1, y + 0.20, 0.80, "steel_dark", bevel=0.02)
            S.box("collar%d%d" % (k, j), x0 + (0.0 if k else 0.12), y - 0.24, 0.48,
                  x0 + (0.06 if k else 0.18), y + 0.24, 0.84, "metal_dark", bevel=0.01)
    # two paste vats on the housing, domed, banded
    for k, x in enumerate((0.80, 2.20)):
        S.cyl("vat%d" % k, x, -1.05, 0.52, 1.06, 1.62, "steel", segs=32, bevel=0.03)
        for z in (1.18, 1.46):
            S.cyl("vat_band%d" % k, x, -1.05, 0.535, z, z + 0.04, "steel_dark", segs=32, bevel=0.0)
        S.ellipsoid("dome%d" % k, x, -1.05, 1.62, 0.50, 0.50, 0.16, "steel", segs=32, rings=10)
        S.cyl("hatch%d" % k, x, -1.05, 0.15, 1.72, 1.79, "metal_dark", segs=20, bevel=0.01)
        S.box("hatch_grip%d" % k, x - 0.10, -1.07, 1.79, x + 0.10, -1.03, 1.83, "steel_dark", bevel=0.005)
        P._bolts(x, -1.05, 0.12, 1.79, 1.80, 6, rb=0.014, material="steel_dark")
    # manifold: from each vat into a junction, then forward to the hood
    for k, x in enumerate((0.80, 2.20)):
        P._rod("feed%d" % k, (x + (0.45 if k == 0 else -0.45), -1.45, 1.22), (1.5, -1.62, 1.22), 0.07,
               "copper", segs=12, bevel=0.0)
    S.cyl("junction", 1.5, -1.62, 0.13, 1.06, 1.34, "steel_dark", segs=18, bevel=0.015)
    P._rod("riser", (1.5, -1.62, 1.28), (1.5, -2.70, 1.28), 0.075, "copper", segs=12, bevel=0.0)
    # vent grilles on the housing, either side of the riser
    for k in range(5):
        y = -2.00 - k * 0.09
        S.box("vent%d" % k, 0.30, y - 0.025, 1.06, 1.00, y + 0.025, 1.075, "metal_dark", bevel=0.0)
        S.box("vent_e%d" % k, 2.00, y - 0.025, 1.06, 2.70, y + 0.025, 1.075, "metal_dark", bevel=0.0)
    # the serving counter over the south row, a clean enamel top
    S.box("counter", 0.30, -3.86, 0.08, 2.70, -2.70, 0.70, "pod_hull", bevel=0.04)
    S.box("counter_top", 0.24, -3.92, 0.70, 2.76, -2.66, 0.76, "ceramic", bevel=0.02)
    S.box("stripe", 0.31, -3.875, 0.50, 2.69, -3.85, 0.58, "pod_paint", bevel=0.0)
    S.box("kick", 0.34, -3.88, 0.08, 2.66, -3.84, 0.16, "steel_dark", bevel=0.005)
    # the hood: a short box against the housing over the middle, its spout
    # reaching south over the tray
    S.box("hood", 1.18, -3.06, 0.76, 1.82, -2.64, 1.24, "steel_dark", bevel=0.03)
    S.box("hood_cap", 1.14, -3.10, 1.22, 1.86, -2.60, 1.28, "steel", bevel=0.015)
    P.glow(S.box("hood_lamp", 1.24, -3.04, 1.28, 1.76, -2.92, 1.295, "backdrop", bevel=0.004), "lamp_warm")
    P._rod("spout", (1.5, -2.96, 1.02), (1.5, -3.30, 1.02), 0.06, "steel_dark", segs=14, bevel=0.0)
    S.cyl("nozzle", 1.5, -3.30, 0.075, 0.90, 1.08, "metal_dark", r_top=0.06, segs=14, bevel=0.005)
    # the tray under the nozzle, a dollop of paste in it
    S.box("tray", 1.16, -3.84, 0.76, 1.84, -3.18, 0.80, "paste_tray", bevel=0.012)
    S.box("tray_well", 1.22, -3.78, 0.795, 1.78, -3.24, 0.805, "steel_dark", bevel=0.0)
    S.blob("dollop", 1.5, -3.55, 0.80, 0.15, "paste", seed=31, lumps=0.12, squash=0.45)
    # a control console at the west end of the counter: a screen and keys
    S.box("console", 0.40, -3.70, 0.76, 0.98, -2.84, 0.82, "steel_dark", bevel=0.015)
    P.glow(S.box("screen", 0.46, -3.20, 0.82, 0.92, -2.90, 0.83, "backdrop", bevel=0.004), "indicator")
    for k in range(3):
        for j in range(2):
            x, y = 0.54 + k * 0.15, -3.36 - j * 0.14
            S.box("key%d%d" % (k, j), x - 0.05, y - 0.045, 0.82, x + 0.05, y + 0.045, 0.84, "metal_dark",
                  bevel=0.004)
    # three ready lamps at the east end
    S.box("lamp_plate", 2.02, -3.70, 0.76, 2.60, -2.84, 0.80, "steel_dark", bevel=0.012)
    for k, y in enumerate((-3.00, -3.27, -3.54)):
        S.cyl("lamp_ring%d" % k, 2.31, y, 0.085, 0.80, 0.81, "metal_dark", segs=16, bevel=0.0)
        P.glow(S.cyl("ready%d" % k, 2.31, y, 0.06, 0.80, 0.825, "backdrop", segs=16, bevel=0.0),
               "paste_ready_glow")
    S.box("cable", 2.70, -2.20, 0.0, 2.78, -0.20, 0.08, "fur_black", bevel=0.01)
    return (3, 4), 1.84


# -------------------------------------------------------------------- meal
def _paste_meal():
    """A moulded steel tray: one big well piled with a piped swirl of paste,
    two small wells with flat dabs."""
    S.box("tray", 0.18, -0.78, 0.0, 0.82, -0.32, 0.05, "paste_tray", bevel=0.02)
    S.box("rim_n", 0.18, -0.36, 0.04, 0.82, -0.32, 0.08, "paste_tray", bevel=0.008)
    S.box("rim_s", 0.18, -0.78, 0.04, 0.82, -0.74, 0.08, "paste_tray", bevel=0.008)
    S.box("rim_w", 0.18, -0.78, 0.04, 0.22, -0.32, 0.08, "paste_tray", bevel=0.008)
    S.box("rim_e", 0.78, -0.78, 0.04, 0.82, -0.32, 0.08, "paste_tray", bevel=0.008)
    S.box("divide", 0.56, -0.74, 0.04, 0.59, -0.36, 0.075, "paste_tray", bevel=0.005)
    S.box("divide2", 0.59, -0.565, 0.04, 0.78, -0.545, 0.075, "paste_tray", bevel=0.005)
    # the big well: a piped swirl, three shrinking turns
    for k, (r, z, h) in enumerate(((0.15, 0.05, 0.06), (0.11, 0.10, 0.055), (0.07, 0.15, 0.05))):
        S.ellipsoid("swirl%d" % k, 0.385 + 0.012 * k, -0.555, z + h * 0.5, r, r * 0.9, h, "paste", segs=20, rings=10)
    S.ellipsoid("tip", 0.41, -0.555, 0.205, 0.03, 0.03, 0.035, "paste", segs=10, rings=6)
    # the small wells
    for k, y in enumerate((-0.455, -0.66)):
        S.blob("dab%d" % k, 0.685, y, 0.04, 0.065, "paste", seed=40 + k, lumps=0.15, squash=0.4)
    return S.render_asset("item", "nutrient_paste_meal", zmax=0.24,
                          extra={"item": "nutrient paste meal", "stack": True})


# ------------------------------------------------------------------- build
BUILDERS = {
    "hopper": (_hopper, {"building": "hopper"}),
    "nutrient_paste_dispenser": (_dispenser, {"building": "nutrient paste dispenser",
                                              "attachments": {"worker": [1.5, 4.15]}}),
}

# icons: ui_icon/<name> cut from the sprite
ICONS = {
    "hopper": ("furniture", "hopper"),
    "nutrient_paste_dispenser": ("furniture", "nutrient_paste_dispenser"),
    "nutrient_paste_meal": ("item", "nutrient_paste_meal"),
}


def build(only=None):
    """The hopper, the dispenser (+ its glow layer), the paste meal and their icons."""
    out = []
    for name, (fn, extra) in BUILDERS.items():
        if only and name not in only:
            continue
        fp, zmax = fn()
        lits = P.glow_colours()
        ex = dict(extra, rotatable=False, facing="south")
        gname = P.glow_name(name)
        if lits:   # emissive parts get a glow layer, drawn while powered
            ex.update(emissive=lits[0], glow="furniture/" + gname)
        out.append(S.render_asset("furniture", name, footprint=fp, zmax=zmax, extra=ex))
        if lits:
            fn()
            out.append(P.glow_layer("furniture", gname, fp, zmax, "furniture/" + name, lits[0]))
    if not only or "nutrient_paste_meal" in only:
        out.append(_paste_meal())
    if not only:
        for icon, (fam, spr) in ICONS.items():
            p = O._icon(icon, fam, spr)
            if p:
                out.append(p)
    return out
