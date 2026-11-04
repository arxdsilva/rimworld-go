"""Neat storage racks and closed stores (NST-005...007, 014...019; ART §12-§13).

Nine pieces, original forms, each with its own silhouette: a pegged weapon
rack in wood, a stepped steel rack that lays weapons flat, a tall steel rack
with sockets and combs that stands them on end, two wardrobes with their doors
swung open, a two-rod textile frame, a gantry of meat hooks over a drip tray, a
safe and a drum. Items are never baked in: the renderer draws stored stacks over
the sprite at the slot attachments, and the safe and the drum draw none.

Rotatable pieces are authored facing SOUTH (open front to the south) and turned
by stage.face() through power.render. Wood frames take the renderer's default
stuff colour shift like the shelf (storage.py); the steel pieces are drawn in
`steel` and `metal_dark`.

Slots (metadata attachments slot0...slot<cap-1>, canvas px of the point a stack's
centre sits on) are given for the first cell of the footprint only. All cells of
a piece share the layout, so every piece repeats its pegs, hooks or rods per
cell; the renderer adds the cell offset. A slot is authored in the south-facing
cell and turned about that cell's centre.
"""
import math

from .. import stage as S
from . import overlay as O
from . import power as P

FACINGS = ("south", "east", "north", "west")

X3 = (0.2, 0.5, 0.8)        # three slots along a cell
X2 = (0.28, 0.72)           # two slots along a cell
TWO_WIDE = ("weapon_rack", "horizontal_weapon_rack", "vertical_weapon_rack", "wardrobe", "meat_hook")


def _plank(name, a, b, width, z0, z1, material, bevel=0.006):
    """A thin upright board on the plan line a-b (an open door leaf)."""
    ax, ay = a
    bx, by = b
    dx, dy = bx - ax, by - ay
    n = math.hypot(dx, dy)
    nx, ny = -dy / n * width / 2, dx / n * width / 2
    S.prism(name, [(ax + nx, ay + ny), (ax - nx, ay - ny), (bx - nx, by - ny), (bx + nx, by + ny)],
            z0, z1, material, bevel=bevel)


def _row(y, z, xs=X3):
    return [(x, y, z) for x in xs]


# --------------------------------------------------------------- weapon racks
def _weapon_rack():
    """2x1 in wood: a tall dark backboard with a comb of light pegs reaching south
    and a low butt rail along the front; the comb reads from above. The board's
    outer face is plank, for the facing that shows its back."""
    x0, x1 = 0.06, 1.94
    S.box("board_low", x0 + 0.02, -0.92, 0.0, x1 - 0.02, -0.12, 0.08, "bark", bevel=0.006)
    S.box("back", x0 + 0.02, -0.17, 0.05, x1 - 0.02, -0.13, 0.88, "bark", bevel=0.006)
    S.box("back_out", x0 + 0.02, -0.13, 0.05, x1 - 0.02, -0.09, 0.88, "wood_plank", bevel=0.006)
    for k, x in enumerate((x0, 0.965, x1 - 0.07)):
        S.box("post%d" % k, x, -0.2, 0.0, x + 0.07, -0.1, 0.92, "wood", bevel=0.012)
        S.box("foot%d" % k, x - 0.005, -0.94, 0.0, x + 0.075, -0.1, 0.09, "wood", bevel=0.012)
        S.box("frontpost%d" % k, x + 0.005, -0.87, 0.06, x + 0.065, -0.8, 0.37, "wood", bevel=0.01)
    S.box("rail_top", x0 - 0.01, -0.21, 0.82, x1 + 0.01, -0.09, 0.92, "wood", bevel=0.012)
    S.box("rail_butt", x0, -0.88, 0.3, x1, -0.79, 0.37, "wood", bevel=0.01)
    for c in range(2):
        for k, x in enumerate(X3):
            S.box("peg%d%d" % (c, k), c + x - 0.03, -0.52, 0.76, c + x + 0.03, -0.17, 0.82, "wood", bevel=0.008)
            S.box("tip%d%d" % (c, k), c + x - 0.04, -0.56, 0.76, c + x + 0.04, -0.49, 0.88, "wood", bevel=0.012)
    return (2, 1), 0.92


