"""AX-A8 escape ship (AX-AR4; ART spec §12-§16): the ship structural beam,
the ship stasis casket, the computer core, the reactor, the engine and the
sensor cluster, the mind core item and the launch flare, plus their
Architect and item icons. Our own designs: nothing traced or
reconstructed. Footprints are the construction Defs (wiki sizes).

Sprites follow power.py (@2x; authored facing south, turned with
stage.face(); emissive parts get a `<name>_glow` layer, same canvas, that
the renderer draws on top, unlit by the light grid, while the part is
live). Ship parts share one material language: pale plasteel plating
(`plasteel_wall`) over dark steel frames, plasteel-trim seams, gold
contacts and cyan system lamps (`indicator`).

- ship_beam_<facing> (structure, 2x6): the keel truss. Two plasteel
  girders along the long axis with cross ties, a raised bolted spine, a
  coupling flange with gold contacts at each end (so beams laid end to end
  read as one keel; beams are not a linked/autotile family) and eight
  casket ports, four down each side, with a cyan lamp each (glow).
- ship_casket_<facing> (structure, 1x2): the ship's stasis pod, head
  north with a docking clamp there that plugs into a beam; a slim plasteel
  shell and a long viewport. Empty: dark glass over a padded cradle, no
  glow. ship_casket_occupied_<facing>: the viewport frosted over, with a
  teal light strip and a green lamp as its glow layer.
- ship_core (structure, 2x2): a monolith cabinet with gold heat fins and
  rows of status lamps, the mind core's glass socket on top. Glow: the
  core and the lamps.
- ship_reactor (structure, 6x7): a shielded containment vessel (uranium
  bands) on a heavy slab, a viewing ring and a domed cap over the core,
  four radiator banks, coolant pipes and the control desk mid-south. The
  sprite shows it cold; its glow layer is the active state (core, ring,
  vents and desk screen lit), drawn while the reactor runs.
- ship_engine_<facing> (structure, 3x4): the drive housing on the north
  rows, fuel feeds and gimbal ring, and a big bell nozzle opening to the
  facing side. Glow: the hot throat and two housing lamps.
- ship_sensor (structure, 2x2): an equipment cabinet carrying a tilted
  dish on a mast, a glass sensor dome and three antenna spikes with tip
  lamps (glow).
- item/mind_core (item "mind core"): a faceted violet core in a gold
  cradle ring, lit from inside.
- effect/launch_flare (256x192 particle, origin at the centre): the
  ground blast of a launch, a white-hot core, an amber flare with eight
  spikes and a low ring of smoke. Shared by pod and ship launches.

    make art FAMILIES=starship
"""
import math

import numpy as np
from mathutils import Matrix

from .. import contracts as C
from .. import stage as S
from . import effect as E
from . import overlay as O
from . import power as P

FAMILY = "structure"
FACINGS = ("south", "east", "north", "west")
HULL, TRIM, FRAME, DARK = "plasteel_wall", "plasteel_trim", "steel_dark", "metal_dark"


# ------------------------------------------------------------------ beam
BEAM_TOP = 0.42


