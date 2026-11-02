"""Recreation family (AX, A6 rows; ART spec §12-§14): the vanilla joy
buildings the Architect gained, and their Architect icons. Our own designs:
nothing traced or reconstructed.

Sprites land in the furniture output family and follow furniture.py and
decor.py: geometry authored facing SOUTH (the side a user approaches from,
the side a screen shows), turned with stage.face(); footprints come from the
construction Def rows (wiki sizes). The stuff-made pieces (ring toss, ur
board, chess, billiards and poker tables) are drawn in `wood`/`wood_plank`
and take the renderer's whole-sprite stuff colour shift like the tables;
the felts keep their own colour. No tint masks.

- furniture/ring_toss (1x1): a wooden peg on a round marked pitch, woven
  hoops stacked on the peg and lying where they fell.
- furniture/ur_board (1x1): a low carved box on a plinth, its lid inlaid
  with the twenty-square race track (rosette squares in blue), a few
  playing discs and pyramid dice.
- furniture/chess_table (1x1): a small square table with an inlaid 8x8
  board and a handful of pieces.
- furniture/billiards_table_<south|east> (2x3 / 3x2): a heavy wooden table,
  green felt, six pockets, a triangle of balls, the cue ball and a cue.
  North and west draw as south and east (the table is symmetric).
- furniture/poker_table (2x2): an octagonal table on a pedestal, wine felt,
  chip stacks at the four seats, cards and a dealer button.
- furniture/tube_television_<facing> (1x1): a boxy set on short legs with a
  deep back, side dials and an antenna.
- furniture/flatscreen_television_<facing> (2x1): a thin panel on a foot,
  tilted back a little.
- furniture/big_screen_television_<facing> (3x1): a wide panel over a low
  media console with speaker grilles.
- The televisions' screens and status lamps are dark in the sprite;
  <name>_glow_<facing> lights them (the screen pale blue with a soft
  picture) while the set is on. The screen faces the facing (the viewers'
  side); every facing keeps a lamp on top lit, and the flat panels' tilt
  shows a sliver of screen from above when turned east or west.
- furniture/telescope_<facing> (1x1): a brass tube on a tripod, the
  eyepiece toward the facing (the user's spot) and the objective raised
  away from it.

Icons ui_icon/<name> are cut from the south sprites, the televisions with
their glow composited (a lit screen reads as a screen).

    make art FAMILIES=recreation
"""
import math
import os
import tempfile

import numpy as np

from .. import contracts as C
from .. import stage as S
from . import overlay as O
from . import power as P
from .item import _spin, _tube

FACINGS = ("south", "east", "north", "west")
TWO = ("south", "east")


# ------------------------------------------------------------ game pieces
def _ring_toss():
    """A peg on a round pitch: a low wooden ring marks the throwing target,
    hoops of woven straw on and around the peg."""
    _tube("pitch", 0.5, -0.5, 0.43, 0.39, 0.0, 0.035, "wood", segs=32)
    for k in range(8):                  # stake pegs holding the pitch ring down
        a = (k + 0.5) / 8 * 2 * math.pi
        S.cyl("pin%d" % k, 0.5 + 0.41 * math.cos(a), -0.5 + 0.41 * math.sin(a), 0.022, 0.0, 0.05, "bark",
              segs=8, bevel=0.0)
    S.cyl("foot", 0.5, -0.5, 0.09, 0.0, 0.04, "bark", segs=16, bevel=0.01)
    S.cyl("peg", 0.5, -0.5, 0.045, 0.04, 0.46, "wood", segs=14, r_top=0.035, bevel=0.0)
    S.cyl("cap", 0.5, -0.5, 0.05, 0.46, 0.49, "bark", segs=14, bevel=0.008)
    for k, z in enumerate((0.04, 0.075)):           # two ringers on the peg
        _tube("on%d" % k, 0.5, -0.5, 0.125, 0.085, z, z + 0.035, "straw", segs=24)
    for k, (x, y) in enumerate(((0.26, -0.74), (0.74, -0.30), (0.70, -0.76))):
        _tube("off%d" % k, x, y, 0.11, 0.072, 0.0, 0.03, "straw", segs=22)
    return (1, 1), 0.49