def _horizontal_weapon_rack():
    """2x1 in steel: two stepped decks of long rails with saddle lugs, the rear
    deck higher, so laid weapons sit in two clear rows."""
    x0, x1 = 0.06, 1.94
    S.box("tray", x0 + 0.02, -0.92, 0.04, x1 - 0.02, -0.1, 0.08, "metal_dark", bevel=0.008)
    for k, x in enumerate((x0, 0.97, x1 - 0.06)):
        S.box("leg_f%d" % k, x, -0.92, 0.0, x + 0.06, -0.86, 0.34, "metal_dark", bevel=0.008)
        S.box("leg_r%d" % k, x, -0.16, 0.0, x + 0.06, -0.1, 0.6, "metal_dark", bevel=0.008)
        S.box("beam_f%d" % k, x, -0.92, 0.28, x + 0.06, -0.5, 0.34, "metal_dark", bevel=0.008)
        S.box("beam_r%d" % k, x, -0.5, 0.54, x + 0.06, -0.1, 0.6, "metal_dark", bevel=0.008)
        S.box("riser%d" % k, x, -0.53, 0.0, x + 0.06, -0.47, 0.6, "metal_dark", bevel=0.008)
    for y, z, tag in ((-0.7, 0.34, "f"), (-0.3, 0.6, "r")):
        for j, dy in enumerate((-0.14, 0.14)):
            S.box("rail_%s%d" % (tag, j), x0, y + dy - 0.035, z, x1, y + dy + 0.035, z + 0.05, "steel", bevel=0.01)
        for c in range(2):
            for k, xl in enumerate((0.05, 0.35, 0.65, 0.95)):
                x = c + xl
                for j, dy in enumerate((-0.14, 0.14)):
                    S.box("lug_%s%d%d%d" % (tag, c, k, j), x - 0.018, y + dy - 0.045, z + 0.03, x + 0.018,
                          y + dy + 0.045, z + 0.11, "steel", bevel=0.006)
    return (2, 1), 0.72


def _vertical_weapon_rack():
    """2x1 in steel: a floor plate of butt sockets in two rows and, overhead, two
    bars with combs that hold the barrels. The tallest of the three racks."""
    x0, x1 = 0.06, 1.94
    S.box("plate", x0, -0.92, 0.0, x1, -0.1, 0.07, "metal_dark", bevel=0.012)
    top = 1.02
    for k, x in enumerate((x0, 0.965, x1 - 0.07)):
        for j, y in enumerate((-0.92, -0.17)):
            S.box("post%d%d" % (k, j), x, y, 0.0, x + 0.07, y + 0.07, top, "steel", bevel=0.01)
        S.box("beam%d" % k, x, -0.92, top - 0.06, x + 0.07, -0.1, top, "steel", bevel=0.01)
    for y, tag in ((-0.3, "r"), (-0.7, "f")):
        S.box("bar_" + tag, x0, y + 0.04, top - 0.09, x1, y + 0.11, top - 0.02, "steel", bevel=0.01)
        for c in range(2):
            for k, xs in enumerate(X3):
                x = c + xs
                S.cyl("socket_%s%d%d" % (tag, c, k), x, y, 0.078, 0.06, 0.12, "steel", segs=14, bevel=0.008)
                S.cyl("bore_%s%d%d" % (tag, c, k), x, y, 0.046, 0.115, 0.126, "bark", segs=12, bevel=0.0)
                for side in (-0.062, 0.062):
                    S.box("tooth_%s%d%d%d" % (tag, c, k, side > 0), x + side - 0.018, y - 0.04, top - 0.08,
                          x + side + 0.018, y + 0.06, top - 0.03, "steel", bevel=0.005)
    return (2, 1), top


