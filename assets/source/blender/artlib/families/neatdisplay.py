"""Display storage furniture (ART spec §12-§13, §4; NST-001..004, 008, 009,
011): pieces that show what they hold. Our own designs: nothing traced.

Sprites land in furniture/. Rotatable pieces are authored facing SOUTH (the
open or glazed front points there) and turned with stage.face(); the frames
are `wood`/`wood_plank` and take the renderer's stuff colour shift like the
shelf (storage.py), which multiplies the whole sprite. What ties the set
together: a tall dark back with a stepped crest on the open shelves and
cabinets, boards in planks like the shelf's, and a wine felt lining under the
glass of the case and cabinets.

Glass has no alpha in this pipeline, so it is drawn the way a flat colour
picture draws it: the lining stays in view through an open top, with pale
`glass` streaks lying across it and a solid pane only on the upright front.
Items draw on top of the sprite, so they read as lying behind the glass.
Items are never baked in.

- furniture/display_case_<facing> (1x1, 1 slot): a slim framed glass box on a
  stepped plinth, a wooden back and a small brass lock.
- furniture/round_pedestal (1x1, 1 slot, no facings): a turned stand: a round
  foot, a bellied shaft with beads and a felt-topped dish.
- furniture/small_display_shelf_<facing> (1x1, 3 slots) and
  display_shelf_<facing> (2x1, 3 slots a cell): one solid board with a lip
  over a dark low board, in front of the crested back.
- furniture/two_tier_display_shelf_<facing> (2x1, 6 slots a cell): two boards
  stepped back and up on one frame, so both rows stay in view.
- furniture/narrow_display_cabinet_<facing> (1x1, 4 slots) and
  display_cabinet_<facing> (2x1, 4 slots a cell): a closed wooden body with
  paired doors under a felt-lined glazed top, split by glazing bars into one
  pane per slot.

Metadata: `capacity` (stacks a cell), `slot_scale` (the item sprite's scale)
and `attachments` slot0..slot<capacity-1> in canvas px for the footprint's
first cell (top left); the renderer adds the cell offset. Slots are numbered
north to south then west to east in each facing, so a later stack draws over
an earlier one the right way round. Icons ui_icon/<slug> are cut from the
south sprites.

    blender -b --factory-startup -P assets/source/blender/build.py -- neatdisplay
"""
from .. import contracts as C
from .. import stage as S
from . import overlay as O
from . import power as P

FACINGS = ("south", "east", "north", "west")
FAMILY = "furniture"

# The same depth as the shelf.
Y0, Y1 = -0.9, -0.12
LINING = "felt_wine"
BRASS = "brass"


def _streak(name, x, y, length, width, z, material="glass"):
    """A glint on a pane: a thin bar leaning north-east from (x, y)."""
    pts = [(x, y), (x + width, y), (x + width + length, y + length), (x + length, y + length)]
    S.prism(name, pts, z, z + 0.004, material, bevel=0.0)


def _cheek(name, x, top, step_y, step_z, width=0.05):
    """A side board stepping down towards the front, its foot on the floor."""
    S.box(name + "_lo", x, Y0, 0.0, x + width, Y1, step_z, "wood", bevel=0.008)
    S.box(name + "_hi", x, step_y, step_z, x + width, Y1, top, "wood", bevel=0.008)


def _crest(n, y0, z0, top, rise=0.1):
    """The crested back: a dark panel that items stand out against, framed in
    wood, boarded behind (it faces the viewer when the piece faces north), one
    raised step centred on each cell and a capping rail."""
    x0, x1 = 0.07, n - 0.07
    skin = 0.014
    S.box("back", x0 + 0.02, y0, z0, x1 - 0.02, Y1 - skin, top, "bark", bevel=0.0)
    for k in range(3 * n):
        S.box("back_board%d" % k, x0 + k * (x1 - x0) / (3 * n) + 0.006, Y1 - skin, z0,
              x0 + (k + 1) * (x1 - x0) / (3 * n) - 0.006, Y1, top, "wood_plank", bevel=0.0)
    S.box("cap", x0 - 0.01, y0 - 0.008, top - 0.012, x1 + 0.01, Y1 + 0.008, top + 0.018, "wood", bevel=0.006)
    for k in range(n):
        S.box("crest%d" % k, k + 0.28, y0, top, k + 0.72, Y1, top + rise, "wood", bevel=0.01)
        S.box("crest_cap%d" % k, k + 0.25, y0 - 0.006, top + rise - 0.008, k + 0.75, Y1 + 0.006, top + rise + 0.016,
              "wood_plank", bevel=0.005)


