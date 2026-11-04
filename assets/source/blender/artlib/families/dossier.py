"""DOS-0A colonist card icons (RHUD; ART spec §17). Our own designs: nothing
traced or reconstructed.

- ui_icon/dossier_gear: the card's settings button, a solid six-toothed cog
  with a knocked-out hub. Heavier than the crafting skill's spoked wheel, so
  it holds at the small size it is drawn over the card.
- ui_icon/dossier_toggle: the play-settings toggle (docked or floating card),
  a person's card lifted out of the tray it docks in.

    blender -b --factory-startup -P assets/source/blender/build.py -- dossier
"""
import numpy as np

from . import overlay as O

_GEAR_RGB = (0.84, 0.82, 0.76)     # pale, like the chrome's text: a tool, not a state
_TOGGLE_RGB = (0.90, 0.74, 0.40)   # warm like the card's own accent


def _g_gear(xx, yy, c):  # a solid cog, six square-shouldered teeth, open hub
    d = np.hypot(xx - c, yy - c) - 19
    for deg in range(0, 180, 60):
        rx, ry = O._rot(xx, yy, c, c, deg)
        d = np.minimum(d, O._rbox(rx, ry, c, c, 6.5, 28, 1.5))
    return [d, (np.hypot(xx - c, yy - c) - 11, "cut")]


def _g_toggle(xx, yy, c):  # a person's card lifted out of its docking tray
    card = O._rbox(xx, yy, c, c - 9, 21, 16, 2.5)
    head = np.hypot(xx - (c - 10), yy - (c - 15)) - 5
    shoulders = O._rbox(xx, yy, c - 10, c - 4, 8, 4, 3)
    rows = np.minimum(O._rbox(xx, yy, c + 9, c - 14, 8, 2.5, 1), O._rbox(xx, yy, c + 9, c - 5, 8, 2.5, 1))
    tray = np.minimum(np.minimum(O._seg(xx, yy, c - 27, c + 15, c - 27, c + 25),
                                 O._seg(xx, yy, c + 27, c + 15, c + 27, c + 25)),
                      O._seg(xx, yy, c - 27, c + 25, c + 27, c + 25))
    return [tray, card, (np.minimum(np.minimum(head, shoulders), rows), "cut")]


def build():
    t = O._base_tokens()
    return [O._badge_icon("dossier_gear", t, _g_gear, _GEAR_RGB),
            O._badge_icon("dossier_toggle", t, _g_toggle, _TOGGLE_RGB)]