# ------------------------------------------------------------------ wardrobes
def _carcass(w):
    """A tall wardrobe body w tiles wide, no lid and no front, its two doors
    swung open as wings at the outer front corners; a hanging rod per cell."""
    x0, x1 = 0.14, w - 0.14
    y0, y1 = -0.78, -0.1
    top = 1.0
    S.box("back", x0, y1 - 0.06, 0.0, x1, y1, top, "wood_plank", bevel=0.008)
    for k in range(int(round((x1 - x0) / 0.18))):        # board lines on the back
        x = x0 + 0.09 + k * 0.18
        S.box("line%d" % k, x - 0.006, y1 - 0.066, 0.1, x + 0.006, y1 - 0.055, top - 0.08, "bark", bevel=0.0)
    S.box("floor", x0 + 0.03, y0, 0.0, x1 - 0.03, y1 - 0.04, 0.09, "bark", bevel=0.006)
    S.box("plinth", x0, y0 - 0.02, 0.0, x1, y0 + 0.05, 0.13, "wood", bevel=0.008)
    for k, (xa, xb) in enumerate(((x0, x0 + 0.07), (x1 - 0.07, x1))):
        S.box("side%d" % k, xa, y0, 0.0, xb, y1, top, "wood", bevel=0.01)
    for c in range(1, w):
        S.box("divider%d" % c, c - 0.03, y0, 0.0, c + 0.03, y1, top, "wood", bevel=0.01)
    S.box("crown_b", x0 - 0.015, y1 - 0.09, top - 0.03, x1 + 0.015, y1 + 0.015, top + 0.05, "wood", bevel=0.012)
    for k, (xa, xb) in enumerate(((x0 - 0.015, x0 + 0.085), (x1 - 0.085, x1 + 0.015))):
        S.box("crown%d" % k, xa, y0 - 0.015, top - 0.03, xb, y1, top + 0.05, "wood", bevel=0.012)
    for c in range(w):
        xa = max(c, x0 + 0.05) if c == 0 else c
        xb = min(c + 1, x1 - 0.05) if c == w - 1 else c + 1
        P._rod("hang%d" % c, (xa, -0.42, top - 0.14), (xb, -0.42, top - 0.14), 0.022, "metal_dark", segs=8, bevel=0.0)
        for k, x in enumerate((0.3, 0.5, 0.7)):          # empty hangers on the rod
            S.box("hanger%d%d" % (c, k), c + x - 0.018, -0.56, top - 0.2, c + x + 0.018, -0.4, top - 0.16, "steel",
                  bevel=0.004)
    # the door leaves, hinged at the front corners
    for k, (a, b, h) in enumerate((((x0 + 0.01, y0 - 0.01), (0.035, -0.95), (0.06, -0.925)),
                                   ((x1 - 0.01, y0 - 0.01), (w - 0.035, -0.95), (w - 0.06, -0.925)))):
        _plank("door%d" % k, a, b, 0.05, 0.03, top - 0.03, "wood_plank")
        _plank("pull%d" % k, (h[0], h[1] - 0.012), (h[0] + (0.016 if k == 0 else -0.016), h[1] + 0.012), 0.022,
               0.42, 0.56, "metal_dark", bevel=0.0)
    return top + 0.05


def _narrow_wardrobe():
    return (1, 1), _carcass(1)


def _wardrobe():
    return (2, 1), _carcass(2)


