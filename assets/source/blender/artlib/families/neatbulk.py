"""Neat storage, bulk pieces (NST-010, -012, -013, -020..-027; ART spec §12-§13):
open and tall shelves, pallets, crates and the corpse dump spot.

Original forms, built like storage.py's shelf: frames in `wood`, boards in
`wood_plank`, dark `bark` under the gaps, no tint mask; stone and steel stuff
colour them through the renderer's default stuff colour shift. Authored facing
south (a shelf's open front points there) and turned with stage.face().

Each sprite's metadata carries `capacity`, `slot_scale` and `attachments`
`slot0..slot<cap-1>` in canvas px for the FIRST CELL of the footprint (all
cells share the layout; the renderer adds the cell offset). Slots are written
in painter order, north to south, so later stacks overlap earlier ones
correctly in every facing. Items are never baked in. Crates list slots too,
though the renderer draws nothing there: their slats hide the contents, full
or empty.
"""
import math

from .. import stage as S
from . import overlay as O
from . import power as P

FACINGS = ("south", "east", "north", "west")
FAMILY = "furniture"

# ------------------------------------------------------------------ slots
# Cell-local layouts: (dx, dy, z) from the centre of a cell, authored south.
COLS = (-0.27, 0.0, 0.27)


def _two_rows(z_front, z_back, dy_front=-0.225, dy_back=0.225):
    """Two rows of three, one on each board of a shelf."""
    return [(dx, dy_back, z_back) for dx in COLS] + [(dx, dy_front, z_front) for dx in COLS]


def _heap(z):
    """3x2 heap: the back row staggered and lifted a little onto the front one."""
    return ([(-0.21, 0.17, z + 0.03), (0.05, 0.2, z + 0.03), (0.27, 0.15, z + 0.03)]
            + [(-0.27, -0.17, z), (-0.01, -0.21, z), (0.24, -0.15, z)])


def _slots(layout, facing, zmax, head=None):
    """Attachments for layout in the first cell of the faced footprint. The
    first `head` slots (the only ones drawn on a dump spot) stay ahead of the rest."""
    a = math.radians(S.FACINGS[facing])
    c, s = math.cos(a), math.sin(a)
    px = [P.project((0.5 + dx * c - dy * s, -0.5 + dx * s + dy * c, z), zmax) for dx, dy, z in layout]
    key = lambda p: (p[1], p[0])
    n = len(px) if head is None else head
    ordered = sorted(px[:n], key=key) + sorted(px[n:], key=key)
    return {"slot%d" % i: p for i, p in enumerate(ordered)}


# --------------------------------------------------------------- builders
OPEN_TOP = 0.46


def _open_shelf():
    """2x1: two wide boards a tile on low trestles, a dark board below, a low
    lip along the back. Lower and plainer than the shelf."""
    w = 2
    x0, x1, y0, y1 = 0.05, w - 0.05, -0.94, -0.06
    top = OPEN_TOP
    S.box("board_low", x0 + 0.03, y0 + 0.03, 0.05, x1 - 0.03, y1 - 0.03, 0.09, "bark", bevel=0.008)
    for k, x in enumerate((x0, w / 2 - 0.045, x1 - 0.09)):      # trestle: two legs, a foot and a bearer
        for j, y in enumerate((y0 + 0.02, y1 - 0.1)):
            S.box("leg%d%d" % (k, j), x + 0.005, y, 0.0, x + 0.085, y + 0.08, top - 0.08, "wood", bevel=0.012)
        S.box("bearer%d" % k, x, y0, top - 0.11, x + 0.09, y1, top - 0.05, "wood", bevel=0.01)
    for c in range(w):                                          # each tile has its own pair of boards
        bx0 = max(x0, c + 0.015)
        bx1 = min(x1, c + 0.985)
        S.box("board_f%d" % c, bx0, y0, top - 0.04, bx1, -0.53, top, "wood_plank", bevel=0.012)
        S.box("board_b%d" % c, bx0, -0.47, top - 0.04, bx1, y1 - 0.05, top, "wood_plank", bevel=0.012)
    S.box("lip", x0, y1 - 0.05, top - 0.05, x1, y1, top + 0.07, "wood", bevel=0.012)
    return (w, 1), top + 0.07


