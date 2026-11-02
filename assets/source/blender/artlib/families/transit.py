"""AX-A9 caskets, pods, scanner and hitching spot (AX-AR4; ART spec
§12-§18): the stasis casket, the transport pod, the pod launcher, the
long-range mineral scanner and the caravan hitching spot, their Architect
icons, and two world-map markers. Our own designs: nothing traced or
reconstructed. Footprints are the construction Defs (wiki sizes).

Sprites follow power.py (@2x; authored facing south, turned with
stage.face(); emissive parts get a `<name>_glow` layer on the same canvas;
attachments in canvas pixels).

- stasis_casket_<facing> (furniture, 1x2): a rounded white capsule on
  four feet, head north with a control pillar and two coolant hoses, a
  long canopy. Empty: the canopy dark (no glow).
  stasis_casket_occupied_<facing>: the canopy frosted over, a teal light
  rim and a green pillar lamp as its glow layer. (The ancient sleeper
  casket, structure/sleeper_casket_*, stays a different, heavier piece.)
- transport_pod (structure, 1x1): an upright capsule on three legs in the
  drop-pod colours, a nose cone, an orange band and a hatch facing south.
- pod_launcher_<facing> (structure, 1x2): a chemfuel cabinet on the north
  tile, a pump and a hose reel with a fuelling arm on the south tile.
  `gauge0` (empty end) and `gauge1` (full end) mark the fuel slot on the
  cabinet's lid, like the battery's charge slot: draw the fuel bar there.
- mineral_scanner_<facing> (power, 3x3): a lattice tower carrying a big
  dish tipped skyward, a tray of ore samples and the operator's console on
  the middle of the south row (the interaction spot is the cell south of
  it). Glow (the working state): console screen, the dish's feed and a
  beacon lamp on the tower.
- overlay/hitching_spot (1x1, flat marking): a dashed chalk ring round a
  hitching post with a coil of rope and two hoof prints.
- ui_icon/pod_flight and ui_icon/mining_site: world-map markers in the
  caravan marker's badge style (dark disc, coloured line glyph): a capsule
  climbing on its trail, and a pick over an ore outcrop.

    make art FAMILIES=transit
"""
import math

import numpy as np

from .. import stage as S
from . import overlay as O
from . import power as P

FACINGS = ("south", "east", "north", "west")


# ------------------------------------------------------------ stasis casket
def _casket(occupied):
    """1x2, head north: feet, rounded body, canopy, pillar, hoses."""
    def build():
        for x in (0.20, 0.80):
            for y in (-1.70, -0.50):
                S.cyl("foot", x, y, 0.06, 0.0, 0.10, "metal_dark", segs=10, bevel=0.0)
        S.box("base", 0.16, -1.84, 0.08, 0.84, -0.32, 0.16, "steel_dark", bevel=0.02)
        S.box("body", 0.12, -1.86, 0.14, 0.88, -0.30, 0.42, "med_white", bevel=0.10, segments=3)
        S.box("trim", 0.11, -1.87, 0.25, 0.89, -0.29, 0.28, "med_teal", bevel=0.0)
        canopy = "casket_frost" if occupied else "casket_cradle"
        S.ellipsoid("canopy", 0.5, -1.08, 0.40, 0.31, 0.70, 0.17, canopy, segs=28, rings=12)
        if occupied:
            P.glow(S.box("rim_w", 0.165, -1.70, 0.40, 0.19, -0.46, 0.43, "backdrop", bevel=0.0), "med_teal_glow")
            P.glow(S.box("rim_e", 0.81, -1.70, 0.40, 0.835, -0.46, 0.43, "backdrop", bevel=0.0), "med_teal_glow")
        else:
            S.ellipsoid("glint", 0.40, -1.20, 0.555, 0.05, 0.36, 0.02, "glass", segs=12, rings=6)
        # head: the control pillar, its lamp and two coolant hoses
        S.box("pillar", 0.28, -0.30, 0.08, 0.72, -0.06, 0.60, "med_white", bevel=0.05, segments=2)
        S.box("pillar_cap", 0.26, -0.32, 0.60, 0.74, -0.04, 0.64, "steel_dark", bevel=0.015)
        S.box("screen", 0.36, -0.325, 0.36, 0.64, -0.31, 0.52, "steel_dark", bevel=0.0)
        lamp = S.ellipsoid("lamp", 0.5, -0.18, 0.66, 0.05, 0.05, 0.03, "backdrop" if occupied else "metal_dark",
                           segs=10, rings=6)
        if occupied:
            P.glow(lamp, "radio_screen_glow")
            P.glow(S.box("screen_lit", 0.38, -0.33, 0.38, 0.62, -0.32, 0.50, "backdrop", bevel=0.0), "med_teal_glow")
        for x in (0.22, 0.78):
            P._rod("hose", (x + (0.10 if x < 0.5 else -0.10), -0.30, 0.40), (x, -0.42, 0.34), 0.03, "steel",
                   segs=8, bevel=0.0)
        return (1, 2), 0.70
    return build


