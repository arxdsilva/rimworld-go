"""Overlays (ART spec §17-§18) and representative UI icons.

Overlays are flat 2D (signed-distance shapes, anti-aliased) so they stay
crisp and never pick up world lighting. Icons are cut from the same 3D
renders as the world sprites so UI and world share one visual language.
"""
import os
import re

import bpy
import numpy as np

from .. import contracts as C
from .. import meta, palette
from .. import stage as S
from .animal_roster import ROSTER

N = C.PPT
UI_DIR = os.path.join(C.ASSETS_DIR, "generated", "ui", "icons")


def _base_tokens():
    txt = open(os.path.join(C.PALETTE_DIR, "base.yaml"), encoding="utf-8").read()
    sect = txt.split("overlay:")[1].split("\n\n")[0]
    return {k: palette.hex_to_srgb(v) for k, v in re.findall(r'(\w+):\s*"(#[0-9a-fA-F]{6})"', sect)}


def _grid():
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32) + 0.5
    return xx, yy


def _cov(d, w=1.0):
    """Coverage from signed distance (inside < 0), ~1px AA."""
    return np.clip(0.5 - d / w, 0.0, 1.0)


def _seg(xx, yy, ax, ay, bx, by):
    px, py, vx, vy = xx - ax, yy - ay, bx - ax, by - ay
    t = np.clip((px * vx + py * vy) / (vx * vx + vy * vy), 0, 1)
    return np.hypot(px - vx * t, py - vy * t)


def _paint(img, cov, rgb, alpha=1.0):
    a = cov * alpha
    out_a = a + img[..., 3] * (1 - a)
    safe = np.maximum(out_a, 1e-6)
    for i in range(3):
        img[..., i] = (rgb[i] * a + img[..., i] * img[..., 3] * (1 - a)) / safe
    img[..., 3] = out_a


def _stroke(img, d, width, rgb, edge=(0.08, 0.07, 0.06)):
    _paint(img, _cov(d - width / 2 - 2.5), edge, 0.8)   # dark halo for contrast on any terrain
    _paint(img, _cov(d - width / 2), rgb)


def _save(name, img, extra):
    path = S.out_path("overlay", name)
    S.write_png(img, path)
    data = {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
            "canvas": [N, N], "footprint": [1, 1], "origin_px": [0, 0]}
    data.update(extra)
    meta.write("overlay", name, data)
    return path


def _selection(t):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    m, L = 6.0, 30.0
    d = np.full((N, N), 1e9, np.float32)
    for cx, sx in ((m, 1), (N - m, -1)):
        for cy, sy in ((m, 1), (N - m, -1)):
            d = np.minimum(d, _seg(xx, yy, cx, cy, cx + sx * L, cy))
            d = np.minimum(d, _seg(xx, yy, cx, cy, cx, cy + sy * L))
    _stroke(img, d, 5.0, t["selection"])
    return _save("selection", img, {"overlay": "selection"})


def _designation(name, t, glyph):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    ring = np.abs(np.hypot(xx - c, yy - c) - 30)
    _paint(img, _cov(np.hypot(xx - c, yy - c) - 30), (0.08, 0.07, 0.06), 0.35)
    _stroke(img, ring, 5.0, t["designation"])
    _stroke(img, glyph(xx, yy, c), 6.0, t["designation"])
    return _save(name, img, {"overlay": "designation", "designation": name.split("_", 1)[1]})


def _g_cut(xx, yy, c):  # axe: handle + blade wedge
    d = _seg(xx, yy, c - 14, c + 16, c + 10, c - 10)
    blade = np.maximum(np.abs(xx - (c + 8)) - 7, np.abs(yy - (c - 12)) - 9)
    return np.minimum(d, np.maximum(blade, 0) - 0.01)


def _g_harvest(xx, yy, c):  # three berries + stem
    d = _seg(xx, yy, c, c - 16, c, c - 6)
    for ox, oy in ((-9, 4), (9, 4), (0, 13)):
        d = np.minimum(d, np.abs(np.hypot(xx - c - ox, yy - c - oy) - 5))
    return d


def _g_deconstruct(xx, yy, c):  # crossed-out block: building coming down
    box = np.abs(np.maximum(np.abs(xx - c) - 13, np.abs(yy - c) - 13))
    d = np.minimum(_seg(xx, yy, c - 17, c - 17, c + 17, c + 17), _seg(xx, yy, c + 17, c - 17, c - 17, c + 17))
    return np.minimum(box, d)


def _g_cancel(xx, yy, c):
    return np.minimum(_seg(xx, yy, c - 15, c - 15, c + 15, c + 15), _seg(xx, yy, c + 15, c - 15, c - 15, c + 15))


def _g_stockpile(xx, yy, c):  # stacked crates
    d = np.full(xx.shape, 1e9, np.float32)
    for ox, oy in ((-11, 8), (11, 8), (0, -10)):
        d = np.minimum(d, np.abs(np.maximum(np.abs(xx - c - ox) - 9, np.abs(yy - c - oy) - 8)))
    return d


def _g_draft(xx, yy, c):  # upward chevrons
    d = np.minimum(_seg(xx, yy, c - 14, c + 2, c, c - 12), _seg(xx, yy, c, c - 12, c + 14, c + 2))
    d2 = np.minimum(_seg(xx, yy, c - 14, c + 16, c, c + 2), _seg(xx, yy, c, c + 2, c + 14, c + 16))
    return np.minimum(d, d2)


SPOT_FACINGS = ("south", "east", "north", "west")


def _sleeping_spot(facing):
    """A bed-sized sleeping place marked on the floor: a dashed bedroll
    outline with a pillow mark at the head end, in pale chalk. 1x2 facing
    south/north, 2x1 on its side; like a bed, the sleeper's feet point to
    the facing, so the pillow is at the opposite end."""
    w, h = (2, 1) if facing in ("east", "west") else (1, 2)
    W, H = w * N, h * N
    yy, xx = np.mgrid[0:H, 0:W].astype(np.float32) + 0.5
    img = np.zeros((H, W, 4), np.float32)
    chalk = (0.93, 0.89, 0.80)

    def rrect(cx, cy, hw, hh, r):
        qx = np.abs(xx - cx) - (hw - r)
        qy = np.abs(yy - cy) - (hh - r)
        return np.hypot(np.maximum(qx, 0), np.maximum(qy, 0)) + np.minimum(np.maximum(qx, qy), 0) - r, qx, qy

    inset = 12.0
    outside, qx, qy = rrect(W / 2, H / 2, W / 2 - inset, H / 2 - inset, 16)
    ring = np.abs(outside)
    t = np.where(qy > qx, xx, yy)  # run the dashes along each side
    dash = (np.floor(t / 12.0) % 2) == 0
    _paint(img, _cov(outside), (0.10, 0.09, 0.08), 0.16)            # faint ground shade
    _paint(img, _cov(ring - 2.6) * dash, (0.10, 0.09, 0.08), 0.45)  # halo
    _paint(img, _cov(ring - 1.8) * dash, chalk, 0.9)
    # pillow at the head end (opposite the facing)
    head = {"south": (W / 2, 34), "north": (W / 2, H - 34), "east": (34, H / 2), "west": (W - 34, H / 2)}[facing]
    pw, ph = (26, 15) if facing in ("south", "north") else (15, 26)
    pill, _, _ = rrect(head[0], head[1], pw, ph, 8)
    _paint(img, _cov(pill), chalk, 0.5)
    _paint(img, _cov(np.abs(pill) - 1.4), chalk, 0.9)
    name = "sleeping_spot_" + facing
    path = S.out_path("overlay", name)
    S.write_png(img, path)
    meta.write("overlay", name, {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [W, H], "footprint": [w, h], "origin_px": [0, 0],
                                 "overlay": "sleeping_spot", "marking": True, "facing": facing,
                                 "rotatable": True})
    return path


def _g_hunt(xx, yy, c):  # crosshair: ring + four ticks
    d = np.abs(np.hypot(xx - c, yy - c) - 13)
    for ax, ay, bx, by in ((c - 24, c, c - 9, c), (c + 9, c, c + 24, c), (c, c - 24, c, c - 9), (c, c + 9, c, c + 24)):
        d = np.minimum(d, _seg(xx, yy, ax, ay, bx, by))
    return d