TALL_LOW, TALL_HIGH = 0.3, 0.74


def _tall(w):
    """w x 1 tall unit: a low front board and a high back board, stepped so both
    rows stay in sight, with tall back posts and a top rail."""
    x0, x1, y0, y1 = 0.06, w - 0.06, -0.94, -0.07
    top = 1.08
    S.box("kick", x0 + 0.03, y0 + 0.02, 0.0, x1 - 0.03, y1 - 0.02, 0.09, "bark", bevel=0.008)
    S.box("riser", x0 + 0.03, -0.53, TALL_LOW - 0.05, x1 - 0.03, -0.47, TALL_HIGH, "bark", bevel=0.006)
    S.box("back", x0 + 0.03, y1 - 0.05, TALL_HIGH - 0.05, x1 - 0.03, y1 - 0.015, top - 0.04, "bark", bevel=0.006)
    for k in range(w + 1):
        x = x0 + (x1 - x0 - 0.08) * k / w
        S.box("post_f%d" % k, x, y0, 0.0, x + 0.08, y0 + 0.08, TALL_LOW + 0.05, "wood", bevel=0.012)
        S.box("post_m%d" % k, x, -0.54, 0.0, x + 0.08, -0.46, TALL_HIGH + 0.05, "wood", bevel=0.012)
        S.box("post_b%d" % k, x, y1 - 0.08, 0.0, x + 0.08, y1, top, "wood", bevel=0.012)
        S.box("side_l%d" % k, x + 0.01, y0 + 0.05, TALL_LOW - 0.1, x + 0.07, -0.5, TALL_LOW - 0.05, "wood_plank", bevel=0.006)
        S.box("side_h%d" % k, x + 0.01, -0.5, TALL_HIGH - 0.1, x + 0.07, y1 - 0.05, TALL_HIGH - 0.05, "wood_plank", bevel=0.006)
    for c in range(w):
        bx0 = max(x0 + 0.02, c + 0.05)
        bx1 = min(x1 - 0.02, c + 0.95)
        S.box("low%d" % c, bx0, y0 + 0.015, TALL_LOW - 0.05, bx1, -0.5, TALL_LOW, "wood_plank", bevel=0.01)
        S.box("high%d" % c, bx0, -0.47, TALL_HIGH - 0.05, bx1, y1 - 0.04, TALL_HIGH, "wood_plank", bevel=0.01)
    S.box("rail", x0, y1 - 0.075, top - 0.07, x1, y1 + 0.005, top, "wood", bevel=0.012)
    return (w, 1), top


PALLET_TOP = 0.2