UR_ROWS = 3
UR_COLS = 8
UR_ROSETTES = {(0, 0), (2, 0), (1, 3), (0, 6), (2, 6)}


def _ur_board():
    """A carved box on a plinth; its lid is the race track: two rows of six
    squares (a gap of two in each) either side of a full row of eight."""
    S.box("plinth", 0.16, -0.78, 0.0, 0.84, -0.30, 0.06, "bark", bevel=0.015)
    S.box("box", 0.08, -0.72, 0.06, 0.92, -0.32, 0.26, "wood", bevel=0.02)
    S.box("lid", 0.06, -0.74, 0.26, 0.94, -0.30, 0.30, "wood_plank", bevel=0.012)
    for k, z in enumerate((0.11, 0.20)):            # carved bands on the front
        S.box("band%d" % k, 0.12, -0.725, z, 0.88, -0.715, z + 0.02, "bark", bevel=0.0)
    cell, x0, y0 = 0.10, 0.10, -0.37
    for r in range(UR_ROWS):
        for c in range(UR_COLS):
            if r != 1 and c in (4, 5):
                continue
            cx, cy = x0 + cell * (c + 0.5), y0 - cell * (r + 0.5)
            S.box("sq%d_%d" % (r, c), cx - 0.043, cy - 0.043, 0.30, cx + 0.043, cy + 0.043, 0.312,
                  "wool_cream", bevel=0.004)
            if (r, c) in UR_ROSETTES:
                S.cyl("rose%d_%d" % (r, c), cx, cy, 0.03, 0.311, 0.318, "med_blue", segs=8, bevel=0.0)
            else:
                S.cyl("dot%d_%d" % (r, c), cx, cy, 0.012, 0.311, 0.316, "bark", segs=8, bevel=0.0)
    for k, (r, c, m) in enumerate(((1, 1, "backdrop"), (0, 2, "ceramic"), (2, 7, "backdrop"), (1, 6, "ceramic"))):
        cx, cy = x0 + cell * (c + 0.5), y0 - cell * (r + 0.5)
        S.cyl("disc%d" % k, cx, cy, 0.034, 0.312, 0.335, m, segs=14, bevel=0.006)
    for k, (x, y) in enumerate(((0.32, -0.82), (0.44, -0.86), (0.58, -0.83))):   # pyramid dice by the box
        S.cyl("die%d" % k, x, y, 0.035, 0.0, 0.05, "ceramic", segs=3, r_top=0.004, bevel=0.0)
    return (1, 1), 0.34


def _legs(x0, x1, y0, y1, h, r=0.045, mat="wood"):
    for k, (x, y) in enumerate(((x0, y0), (x1, y0), (x0, y1), (x1, y1))):
        S.box("leg%d" % k, x - r, y - r, 0.0, x + r, y + r, h, mat, bevel=0.012)


def _chess_table():
    """A small square table, an 8x8 board inlaid in its top and a few
    pieces in play."""
    _legs(0.17, 0.83, -0.17, -0.83, 0.46)
    S.box("apron", 0.14, -0.86, 0.38, 0.86, -0.14, 0.46, "wood", bevel=0.015)
    S.box("top", 0.08, -0.92, 0.46, 0.92, -0.08, 0.52, "wood", bevel=0.02)
    S.box("frame", 0.18, -0.82, 0.519, 0.82, -0.18, 0.524, "bark", bevel=0.0)
    sq, b0 = 0.075, 0.20
    for r in range(8):
        for c in range(8):
            if (r + c) % 2:
                continue
            x, y = b0 + c * sq, -b0 - r * sq
            S.box("sq%d_%d" % (r, c), x, y - sq, 0.522, x + sq, y, 0.527, "book_ochre", bevel=0.0)
    pieces = ((1, 2, "ceramic", 0.07), (2, 4, "ceramic", 0.09), (1, 6, "ceramic", 0.07), (5, 3, "backdrop", 0.07),
              (6, 5, "backdrop", 0.10), (6, 1, "backdrop", 0.07), (4, 4, "ceramic", 0.07))
    for k, (r, c, m, h) in enumerate(pieces):
        cx, cy = b0 + (c + 0.5) * sq, -b0 - (r + 0.5) * sq
        S.cyl("base%d" % k, cx, cy, 0.032, 0.524, 0.545, m, segs=12, bevel=0.005)
        S.cyl("body%d" % k, cx, cy, 0.022, 0.545, 0.545 + h, m, segs=10, r_top=0.015, bevel=0.0)
        S.ellipsoid("head%d" % k, cx, cy, 0.545 + h + 0.016, 0.027, 0.027, 0.024, m, segs=10, rings=6)
    for k, (x, y, m) in enumerate(((0.12, -0.12, "ceramic"), (0.88, -0.88, "backdrop"))):   # taken pieces
        S.cyl("taken%d" % k, x, y, 0.025, 0.52, 0.56, m, segs=10, bevel=0.004)
    return (1, 1), 0.68