def _beam():
    """2x6 keel truss along y (north 0 .. south -6)."""
    y0, y1 = -5.86, -0.14
    S.box("bed", 0.10, y0, 0.0, 1.90, y1, 0.08, DARK, bevel=0.02)
    for x in (0.30, 1.70):                                       # two I-girders
        S.box("girder_web", x - 0.05, y0 + 0.1, 0.08, x + 0.05, y1 - 0.1, 0.32, FRAME, bevel=0.01)
        S.box("girder_cap", x - 0.14, y0 + 0.1, 0.32, x + 0.14, y1 - 0.1, 0.38, TRIM, bevel=0.012)
    for k in range(11):                                          # cross ties
        y = y0 + 0.40 + k * 0.50
        S.box("tie%d" % k, 0.30, y - 0.04, 0.16, 1.70, y + 0.04, 0.24, FRAME, bevel=0.008)
    S.box("spine", 0.66, y0 + 0.25, 0.20, 1.34, y1 - 0.25, BEAM_TOP - 0.04, HULL, bevel=0.03)
    S.box("spine_ridge", 0.92, y0 + 0.35, BEAM_TOP - 0.04, 1.08, y1 - 0.35, BEAM_TOP, TRIM, bevel=0.01)
    for k in range(12):                                          # spine bolts
        y = y0 + 0.45 + k * 0.47
        for x in (0.76, 1.24):
            S.cyl("bolt", x, y, 0.025, BEAM_TOP - 0.05, BEAM_TOP - 0.025, DARK, segs=6, bevel=0.0)
    for k, (ya, yb) in enumerate(((y0 - 0.06, y0 + 0.16), (y1 - 0.16, y1 + 0.06))):   # end couplings
        S.box("flange%d" % k, 0.18, ya, 0.06, 1.82, yb, 0.36, DARK, bevel=0.02)
        for x in (0.45, 1.0, 1.55):
            S.box("contact%d" % k, x - 0.09, ya - 0.006, 0.18, x + 0.09, yb + 0.006, 0.26, "gold", bevel=0.006)
    for k in range(4):                                           # casket ports, both sides
        y = y0 + 0.95 + k * 1.30
        for s, x in ((-1, 0.10), (1, 1.90)):
            S.box("port", x - 0.10 if s < 0 else x - 0.04, y - 0.22, 0.08, x + 0.04 if s < 0 else x + 0.10,
                  y + 0.22, 0.30, TRIM, bevel=0.012)
            S.box("port_mouth", x - 0.11 if s < 0 else x + 0.05, y - 0.15, 0.12, x - 0.05 if s < 0 else x + 0.11,
                  y + 0.15, 0.26, "backdrop", bevel=0.0)
            P.glow(S.box("port_lamp", x - 0.06, y + 0.24, 0.30, x + 0.06, y + 0.34, 0.32, "backdrop", bevel=0.0),
                   "indicator")
    return (2, 6), BEAM_TOP + 0.02


# ------------------------------------------------------------ ship casket
def _ship_casket(occupied):
    """1x2, head north: cradle rails, slim shell, viewport, docking clamp."""
    def build():
        S.box("rails", 0.16, -1.90, 0.0, 0.84, -0.14, 0.08, DARK, bevel=0.015)
        S.box("shell", 0.12, -1.86, 0.08, 0.88, -0.26, 0.40, HULL, bevel=0.06, segments=2)
        S.box("seam", 0.10, -1.88, 0.22, 0.90, -0.24, 0.25, TRIM, bevel=0.0)
        S.box("lid", 0.18, -1.80, 0.40, 0.82, -0.34, 0.50, HULL, bevel=0.05, segments=2)
        if occupied:
            S.box("viewport", 0.27, -1.66, 0.50, 0.73, -0.56, 0.53, "casket_frost", bevel=0.02)
            P.glow(S.box("strip_w", 0.235, -1.66, 0.49, 0.26, -0.56, 0.515, "backdrop", bevel=0.0), "med_teal_glow")
            P.glow(S.box("strip_e", 0.74, -1.66, 0.49, 0.765, -0.56, 0.515, "backdrop", bevel=0.0), "med_teal_glow")
        else:
            S.box("viewport", 0.27, -1.66, 0.50, 0.73, -0.56, 0.53, "casket_cradle", bevel=0.02)
            S.box("glint", 0.31, -1.40, 0.53, 0.36, -0.70, 0.535, "glass", bevel=0.0)
        # head: the docking clamp that plugs into a beam port
        S.box("clamp", 0.22, -0.30, 0.10, 0.78, -0.04, 0.44, FRAME, bevel=0.02)
        S.box("clamp_jaw", 0.34, -0.06, 0.16, 0.66, 0.0, 0.36, "gold", bevel=0.008)
        S.box("panel", 0.30, -0.46, 0.50, 0.70, -0.36, 0.56, DARK, bevel=0.008)
        lamp = S.box("lamp", 0.44, -0.44, 0.56, 0.56, -0.38, 0.575, "backdrop" if occupied else "steel_dark",
                     bevel=0.0)
        if occupied:
            P.glow(lamp, "radio_screen_glow")
        S.box("foot_vent", 0.32, -1.875, 0.14, 0.68, -1.86, 0.32, DARK, bevel=0.0)
        return (1, 2), 0.60
    return build


