"""AX-A4 power and industry (AX-AR2; ART spec §7, §12-§14): the waterproof
conduit, the chemfuel generator, the watermill, the deep drill, the
ground-penetrating scanner, the moisture pump and the electric crematorium,
the deep-ore marker, and their Architect icons. Our own designs: nothing
traced or reconstructed. Sizes are the wiki footprints (the construction
Defs).

Sprites follow power.py (@2x; authored facing south, turned with
stage.face(); emissive parts get a `<name>_glow` layer drawn while the
device is powered or working; attachments in canvas pixels).

- waterproof_conduit_<NN> (power): the linked 16-mask conduit (N=1, E=2,
  S=4, W=8) in the plain conduit's recipe, its pipe in a sealed blue-grey
  jacket with brass collars, standing on short stilts so it reads as laid
  over water. `autotile: conduit` like the plain one.
- chemfuel_generator (power, 2x2): a rust-orange fuel tank with a filler
  and a sight gauge, an engine block with an exhaust stack (`smoke`
  attachment), the alternator and a control box. Glow: the run lamp and the
  combustion window.
- watermill_<facing> (power, 5x6): a timber deck over the bank (5x5) with
  the generator shed, its gear train and a cable box, and two bearing posts
  carrying the wheel's axle over the water row (the sixth row, south when
  facing south). Glow: the shed's lamps. The wheel is its own layer,
  watermill_wheel_<facing>_01..06: eight paddles on a drum, turned 7.5
  degrees per frame (six frames make one paddle pitch, so they loop). A
  frame has a tight canvas over the wheel's row: draw it at the base's
  top-left tile plus its `offset_tiles`, by its own origin_px, after the
  base. The base's geometry is held out of it, so the deck hides the wheel
  where it should. Advance frames with the flow.
- deep_drill_<facing> (power, 1x2): a lattice derrick over the north tile
  with the drill string down into a collared casing, the motor and a
  control console on the south tile, facing the operator. Glow: the console
  screen and the derrick lamp.
- ground_scanner_<facing> (power, 3x3): a ring of six transducer pods
  round a central emitter column on a heavy round bed, a cable trunk to the
  operator's console on the middle of the south row and a sensor mast.
  Glow: the console's scan screen, the pod lights and the mast lamp.
- moisture_pump (power, 1x1): a drum tank on a skid, its intake pipe into
  the ground, the motor and fan on top, an outlet spout and a lamp (glow).
- electric_crematorium_<facing> (furniture, 3x2; a bench): a masonry
  furnace (the `stone_tint` group, so the renderer tints it with the
  stone), a steel chamber door in the middle of the south face (the
  worker's spot), a control panel west, an ash drawer east and a chimney at
  the back. Glow: the door's spy window and the panel lamp.
- deep_ore (overlay, 1x1 flat): the deep-ore marker, a dashed ring round
  three faceted nuggets, near-white with a dark rim so the overlay can
  multiply it by the ore's colour (metadata `ore_colour: true`).

    make art FAMILIES=industry
"""
import math
import os
import tempfile

import numpy as np

from .. import contracts as C
from .. import meta
from .. import stage as S
from . import overlay as O
from . import power as P
from .linked import BITS

FACINGS = P.FACINGS
EXT = P.EXT


# ------------------------------------------------------- waterproof conduit
WC_H = 0.10       # pipe top
WC_Z0 = 0.04      # pipe bottom: it stands on stilts


def _waterproof_conduit(mask):
    """The plain conduit's arms in a sealed jacket, raised on stilts, a
    brass collar on every arm and a sealed junction box on bends, tees,
    crosses and ends."""
    con = {d for d, b in BITS if mask & b}
    cx, cy, r = 0.5, -0.5, 0.055
    pipe = "conduit_jacket"
    if con in ({"N", "S"}, {"E", "W"}):
        if con == {"N", "S"}:
            S.box("pipe", cx - r, -1 - EXT, WC_Z0, cx + r, EXT, WC_H, pipe, bevel=0.035)
        else:
            S.box("pipe", -EXT, cy - r, WC_Z0, 1 + EXT, cy + r, WC_H, pipe, bevel=0.035)
    else:
        arms = {"N": (cx - r, cy, cx + r, EXT), "S": (cx - r, -1 - EXT, cx + r, cy),
                "E": (cx, cy - r, 1 + EXT, cy + r), "W": (-EXT, cy - r, cx, cy + r)}
        for d in sorted(con):
            S.box("pipe_" + d, *arms[d][:2], WC_Z0, *arms[d][2:], WC_H, pipe, bevel=0.035)
    # stilts and collars: one pair per linked arm, at a quarter tile from the centre
    spots = {"N": (cx, cy + 0.3), "S": (cx, cy - 0.3), "E": (cx + 0.3, cy), "W": (cx - 0.3, cy)}
    for d in sorted(con):
        x, y = spots[d]
        S.cyl("stilt_" + d, x, y, 0.025, 0.0, WC_Z0 + 0.01, "metal_dark", segs=8, bevel=0.0)
        S.cyl("foot_" + d, x, y, 0.045, 0.0, 0.012, "metal_dark", segs=10, bevel=0.0)
        if d in ("N", "S"):
            S.box("collar_" + d, x - 0.075, y - 0.025, WC_Z0 - 0.01, x + 0.075, y + 0.025, WC_H + 0.015, "brass",
                  bevel=0.01)
        else:
            S.box("collar_" + d, x - 0.025, y - 0.075, WC_Z0 - 0.01, x + 0.025, y + 0.075, WC_H + 0.015, "brass",
                  bevel=0.01)
    if len(con) != 2 or con not in ({"N", "S"}, {"E", "W"}):
        if not con:
            S.box("stub", cx - 0.26, cy - r, WC_Z0, cx + 0.26, cy + r, WC_H, pipe, bevel=0.035)
            for x in (cx - 0.2, cx + 0.2):
                S.cyl("stilt", x, cy, 0.025, 0.0, WC_Z0 + 0.01, "metal_dark", segs=8, bevel=0.0)
        S.cyl("box", cx, cy, 0.13, 0.0, 0.15, "conduit_jacket", segs=16, bevel=0.03)
        S.cyl("seal", cx, cy, 0.135, 0.10, 0.125, "brass", segs=16, bevel=0.0)
        S.cyl("cap", cx, cy, 0.09, 0.15, 0.17, "steel", segs=14, bevel=0.012)
    return con


