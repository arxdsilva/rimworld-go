"""Gear tab buttons and the game's own mouse pointer (ART spec §17). Our
own designs: nothing traced or reconstructed.

Gear tab (GEAR-2), on the order-badge disc like every pawn command:
- ui_icon/gear_lock, gear_unlock: a worn piece is kept on (forced) or free;
  a shut brass padlock, and the same lock with its shackle swung open in
  the chrome's pale grey, so the pair reads as one toggle's two states.
- ui_icon/gear_drop: put a carried thing down; a tied sack on the ground
  line, a down arrow beside it.
- ui_icon/gear_eat: eat from the pack; a drumstick held up (the order to
  eat a meal on the map keeps the plate, ui_icon/eat).
- ui_icon/gear_info: open the thing's info card; a bold "i" with a flag.

Pointer (OPT-028), family `cursor`, drawn by the game in place of the
system arrow (Ebitengine offers only the system shapes):
- cursor/pointer, cursor/pointer_busy: an arrow, its tip the hotspot
  (attachment `hotspot`, canvas px). The busy one adds a gold progress arc
  at the lower right. Canvas 64 px is the @2x master of a 32 px OS cursor
  (scale 2, like every sprite here); the renderer scales it with the UI.

    blender -b --factory-startup -P assets/source/blender/build.py -- gearopt
"""
import os

import numpy as np

from .. import contracts as C
from .. import meta
from .. import stage as S
from . import overlay as O

LOCK_RGB = (0.93, 0.76, 0.40)     # brass: the piece is held on
UNLOCK_RGB = (0.78, 0.82, 0.88)   # chrome grey: nothing holds it
DROP_RGB = (0.86, 0.68, 0.50)     # sacking tan
EAT_RGB = (0.98, 0.72, 0.38)      # ui_icon/eat's orange: the same act
INFO_RGB = (0.55, 0.75, 0.96)     # the info letter's blue

CURSOR_DIR = os.path.join(C.ASSETS_DIR, "generated", "ui", "cursors")
CURSOR_PX = 64                    # @2x of a 32 px OS cursor
TIP = (7.0, 5.0)                  # the hotspot, canvas px
INK = (0.059, 0.063, 0.071)       # BADGE_EDGE: the chrome's near-black
BODY = (0.91, 0.89, 0.85)         # ui.text's warm off-white
SHADE = (0.70, 0.68, 0.64)        # the facet turned from the upper-left light
GOLD = (0.85, 0.71, 0.36)         # ui.accent


# --------------------------------------------------------------- gear tab
def _shackle(xx, yy, c, lift, swing):
    """A padlock's U shackle over the body top (y = c - 2); lift raises it,
    swing turns it open about its left foot (degrees)."""
    rx, ry = O._rot(xx, yy, c - 12, c - 2, swing) if swing else (xx, yy)
    top = c - 16 - lift
    arc = O._arc(rx, ry, c, top, 12, 180, 360)
    legs = np.minimum(O._seg(rx, ry, c - 12, top, c - 12, c - 2), O._seg(rx, ry, c + 12, top, c + 12, c - 2 - lift))
    return np.minimum(arc, legs)


def _lock_body(xx, yy, c):
    body = O._rbox(xx, yy, c, c + 12, 20, 15, 4)
    hole = np.minimum(np.hypot(xx - c, yy - (c + 8)) - 4.5, O._poly(xx, yy, [(c - 2.5, c + 9), (c + 2.5, c + 9), (c + 3.5, c + 19), (c - 3.5, c + 19)]))
    return body, hole


def _g_lock(xx, yy, c):  # a shut padlock: shackle home, keyhole in the body
    body, hole = _lock_body(xx, yy, c)
    return [_shackle(xx, yy, c, 0, 0), body, (hole, "cut")]


def _g_unlock(xx, yy, c):  # the same padlock, shackle lifted and swung open
    body, hole = _lock_body(xx, yy, c)
    return [_shackle(xx, yy, c, 6, -28), body, (hole, "cut")]


def _g_drop(xx, yy, c):  # a tied sack set down on the ground line, an arrow down beside it
    sx, sy = c + 10, c + 9
    sack = np.minimum(O._poly(xx, yy, [(sx - 6, sy - 6), (sx + 6, sy - 6), (sx + 14, sy + 12), (sx - 14, sy + 12)]) - 3,
                      O._poly(xx, yy, [(sx - 3, sy - 8), (sx + 3, sy - 8), (sx + 8, sy - 16), (sx - 8, sy - 16)]))
    tie = O._seg(xx, yy, sx - 6, sy - 8, sx + 6, sy - 8)
    ground = O._seg(xx, yy, c - 30, c + 26, c + 28, c + 26)
    arrow = np.minimum(O._seg(xx, yy, c - 16, c - 26, c - 16, c + 14),
                       O._polyline(xx, yy, [(c - 26, c + 3), (c - 16, c + 14), (c - 6, c + 3)]))
    return [ground + 1.0, sack, (tie + 1.2, "cut"), arrow]