# -------------------------------------------------------------- the core
def _core():
    """2x2 monolith: plinth, cabinet, gold fins, lamp rows, core socket."""
    S.box("plinth", 0.10, -1.90, 0.0, 1.90, -0.10, 0.10, DARK, bevel=0.02)
    S.box("cabinet", 0.24, -1.72, 0.10, 1.76, -0.28, 0.92, HULL, bevel=0.05, segments=2)
    S.box("cap", 0.20, -1.76, 0.92, 1.80, -0.24, 0.98, TRIM, bevel=0.02)
    for k in range(5):                                           # gold heat fins, east and west flanks
        z = 0.24 + k * 0.13
        for x0, x1 in ((0.16, 0.25), (1.75, 1.84)):
            S.box("fin", x0, -1.60, z, x1, -0.40, z + 0.05, "gold", bevel=0.006)
    S.box("face", 0.44, -1.735, 0.22, 1.56, -1.72, 0.80, DARK, bevel=0.006)
    for r in range(4):                                           # status lamp rows on the south face
        for c in range(6):
            x = 0.56 + c * 0.176
            z = 0.34 + r * 0.13
            P.glow(S.box("lamp", x - 0.04, -1.745, z - 0.025, x + 0.04, -1.735, z + 0.025, "backdrop", bevel=0.0),
                   "indicator")
    S.cyl("socket", 1.0, -1.0, 0.38, 0.98, 1.06, FRAME, segs=28, bevel=0.012)
    S.cyl("socket_ring", 1.0, -1.0, 0.33, 1.06, 1.09, "gold", segs=28, bevel=0.0)
    P.glow(S.ellipsoid("core", 1.0, -1.0, 1.10, 0.22, 0.22, 0.16, "reagent_violet", segs=10, rings=6),
           "crystal_glow")
    S.ellipsoid("dome", 1.0, -1.0, 1.08, 0.30, 0.30, 0.24, "glass", segs=24, rings=10)
    return (2, 2), 1.34


# ------------------------------------------------------------- reactor
RX_C = (3.0, -3.6)