BALLS = ("corn_yellow", "med_blue", "food_red", "jacket_navy", "backdrop", "pumpkin", "felt_wine", "foam_paint",
         "book_ochre", "corn_yellow")


def _billiards_table():
    """2x3, long side north-south: six legs under a deep apron, wooden
    rails with sight dots, green felt and six pockets; a triangle of balls
    toward the north end, the cue ball and a cue toward the south."""
    w, h = 2.0, 3.0
    for k, (x, y) in enumerate(((0.22, -0.22), (1.78, -0.22), (0.22, -1.5), (1.78, -1.5), (0.22, -2.78),
                                (1.78, -2.78))):
        S.box("leg%d" % k, x - 0.08, y - 0.08, 0.0, x + 0.08, y + 0.08, 0.44, "wood", bevel=0.02)
        S.box("foot%d" % k, x - 0.10, y - 0.10, 0.0, x + 0.10, y + 0.10, 0.06, "bark", bevel=0.012)
    S.box("apron", 0.10, -(h - 0.10), 0.40, w - 0.10, -0.10, 0.62, "wood", bevel=0.03)
    S.box("rail", 0.04, -(h - 0.04), 0.60, w - 0.04, -0.04, 0.70, "wood", bevel=0.03)
    S.box("felt", 0.20, -(h - 0.20), 0.66, w - 0.20, -0.20, 0.705, "felt", bevel=0.0)
    for k, (x0, y0, x1, y1) in enumerate(((0.20, -(h - 0.20), 0.25, -0.20), (w - 0.25, -(h - 0.20), w - 0.20, -0.20),
                                          (0.20, -0.25, w - 0.20, -0.20), (0.20, -(h - 0.20), w - 0.20, -(h - 0.25)))):
        S.box("cushion%d" % k, x0, y0, 0.70, x1, y1, 0.725, "felt_dark", bevel=0.006)
    for k, (x, y) in enumerate(((0.22, -0.22), (w - 0.22, -0.22), (0.20, -h / 2), (w - 0.20, -h / 2),
                                (0.22, -(h - 0.22)), (w - 0.22, -(h - 0.22)))):
        S.cyl("pocket%d" % k, x, y, 0.075, 0.70, 0.728, "backdrop", segs=16, bevel=0.0)
    for k in range(1, 4):               # sight dots on the long rails and the end rails
        for side, x in enumerate((0.10, w - 0.10)):
            for j, y in enumerate((-h / 2 + k * 0.32, -h / 2 - k * 0.32)):
                S.cyl("sight%d_%d_%d" % (k, side, j), x, y, 0.014, 0.70, 0.706, "brass", segs=8, bevel=0.0)
    r = 0.05
    i = 0
    for row in range(4):                # the triangle, apex toward the cue ball
        for c in range(row + 1):
            x = w / 2 + (c - row / 2) * 2.05 * r
            y = -0.85 + row * 1.8 * r
            S.ellipsoid("ball%d" % i, x, y, 0.705 + r, r, r, r, BALLS[i % len(BALLS)], segs=14, rings=8)
            i += 1
    S.ellipsoid("cue_ball", w / 2 - 0.10, -2.25, 0.705 + r, r, r, r, "ceramic", segs=14, rings=8)
    P._rod("cue", (0.46, -2.62, 0.735), (1.46, -1.40, 0.735), 0.018, "wood_plank", segs=10, bevel=0.0)
    P._rod("butt", (0.46, -2.62, 0.735), (0.70, -2.33, 0.735), 0.024, "bark", segs=10, bevel=0.0)
    P._rod("tip", (1.44, -1.425, 0.735), (1.47, -1.39, 0.735), 0.016, "ceramic", segs=10, bevel=0.0)
    return (2, 3), 0.82