# -------------------------------------------------------------- textile rack
def _textile_rack():
    """1x1: two stepped end frames carrying two thick rods, the rear one high and
    the front one low, so hung cloth falls in two rows over a clear floor."""
    for k, x in enumerate((0.09, 0.91)):
        S.box("foot%d" % k, x - 0.05, -0.94, 0.0, x + 0.05, -0.06, 0.07, "wood", bevel=0.014)
        S.box("rear%d" % k, x - 0.04, -0.27, 0.03, x + 0.04, -0.19, 0.84, "wood", bevel=0.012)
        S.box("front%d" % k, x - 0.04, -0.81, 0.03, x + 0.04, -0.73, 0.5, "wood", bevel=0.012)
    S.box("tie_r", 0.1, -0.26, 0.14, 0.9, -0.2, 0.2, "wood_plank", bevel=0.008)
    S.box("tie_f", 0.1, -0.8, 0.14, 0.9, -0.74, 0.2, "wood_plank", bevel=0.008)
    for j, (y, z) in enumerate(((-0.32, 0.76), (-0.68, 0.53))):
        P._rod("rod%d" % j, (0.05, y, z), (0.95, y, z), 0.042, "wood", segs=10, bevel=0.0)
        for k, x in enumerate((0.09, 0.91)):
            S.box("seat%d%d" % (j, k), x - 0.05, y - 0.055, z - 0.05, x + 0.05, y + 0.055, z + 0.055, "bark", bevel=0.014)
    return (1, 1), 0.84


# ------------------------------------------------------------------ meat hook
HOOK_RAILS = (-0.28, -0.68)
HOOK_TOP = 0.99


def _hook(tag, base, forward):
    """An S-hook hung from the rail at the faced point base: a short arm towards
    the rack's front clears the rail, the shank drops, and the point curls up to
    the east. Built after facing, so that the curl shows in every facing."""
    x, y, z = base
    fx, fy = forward
    sx, sy = x + fx * 0.07, y + fy * 0.07
    pts = [(x, y, z), (sx, sy, z - 0.04), (sx, sy, z - 0.4), (sx + 0.03, sy, z - 0.47), (sx + 0.07, sy, z - 0.45),
           (sx + 0.085, sy, z - 0.33)]
    for k in range(len(pts) - 1):
        P._rod("hook_%s_%d" % (tag, k), pts[k], pts[k + 1], 0.025, "steel", segs=8, bevel=0.0)
    S.ellipsoid("knob_" + tag, sx, sy, z - 0.04, 0.03, 0.03, 0.03, "steel", segs=8, rings=5)


def _meat_hook():
    """2x1 in steel: portal frames carrying two long rails, three hooks a cell on
    each, over a dark drip tray. All of it reads with nothing on the hooks."""
    x0, x1 = 0.06, 1.94
    top = HOOK_TOP + 0.09
    S.box("tray", x0 + 0.03, -0.93, 0.0, x1 - 0.03, -0.1, 0.05, "metal_dark", bevel=0.008)
    S.box("tray_f", x0 + 0.03, -0.93, 0.03, x1 - 0.03, -0.895, 0.09, "steel", bevel=0.006)
    S.box("tray_b", x0 + 0.03, -0.135, 0.03, x1 - 0.03, -0.1, 0.09, "steel", bevel=0.006)
    for k, x in enumerate((x0, 0.965, x1 - 0.07)):
        S.box("base%d" % k, x - 0.01, -0.95, 0.0, x + 0.08, -0.08, 0.05, "metal_dark", bevel=0.008)
        for j, y in enumerate((-0.93, -0.16)):
            S.box("post%d%d" % (k, j), x, y, 0.0, x + 0.07, y + 0.07, top, "steel", bevel=0.01)
        S.box("beam%d" % k, x, -0.93, top - 0.08, x + 0.07, -0.09, top, "steel", bevel=0.01)
    for j, y in enumerate(HOOK_RAILS):
        S.box("rail%d" % j, x0, y - 0.025, top - 0.07, x1, y + 0.025, top - 0.01, "metal_dark", bevel=0.008)
    return (2, 1), top


def _meat_hooks(facing):
    """The hooks along the faced rails, three a cell on each."""
    ox, oy, _ = P.turn((0.5, -0.5, 0.0), facing, (1, 1))
    fx, fy, _ = P.turn((0.5, -1.5, 0.0), facing, (1, 1))
    forward = (fx - ox, fy - oy)
    for j, y in enumerate(HOOK_RAILS):
        for c in range(2):
            for k, xs in enumerate(X3):
                _hook("%d%d%d" % (j, c, k), P.turn((c + xs, y, HOOK_TOP), facing, (2, 1)), forward)