def _planks(name, x0, x1, y0, y1, z0, z1, count=3, gap=0.024):
    """A board of `count` planks along x, on a dark plate that shows in the gaps."""
    S.box(name + "_plate", x0 + 0.01, y0 + 0.01, z0 - 0.02, x1 - 0.01, y1 - 0.01, z1 - 0.012, "bark", bevel=0.0)
    d = (y1 - y0) / count
    for k in range(count):
        S.box("%s%d" % (name, k), x0, y0 + k * d + gap / 2, z0, x1, y0 + (k + 1) * d - gap / 2, z1,
              "wood_plank", bevel=0.008)


# --------------------------------------------------------------- the case
def _display_case():
    """A glass box on a plinth; the back is wood, so the facing shows."""
    S.box("kick", 0.09, -0.91, 0.0, 0.91, -0.11, 0.05, "bark", bevel=0.006)
    S.box("plinth", 0.07, -0.93, 0.05, 0.93, -0.09, 0.24, "wood", bevel=0.012)
    S.box("plinth_top", 0.1, -0.9, 0.24, 0.9, -0.12, 0.3, "wood_plank", bevel=0.01)
    S.box("lining", 0.13, -0.87, 0.3, 0.87, -0.15, 0.32, LINING, bevel=0.0)
    top = 0.8
    x0, x1, y0, y1 = 0.11, 0.89, -0.89, -0.13
    for k, x in enumerate((x0, x1 - 0.05)):
        for j, y in enumerate((y0, y1 - 0.05)):
            S.box("post%d%d" % (k, j), x, y, 0.3, x + 0.05, y + 0.05, top, "wood", bevel=0.006)
    # the panes: glass on the front and the flanks, wood at the back
    S.box("pane_s", x0 + 0.05, y0 + 0.012, 0.32, x1 - 0.05, y0 + 0.03, top - 0.02, "glass", bevel=0.0)
    S.box("board_n", x0 + 0.05, y1 - 0.04, 0.32, x1 - 0.05, y1 - 0.01, top - 0.02, "wood_plank", bevel=0.0)
    S.box("pane_w", x0 + 0.012, y0 + 0.05, 0.32, x0 + 0.03, y1 - 0.05, top - 0.02, "glass", bevel=0.0)
    S.box("pane_e", x1 - 0.03, y0 + 0.05, 0.32, x1 - 0.012, y1 - 0.05, top - 0.02, "glass", bevel=0.0)
    # the top frame, open: the lining shows through, with glints across it
    S.box("rail_s", x0, y0, top, x1, y0 + 0.06, top + 0.04, "wood", bevel=0.006)
    S.box("rail_n", x0, y1 - 0.06, top, x1, y1, top + 0.04, "wood", bevel=0.006)
    S.box("rail_w", x0, y0, top, x0 + 0.06, y1, top + 0.04, "wood", bevel=0.006)
    S.box("rail_e", x1 - 0.06, y0, top, x1, y1, top + 0.04, "wood", bevel=0.006)
    _streak("glint0", 0.2, -0.82, 0.28, 0.1, top + 0.01)
    _streak("glint1", 0.38, -0.82, 0.16, 0.05, top + 0.01)
    _streak("glint2", 0.56, -0.5, 0.25, 0.07, top + 0.01)
    # streaks on the upright pane, the lock under it
    S.box("pane_hi", 0.24, y0 + 0.006, 0.42, 0.3, y0 + 0.012, 0.72, "snow", bevel=0.0)
    S.box("pane_hi2", 0.34, y0 + 0.006, 0.52, 0.37, y0 + 0.012, 0.72, "snow", bevel=0.0)
    S.box("lock", 0.45, -0.94, 0.12, 0.55, -0.925, 0.2, BRASS, bevel=0.004)
    return (1, 1), top + 0.04


PEDESTAL_Z = 0.866


