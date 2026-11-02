"""AX-A1 orders (AX-AR2; ART spec §15, §17): the crated building item and
the order and area-clearing badges the Architect's Orders and Zone tabs
use. Our own designs: nothing traced or reconstructed.

- item/crated_building: an uninstalled building boxed for hauling, a
  slatted wooden crate with dark corner braces, a steel strap and a
  stencilled arrow on its lid (metadata `item: crated building`). One
  sprite for every boxed building; the inspector names what is inside.
- ui_icon/order_uninstall: a spanner over an open crate.
- ui_icon/order_chop: an axe bitten into a tree stump (order_cut is the bare
  axe, any plant; chop is trees only).
- ui_icon/order_claim: a pennant on a planted pole.
- ui_icon/order_strip: a shirt lifted away, an arrow over it.
- ui_icon/order_open: a chest with its lid swung up.
- ui_icon/roof_area_clear, roof_remove_clear: the build-roof and
  remove-roof glyphs shrunk into the upper left, the zone tools' corner
  cross on the lower right.
- ui_icon/floor_remove: three floor tiles laid, the fourth lifted off.

- overlay/designation_uninstall, designation_open, designation_strip: the
  map marks of those orders, the badges' glyphs in the designation ring
  (like designation_cut and the rest).

The delete-zone tool reuses ui_icon/zone_delete and the slaughter order
ui_icon/slaughter (PLAY-018), so neither is drawn again here.

    make art FAMILIES=orders
"""
import numpy as np

from .. import stage as S
from . import overlay as O
from . import power as P


# ---------------------------------------------------------------- the crate
def _crate():
    """A slatted crate on two skids, corner braces, a strap and a lid arrow."""
    x0, x1, y0, y1, h = 0.18, 0.82, -0.80, -0.24, 0.42
    for k, y in enumerate((y0 + 0.08, y1 - 0.08)):
        S.box("skid%d" % k, x0 - 0.02, y - 0.05, 0.0, x1 + 0.02, y + 0.05, 0.05, "bark", bevel=0.01)
    S.box("body", x0, y0, 0.05, x1, y1, h, "wood_plank", bevel=0.02)
    for k, z in enumerate((0.15, 0.27)):                       # slat seams on the south face
        S.box("seam%d" % k, x0 + 0.03, y0 - 0.006, z, x1 - 0.03, y0 + 0.004, z + 0.012, "bark", bevel=0.0)
    for k, (x, y) in enumerate(((x0, y0), (x1, y0), (x0, y1), (x1, y1))):   # corner posts
        S.box("post%d" % k, x - 0.035, y - 0.035, 0.05, x + 0.035, y + 0.035, h + 0.01, "wood", bevel=0.008)
    S.box("lid", x0 - 0.01, y0 - 0.01, h, x1 + 0.01, y1 + 0.01, h + 0.035, "wood", bevel=0.012)
    S.box("strap", 0.47, y0 - 0.015, 0.05, 0.53, y1 + 0.015, h + 0.045, "metal_dark", bevel=0.004)
    # a diagonal brace across the south face
    P._rod("brace", (x0 + 0.06, y0 - 0.012, 0.09), (0.44, y0 - 0.012, h - 0.04), 0.022, "wood", segs=6, bevel=0.0)
    P._rod("brace2", (x1 - 0.06, y0 - 0.012, 0.09), (0.56, y0 - 0.012, h - 0.04), 0.022, "wood", segs=6,
           bevel=0.0)
    # stencilled "this side up" arrow on the lid, west of the strap
    S.prism("arrow", [(0.26, -0.60), (0.36, -0.44), (0.42, -0.60)], h + 0.035, h + 0.04, "sign_paint", bevel=0.0)
    S.box("arrow_tail", 0.315, -0.70, h + 0.035, 0.365, -0.60, h + 0.04, "sign_paint", bevel=0.0)
    return S.render_asset("item", "crated_building", zmax=h + 0.06, extra={"item": "crated building"})


# ------------------------------------------------------------------ glyphs
def _scaled(glyph, s, ox, oy):
    """glyph shrunk by s about the icon centre and moved by (ox, oy) px."""
    def g(xx, yy, c):
        out = glyph((xx - c - ox) / s + c, (yy - c - oy) / s + c, c)
        return [f * s if not isinstance(f, tuple) else (f[0] * s, f[1]) for f in (out if isinstance(out, list) else [out])]
    return g


def _spanner(xx, yy, cx, cy, deg, s=1.0):
    """A spanner standing on (cx, cy), turned deg: shaft and an open jaw."""
    rx, ry = O._rot(xx, yy, cx, cy, deg)
    shaft = O._rbox(rx, ry, cx, cy + 6 * s, 2.5 * s, 15 * s, 1 * s)
    head = np.hypot(rx - cx, ry - (cy - 13 * s)) - 11 * s
    notch = O._rbox(rx, ry, cx, cy - 22 * s, 8 * s, 9 * s)
    return np.maximum(np.minimum(shaft, head), -notch)


def _g_uninstall(xx, yy, c):  # a spanner over an open crate
    crate = np.abs(O._rbox(xx, yy, c, c + 17, 22, 9, 2.5))
    slat = O._seg(xx, yy, c - 22, c + 17, c + 22, c + 17)
    return [np.minimum(crate, slat), _spanner(xx, yy, c + 2, c - 8, -35)]


def _ring(xx, yy, cx, cy, rx, ry, n=24):
    """Distance to an ellipse outline (a closed polyline)."""
    pts = [(cx + rx * np.cos(2 * np.pi * k / n), cy + ry * np.sin(2 * np.pi * k / n)) for k in range(n + 1)]
    d = np.full(xx.shape, 1e9, np.float32)
    for (ax, ay), (bx, by) in zip(pts, pts[1:]):
        d = np.minimum(d, O._seg(xx, yy, ax, ay, bx, by))
    return d


