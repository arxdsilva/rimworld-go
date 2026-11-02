"""MAP-11 multiple maps and the world view (ART spec §17-§18): the world
globe's marks for the colony and the temporary maps, and two colonist-bar
marks. Our own designs: nothing traced or reconstructed.

Globe marks are white silhouettes in the settlement emblems' style
(overlay._white_icon, no shield, dark contour, light from the upper-left);
the renderer multiplies them by the owner's accent (`tint: accent`):

- ui_icon/colony_home: the player's colony. A gabled house with a door and
  two windows, a pennant on a tall pole beside it, a low stone footing.
- ui_icon/site_camp: a temporary site map (a mine, an entered tile). A dome
  tent with a dark doorway and a swallowtail flag on a stake.
- ui_icon/encounter: an encounter map, an ambush on the road. Two crossed
  swords over a road running off to the horizon.
- ui_icon/site_ruined: a destroyed settlement. Two broken wall stubs with
  jagged tops, a fallen roof slab between them and rubble, in a dimmer grey.

Colonist-bar marks are badges (overlay._badge_icon, the status_* family):

- ui_icon/status_caravan: away with a caravan. A covered wagon rolling
  right with speed lines behind it.
- ui_icon/map_threat: the map chip's threat mark. A warning triangle with
  a "!" knocked out, in the danger red. (letter_threat is an envelope, a
  letter's kind, so it does not fit a map chip.)

    make art FAMILIES=worldmap
"""
import numpy as np

from . import overlay as O

PAL = (0.20, 0.20, 0.22)      # contour, as the settlement emblems
LIT = (0.96, 0.96, 0.94)      # faces towards the upper-left light
MID = (0.86, 0.86, 0.85)
SHADE = (0.74, 0.74, 0.75)    # faces turned away
DARK = (0.34, 0.34, 0.37)     # openings


def _shape(img, d, rgb, edge=2.8):
    """A filled piece with the emblem contour."""
    O._paint(img, O._cov(d - edge), PAL, 0.95)
    O._paint(img, O._cov(d), rgb)


def _colony_home(img, xx, yy):
    # flag pole and pennant, right of the house
    pole = O._rbox(xx, yy, 104, 60, 2.6, 40, 1.2)
    pen = O._poly(xx, yy, [(104, 22), (124, 30), (104, 39)])
    _shape(img, np.minimum(pole, pen), MID)
    O._paint(img, O._cov(pen), LIT)
    # stone footing
    _shape(img, O._rbox(xx, yy, 56, 102, 40, 6, 2.0), SHADE)
    # house body, the right half in shade
    body = O._rbox(xx, yy, 56, 80, 30, 20, 1.0)
    _shape(img, body, LIT)
    O._paint(img, O._cov(np.maximum(body, 66 - xx)), MID)
    # gabled roof: lit left slope, shaded right slope, eave line
    roof = O._poly(xx, yy, [(18, 64), (56, 28), (94, 64)])
    _shape(img, roof, LIT)
    O._paint(img, O._cov(np.maximum(roof, 56 - xx)), SHADE)
    O._paint(img, O._cov(O._seg(xx, yy, 20, 63, 92, 63) - 1.6), PAL)
    # door and windows
    O._paint(img, O._cov(O._rbox(xx, yy, 56, 90, 7, 10, 1.5)), DARK)
    for wx in (38, 75):
        O._paint(img, O._cov(O._rbox(xx, yy, wx, 78, 6, 6, 1.0)), DARK)


def _site_camp(img, xx, yy):
    # stake and swallowtail flag
    pole = O._rbox(xx, yy, 100, 70, 2.6, 30, 1.2)
    flag = np.maximum(O._rbox(xx, yy, 111, 46, 11, 9, 1.0),
                      -O._poly(xx, yy, [(123, 37), (114, 46), (123, 55)]))
    _shape(img, np.minimum(pole, flag), MID)
    O._paint(img, O._cov(flag), LIT)
    # dome tent: half an ellipse on the ground, the right side shaded
    dome = np.maximum(np.hypot((xx - 50) / 40.0, (yy - 98) / 46.0) * 40.0 - 40.0, yy - 98)
    _shape(img, dome, LIT)
    O._paint(img, O._cov(np.maximum(dome, 58 - xx + 0.25 * (98 - yy))), SHADE)
    # seam over the top and the doorway
    O._paint(img, O._cov(np.abs(np.hypot((xx - 50) / 22.0, (yy - 98) / 46.0) * 22.0 - 22.0) - 1.2
                         + 99 * (yy > 97)), PAL, 0.7)
    door = np.maximum(np.hypot((xx - 50) / 10.0, (yy - 98) / 22.0) * 10.0 - 10.0, yy - 98)
    O._paint(img, O._cov(door), DARK)
    # pegged ground line
    O._paint(img, O._cov(O._rbox(xx, yy, 54, 102, 48, 2.6, 1.2)), PAL)


def _sword(tip, guard, grip, gl):
    """Blade as a tapering wedge from the tip to the guard, a crossguard of
    half-length gl, a grip and a round pommel: (blade, hilt) fields."""
    (tx, ty), (gx, gy), (hx, hy) = tip, guard, grip
    ux, uy = gx - tx, gy - ty
    n = np.hypot(ux, uy)
    nx, ny = -uy / n, ux / n

    def d(xx, yy):
        blade = O._poly(xx, yy, [(tx, ty), (gx + nx * 8, gy + ny * 8), (gx - nx * 8, gy - ny * 8)])
        cross = O._cap(xx, yy, gx + nx * gl, gy + ny * gl, gx - nx * gl, gy - ny * gl, 4.2)
        hilt = O._cap(xx, yy, gx, gy, hx, hy, 4.0)
        pommel = np.hypot(xx - hx, yy - hy) - 6.5
        return blade, np.minimum(np.minimum(cross, hilt), pommel)
    return d


