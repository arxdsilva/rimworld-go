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


def _stockpile(t):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    _paint(img, np.ones((N, N), np.float32), t["stockpile"], 0.16)
    stripes = ((xx + yy) % 24) < 3
    border = (xx < 4) | (yy < 4) | (xx > N - 4) | (yy > N - 4)
    _paint(img, (stripes & border).astype(np.float32), t["stockpile"], 0.75)
    return _save("stockpile", img, {"overlay": "zone", "tileable": True})


def _grow_zone(t):
    xx, yy = _grid()
    img = np.zeros((N, N, 4), np.float32)
    green = (0.45, 0.78, 0.36)
    _paint(img, np.ones((N, N), np.float32), green, 0.14)
    rows = ((yy % 32) < 3)
    border = (xx < 4) | (yy < 4) | (xx > N - 4) | (yy > N - 4)
    _paint(img, (rows & ~border).astype(np.float32), green, 0.35)
    _paint(img, ((((xx + yy) % 24) < 3) & border).astype(np.float32), green, 0.8)
    return _save("grow_zone", img, {"overlay": "zone", "tileable": True})


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


# ---------------------------------------------------------------- icons
ICONS = {
    # icon name: (family, sprite)
    "wood": ("item", "wood_pile"),
    "food": ("item", "ration_pack"),
    "berries": ("item", "berries_basket"),
    "amberroot": ("item", "food_crate"),
    "meat": ("item", "meat_pile"),
    "meal": ("item", "meal_simple"),
    "steel": ("item", "steel_bars"),
    "stone": ("item", "stone_blocks"),
    "bed": ("furniture", "bed_basic_south"),
    "cot": ("furniture", "bed_cot_south"),
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
    "crop": ("plant", "crop_harvestable"),
    "colonist": ("pawn", "body_south"),
    "dustbuck": ("animal", "dustbuck_east"),
    "butcher_table": ("furniture", "butcher_table_south"),
    "corpse_dustbuck": ("item", "corpse_dustbuck"),
    "corpse_ridgewolf": ("item", "corpse_ridgewolf"),
    "ridgewolf": ("animal", "ridgewolf_east"),
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
           _stockpile(t), _blueprint(t), _grow_zone(t)]
    out += [_badge_icon("order_harvest", t, _g_harvest, t["designation"]),
            _badge_icon("order_deconstruct", t, _g_deconstruct, t["danger"]),
            _badge_icon("order_cancel", t, _g_cancel, t["selection"]),
            _badge_icon("zone_stockpile", t, _g_stockpile, t["stockpile"]),
            _badge_icon("draft", t, _g_draft, t["danger"]),
            _badge_icon("order_cut", t, _g_cut, t["designation"]),
            _badge_icon("order_mine", t, _g_mine, t["designation"]),
            _badge_icon("order_hunt", t, _g_hunt, t["danger"]),
            _badge_icon("zone_grow", t, _g_sprout, (0.45, 0.78, 0.36))]
    for name, (fam, spr) in ICONS.items():
        p = _icon(name, fam, spr)
        if p:
            out.append(p)
    return out