# ------------------------------------------------------- chemfuel generator
CHEM_STACK = (1.52, -0.30, 1.42)


def _chemfuel_generator():
    """2x2: fuel tank (west, lying north-south) with filler and sight gauge;
    engine block with a combustion window and exhaust stack (north-east);
    the alternator (south-east) and a control box at the front."""
    S.box("skid", 0.06, -1.94, 0.0, 1.94, -0.06, 0.08, "steel_dark", bevel=0.02)
    for x, y in ((0.12, -0.12), (1.88, -0.12), (0.12, -1.88), (1.88, -1.88)):
        S.cyl("skid_bolt", x, y, 0.025, 0.08, 0.10, "metal_dark", segs=6, bevel=0.0)
    # the tank on two saddles
    for y in (-0.42, -1.50):
        S.box("saddle", 0.12, y - 0.07, 0.08, 0.84, y + 0.07, 0.30, "metal_dark", bevel=0.01)
    P._rod("tank", (0.48, -1.78, 0.50), (0.48, -0.18, 0.50), 0.34, "fuel_drum", segs=26, bevel=0.04)
    for y in (-0.70, -1.26):
        P._rod("hoop", (0.48, y - 0.03, 0.50), (0.48, y + 0.03, 0.50), 0.348, "metal_dark", segs=26, bevel=0.0)
    S.cyl("filler", 0.48, -0.40, 0.08, 0.80, 0.90, "brass", segs=12, bevel=0.01)
    S.cyl("filler_cap", 0.48, -0.40, 0.095, 0.90, 0.94, "metal_dark", segs=12, bevel=0.008)
    S.box("gauge", 0.38, -1.80, 0.36, 0.58, -1.77, 0.66, "glass", bevel=0.006)
    S.box("gauge_fill", 0.42, -1.81, 0.38, 0.54, -1.79, 0.54, "fuel_drum", bevel=0.0)
    P._rod("fuel_line", (0.80, -0.70, 0.30), (1.10, -0.70, 0.30), 0.03, "copper", segs=8)
    # engine block (north-east)
    S.box("engine", 1.00, -1.06, 0.08, 1.86, -0.16, 0.76, "steel_dark", bevel=0.04)
    S.box("head", 1.06, -1.00, 0.76, 1.80, -0.22, 0.90, "metal_dark", bevel=0.03)
    for k in range(4):
        x = 1.14 + 0.17 * k
        S.box("fin", x, -0.96, 0.90, x + 0.08, -0.26, 0.98, "steel", bevel=0.01)
    S.box("window_frame", 1.12, -1.075, 0.26, 1.50, -1.055, 0.56, "metal_dark", bevel=0.008)
    P.glow(S.box("window", 1.16, -1.085, 0.30, 1.46, -1.07, 0.52, "ash_char", bevel=0.004), "gland_ember")
    S.cyl("stack", CHEM_STACK[0], CHEM_STACK[1], 0.075, 0.90, CHEM_STACK[2] - 0.08, "metal_dark", segs=14)
    S.cyl("stack_cap", CHEM_STACK[0], CHEM_STACK[1], 0.11, CHEM_STACK[2] - 0.10, CHEM_STACK[2], "steel_dark",
          segs=14, bevel=0.01)
    # alternator (south-east), axis east-west, and the control box in front
    P._rod("alternator", (1.06, -1.48, 0.36), (1.86, -1.48, 0.36), 0.26, "steel", segs=22, bevel=0.03)
    P._rod("winding", (1.36, -1.48, 0.36), (1.56, -1.48, 0.36), 0.272, "copper", segs=22, bevel=0.0)
    for x in (1.14, 1.78):
        P._rod("end_cap", (x - 0.03, -1.48, 0.36), (x + 0.03, -1.48, 0.36), 0.275, "metal_dark", segs=22, bevel=0.0)
    S.box("control", 1.10, -1.92, 0.08, 1.70, -1.76, 0.50, "steel", bevel=0.02)
    P._rod("dial", (1.26, -1.93, 0.36), (1.26, -1.91, 0.36), 0.06, "ceramic", segs=16, bevel=0.0)
    S.box("needle", 1.255, -1.935, 0.36, 1.31, -1.93, 0.372, "med_red", bevel=0.0)
    P.glow(P._rod("run_lamp", (1.52, -1.935, 0.38), (1.52, -1.915, 0.38), 0.035, "backdrop", segs=10, bevel=0.0),
           "indicator")
    P.glow(S.box("top_lamp", 1.46, -1.88, 0.50, 1.60, -1.80, 0.515, "backdrop", bevel=0.003), "indicator")
    P._hazard(1.14, 1.66, -1.925, -1.91, 0.12, 0.20, n=4)
    return (2, 2), CHEM_STACK[2] + 0.02, {"smoke": CHEM_STACK}