def _g_eat(xx, yy, c):  # a drumstick: the meat a plump teardrop, the bone out of it with a double knob
    rx, ry = O._rot(xx, yy, c, c, 45)
    meat = np.minimum(np.hypot(rx - c, ry - (c - 9)) - 16, O._poly(rx, ry, [(c - 15, c - 3), (c + 15, c - 3), (c + 5, c + 13), (c - 5, c + 13)]))
    bone = np.minimum(O._cap(rx, ry, c, c + 8, c, c + 24, 3.2),
                      np.minimum(np.hypot(rx - (c - 5), ry - (c + 27)) - 5, np.hypot(rx - (c + 5), ry - (c + 27)) - 5))
    shine = O._cap(rx, ry, c - 8, c - 15, c - 3, c - 19, 1.6)
    return [bone, meat, (shine, "cut")]


def _g_info(xx, yy, c):  # a bold "i": dot, a stem with a flag at its top and a foot
    dot = np.hypot(xx - (c + 1), yy - (c - 22)) - 6.5
    stem = O._rbox(xx, yy, c + 1, c + 6, 5.5, 16, 1.5)
    flag = O._rbox(xx, yy, c - 7, c - 7.5, 6, 2.5, 1)
    foot = O._rbox(xx, yy, c + 1, c + 22, 13, 2.5, 1)
    return np.minimum(np.minimum(dot, stem), np.minimum(flag, foot)) - 0.5


GEAR_ICONS = (   # name, glyph, colour
    ("gear_lock", _g_lock, LOCK_RGB),
    ("gear_unlock", _g_unlock, UNLOCK_RGB),
    ("gear_drop", _g_drop, DROP_RGB),
    ("gear_eat", _g_eat, EAT_RGB),
    ("gear_info", _g_info, INFO_RGB),
)


# ---------------------------------------------------------------- pointer
def _arrow(xx, yy):
    """The pointer: a dart from the tip, its base swept in to a notch (two
    convex halves), and a short tail out of the notch."""
    tx, ty = TIP
    wl, notch, wr = (tx + 1, ty + 35), (tx + 12, ty + 24), (tx + 26, ty + 28)
    head = np.minimum(O._poly(xx, yy, [(tx, ty), wl, notch]), O._poly(xx, yy, [(tx, ty), notch, wr]))
    tail = O._cap(xx, yy, notch[0] - 1, notch[1] - 2, tx + 21, ty + 39, 3.4)
    return np.minimum(head, tail)


def _facet(xx, yy):
    """The arrow's lower-right half, turned from the upper-left light."""
    tx, ty = TIP
    return (xx - tx) * 0.82 - (yy - ty) * 0.57 > 2.0


def _cursor(name, busy):
    yy, xx = np.mgrid[0:CURSOR_PX, 0:CURSOR_PX].astype(np.float32) + 0.5
    img = np.zeros((CURSOR_PX, CURSOR_PX, 4), np.float32)
    d = _arrow(xx, yy)
    O._paint(img, O._cov(d - 3.0), INK)                     # the ink line, 1.5 px at 1x
    O._paint(img, O._cov(d), BODY)
    O._paint(img, O._cov(d + 1.0) * _facet(xx, yy), SHADE)  # inside the line, so the edge stays crisp
    if busy:
        cx, cy = 48.0, 48.0
        O._paint(img, O._cov(np.hypot(xx - cx, yy - cy) - 14.5), INK)
        O._paint(img, O._cov(np.hypot(xx - cx, yy - cy) - 12.0), (0.188, 0.196, 0.212))  # BADGE_DISC
        O._paint(img, O._cov(O._arc(xx, yy, cx, cy, 7.5, -90, 180) - 2.6), GOLD)
        O._paint(img, O._cov(np.hypot(xx - (cx + 7.5), yy - cy) - 3.6), GOLD)          # the arc's head
    path = os.path.join(CURSOR_DIR, name + ".png")
    S.write_png(img, path)
    meta.write("cursor", name, {"image": "generated/ui/cursors/%s.png" % name, "scale": C.SCALE,
                                "canvas": [CURSOR_PX, CURSOR_PX], "source": "glyph",
                                "attachments": {"hotspot": [TIP[0], TIP[1]]}})
    return path


def build():
    t = O._base_tokens()
    out = [O._badge_icon(n, t, g, rgb) for n, g, rgb in GEAR_ICONS]
    return out + [_cursor("pointer", False), _cursor("pointer_busy", True)]