CHIPS = ("food_red", "ceramic", "med_blue", "backdrop")


def _poker_table():
    """2x2: an octagon of wood round wine felt on a turned pedestal; chip
    stacks at the four seats, the pot and a dealt hand in the middle."""
    S.cyl("base", 1.0, -1.0, 0.46, 0.0, 0.06, "bark", segs=8, bevel=0.015)
    S.cyl("column", 1.0, -1.0, 0.16, 0.06, 0.50, "wood", segs=16, r_top=0.12, bevel=0.0)
    n0 = len(S.objects())
    S.cyl("rim", 1.0, -1.0, 0.96, 0.50, 0.62, "wood", segs=8, bevel=0.03)
    S.cyl("felt", 1.0, -1.0, 0.80, 0.62, 0.635, "felt_wine", segs=8, bevel=0.0)
    S.cyl("rail", 1.0, -1.0, 0.70, 0.632, 0.638, "felt_dark", segs=8, bevel=0.0)
    S.cyl("felt_in", 1.0, -1.0, 0.68, 0.636, 0.640, "felt_wine", segs=8, bevel=0.0)
    _spin(n0, 1.0, -1.0, 22.5)           # flat sides to the four seats
    for k, (x, y) in enumerate(((1.0, -1.66), (1.66, -1.0), (1.0, -0.34), (0.34, -1.0))):
        for j in range(2):              # two stacks per seat
            dx, dy = (0.09 * (j * 2 - 1), 0.0) if k % 2 == 0 else (0.0, 0.09 * (j * 2 - 1))
            n = 3 + (k + j) % 3
            for s in range(n):
                z = 0.64 + s * 0.02
                S.cyl("chip%d_%d_%d" % (k, j, s), x + dx, y + dy, 0.055, z, z + 0.018, CHIPS[(k + j + s) % 4],
                      segs=14, bevel=0.0)
    for s in range(4):                  # the pot
        z = 0.64 + s * 0.02
        S.cyl("pot%d" % s, 1.12, -0.86, 0.055, z, z + 0.018, CHIPS[s % 4], segs=14, bevel=0.0)
    for k, (x, y, yaw, m) in enumerate(((0.80, -1.04, 12, "ceramic"), (0.90, -1.07, -8, "ceramic"),
                                        (1.00, -1.04, 4, "ceramic"), (1.20, -1.22, 30, "book_blue"),
                                        (1.23, -1.27, 18, "book_blue"))):
        n0 = len(S.objects())
        S.box("card%d" % k, x - 0.058, y - 0.085, 0.641 + k * 0.002, x + 0.058, y + 0.085, 0.646 + k * 0.002, m,
              bevel=0.003)
        if m == "ceramic":
            S.box("pip%d" % k, x - 0.018, y - 0.018, 0.646 + k * 0.002, x + 0.018, y + 0.018, 0.648 + k * 0.002,
                  "food_red" if k % 2 else "backdrop", bevel=0.0)
        _spin(n0, x, y, yaw)
    S.cyl("button", 0.80, -1.30, 0.035, 0.64, 0.655, "wool_cream", segs=14, bevel=0.004)
    return (2, 2), 0.74