# ---------------------------------------------------------------- watermill
WM_FP = (5, 6)
WM_AXLE_Y = -5.5                  # the wheel row's centre (authored south)
WM_AXLE_Z = 0.36
WM_R = 0.52                       # paddle tip radius
WM_PADDLES = 8
WM_FRAMES = 6
WM_STEP = 7.5                     # 6 x 7.5 = 45, one paddle pitch
WM_ZMAX = 2.0


def _watermill_base():
    """The 5x5 deck and shed, and the two bearing posts over the water row."""
    S.box("sill", 0.04, -5.0, 0.0, 4.96, -0.04, 0.10, "bark", bevel=0.02)
    for k in range(10):                                            # deck planks, east-west
        y0 = -0.06 - k * 0.492
        S.box("plank%d" % k, 0.06, y0 - 0.47, 0.10, 4.94, y0, 0.16, "wood_plank" if k % 2 else "wood", bevel=0.01)
    for x in (0.10, 4.90):                                         # edge beams
        S.box("beam", x - 0.06, -4.98, 0.10, x + 0.06, -0.06, 0.20, "bark", bevel=0.01)
    # the shed: plank walls, a gabled plank roof (ridge north-south)
    sx0, sx1, sy0, sy1, wall = 0.60, 3.40, -3.60, -0.50, 0.95
    S.box("shed", sx0, sy0, 0.16, sx1, sy1, wall, "wood", bevel=0.03)
    for k in range(6):
        x = sx0 + 0.10 + k * 0.46
        S.box("batten", x, sy0 - 0.012, 0.18, x + 0.04, sy0 + 0.01, wall - 0.04, "bark", bevel=0.0)
    S.box("door", 1.70, sy0 - 0.02, 0.16, 2.30, sy0 + 0.02, 0.80, "wood_plank", bevel=0.01)
    cxr = (sx0 + sx1) / 2
    for side in (-1, 1):
        S.prism("roof", [(cxr, sy0 - 0.12), (cxr, sy1 + 0.12), (cxr + side * 1.55, sy1 + 0.12),
                         (cxr + side * 1.55, sy0 - 0.12)], 0.0, 0.05, "bark", bevel=0.01)
        ob = S.objects()[-1]
        from mathutils import Matrix
        ob.matrix_world = (Matrix.Translation((cxr, 0, wall + 0.42)) @ Matrix.Rotation(math.radians(-side * 16), 4, "Y")
                           @ Matrix.Translation((-cxr, 0, 0)))
    S.box("ridge", cxr - 0.06, sy0 - 0.14, wall + 0.40, cxr + 0.06, sy1 + 0.14, wall + 0.50, "metal_dark", bevel=0.01)
    S.cyl("vent", cxr, -1.4, 0.12, wall + 0.40, wall + 0.80, "steel_dark", segs=14)
    S.cyl("vent_cap", cxr, -1.4, 0.17, wall + 0.78, wall + 0.84, "metal_dark", segs=14, bevel=0.01)
    # the generator under a steel hood beside the shed (east), cable box with lamps
    S.box("gen_pad", 3.60, -3.40, 0.16, 4.70, -1.20, 0.22, "stone_dark", bevel=0.01)
    P._rod("generator", (4.15, -3.20, 0.62), (4.15, -1.50, 0.62), 0.38, "steel", segs=24, bevel=0.03)
    P._rod("gen_band", (4.15, -2.50, 0.62), (4.15, -2.20, 0.62), 0.392, "copper", segs=24, bevel=0.0)
    for y in (-3.10, -1.60):
        S.box("gen_foot", 3.82, y - 0.08, 0.22, 4.48, y + 0.08, 0.40, "metal_dark", bevel=0.01)
    S.box("cable_box", 3.70, -1.05, 0.16, 4.60, -0.60, 0.80, "steel_dark", bevel=0.025)
    for k, x in enumerate((3.88, 4.10)):
        P.glow(S.box("lamp%d" % k, x, -1.065, 0.60, x + 0.12, -1.05, 0.70, "backdrop", bevel=0.003), "indicator")
    P.glow(S.box("box_top_lamp", 3.86, -0.92, 0.80, 4.40, -0.74, 0.815, "backdrop", bevel=0.003), "indicator")
    # gear train: the axle runs from the wheel up into the gearbox at the south-east
    S.box("gearbox", 3.60, -4.70, 0.16, 4.60, -3.70, 0.70, "metal_dark", bevel=0.03)
    P._rod("big_gear", (4.10, -4.74, 0.66), (4.10, -4.66, 0.66), 0.42, "steel_dark", segs=32, bevel=0.01)
    for k in range(12):
        a = 2 * math.pi * k / 12
        S.box("tooth", 4.10 + 0.44 * math.cos(a) - 0.03, -4.76, 0.66 + 0.44 * math.sin(a) - 0.03,
              4.10 + 0.44 * math.cos(a) + 0.03, -4.66, 0.66 + 0.44 * math.sin(a) + 0.03, "steel_dark", bevel=0.0)
    P._rod("hub", (4.10, -4.78, 0.66), (4.10, -4.64, 0.66), 0.08, "steel", segs=14)
    P._rod("shaft", (4.10, -3.70, 0.40), (4.10, -1.50, 0.40), 0.05, "steel", segs=10)
    # bearing posts on the bank edge and over the water, the axle between them
    for x in (0.12, 4.88):
        S.box("post", x - 0.10, -5.86, 0.0, x + 0.10, -5.14, WM_AXLE_Z + 0.12, "bark", bevel=0.015)
        P._rod("bearing", (x - 0.13, WM_AXLE_Y, WM_AXLE_Z), (x + 0.13, WM_AXLE_Y, WM_AXLE_Z), 0.09, "metal_dark",
               segs=14)
    P._rod("axle", (0.0, WM_AXLE_Y, WM_AXLE_Z), (5.0, WM_AXLE_Y, WM_AXLE_Z), 0.05, "steel", segs=12)
    P._rod("drive", (4.10, WM_AXLE_Y, WM_AXLE_Z), (4.10, -4.70, 0.62), 0.05, "steel", segs=10)
    # bags and a tool crate on the deck
    S.box("crate", 0.30, -4.70, 0.16, 0.80, -4.20, 0.50, "wood", bevel=0.02)
    S.blob("sack", 1.10, -4.50, 0.20, 0.22, "burlap", seed=31, lumps=0.12, squash=0.6)
    return WM_FP, WM_ZMAX