def _encounter(img, xx, yy):
    # the road running off to the horizon, a dashed centre line
    road = O._poly(xx, yy, [(50, 82), (78, 82), (114, 122), (14, 122)])
    _shape(img, road, MID)
    O._paint(img, O._cov(np.maximum(road, 64 - xx + 0.0 * yy)), SHADE)
    for y0, y1, w in ((86, 92, 1.6), (98, 106, 2.0), (112, 120, 2.4)):
        O._paint(img, O._cov(O._seg(xx, yy, 64, y0, 64, y1) - w), PAL, 0.85)
    # two crossed swords over it, the one from the upper left in front
    back = _sword((106, 8), (46, 66), (34, 78), 16)(xx, yy)
    front = _sword((22, 8), (82, 66), (94, 78), 16)(xx, yy)
    for blade, hilt in (back, front):
        _shape(img, np.minimum(blade, hilt), SHADE)
        O._paint(img, O._cov(blade), LIT)
        O._paint(img, O._cov(hilt), MID)


def _site_ruined(img, xx, yy):
    lit, shade = (0.84, 0.84, 0.83), (0.66, 0.66, 0.68)
    # left wall stub, jagged top, a broken window (convex pieces that overlap)
    left = np.minimum.reduce([O._rbox(xx, yy, 32, 79, 14, 23, 0.5),
                              O._poly(xx, yy, [(18, 60), (18, 48), (27, 40), (33, 52), (35, 60)]),
                              O._poly(xx, yy, [(33, 52), (42, 46), (46, 58), (46, 62), (33, 62)])])
    _shape(img, left, lit)
    O._paint(img, O._cov(np.maximum(left, 36 - xx)), shade)
    O._paint(img, O._cov(O._rbox(xx, yy, 29, 74, 5, 8, 1.0)), DARK)
    # right wall stub, lower
    right = np.minimum.reduce([O._rbox(xx, yy, 96, 89, 14, 13, 0.5),
                               O._poly(xx, yy, [(82, 78), (82, 70), (90, 62), (96, 72), (98, 78)]),
                               O._poly(xx, yy, [(96, 72), (106, 66), (110, 76), (110, 78), (96, 78)])])
    _shape(img, right, lit)
    O._paint(img, O._cov(np.maximum(right, 98 - xx)), shade)
    # fallen roof slab leaning from the right stub to the ground
    slab = O._poly(xx, yy, [(40, 100), (48, 88), (90, 70), (92, 80)])
    _shape(img, slab, shade)
    O._paint(img, O._cov(O._seg(xx, yy, 48, 90, 88, 74) - 1.2), PAL, 0.6)
    # rubble
    for cx, cy, hw, hh in ((60, 101, 8, 5), (74, 102, 6, 4), (12, 103, 5, 3.5), (118, 102, 5, 4)):
        _shape(img, O._rbox(xx, yy, cx, cy, hw, hh, 1.5), lit)
    # a wisp of smoke over the ruin
    for cx, cy, r in ((66, 62, 6), (73, 50, 7), (67, 37, 6)):
        O._paint(img, O._cov(np.hypot(xx - cx, yy - cy) - r), (0.72, 0.72, 0.74), 0.7)


def _g_status_caravan(xx, yy, c):
    """A covered wagon rolling right, speed lines behind it."""
    wx = c + 7
    bed = O._seg(xx, yy, wx - 19, c + 4, wx + 19, c + 4)
    cover = np.minimum(O._arc(xx, yy, wx, c + 1, 19, 180, 360),
                       np.minimum(O._seg(xx, yy, wx - 19, c + 1, wx - 19, c + 4),
                                  O._seg(xx, yy, wx + 19, c + 1, wx + 19, c + 4)))
    wheels = np.minimum(np.hypot(xx - (wx - 11), yy - (c + 19)) - 3.0,
                        np.hypot(xx - (wx + 11), yy - (c + 19)) - 3.0)
    lines = np.minimum(O._seg(xx, yy, c - 31, c - 10, c - 20, c - 10), O._seg(xx, yy, c - 33, c + 2, c - 20, c + 2))
    lines = np.minimum(lines, O._seg(xx, yy, c - 29, c + 14, c - 20, c + 14))
    return [lines + 1.0, wheels, np.minimum(bed, cover)]


def _g_map_threat(xx, yy, c):
    """A warning triangle with the "!" knocked out."""
    tri = O._poly(xx, yy, [(c, c - 26), (c + 26, c + 20), (c - 26, c + 20)])
    mark = np.minimum(O._cap(xx, yy, c, c - 11, c, c + 3, 3.4), np.hypot(xx - c, yy - (c + 11)) - 3.8)
    return [tri - 1.0, (mark, "cut")]


def build():
    t = O._base_tokens()
    out = [O._white_icon(name, painter, {"marker": marker}, shield=False) for name, painter, marker in (
        ("colony_home", _colony_home, "colony"), ("site_camp", _site_camp, "site"),
        ("encounter", _encounter, "encounter"), ("site_ruined", _site_ruined, "ruin"))]
    out += [O._badge_icon("status_caravan", t, _g_status_caravan, (0.94, 0.78, 0.48)),
            O._badge_icon("map_threat", t, _g_map_threat, t["danger"])]
    return out
