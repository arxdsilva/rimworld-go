"""Bedroom family (ART spec §12-§13, BED-1): bedroll, double bedroll, animal
bed, end table, dresser, and the animal sleeping spot floor marking, plus
their Architect icons.

Sprites land in the furniture output family (the spot in overlay, beside
the colonist sleeping spot) and follow the bed rules in furniture.py:
geometry authored facing SOUTH (a sleeper's feet / a piece's front point
there), rotated with stage.face(); wood frames are drawn in `wood` and take
no stuff tint (stone/steel stuff is handled by the renderer as for beds);
the one tint group per sprite is `cloth` (blankets, the animal bed cushion).
"""
import numpy as np

from .. import contracts as C
from .. import meta
from .. import stage as S
from . import overlay as O

FACINGS = ("south", "east", "north", "west")


def _head_roll(x0, x1, y, tag=""):
    """A stuffed canvas head roll lying across the bedroll, two leather ties."""
    cx, rx = (x0 + x1) / 2, (x1 - x0) / 2
    S.ellipsoid("roll" + tag, cx, y, 0.12, rx, 0.14, 0.095, "burlap", segs=20, rings=10)
    for k, sx in enumerate((-1, 1)):
        S.ellipsoid("tie%s%d" % (tag, k), cx + sx * rx * 0.58, y, 0.12, 0.028, 0.152, 0.104, "leather",
                    segs=12, rings=8)


def _bedroll():
    """1x2 bedroll: a leather ground sheet, a wool pad, a canvas head
    roll tied with straps and a blanket turned back at the head. No frame:
    it lies on the floor, lower than a bed."""
    S.box("sheet", 0.07, -1.96, 0.0, 0.93, -0.04, 0.035, "leather", bevel=0.025)
    S.box("pad", 0.17, -1.87, 0.02, 0.83, -0.13, 0.09, "wool_cream", bevel=0.035, segments=2)
    _head_roll(0.20, 0.80, -0.30)
    S.box("blanket", 0.13, -1.91, 0.05, 0.87, -0.76, 0.15, "cloth", bevel=0.04, segments=2)
    S.box("fold", 0.13, -0.88, 0.07, 0.87, -0.68, 0.19, "cloth", bevel=0.05, segments=2)
    return (1, 2), 0.24


def _bedroll_double():
    """2x2 double bedroll: one wide ground sheet and pad, two head rolls and a
    shared blanket."""
    S.box("sheet", 0.06, -1.96, 0.0, 1.94, -0.04, 0.035, "leather", bevel=0.025)
    S.box("pad", 0.15, -1.87, 0.02, 1.85, -0.13, 0.09, "wool_cream", bevel=0.035, segments=2)
    S.box("seam_mid", 0.992, -1.86, 0.085, 1.008, -0.14, 0.092, "burlap", bevel=0.0)
    _head_roll(0.20, 0.92, -0.30, "a")
    _head_roll(1.08, 1.80, -0.30, "b")
    S.box("blanket", 0.11, -1.91, 0.05, 1.89, -0.76, 0.15, "cloth", bevel=0.04, segments=2)
    S.box("fold", 0.11, -0.88, 0.07, 1.89, -0.68, 0.19, "cloth", bevel=0.05, segments=2)
    return (2, 2), 0.24


def _animal_bed():
    """1x1 animal bed: a round woven basket, wicker bands round its wall and a
    domed cushion (cloth, tinted) sunk inside the rim, low at the front so
    an animal can step in."""
    S.cyl("base", 0.5, -0.5, 0.42, 0.0, 0.03, "wood", segs=28, bevel=0.01)
    S.cyl("wall", 0.5, -0.5, 0.40, 0.02, 0.20, "straw", segs=28, bevel=0.03)
    for k, z in enumerate((0.07, 0.14)):
        S.cyl("band%d" % k, 0.5, -0.5, 0.408, z - 0.012, z + 0.012, "wood_plank", segs=28, bevel=0.0)
    S.cyl("rim", 0.5, -0.5, 0.415, 0.185, 0.225, "wood_plank", segs=28, bevel=0.015)
    S.ellipsoid("cushion", 0.5, -0.52, 0.19, 0.335, 0.32, 0.075, "cloth", segs=24, rings=10)
    S.ellipsoid("dent", 0.5, -0.56, 0.235, 0.20, 0.17, 0.035, "cloth", segs=20, rings=8)
    return (1, 1), 0.27