def _watermill_wheel(frame):
    """The wheel alone: two rims, the drum and eight paddles about the
    east-west axle over the water row, turned frame x 7.5 degrees."""
    def build():
        n0 = len(S.objects())
        x0, x1 = 0.30, 4.70
        P._rod("drum", (x0, WM_AXLE_Y, WM_AXLE_Z), (x1, WM_AXLE_Y, WM_AXLE_Z), 0.16, "wood", segs=16, bevel=0.01)
        for x in (x0, 2.5, x1):
            P._rod("rim", (x - 0.04, WM_AXLE_Y, WM_AXLE_Z), (x + 0.04, WM_AXLE_Y, WM_AXLE_Z), WM_R, "bark", segs=32,
                   bevel=0.0)
        for k in range(WM_PADDLES):
            a = 2 * math.pi * k / WM_PADDLES
            S.box("paddle%d" % k, x0, -0.025, 0.14, x1, 0.025, WM_R - 0.01, "wood_plank", bevel=0.008)
            ob = S.objects()[-1]
            from mathutils import Matrix
            ob.matrix_world = (Matrix.Translation((0, WM_AXLE_Y, WM_AXLE_Z)) @ Matrix.Rotation(a, 4, "X"))
        P._spin(S.objects()[n0:], (0, WM_AXLE_Y, WM_AXLE_Z), "X", -frame * WM_STEP)
        return WM_FP, WM_ZMAX
    return build


WM_WHEEL_AT = {"south": (0, 5), "east": (5, 0), "north": (0, 0), "west": (0, 0)}   # wheel row's top-left tile


def _wheel_layer(facing, frame):
    """Render one wheel frame on a tight canvas over the wheel's own row
    (offset_tiles from the base's top-left tile), the base held out."""
    from mathutils import Matrix
    _watermill_base()
    base = list(S.objects())
    _watermill_wheel(frame)()
    S.face(S.objects(), facing, WM_FP)
    ox, oy = WM_WHEEL_AT[facing]
    S.transform(S.objects(), Matrix.Translation((-ox, oy, 0)))
    fp = (1, 5) if facing in ("east", "west") else (5, 1)
    S.holdout(base)
    name = "watermill_wheel_%s_%02d" % (facing, frame + 1)
    return S.render_asset("power", name, footprint=fp, zmax=WM_R + WM_AXLE_Z + 0.04, tint=False,
                          margins=(0.1, 0.1, 0.1, 0.25),
                          extra={"layer": "wheel", "of": "power/watermill_" + facing, "frame": frame + 1,
                                 "frames": WM_FRAMES, "step_deg": WM_STEP, "facing": facing,
                                 "offset_tiles": [ox, oy]})


# --------------------------------------------------------------- deep drill
DRILL_TOP = 1.90


