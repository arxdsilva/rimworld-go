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


def _g_mine(xx, yy, c):  # pick: shaft + curved head
    d = _seg(xx, yy, c - 12, c + 14, c + 6, c - 8)
    head = np.abs(np.hypot(xx - (c + 18), yy - (c + 6)) - 24)
    head = np.maximum(head, np.abs(np.arctan2(yy - (c + 6), xx - (c + 18)) + 2.2) * 24 - 16)
    return np.minimum(d, head)


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
    "lamp": ("furniture", "lamp_standing"),
    "tree": ("plant", "tree_00"),
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


def build():
    t = _base_tokens()
    out = [_selection(t), _designation("designation_cut", t, _g_cut),
           _designation("designation_harvest", t, _g_harvest), _designation("designation_mine", t, _g_mine),
           _designation("designation_deconstruct", t, _g_deconstruct),
           _designation("designation_hunt", t, _g_hunt),
           _designation("designation_tame", t, _g_tame), _pen_zone(t),
           _stockpile(t), _blueprint(t), _grow_zone(t)] + [_sleeping_spot(f) for f in SPOT_FACINGS]
    out += [_badge_icon("order_harvest", t, _g_harvest, t["designation"]),
            _badge_icon("order_deconstruct", t, _g_deconstruct, t["danger"]),
            _badge_icon("order_cancel", t, _g_cancel, t["selection"]),
            _badge_icon("zone_stockpile", t, _g_stockpile, t["stockpile"]),
            _badge_icon("draft", t, _g_draft, t["danger"]),
            _badge_icon("order_cut", t, _g_cut, t["designation"]),
            _badge_icon("order_mine", t, _g_mine, t["designation"]),
            _badge_icon("order_hunt", t, _g_hunt, t["danger"]),
            _badge_icon("order_tame", t, _g_tame, (0.95, 0.55, 0.62)),
            _badge_icon("zone_pen", t, _g_pen, (0.78, 0.55, 0.32)),
            _badge_icon("zone_grow", t, _g_sprout, (0.45, 0.78, 0.36)),
            _badge_icon("roof_build", t, _g_roof_build, (0.35, 0.55, 0.82)),
            _badge_icon("roof_remove", t, _g_roof_remove, t["danger"]),
            _badge_icon("overlay_roof", t, _g_overlay_roof, (0.45, 0.65, 0.90))]
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