def _pallet(w, h, along_y=False):
    """w x h low board platform: a slat deck over three runners on blocks, a dark
    bottom board so the gaps and fork openings read. A heavier lead board on the
    south edge turns with the facing. along_y: the slats run north-south."""
    x0, x1, y0, y1 = 0.04, w - 0.04, -h + 0.04, -0.04
    S.box("bottom", x0 + 0.01, y0 + 0.01, 0.0, x1 - 0.01, y1 - 0.01, 0.03, "bark", bevel=0.006)
    deck0, deck1 = 0.14, PALLET_TOP
    n_run = 3 if max(w, h) == 1 or along_y else 2 * max(w, h) - 1
    a0, a1 = (y0, y1) if along_y else (x0, x1)    # along the slats: runners stand across them
    for k in range(n_run):
        p = a0 + (a1 - a0 - 0.1) * k / (n_run - 1)
        if along_y:
            S.box("runner%d" % k, x0 + 0.01, p + 0.01, 0.09, x1 - 0.01, p + 0.09, deck0, "wood", bevel=0.006)
            for j, b in enumerate((x0, (x0 + x1) / 2 - 0.05, x1 - 0.1)):
                S.box("block%d%d" % (k, j), b, p, 0.0, b + 0.1, p + 0.1, 0.1, "wood", bevel=0.008)
        else:
            S.box("runner%d" % k, p + 0.01, y0 + 0.01, 0.09, p + 0.09, y1 - 0.01, deck0, "wood", bevel=0.006)
            for j, b in enumerate((y0, (y0 + y1) / 2 - 0.05, y1 - 0.1)):
                S.box("block%d%d" % (k, j), p, b, 0.0, p + 0.1, b + 0.1, 0.1, "wood", bevel=0.008)
    b0, b1 = (x0, x1) if along_y else (y0, y1)    # across the slats
    pitch = 0.2
    n = int(round((b1 - b0) / pitch))
    gap = 0.038
    step = (b1 - b0 + gap) / n
    for k in range(n):
        s0 = b0 + k * step
        s1 = s0 + step - gap
        mat = "wood" if k % 3 == 1 else "wood_plank"
        if along_y:
            S.box("slat%d" % k, s0, y0, deck0, s1, y1, deck1, mat, bevel=0.01)
        else:
            S.box("slat%d" % k, x0, s0, deck0, x1, s1, deck1, mat, bevel=0.01)
    S.box("lead", x0, y0 - 0.005, deck0 - 0.01, x1, y0 + 0.07, deck1 + 0.012, "wood", bevel=0.012)
    return (w, h), deck1 + 0.012


def _slats(ax0, ay0, ax1, ay1, z):
    """Boards across a crate's opening with wide dark gaps, sunk under the rim."""
    n = max(2, int(round((ay1 - ay0) / 0.2)))
    gap = 0.07
    step = (ay1 - ay0 + gap) / n
    for k in range(n):
        y = ay0 + k * step
        mat = "wood" if k % 3 == 1 else "wood_plank"
        S.box("slat%d" % k, ax0, y, z, ax1, y + step - gap, z + 0.03, mat, bevel=0.008)