def _deep_drill():
    """1x2: the derrick on the north tile, the motor and console on the
    south tile facing the operator (who stands south of it)."""
    cx, cy = 0.5, -0.5
    S.box("pad", 0.06, -1.94, 0.0, 0.94, -0.06, 0.06, "stone_dark", bevel=0.015)
    S.cyl("casing", cx, cy, 0.17, 0.06, 0.24, "steel_dark", segs=18, bevel=0.015)
    S.cyl("collar", cx, cy, 0.21, 0.20, 0.26, "metal_dark", segs=18, bevel=0.01)
    P._bolts(cx, cy, 0.19, 0.26, 0.28, 8, rb=0.014)
    # four tapering legs to the crown, three girts with cross braces
    feet = ((0.12, -0.12), (0.88, -0.12), (0.12, -0.88), (0.88, -0.88))
    crown = [(cx + (x - cx) * 0.22, cy + (y - cy) * 0.22) for x, y in feet]
    for k, ((fx, fy), (tx, ty)) in enumerate(zip(feet, crown)):
        P._rod("leg%d" % k, (fx, fy, 0.06), (tx, ty, DRILL_TOP - 0.10), 0.03, "steel", segs=8, bevel=0.0)
    for z in (0.55, 1.05, 1.50):
        t = (z - 0.06) / (DRILL_TOP - 0.16)
        pts = [(fx + (tx - fx) * t, fy + (ty - fy) * t) for (fx, fy), (tx, ty) in zip(feet, crown)]
        for (ax, ay), (bx, by) in ((pts[0], pts[1]), (pts[2], pts[3]), (pts[0], pts[2]), (pts[1], pts[3])):
            P._rod("girt", (ax, ay, z), (bx, by, z), 0.018, "steel_dark", segs=6, bevel=0.0)
    for (ax, ay), (bx, by) in ((feet[2], crown[3]), (feet[3], crown[2])):     # south face X brace
        P._rod("brace", (ax, ay, 0.10), (bx, by, 1.0), 0.014, "steel_dark", segs=6, bevel=0.0)
    S.box("crown", cx - 0.14, cy - 0.14, DRILL_TOP - 0.12, cx + 0.14, cy + 0.14, DRILL_TOP - 0.02, "metal_dark",
          bevel=0.01)
    P._rod("sheave", (cx - 0.08, cy, DRILL_TOP + 0.02), (cx + 0.08, cy, DRILL_TOP + 0.02), 0.09, "steel", segs=16)
    P._rod("string", (cx, cy, 0.24), (cx, cy, DRILL_TOP - 0.12), 0.035, "steel_dark", segs=10)
    S.box("swivel", cx - 0.07, cy - 0.07, 0.95, cx + 0.07, cy + 0.07, 1.15, "corn_yellow", bevel=0.015)
    P.glow(S.ellipsoid("lamp", cx, cy, DRILL_TOP + 0.13, 0.04, 0.04, 0.04, "med_red", segs=12, rings=8),
           "sentry_lamp_glow")
    # south tile: motor, cable up to the swivel, console facing south
    S.box("motor", 0.14, -1.42, 0.06, 0.62, -1.04, 0.44, "steel_dark", bevel=0.03)
    for z in (0.14, 0.24, 0.34):
        S.box("motor_fin", 0.12, -1.40, z, 0.64, -1.06, z + 0.03, "metal_dark", bevel=0.0)
    P._rod("cable", (0.40, -1.04, 0.40), (cx, cy - 0.06, 1.0), 0.016, "fur_black", segs=6, bevel=0.0)
    S.box("console", 0.66, -1.86, 0.06, 0.90, -1.30, 0.56, "steel", bevel=0.02)
    S.box("console_top", 0.64, -1.88, 0.56, 0.92, -1.28, 0.60, "metal_dark", bevel=0.01)
    S.box("screen_frame", 0.68, -1.875, 0.28, 0.88, -1.86, 0.50, "metal_dark", bevel=0.004)
    P.glow(S.box("screen", 0.70, -1.885, 0.30, 0.86, -1.87, 0.48, "backdrop", bevel=0.003), "radio_screen_glow")
    P.glow(S.box("console_lamp", 0.70, -1.70, 0.60, 0.86, -1.58, 0.612, "backdrop", bevel=0.003), "indicator")
    # pipe rack west of the console
    for k, x in enumerate((0.16, 0.26, 0.36)):
        P._rod("pipe%d" % k, (x, -1.88, 0.10), (x, -1.52, 0.10), 0.04, "steel" if k % 2 else "steel_dark", segs=8)
    P._hazard(0.16, 0.56, -1.95, -1.935, 0.0, 0.05, n=3)
    return (1, 2), DRILL_TOP + 0.2


# --------------------------------------------------------- ground scanner
SCAN_MAST = (2.60, -0.40, 1.70)