def _pedestal():
    """A stand turned from one post: every part is round."""
    cx, cy = 0.5, -0.5
    S.cyl("foot", cx, cy, 0.37, 0.0, 0.06, "wood", segs=32, r_top=0.35, bevel=0.012)
    S.cyl("foot2", cx, cy, 0.29, 0.06, 0.12, "wood_plank", segs=28, r_top=0.25, bevel=0.012)
    S.cyl("bead_lo", cx, cy, 0.18, 0.12, 0.17, "bark", segs=24, bevel=0.016)
    S.cyl("shaft_lo", cx, cy, 0.1, 0.17, 0.26, "wood", segs=20, r_top=0.15, bevel=0.0)
    S.ellipsoid("belly", cx, cy, 0.33, 0.175, 0.175, 0.09, "wood_plank", segs=24, rings=10)
    S.cyl("shaft_hi", cx, cy, 0.14, 0.4, 0.66, "wood", segs=20, r_top=0.085, bevel=0.0)
    S.cyl("bead_hi", cx, cy, 0.13, 0.66, 0.7, "bark", segs=20, bevel=0.014)
    S.cyl("flare", cx, cy, 0.1, 0.7, 0.8, "wood", segs=24, r_top=0.27, bevel=0.0)
    S.cyl("dish", cx, cy, 0.285, 0.8, 0.86, "wood_plank", segs=32, bevel=0.016)
    S.cyl("felt", cx, cy, 0.225, 0.86, PEDESTAL_Z, LINING, segs=28, bevel=0.0)
    return (1, 1), 0.87


# -------------------------------------------------------------- the shelves
BOARD_Z = 0.47


def _display_shelf(n):
    """n cells wide: one solid board with a lip, a dark low board, posts at
    the ends and between cells, cheeks stepping down to the front."""
    def build():
        x0, x1 = 0.07, n - 0.07
        for k in range(n + 1):
            x = x0 if k == 0 else (x1 - 0.06 if k == n else k - 0.03)
            S.box("post_f%d" % k, x, Y0, 0.0, x + 0.06, Y0 + 0.06, BOARD_Z - 0.05, "wood", bevel=0.01)
            S.box("post_r%d" % k, x, Y1 - 0.06, 0.0, x + 0.06, Y1, BOARD_Z - 0.05, "wood", bevel=0.01)
        _cheek("cheek_w", x0 - 0.01, 0.76, -0.5, BOARD_Z + 0.03)
        _cheek("cheek_e", x1 - 0.04, 0.76, -0.5, BOARD_Z + 0.03)
        S.box("low", x0 + 0.04, Y0 + 0.03, 0.1, x1 - 0.04, Y1 - 0.03, 0.15, "bark", bevel=0.01)
        S.box("kick", x0 + 0.06, Y0 - 0.005, 0.02, x1 - 0.06, Y0 + 0.03, 0.09, "bark", bevel=0.006)
        S.box("under", x0 + 0.04, Y0 + 0.02, BOARD_Z - 0.09, x1 - 0.04, Y1 - 0.06, BOARD_Z - 0.05, "wood", bevel=0.006)
        _planks("board", x0 + 0.04, x1 - 0.04, Y0 + 0.02, Y1 - 0.06, BOARD_Z - 0.04, BOARD_Z)
        S.box("lip", x0 + 0.04, Y0 - 0.01, BOARD_Z - 0.055, x1 - 0.04, Y0 + 0.035, BOARD_Z + 0.03, "wood", bevel=0.008)
        _crest(n, Y1 - 0.06, 0.0, 0.9)
        return (n, 1), 1.016
    return build


LOW_Z, HIGH_Z = 0.31, 0.65