# --------------------------------------------------------------- closed stores
def _safe():
    """1x1, closed: a squat steel strongbox with two riveted straps over its lid
    and a dark armoured door with a big spoked wheel on the south face."""
    x0, x1, y0, y1 = 0.1, 0.9, -0.84, -0.16
    z = 0.86
    for k, x in enumerate((x0 + 0.02, x1 - 0.14)):
        S.box("foot%d" % k, x, y0 + 0.02, 0.0, x + 0.12, y1 - 0.02, 0.06, "metal_dark", bevel=0.008)
    S.box("body", x0, y0, 0.05, x1, y1, z, "steel", bevel=0.024)
    for k, x in enumerate((x0 + 0.1, x1 - 0.24)):        # the straps over the lid
        S.box("strap%d" % k, x, y0 + 0.08, z - 0.01, x + 0.14, y1 + 0.012, z + 0.024, "metal_dark", bevel=0.008)
        S.box("strap_b%d" % k, x, y1 - 0.01, 0.1, x + 0.14, y1 + 0.012, z, "metal_dark", bevel=0.006)
        for j, y in enumerate((y0 + 0.17, (y0 + y1) / 2, y1 - 0.09)):
            S.cyl("rivet%d%d" % (k, j), x + 0.07, y, 0.026, z + 0.02, z + 0.04, "steel", segs=8, bevel=0.004)
    # the door, proud of the south face
    S.box("door", x0 + 0.05, y0 - 0.03, 0.09, x1 - 0.05, y0 + 0.01, z - 0.04, "metal_dark", bevel=0.012)
    for k, z0 in enumerate((0.17, 0.62)):
        S.box("hinge%d" % k, x0 + 0.005, y0 - 0.045, z0, x0 + 0.1, y0 + 0.005, z0 + 0.1, "steel", bevel=0.006)
    cx, cz = 0.5, 0.46
    for k in range(3):
        a = math.radians(30 + k * 60)
        dx, dz = 0.19 * math.cos(a), 0.3 * math.sin(a)   # taller than round: the shear squats it
        P._rod("spoke%d" % k, (cx - dx, y0 - 0.06, cz - dz), (cx + dx, y0 - 0.06, cz + dz), 0.024, "steel", segs=6,
               bevel=0.0)
    P._rod("hub", (cx, y0 - 0.02, cz), (cx, y0 - 0.08, cz), 0.07, "steel", segs=14, bevel=0.0)
    return (1, 1), z + 0.04


def _drum():
    """1x1, closed: a tall steel barrel with two rolled hoops and a yellow placard,
    a raised rim and two bungs on a dark lid."""
    cx, cy = 0.5, -0.5
    h = 0.88
    S.cyl("body", cx, cy, 0.36, 0.0, h, "steel", segs=32, bevel=0.012)
    S.cyl("chime_low", cx, cy, 0.372, 0.0, 0.04, "metal_dark", segs=32, bevel=0.008)
    for k, z in enumerate((h * 0.3, h * 0.72)):
        S.cyl("hoop%d" % k, cx, cy, 0.374, z - 0.03, z + 0.03, "metal_dark", segs=32, bevel=0.008)
    S.box("placard", cx - 0.13, cy - 0.372, h * 0.36, cx + 0.13, cy - 0.33, h * 0.66, "corn_yellow", bevel=0.004)
    S.cyl("chime", cx, cy, 0.38, h - 0.02, h + 0.03, "steel", segs=32, bevel=0.012)
    S.cyl("lid", cx, cy, 0.325, h + 0.01, h + 0.036, "metal_dark", segs=32, bevel=0.0)
    S.cyl("bung0", cx - 0.15, cy + 0.08, 0.08, h + 0.03, h + 0.065, "steel", segs=14, bevel=0.008)
    S.cyl("bung0t", cx - 0.15, cy + 0.08, 0.038, h + 0.06, h + 0.072, "fur_black", segs=10, bevel=0.0)
    S.cyl("bung1", cx + 0.17, cy - 0.06, 0.055, h + 0.03, h + 0.058, "steel", segs=12, bevel=0.006)
    return (1, 1), h + 0.072