def _ground_scanner():
    """3x3: a heavy round bed, six transducer pods round the emitter column,
    a cable trunk south to the operator's console (middle of the south row)
    and a sensor mast at the north-east."""
    cx, cy = 1.5, -1.4
    S.box("footing", 0.08, -2.92, 0.0, 2.92, -0.08, 0.05, "stone", bevel=0.015)
    S.cyl("bed", cx, cy, 1.18, 0.05, 0.20, "steel_dark", segs=36, bevel=0.03)
    S.cyl("bed_ring", cx, cy, 1.05, 0.20, 0.24, "metal_dark", segs=36, bevel=0.01)
    P._bolts(cx, cy, 1.12, 0.20, 0.22, 18, rb=0.018)
    for k in range(6):
        a = 2 * math.pi * k / 6 + math.pi / 6
        px, py = cx + 0.78 * math.cos(a), cy + 0.78 * math.sin(a)
        S.cyl("pod%d" % k, px, py, 0.18, 0.24, 0.52, "steel", segs=16, r_top=0.14, bevel=0.02)
        S.cyl("pod_cap%d" % k, px, py, 0.15, 0.52, 0.56, "metal_dark", segs=16, bevel=0.01)
        P.glow(S.cyl("pod_lamp%d" % k, px, py, 0.06, 0.56, 0.575, "backdrop", segs=10, bevel=0.0), "indicator")
        P._rod("pod_arm%d" % k, (px, py, 0.40), (cx + 0.30 * math.cos(a), cy + 0.30 * math.sin(a), 0.70), 0.03,
               "steel_dark", segs=8, bevel=0.0)
    S.cyl("column", cx, cy, 0.32, 0.24, 1.02, "steel", segs=24, bevel=0.03)
    for z in (0.40, 0.66):
        S.cyl("coil", cx, cy, 0.335, z, z + 0.10, "copper", segs=24, bevel=0.01)
    S.cyl("emitter", cx, cy, 0.38, 1.02, 1.12, "metal_dark", segs=24, bevel=0.015)
    S.ellipsoid("dome", cx, cy, 1.12, 0.26, 0.26, 0.14, "plasteel_blue", segs=20, rings=10)
    # cable trunk to the console (south middle), console facing south
    S.box("trunk", 1.42, -2.56, 0.05, 1.58, -2.20, 0.12, "fur_black", bevel=0.02)
    S.box("console", 1.06, -2.94, 0.05, 1.94, -2.56, 0.56, "steel", bevel=0.025)
    S.box("console_top", 1.04, -2.96, 0.56, 1.96, -2.54, 0.60, "metal_dark", bevel=0.012)
    S.box("screen_frame", 1.14, -2.955, 0.24, 1.70, -2.94, 0.52, "metal_dark", bevel=0.006)
    P.glow(S.box("screen", 1.17, -2.965, 0.27, 1.67, -2.95, 0.49, "backdrop", bevel=0.004), "radio_screen_glow")
    for k in range(3):                                               # scan rings on the screen (dark)
        S.box("trace%d" % k, 1.22 + 0.15 * k, -2.97, 0.34 + 0.03 * k, 1.30 + 0.15 * k, -2.965, 0.37 + 0.03 * k,
              "fur_black", bevel=0.0)
    for k, x in enumerate((1.76, 1.86)):
        P.glow(P._rod("key_lamp%d" % k, (x, -2.96, 0.40), (x, -2.94, 0.40), 0.025, "backdrop", segs=8, bevel=0.0),
               "indicator")
    P.glow(S.box("top_strip", 1.14, -2.80, 0.60, 1.86, -2.70, 0.612, "backdrop", bevel=0.003), "indicator")
    # sensor mast (north-east corner)
    mx, my, top = SCAN_MAST
    S.box("mast_foot", mx - 0.10, my - 0.10, 0.05, mx + 0.10, my + 0.10, 0.12, "metal_dark", bevel=0.01)
    S.cyl("mast", mx, my, 0.035, 0.12, top - 0.05, "steel", r_top=0.022, segs=10, bevel=0.0)
    for z, half in ((1.10, 0.18), (1.36, 0.12)):
        P._rod("crossbar", (mx - half, my, z), (mx + half, my, z), 0.014, "steel", segs=6, bevel=0.0)
    P.glow(S.ellipsoid("mast_lamp", mx, my, top, 0.045, 0.045, 0.045, "med_red", segs=12, rings=8),
           "sentry_lamp_glow")
    P._hazard(0.20, 0.90, -2.93, -2.915, 0.0, 0.05, n=4)
    return (3, 3), top + 0.08


# ------------------------------------------------------------ moisture pump
def _moisture_pump():
    """1x1: drum tank on a skid, intake pipe into the ground (south-west),
    the motor and a top fan, an outlet spout (east) and a lamp."""
    cx, cy = 0.5, -0.52
    S.box("skid", 0.10, -0.90, 0.0, 0.90, -0.14, 0.05, "steel_dark", bevel=0.01)
    S.cyl("tank", cx, cy, 0.28, 0.05, 0.56, "steel", segs=24, bevel=0.03)
    for z in (0.18, 0.40):
        S.cyl("band", cx, cy, 0.288, z, z + 0.04, "metal_dark", segs=24, bevel=0.0)
    S.cyl("motor", cx, cy, 0.20, 0.56, 0.74, "steel_dark", segs=20, bevel=0.02)
    O_fan = (cx, cy, 0.75)
    P._fan(O_fan[0], O_fan[1], O_fan[2], 0.15, "Z")
    # intake into the ground, a flange where it goes in
    P._rod("intake", (0.26, -0.70, 0.30), (0.18, -0.80, 0.08), 0.045, "copper", segs=10)
    S.cyl("intake_flange", 0.18, -0.80, 0.075, 0.0, 0.06, "metal_dark", segs=12, bevel=0.008)
    # outlet spout to the east with a drip tray
    P._rod("outlet", (0.76, -0.52, 0.22), (0.92, -0.52, 0.22), 0.04, "copper", segs=10)
    P._rod("spout", (0.92, -0.52, 0.22), (0.94, -0.52, 0.12), 0.035, "copper", segs=10)
    S.box("tray", 0.84, -0.66, 0.0, 0.98, -0.38, 0.04, "metal_dark", bevel=0.008)
    S.box("tray_water", 0.86, -0.64, 0.035, 0.96, -0.40, 0.042, "water_shallow", bevel=0.0)
    S.box("panel", 0.40, -0.83, 0.10, 0.60, -0.79, 0.36, "steel_dark", bevel=0.008)
    P.glow(P._rod("lamp", (0.50, -0.835, 0.28), (0.50, -0.815, 0.28), 0.03, "backdrop", segs=10, bevel=0.0),
           "indicator")
    return (1, 1), 0.92


# ---------------------------------------------------- electric crematorium
CREM_STACK = (2.40, -0.40, 1.60)