def _badge_icon(name, t, glyph, rgb):
    """Order/action icon: dark disc + coloured glyph, @2x of a 64 px icon."""
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    _paint(img, _cov(np.hypot(xx - c, yy - c) - 52), (0.12, 0.10, 0.09), 0.92)
    _stroke(img, np.abs(np.hypot(xx - c, yy - c) - 50), 4.0, rgb)
    _stroke(img, glyph(xx, yy, c), 7.0, rgb)
    path = os.path.join(UI_DIR, name + ".png")
    S.write_png(img, path)
    meta.write("ui_icon", name, {"image": "generated/ui/icons/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [N, N], "source": "glyph"})
    return path


def _g_forbid(xx, yy, c):  # circle with a slash
    return np.minimum(np.abs(np.hypot(xx - c, yy - c) - 17), _seg(xx, yy, c - 12, c - 12, c + 12, c + 12))


def _g_allow(xx, yy, c):  # check mark
    return np.minimum(_seg(xx, yy, c - 17, c + 1, c - 6, c + 13), _seg(xx, yy, c - 6, c + 13, c + 18, c - 13))


def _g_rooms(xx, yy, c):  # floor plan: outer box, dividing wall with a doorway
    box = np.abs(np.maximum(np.abs(xx - c) - 20, np.abs(yy - c) - 16))
    d = np.minimum(box, _seg(xx, yy, c + 2, c - 16, c + 2, c - 4))
    return np.minimum(d, _seg(xx, yy, c + 2, c + 6, c + 2, c + 16))


def _forbidden_marker(t):
    """Small corner chip for forbidden items: red disc, white slash ring."""
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    cx, cy, r = N - 34.0, 34.0, 26.0
    d = np.hypot(xx - cx, yy - cy)
    _paint(img, _cov(d - r - 3), (0.08, 0.07, 0.06), 0.85)
    _paint(img, _cov(d - r), t["danger"])
    ring = np.abs(d - (r - 7.0))
    sl = _seg(xx, yy, cx - 12, cy - 12, cx + 12, cy + 12)
    w = (0.97, 0.95, 0.92)
    _paint(img, _cov(ring - 3.0), w)
    _paint(img, _cov(sl - 3.2), w)
    return _save("forbidden", img, {"overlay": "forbidden", "marking": True,
                                    "note": "corner chip, upper-right of the item tile"})


def _g_mine(xx, yy, c):  # pick: shaft + curved head
    d = _seg(xx, yy, c - 12, c + 14, c + 6, c - 8)
    head = np.abs(np.hypot(xx - (c + 18), yy - (c + 6)) - 24)
    head = np.maximum(head, np.abs(np.arctan2(yy - (c + 6), xx - (c + 18)) + 2.2) * 24 - 16)
    return np.minimum(d, head)


def _g_smooth(xx, yy, c):  # flat trowel over a levelled surface: two level lines + a wiping stroke
    lines = np.minimum(_seg(xx, yy, c - 20, c + 14, c + 20, c + 14), _seg(xx, yy, c - 20, c + 4, c + 6, c + 4))
    blade = _seg(xx, yy, c - 6, c - 4, c + 18, c - 4)
    handle = _seg(xx, yy, c + 18, c - 4, c + 22, c - 20)
    return np.minimum(np.minimum(lines, blade), handle)


# Zone fills carry no border: the game outlines a whole zone along its
# outer edge only (ZONE-001), so adjacent tiles read as one area. Pattern
# periods divide N so the fill tiles without seams.


def _stockpile(t):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    _paint(img, np.ones((N, N), np.float32), t["stockpile"], 0.16)
    stripes = ((xx + yy) % 32) < 3
    _paint(img, stripes.astype(np.float32), t["stockpile"], 0.22)
    return _save("stockpile", img, {"overlay": "zone", "tileable": True})


def _grow_zone(t):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    green = (0.45, 0.78, 0.36)
    _paint(img, np.ones((N, N), np.float32), green, 0.14)
    rows = ((yy % 32) < 3)
    _paint(img, rows.astype(np.float32), green, 0.35)
    return _save("grow_zone", img, {"overlay": "zone", "tileable": True})


def _pen_zone(t):
    """Animal pen fill: a warm brown wash (the fence around the pen's edge
    is drawn by the game)."""
    img = np.zeros((N, N, 4), np.float32)
    _paint(img, np.ones((N, N), np.float32), (0.78, 0.55, 0.32), 0.13)
    return _save("pen_zone", img, {"overlay": "zone", "tileable": True})


def _g_tame(xx, yy, c):  # heart
    a = np.abs(np.hypot(xx - (c - 8), yy - (c - 5)) - 9)
    b = np.abs(np.hypot(xx - (c + 8), yy - (c - 5)) - 9)
    v = np.minimum(_seg(xx, yy, c - 16, c + 0, c, c + 17), _seg(xx, yy, c + 16, c + 0, c, c + 17))
    return np.minimum(np.minimum(a, b), v)


def _g_pen(xx, yy, c):  # fence: three posts, two rails
    d = np.full(xx.shape, 1e9, np.float32)
    for ox in (-16, 0, 16):
        d = np.minimum(d, _seg(xx, yy, c + ox, c - 16, c + ox, c + 16))
    for oy in (-7, 7):
        d = np.minimum(d, _seg(xx, yy, c - 22, c + oy, c + 22, c + oy))
    return d


def _g_sprout(xx, yy, c):
    stem = _seg(xx, yy, c, c + 18, c, c - 4)
    leaf1 = np.abs(np.hypot((xx - c + 10) / 1.6, yy - c + 6) - 8)
    leaf2 = np.abs(np.hypot((xx - c - 10) / 1.6, yy - c + 12) - 8)
    return np.minimum(stem, np.minimum(leaf1, leaf2))


def _blueprint(t):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    _paint(img, np.ones((N, N), np.float32), t["blueprint"], 0.18)
    edge = np.minimum(np.minimum(xx, N - xx), np.minimum(yy, N - yy))
    dash = (((xx + yy) // 12) % 2) == 0
    _paint(img, (_cov(edge - 4) * dash).astype(np.float32), t["blueprint"], 0.9)
    return _save("blueprint", img, {"overlay": "blueprint", "tileable": True})


# ------------------------------------------------- shadow + roof textures
def _save_flat(name, w, h, img, extra=None):
    path = S.out_path("overlay", name)
    S.write_png(img, path)
    data = {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
            "canvas": [w, h], "footprint": [w / N, h / N], "origin_px": [0, 0],
            "overlay": name, "shadow": {"kind": "none", "w": 0.0, "h": 0.0, "d": 0.0}}
    data.update(extra or {})
    meta.write("overlay", name, data)
    return path


def _shadow_blob():
    """128x64 soft Gaussian ellipse, white RGB, alpha 1 at the centre -> 0 at the edge."""
    H, W = N // 2, N
    yy, xx = np.mgrid[0:H, 0:W].astype(np.float32) + 0.5
    r2 = ((xx - W / 2) / (W / 2)) ** 2 + ((yy - H / 2) / (H / 2)) ** 2
    g = np.exp(-4.0 * r2)
    g = np.clip((g - np.exp(-4.0)) / (1.0 - np.exp(-4.0)), 0.0, 1.0) * (r2 < 1.0)
    img = np.ones((H, W, 4), np.float32)
    img[..., 3] = g
    return _save_flat("shadow_blob", W, H, img, {"marking": True})


def _shadow_box():
    """128x128 white rectangle with an 8 px feather to alpha 0 at the border."""
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32) + 0.5
    edge = np.minimum(np.minimum(xx, N - xx), np.minimum(yy, N - yy))
    img = np.ones((N, N, 4), np.float32)
    img[..., 3] = np.clip(edge / 8.0, 0.0, 1.0)
    return _save_flat("shadow_box", N, N, img, {"marking": True})


def _g_roof(xx, yy, c):  # gable roof: chevron + eave line
    d = np.minimum(_seg(xx, yy, c - 22, c + 2, c, c - 18), _seg(xx, yy, c, c - 18, c + 22, c + 2))
    return np.minimum(d, _seg(xx, yy, c - 22, c + 2, c + 22, c + 2))


def _g_roof_build(xx, yy, c):  # roof + plus sign under it
    d = _g_roof(xx, yy, c)
    d = np.minimum(d, _seg(xx, yy, c - 8, c + 16, c + 8, c + 16))
    return np.minimum(d, _seg(xx, yy, c, c + 8, c, c + 24))


def _g_roof_remove(xx, yy, c):  # roof + strike-through
    return np.minimum(_g_roof(xx, yy, c), _seg(xx, yy, c - 20, c - 18, c + 20, c + 20))


def _g_overlay_roof(xx, yy, c):  # roof with rafters
    d = _g_roof(xx, yy, c)
    for ox in (-10, 0, 10):
        d = np.minimum(d, _seg(xx, yy, c + ox, c + 2, c + ox * 0.55, c - 9 - (10 - abs(ox)) * 0.2))
    return d


def _g_house(xx, yy, c):  # small house: gable roof over walls
    d = np.minimum(_seg(xx, yy, c - 26, c - 4, c, c - 26), _seg(xx, yy, c, c - 26, c + 26, c - 4))
    d = np.minimum(d, _seg(xx, yy, c - 18, c - 6, c - 18, c + 24))
    d = np.minimum(d, _seg(xx, yy, c + 18, c - 6, c + 18, c + 24))
    return np.minimum(d, _seg(xx, yy, c - 18, c + 24, c + 18, c + 24))


def _g_home(xx, yy, c):  # house with a door
    d = _g_house(xx, yy, c)
    d = np.minimum(d, _seg(xx, yy, c - 6, c + 24, c - 6, c + 10))
    d = np.minimum(d, _seg(xx, yy, c + 6, c + 24, c + 6, c + 10))
    return np.minimum(d, _seg(xx, yy, c - 6, c + 10, c + 6, c + 10))


def _g_home_clear(xx, yy, c):  # house with a cross
    d = _g_house(xx, yy, c)
    d = np.minimum(d, _seg(xx, yy, c - 8, c + 4, c + 8, c + 20))
    return np.minimum(d, _seg(xx, yy, c + 8, c + 4, c - 8, c + 20))


def _g_plan(xx, yy, c):  # dotted square with a pencil across its corner
    d = np.full(xx.shape, 1e9, np.float32)
    lo, hi = c - 22, c + 22
    for t in (0.0, 0.5):  # dashes along each side, two per side
        for a, b in (((lo, lo), (hi, lo)), ((hi, lo), (hi, hi)), ((hi, hi), (lo, hi)), ((lo, hi), (lo, lo))):
            for k in (0.08, 0.58):
                p0 = (a[0] + (b[0] - a[0]) * k, a[1] + (b[1] - a[1]) * k)
                p1 = (a[0] + (b[0] - a[0]) * (k + 0.22), a[1] + (b[1] - a[1]) * (k + 0.22))
                d = np.minimum(d, _seg(xx, yy, p0[0], p0[1], p1[0], p1[1]))
        break
    # pencil: body from lower-left toward upper-right, tip at (c - 8, c + 8)
    d = np.minimum(d, _seg(xx, yy, c - 8, c + 8, c + 16, c - 16))
    d = np.minimum(d, _seg(xx, yy, c - 14, c + 14, c - 8, c + 8))
    return np.minimum(d, _seg(xx, yy, c + 12, c - 20, c + 20, c - 12))


def _g_plan_clear(xx, yy, c):  # the same, crossed out
    d = _g_plan(xx, yy, c)
    return np.minimum(d, _seg(xx, yy, c - 24, c - 24, c + 24, c + 24))


def _roof_tile(name, kind):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    edge = np.minimum(np.minimum(xx, N - xx), np.minimum(yy, N - yy))
    if kind == "built":
        _paint(img, np.ones((N, N), np.float32), (90 / 255, 140 / 255, 210 / 255), 70 / 255)
    elif kind == "natural":
        _paint(img, np.ones((N, N), np.float32), (110 / 255, 110 / 255, 125 / 255), 85 / 255)
        stipple = ((xx + yy) % 16 < 3.0) & (xx % 2 < 1)
        _paint(img, stipple.astype(np.float32), (70 / 255, 70 / 255, 85 / 255), 0.55)
    elif kind == "plan":
        amber = (235 / 255, 175 / 255, 60 / 255)
        _paint(img, np.ones((N, N), np.float32), amber, 30 / 255)
        dash = (((xx + yy) // 14) % 2) == 0
        _paint(img, (_cov(edge - 4) * dash).astype(np.float32), amber, 200 / 255)
    else:  # remove: red hatch
        red = (220 / 255, 70 / 255, 60 / 255)
        hatch = ((xx - yy) % 20) < 4
        _paint(img, hatch.astype(np.float32), red, 120 / 255)
    return _save_flat(name, N, N, img, {"overlay": "roof", "roof": kind, "tileable": True})


# ---------------------------------------------------------------- icons
ICONS = {
    # icon name: (family, sprite)
    "wood": ("item", "wood_pile"),
    "food": ("item", "ration_pack"),
    "berries": ("item", "berries_basket"),
    "potatoes": ("item", "potatoes"),
    "corn": ("item", "corn_pile"),
    "wheat": ("item", "wheat_sheaf"),
    "beans": ("item", "beans_sack"),
    "lettuce": ("item", "lettuce_heads"),
    "radishes": ("item", "radish_bunch"),
    "pumpkins": ("item", "pumpkins"),
    "herbal_medicine": ("item", "herbal_medicine"),
    "meat": ("item", "meat_pile"),
    "meal": ("item", "meal_simple"),
    "steel": ("item", "steel_bars"),
    "stone": ("item", "stone_blocks"),
    "bed": ("furniture", "bed_basic_south"),
    "double_bed": ("furniture", "bed_double_south"),
    "sleeping_spot": ("overlay", "sleeping_spot_south"),
    "stove": ("furniture", "stove_basic_south"),
    "club": ("item", "club_wood"),
    "rifle": ("item", "rifle_ground"),
    "floor": ("structure", "floor_wood"),
    "table": ("furniture", "table_small"),
    "chair": ("furniture", "chair_basic_south"),
    "wall": ("structure", "wall_wood_00"),
    "door": ("structure", "door_wood_ew_closed"),
    "workbench": ("furniture", "workbench_basic_south"),
    "stonecutter_table": ("furniture", "stonecutter_table_south"),
    "lamp": ("furniture", "lamp_standing"),
    "tree": ("plant", "tree_oak_00"),
    "crop": ("plant", "potato_harvestable"),
    "colonist": ("pawn", "body_south"),
    "butcher_table": ("furniture", "butcher_table_south"),
    "drying_rack": ("furniture", "drying_rack_south"),
    "campfire": ("furniture", "campfire"),
    "horseshoe_pin": ("furniture", "horseshoe_pin"),
    "sandbags": ("furniture", "sandbags"),
    "research_bench": ("furniture", "research_bench_south"),
    "spike_trap": ("furniture", "spike_trap"),
    "tailoring_bench": ("furniture", "tailoring_bench_south"),
    "cloth": ("item", "cloth_bolt"),
    "straw": ("item", "straw_bundle"),
    "straw_carpet": ("item", "carpet_straw_rolled"),
    "cloth_carpet": ("item", "carpet_cloth_rolled"),
    "leather_carpet": ("item", "carpet_leather_rolled"),
    "wood_carpet": ("item", "carpet_wood_rolled"),
    "wool_carpet": ("item", "carpet_wool_rolled"),
    "milk": ("item", "milk"),
    "wool": ("item", "wool"),
    "egg": ("item", "egg"),
    "fertile_egg": ("item", "fertile_egg"),
    "ember_oil": ("item", "ember_oil"),
    "wool_parka": ("item", "parka_wool"),
    "feeding_trough": ("furniture", "feeding_trough_empty_south"),
    "fence": ("structure", "fence_wood_10"),    # the Architect fence tool
    "fence_gate": ("structure", "fence_gate_south"),
    "carpet": ("furniture", "carpet_cloth"),   # the Architect tool: a laid mat
    # NG-1 items and buildings (cut from the 3D sprites like the rest)
    "components": ("item", "components_m"),
    "industrial_medicine": ("item", "industrial_medicine_m"),
    "tool_cabinet": ("furniture", "tool_cabinet"),
    "hospital_bed": ("furniture", "hospital_bed_south"),
    # ENG-003 social & host systems
    "brewing_bench": ("furniture", "brewing_bench_south"),
    "drug_table": ("furniture", "drug_table_south"),
    "marriage_spot": ("overlay", "marriage_spot_south"),
    "party_spot": ("overlay", "party_spot"),
    "brewgrain": ("item", "brewgrain"),
    "ashleaf": ("item", "ashleaf"),
    # MAPGEN-3
    "bridge": ("structure", "bridge_10"),
    "open_casket": ("structure", "sleeper_casket_open"),
}
ICON_PX = 128          # @2x of a 64 px icon
ICON_PAD = 10


def _icon(name, family, sprite):
    src = S.out_path(family, sprite)
    if not os.path.exists(src):
        return None
    im = bpy.data.images.load(src, check_existing=False)
    w, h = im.size
    px = np.empty(w * h * 4, np.float32)
    im.pixels.foreach_get(px)
    arr = px.reshape(h, w, 4)
    ys, xs = np.nonzero(arr[..., 3] > 0.35)        # drop the soft contact shadow
    y0, y1, x0, x1 = ys.min(), ys.max() + 1, xs.min(), xs.max() + 1
    bpy.data.images.remove(im)
    crop = arr[y0:y1, x0:x1].copy()
    crop[..., 3] = np.where(crop[..., 3] > 0.35, crop[..., 3], 0)
    side = ICON_PX - 2 * ICON_PAD
    k = side / max(crop.shape[0], crop.shape[1])
    tw, th = max(1, int(round(crop.shape[1] * k))), max(1, int(round(crop.shape[0] * k)))
    tmp = bpy.data.images.new("rwg_icon_tmp", crop.shape[1], crop.shape[0], alpha=True)
    tmp.pixels.foreach_set(crop.ravel())
    tmp.scale(tw, th)
    small = np.empty(tw * th * 4, np.float32)
    tmp.pixels.foreach_get(small)
    bpy.data.images.remove(tmp)
    small = small.reshape(th, tw, 4)
    out = np.zeros((ICON_PX, ICON_PX, 4), np.float32)
    ox, oy = (ICON_PX - tw) // 2, (ICON_PX - th) // 2
    out[oy:oy + th, ox:ox + tw] = small        # bottom-up both sides: consistent
    path = os.path.join(UI_DIR, name + ".png")
    S.write_png(out[::-1], path)
    meta.write("ui_icon", name, {"image": "generated/ui/icons/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [ICON_PX, ICON_PX], "source": "%s:%s/%s" % (C.NAMESPACE, family, sprite)})
    return path


# ---------------------------------------------------------------- biome icons
# 12 round swatch emblems for the new-game biome picker: sky over ground
# inside the same dark disc the order badges use, with a tiny scene glyph.
def _tri(xx, yy, cx, top, base, half):
    """Signed-ish distance of an isoceles triangle (apex up)."""
    t = np.clip((yy - top) / max(base - top, 1e-3), -0.2, 1.2)
    w = half * t
    return np.maximum(np.abs(xx - cx) - w, np.maximum(top - yy, yy - base))


def _ell(xx, yy, cx, cy, rx, ry):
    return np.hypot((xx - cx) / rx, (yy - cy) / ry) * min(rx, ry) - min(rx, ry)


def _box(xx, yy, x0, y0, x1, y1):
    return np.maximum(np.maximum(x0 - xx, xx - x1), np.maximum(y0 - yy, yy - y1))


def _fill(img, d, rgb, alpha=1.0, edge=True):
    if edge:
        _paint(img, _cov(d - 2.0), (0.10, 0.09, 0.08), 0.75)
    _paint(img, _cov(d), rgb, alpha)


def _pine(img, xx, yy, cx, base, h, rgb=(0.17, 0.38, 0.30), snow=False):
    _fill(img, _box(xx, yy, cx - 2.5, base - 6, cx + 2.5, base), (0.38, 0.28, 0.2))
    for k in range(3):
        top = base - h + k * h * 0.22
        b = base - 4 - (2 - k) * h * 0.0 - h * 0.12 * (2 - k) * -1
        _fill(img, _tri(xx, yy, cx, top, top + h * 0.5, h * 0.3 + k * 2), rgb)
        if snow:
            _fill(img, _tri(xx, yy, cx, top, top + h * 0.2, h * 0.12 + k), (0.96, 0.98, 1.0), edge=False)


def _round_tree(img, xx, yy, cx, base, r, rgb=(0.28, 0.56, 0.25)):
    _fill(img, _box(xx, yy, cx - 3, base - r, cx + 3, base), (0.38, 0.28, 0.2))
    _fill(img, np.hypot(xx - cx, yy - (base - r * 1.5)) - r, rgb)
    _paint(img, _cov(np.hypot(xx - cx + r * 0.3, yy - (base - r * 1.7)) - r * 0.45), tuple(min(1, v + 0.15) for v in rgb), 0.8)


def _reed(img, xx, yy, cx, base, h, lean=0.0):
    _fill(img, _seg(xx, yy, cx, base, cx + lean, base - h) - 1.6, (0.50, 0.62, 0.30))
    _fill(img, _ell(xx, yy, cx + lean, base - h - 3, 3.2, 8), (0.45, 0.30, 0.18))


def _cactus(img, xx, yy, cx, base, h):
    g = (0.30, 0.60, 0.40)
    _fill(img, _seg(xx, yy, cx, base, cx, base - h) - 5, g)
    _fill(img, _seg(xx, yy, cx - 9, base - h * 0.5, cx - 9, base - h * 0.75) - 3.5, g)
    _fill(img, _seg(xx, yy, cx - 9, base - h * 0.5, cx, base - h * 0.5) - 3.5, g, edge=False)
    _fill(img, _seg(xx, yy, cx + 9, base - h * 0.4, cx + 9, base - h * 0.65) - 3.5, g)
    _fill(img, _seg(xx, yy, cx + 9, base - h * 0.4, cx, base - h * 0.4) - 3.5, g, edge=False)


def _dune(img, xx, yy, y0, amp, ph, rgb):
    ysurf = y0 + amp * np.sin(xx / 22.0 + ph)
    _paint(img, _cov(ysurf - yy), rgb)


def _water(img, xx, yy, y0, rgb, hi):
    _paint(img, _cov(y0 - yy) * 0, rgb)
    _paint(img, _cov(_box(xx, yy, 0, y0, 128, 128)), rgb)
    for k, (wx, wy) in enumerate(((40, y0 + 10), (82, y0 + 20), (60, y0 + 30))):
        _paint(img, _cov(_seg(xx, yy, wx - 8, wy, wx + 8, wy) - 1.4), hi, 0.9)


def _crack(img, xx, yy, pts, rgb):
    for (ax, ay), (bx, by) in zip(pts, pts[1:]):
        _paint(img, _cov(_seg(xx, yy, ax, ay, bx, by) - 1.5), rgb)


def _glyph_biome(key, img, xx, yy):
    hz = 76  # horizon row
    if key == "temperate_forest":
        _round_tree(img, xx, yy, 42, 96, 17); _round_tree(img, xx, yy, 78, 100, 20, (0.22, 0.48, 0.22))
    elif key == "boreal_forest":
        _pine(img, xx, yy, 40, 98, 40, snow=True); _pine(img, xx, yy, 72, 102, 52, snow=True)
        _pine(img, xx, yy, 92, 94, 30, snow=True)
    elif key == "tundra":
        for cx, h in ((36, 12), (60, 16), (88, 11)):
            for dx in (-4, 0, 4):
                _paint(img, _cov(_seg(xx, yy, cx + dx, 96, cx + dx * 1.6, 96 - h) - 1.5), (0.52, 0.56, 0.36))
        _paint(img, _cov(_ell(xx, yy, 70, 88, 16, 4)), (0.95, 0.97, 1.0), 0.9)
        _paint(img, _cov(_ell(xx, yy, 34, 100, 12, 3.5)), (0.95, 0.97, 1.0), 0.9)
    elif key == "cold_bog":
        _paint(img, _cov(_ell(xx, yy, 56, 96, 30, 8)), (0.55, 0.72, 0.80))
        _reed(img, xx, yy, 82, 98, 28, 3); _reed(img, xx, yy, 92, 98, 20, -3); _reed(img, xx, yy, 36, 94, 22, 2)
        _paint(img, _cov(_ell(xx, yy, 60, 70, 22, 4)), (0.92, 0.95, 0.98), 0.7)
    elif key == "ice_sheet":
        _crack(img, xx, yy, [(30, 84), (48, 92), (58, 88), (78, 100)], (0.45, 0.65, 0.82))
        _crack(img, xx, yy, [(58, 88), (64, 78), (80, 74)], (0.45, 0.65, 0.82))
        _crack(img, xx, yy, [(78, 100), (96, 96)], (0.45, 0.65, 0.82))
        _fill(img, _tri(xx, yy, 80, 52, 78, 12), (0.84, 0.94, 1.0)); _fill(img, _tri(xx, yy, 56, 60, 78, 9), (0.74, 0.88, 0.98))
    elif key == "sea_ice":
        _water(img, xx, yy, hz, (0.30, 0.52, 0.68), (0.62, 0.82, 0.92))
        for cx, cy, rx in ((46, 94, 20), (86, 104, 16)):
            _fill(img, _ell(xx, yy, cx, cy, rx, 7), (0.93, 0.97, 1.0))
        _fill(img, _tri(xx, yy, 46, 76, 94, 8), (0.80, 0.92, 1.0))
    elif key == "temperate_swamp":
        _water(img, xx, yy, 86, (0.34, 0.45, 0.32), (0.62, 0.76, 0.58))
        _round_tree(img, xx, yy, 40, 92, 14, (0.30, 0.50, 0.28)); _reed(img, xx, yy, 78, 98, 24, 2); _reed(img, xx, yy, 88, 100, 17, -2)
    elif key == "tropical_swamp":
        _water(img, xx, yy, 84, (0.20, 0.44, 0.38), (0.55, 0.85, 0.70))
        _round_tree(img, xx, yy, 56, 88, 17, (0.20, 0.58, 0.30))
        for dx in (-9, 0, 9):
            _paint(img, _cov(_seg(xx, yy, 56 + dx * 0.3, 78, 56 + dx * 1.3, 96) - 1.8), (0.42, 0.30, 0.2))
        _reed(img, xx, yy, 92, 100, 20, 2)
    elif key == "tropical_rainforest":
        _fill(img, _box(xx, yy, 60, 56, 68, 104), (0.38, 0.28, 0.2))
        for cx, cy, r, c in ((64, 50, 24, (0.20, 0.62, 0.28)), (44, 58, 14, (0.25, 0.70, 0.30)), (86, 58, 14, (0.16, 0.54, 0.26))):
            _fill(img, np.hypot((xx - cx) / 1.35, yy - cy) - r, c)
        _paint(img, _cov(_ell(xx, yy, 52, 44, 8, 5)), (0.55, 0.85, 0.40), 0.8)
        _fill(img, _ell(xx, yy, 96, 84, 3, 3), (0.9, 0.35, 0.45))
    elif key == "arid_shrubland":
        for cx, cy, r, c in ((46, 90, 14, (0.55, 0.60, 0.34)), (76, 94, 11, (0.62, 0.55, 0.34)), (96, 98, 8, (0.55, 0.60, 0.34))):
            _fill(img, np.hypot((xx - cx) / 1.3, yy - cy) - r, c)
        for dx in (-3, 0, 3):
            _paint(img, _cov(_seg(xx, yy, 24 + dx, 100, 24 + dx * 2, 90) - 1.3), (0.70, 0.58, 0.32))
    elif key == "desert":
        _dune(img, xx, yy, 88, 6, 0.5, (0.86, 0.70, 0.42))
        _cactus(img, xx, yy, 46, 98, 30)
        _fill(img, _ell(xx, yy, 92, 38, 8, 8), (1.0, 0.9, 0.55))
    else:  # extreme_desert
        _fill(img, _ell(xx, yy, 64, 52, 17, 17), (1.0, 0.86, 0.45))
        _dune(img, xx, yy, 90, 8, 0.0, (0.80, 0.52, 0.30)); _dune(img, xx, yy, 102, 5, 2.0, (0.70, 0.42, 0.24))
        for k, (a, b) in enumerate(((24, 76), (100, 70), (82, 80))):
            _paint(img, _cov(_seg(xx, yy, a - 5, b, a + 5, b) - 1.2), (1.0, 0.85, 0.6), 0.7)


BIOMES = {
    # key: (sky, ground, rim)
    "temperate_forest": ((0.64, 0.82, 0.92), (0.42, 0.62, 0.32), (0.36, 0.66, 0.34)),
    "boreal_forest": ((0.72, 0.82, 0.88), (0.90, 0.94, 0.98), (0.30, 0.55, 0.50)),
    "tundra": ((0.76, 0.84, 0.90), (0.60, 0.60, 0.48), (0.62, 0.68, 0.74)),
    "cold_bog": ((0.66, 0.72, 0.76), (0.40, 0.48, 0.40), (0.40, 0.58, 0.62)),
    "ice_sheet": ((0.80, 0.90, 0.96), (0.88, 0.95, 1.00), (0.55, 0.78, 0.92)),
    "sea_ice": ((0.78, 0.88, 0.94), (0.78, 0.88, 0.94), (0.40, 0.62, 0.80)),
    "temperate_swamp": ((0.70, 0.78, 0.70), (0.36, 0.46, 0.30), (0.46, 0.60, 0.34)),
    "tropical_swamp": ((0.62, 0.84, 0.74), (0.22, 0.42, 0.30), (0.26, 0.64, 0.50)),
    "tropical_rainforest": ((0.52, 0.82, 0.72), (0.16, 0.46, 0.24), (0.22, 0.70, 0.34)),
    "arid_shrubland": ((0.86, 0.84, 0.68), (0.76, 0.64, 0.40), (0.74, 0.62, 0.30)),
    "desert": ((0.95, 0.88, 0.66), (0.90, 0.78, 0.50), (0.92, 0.74, 0.34)),
    "extreme_desert": ((1.00, 0.78, 0.50), (0.86, 0.58, 0.34), (0.90, 0.46, 0.22)),
}


def _biome_icon(key):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    sky, ground, rim = BIOMES[key]
    disc = _cov(np.hypot(xx - c, yy - c) - 52)
    _paint(img, disc, (0.12, 0.10, 0.09), 0.92)
    inner = _cov(np.hypot(xx - c, yy - c) - 46)
    # sky, then ground below the horizon
    g = np.where(yy > 76, 1.0, 0.0).astype(np.float32)
    _paint(img, inner, sky)
    _paint(img, inner * g, ground)
    # clip the scene to the disc: paint into a scratch layer, mask by disc
    scene = np.zeros_like(img)
    _glyph_biome(key, scene, xx, yy)
    mask = inner[..., None]
    a = scene[..., 3:4] * mask
    img[..., :3] = scene[..., :3] * a + img[..., :3] * (1 - a)
    img[..., 3:4] = np.maximum(img[..., 3:4], a)
    _stroke(img, np.abs(np.hypot(xx - c, yy - c) - 50), 4.0, rim)
    name = "biome_" + key
    path = os.path.join(UI_DIR, name + ".png")
    S.write_png(img, path)
    meta.write("ui_icon", name, {"image": "generated/ui/icons/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [N, N], "source": "glyph", "biome": key})
    return path


# ---------------------------------------------------------------- NG-1 glyph icons
# Scenario scenes, skills, traits, passions, factions and settlements. All
# original, drawn from signed distance fields at 128 px (@2x of 64).
def _save_icon(name, img, extra=None):
    path = os.path.join(UI_DIR, name + ".png")
    S.write_png(img, path)
    data = {"image": "generated/ui/icons/%s.png" % name, "scale": C.SCALE, "canvas": [N, N], "source": "glyph"}
    data.update(extra or {})
    meta.write("ui_icon", name, data)
    return path


def _rot(xx, yy, cx, cy, deg):
    """Coordinates rotated about (cx, cy): draw an upright shape, turned by deg."""
    a = np.radians(deg)
    dx, dy = xx - cx, yy - cy
    return cx + dx * np.cos(a) + dy * np.sin(a), cy - dx * np.sin(a) + dy * np.cos(a)


def _rbox(xx, yy, cx, cy, hw, hh, r=0.0):
    """Signed distance of a rounded box centred (cx, cy), half sizes hw, hh."""
    qx, qy = np.abs(xx - cx) - (hw - r), np.abs(yy - cy) - (hh - r)
    return np.hypot(np.maximum(qx, 0), np.maximum(qy, 0)) + np.minimum(np.maximum(qx, qy), 0) - r


def _cap(xx, yy, ax, ay, bx, by, r):
    """Signed distance of a capsule (a thick segment)."""
    return _seg(xx, yy, ax, ay, bx, by) - r


def _poly(xx, yy, pts):
    """Signed distance of a CONVEX polygon (either winding); compose concave
    shapes with np.minimum of convex pieces."""
    n = len(pts)
    best = np.full(xx.shape, 1e9, np.float32)
    crosses = []
    for i in range(n):
        ax, ay = pts[i]
        bx, by = pts[(i + 1) % n]
        best = np.minimum(best, _seg(xx, yy, ax, ay, bx, by))
        crosses.append((bx - ax) * (yy - ay) - (by - ay) * (xx - ax))
    inside = np.all([c <= 0 for c in crosses], axis=0) | np.all([c >= 0 for c in crosses], axis=0)
    return np.where(inside, -best, best)


def _disc_icon(name, sky, ground, rim, scene, horizon=76, extra=None):
    """Round scene emblem (like the biome swatches): sky over ground in a dark
    disc, `scene(img, xx, yy)` drawn clipped to the disc."""
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    _paint(img, _cov(np.hypot(xx - c, yy - c) - 52), (0.12, 0.10, 0.09), 0.92)
    inner = _cov(np.hypot(xx - c, yy - c) - 46)
    _paint(img, inner, sky)
    _paint(img, inner * (yy > horizon), ground)
    layer = np.zeros_like(img)
    scene(layer, xx, yy)
    a = layer[..., 3:4] * inner[..., None]
    img[..., :3] = layer[..., :3] * a + img[..., :3] * (1 - a)
    img[..., 3:4] = np.maximum(img[..., 3:4], a)
    _stroke(img, np.abs(np.hypot(xx - c, yy - c) - 50), 4.0, rim)
    return _save_icon(name, img, extra)


def _flame(img, xx, yy, cx, base, h, outer, inner, edge=True):
    """A teardrop flame standing on y=base, height h; inner core lighter."""
    w = h * 0.30
    body = np.minimum(np.hypot(xx - cx, yy - (base - w)) - w,
                      _tri(xx, yy, cx, base - h, base - w, w * 1.0))
    # flick the tip a touch to the right
    _fill(img, body, outer, edge=edge)
    w2 = w * 0.55
    core = np.minimum(np.hypot(xx - cx, yy - (base - w2 * 0.9)) - w2,
                      _tri(xx, yy, cx, base - h * 0.62, base - w2 * 0.9, w2 * 1.0))
    _paint(img, _cov(core), inner)


# scenario scenes --------------------------------------------------------
def _scene_hull(img, xx, yy):
    for sx, sy, r in ((24, 18, 1.4), (50, 30, 1.1), (88, 22, 1.5), (104, 40, 1.0), (70, 14, 1.0), (36, 46, 1.0)):
        _paint(img, _cov(np.hypot(xx - sx, yy - sy) - r), (0.95, 0.95, 1.0), 0.9)
    rx, ry = _rot(xx, yy, 62, 78, -16)
    hull = _rbox(rx, ry, 62, 78, 38, 15, 9)
    _fill(img, hull, (0.62, 0.66, 0.72))
    _paint(img, _cov(_rbox(rx, ry, 62, 70, 36, 5, 4)), (0.78, 0.82, 0.88))
    _paint(img, _cov(_rbox(rx, ry, 62, 84, 36, 4, 3)), (0.45, 0.49, 0.56))
    _paint(img, _cov(np.hypot(rx - 86, ry - 76) - 5.5), (0.30, 0.55, 0.70))          # porthole
    _paint(img, _cov(np.abs(np.hypot(rx - 86, ry - 76) - 5.5) - 1.2), (0.20, 0.22, 0.26))
    # the break: a jagged dark crack and a torn-off plate
    for (ax, ay), (bx, by) in zip([(54, 64), (58, 74), (52, 82), (57, 92)], [(58, 74), (52, 82), (57, 92), (53, 100)]):
        _paint(img, _cov(_seg(xx, yy, ax, ay, bx, by) - 2.2), (0.10, 0.09, 0.10))
    _fill(img, _poly(xx, yy, [(36, 100), (48, 96), (52, 106), (40, 108)]), (0.50, 0.54, 0.60))
    # fire and smoke
    _flame(img, xx, yy, 56, 98, 24, (0.96, 0.52, 0.18), (1.0, 0.85, 0.40))
    for cx, cy, r, a in ((50, 52, 7, 0.7), (45, 42, 9, 0.55), (40, 31, 10, 0.4)):
        _paint(img, _cov(np.hypot(xx - cx, yy - cy) - r), (0.55, 0.55, 0.58), a)


def _scene_ember(img, xx, yy):
    # a tent behind the fire
    _fill(img, _tri(xx, yy, 82, 34, 92, 24), (0.50, 0.34, 0.22))
    _paint(img, _cov(_tri(xx, yy, 82, 62, 92, 7)), (0.17, 0.11, 0.09))
    _fill(img, _seg(xx, yy, 82, 34, 84, 24) - 1.6, (0.38, 0.26, 0.18))
    _paint(img, _cov(_seg(xx, yy, 62, 92, 76, 40) - 1.4), (0.38, 0.26, 0.18))
    # fire ring of stones, crossed logs, flame, sparks
    for k, ang in enumerate(np.linspace(0, 2 * np.pi, 9)[:-1]):
        _fill(img, np.hypot((xx - (46 + np.cos(ang) * 19)) / 1.2, yy - (96 + np.sin(ang) * 7)) - 4.0,
              (0.55, 0.55, 0.57) if k % 2 else (0.43, 0.43, 0.47), edge=False)
    _fill(img, _cap(xx, yy, 32, 100, 60, 90, 3.6), (0.46, 0.30, 0.18))
    _fill(img, _cap(xx, yy, 32, 90, 60, 100, 3.6), (0.54, 0.36, 0.22))
    _flame(img, xx, yy, 46, 96, 40, (0.95, 0.42, 0.14), (1.0, 0.84, 0.36))
    for px, py in ((30, 44), (62, 40), (36, 30), (56, 28)):
        _paint(img, _cov(np.hypot(xx - px, yy - py) - 1.7), (1.0, 0.78, 0.34), 0.9)


def _scene_gilded(img, xx, yy):
    gold, dgold = (0.93, 0.76, 0.28), (0.72, 0.52, 0.16)
    # a compass star behind the chest
    for k in range(4):
        ax, ay = _rot(xx, yy, 64, 42, 90 * k)
        _fill(img, _tri(ax, ay, 64, 14, 42, 6.5), (1.0, 0.93, 0.62), edge=False)
    _paint(img, _cov(np.abs(np.hypot(xx - 64, yy - 42) - 15) - 1.3), gold, 0.9)
    # treasure chest: body, curved lid, gold bands and lock
    _fill(img, _rbox(xx, yy, 64, 80, 31, 15, 3), (0.50, 0.32, 0.20))
    lid = np.minimum(_rbox(xx, yy, 64, 63, 31, 6, 2), np.hypot((xx - 64) / 1.0, (yy - 73) * 1.65) - 31)
    lid = np.where(yy > 66, 1e3, lid)
    _fill(img, np.maximum(np.hypot((xx - 64), (yy - 68) * 1.7) - 31, yy - 68), (0.58, 0.38, 0.23))
    for bx in (42, 86):
        _fill(img, _rbox(xx, yy, bx, 74, 3.6, 21, 1.5), gold, edge=False)
    _fill(img, _rbox(xx, yy, 64, 76, 6, 7, 2), gold)
    _paint(img, _cov(_rbox(xx, yy, 64, 77, 1.6, 3.2, 0.8)), (0.25, 0.16, 0.10))
    for cx, cy in ((86, 100), (96, 97), (91, 92)):
        _fill(img, _rbox(xx, yy, cx, cy, 6, 3.2, 3.0), gold)
        _paint(img, _cov(_rbox(xx, yy, cx, cy - 1, 4, 1.0, 1.0)), (1.0, 0.92, 0.55), 0.9)
    _fill(img, _rbox(xx, yy, 32, 99, 6, 3.2, 3.0), dgold)


def _scene_hands(img, xx, yy):
    skin, edge = (0.94, 0.76, 0.62), (0.62, 0.42, 0.32)
    def part(d):
        _paint(img, _cov(d - 2.4), (0.10, 0.09, 0.08), 0.9)
        _paint(img, _cov(d), skin)
    part(_rbox(xx, yy, 64, 82, 19, 17, 8))                                  # palm
    for fx, top, lean in ((46, 40, -6), (56, 32, -2), (68, 32, 2), (79, 40, 6)):   # four fingers
        part(_cap(xx, yy, fx, 74, fx + lean, top, 5.6))
    part(_cap(xx, yy, 47, 92, 30, 70, 6.0))                                 # thumb
    for fx in (46, 56, 68, 79):
        _paint(img, _cov(_seg(xx, yy, fx, 78, fx, 86) - 0.8), edge, 0.55)   # finger creases
    _paint(img, _cov(_rbox(xx, yy, 64, 92, 14, 1.4, 1.0)), edge, 0.5)     # wrist crease
    _paint(img, _cov(_rbox(xx, yy, 64, 108, 21, 8, 4)), (0.55, 0.46, 0.38))  # cuffless forearm in shadow


def _scene_custom(img, xx, yy):
    cols = ((0.95, 0.62, 0.40), (0.55, 0.85, 0.60), (0.60, 0.72, 0.98))
    for k, (y, kx) in enumerate(((42, 44), (64, 78), (86, 56))):
        _fill(img, _rbox(xx, yy, 64, y, 30, 3.2, 3.0), (0.22, 0.24, 0.30))
        _paint(img, _cov(_rbox(xx, yy, (34 + kx) / 2, y, (kx - 34) / 2, 2.2, 2.0)), cols[k])
        _fill(img, np.hypot(xx - kx, yy - y) - 8.0, (0.92, 0.93, 0.96))
        _paint(img, _cov(np.hypot(xx - kx, yy - y) - 3.2), cols[k])


SCENARIOS = {
    # key: (sky, ground, rim, scene)
    "broken_hull": ((0.13, 0.16, 0.30), (0.33, 0.30, 0.30), (0.42, 0.62, 0.86), _scene_hull),
    "ember_clan": ((0.30, 0.21, 0.28), (0.34, 0.25, 0.19), (0.93, 0.50, 0.22), _scene_ember),
    "gilded_expedition": ((0.97, 0.88, 0.62), (0.80, 0.66, 0.40), (0.91, 0.74, 0.30), _scene_gilded),
    "bare_hands": ((0.72, 0.84, 0.90), (0.42, 0.58, 0.36), (0.86, 0.85, 0.80), _scene_hands),
    "custom": ((0.20, 0.22, 0.29), (0.20, 0.22, 0.29), (0.62, 0.70, 0.88), _scene_custom),
}


def _scenario_icon(key):
    sky, ground, rim, scene = SCENARIOS[key]
    return _disc_icon("scenario_" + key, sky, ground, rim, scene, extra={"scenario": key})


# passion flames -----------------------------------------------------------
def _passion_icon(level):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    if level == "minor":
        _flame(img, xx, yy, 64, 108, 78, (0.97, 0.58, 0.20), (1.0, 0.90, 0.48))
    else:
        _flame(img, xx, yy, 36, 108, 50, (0.93, 0.36, 0.20), (1.0, 0.72, 0.36))
        _flame(img, xx, yy, 92, 108, 50, (0.93, 0.36, 0.20), (1.0, 0.72, 0.36))
        _flame(img, xx, yy, 64, 112, 100, (1.0, 0.64, 0.16), (1.0, 0.94, 0.52))
        for sx, sy, r in ((24, 26, 6.0), (104, 20, 5.0)):      # sparks: four-point stars
            for ang in (0, 90):
                px, py = _rot(xx, yy, sx, sy, ang)
                _fill(img, _rbox(px, py, sx, sy, r, 1.5, 1.0), (1.0, 0.93, 0.55), edge=False)
    return _save_icon("passion_" + level, img, {"passion": level})


# skill glyphs (distance fields for _badge_icon) ----------------------------
def _g_shoot(xx, yy, c):  # target reticle
    d = np.minimum(np.abs(np.hypot(xx - c, yy - c) - 17), np.abs(np.hypot(xx - c, yy - c) - 1.5))
    for ax, ay, bx, by in ((c - 25, c, c - 9, c), (c + 9, c, c + 25, c), (c, c - 25, c, c - 9), (c, c + 9, c, c + 25)):
        d = np.minimum(d, _seg(xx, yy, ax, ay, bx, by))
    return d


def _g_melee(xx, yy, c):  # a sword: blade, crossguard, grip, pommel
    d = _seg(xx, yy, c + 19, c - 21, c - 5, c + 3)                          # blade
    d = np.minimum(d, _seg(xx, yy, c - 13, c - 5, c + 3, c + 11))           # crossguard (square to the blade)
    d = np.minimum(d, _seg(xx, yy, c - 5, c + 3, c - 16, c + 14))           # grip
    d = np.minimum(d, np.hypot(xx - (c - 19), yy - (c + 17)) - 1.6)         # pommel
    return d


def _g_pick(xx, yy, c):  # pickaxe: haft and a swept double-pointed head
    d = _seg(xx, yy, c - 14, c + 21, c + 4, c - 8)
    for (ax, ay), (bx, by) in zip([(c - 24, c + 2), (c - 12, c - 14), (c + 6, c - 20)],
                                  [(c - 12, c - 14), (c + 6, c - 20), (c + 22, c - 8)]):
        d = np.minimum(d, _seg(xx, yy, ax, ay, bx, by))
    return d


def _g_build(xx, yy, c):  # hammer
    handle = _seg(xx, yy, c - 17, c + 19, c + 3, c - 3)
    head = _seg(xx, yy, c - 6, c - 16, c + 14, c + 2) - 6.0
    return np.minimum(handle, head)


def _g_cook(xx, yy, c):  # lidded pot with curling steam
    pot = np.abs(np.maximum(np.abs(xx - c) - 14, np.abs(yy - c - 9) - 9))
    d = np.minimum(pot, np.minimum(_seg(xx, yy, c - 14, c + 5, c - 23, c + 5), _seg(xx, yy, c + 14, c + 5, c + 23, c + 5)))
    d = np.minimum(d, _seg(xx, yy, c - 16, c - 1, c + 16, c - 1))
    for ox in (-8, 6):
        pts = [(c + ox, c - 5), (c + ox + 4, c - 11), (c + ox - 1, c - 17), (c + ox + 3, c - 23)]
        for (ax, ay), (bx, by) in zip(pts, pts[1:]):
            d = np.minimum(d, _seg(xx, yy, ax, ay, bx, by))
    return d


def _g_paw(xx, yy, c):  # pad and four toes (stroke widens each, so the cores are tiny)
    d = np.hypot((xx - c) / 1.3, yy - (c + 9)) - 6.0
    for ox, oy in ((-17, -5), (-6, -16), (6, -16), (17, -5)):
        d = np.minimum(d, np.hypot((xx - c - ox) / 0.85, yy - c - oy) - 1.5)
    return d


def _g_gear(xx, yy, c):
    d = np.abs(np.hypot(xx - c, yy - c) - 12)
    for k in range(8):
        a = k * np.pi / 4
        d = np.minimum(d, _seg(xx, yy, c + np.cos(a) * 13, c + np.sin(a) * 13, c + np.cos(a) * 21, c + np.sin(a) * 21))
    return np.minimum(d, np.abs(np.hypot(xx - c, yy - c) - 3))


def _g_med(xx, yy, c):  # a bold plus
    d = np.minimum(_seg(xx, yy, c - 16, c, c + 16, c), _seg(xx, yy, c, c - 16, c, c + 16))
    return d - 2.5


def _g_social(xx, yy, c):  # two speech bubbles
    a = np.abs(_rbox(xx, yy, c - 8, c - 8, 16, 10, 6))
    b = np.abs(_rbox(xx, yy, c + 8, c + 9, 16, 10, 6))
    tails = np.minimum(_seg(xx, yy, c - 18, c, c - 22, c + 6), _seg(xx, yy, c + 18, c + 17, c + 22, c + 23))
    return np.minimum(np.minimum(a, b), tails)


def _g_mind(xx, yy, c):  # light bulb
    d = np.abs(np.hypot(xx - c, yy - c + 5) - 12)
    d = np.minimum(d, np.minimum(_seg(xx, yy, c - 6, c + 10, c - 6, c + 16), _seg(xx, yy, c + 6, c + 10, c + 6, c + 16)))
    d = np.minimum(d, _seg(xx, yy, c - 6, c + 18, c + 6, c + 18))
    for ang in (-90, -45, 0, 45, 90):
        a = np.radians(ang)
        d = np.minimum(d, _seg(xx, yy, c + np.sin(a) * 18, c - 5 - np.cos(a) * 18, c + np.sin(a) * 24, c - 5 - np.cos(a) * 24))
    return d


def _g_trait(xx, yy, c):  # a hex chip with a diamond core
    pts = [(c + 22 * np.cos(np.radians(60 * k + 30)), c + 22 * np.sin(np.radians(60 * k + 30))) for k in range(6)]
    d = np.full(xx.shape, 1e9, np.float32)
    for i in range(6):
        d = np.minimum(d, _seg(xx, yy, *pts[i], *pts[(i + 1) % 6]))
    for i, (ax, ay, bx, by) in enumerate(((c, c - 10, c + 10, c), (c + 10, c, c, c + 10), (c, c + 10, c - 10, c), (c - 10, c, c, c - 10))):
        d = np.minimum(d, _seg(xx, yy, ax, ay, bx, by))
    return d


def _g_factions(xx, yy, c):  # two pennants on poles
    d = np.minimum(_seg(xx, yy, c - 12, c + 22, c - 12, c - 20), _seg(xx, yy, c + 12, c + 22, c + 12, c - 8))
    f1 = np.minimum(_seg(xx, yy, c - 12, c - 20, c + 2, c - 14), np.minimum(_seg(xx, yy, c + 2, c - 14, c - 12, c - 6), 1e9))
    f2 = np.minimum(_seg(xx, yy, c + 12, c - 8, c + 25, c - 2), _seg(xx, yy, c + 25, c - 2, c + 12, c + 4))
    return np.minimum(np.minimum(d, f1), f2)


def _g_minimap(xx, yy, c):  # a map card with a viewport frame
    d = np.abs(_rbox(xx, yy, c, c, 23, 19, 3))
    d = np.minimum(d, np.abs(_rbox(xx, yy, c + 6, c - 3, 9, 7, 1)))
    d = np.minimum(d, _seg(xx, yy, c - 23, c + 4, c - 6, c + 4))
    return np.minimum(d, _seg(xx, yy, c - 10, c - 19, c - 10, c + 19))


SKILL_ICONS = (
    ("shooting", _g_shoot, (0.93, 0.50, 0.36)), ("melee", _g_melee, (0.84, 0.86, 0.92)),
    ("construction", _g_build, (0.96, 0.74, 0.34)), ("mining", _g_pick, (0.62, 0.72, 0.86)),
    ("cooking", _g_cook, (0.97, 0.66, 0.40)), ("plants", _g_sprout, (0.45, 0.78, 0.36)),
    ("animals", _g_paw, (0.84, 0.64, 0.44)), ("crafting", _g_gear, (0.48, 0.80, 0.82)),
    ("medicine", _g_med, (0.46, 0.86, 0.66)), ("social", _g_social, (0.95, 0.60, 0.76)),
    ("intellectual", _g_mind, (0.98, 0.88, 0.46)),
)


# faction emblems and settlement icons (white-ish: the renderer multiplies by the accent) -----
SHIELD_FILL = (0.97, 0.97, 0.95)
SHIELD_INK = (0.58, 0.58, 0.60)


def _white_icon(name, painter, extra=None, shield=True):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    if shield:
        sh = np.minimum(_poly(xx, yy, [(22, 14), (106, 14), (106, 62), (64, 116), (22, 62)]),
                        _rbox(xx, yy, 64, 40, 42, 26, 5))
        _paint(img, _cov(sh - 3.0), (0.20, 0.20, 0.22), 0.95)
        _paint(img, _cov(sh), SHIELD_FILL)
        _paint(img, _cov(np.abs(sh + 6.5) - 1.0), SHIELD_INK, 0.45)
    painter(img, xx, yy)
    d = dict({"tintable": True, "tint": "accent"}, **(extra or {}))
    return _save_icon(name, img, d)


def _ink(img, d, w=7.0, rgb=SHIELD_INK):
    _paint(img, _cov(d - w / 2), rgb)


def _em_settlers(img, xx, yy):   # a homestead: gabled house, door, window, sprig of wheat
    _ink(img, _poly(xx, yy, [(32, 66), (64, 36), (96, 66)]) - 1.0, 7.0)
    _paint(img, _cov(_poly(xx, yy, [(36, 64), (64, 40), (92, 64)])), (0.80, 0.80, 0.82))
    _ink(img, _rbox(xx, yy, 64, 78, 22, 14, 2.0) - 1.0, 5.0)
    _paint(img, _cov(_rbox(xx, yy, 64, 78, 20, 12, 2.0)), (0.86, 0.86, 0.88))
    _ink(img, _rbox(xx, yy, 64, 83, 5, 9, 1.5), 4.0, (0.40, 0.40, 0.43))
    _paint(img, _cov(_rbox(xx, yy, 50, 76, 4, 4, 1.0)), SHIELD_INK)
    _paint(img, _cov(_rbox(xx, yy, 78, 76, 4, 4, 1.0)), SHIELD_INK)


def _em_clans(img, xx, yy):      # three claw slashes through a crescent
    for ox in (-17, 0, 17):
        _ink(img, _seg(xx, yy, 64 + ox + 9, 28, 64 + ox - 9, 88) - 0.5, 8.0)
    _ink(img, np.abs(np.hypot(xx - 64, yy - 56) - 30) - 0.0 + 6.0 * (yy < 52), 5.0, (0.76, 0.76, 0.78))


def _em_drifters(img, xx, yy):   # a cart wheel on the road
    c = (64, 56)
    _ink(img, np.abs(np.hypot(xx - c[0], yy - c[1]) - 27), 6.5)
    for k in range(8):
        a = np.radians(22.5 + 45 * k)
        _ink(img, _seg(xx, yy, c[0], c[1], c[0] + np.cos(a) * 26, c[1] + np.sin(a) * 26), 3.6)
    _ink(img, np.hypot(xx - c[0], yy - c[1]) - 5.0, 5.0, (0.40, 0.40, 0.43))


def _em_freebooters(img, xx, yy):  # skull over crossed blades
    _ink(img, np.minimum(_seg(xx, yy, 30, 36, 98, 94), _seg(xx, yy, 98, 36, 30, 94)), 6.5, (0.74, 0.74, 0.76))
    skull = np.minimum(np.hypot(xx - 64, yy - 50) - 21, _rbox(xx, yy, 64, 68, 13, 11, 4))
    _paint(img, _cov(skull - 3.0), (0.22, 0.22, 0.25))
    _paint(img, _cov(skull), (0.95, 0.95, 0.93))
    for ex in (54, 74):
        _paint(img, _cov(np.hypot(xx - ex, yy - 52) - 6.0), (0.25, 0.25, 0.28))
    _paint(img, _cov(_poly(xx, yy, [(64, 58), (59, 68), (69, 68)])), (0.25, 0.25, 0.28))
    for tx in (57, 64, 71):
        _paint(img, _cov(_seg(xx, yy, tx, 74, tx, 78) - 0.9), (0.30, 0.30, 0.33))


def _em_swarm(img, xx, yy):      # a beetle seen from above
    body = np.hypot((xx - 64) / 0.74, yy - 68) - 26
    _paint(img, _cov(body - 2.5), (0.30, 0.30, 0.33))
    _paint(img, _cov(body), (0.86, 0.86, 0.88))
    _ink(img, _seg(xx, yy, 64, 44, 64, 94), 3.0)                         # wing seam
    _paint(img, _cov(np.hypot(xx - 64, yy - 38) - 10.5), (0.30, 0.30, 0.33))
    _paint(img, _cov(np.hypot(xx - 64, yy - 38) - 8.0), SHIELD_INK)
    for sy, dx in ((54, 30), (68, 33), (82, 28)):
        for sx in (-1, 1):
            _ink(img, _seg(xx, yy, 64 + sx * 20, sy, 64 + sx * dx, sy + (4 if sy < 60 else 8)), 3.4)
    for sx in (-1, 1):
        _ink(img, _seg(xx, yy, 64 + sx * 5, 30, 64 + sx * 14, 18), 3.0)


def _star(xx, yy, cx, cy, r_out, r_in=None):
    """Signed distance of a five-point star (union of five spikes and the core)."""
    r_in = r_out * 0.42 if r_in is None else r_in
    tips = [(cx + r_out * np.cos(np.radians(-90 + 72 * k)), cy + r_out * np.sin(np.radians(-90 + 72 * k))) for k in range(5)]
    mids = [(cx + r_in * np.cos(np.radians(-54 + 72 * k)), cy + r_in * np.sin(np.radians(-54 + 72 * k))) for k in range(5)]
    d = _poly(xx, yy, mids)
    for k in range(5):
        d = np.minimum(d, _poly(xx, yy, [mids[k - 1], tips[k], mids[k]]))
    return d


def _em_player(img, xx, yy):     # the colony banner: pole, pennant, star
    _ink(img, _seg(xx, yy, 38, 98, 38, 22), 5.5)
    pen = _poly(xx, yy, [(40, 24), (96, 40), (40, 62)])
    _paint(img, _cov(pen), (0.86, 0.86, 0.88))
    _ink(img, np.abs(pen), 4.0)
    star = _star(xx, yy, 58, 45, 13)
    _paint(img, _cov(star - 1.5), (0.36, 0.36, 0.40))


def _set_neolithic(img, xx, yy):  # three tents, a palisade and a fire
    pal = (0.20, 0.20, 0.22)
    for cx, top, base, hw, fill in ((64, 34, 84, 26, 0.95), (28, 56, 90, 20, 0.88), (100, 56, 90, 20, 0.88)):
        t = _tri(xx, yy, cx, top, base, hw)
        _paint(img, _cov(t - 2.8), pal, 0.95)
        _paint(img, _cov(t), (fill, fill, fill - 0.01))
        _paint(img, _cov(_tri(xx, yy, cx, base - 20, base, 6)), (0.35, 0.33, 0.33))
        _paint(img, _cov(_seg(xx, yy, cx, top, cx, top - 10) - 1.8), pal)
    for px in range(14, 118, 12):
        _paint(img, _cov(_rbox(xx, yy, px, 100, 3.2, 8, 1.2) - 0.0), (0.30, 0.30, 0.32))
        _paint(img, _cov(_rbox(xx, yy, px, 100, 2.2, 7, 1.0)), (0.88, 0.88, 0.86))


def _set_industrial(img, xx, yy):  # sawtooth factory with stacks and smoke
    pal = (0.20, 0.20, 0.22)
    body = np.minimum(_rbox(xx, yy, 44, 76, 30, 20, 1.0),
                      np.minimum(_poly(xx, yy, [(14, 56), (34, 42), (34, 58)]),
                                 np.minimum(_poly(xx, yy, [(34, 56), (54, 42), (54, 58)]),
                                            _poly(xx, yy, [(54, 56), (74, 42), (74, 58)]))))
    _paint(img, _cov(body - 2.8), pal, 0.95)
    _paint(img, _cov(body), (0.92, 0.92, 0.90))
    for wx in (26, 44, 62):
        _paint(img, _cov(_rbox(xx, yy, wx, 78, 5, 6, 1.0)), (0.40, 0.42, 0.46))
    for sx, top in ((90, 26), (106, 38)):
        st = _rbox(xx, yy, sx, (top + 96) / 2, 7, (96 - top) / 2, 1.0)
        _paint(img, _cov(st - 2.8), pal, 0.95)
        _paint(img, _cov(st), (0.86, 0.86, 0.85))
        _paint(img, _cov(_rbox(xx, yy, sx, top + 4, 7.5, 2.6, 0.5)), (0.55, 0.55, 0.58))
    for cx, cy, r in ((90, 14, 7), (98, 6, 8)):
        _paint(img, _cov(np.hypot(xx - cx, yy - cy) - r), (0.72, 0.72, 0.74), 0.8)


def _ng1_icons():
    t = _base_tokens()
    out = [_scenario_icon(k) for k in SCENARIOS]
    out += [_passion_icon(l) for l in ("minor", "major")]
    out += [_badge_icon("skill_" + k, t, g, rgb) for k, g, rgb in SKILL_ICONS]
    out += [_badge_icon("trait", t, _g_trait, (0.76, 0.64, 0.96)),
            _badge_icon("factions", t, _g_factions, (0.95, 0.78, 0.40)),
            _badge_icon("minimap", t, _g_minimap, (0.45, 0.78, 0.82))]
    out += [_white_icon("faction_settlers", _em_settlers, {"faction": "settlers"}),
            _white_icon("faction_clans", _em_clans, {"faction": "clans"}),
            _white_icon("faction_drifters", _em_drifters, {"faction": "drifters"}),
            _white_icon("faction_freebooters", _em_freebooters, {"faction": "freebooters"}),
            _white_icon("faction_swarm", _em_swarm, {"faction": "swarm"}),
            _white_icon("faction_player", _em_player, {"faction": "player"}),
            _white_icon("settlement_neolithic", _set_neolithic, {"settlement": "neolithic"}, shield=False),
            _white_icon("settlement_industrial", _set_industrial, {"settlement": "industrial"}, shield=False)]
    return out


# ------------------------------------------------ social & host systems (ENG-003, ART §37)
def _arc(xx, yy, cx, cy, r, a0, a1):
    """Distance to a circular arc from angle a0 to a1 (degrees, +y down, 0 = right)."""
    ang = np.degrees(np.arctan2(yy - cy, xx - cx)) % 360.0
    lo, hi = a0 % 360.0, a1 % 360.0
    inside = (ang >= lo) & (ang <= hi) if lo <= hi else (ang >= lo) | (ang <= hi)
    d_ring = np.abs(np.hypot(xx - cx, yy - cy) - r)
    ends = [(cx + r * np.cos(np.radians(a)), cy + r * np.sin(np.radians(a))) for a in (a0, a1)]
    d_end = np.minimum(np.hypot(xx - ends[0][0], yy - ends[0][1]), np.hypot(xx - ends[1][0], yy - ends[1][1]))
    return np.where(inside, d_ring, d_end)


def _dot(xx, yy, x, y, r=1.5):
    return np.hypot(xx - x, yy - y) - r


def _heart(xx, yy, cx, cy, s=1.0):
    a = np.abs(np.hypot(xx - (cx - 8 * s), yy - (cy - 5 * s)) - 9 * s)
    b = np.abs(np.hypot(xx - (cx + 8 * s), yy - (cy - 5 * s)) - 9 * s)
    v = np.minimum(_seg(xx, yy, cx - 16 * s, cy, cx, cy + 17 * s), _seg(xx, yy, cx + 16 * s, cy, cx, cy + 17 * s))
    return np.minimum(np.minimum(a, b), v)


def _bubble(xx, yy, c, cy_off=-3, hw=22, hh=15):
    d = np.abs(_rbox(xx, yy, c, c + cy_off, hw, hh, 8))
    return np.minimum(d, _seg(xx, yy, c - 9, c + cy_off + hh, c - 15, c + cy_off + hh + 9))


def _g_s_chitchat(xx, yy, c):  # a speech bubble with three dots
    d = _bubble(xx, yy, c)
    for ox in (-10, 0, 10):
        d = np.minimum(d, _dot(xx, yy, c + ox, c - 3))
    return d


def _g_s_deep(xx, yy, c):  # two stacked bubbles with text lines: a long conversation
    a = np.abs(_rbox(xx, yy, c - 6, c - 10, 20, 11, 6))
    b = np.abs(_rbox(xx, yy, c + 6, c + 12, 20, 11, 6))
    d = np.minimum(a, b)
    d = np.minimum(d, _seg(xx, yy, c - 18, c - 10, c + 4, c - 10))
    d = np.minimum(d, _seg(xx, yy, c - 6, c + 12, c + 18, c + 12))
    return np.minimum(d, _seg(xx, yy, c - 16, c - 1, c - 20, c + 4))


def _g_s_slight(xx, yy, c):  # a frowning face
    d = np.abs(np.hypot(xx - c, yy - c) - 22)
    d = np.minimum(d, _dot(xx, yy, c - 8, c - 6, 2))
    d = np.minimum(d, _dot(xx, yy, c + 8, c - 6, 2))
    return np.minimum(d, _arc(xx, yy, c, c + 20, 12, 215, 325))


def _g_s_insult(xx, yy, c):  # a bubble with a hard exclamation mark
    d = _bubble(xx, yy, c)
    d = np.minimum(d, _seg(xx, yy, c, c - 14, c, c - 1))
    return np.minimum(d, _dot(xx, yy, c, c + 5, 2))


def _g_s_kind(xx, yy, c):  # a smiling face
    d = np.abs(np.hypot(xx - c, yy - c) - 22)
    d = np.minimum(d, _dot(xx, yy, c - 8, c - 6, 2))
    d = np.minimum(d, _dot(xx, yy, c + 8, c - 6, 2))
    return np.minimum(d, _arc(xx, yy, c, c + 2, 13, 30, 150))


def _g_s_romance(xx, yy, c):  # a heart
    return _heart(xx, yy, c, c - 2, 1.15)


def _g_s_rebuff(xx, yy, c):  # a small heart struck out by a slash ring
    d = _heart(xx, yy, c, c - 4, 0.8)
    d = np.minimum(d, np.abs(np.hypot(xx - c, yy - c) - 24))
    return np.minimum(d, _seg(xx, yy, c - 17, c - 17, c + 17, c + 17))


def _g_s_proposal(xx, yy, c):  # a ring with a gem
    d = np.abs(np.hypot(xx - c, yy - (c + 6)) - 15)
    gem = [(c, c - 22), (c + 8, c - 14), (c, c - 6), (c - 8, c - 14)]
    for i in range(4):
        ax, ay = gem[i]
        bx, by = gem[(i + 1) % 4]
        d = np.minimum(d, _seg(xx, yy, ax, ay, bx, by))
    return d


def _g_s_breakup(xx, yy, c):  # a heart broken down a zigzag, the halves apart
    d = _heart(xx, yy, c, c - 2, 1.15)
    zig = np.minimum(_seg(xx, yy, c + 2, c - 18, c - 5, c - 6), _seg(xx, yy, c - 5, c - 6, c + 4, c + 4))
    zig = np.minimum(zig, _seg(xx, yy, c + 4, c + 4, c - 2, c + 20))
    return np.minimum(d, zig)


def _g_s_fight(xx, yy, c):  # a lightning bolt
    pts = [(c + 8, c - 24), (c - 10, c + 2), (c + 2, c + 2), (c - 8, c + 24), (c + 12, c - 6), (c, c - 6), (c + 8, c - 24)]
    d = np.full(xx.shape, 1e9, np.float32)
    for i in range(len(pts) - 1):
        d = np.minimum(d, _seg(xx, yy, *pts[i], *pts[i + 1]))
    return d


def _g_s_lovin(xx, yy, c):  # two hearts, one behind the other
    return np.minimum(_heart(xx, yy, c - 7, c - 5, 0.8), _heart(xx, yy, c + 8, c + 3, 0.95))


def _g_capture(xx, yy, c):  # a round net: ring and lattice
    d = np.abs(np.hypot(xx - c, yy - c) - 22)
    for o in (-11, 0, 11):
        d = np.minimum(d, _seg(xx, yy, c + o, c - 20, c + o, c + 20) + (np.hypot(xx - c, yy - c) > 22) * 99)
        d = np.minimum(d, _seg(xx, yy, c - 20, c + o, c + 20, c + o) + (np.hypot(xx - c, yy - c) > 22) * 99)
    return d


def _g_arrest(xx, yy, c):  # handcuffs: two cuffs joined by a chain
    d = np.minimum(np.abs(np.hypot(xx - (c - 14), yy - c) - 10), np.abs(np.hypot(xx - (c + 14), yy - c) - 10))
    d = np.minimum(d, _seg(xx, yy, c - 4, c, c + 4, c))
    d = np.minimum(d, _seg(xx, yy, c - 14, c - 10, c - 14, c - 18))
    return np.minimum(d, _seg(xx, yy, c + 14, c - 10, c + 14, c - 18))


def _g_prisoner(xx, yy, c):  # a barred window
    d = np.abs(_rbox(xx, yy, c, c, 20, 24, 8))
    for o in (-10, 0, 10):
        d = np.minimum(d, _seg(xx, yy, c + o, c - 22, c + o, c + 22))
    return d


def _g_caravan(xx, yy, c):  # a covered wagon
    d = np.minimum(_seg(xx, yy, c - 22, c + 6, c + 22, c + 6), _seg(xx, yy, c - 22, c + 6, c - 22, c - 4))
    d = np.minimum(d, _seg(xx, yy, c + 22, c + 6, c + 22, c - 4))
    d = np.minimum(d, _arc(xx, yy, c, c - 4, 22, 180, 360))
    d = np.minimum(d, np.abs(np.hypot(xx - (c - 12), yy - (c + 14)) - 6))
    d = np.minimum(d, np.abs(np.hypot(xx - (c + 12), yy - (c + 14)) - 6))
    return np.minimum(d, _seg(xx, yy, c + 22, c + 6, c + 30, c + 12))


def _g_history(xx, yy, c):  # a clock with a counter-clockwise arrow
    d = _arc(xx, yy, c, c, 22, 0, 300)
    d = np.minimum(d, _seg(xx, yy, c, c, c, c - 13))
    d = np.minimum(d, _seg(xx, yy, c, c, c + 9, c + 5))
    d = np.minimum(d, _seg(xx, yy, c + 22, c, c + 14, c - 8))
    return np.minimum(d, _seg(xx, yy, c + 22, c, c + 28, c - 9))


def _g_surgery(xx, yy, c):  # a scalpel
    d = _seg(xx, yy, c - 22, c + 22, c + 2, c - 2)
    d = np.minimum(d, _seg(xx, yy, c + 2, c - 2, c + 22, c - 24))
    d = np.minimum(d, _seg(xx, yy, c + 22, c - 24, c + 14, c - 2))
    return np.minimum(d, _seg(xx, yy, c + 14, c - 2, c + 2, c - 2))


SOCIAL_ICONS = (
    ("social_chitchat", _g_s_chitchat, (0.80, 0.85, 0.92)), ("social_deep_talk", _g_s_deep, (0.55, 0.75, 0.95)),
    ("social_slight", _g_s_slight, (0.86, 0.62, 0.40)), ("social_insult", _g_s_insult, (0.93, 0.40, 0.34)),
    ("social_kind_words", _g_s_kind, (0.98, 0.85, 0.45)), ("social_romance", _g_s_romance, (0.95, 0.55, 0.70)),
    ("social_rebuff", _g_s_rebuff, (0.70, 0.60, 0.82)), ("social_proposal", _g_s_proposal, (0.98, 0.82, 0.45)),
    ("social_breakup", _g_s_breakup, (0.76, 0.55, 0.62)), ("social_fight", _g_s_fight, (0.88, 0.32, 0.24)),
    ("social_lovin", _g_s_lovin, (0.98, 0.45, 0.60)),
    ("capture", _g_capture, (0.78, 0.55, 0.32)), ("arrest", _g_arrest, (0.62, 0.72, 0.86)),
    ("prisoner", _g_prisoner, (0.72, 0.72, 0.76)), ("caravan", _g_caravan, (0.88, 0.72, 0.42)),
    ("history", _g_history, (0.72, 0.78, 0.90)), ("surgery", _g_surgery, (0.46, 0.86, 0.66)),
)

MARRIAGE_FACINGS = SPOT_FACINGS


def _marriage_spot(facing):
    """A wedding place marked on the floor: a dashed chalk border, two ribbon
    bows at the ends and a pair of linked rings in the middle. Authored 2x1
    (south/north) and turned to 1x2 for east/west."""
    horizontal = facing in ("south", "north")
    w, h = (2, 1) if horizontal else (1, 2)
    W, H = w * N, h * N
    yy, xx = np.mgrid[0:H, 0:W].astype(np.float32) + 0.5
    if not horizontal:   # draw upright in a 2x1 frame, then read it transposed
        xx, yy = yy, xx
    LW, LH = 2 * N, N
    img = np.zeros((H, W, 4), np.float32)   # drawn in the long-axis frame (xx runs along the 2-tile side)
    chalk, ink, rose = (0.95, 0.91, 0.82), (0.10, 0.09, 0.08), (0.93, 0.55, 0.62)
    cx, cy = LW / 2, LH / 2
    outside = _rbox(xx, yy, cx, cy, LW / 2 - 12, LH / 2 - 12, 16)
    ring = np.abs(outside)
    dash = (np.floor(xx / 12.0) % 2) == 0
    dash = dash | (np.abs(xx - cx) > LW / 2 - 40)
    _paint(img, _cov(outside), ink, 0.14)
    _paint(img, _cov(ring - 2.6) * dash, ink, 0.45)
    _paint(img, _cov(ring - 1.8) * dash, chalk, 0.9)
    for sx in (-1, 1):   # ribbon bows at both ends
        bx = cx + sx * (LW / 2 - 34)
        for k in (-1, 1):
            _paint(img, _cov(np.hypot((xx - (bx + k * 9)) / 1.4, yy - (cy - 2)) - 8), ink, 0.45)
            _paint(img, _cov(np.hypot((xx - (bx + k * 9)) / 1.4, yy - (cy - 2)) - 6.6), rose, 0.95)
        _paint(img, _cov(np.hypot(xx - bx, yy - (cy - 2)) - 4.5), chalk, 0.95)
        for k in (-1, 1):
            _paint(img, _cov(_cap(xx, yy, bx, cy + 2, bx + k * 8, cy + 20, 2.4)), rose, 0.9)
    for sx in (-1, 1):   # two linked rings
        r = np.abs(np.hypot(xx - (cx + sx * 9), yy - cy) - 15)
        _paint(img, _cov(r - 4.5), ink, 0.5)
        _paint(img, _cov(r - 3.0), (0.97, 0.82, 0.42) if sx < 0 else chalk, 0.95)
    name = "marriage_spot_" + facing
    S.write_png(img, S.out_path("overlay", name))
    meta.write("overlay", name, {"image": "generated/world/overlay/%s.png" % name, "scale": C.SCALE,
                                 "canvas": [W, H], "footprint": [w, h], "origin_px": [0, 0],
                                 "overlay": "marriage_spot", "marking": True, "facing": facing,
                                 "rotatable": True})
    return S.out_path("overlay", name)


def _party_spot():
    """A party place: a dashed chalk ring with eight small paper lanterns."""
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    chalk, ink = (0.95, 0.91, 0.82), (0.10, 0.09, 0.08)
    ring = np.abs(np.hypot(xx - c, yy - c) - 44)
    ang = np.arctan2(yy - c, xx - c)
    dash = (np.floor((ang + np.pi) * 44 / 12.0) % 2) == 0
    _paint(img, _cov(np.hypot(xx - c, yy - c) - 44), ink, 0.12)
    _paint(img, _cov(ring - 2.6) * dash, ink, 0.45)
    _paint(img, _cov(ring - 1.8) * dash, chalk, 0.9)
    cols = ((1.0, 0.78, 0.30), (0.95, 0.45, 0.38), (0.55, 0.80, 0.90), (0.62, 0.85, 0.50))
    for k in range(8):
        a = 2 * np.pi * k / 8 + np.pi / 8
        lx, ly = c + 44 * np.cos(a), c + 44 * np.sin(a)
        col = cols[k % 4]
        d = np.hypot(xx - lx, yy - ly)
        _paint(img, _cov(d - 15), col, 0.18)     # warm glow
        _paint(img, _cov(d - 8.2), ink, 0.55)
        _paint(img, _cov(d - 6.6), col, 1.0)
        _paint(img, _cov(np.hypot(xx - (lx - 2), yy - (ly - 2)) - 2.2), (1, 1, 0.9), 0.8)
    for k, (px, py) in enumerate(((c - 12, c - 6), (c + 10, c + 8), (c + 2, c - 16), (c - 8, c + 14), (c + 18, c - 4))):
        _paint(img, _cov(_rbox(xx, yy, px, py, 3.0, 1.6, 0.6)), cols[k % 4], 0.9)   # confetti
    return _save("party_spot", img, {"overlay": "party_spot", "marking": True})


def _triangulate(pts):
    """Ear-clip a simple polygon (any winding) into triangles for `_poly`."""
    pts = list(pts)
    area = sum(pts[i][0] * pts[(i + 1) % len(pts)][1] - pts[(i + 1) % len(pts)][0] * pts[i][1] for i in range(len(pts)))
    if area < 0:
        pts.reverse()

    def cross(o, a, b):
        return (a[0] - o[0]) * (b[1] - o[1]) - (a[1] - o[1]) * (b[0] - o[0])

    def inside(p, a, b, c):
        return cross(a, b, p) >= 0 and cross(b, c, p) >= 0 and cross(c, a, p) >= 0

    tris = []
    while len(pts) > 3:
        for i in range(len(pts)):
            a, b, c = pts[i - 1], pts[i], pts[(i + 1) % len(pts)]
            if cross(a, b, c) <= 0:
                continue
            if any(inside(p, a, b, c) for p in pts if p not in (a, b, c)):
                continue
            tris.append((a, b, c))
            del pts[i]
            break
        else:
            break
    tris.append(tuple(pts))
    return tris


def _fight_lightning():
    """A red lightning bolt with a dark halo and a hot core, floated over the
    heads of two brawlers (or a portrait). 1x1, origin at the top left."""
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    red, hot, ink = (0.88, 0.28, 0.20), (1.0, 0.82, 0.55), (0.10, 0.07, 0.06)
    bolt = [(c + 4, c - 56), (c + 28, c - 56), (c + 12, c - 14), (c + 32, c - 14), (c - 12, c + 56),
            (c - 2, c + 6), (c - 24, c + 6)]
    d = np.full(xx.shape, 1e9, np.float32)
    for tri in _triangulate(bolt):
        d = np.minimum(d, _poly(xx, yy, tri))
    _paint(img, _cov(d - 4.0), ink, 0.9)
    _paint(img, _cov(d), red)
    core = np.minimum(_seg(xx, yy, c + 14, c - 48, c - 2, c - 4), _seg(xx, yy, c + 10, c - 8, c - 6, c + 36))
    _paint(img, _cov(core - 2.0), hot, 0.9)
    return _save("fight_lightning", img, {"overlay": "fight_lightning", "marking": True})


def _social_icons(t):
    return [_badge_icon(n, t, g, rgb) for n, g, rgb in SOCIAL_ICONS]


# ------------------------------------------------------------ PLAY-015 fire: ash decal and icons
def _ash(k):
    """Burnt-ground decal, 128x128 (one tile): a ragged charcoal blotch with grey ash
    drifts and a few dim ember specks. No outline; edges fade to nothing."""
    from .effect import _mat
    char, grey, emb = _mat("ash_char"), _mat("ash_grey"), _mat("ash_ember")
    yy, xx = np.mgrid[0:N, 0:N].astype(np.float32) + 0.5
    rng = np.random.RandomState(1500 + k)
    cx, cy = N / 2 + rng.uniform(-4, 4), N / 2 + rng.uniform(-4, 4)
    ang = np.arctan2(yy - cy, xx - cx)
    rad = 44 + sum(rng.uniform(3, 7) * np.sin(ang * f + rng.uniform(0, 6.28)) for f in (2, 3, 5, 7))
    d = np.hypot(xx - cx, yy - cy)
    blot = np.clip((rad - d) / 7.0, 0, 1)
    for _ in range(7):                                   # extra scorched lobes
        px, py, r = cx + rng.uniform(-34, 34), cy + rng.uniform(-34, 34), rng.uniform(9, 18)
        blot = np.maximum(blot, np.clip((r - np.hypot(xx - px, yy - py)) / 5.0, 0, 1) * 0.9)
    img = np.zeros((N, N, 4), np.float32)
    img[..., :3] = char["base"]
    img[..., 3] = blot * 0.88
    # darker charred core
    _paint(img, np.clip((rad * 0.62 - d) / 8.0, 0, 1) * (blot > 0.3), char["shadow"], 0.7)
    # grey ash drifts: soft lobes of lighter powder
    for _ in range(9):
        px, py, r = cx + rng.uniform(-32, 32), cy + rng.uniform(-32, 32), rng.uniform(6, 15)
        m = np.clip((r - np.hypot((xx - px) * 0.8, yy - py)) / 4.0, 0, 1) * (blot > 0.5)
        _paint(img, m, grey["base"] if rng.rand() < 0.6 else grey["shadow"], 0.55)
    for _ in range(10):                                  # fine light flecks
        px, py = cx + rng.uniform(-40, 40), cy + rng.uniform(-40, 40)
        _paint(img, np.clip(1.4 - np.hypot(xx - px, yy - py) / 1.6, 0, 1) * (blot > 0.5), grey["light"], 0.6)
    for _ in range(5):                                   # a few dim ember specks
        px, py = cx + rng.uniform(-26, 26), cy + rng.uniform(-26, 26)
        _paint(img, np.clip(1.4 - np.hypot(xx - px, yy - py) / 2.0, 0, 1) * (blot > 0.6), emb["shadow"], 0.95)
        _paint(img, np.clip(1.2 - np.hypot(xx - px, yy - py) / 0.9, 0, 1) * (blot > 0.6), emb["base"], 0.9)
    return _save_flat("ash_%d" % k, N, N, img, {"overlay": "ash", "variant": k, "tileable": False, "marking": True})


def _g_extinguish(xx, yy, c):  # water drop
    ring = np.abs(np.hypot(xx - c, yy - (c + 8)) - 13)
    ring = np.where(yy > c + 2, ring, 1e9)
    d = np.minimum(_seg(xx, yy, c, c - 22, c - 11, c + 2), _seg(xx, yy, c, c - 22, c + 11, c + 2))
    return np.minimum(ring, d)


def _g_start_fire(xx, yy, c):  # flame with an inner tongue
    pts = [(c + 2, c - 24), (c + 14, c - 2), (c + 12, c + 14), (c, c + 22), (c - 12, c + 14), (c - 14, c - 2),
           (c - 5, c - 9), (c + 2, c - 24)]
    d = np.full(xx.shape, 1e9, np.float32)
    for a, b in zip(pts, pts[1:]):
        d = np.minimum(d, _seg(xx, yy, a[0], a[1], b[0], b[1]))
    return np.minimum(d, _seg(xx, yy, c, c + 14, c, c + 8))


def _g_pen_marker(xx, yy, c):  # post with a sign board carrying a small animal
    d = _seg(xx, yy, c, c + 24, c, c + 8)
    d = np.minimum(d, np.abs(_rbox(xx, yy, c, c - 7, 21, 14, 3)))
    d = np.minimum(d, _seg(xx, yy, c - 9, c - 7, c + 7, c - 7))     # body
    d = np.minimum(d, _seg(xx, yy, c - 9, c - 7, c - 9, c - 1))     # legs
    return np.minimum(d, _seg(xx, yy, c + 7, c - 7, c + 7, c - 1))


def _g_snow_clear(xx, yy, c):  # snowflake over a scraped line
    d = np.full(xx.shape, 1e9, np.float32)
    for a in (90, 30, -30):
        r = np.radians(a)
        dx, dy = np.cos(r) * 17, np.sin(r) * 17
        d = np.minimum(d, _seg(xx, yy, c - dx, c - 4 - dy, c + dx, c - 4 + dy))
    return np.minimum(d, _seg(xx, yy, c - 20, c + 20, c + 20, c + 20))


def _g_home_auto(xx, yy, c):  # house with an auto-expanding plus
    d = _g_house(xx + 5, yy - 4, c)
    d = np.minimum(d, _seg(xx, yy, c + 21, c - 28, c + 21, c - 14))
    return np.minimum(d, _seg(xx, yy, c + 14, c - 21, c + 28, c - 21))


def _g_roof_area(xx, yy, c):  # roof chevron inside dashed corner brackets
    d = _g_roof(xx, yy - 4, c)
    for sx in (-1, 1):
        for sy in (-1, 1):
            x0, y0 = c + sx * 26, c + sy * 22
            d = np.minimum(d, _seg(xx, yy, x0, y0, x0 - sx * 11, y0))
            d = np.minimum(d, _seg(xx, yy, x0, y0, x0, y0 - sy * 9))
    return d


def build():
    t = _base_tokens()
    out = [_selection(t), _designation("designation_cut", t, _g_cut),
           _designation("designation_harvest", t, _g_harvest), _designation("designation_mine", t, _g_mine), _designation("designation_smooth", t, _g_smooth),
           _designation("designation_deconstruct", t, _g_deconstruct),
           _designation("designation_hunt", t, _g_hunt),
           _designation("designation_tame", t, _g_tame), _pen_zone(t), _forbidden_marker(t),
           _stockpile(t), _blueprint(t), _grow_zone(t)] + [_sleeping_spot(f) for f in SPOT_FACINGS]
    out += [_badge_icon("order_harvest", t, _g_harvest, t["designation"]),
            _badge_icon("order_deconstruct", t, _g_deconstruct, t["danger"]),
            _badge_icon("order_cancel", t, _g_cancel, t["selection"]),
            _badge_icon("zone_stockpile", t, _g_stockpile, t["stockpile"]),
            _badge_icon("draft", t, _g_draft, t["danger"]),
            _badge_icon("order_cut", t, _g_cut, t["designation"]),
            _badge_icon("order_mine", t, _g_mine, t["designation"]),
            _badge_icon("order_smooth", t, _g_smooth, t["designation"]),
            _badge_icon("order_hunt", t, _g_hunt, t["danger"]),
            _badge_icon("order_tame", t, _g_tame, (0.95, 0.55, 0.62)),
            _badge_icon("zone_pen", t, _g_pen, (0.78, 0.55, 0.32)),
            _badge_icon("zone_grow", t, _g_sprout, (0.45, 0.78, 0.36)),
            _badge_icon("roof_build", t, _g_roof_build, (0.35, 0.55, 0.82)),
            _badge_icon("roof_remove", t, _g_roof_remove, t["danger"]),
            _badge_icon("overlay_roof", t, _g_overlay_roof, (0.45, 0.65, 0.90)),
            _badge_icon("order_forbid", t, _g_forbid, t["danger"]),
            _badge_icon("order_allow", t, _g_allow, (0.45, 0.78, 0.36)),
            _badge_icon("overlay_rooms", t, _g_rooms, (0.35, 0.72, 0.70)),
            _badge_icon("home", t, _g_home, (0.29, 0.50, 0.83)),
            _badge_icon("home_clear", t, _g_home_clear, t["danger"]),
            _badge_icon("plan", t, _g_plan, (0.72, 0.83, 0.94)),
            _badge_icon("plan_clear", t, _g_plan_clear, t["danger"]),
            _ash(0), _ash(1), _ash(2),
            _badge_icon("extinguish", t, _g_extinguish, (0.40, 0.70, 0.95)),
            _badge_icon("start_fire", t, _g_start_fire, (0.98, 0.58, 0.22)),
            _badge_icon("pen_marker", t, _g_pen_marker, (0.78, 0.55, 0.32)),
            _badge_icon("snow_clear", t, _g_snow_clear, (0.80, 0.90, 0.98)),
            _badge_icon("home_auto", t, _g_home_auto, (0.29, 0.50, 0.83)),
            _badge_icon("roof_area", t, _g_roof_area, (0.45, 0.65, 0.90))]
    out += [_biome_icon(k) for k in BIOMES]
    out += _ng1_icons()
    out += [_marriage_spot(f) for f in MARRIAGE_FACINGS] + [_party_spot(), _fight_lightning()]
    out += _social_icons(t)
    out += [_shadow_blob(), _shadow_box(), _roof_tile("roof_built", "built"),
            _roof_tile("roof_natural", "natural"), _roof_tile("roof_plan", "plan"),
            _roof_tile("roof_remove", "remove")]
    # One icon per roster species (ui_icon/<key>), cut from the standing
    # east frame; a species whose art is not rendered yet is skipped.
    icons = dict(ICONS)
    for sp in ROSTER:
        icons[sp.key] = ("animal", sp.key + "_east")
    for name, (fam, spr) in icons.items():
        p = _icon(name, fam, spr)
        if p:
            out.append(p)
    return out