# ----------------------------------------------------------------- the pieces
# Slots: the south-facing layout of one cell, as (x, y, z) on the part a stack
# rests on or hangs from. Infill: (material, height, facings) of a dark plate
# along the footprint's north edge once faced. Under the shear a tall frame stands
# clear of its floor there, and those pockets would fill with the contour's dark
# line; on the north edge the plate is always behind the piece and hides nothing.
# A sixth column, when present, adds parts once faced (the meat hook's hooks).
PIECES = {
    "weapon_rack": (_weapon_rack, FACINGS, 0.42, _row(-0.5, 0.6), ("bark", 0.88, ("east", "west"))),
    "horizontal_weapon_rack": (_horizontal_weapon_rack, FACINGS, 0.36,
                               _row(-0.3, 0.62) + _row(-0.7, 0.36), ("metal_dark", 0.6, FACINGS)),
    "vertical_weapon_rack": (_vertical_weapon_rack, FACINGS, 0.36,
                             _row(-0.3, 0.5) + _row(-0.7, 0.5), ("metal_dark", 0.98, FACINGS)),
    "narrow_wardrobe": (_narrow_wardrobe, FACINGS, 0.42,
                        _row(-0.3, 0.45, X2) + _row(-0.6, 0.45, X2), None),
    "wardrobe": (_wardrobe, FACINGS, 0.42,
                 _row(-0.3, 0.45, X2) + _row(-0.6, 0.45, X2), None),
    "textile_rack": (_textile_rack, FACINGS, 0.38,
                     _row(-0.32, 0.72) + _row(-0.68, 0.49), None),
    "meat_hook": (_meat_hook, FACINGS, 0.38,
                  _row(-0.35, 0.56) + _row(-0.75, 0.56), ("metal_dark", 1.03, FACINGS), _meat_hooks),
    "safe": (_safe, None, 0.4, _row(-0.32, 0.88) + _row(-0.66, 0.88), None),
    "drum": (_drum, None, 0.4, _row(-0.34, 0.95) + _row(-0.66, 0.95), None),
}


def _slots(layout, facing):
    """slot<N> -> the faced point in the first cell of the footprint."""
    return {"slot%d" % n: P.turn(pt, facing, (1, 1)) if facing else pt for n, pt in enumerate(layout)}


def _after(fw, fh, layout, infill, dress):
    """power.render's hook: adds the infill plate and any faced parts, returns the
    slots."""
    def run(facing):
        if dress:
            dress(facing)
        if infill and facing in infill[2]:
            w, h = (fh, fw) if facing in ("east", "west") else (fw, fh)
            S.box("infill", 0.07, -0.14, 0.0, w - 0.07, -0.08, infill[1], infill[0], bevel=0.004)
        return _slots(layout, facing)
    return run


def build(only=None):
    """Every piece in its facings and its icon (ui_icon/<slug>)."""
    out = []
    for slug, row in PIECES.items():
        fn, facings, scale, layout, infill = row[:5]
        dress = row[5] if len(row) > 5 else None
        if only and slug not in only:
            continue
        w = 2 if slug in TWO_WIDE else 1
        extra = {"building": slug.replace("_", " "), "capacity": len(layout), "slot_scale": scale}
        for f in facings or (None,):
            out += P.render(slug + ("_" + f if f else ""), fn, facing=f, extra=dict(extra), family="furniture",
                            after=_after(w, 1, layout, infill, dress))
        p = O._icon(slug, "furniture", slug + ("_south" if facings else ""))
        if p:
            out.append(p)
    return out