def _crematorium():
    """3x2 bench, stepped: the masonry chamber block (stone tint) along the
    back with a steel vent and the chimney; a lower front block whose south
    face holds the chamber door (middle cell, the worker's spot), the
    control panel (west) and the ash drawer (east); a charging tray with
    rails on the front block's top leads into the chamber."""
    S.box("plinth", 0.06, -1.94, 0.0, 2.94, -0.06, 0.08, "steel_dark", bevel=0.015)
    # back: the chamber block, mortar courses, a cap and a steel vent grille
    S.box("chamber", 0.12, -1.16, 0.08, 2.88, -0.14, 1.06, "stone_tint", bevel=0.04, segments=2)
    for k in range(4):
        z = 0.66 + 0.10 * k
        S.box("course%d" % k, 0.14, -1.172, z, 2.86, -1.162, z + 0.012, "stone_tint", bevel=0.0)
    for k, x in enumerate((0.70, 1.20, 1.80, 2.30)):
        S.box("joint%d" % k, x, -1.172, 0.62, x + 0.012, -1.162, 1.04, "stone_tint", bevel=0.0)
    S.box("cap", 0.08, -1.20, 1.06, 2.92, -0.10, 1.12, "stone_tint", bevel=0.02)
    S.box("vent", 0.90, -0.92, 1.12, 1.90, -0.38, 1.16, "metal_dark", bevel=0.01)
    for k in range(6):
        x = 0.98 + 0.15 * k
        S.box("vent_slot%d" % k, x, -0.86, 1.16, x + 0.06, -0.44, 1.17, "steel_dark", bevel=0.0)
    # front: a lower block, its top a steel work plate with the charging tray
    S.box("front", 0.12, -1.86, 0.08, 2.88, -1.10, 0.62, "stone_tint", bevel=0.03, segments=2)
    S.box("front_course", 0.14, -1.872, 0.36, 2.86, -1.862, 0.372, "stone_tint", bevel=0.0)
    S.box("plate", 0.94, -1.82, 0.62, 2.06, -1.14, 0.65, "metal_dark", bevel=0.01)
    for x in (1.08, 1.92):
        S.box("rail", x - 0.03, -1.80, 0.65, x + 0.03, -1.12, 0.68, "steel", bevel=0.005)
    S.box("tray", 1.12, -1.70, 0.68, 1.88, -1.24, 0.71, "steel_dark", bevel=0.008)
    # the steel chamber door, a spy window and a wheel latch
    S.box("door_frame", 0.98, -1.89, 0.10, 2.02, -1.85, 0.58, "metal_dark", bevel=0.015)
    S.box("door", 1.06, -1.91, 0.14, 1.94, -1.88, 0.54, "steel", bevel=0.015)
    S.box("window_frame", 1.30, -1.925, 0.36, 1.70, -1.905, 0.50, "metal_dark", bevel=0.006)
    P.glow(S.box("window", 1.34, -1.935, 0.385, 1.66, -1.92, 0.475, "ash_char", bevel=0.004), "gland_ember")
    P._rod("latch", (1.50, -1.95, 0.24), (1.50, -1.92, 0.24), 0.06, "metal_dark", segs=14, bevel=0.0)
    P._rod("latch_hub", (1.50, -1.965, 0.24), (1.50, -1.95, 0.24), 0.022, "steel", segs=8, bevel=0.0)
    # the chamber mouth glows through a slot above the tray too
    S.box("mouth", 1.10, -1.18, 0.66, 1.90, -1.15, 0.86, "metal_dark", bevel=0.008)
    P.glow(S.box("mouth_glow", 1.16, -1.185, 0.68, 1.84, -1.17, 0.82, "ash_char", bevel=0.004), "gland_ember")
    # west: the control panel on the front block's top, dials and a lamp
    S.box("panel", 0.24, -1.70, 0.62, 0.80, -1.26, 0.72, "steel_dark", bevel=0.012)
    for k, x in enumerate((0.36, 0.52)):
        S.cyl("dial%d" % k, x, -1.48, 0.05, 0.72, 0.74, "ceramic", segs=14, bevel=0.0)
    P.glow(S.cyl("panel_lamp", 0.68, -1.48, 0.035, 0.72, 0.745, "backdrop", segs=10, bevel=0.0), "indicator")
    S.box("panel_face", 0.24, -1.875, 0.40, 0.80, -1.862, 0.56, "steel_dark", bevel=0.004)
    # east: the ash drawer with a handle, an urn ready on the front top
    S.box("drawer", 2.20, -1.89, 0.12, 2.76, -1.86, 0.32, "metal_dark", bevel=0.01)
    S.box("handle", 2.40, -1.905, 0.20, 2.56, -1.89, 0.24, "steel", bevel=0.004)
    S.cyl("urn", 2.50, -1.48, 0.10, 0.62, 0.82, "ceramic", segs=16, r_top=0.07, bevel=0.015)
    S.cyl("urn_lid", 2.50, -1.48, 0.075, 0.82, 0.86, "metal_dark", segs=14, bevel=0.008)
    # chimney at the back (north-east)
    cx, cy, top = CREM_STACK
    S.box("chimney", cx - 0.20, cy - 0.20, 1.12, cx + 0.20, cy + 0.20, top - 0.06, "stone_tint", bevel=0.02)
    S.box("chimney_cap", cx - 0.24, cy - 0.24, top - 0.06, cx + 0.24, cy + 0.24, top, "stone_tint", bevel=0.015)
    S.box("chimney_flue", cx - 0.12, cy - 0.12, top, cx + 0.12, cy + 0.12, top + 0.01, "ash_char", bevel=0.0)
    return (3, 2), top + 0.04


# ------------------------------------------------------------ deep-ore marker
ORE_INK = (0.93, 0.93, 0.90)
ORE_EDGE = (0.07, 0.07, 0.08)