def _reactor():
    """6x7: slab, radiator banks, vessel with uranium bands, viewing ring,
    domed cap, pipes, the control desk on the south edge."""
    cx, cy = RX_C
    S.box("slab", 0.10, -6.90, 0.0, 5.90, -0.10, 0.14, DARK, bevel=0.03)
    S.box("slab_trim", 0.30, -6.70, 0.14, 5.70, -0.30, 0.18, FRAME, bevel=0.01)
    P._hazard(0.6, 2.2, -6.92, -6.90, 0.02, 0.12, n=4)
    P._hazard(3.8, 5.4, -6.92, -6.90, 0.02, 0.12, n=4)
    # four radiator banks at the corners
    for k, (bx, by) in enumerate(((0.95, -1.15), (5.05, -1.15), (0.95, -5.55), (5.05, -5.55))):
        S.box("rad_base%d" % k, bx - 0.62, by - 0.72, 0.18, bx + 0.62, by + 0.72, 0.30, FRAME, bevel=0.02)
        for j in range(7):
            y = by - 0.60 + j * 0.20
            S.box("rad_fin", bx - 0.55, y - 0.035, 0.30, bx + 0.55, y + 0.035, 1.05, TRIM, bevel=0.008)
        for x in (bx - 0.58, bx + 0.48):                          # end rails, fins left open between
            S.box("rad_rail", x, by - 0.70, 0.30, x + 0.10, by + 0.70, 1.10, HULL, bevel=0.015)
        P.glow(S.box("rad_vent", bx - 0.04, by - 0.62, 1.05, bx + 0.04, by + 0.62, 1.07, "backdrop", bevel=0.0),
               "crystal_glow")
        # coolant pipe from each bank to the vessel
        P._rod("pipe%d" % k, (bx + (0.55 if bx < cx else -0.55), by, 0.55),
               (cx + (-1.35 if bx < cx else 1.35), cy + (0.5 if by > cy else -0.5), 0.55), 0.09, "steel", segs=12)
    # containment vessel
    S.cyl("plinth", cx, cy, 1.85, 0.18, 0.40, FRAME, segs=40, bevel=0.03)
    S.cyl("vessel", cx, cy, 1.55, 0.40, 1.55, HULL, segs=40, bevel=0.04)
    for z in (0.62, 1.20):
        S.cyl("band", cx, cy, 1.58, z, z + 0.14, "uranium_green", segs=40, bevel=0.01)
    S.cyl("ring", cx, cy, 1.60, 1.55, 1.70, DARK, segs=40, bevel=0.012)
    for k in range(12):                                          # viewing slots in the ring
        a = 2 * math.pi * (k + 0.5) / 12
        P.glow(S.box("slot%d" % k, cx + 1.60 * math.cos(a) - 0.12, cy + 1.60 * math.sin(a) - 0.12, 1.58,
                     cx + 1.60 * math.cos(a) + 0.12, cy + 1.60 * math.sin(a) + 0.12, 1.67, "backdrop", bevel=0.0),
               "crystal_glow")
    S.cyl("shoulder", cx, cy, 1.45, 1.70, 1.82, HULL, segs=40, r_top=1.15, bevel=0.02)
    S.ellipsoid("cap", cx, cy, 1.82, 1.15, 1.15, 0.45, HULL, segs=40, rings=12)
    S.cyl("eye_rim", cx, cy, 0.52, 2.18, 2.30, DARK, segs=32, bevel=0.012)
    P.glow(S.ellipsoid("eye", cx, cy, 2.30, 0.42, 0.42, 0.14, "glass", segs=28, rings=8), "crystal_glow")
    for k in range(8):                                           # cap ribs
        a = 2 * math.pi * k / 8
        P._rod("rib%d" % k, (cx + 0.55 * math.cos(a), cy + 0.55 * math.sin(a), 2.20),
               (cx + 1.12 * math.cos(a), cy + 1.12 * math.sin(a), 1.84), 0.04, TRIM, segs=8)
    # control desk on the south edge, facing south
    S.box("desk", 2.10, -6.60, 0.18, 3.90, -5.95, 0.62, FRAME, bevel=0.03)
    S.box("desk_top", 2.05, -6.65, 0.62, 3.95, -5.90, 0.68, DARK, bevel=0.01)
    S.box("screen_frame", 2.35, -6.40, 0.68, 3.65, -6.10, 0.92, DARK, bevel=0.01)
    P.glow(S.box("screen", 2.42, -6.41, 0.71, 3.58, -6.39, 0.89, "backdrop", bevel=0.0), "radio_screen_glow")
    P._rod("desk_cable", (3.0, -5.95, 0.40), (cx, cy - 1.80, 0.30), 0.05, "fur_black", segs=8, bevel=0.0)
    return (6, 7), 2.50


# -------------------------------------------------------------- engine
EN_AX_Z = 0.78