# ------------------------------------------------------------- screens
def _screen(name, x0, x1, y, z0, z1, picture=True):
    """A screen on the south face at y (dark glass in the sprite, lit in the
    glow layer), with a soft picture: rolling ground and a pale sun."""
    P.glow(S.box(name, x0, y - 0.008, z0, x1, y, z1, "backdrop", bevel=0.004), "tv_screen_glow")
    if not picture:
        return
    w, h = x1 - x0, z1 - z0
    P.glow(S.box(name + "_ground", x0 + 0.03 * w, y - 0.011, z0 + 0.04 * h, x1 - 0.03 * w, y - 0.007,
                 z0 + 0.40 * h, "backdrop", bevel=0.0), "tv_picture_glow")
    P.glow(S.box(name + "_hill", x0 + 0.45 * w, y - 0.013, z0 + 0.38 * h, x1 - 0.12 * w, y - 0.009,
                 z0 + 0.52 * h, "backdrop", bevel=0.0), "tv_picture_glow")
    r = 0.07 * min(w, 2.4 * h)
    P.glow(S.ellipsoid(name + "_sun", x0 + 0.24 * w, y - 0.010, z0 + 0.72 * h, r, 0.003, r, "backdrop", segs=16,
                       rings=6), "lamp_warm")


def _lamp(name, x, y, z):
    """A status lamp on a top face: lit in every facing."""
    P.glow(S.cyl(name, x, y, 0.024, z, z + 0.012, "backdrop", segs=10, bevel=0.0), "indicator")


def _tube_television():
    """A boxy set on four short legs: a deep back, the screen in a wide
    bezel on the front, two dials and a speaker grille beside it, a lamp
    and a two-rod antenna on top."""
    for k, (x, y) in enumerate(((0.22, -0.78), (0.78, -0.78), (0.24, -0.30), (0.76, -0.30))):
        P._rod("leg%d" % k, (x, y, 0.0), (0.5 + (x - 0.5) * 0.8, -0.54 + (y + 0.54) * 0.8, 0.20), 0.022,
               "metal_dark", segs=8, bevel=0.0)
    S.box("back", 0.24, -0.50, 0.26, 0.76, -0.22, 0.66, "steel", bevel=0.05, segments=2)
    S.box("body", 0.14, -0.82, 0.18, 0.86, -0.42, 0.76, "steel", bevel=0.03)
    S.box("bezel", 0.17, -0.835, 0.23, 0.64, -0.80, 0.71, "metal_dark", bevel=0.02)
    _screen("screen", 0.21, 0.60, -0.835, 0.28, 0.66)
    S.box("panel", 0.67, -0.835, 0.23, 0.83, -0.80, 0.71, "steel_dark", bevel=0.01)
    for k, z in enumerate((0.60, 0.48)):
        P._rod("dial%d" % k, (0.75, -0.835, z), (0.75, -0.875, z), 0.04, "metal_dark", segs=14, bevel=0.0)
        S.box("mark%d" % k, 0.745, -0.878, z + 0.01, 0.755, -0.874, z + 0.035, "wool_cream", bevel=0.0)
    for k in range(4):
        z = 0.27 + k * 0.035
        S.box("grille%d" % k, 0.69, -0.84, z, 0.81, -0.835, z + 0.015, "backdrop", bevel=0.0)
    _lamp("lamp", 0.78, -0.74, 0.76)
    S.cyl("ant_base", 0.46, -0.48, 0.06, 0.76, 0.80, "metal_dark", segs=14, bevel=0.008)
    for k, (dx, dy) in enumerate(((-0.24, 0.06), (0.22, 0.10))):
        tip = (0.46 + dx, -0.48 + dy, 1.12)
        P._rod("ant%d" % k, (0.46, -0.48, 0.79), tip, 0.008, "steel", segs=6, bevel=0.0)
        S.ellipsoid("ant_tip%d" % k, tip[0], tip[1], tip[2], 0.016, 0.016, 0.016, "steel", segs=8, rings=5)
    return (1, 1), 1.14