def _deep_ore():
    """Flat 1x1 marker: a dashed ring round three faceted nuggets."""
    n = C.PPT
    xx, yy = O._grid()
    c = n / 2
    r = np.hypot(xx - c, yy - c)
    ang = (np.degrees(np.arctan2(yy - c, xx - c)) + 360.0) % 30.0
    ring = np.where(ang < 18.0, np.abs(r - 40.0), 1e9)
    nug = np.minimum(np.minimum(O._poly(xx, yy, [(c - 20, c + 6), (c - 12, c - 6), (c - 2, c - 2), (c - 6, c + 12),
                                                  (c - 16, c + 13)]),
                                O._poly(xx, yy, [(c + 2, c - 4), (c + 10, c - 16), (c + 20, c - 10), (c + 18, c + 2),
                                                 (c + 8, c + 4)])),
                     O._poly(xx, yy, [(c - 2, c + 8), (c + 8, c + 6), (c + 14, c + 14), (c + 6, c + 20),
                                      (c - 2, c + 16)]))
    img = np.zeros((n, n, 4), np.float32)
    O._paint(img, O._cov(np.minimum(ring - 3.0, nug) - 2.5), ORE_EDGE, 0.75)
    O._paint(img, O._cov(ring - 3.0), ORE_INK, 0.85)
    O._paint(img, O._cov(nug), ORE_INK)
    return O._save_flat("deep_ore", n, n, img, {"overlay": "deep_ore", "ore_colour": True, "tileable": False,
                                                 "marking": True})


# --------------------------------------------------------------------- icons
def _wheel_composite(facing="south", frame=1):
    """The watermill base with one wheel frame over it, placed by the
    frame's offset_tiles and both origins (a temp PNG for the icon)."""
    base = S.out_path("power", "watermill_" + facing)
    wheel = S.out_path("power", "watermill_wheel_%s_%02d" % (facing, frame))
    if not (os.path.exists(base) and os.path.exists(wheel)):
        return None
    ob = P.origin(WM_ZMAX)
    ow = P.origin(WM_R + WM_AXLE_Z + 0.04, (0.1, 0.1, 0.1, 0.25))
    tx, ty = WM_WHEEL_AT[facing]
    dx, dy = ob[0] + tx * C.PPT - ow[0], ob[1] + ty * C.PPT - ow[1]
    out, g = S._read_png(base).copy(), S._read_png(wheel)
    h, w = g.shape[:2]
    y0, x0 = max(0, dy), max(0, dx)
    y1, x1 = min(out.shape[0], dy + h), min(out.shape[1], dx + w)
    src = g[y0 - dy:y1 - dy, x0 - dx:x1 - dx]
    dst = out[y0:y1, x0:x1]
    a = src[..., 3:4]
    oa = a + dst[..., 3:4] * (1 - a)
    dst[..., :3] = (src[..., :3] * a + dst[..., :3] * dst[..., 3:4] * (1 - a)) / np.maximum(oa, 1e-6)
    dst[..., 3:4] = oa
    path = os.path.join(tempfile.gettempdir(), "rwg_watermill_preview.png")
    S.write_png(out, path)
    return path


ICONS = {
    "waterproof_conduit": ("power", "waterproof_conduit_14"),
    "chemfuel_generator": ("power", "chemfuel_generator"),
    "deep_drill": ("power", "deep_drill_south"),
    "ground_scanner": ("power", "ground_scanner_south"),
    "moisture_pump": ("power", "moisture_pump"),
    "electric_crematorium": ("furniture", "electric_crematorium_south"),
}


def icons():
    out = [O._icon(k, fam, spr) for k, (fam, spr) in ICONS.items()]
    out.append(O._cut_icon("watermill", _wheel_composite("south", 1),
                           "%s:power/watermill_south+watermill_wheel_south_01" % C.NAMESPACE))
    return [p for p in out if p]


# --------------------------------------------------------------------- build
def build(only=None):
    """Every A4 sprite and icon; `only` (conduit, chemfuel, watermill, drill,
    scanner, pump, crematorium, ore) renders just those, without icons."""
    def want(n):
        return not only or n in only
    out = []
    if want("conduit"):
        for mask in range(16):
            con = _waterproof_conduit(mask)
            out.append(S.render_asset("power", "waterproof_conduit_%02d" % mask, zmax=0.2, clip=tuple(sorted(con)),
                                      extra={"autotile": "conduit", "mask": mask, "building": "waterproof conduit"}))
    if want("chemfuel"):
        out += P.render("chemfuel_generator", _chemfuel_generator,
                        extra={"building": "chemfuel generator", "fuel": "chemfuel"})
    if want("watermill"):
        for f in FACINGS:
            out += P.render("watermill_" + f, _watermill_base, facing=f,
                            extra={"building": "watermill generator", "wheel_layer": "power/watermill_wheel_" + f,
                                   "frames": WM_FRAMES})
            for i in range(WM_FRAMES):
                out.append(_wheel_layer(f, i))
    for key, name, fn, fam, extra in (
            ("drill", "deep_drill", _deep_drill, "power", {"building": "deep drill"}),
            ("scanner", "ground_scanner", _ground_scanner, "power", {"building": "ground-penetrating scanner"}),
            ("crematorium", "electric_crematorium", _crematorium, "furniture",
             {"building": "electric crematorium", "stuff_tint": "stone_tint",
              "attachments": {"worker": [1.5, 2.15]}})):
        if want(key):
            for f in FACINGS:
                out += P.render("%s_%s" % (name, f), fn, facing=f, family=fam, extra=extra)
    if want("pump"):
        out += P.render("moisture_pump", _moisture_pump, extra={"building": "moisture pump"})
    if want("ore"):
        out.append(_deep_ore())
    if not only:
        out += icons()
    return out