def _two_tier():
    """Two cells wide: a low front board, a high rear board and a dark riser
    between them, so the rows stand apart and neither hides the other."""
    n = 2
    x0, x1 = 0.07, n - 0.07
    for k in range(n + 1):
        x = x0 if k == 0 else (x1 - 0.06 if k == n else k - 0.03)
        S.box("post_f%d" % k, x, Y0, 0.0, x + 0.06, Y0 + 0.06, LOW_Z - 0.04, "wood", bevel=0.01)
        S.box("post_m%d" % k, x, -0.53, 0.0, x + 0.06, -0.47, HIGH_Z - 0.04, "wood", bevel=0.01)
        S.box("post_r%d" % k, x, Y1 - 0.06, 0.0, x + 0.06, Y1, HIGH_Z - 0.04, "wood", bevel=0.01)
    for name, x in (("cheek_w", x0 - 0.01), ("cheek_e", x1 - 0.04)):
        S.box(name + "_lo", x, Y0, 0.0, x + 0.05, Y1, LOW_Z + 0.03, "wood", bevel=0.008)
        S.box(name + "_hi", x, -0.52, LOW_Z + 0.03, x + 0.05, Y1, 0.78, "wood", bevel=0.008)
    S.box("kick", x0 + 0.06, Y0 - 0.005, 0.02, x1 - 0.06, Y0 + 0.03, 0.09, "bark", bevel=0.006)
    S.box("under", x0 + 0.04, Y0 + 0.02, LOW_Z - 0.08, x1 - 0.04, -0.53, LOW_Z - 0.04, "bark", bevel=0.006)
    _planks("board_lo", x0 + 0.04, x1 - 0.04, Y0 + 0.02, -0.52, LOW_Z - 0.04, LOW_Z, count=2)
    S.box("lip_lo", x0 + 0.04, Y0 - 0.01, LOW_Z - 0.045, x1 - 0.04, Y0 + 0.03, LOW_Z + 0.025, "wood", bevel=0.008)
    S.box("riser", x0 + 0.04, -0.52, LOW_Z, x1 - 0.04, -0.49, HIGH_Z - 0.04, "bark", bevel=0.004)
    _planks("board_hi", x0 + 0.04, x1 - 0.04, -0.5, Y1 - 0.05, HIGH_Z - 0.04, HIGH_Z, count=2)
    S.box("lip_hi", x0 + 0.04, -0.53, HIGH_Z - 0.045, x1 - 0.04, -0.495, HIGH_Z + 0.025, "wood", bevel=0.008)
    _crest(n, Y1 - 0.05, 0.0, 0.84, rise=0.08)
    return (n, 1), 0.936


# ------------------------------------------------------------- the cabinets
CAB_Z = 0.56


def _cabinet(n):
    """n cells wide: a closed body with paired doors, a felt-lined top glazed
    in one pane per slot, and the crested back."""
    def build():
        x0, x1 = 0.06, n - 0.06
        fy0, fy1 = -0.92, Y1 - 0.08
        S.box("kick", x0 + 0.03, fy0 + 0.02, 0.0, x1 - 0.03, Y1, 0.05, "bark", bevel=0.006)
        S.box("body", x0, fy0, 0.05, x1, Y1, CAB_Z - 0.04, "wood", bevel=0.012)
        # paired doors on each cell, with a seam, panel and brass knob each
        for k in range(n):
            for j, (d0, d1) in enumerate(((k + 0.09, k + 0.49), (k + 0.51, k + 0.91))):
                S.box("door%d%d" % (k, j), d0, fy0 - 0.012, 0.08, d1, fy0 + 0.01, CAB_Z - 0.08, "wood_plank", bevel=0.005)
                S.box("inset%d%d" % (k, j), d0 + 0.05, fy0 - 0.016, 0.14, d1 - 0.05, fy0 - 0.008, CAB_Z - 0.14, "bark", bevel=0.0)
            S.box("knob_w%d" % k, k + 0.42, fy0 - 0.03, 0.26, k + 0.47, fy0 - 0.008, 0.32, BRASS, bevel=0.004)
            S.box("knob_e%d" % k, k + 0.53, fy0 - 0.03, 0.26, k + 0.58, fy0 - 0.008, 0.32, BRASS, bevel=0.004)
        # the top: frame, lining, one glazing bar along and one per cell across
        S.box("top_s", x0 - 0.01, fy0 - 0.01, CAB_Z - 0.04, x1 + 0.01, fy0 + 0.06, CAB_Z + 0.03, "wood_plank", bevel=0.008)
        S.box("top_w", x0 - 0.01, fy0, CAB_Z - 0.04, x0 + 0.06, fy1 + 0.01, CAB_Z + 0.03, "wood_plank", bevel=0.008)
        S.box("top_e", x1 - 0.06, fy0, CAB_Z - 0.04, x1 + 0.01, fy1 + 0.01, CAB_Z + 0.03, "wood_plank", bevel=0.008)
        S.box("lining", x0 + 0.05, fy0 + 0.05, CAB_Z - 0.04, x1 - 0.05, fy1, CAB_Z - 0.01, LINING, bevel=0.0)
        mid = (fy0 + 0.06 + fy1) / 2
        S.box("bar_ew", x0 + 0.06, mid - 0.016, CAB_Z - 0.01, x1 - 0.06, mid + 0.016, CAB_Z + 0.025, "wood", bevel=0.004)
        for k in range(n):
            S.box("bar_ns%d" % k, k + 0.484, fy0 + 0.06, CAB_Z - 0.01, k + 0.516, fy1, CAB_Z + 0.025, "wood", bevel=0.004)
            if k:
                S.box("bar_cell%d" % k, k - 0.02, fy0 + 0.06, CAB_Z - 0.01, k + 0.02, fy1, CAB_Z + 0.03, "wood_plank", bevel=0.004)
            for j, (sx, sy) in enumerate(((k + 0.13, mid + 0.04), (k + 0.56, mid + 0.04), (k + 0.13, fy0 + 0.1), (k + 0.56, fy0 + 0.1))):
                _streak("glint%d%d" % (k, j), sx, sy, 0.15, 0.05, CAB_Z)
        _crest(n, Y1 - 0.08, CAB_Z - 0.04, 0.82, rise=0.08)
        return (n, 1), 0.916
    return build