# ------------------------------------------------------------ transport pod
POD_C = (0.5, -0.55)


def _pod():
    """1x1: three legs, capsule body, band, nose cone, cap, hatch south."""
    cx, cy = POD_C
    for k in range(3):
        a = math.radians(-90 + 120 * k)
        P._rod("leg%d" % k, (cx + 0.22 * math.cos(a), cy + 0.22 * math.sin(a), 0.22),
               (cx + 0.40 * math.cos(a), cy + 0.40 * math.sin(a), 0.0), 0.03, "metal_dark", segs=8, bevel=0.0)
        S.cyl("pad%d" % k, cx + 0.40 * math.cos(a), cy + 0.40 * math.sin(a), 0.06, 0.0, 0.025, "metal_dark",
              segs=10, bevel=0.0)
    S.cyl("skirt", cx, cy, 0.34, 0.08, 0.18, "pod_scorch", segs=24, r_top=0.37, bevel=0.01)
    S.cyl("body", cx, cy, 0.37, 0.18, 0.56, "pod_hull", segs=28, bevel=0.03)
    S.cyl("band", cx, cy, 0.375, 0.40, 0.47, "pod_paint", segs=28, bevel=0.0)
    S.cyl("nose", cx, cy, 0.37, 0.56, 0.80, "pod_hull", segs=28, r_top=0.12, bevel=0.01)
    S.cyl("cap", cx, cy, 0.125, 0.80, 0.86, "pod_paint", segs=20, bevel=0.01)
    S.box("hatch", cx - 0.13, cy - 0.385, 0.22, cx + 0.13, cy - 0.35, 0.50, "steel_dark", bevel=0.01)
    S.box("hatch_handle", cx - 0.06, cy - 0.395, 0.30, cx + 0.06, cy - 0.385, 0.32, "steel", bevel=0.0)
    return (1, 1), 0.90


# ------------------------------------------------------------ pod launcher
LID = 0.58


def _launcher():
    """1x2: skid; chemfuel cabinet north with the fuel slot on its lid;
    pump, hose reel and fuelling arm south."""
    S.box("skid", 0.08, -1.92, 0.0, 0.92, -0.08, 0.08, "metal_dark", bevel=0.015)
    S.box("cabinet", 0.14, -0.94, 0.08, 0.86, -0.12, LID - 0.03, "fuel_drum", bevel=0.04)
    S.box("lid", 0.12, -0.96, LID - 0.03, 0.88, -0.10, LID, "steel_dark", bevel=0.012)
    S.box("slot", 0.42, -0.90, LID, 0.58, -0.16, LID + 0.012, "backdrop", bevel=0.0)
    S.cyl("filler", 0.76, -0.22, 0.05, LID, LID + 0.06, "brass", segs=10, bevel=0.0)
    P._hazard(0.20, 0.80, -0.955, -0.94, 0.14, 0.24, n=3)
    # south tile: pump block, hose reel, fuelling arm out over the south edge
    S.box("pump", 0.14, -1.52, 0.08, 0.50, -1.06, 0.40, "steel", bevel=0.03)
    S.box("pump_top", 0.18, -1.48, 0.40, 0.46, -1.10, 0.44, "steel_dark", bevel=0.01)
    P._rod("reel", (0.56, -1.40, 0.30), (0.86, -1.40, 0.30), 0.20, "steel_dark", segs=20, bevel=0.01)
    P._rod("hose_coil", (0.60, -1.40, 0.30), (0.82, -1.40, 0.30), 0.215, "fur_black", segs=20, bevel=0.0)
    P._rod("arm_post", (0.32, -1.70, 0.08), (0.32, -1.70, 0.62), 0.04, "steel", segs=10)
    P._rod("arm", (0.32, -1.70, 0.62), (0.66, -1.94, 0.58), 0.03, "steel", segs=8)
    S.box("nozzle", 0.62, -1.98, 0.50, 0.72, -1.90, 0.60, "pod_paint", bevel=0.01)
    P._rod("feed", (0.32, -1.06, 0.20), (0.32, -0.94, 0.20), 0.04, "copper", segs=8)
    return (1, 2), 0.66, {"gauge0": (0.5, -0.88, LID + 0.012), "gauge1": (0.5, -0.18, LID + 0.012)}