def _end_table():
    """1x1 end table: a bedside cabinet with one drawer on its front (south),
    a brass pull, short feet, and a book and a candle on top."""
    for k, (x, y) in enumerate([(0.20, -0.26), (0.80, -0.26), (0.20, -0.80), (0.80, -0.80)]):
        S.box("foot%d" % k, x - 0.045, y - 0.045, 0.0, x + 0.045, y + 0.045, 0.07, "bark", bevel=0.01)
    S.box("body", 0.14, -0.86, 0.06, 0.86, -0.20, 0.48, "wood", bevel=0.03, segments=2)
    S.box("top", 0.10, -0.90, 0.47, 0.90, -0.16, 0.55, "wood_plank", bevel=0.03)
    S.box("drawer", 0.20, -0.875, 0.29, 0.80, -0.855, 0.43, "wood_plank", bevel=0.01)
    S.box("pull", 0.44, -0.89, 0.345, 0.56, -0.87, 0.375, "brass", bevel=0.005)
    # a leather-bound book with its page edge showing, and a candle on a brass dish
    S.box("book", 0.20, -0.66, 0.55, 0.56, -0.30, 0.58, "leather", bevel=0.01)
    S.box("pages", 0.215, -0.665, 0.555, 0.545, -0.315, 0.585, "wool_cream", bevel=0.0)
    S.box("book_top", 0.20, -0.66, 0.585, 0.56, -0.30, 0.605, "leather", bevel=0.008)
    S.cyl("dish", 0.71, -0.66, 0.095, 0.55, 0.57, "brass", segs=16, bevel=0.006)
    S.cyl("candle", 0.71, -0.66, 0.045, 0.57, 0.70, "wool_cream", segs=12, bevel=0.008)
    return (1, 1), 0.70


def _lean_back(objs, degrees, y, z):
    """Tilt objs back (top toward north) about the x-parallel axis at (y, z),
    so a south-facing panel shows its face from above in every facing."""
    import math
    from mathutils import Matrix
    m = (Matrix.Translation((0, y, z)) @ Matrix.Rotation(math.radians(-degrees), 4, "X")
         @ Matrix.Translation((0, -y, -z)))
    S.transform(objs, m)


def _dresser():
    """2x1 dresser: a long chest of drawers (two columns of two) with brass
    pulls on a plinth; on its top a framed swing mirror tilted back, a stack of
    folded linen and a lidded jar."""
    S.box("plinth", 0.10, -0.86, 0.0, 1.90, -0.20, 0.06, "bark", bevel=0.015)
    S.box("body", 0.08, -0.88, 0.05, 1.92, -0.18, 0.60, "wood", bevel=0.03, segments=2)
    S.box("top", 0.05, -0.92, 0.59, 1.95, -0.14, 0.67, "wood_plank", bevel=0.025)
    S.box("divider", 0.985, -0.895, 0.08, 1.015, -0.88, 0.57, "bark", bevel=0.0)
    for col, x0 in enumerate((0.14, 1.04)):
        for row, (z0, z1) in enumerate(((0.10, 0.31), (0.35, 0.56))):
            S.box("drawer%d%d" % (col, row), x0, -0.905, z0, x0 + 0.82, -0.88, z1, "wood_plank", bevel=0.012)
            cx = x0 + 0.41
            S.box("pull%d%d" % (col, row), cx - 0.08, -0.92, (z0 + z1) / 2 - 0.018, cx + 0.08, -0.90,
                  (z0 + z1) / 2 + 0.018, "brass", bevel=0.006)
    # swing mirror: wooden frame and glass, tilted back between two posts
    n0 = len(S.objects())
    S.box("mirror_frame", 0.20, -0.62, 0.67, 0.92, -0.56, 1.10, "wood", bevel=0.02)
    S.box("mirror", 0.26, -0.63, 0.72, 0.86, -0.61, 1.05, "glass", bevel=0.0)
    _lean_back(S.objects()[n0:], 52, -0.56, 0.67)
    for k, x in enumerate((0.165, 0.955)):   # the swing posts either side
        S.box("post%d" % k, x - 0.03, -0.50, 0.67, x + 0.03, -0.40, 0.86, "bark", bevel=0.008)
    # folded linen, two layers, and a lidded jar
    S.box("linen0", 1.10, -0.74, 0.67, 1.50, -0.36, 0.73, "wool_cream", bevel=0.02, segments=2)
    S.box("linen1", 1.13, -0.70, 0.73, 1.47, -0.40, 0.78, "burlap", bevel=0.02, segments=2)
    S.cyl("jar", 1.72, -0.58, 0.075, 0.67, 0.79, "ceramic", segs=14, bevel=0.012)
    S.cyl("jar_lid", 1.72, -0.58, 0.05, 0.79, 0.82, "brass", segs=12, bevel=0.006)
    return (2, 1), 0.98


