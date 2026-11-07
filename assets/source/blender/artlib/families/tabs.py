"""UIS-041 main tab bar icon tabs (ART spec §17). Our own designs: nothing
traced, reconstructed or modelled on another game's icons.

Drawn as the play-settings badges are (`overlay._badge_icon`: dark disc,
pale glyph with a dark halo, 128 px canvas, @2x of a 64 px icon), so the
tab bar and the lower-right toggles read as one family. The tab draws them
at about 24 logical px; the 128 canvas covers UI steps up to 4x and HiDPI.
All three share the chrome's text colour (`ui.text` in palettes/ui.yaml):
they name places, not states.

- ui_icon/tab_history: a quill laying down a line of writing: the
  colony's record being kept. A solid vane (a scroll or a page with lines
  blurred into a block at 24 px; a solid shape holds).
- ui_icon/tab_factions: two heater shields overlapping, the front one
  divided down the middle; more than one banner, so more than one people.
- ui_icon/tab_menu: three round-ended bars, the common sign for "more".

    blender -b --factory-startup -P assets/source/blender/build.py -- tabs
"""
import os

import numpy as np

from .. import contracts as C
from .. import palette
from . import overlay as O


def _ui_text():
    for line in open(os.path.join(C.PALETTE_DIR, "ui.yaml"), encoding="utf-8"):
        if line.startswith("ui.text:"):
            return palette.hex_to_srgb(line.split('"')[1][:7])
    raise ValueError("palettes/ui.yaml has no ui.text")


def _lens(xx, yy, cx, cy, a, b):
    """Signed distance of an upright lens (two arcs): half height a, half width b."""
    r = (a * a + b * b) / (2.0 * b)
    return np.maximum(np.hypot(xx - (cx - (r - b)), yy - cy), np.hypot(xx - (cx + (r - b)), yy - cy)) - r


def _g_history(xx, yy, c):  # a quill laying down a line of writing
    vx, vy = c + 6, c - 7
    rx, ry = O._rot(xx, yy, vx, vy, 45)
    vane = _lens(rx, ry, vx, vy, 21, 7.5)
    shaft = O._seg(xx, yy, vx - 8, vy + 8, c - 17, c + 16)
    line = O._seg(xx, yy, c - 6, c + 21, c + 20, c + 21)
    rachis = O._seg(rx, ry, vx, vy - 15, vx, vy + 17) - 1.2
    return [np.minimum(line, shaft), vane, (rachis, "cut")]


def _shield(cx, cy, hw, hh):
    """A heater shield's corners: flat top, straight sides, a point below."""
    return [(cx - hw, cy - hh), (cx + hw, cy - hh), (cx + hw, cy + hh * 0.15),
            (cx, cy + hh), (cx - hw, cy + hh * 0.15)]


def _g_factions(xx, yy, c):  # two overlapping shields, the front one divided
    back = _shield(c - 9, c - 7, 15, 19)
    front = _shield(c + 8, c + 5, 15, 19)
    front_d = O._poly(xx, yy, front)
    divide = O._seg(xx, yy, c + 8, c - 14, c + 8, c + 24)
    return [np.abs(O._poly(xx, yy, back)), (front_d - 4.5, "cut"),
            np.minimum(np.abs(front_d), divide)]


def _g_menu(xx, yy, c):  # three round-ended bars
    d = np.full(xx.shape, 1e9, np.float32)
    for dy in (-16, 0, 16):
        d = np.minimum(d, O._seg(xx, yy, c - 19, c + dy, c + 19, c + dy))
    return d - 1.5


def build():
    t = O._base_tokens()
    rgb = _ui_text()
    return [O._badge_icon("tab_history", t, _g_history, rgb),
            O._badge_icon("tab_factions", t, _g_factions, rgb),
            O._badge_icon("tab_menu", t, _g_menu, rgb)]