# ---------------------------------------------------------- mineral scanner
SC_TOWER = (1.5, -1.25)
SC_DISH_Z = 1.55


def _scanner():
    """3x3: slab, a lattice tower with a skyward dish, a sample tray, the
    operator's console on the middle of the south row."""
    tx, ty = SC_TOWER
    S.box("slab", 0.08, -2.92, 0.0, 2.92, -0.08, 0.10, "concrete_dark", bevel=0.02)
    S.box("plinth", tx - 0.55, ty - 0.55, 0.10, tx + 0.55, ty + 0.55, 0.22, "steel_dark", bevel=0.02)
    feet = [(tx + sx * 0.45, ty + sy * 0.45) for sx, sy in ((-1, -1), (1, -1), (1, 1), (-1, 1))]
    top = [(tx + sx * 0.14, ty + sy * 0.14) for sx, sy in ((-1, -1), (1, -1), (1, 1), (-1, 1))]
    for k, ((fx, fy), (ux, uy)) in enumerate(zip(feet, top)):
        P._rod("leg%d" % k, (fx, fy, 0.22), (ux, uy, SC_DISH_Z - 0.15), 0.035, "steel", segs=8, bevel=0.0)
    for z in (0.55, 0.95):
        t = (z - 0.22) / (SC_DISH_Z - 0.37)
        pts = [(fx + (ux - fx) * t, fy + (uy - fy) * t) for (fx, fy), (ux, uy) in zip(feet, top)]
        for a, b in zip(pts, pts[1:] + pts[:1]):
            P._rod("girt", (a[0], a[1], z), (b[0], b[1], z), 0.02, "steel_dark", segs=6, bevel=0.0)
    S.cyl("head", tx, ty, 0.20, SC_DISH_Z - 0.18, SC_DISH_Z - 0.04, "metal_dark", segs=20, bevel=0.01)
    # the dish, tipped back to the north so its face shows to the sky and south
    n0 = len(S.objects())
    S.cyl("dish_back", tx, ty, 0.18, SC_DISH_Z, SC_DISH_Z + 0.20, "steel_dark", segs=40, r_top=0.98, bevel=0.0)
    S.cyl("dish_face", tx, ty, 0.94, SC_DISH_Z + 0.195, SC_DISH_Z + 0.205, "steel", segs=40, bevel=0.0)
    S.cyl("dish_ring", tx, ty, 0.62, SC_DISH_Z + 0.20, SC_DISH_Z + 0.21, "plasteel_trim", segs=36, bevel=0.0)
    S.cyl("dish_hub", tx, ty, 0.30, SC_DISH_Z + 0.205, SC_DISH_Z + 0.215, "steel_dark", segs=28, bevel=0.0)
    for k in range(3):
        a = math.radians(90 + 120 * k)
        P._rod("feed_strut", (tx + 0.86 * math.cos(a), ty + 0.86 * math.sin(a), SC_DISH_Z + 0.2),
               (tx, ty, SC_DISH_Z + 0.70), 0.02, "metal_dark", segs=6, bevel=0.0)
    S.cyl("feed_body", tx, ty, 0.07, SC_DISH_Z + 0.62, SC_DISH_Z + 0.74, "metal_dark", segs=12, bevel=0.0)
    P.glow(S.ellipsoid("feed", tx, ty, SC_DISH_Z + 0.76, 0.06, 0.06, 0.05, "backdrop", segs=12, rings=6),
           "indicator")
    P._spin(S.objects()[n0:], (tx, ty, SC_DISH_Z), "X", 28)
    # beacon on a short mast at the north-east corner
    P._rod("mast", (2.60, -0.40, 0.10), (2.60, -0.40, 1.20), 0.03, "steel", segs=8)
    P.glow(S.ellipsoid("beacon", 2.60, -0.40, 1.24, 0.06, 0.06, 0.06, "backdrop", segs=12, rings=6),
           "sentry_lamp_glow")
    # ore sample tray (west of the console)
    S.box("tray", 0.30, -2.80, 0.10, 0.95, -2.20, 0.20, "steel_dark", bevel=0.015)
    for k, (x, y, mat) in enumerate(((0.45, -2.42, "gold"), (0.62, -2.62, "silver"), (0.80, -2.38, "jade"),
                                     (0.50, -2.66, "uranium_green"), (0.80, -2.64, "steel"))):
        S.blob("sample%d" % k, x, y, 0.20, 0.08, mat, seed=310 + k, lumps=0.25, squash=0.7)
    # operator console on the middle of the south row, facing south
    S.box("console", 1.15, -2.92, 0.10, 1.85, -2.45, 0.62, "steel", bevel=0.03)
    S.box("console_top", 1.12, -2.95, 0.62, 1.88, -2.42, 0.66, "metal_dark", bevel=0.01)
    S.box("screen_frame", 1.22, -2.935, 0.30, 1.78, -2.92, 0.56, "metal_dark", bevel=0.004)
    P.glow(S.box("screen", 1.25, -2.945, 0.32, 1.75, -2.93, 0.54, "backdrop", bevel=0.0), "radio_screen_glow")
    P.glow(S.box("console_lamp", 1.40, -2.80, 0.66, 1.60, -2.66, 0.672, "backdrop", bevel=0.0), "indicator")
    P._rod("trunk", (1.5, -2.45, 0.16), (tx, ty - 0.55, 0.16), 0.05, "fur_black", segs=8, bevel=0.0)
    # hazard band and a cabinet east
    S.box("cabinet", 2.20, -2.80, 0.10, 2.80, -2.20, 0.48, "steel_dark", bevel=0.03)
    for k in range(3):
        S.box("cab_vent", 2.30, -2.81, 0.18 + k * 0.08, 2.70, -2.80, 0.21 + k * 0.08, "metal_dark", bevel=0.0)
    return (3, 3), SC_DISH_Z + 1.10


