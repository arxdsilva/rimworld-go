"""Furniture family (ART spec §12-§14): bed, table, chair, workbench, stove,
butcher table, lamp. (Sleeping spots are floor markings: overlay family.)

Geometry is authored facing SOUTH (the side a user approaches from /
the sleeper's feet point to) and rotated with stage.face().
"""
from .. import stage as S

FACINGS = ("south", "east", "north", "west")


def _bed():
    S.box("frame", 0.07, -1.96, 0.0, 0.93, -0.04, 0.26, "wood", bevel=0.04)
    S.box("headboard", 0.05, -0.17, 0.0, 0.95, -0.02, 0.58, "wood", bevel=0.045)
    S.box("post_l", 0.05, -1.96, 0.0, 0.17, -1.84, 0.34, "wood", bevel=0.03)
    S.box("post_r", 0.83, -1.96, 0.0, 0.95, -1.84, 0.34, "wood", bevel=0.03)
    S.box("mattress", 0.12, -1.88, 0.20, 0.88, -0.15, 0.36, "ceramic", bevel=0.06, segments=2)
    S.box("pillow", 0.20, -0.52, 0.34, 0.80, -0.21, 0.46, "ceramic", bevel=0.07, segments=2)
    S.box("blanket", 0.10, -1.90, 0.30, 0.90, -0.72, 0.43, "cloth", bevel=0.05, segments=2)
    S.box("fold", 0.10, -0.84, 0.34, 0.90, -0.64, 0.47, "cloth", bevel=0.06, segments=2)
    return (1, 2), 0.58


def _table():
    for i, (x, y) in enumerate([(0.12, -0.12), (0.88, -0.12), (0.12, -0.88), (0.88, -0.88)]):
        S.box("leg%d" % i, x - 0.05, y - 0.05, 0.0, x + 0.05, y + 0.05, 0.50, "wood", bevel=0.015)
    S.box("apron", 0.10, -0.90, 0.40, 0.90, -0.10, 0.50, "wood", bevel=0.02)
    S.box("top", 0.04, -0.96, 0.50, 0.96, -0.04, 0.58, "wood_plank", bevel=0.03)
    S.box("seam", 0.49, -0.95, 0.575, 0.51, -0.05, 0.582, "wood", bevel=0.0)
    return (1, 1), 0.58


def _chair():
    for i, (x, y) in enumerate([(0.27, -0.30), (0.73, -0.30), (0.27, -0.74), (0.73, -0.74)]):
        S.box("leg%d" % i, x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.32, "wood", bevel=0.01)
    S.box("seat", 0.20, -0.80, 0.30, 0.80, -0.24, 0.38, "wood_plank", bevel=0.03)
    S.box("back", 0.20, -0.30, 0.38, 0.80, -0.20, 0.78, "wood", bevel=0.03)
    S.box("rail", 0.24, -0.29, 0.62, 0.76, -0.21, 0.72, "wood_plank", bevel=0.02)
    return (1, 1), 0.78


def _workbench():
    # 2 x 1, worker stands on the south side.
    for i, x in enumerate([0.12, 1.88]):
        for j, y in enumerate([-0.18, -0.82]):
            S.box("leg%d%d" % (i, j), x - 0.06, y - 0.06, 0.0, x + 0.06, y + 0.06, 0.52, "metal_dark", bevel=0.015)
    S.box("shelf", 0.10, -0.86, 0.14, 1.90, -0.14, 0.18, "wood", bevel=0.015)
    S.box("top", 0.04, -0.94, 0.52, 1.96, -0.06, 0.62, "steel", bevel=0.03)
    S.box("board", 0.06, -0.20, 0.62, 1.94, -0.06, 1.05, "wood_plank", bevel=0.03)
    S.box("vise_base", 0.20, -0.72, 0.62, 0.48, -0.42, 0.72, "metal_dark", bevel=0.02)
    S.box("vise_jaw", 0.24, -0.82, 0.66, 0.44, -0.70, 0.78, "steel", bevel=0.02)
    S.box("toolbox", 1.38, -0.62, 0.62, 1.80, -0.34, 0.80, "food_red", bevel=0.03)
    S.box("toolbox_lid", 1.36, -0.64, 0.80, 1.82, -0.32, 0.84, "metal_dark", bevel=0.015)
    S.cyl("hammer_handle", 0.95, -0.55, 0.03, 0.62, 0.66, "wood", segs=8, ry=0.25)
    S.box("hammer_head", 0.78, -0.64, 0.62, 0.86, -0.46, 0.70, "steel", bevel=0.015)
    for i, x in enumerate([0.70, 0.95, 1.20]):
        S.box("peg%d" % i, x - 0.025, -0.24, 0.80, x + 0.025, -0.19, 0.98, "metal_dark", bevel=0.005)
    S.box("light", 1.55, -0.20, 0.96, 1.70, -0.15, 1.02, "indicator", bevel=0.0)
    return (2, 1), 1.05