def _g_chop(xx, yy, c):  # an axe bitten into a tree stump
    side = np.minimum(O._seg(xx, yy, c - 18, c + 4, c - 18, c + 22), O._seg(xx, yy, c + 18, c + 4, c + 18, c + 22))
    base = O._seg(xx, yy, c - 18, c + 22, c + 18, c + 22)
    top = _ring(xx, yy, c, c + 4, 18, 7)
    handle = O._seg(xx, yy, c + 2, c - 2, c + 22, c - 26)
    head = O._poly(xx, yy, [(c - 12, c - 4), (c - 2, c - 16), (c + 8, c - 6), (c - 2, c + 4)])
    return [np.minimum(np.minimum(side, base), top), handle, head]


def _g_claim(xx, yy, c):  # a pennant on a planted pole
    pole = O._seg(xx, yy, c - 14, c - 26, c - 14, c + 22)
    ground = O._seg(xx, yy, c - 26, c + 24, c + 4, c + 24)
    flag = O._poly(xx, yy, [(c - 12, c - 26), (c + 24, c - 16), (c - 12, c - 4)])
    return [np.minimum(pole, ground), flag]


def _g_strip(xx, yy, c):  # a shirt lifted away, an arrow over it
    body = O._rbox(xx, yy, c, c + 12, 12, 13, 2)
    sleeves = np.minimum(O._poly(xx, yy, [(c - 12, c - 1), (c - 24, c + 8), (c - 19, c + 14), (c - 12, c + 8)]),
                         O._poly(xx, yy, [(c + 12, c - 1), (c + 24, c + 8), (c + 19, c + 14), (c + 12, c + 8)]))
    neck = np.hypot(xx - c, yy - (c - 2)) - 5
    shirt = np.maximum(np.minimum(body, sleeves), -neck)
    arrow = np.minimum(O._seg(xx, yy, c, c - 10, c, c - 28),
                       np.minimum(O._seg(xx, yy, c - 9, c - 19, c, c - 28), O._seg(xx, yy, c + 9, c - 19, c, c - 28)))
    return [shirt, arrow]


def _g_open(xx, yy, c):  # a chest with its lid swung up and back
    chest = np.abs(O._rbox(xx, yy, c, c + 16, 22, 10, 2.5))
    band = O._seg(xx, yy, c - 22, c + 11, c + 22, c + 11)
    lid = np.full(xx.shape, 1e9, np.float32)
    pts = [(c - 22, c + 4), (c - 28, c - 18), (c + 16, c - 18), (c + 22, c + 4)]
    for (ax, ay), (bx, by) in zip(pts, pts[1:]):
        lid = np.minimum(lid, O._seg(xx, yy, ax, ay, bx, by))
    return [np.minimum(chest, band), lid]


def _g_floor_remove(xx, yy, c):  # three floor tiles laid, the fourth lifted off
    d = np.full(xx.shape, 1e9, np.float32)
    for ox, oy in ((-12, 14), (12, 14), (-12, -10)):
        d = np.minimum(d, np.abs(O._rbox(xx, yy, c + ox, c + oy, 10, 10, 2)))
    rx, ry = O._rot(xx, yy, c + 16, c - 22, 18)
    lifted = O._rbox(rx, ry, c + 16, c - 22, 9, 5, 2)
    arrow = np.minimum(O._seg(xx, yy, c + 12, c + 0, c + 12, c - 10),
                       np.minimum(O._seg(xx, yy, c + 7, c - 5, c + 12, c - 10), O._seg(xx, yy, c + 17, c - 5, c + 12, c - 10)))
    return [d, arrow, lifted]


def _with_cross(glyph):
    """A tool's glyph shrunk to the upper left with the zone tools' cross."""
    small = _scaled(glyph, 0.8, -5, -3)

    def g(xx, yy, c):
        sx, sy, r = c + 18, c + 18, 7.5
        cross = np.minimum(O._seg(xx, yy, sx - r, sy - r, sx + r, sy + r), O._seg(xx, yy, sx + r, sy - r, sx - r, sy + r))
        return small(xx, yy, c) + [cross]
    return g


def _merge(glyph):
    """A layered glyph as the one field a map designation strokes."""
    def g(xx, yy, c):
        out = glyph(xx, yy, c)
        d = np.full(xx.shape, 1e9, np.float32)
        for f in out if isinstance(out, list) else [out]:
            d = np.minimum(d, f[0] if isinstance(f, tuple) else f)
        return d
    return g


def designations():
    """Map marks for the uninstall, open and strip orders (overlay family)."""
    t = O._base_tokens()
    return [O._designation("designation_" + k, t, _merge(_scaled(g, 0.72, 0, 1)))
            for k, g in (("uninstall", _g_uninstall), ("open", _g_open), ("strip", _g_strip))]


def icons():
    t = O._base_tokens()
    return [O._badge_icon(n, t, g, t[rgb] if isinstance(rgb, str) else rgb) for n, g, rgb in (
        ("order_uninstall", _g_uninstall, "blueprint"),
        ("order_chop", _g_chop, "designation"),
        ("order_claim", _g_claim, (0.95, 0.78, 0.40)),
        ("order_strip", _g_strip, "designation"),
        ("order_open", _g_open, "designation"),
        ("roof_area_clear", _with_cross(O._g_roof_build), "danger"),
        ("roof_remove_clear", _with_cross(O._g_roof_remove), "danger"),
        ("floor_remove", _g_floor_remove, "designation"),
    )]


def build():
    return [_crate()] + icons() + designations()