def _engine():
    """3x4, exhaust to the south: housing on the north rows, gimbal ring,
    bell nozzle opening south (its mouth at the south edge)."""
    cx = 1.5
    S.box("skid", 0.12, -3.70, 0.0, 2.88, -0.12, 0.12, DARK, bevel=0.02)
    S.box("housing", 0.26, -1.95, 0.12, 2.74, -0.20, 1.30, HULL, bevel=0.07, segments=2)
    S.box("housing_seam", 0.24, -1.20, 0.12, 2.76, -1.12, 1.32, TRIM, bevel=0.01)
    S.box("housing_top", 0.70, -1.75, 1.30, 2.30, -0.40, 1.42, TRIM, bevel=0.02)
    for x in (0.95, 2.05):                                       # fuel feeds over the top into the gimbal
        P._rod("feed", (x, -0.60, 1.40), (x, -2.05, 1.18), 0.07, "steel", segs=12)
        P._rod("feed_drop", (x, -2.05, 1.18), (x, -2.15, 0.95), 0.07, "steel", segs=12)
    for k, x in enumerate((0.40, 2.60)):
        P.glow(S.box("lamp%d" % k, x - 0.07, -1.965, 0.95, x + 0.07, -1.95, 1.05, "backdrop", bevel=0.0),
               "indicator")
    for k, y in enumerate((-1.50, -0.65)):                       # lamps on the roof show in every facing
        P.glow(S.box("roof_lamp%d" % k, 1.30, y - 0.06, 1.42, 1.70, y + 0.06, 1.435, "backdrop", bevel=0.0),
               "indicator")
    P._rod("gimbal", (cx, -1.95, EN_AX_Z), (cx, -2.25, EN_AX_Z), 0.70, DARK, segs=32, bevel=0.02)
    P._rod("throat", (cx, -2.25, EN_AX_Z), (cx, -2.45, EN_AX_Z), 0.46, FRAME, segs=28, bevel=0.01)
    # the bell: a cone from the throat flaring to the mouth at the south edge
    bell = S.cyl("bell", 0, 0, 0.48, 0.0, 1.20, TRIM, segs=36, r_top=0.76, bevel=0.0)
    bell.matrix_world = Matrix.Translation((cx, -2.45, EN_AX_Z)) @ Matrix.Rotation(math.radians(90), 4, "X")
    for k, t in enumerate((0.35, 0.75)):                         # cooling bands round the bell
        r = 0.48 + (0.76 - 0.48) * t + 0.015
        P._rod("bell_band%d" % k, (cx, -2.45 - 1.2 * t, EN_AX_Z), (cx, -2.50 - 1.2 * t, EN_AX_Z), r, FRAME,
               segs=36, bevel=0.0)
    P._rod("mouth_rim", (cx, -3.63, EN_AX_Z), (cx, -3.67, EN_AX_Z), 0.78, DARK, segs=36, bevel=0.0)
    P.glow(P._rod("mouth", (cx, -3.60, EN_AX_Z), (cx, -3.675, EN_AX_Z), 0.64, "backdrop", segs=36, bevel=0.0),
           "mech_flash")
    for s in (-1, 1):                                            # bell struts to the skid
        P._rod("strut", (cx + s * 0.62, -3.05, EN_AX_Z - 0.3), (cx + s * 1.15, -3.20, 0.12), 0.05, FRAME, segs=8)
    return (3, 4), EN_AX_Z + 0.82