# ------------------------------------------------------------ hitching spot
def _hitching_spot():
    """A dashed chalk ring round a post, a rope coil and two hoof prints."""
    xx, yy = O._grid()
    N = O.N
    img = np.zeros((N, N, 4), np.float32)
    c = N / 2
    chalk, ink = (0.95, 0.91, 0.82), (0.10, 0.09, 0.08)
    rope, wood = (0.80, 0.66, 0.42), (0.52, 0.38, 0.24)
    ring = np.abs(np.hypot(xx - c, yy - c) - 46)
    ang = np.arctan2(yy - c, xx - c)
    dash = (np.floor((ang + np.pi) * 46 / 12.0) % 2) == 0
    O._paint(img, O._cov(np.hypot(xx - c, yy - c) - 46), ink, 0.12)
    O._paint(img, O._cov(ring - 2.6) * dash, ink, 0.45)
    O._paint(img, O._cov(ring - 1.8) * dash, chalk, 0.9)
    # rope coil round the post, its loose end trailing south-east
    for r in (15.0, 20.0):
        d = np.abs(np.hypot(xx - c, (yy - c) * 1.1) - r)
        O._paint(img, O._cov(d - 3.6), ink, 0.55)
        O._paint(img, O._cov(d - 2.4), rope, 1.0)
    tail = O._polyline(xx, yy, [(c + 19, c + 6), (c + 28, c + 18), (c + 26, c + 30)])
    O._paint(img, O._cov(tail - 3.6), ink, 0.55)
    O._paint(img, O._cov(tail - 2.4), rope, 1.0)
    # the post seen from above: a ringed wooden top
    post = np.hypot(xx - c, yy - c)
    O._paint(img, O._cov(post - 10.0), ink, 0.85)
    O._paint(img, O._cov(post - 8.4), wood, 1.0)
    O._paint(img, O._cov(np.abs(post - 4.5) - 0.7), ink, 0.35)
    # two hoof prints north-west
    for hx, hy in ((c - 26, c - 22), (c - 14, c - 32)):
        d = np.abs(np.hypot(xx - hx, (yy - hy) * 1.15) - 5.0) + (yy > hy + 3) * 99
        O._paint(img, O._cov(d - 1.6), chalk, 0.75)
    return O._save("hitching_spot", img, {"overlay": "hitching_spot", "marking": True})