def _stove():
    # 2 x 1, the cook stands on the south side.
    S.box("body", 0.06, -0.92, 0.0, 1.94, -0.10, 0.62, "metal_dark", bevel=0.04)
    S.box("top", 0.04, -0.94, 0.62, 1.96, -0.08, 0.68, "steel", bevel=0.03)
    for i, (cx, cy) in enumerate([(0.45, -0.48), (1.0, -0.48)]):
        S.cyl("ring%d" % i, cx, cy, 0.17, 0.68, 0.71, "metal_dark", segs=20, bevel=0.0)
        S.cyl("glow%d" % i, cx, cy, 0.10, 0.70, 0.715, "lamp_warm", segs=20, bevel=0.0)
    S.cyl("pot", 0.45, -0.48, 0.15, 0.71, 0.86, "steel", r_top=0.16, segs=18)
    S.cyl("stew", 0.45, -0.48, 0.13, 0.855, 0.865, "crop_fruit", segs=18, bevel=0.0)
    S.box("board", 1.40, -0.72, 0.68, 1.86, -0.26, 0.72, "wood_plank", bevel=0.015)
    S.ellipsoid("veg", 1.55, -0.45, 0.76, 0.06, 0.06, 0.05, "crop_fruit", segs=10, rings=6)
    S.box("door", 0.20, -0.935, 0.12, 1.00, -0.90, 0.48, "steel", bevel=0.015)
    S.box("handle", 0.30, -0.95, 0.40, 0.90, -0.93, 0.43, "metal_dark", bevel=0.005)
    S.box("pipe", 1.70, -0.20, 0.68, 1.84, -0.10, 1.10, "metal_dark", bevel=0.02)
    return (2, 1), 1.10


def _butcher_table():
    # 2 x 1, the butcher stands on the south side.
    for i, x in enumerate([0.14, 1.86]):
        for j, y in enumerate([-0.2, -0.8]):
            S.box("leg%d%d" % (i, j), x - 0.07, y - 0.07, 0.0, x + 0.07, y + 0.07, 0.5, "wood", bevel=0.02)
    S.box("stretcher", 0.12, -0.56, 0.12, 1.88, -0.44, 0.18, "wood", bevel=0.015)
    S.box("slab", 0.04, -0.94, 0.50, 1.96, -0.06, 0.66, "wood_plank", bevel=0.04, segments=2)
    S.box("block", 0.12, -0.86, 0.66, 0.86, -0.18, 0.74, "wood", bevel=0.03)
    for i, y in enumerate([-0.34, -0.52, -0.70]):
        S.box("groove%d" % i, 0.16, y - 0.008, 0.738, 0.82, y + 0.008, 0.744, "bark", bevel=0.0)
    S.box("blade", 1.05, -0.70, 0.66, 1.42, -0.48, 0.69, "steel", bevel=0.01)
    S.box("handle", 1.42, -0.62, 0.66, 1.62, -0.56, 0.70, "metal_dark", bevel=0.01)
    S.ellipsoid("cut", 1.62, -0.30, 0.71, 0.13, 0.09, 0.05, "food_red", segs=14, rings=8)
    S.ellipsoid("bone", 1.62, -0.30, 0.74, 0.05, 0.03, 0.02, "ceramic", segs=10, rings=6)
    S.box("rail", 0.06, -0.10, 0.66, 1.94, -0.04, 1.02, "wood", bevel=0.02)
    for i, x in enumerate([0.40, 0.80, 1.20]):
        S.cyl("hook%d" % i, x, -0.12, 0.012, 0.86, 1.0, "steel", segs=6, bevel=0.0)
    S.ellipsoid("hang", 1.20, -0.13, 0.78, 0.07, 0.04, 0.09, "food_red", segs=12, rings=8)
    return (2, 1), 1.02


def _lamp():
    S.cyl("base", 0.5, -0.5, 0.22, 0.0, 0.06, "metal_dark", segs=20)
    S.cyl("pole", 0.5, -0.5, 0.035, 0.06, 1.25, "steel", segs=10)
    S.cyl("shade", 0.5, -0.5, 0.26, 1.18, 1.38, "metal_dark", r_top=0.16, segs=20)
    S.cyl("bulb", 0.5, -0.5, 0.15, 1.38, 1.40, "lamp_warm", segs=20, bevel=0.0)
    return (1, 1), 1.40


BUILDERS = {
    "bed_basic": (_bed, {"attachments": {"sleeper": [0.5, 0.42]}}),
    "table_small": (_table, {}),
    "chair_basic": (_chair, {"attachments": {"seat": [0.5, 0.52]}}),
    "workbench_basic": (_workbench, {"attachments": {"worker": [0.5, 1.15]}}),
    "lamp_standing": (_lamp, {"emissive": "lamp_warm", "light_radius": 6}),
    "stove_basic": (_stove, {"attachments": {"cook": [0.25, 1.5]}}),
    "butcher_table": (_butcher_table, {"attachments": {"worker": [0.5, 1.15]}}),
}
ROTATABLE = {"bed_basic", "chair_basic", "workbench_basic", "stove_basic", "butcher_table"}


# Blanket overlays: drawn over a sleeper so they read as tucked in. The
# rest of the bed is a holdout, so the overlay lines up pixel for pixel.
BLANKETS = {"bed_basic": (_bed, {"blanket", "fold", "post_l", "post_r"})}


def _blankets():
    out = []
    for name, (fn, keep) in BLANKETS.items():
        for f in FACINGS:
            fp, zmax = fn()
            hold = [o for o in S.objects() if o.name.split(".")[0] not in keep]
            S.holdout(hold)
            fp = S.face(S.objects(), f, fp)
            out.append(S.render_asset("furniture", "%s_blanket_%s" % (name, f), footprint=fp, zmax=zmax,
                                      shadow=False, extra={"overlay": "blanket", "facing": f}))
    return out


def build():
    out = _blankets()
    for name, (fn, extra) in BUILDERS.items():
        facings = FACINGS if name in ROTATABLE else ("south",)
        for f in facings:
            fp, zmax = fn()
            fp = S.face(S.objects(), f, fp)
            ex = dict(extra, rotatable=name in ROTATABLE, facing=f)
            suffix = "_" + f if name in ROTATABLE else ""
            out.append(S.render_asset("furniture", name + suffix, footprint=fp, zmax=zmax, extra=ex))
    return out