# ------------------------------------------------------------------- slots
def _row(y, z, xs=(0.2, 0.5, 0.8)):
    return [(x, y, z) for x in xs]


CASE_SLOT = [(0.5, -0.5, 0.32)]
PEDESTAL_SLOT = [(0.5, -0.5, PEDESTAL_Z)]
SHELF_SLOTS = _row(-0.5, BOARD_Z)
TIER_SLOTS = _row(-0.31, HIGH_Z, (0.19, 0.5, 0.81)) + _row(-0.7, LOW_Z, (0.19, 0.5, 0.81))
CAB_SLOTS = _row(-0.345, CAB_Z, (0.28, 0.72)) + _row(-0.675, CAB_Z, (0.28, 0.72))


def _cell_points(points, facing):
    """The first cell's slots in `facing`, north to south then west to east.
    A cell turns about its own centre, so every cell of a piece shares them."""
    turned = [P.turn(p, facing, (1, 1)) if facing else p for p in points]
    turned.sort(key=lambda p: (round(-(p[1] + C.SHEAR_K * p[2]), 3), round(p[0], 3)))
    return {"slot%d" % k: p for k, p in enumerate(turned)}


# slug: (builder, build name, per-cell slots, slot_scale, facings)
PIECES = {
    "display_case": (_display_case, "display case", CASE_SLOT, 0.72, FACINGS),
    "round_pedestal": (_pedestal, "round pedestal", PEDESTAL_SLOT, 0.55, None),
    "small_display_shelf": (_display_shelf(1), "small display shelf", SHELF_SLOTS, 0.42, FACINGS),
    "display_shelf": (_display_shelf(2), "display shelf", SHELF_SLOTS, 0.42, FACINGS),
    "two_tier_display_shelf": (_two_tier, "two tier display shelf", TIER_SLOTS, 0.36, FACINGS),
    "narrow_display_cabinet": (_cabinet(1), "narrow display cabinet", CAB_SLOTS, 0.4, FACINGS),
    "display_cabinet": (_cabinet(2), "display cabinet", CAB_SLOTS, 0.4, FACINGS),
}


def build(only=None):
    """Every display piece in its facings and its icon (ui_icon/<slug>);
    `only` (PIECES slugs) renders just those."""
    out = []
    for slug, (fn, name, slots, scale, facings) in PIECES.items():
        if only and slug not in only:
            continue
        extra = {"building": name, "capacity": len(slots), "slot_scale": scale}
        for f in facings or (None,):
            sprite = slug + ("_" + f if f else "")
            out += P.render(sprite, fn, facing=f, extra=extra, family=FAMILY,
                            after=lambda facing, pts=slots: _cell_points(pts, facing))
        p = O._icon(slug, FAMILY, slug + ("_south" if facings else ""))
        if p:
            out.append(p)
    return out