# ---------------------------------------------------------- world markers
def _g_pod_flight(xx, yy, c):
    """A capsule climbing up-right, three speed lines behind it."""
    ax, ay, bx, by = c - 4, c + 6, c + 12, c - 10
    body = np.abs(O._seg(xx, yy, ax, ay, bx, by) - 9.0)
    nose = O._seg(xx, yy, bx + 4, by - 4, bx + 9, by - 9)
    trail = np.minimum.reduce([O._seg(xx, yy, ax - 10 + o, ay + 10 + o, ax - 10 + o - ln, ay + 10 + o + ln)
                               for o, ln in ((-7, 8), (0, 14), (7, 8))])
    return [np.minimum(trail, nose), body]


def _g_mining_site(xx, yy, c):
    """A pick over an ore outcrop."""
    rock = O._polyline(xx, yy, [(c - 28, c + 26), (c - 20, c + 14), (c - 8, c + 11), (c + 2, c + 16),
                                (c + 14, c + 12), (c + 28, c + 26), (c - 28, c + 26)])
    rock = np.minimum(rock, np.hypot(xx - (c + 12), yy - (c + 20)) - 1.5)
    handle = O._seg(xx, yy, c - 14, c + 8, c + 4, c - 12)
    head = O._arc(xx, yy, c - 12, c + 4, 24, 270, 352)
    return [rock, np.minimum(handle, head)]


def markers():
    t = O._base_tokens()
    return [O._badge_icon("pod_flight", t, _g_pod_flight, (0.98, 0.70, 0.40)),
            O._badge_icon("mining_site", t, _g_mining_site, (0.80, 0.82, 0.62))]


# ------------------------------------------------------------------- icons
ICONS = {
    "stasis_casket": ("furniture", "stasis_casket_occupied_south"),
    "transport_pod": ("structure", "transport_pod"),
    "pod_launcher": ("structure", "pod_launcher_south"),
    "mineral_scanner": ("power", "mineral_scanner_south"),
    "hitching_spot": ("overlay", "hitching_spot"),
}


def icons():
    return [p for p in (O._icon(k, fam, spr) for k, (fam, spr) in ICONS.items()) if p]


# ------------------------------------------------------------------- build
def build(only=None):
    """Every A9 sprite, icon and marker; `only` (casket, pod, launcher,
    scanner, spot, markers) renders just those, without icons."""
    def want(n):
        return not only or n in only
    out = []
    if want("casket"):
        for f in FACINGS:
            out += P.render("stasis_casket_" + f, _casket(False), facing=f, family="furniture",
                            extra={"building": "stasis casket", "state": "empty"})
            out += P.render("stasis_casket_occupied_" + f, _casket(True), facing=f, family="furniture",
                            extra={"building": "stasis casket", "state": "occupied"})
    if want("pod"):
        out += P.render("transport_pod", _pod, family="structure", extra={"building": "transport pod"})
    if want("launcher"):
        for f in FACINGS:
            out += P.render("pod_launcher_" + f, _launcher, facing=f, family="structure",
                            extra={"building": "pod launcher", "fuel": "chemfuel"})
    if want("scanner"):
        for f in FACINGS:
            out += P.render("mineral_scanner_" + f, _scanner, facing=f, family="power",
                            extra={"building": "long-range mineral scanner"})
    if want("spot"):
        out.append(_hitching_spot())
    if want("markers"):
        out += markers()
    if not only:
        out += icons()
    return out