def _crate(w, h, top, courses, bands=(), post=False, cross=False, front=False):
    """w x h plank box without a lid: plank courses round a dark core, corner
    posts and a rim, spaced slats nailed across the dark opening. The contents
    are not drawn, and the slats look the same full or empty."""
    x0, x1, y0, y1 = 0.06, w - 0.06, -h + 0.06, -0.06
    t = 0.06
    inner = top - 0.07
    S.box("core", x0 + 0.015, y0 + 0.015, 0.0, x1 - 0.015, y1 - 0.015, inner, "bark", bevel=0.0)
    dz = (top - 0.035) / courses
    for k in range(courses):
        z0, z1 = k * dz + 0.045, (k + 1) * dz
        mat = "wood" if k % 3 == 1 else "wood_plank"
        S.box("wall_s%d" % k, x0, y0, z0, x1, y0 + t, z1, mat, bevel=0.008)
        S.box("wall_n%d" % k, x0, y1 - t, z0, x1, y1, z1, mat, bevel=0.008)
        S.box("wall_w%d" % k, x0, y0, z0, x0 + t, y1, z1, mat, bevel=0.008)
        S.box("wall_e%d" % k, x1 - t, y0, z0, x1, y1, z1, mat, bevel=0.008)
    rims = ((x0, y0, x1, y0 + t), (x0, y1 - t, x1, y1), (x0, y0 + t, x0 + t, y1 - t), (x1 - t, y0 + t, x1, y1 - t))
    for k, (a, b, c, d) in enumerate(rims):
        S.box("rim%d" % k, a - 0.008, b - 0.008, top - 0.045, c + 0.008, d + 0.008, top, "wood", bevel=0.01)
    _slats(x0 + t, y0 + t, x1 - t, y1 - t, inner)
    for k, (cx, cy) in enumerate(((x0, y0), (x1 - 0.09, y0), (x0, y1 - 0.09), (x1 - 0.09, y1 - 0.09))):
        S.box("corner%d" % k, cx - 0.008, cy - 0.008, 0.0, cx + 0.098, cy + 0.098, top + 0.01, "wood", bevel=0.014)
    if cross:                                                   # a batten over the cloth where two tiles meet, north-south
        for k in range(1, w):
            S.box("batten_x%d" % k, k - 0.05, y0 + 0.01, top - 0.02, k + 0.05, y1 - 0.01, top + 0.02, "wood", bevel=0.012)
    for k, zb in enumerate(bands):                              # steel straps
        z0, z1 = zb, zb + 0.15
        S.box("band_s%d" % k, x0 - 0.012, y0 - 0.012, z0, x1 + 0.012, y0 + 0.02, z1, "steel", bevel=0.006)
        S.box("band_w%d" % k, x0 - 0.012, y0, z0, x0 + 0.02, y1, z1, "steel", bevel=0.006)
        S.box("band_e%d" % k, x1 - 0.02, y0, z0, x1 + 0.012, y1, z1, "steel", bevel=0.006)
    if post:                                                    # a heavy post down the middle of the south face
        S.box("post", w / 2 - 0.06, y0 - 0.016, 0.0, w / 2 + 0.06, y0 + 0.02, top, "wood", bevel=0.012)
    if front:                                                   # the crate's front: a rope grip on a cleat, a heavy back rim
        cxm = w / 2
        S.box("cleat", cxm - 0.24, y0 - 0.016, top * 0.3, cxm + 0.24, y0 + 0.01, top * 0.3 + 0.17, "bark", bevel=0.008)
        S.box("hasp", cxm - 0.05, y0 - 0.024, top * 0.2, cxm + 0.05, y0 + 0.01, top - 0.05, "metal_dark", bevel=0.008)
        S.box("back_rim", x0 + 0.09, y1 - 0.105, top - 0.04, x1 - 0.09, y1 - 0.005, top + 0.02, "wood", bevel=0.012)
    return (w, h), top + (0.02 if cross or front else 0.01)


TARP_Z = 0.035


def _dump_spot():
    """1x1: a ragged patch of bare dirt, a dark tarp pulled out to four stakes by
    its corners, so its edges sag in; a taller marker stake with a rag. No stains."""
    S.prism("patch", [(0.1, -0.97), (0.34, -0.93), (0.62, -0.97), (0.9, -0.95), (0.97, -0.72),
                      (0.94, -0.45), (0.97, -0.2), (0.86, -0.05), (0.56, -0.08), (0.3, -0.04),
                      (0.08, -0.07), (0.03, -0.3), (0.06, -0.58), (0.03, -0.82)], 0.0, 0.012, "soil_rich", bevel=0.0)
    S.prism("tarp", [(0.13, -0.87), (0.5, -0.78), (0.87, -0.87), (0.78, -0.5), (0.87, -0.13),
                     (0.5, -0.22), (0.13, -0.13), (0.22, -0.5)], 0.012, TARP_Z, "stone_dark", bevel=0.006)
    stakes = ((0.08, -0.92), (0.92, -0.92), (0.92, -0.08), (0.08, -0.08))
    for k, (x, y) in enumerate(stakes):
        z1 = 0.66 if k == 3 else 0.26                           # the north-west stake carries the marker
        S.cyl("stake%d" % k, x, y, 0.048, 0.0, z1, "wood", segs=8, r_top=0.026, bevel=0.006).visible_shadow = False
    marker = [S.box("rag_knot", 0.04, -0.12, 0.5, 0.12, -0.04, 0.56, "burlap", bevel=0.008),
              S.box("rag_a", 0.1, -0.12, 0.4, 0.25, -0.08, 0.6, "burlap", bevel=0.0),
              S.box("rag_b", 0.25, -0.12, 0.34, 0.38, -0.08, 0.54, "burlap", bevel=0.0)]
    for ob in marker:                                           # its shadow must not read as a stain on the tarp
        ob.visible_shadow = False
    return (1, 1), 0.66