def _panel(w, x0, x1, ypan, z0, z1, tilt, pivot_z):
    """A thin panel set (bezel + screen) standing at y=ypan, tipped back by
    tilt degrees about its foot; a status lamp on its top edge."""
    n0 = len(S.objects())
    S.box("panel", x0, ypan - 0.03, z0, x1, ypan + 0.03, z1, "metal_dark", bevel=0.012)
    S.box("panel_back", x0 + 0.12, ypan + 0.02, z0 + 0.10, x1 - 0.12, ypan + 0.07, z1 - 0.10, "steel_dark", bevel=0.02)
    _screen("screen", x0 + 0.04, x1 - 0.04, ypan - 0.03, z0 + 0.04, z1 - 0.07)
    _lamp("lamp", (x0 + x1) / 2, ypan, z1)
    P._spin(P._since(n0), (w / 2, ypan, pivot_z), "X", -tilt)


def _flatscreen_television():
    """2x1: a thin wide panel on a round foot and neck, tipped back."""
    S.cyl("foot", 1.0, -0.52, 0.26, 0.0, 0.035, "metal_dark", segs=24, bevel=0.01)
    S.box("neck", 0.94, -0.54, 0.03, 1.06, -0.46, 0.40, "steel_dark", bevel=0.012)
    _panel(2.0, 0.06, 1.94, -0.52, 0.30, 1.10, 12, 0.36)
    return (2, 1), 1.14


def _big_screen_television():
    """3x1: a wide panel on two posts over a low media console with
    speaker grilles at either end and a lamp on its top."""
    S.box("console", 0.08, -0.86, 0.0, 2.92, -0.30, 0.34, "metal_dark", bevel=0.025)
    S.box("console_top", 0.06, -0.88, 0.32, 2.94, -0.28, 0.37, "steel_dark", bevel=0.015)
    for k, x in enumerate((0.16, 2.44)):            # speaker grilles
        S.box("spk%d" % k, x, -0.875, 0.05, x + 0.40, -0.86, 0.29, "backdrop", bevel=0.008)
        for j in range(4):
            z = 0.08 + j * 0.05
            S.box("spk%d_%d" % (k, j), x + 0.03, -0.88, z, x + 0.37, -0.874, z + 0.016, "metal_dark", bevel=0.0)
    for k, x in enumerate((0.70, 1.50, 2.30)):      # drawer fronts
        S.box("drawer%d" % k, x - 0.13 if k == 1 else x - 0.06, -0.875, 0.08, x + 0.13 if k == 1 else x + 0.06,
              -0.865, 0.27, "steel_dark", bevel=0.006)
    _lamp("console_lamp", 2.70, -0.80, 0.37)
    for k, x in enumerate((0.9, 2.1)):
        S.box("post%d" % k, x - 0.05, -0.50, 0.37, x + 0.05, -0.42, 0.52, "steel_dark", bevel=0.01)
    _panel(3.0, 0.10, 2.90, -0.46, 0.46, 1.62, 9, 0.50)
    return (3, 1), 1.66


# --------------------------------------------------------------- scope
def _telescope():
    """A brass tube on a dark tripod: eyepiece low toward the user's spot
    (south), the objective raised toward the north sky; a finder scope
    and a counterweight arm."""
    apex = (0.5, -0.50, 0.58)
    for k, foot in enumerate(((0.18, -0.76, 0.0), (0.82, -0.76, 0.0), (0.5, -0.14, 0.0))):
        P._rod("leg%d" % k, foot, apex, 0.022, "steel_dark", segs=8, bevel=0.0)
        S.cyl("shoe%d" % k, foot[0], foot[1], 0.035, 0.0, 0.025, "metal_dark", segs=10, bevel=0.0)
    S.cyl("tray", 0.5, -0.50, 0.12, 0.26, 0.29, "metal_dark", segs=3, bevel=0.0)
    S.cyl("mount", 0.5, -0.50, 0.07, 0.56, 0.68, "metal_dark", segs=14, bevel=0.01)
    lo, hi = (0.5, -0.80, 0.66), (0.5, -0.12, 1.06)
    P._rod("tube", lo, hi, 0.075, "brass", segs=18, bevel=0.0)
    for k, t in enumerate((0.18, 0.55, 0.92)):
        p0 = tuple(lo[i] + (hi[i] - lo[i]) * (t - 0.02) for i in range(3))
        p1 = tuple(lo[i] + (hi[i] - lo[i]) * (t + 0.02) for i in range(3))
        P._rod("band%d" % k, p0, p1, 0.082, "metal_dark", segs=18, bevel=0.0)
    hood = tuple(hi[i] + (hi[i] - lo[i]) * 0.10 for i in range(3))
    P._rod("hood", hi, hood, 0.095, "metal_dark", segs=18, bevel=0.0)
    P._rod("lens", hood, tuple(hood[i] + (hi[i] - lo[i]) * 0.005 for i in range(3)), 0.08, "glass", segs=18,
           bevel=0.0)
    eye = (0.5, -0.86, 0.60)
    P._rod("eyepiece", lo, eye, 0.03, "metal_dark", segs=10, bevel=0.0)
    P._rod("finder", (0.58, -0.62, 0.82), (0.58, -0.26, 1.02), 0.022, "steel", segs=10, bevel=0.0)
    P._rod("arm", (0.5, -0.50, 0.66), (0.30, -0.56, 0.52), 0.016, "steel", segs=8, bevel=0.0)
    S.cyl("weight", 0.28, -0.565, 0.05, 0.44, 0.54, "metal_dark", segs=14, bevel=0.01)
    return (1, 1), 1.22