# -------------------------------------------------------------- sensor
def _sensor():
    """2x2: cabinet, mast with a tilted dish, sensor dome, antennae."""
    S.box("plinth", 0.10, -1.90, 0.0, 1.90, -0.10, 0.10, DARK, bevel=0.02)
    S.box("cabinet", 0.22, -1.78, 0.10, 1.78, -0.22, 0.50, HULL, bevel=0.05, segments=2)
    S.box("vent", 0.40, -1.795, 0.20, 1.10, -1.78, 0.40, DARK, bevel=0.0)
    for k in range(4):
        S.box("vent_slat", 0.42, -1.80, 0.22 + k * 0.05, 1.08, -1.79, 0.24 + k * 0.05, FRAME, bevel=0.0)
    P.glow(S.box("panel_lamp", 1.30, -1.795, 0.34, 1.50, -1.78, 0.40, "backdrop", bevel=0.0), "indicator")
    # the dish on a mast at the north-west, tilted to face south and up
    mx, my = 0.72, -0.78
    S.cyl("mast", mx, my, 0.08, 0.50, 1.05, FRAME, segs=12, bevel=0.0)
    n0 = len(S.objects())
    S.cyl("dish", mx, my, 0.12, 1.05, 1.22, TRIM, segs=32, r_top=0.56, bevel=0.0)
    S.cyl("dish_face", mx, my, 0.50, 1.215, 1.225, "steel", segs=32, bevel=0.0)
    S.cyl("dish_inner", mx, my, 0.30, 1.22, 1.23, FRAME, segs=24, bevel=0.0)
    for k in range(3):
        a = math.radians(90 + 120 * k)
        P._rod("feed_strut", (mx + 0.46 * math.cos(a), my + 0.46 * math.sin(a), 1.22), (mx, my, 1.55), 0.014,
               DARK, segs=6, bevel=0.0)
    P.glow(S.ellipsoid("feed", mx, my, 1.56, 0.05, 0.05, 0.05, "backdrop", segs=10, rings=6), "indicator")
    P._spin(S.objects()[n0:], (mx, my, 1.10), "X", -32)
    # glass sensor dome east, three antennae with tip lamps
    S.cyl("dome_base", 1.40, -0.70, 0.26, 0.50, 0.58, FRAME, segs=24, bevel=0.01)
    S.ellipsoid("dome", 1.40, -0.70, 0.58, 0.24, 0.24, 0.22, "glass", segs=24, rings=10)
    for k, (x, y, h) in enumerate(((1.55, -1.45, 1.30), (1.25, -1.55, 1.05), (0.40, -1.50, 0.95))):
        P._rod("antenna%d" % k, (x, y, 0.50), (x, y, h), 0.03, "steel", segs=8, bevel=0.0)
        S.cyl("antenna_foot%d" % k, x, y, 0.07, 0.50, 0.56, FRAME, segs=10, bevel=0.0)
        P.glow(S.ellipsoid("tip%d" % k, x, y, h + 0.03, 0.05, 0.05, 0.05, "backdrop", segs=10, rings=6),
               "sentry_lamp_glow")
    return (2, 2), 1.62


# ---------------------------------------------------------- mind core item
def _mind_core():
    """A faceted violet core lit from inside, in a gold cradle ring."""
    cx, cy = 0.5, -0.52
    S.cyl("foot", cx, cy, 0.20, 0.0, 0.05, DARK, segs=20, bevel=0.01)
    S.cyl("cradle", cx, cy, 0.17, 0.05, 0.12, "gold", segs=20, r_top=0.21, bevel=0.01)
    for k in range(3):
        a = math.radians(90 + 120 * k)
        P._rod("claw%d" % k, (cx + 0.19 * math.cos(a), cy + 0.19 * math.sin(a), 0.11),
               (cx + 0.19 * math.cos(a), cy + 0.19 * math.sin(a), 0.34), 0.025, "gold", segs=6, bevel=0.0)
    S.blob("core", cx, cy, 0.28, 0.19, "reagent_violet", seed=91, lumps=0.05, squash=1.0)
    S.ellipsoid("light", cx - 0.04, cy + 0.03, 0.40, 0.09, 0.09, 0.08, "crystal_glow", segs=10, rings=6)
    return S.render_asset("item", "mind_core", zmax=0.50, extra={"item": "mind core"})