BUILDERS = {
    "bedroll": (_bedroll, {"attachments": {"sleeper": [0.5, 0.42]}, "building": "bedroll"}),
    "bedroll_double": (_bedroll_double, {"attachments": {"sleeper": [0.5, 0.42], "sleeper2": [1.5, 0.42]},
                                         "building": "double bedroll"}),
    "animal_bed": (_animal_bed, {"attachments": {"sleeper": [0.5, 0.5]}, "building": "animal bed"}),
    "end_table": (_end_table, {"building": "end table"}),
    "dresser": (_dresser, {"building": "dresser"}),
}
ROTATABLE = {"bedroll", "bedroll_double", "dresser"}

# Blanket overlays (as furniture.BLANKETS): drawn over a sleeper so the
# blanket reads as tucked in; the rest of the bedroll is a holdout.
BLANKETS = {"bedroll": (_bedroll, {"blanket", "fold"}),
            "bedroll_double": (_bedroll_double, {"blanket", "fold"})}

# Architect icons, cut from the south sprite (ui_icon/<name>).
ICONS = {
    "bedroll": ("furniture", "bedroll_south"),
    "double_bedroll": ("furniture", "bedroll_double_south"),
    "animal_bed": ("furniture", "animal_bed"),
    "end_table": ("furniture", "end_table"),
    "dresser": ("furniture", "dresser_south"),
    "animal_sleeping_spot": ("overlay", "animal_sleeping_spot"),
}


def _animal_sleeping_spot():
    """1x1 floor marking for an animal to sleep on: a dashed chalk ring with
    a paw mark in it, drawn flat like the colonist sleeping spot."""
    N = C.PPT
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32) + 0.5
    img = np.zeros((N, N, 4), np.float32)
    chalk = (0.93, 0.89, 0.80)
    shade = (0.10, 0.09, 0.08)
    c = N / 2
    R = N / 2 - 14.0
    r = np.hypot(xx - c, yy - c)
    ring = np.abs(r - R)
    dash = (np.floor((np.arctan2(yy - c, xx - c) + np.pi) * R / 11.0) % 2) == 0
    O._paint(img, O._cov(r - R), shade, 0.16)                 # faint ground shade
    O._paint(img, O._cov(ring - 2.6) * dash, shade, 0.45)     # halo
    O._paint(img, O._cov(ring - 1.8) * dash, chalk, 0.9)
    # paw: a rounded heel pad and four toe pads above it
    heel = np.hypot((xx - c) / 15.0, (yy - (c + 9)) / 11.5) - 1.0
    paw = heel * 11.5
    for dx, dy, rr in ((-16, -7, 5.6), (-6, -17, 6.0), (6, -17, 6.0), (16, -7, 5.6)):
        paw = np.minimum(paw, np.hypot(xx - (c + dx), yy - (c + dy)) - rr)
    O._paint(img, O._cov(paw - 1.4), shade, 0.35)
    O._paint(img, O._cov(paw), chalk, 0.85)
    name = "animal_sleeping_spot"
    path = S.out_path("overlay", name)
    S.write_png(img, path)
    meta.write("overlay", name, {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0],
                                 "overlay": "animal_sleeping_spot", "marking": True, "rotatable": False})
    return path


def _blankets():
    out = []
    for name, (fn, keep) in BLANKETS.items():
        for f in FACINGS:
            fp, zmax = fn()
            S.holdout([o for o in S.objects() if o.name.split(".")[0] not in keep])
            fp = S.face(S.objects(), f, fp)
            out.append(S.render_asset("furniture", "%s_blanket_%s" % (name, f), footprint=fp, zmax=zmax,
                                      shadow=False, extra={"overlay": "blanket", "facing": f}))
    return out


def build(only=None):
    """Every bedroom sprite and icon; `only` (names from BUILDERS) renders just those."""
    out = [] if only else _blankets() + [_animal_sleeping_spot()]
    for name, (fn, extra) in BUILDERS.items():
        if only and name not in only:
            continue
        for f in FACINGS if name in ROTATABLE else ("south",):
            fp0, zmax = fn()
            fp = S.face(S.objects(), f, fp0)
            suffix = "_" + f if name in ROTATABLE else ""
            ex = dict(extra, rotatable=name in ROTATABLE, facing=f)
            out.append(S.render_asset("furniture", name + suffix, footprint=fp, zmax=zmax, extra=ex))
    if not only:
        for icon, (fam, spr) in ICONS.items():
            p = O._icon(icon, fam, spr)
            if p:
                out.append(p)
    return out