# ------------------------------------------------------------------ build
# name: (builder, extra, facings); builder returns (footprint, zmax)
PIECES = {
    "ring_toss": (_ring_toss, {"building": "ring toss"}, None),
    "ur_board": (_ur_board, {"building": "ur board"}, None),
    "chess_table": (_chess_table, {"building": "chess table"}, None),
    "billiards_table": (_billiards_table, {"building": "billiards table"}, TWO),
    "poker_table": (_poker_table, {"building": "poker table"}, None),
    "tube_television": (_tube_television, {"building": "tube television"}, FACINGS),
    "flatscreen_television": (_flatscreen_television, {"building": "flatscreen television"}, FACINGS),
    "big_screen_television": (_big_screen_television, {"building": "big screen television"}, FACINGS),
    "telescope": (_telescope, {"building": "telescope"}, FACINGS),
}

# Architect icons: ui_icon/<name> from (sprite, composite its glow).
ICONS = {
    "ring_toss": ("ring_toss", False),
    "ur_board": ("ur_board", False),
    "chess_table": ("chess_table", False),
    "billiards_table": ("billiards_table_east", False),
    "poker_table": ("poker_table", False),
    "tube_television": ("tube_television_south", True),
    "flatscreen_television": ("flatscreen_television_south", True),
    "big_screen_television": ("big_screen_television_south", True),
    "telescope": ("telescope_east", False),
}


def lit_icon(name, family, sprite):
    """ui_icon/<name> cut from family/sprite with its glow layer laid over
    (alpha over, same canvas and origin)."""
    src, glw = S.out_path(family, sprite), S.out_path(family, P.glow_name(sprite))
    if not (os.path.exists(src) and os.path.exists(glw)):
        return None
    img, g = S._read_png(src), S._read_png(glw)
    ga = g[..., 3:4] * (g[..., 3:4] > 0.35)        # the halo stays off the icon
    img[..., :3] = img[..., :3] * (1 - ga) + g[..., :3] * ga
    tmp = os.path.join(tempfile.gettempdir(), "rwg_icon_%s_%d.png" % (name, os.getpid()))
    S.write_png(img, tmp)
    return O._cut_icon(name, tmp, "%s:%s/%s" % (C.NAMESPACE, family, sprite))


def _icons():
    out = []
    for name, (spr, lit) in ICONS.items():
        p = lit_icon(name, "furniture", spr) if lit else O._icon(name, "furniture", spr)
        if p:
            out.append(p)
    return out


def _pieces(only=None):
    out = []
    for name, (fn, extra, facings) in PIECES.items():
        if only and name not in only:
            continue
        for f in facings or (None,):
            out += P.render(name + ("_" + f if f else ""), fn, facing=f, extra=dict(extra), family="furniture")
    return out


def build(only=None):
    """Every recreation sprite and icon; `only` (PIECES names) renders just
    those pieces."""
    if only:
        return _pieces(only)
    return _pieces() + _icons()