# ---------------------------------------------------------- launch flare
def _launch_flare():
    """256x192: a white-hot core, an amber flare with eight spikes, a low
    ring of smoke round it. Origin at the centre."""
    core, body = E._mat("fire_core"), E._mat("fire_body")
    smoke, dust = E._mat("fire_smoke"), E._mat("dust_cloud")
    w, h = 256, 192
    cx, cy = w / 2, h / 2
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32) + 0.5
    rng = np.random.RandomState(808)
    img = np.zeros((h, w, 4), np.float32)
    dx, dy = (xx - cx), (yy - cy) / 0.62                         # flattened: seen on the ground
    r = np.hypot(dx, dy)
    ang = np.arctan2(dy, dx)
    # smoke ring: lumpy puffs at radius ~95
    for k in range(16):
        a = 2 * np.pi * k / 16 + rng.uniform(-0.12, 0.12)
        rr = 92 + rng.uniform(-8, 8)
        px, py = cx + rr * np.cos(a), cy + rr * np.sin(a) * 0.62
        pr = rng.uniform(17, 25)
        cov = np.clip((pr - np.hypot(xx - px, (yy - py) / 0.8)) / 6.0, 0, 1)
        col = E._mix(smoke["base"], dust["base"], 0.4) if k % 2 else smoke["light"]
        E._over(img, cov, col, 0.75)
    # spikes
    spike = np.clip(1.0 - np.abs(np.sin(ang * 4)) * r / 9.0, 0, 1) * np.clip(1.0 - r / 92.0, 0, 1)
    E._over(img, spike, body["base"], 0.85)
    # flare body and core
    E._over(img, np.clip((58 - r) / 22.0, 0, 1), body["light"], 0.95)
    E._over(img, np.clip((34 - r) / 12.0, 0, 1), core["base"], 1.0)
    E._over(img, np.clip((18 - r) / 6.0, 0, 1), core["light"], 1.0)
    return E._save("launch_flare", img, (int(cx), int(cy)), {"effect": "launch_flare"})


# ------------------------------------------------------------------- icons
ICONS = {
    "ship_beam": (FAMILY, "ship_beam_east"),
    "ship_casket": (FAMILY, "ship_casket_occupied_south"),
    "ship_core": (FAMILY, "ship_core"),
    "ship_reactor": (FAMILY, "ship_reactor"),
    "ship_engine": (FAMILY, "ship_engine_south"),
    "ship_sensor": (FAMILY, "ship_sensor"),
    "mind_core": ("item", "mind_core"),
}


def icons():
    return [p for p in (O._icon(k, fam, spr) for k, (fam, spr) in ICONS.items()) if p]


# ------------------------------------------------------------------- build
def build(only=None):
    """Every A8 sprite and icon; `only` (beam, casket, core, reactor,
    engine, sensor, item, flare) renders just those, without icons."""
    def want(n):
        return not only or n in only
    out = []
    if want("beam"):
        for f in FACINGS:
            out += P.render("ship_beam_" + f, _beam, facing=f, family=FAMILY,
                            extra={"building": "ship structural beam"})
    if want("casket"):
        for f in FACINGS:
            out += P.render("ship_casket_" + f, _ship_casket(False), facing=f, family=FAMILY,
                            extra={"building": "ship stasis casket", "state": "empty",
                                   "occupied_sprite": "%s/ship_casket_occupied_%s" % (FAMILY, f)})
            out += P.render("ship_casket_occupied_" + f, _ship_casket(True), facing=f, family=FAMILY,
                            extra={"building": "ship stasis casket", "state": "occupied"})
    if want("core"):
        out += P.render("ship_core", _core, family=FAMILY, extra={"building": "ship computer core"})
    if want("reactor"):
        out += P.render("ship_reactor", _reactor, family=FAMILY,
                        extra={"building": "ship reactor"})
    if want("engine"):
        for f in FACINGS:
            out += P.render("ship_engine_" + f, _engine, facing=f, family=FAMILY,
                            extra={"building": "ship engine"})
    if want("sensor"):
        out += P.render("ship_sensor", _sensor, family=FAMILY, extra={"building": "ship sensor cluster"})
    if want("item"):
        out.append(_mind_core())
    if want("flare"):
        out.append(_launch_flare())
    if not only:
        out += icons()
    return out