# ----------------------------------------------------------------- pieces
PALLET_SLOTS = _heap(PALLET_TOP)
SHELF_SCALE = 0.4
PALLET_SCALE = 0.44
CRATE_SCALE = 0.4


def _crate_slots(top):
    return _heap(top - 0.07)


def _dump_slots():
    """Twelve heap points on the tarp: the four the renderer draws, spread, then the rest."""
    z = TARP_Z
    return [(-0.17, 0.16, z), (0.17, 0.17, z), (-0.16, -0.14, z), (0.18, -0.13, z),
            (0.0, 0.02, z), (-0.26, 0.0, z), (0.26, 0.02, z), (0.02, 0.26, z),
            (0.0, -0.26, z), (-0.27, -0.26, z), (0.27, -0.26, z), (-0.05, -0.1, z)]


# slug: (builder, facings or None, layout, slot_scale, capacity, head)
PIECES = {
    "open_shelf": (_open_shelf, FACINGS, _two_rows(OPEN_TOP, OPEN_TOP), SHELF_SCALE, 6, None),
    "narrow_tall_shelf": (lambda: _tall(1), FACINGS, _two_rows(TALL_LOW, TALL_HIGH), SHELF_SCALE, 6, None),
    "tall_shelf": (lambda: _tall(2), FACINGS, _two_rows(TALL_LOW, TALL_HIGH), SHELF_SCALE, 6, None),
    "mini_pallet": (lambda: _pallet(1, 1), None, PALLET_SLOTS, PALLET_SCALE, 6, None),
    "half_pallet": (lambda: _pallet(2, 1, along_y=True), FACINGS, PALLET_SLOTS, PALLET_SCALE, 6, None),
    "pallet": (lambda: _pallet(2, 2), None, PALLET_SLOTS, PALLET_SCALE, 6, None),
    "small_crate": (lambda: _crate(1, 1, 0.44, 2), None, _crate_slots(0.44), CRATE_SCALE, 6, None),
    "crate": (lambda: _crate(2, 1, 0.5, 3, cross=True, front=True), FACINGS, _crate_slots(0.5), CRATE_SCALE, 6, None),
    "large_crate": (lambda: _crate(2, 2, 0.62, 3, post=True, cross=True), None, _crate_slots(0.62), CRATE_SCALE, 6, None),
    "tall_crate": (lambda: _crate(2, 2, 1.3, 7, bands=(0.26, 0.96), cross=True), None, _crate_slots(1.3), CRATE_SCALE, 6, None),
    "corpse_dump_spot": (_dump_spot, None, _dump_slots(), 0.45, 12, 4),
}


def _render(slug, spec, facing):
    fn, _, layout, scale, cap, head = spec
    fp0, zmax = fn()
    fp = S.face(S.objects(), facing, fp0) if facing else fp0
    extra = {"building": slug.replace("_", " "), "rotatable": facing is not None, "facing": facing or "south",
             "capacity": cap, "slot_scale": scale, "attachments": _slots(layout, facing or "south", zmax, head)}
    return S.render_asset(FAMILY, slug + ("_" + facing if facing else ""), footprint=fp, zmax=zmax, extra=extra)


def build(only=None):
    """Every piece in its facings and its Architect icon; `only` (slugs) limits them."""
    out = []
    for slug, spec in PIECES.items():
        if only and slug not in only:
            continue
        for f in spec[1] or (None,):
            out.append(_render(slug, spec, f))
        p = O._icon(slug, FAMILY, slug + ("_south" if spec[1] else ""))
        if p:
            out.append(p)
    return out
