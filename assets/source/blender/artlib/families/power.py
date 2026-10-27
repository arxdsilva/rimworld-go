"""Power family (PLAY-021; ART spec §7, §12, §14): conduits, the power
switch, the four generators, the battery, the heater and the cooler. Our
own designs: nothing traced or reconstructed.

Sprites go to the `power` family (@2x). Geometry is authored facing south
and turned with stage.face().

- power_conduit_<NN>: the visible conduit, a linked 16-mask family (N=1,
  E=2, S=4, W=8; the fence recipe: an arm to each linked edge, clipped
  flush). Low and thin so it stays subtle on the map; no sun shadow.
- hidden_conduit_<NN>: the buried conduit's power-overlay marking, flat and
  near-white so the overlay can multiply it by the net's colour: a dashed
  run to each linked edge (a dash centred on every tile edge, so runs join)
  and a node. Metadata `net_colour: true`.
- power_switch_on / power_switch_off: the lever leans back (on) or forward
  (off); the lens colour only repeats it.
- wood_fired_generator (2x2): firebox and log stack, boiler, smokestack,
  alternator behind a flywheel. `smoke` attachment at the stack top.
- solar_panel (3x3): a rack of nine modules tilted to the south.
- geothermal_generator (2x2): a bolted collar over the steam vent, the
  turbine drum, an exhaust stack (`steam` attachment) and a cabinet.
- battery_<facing> (1x2): a finned cabinet; `gauge0` (empty end) and
  `gauge1` (full end) mark the charge slot on its lid.
- heater (1x1): heating elements behind a guard, a dial and a lamp.
- cooler_<facing> (1x1): stands in a wall gap and joins the walls either
  side like a door (clip). Cold side = facing: blue louvres on that face and
  half of the top; hot side: a round exhaust fan.
- wind_turbine_<facing> (1x2): footing, mast and generator cabinet turn
  with the facing; the nacelle on the mast yaws into the wind, so it is
  always drawn nose to the viewer. The rotor is its own facing-independent
  layer, wind_turbine_blades_01..08: three tapered blades (orange tips) and
  a spinner, turned 15 degrees per frame in the rotor plane; the rotor
  repeats every 120 degrees, so the eight frames loop. Draw a frame with its
  `hub` attachment on the base's `rotor` attachment, after the base (the
  blades overhang the tile); advance frames with the wind.

Emissive parts (indicators, screens, the firebox, heating elements, stove
rings) are drawn dark in the sprite. `<name>_glow` (same canvas, origin and
footprint) holds only those parts lit, with a soft halo, for the renderer to
draw on top, unlit by the light grid, while the device is powered or
working. The sprite's metadata names the layer in `glow:` and its main
colour (a palette emissive) in `emissive:`.

Attachments are canvas pixels (@2x), like the pawn hand points.

    make art FAMILIES=power
"""
import math
import os
import tempfile

import numpy as np
from mathutils import Matrix, Vector

from .. import contracts as C
from .. import meta, post
from .. import stage as S
from .linked import BITS

FAMILY = "power"
FACINGS = ("south", "east", "north", "west")
EXT = 0.2              # overshoot past linked edges (clipped away)


# ------------------------------------------------------------ emissive
def glow(ob, lit):
    """Mark ob as an emissive part: its own (dark) material in the sprite,
    the palette emissive `lit` in the glow layer."""
    ob["rwg_glow"] = lit
    return ob


def glow_colours():
    """The emissive colours staged, in creation order (the first is the
    sprite's main `emissive`)."""
    out = []
    for ob in S.objects():
        lit = ob.get("rwg_glow")
        if lit and lit not in out:
            out.append(lit)
    return out


def glow_name(name):
    """<base>_glow, keeping a facing suffix last: battery_glow_south."""
    for f in FACINGS:
        if name.endswith("_" + f):
            return name[:-len(f) - 1] + "_glow_" + f
    return name + "_glow"


def _light_up():
    hold = []
    for ob in S.objects():
        lit = ob.get("rwg_glow")
        if lit:
            for s in ob.material_slots:
                s.material = S.mat(lit)
        else:
            hold.append(ob)
    S.holdout(hold)


def halo(path, alpha=0.45, passes=5):
    """Soft halo of the emissive pixels around themselves (same canvas)."""
    img = S._read_png(path)
    a = img[..., 3]
    ba = post.box_blur(a, passes)
    bc = post.box_blur(img[..., :3] * a[..., None], passes) / np.maximum(ba, 1e-6)[..., None]
    ha = np.clip(ba * 1.8, 0.0, 1.0) * alpha
    out = np.zeros_like(img)
    oa = a + ha * (1.0 - a)
    safe = np.maximum(oa, 1e-6)[..., None]
    out[..., :3] = (img[..., :3] * a[..., None] + bc * (ha * (1.0 - a))[..., None]) / safe
    out[..., 3] = oa
    S.write_png(out, path)
    return path


def glow_layer(family, name, footprint, zmax, of, lit, clip=(), margins=None):
    """Render the staged scene's emissive parts alone (the rest holds them
    out) as `name`, the glow layer of sprite `of`."""
    _light_up()
    p = S.render_asset(family, name, footprint=footprint, zmax=zmax, margins=margins, outline=False,
                       tint=False, shadow_kind="none", clip=clip,
                       extra={"layer": "glow", "of": of, "emissive": lit})
    if S._read_png(p)[..., 3].max() <= 0.0:
        raise ValueError("%s: every emissive part is hidden in this facing; add one on a face it shows" % name)
    return halo(p)


# ----------------------------------------------------------- projection
def origin(zmax, margins=None):
    """The canvas origin render_asset derives for zmax and margins."""
    ml, _, mt, _ = margins or (C.DEFAULT_MARGIN,) * 4
    return int(round(ml * C.PPT)), int(math.ceil((C.SHEAR_K * zmax + mt) * C.PPT))


def project(pt, zmax, margins=None):
    """Blender point (x, y, z) -> canvas pixel of a sprite rendered with zmax."""
    ox, oy = origin(zmax, margins)
    x, y, z = pt
    return [round(ox + x * C.PPT, 1), round(oy + (-y - C.SHEAR_K * z) * C.PPT, 1)]


def turn(pt, facing, footprint):
    """Where stage.face(.., facing, footprint) moves the authored point pt."""
    w, h = footprint
    deg = S.FACINGS[facing]
    nw, nh = (h, w) if deg in (90, -90) else (w, h)
    m = (Matrix.Translation((nw / 2, -nh / 2, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-w / 2, h / 2, 0)))
    v = m @ Vector(pt)
    return (v.x, v.y, v.z)


def render(name, builder, facing=None, extra=None, shadow_kind="box", clip=(), family=FAMILY,
           margins=None, after=None):
    """Build, face and render one sprite (+ its glow layer when any part
    glows). builder() returns (footprint, zmax[, {attachment: (x, y, z)}]);
    after(facing), if given, adds geometry once faced and returns more
    attachments, already in faced coordinates."""
    ret = builder()
    fp0, zmax = ret[0], ret[1]
    pts = ret[2] if len(ret) > 2 else {}
    lits = glow_colours()
    fp = S.face(S.objects(), facing, fp0) if facing else fp0
    pts = {k: turn(p, facing, fp0) if facing else p for k, p in pts.items()}
    if after:
        pts.update(after(facing))
    ex = {"rotatable": facing is not None, "facing": facing or "south"}
    ex.update(extra or {})
    if pts:
        ex["attachments"] = {k: project(p, zmax, margins) for k, p in pts.items()}
    gname = glow_name(name)
    if lits:
        ex["emissive"] = lits[0]
        ex["glow"] = "%s/%s" % (family, gname)
    out = [S.render_asset(family, name, footprint=fp, zmax=zmax, margins=margins, extra=ex,
                          shadow_kind=shadow_kind, clip=clip)]
    if lits:
        builder()
        if facing:
            S.face(S.objects(), facing, fp0)
        if after:
            after(facing)
        out.append(glow_layer(family, gname, fp, zmax, "%s/%s" % (family, name), lits[0], clip, margins))
    return out


# -------------------------------------------------------------- helpers
def _spin(objs, pivot, axis, deg):
    """Rotate objs by deg about the axis ("X", "Y" or "Z") through pivot."""
    m = (Matrix.Translation(pivot) @ Matrix.Rotation(math.radians(deg), 4, axis)
         @ Matrix.Translation((-pivot[0], -pivot[1], -pivot[2])))
    S.transform(objs, m)


def _since(n0):
    return S.objects()[n0:]


def _rod(name, p0, p1, r, material, segs=14, bevel=0.01):
    """A round bar from p0 to p1 (any direction)."""
    a, b = Vector(p0), Vector(p1)
    d = b - a
    ob = S.cyl(name, 0, 0, r, 0.0, d.length, material, segs=segs, bevel=bevel)
    ob.matrix_world = Matrix.Translation(a) @ d.to_track_quat("Z", "Y").to_matrix().to_4x4()
    return ob


def _bolts(cx, cy, r, z0, z1, n, rb=0.022, material="metal_dark", phase=0.0):
    for k in range(n):
        a = phase + k / n * 2 * math.pi
        S.cyl("bolt", cx + r * math.cos(a), cy + r * math.sin(a), rb, z0, z1, material, segs=6, bevel=0.0)


def _hazard(x0, x1, y0, y1, z0, z1, n=4):
    """A yellow plate with dark bars (a warning band), facing south."""
    S.box("hazard", x0, y0, z0, x1, y1, z1, "corn_yellow", bevel=0.004)
    step = (x1 - x0) / (2 * n)
    for k in range(n):
        xa = x0 + step * (2 * k + 0.5)
        S.box("hazard_bar", xa, y0 - 0.004, z0 + 0.008, xa + step, y1, z1 - 0.008, "fur_black", bevel=0.0)


# ------------------------------------------------------------- conduits
CONDUIT_H = 0.075


def _conduit(mask):
    """Low steel conduit: an arm to each linked edge, a clamp on each arm,
    and a fitting at the centre unless the run is straight."""
    con = {d for d, b in BITS if mask & b}
    cx, cy, r = 0.5, -0.5, 0.045
    pipe = "steel_dark"
    if con in ({"N", "S"}, {"E", "W"}):
        if con == {"N", "S"}:
            S.box("pipe", cx - r, -1 - EXT, 0.0, cx + r, EXT, CONDUIT_H, pipe, bevel=0.03)
        else:
            S.box("pipe", -EXT, cy - r, 0.0, 1 + EXT, cy + r, CONDUIT_H, pipe, bevel=0.03)
    else:
        arms = {"N": (cx - r, cy, cx + r, EXT), "S": (cx - r, -1 - EXT, cx + r, cy),
                "E": (cx, cy - r, 1 + EXT, cy + r), "W": (-EXT, cy - r, cx, cy + r)}
        for d in sorted(con):
            S.box("pipe_" + d, *arms[d][:2], 0.0, *arms[d][2:], CONDUIT_H, pipe, bevel=0.03)
    if con == {"N", "S"}:                                            # one clamp per straight tile
        S.box("clamp", cx - 0.075, cy - 0.03, 0.0, cx + 0.075, cy + 0.03, CONDUIT_H + 0.02, "metal_dark", bevel=0.01)
    elif con == {"E", "W"}:
        S.box("clamp", cx - 0.03, cy - 0.075, 0.0, cx + 0.03, cy + 0.075, CONDUIT_H + 0.02, "metal_dark", bevel=0.01)
    elif len(con) == 2:                                              # elbow
        S.box("elbow", cx - 0.08, cy - 0.08, 0.0, cx + 0.08, cy + 0.08, CONDUIT_H + 0.01, pipe, bevel=0.035)
    elif len(con) != 2:                                              # end, tee, cross or a lone box
        if not con:
            S.box("stub", cx - 0.24, cy - r, 0.0, cx + 0.24, cy + r, CONDUIT_H, pipe, bevel=0.03)
        S.box("box", cx - 0.11, cy - 0.11, 0.0, cx + 0.11, cy + 0.11, 0.12, "metal_dark", bevel=0.025)
        S.box("lid", cx - 0.085, cy - 0.085, 0.12, cx + 0.085, cy + 0.085, 0.135, "steel", bevel=0.015)
        S.cyl("terminal", cx, cy, 0.025, 0.135, 0.15, "copper", segs=8, bevel=0.0)
    return con


# ---------------------------------------------------- hidden conduit (2D)
HC_INK = (0.93, 0.93, 0.90)      # near white: the overlay multiplies the net colour
HC_EDGE = (0.07, 0.07, 0.08)


def _cov(d, w=1.0):
    return np.clip(0.5 - d / w, 0.0, 1.0)


def _paint(img, cov, rgb, alpha=1.0):
    a = cov * alpha
    out_a = a + img[..., 3] * (1 - a)
    safe = np.maximum(out_a, 1e-6)
    for i in range(3):
        img[..., i] = (rgb[i] * a + img[..., i] * img[..., 3] * (1 - a)) / safe
    img[..., 3] = out_a


def _hidden_conduit(mask):
    """Flat overlay marking: dashed runs to the linked edges and a node."""
    n = C.PPT
    yy, xx = np.mgrid[0:n, 0:n].astype(np.float32) + 0.5
    c = n / 2
    half_w, period, dash = 5.0, 22.0, 14.0
    d = np.full((n, n), 1e9, np.float32)
    for name, bit in BITS:
        if not mask & bit:
            continue
        if name in ("N", "S"):
            across = np.abs(xx - c)
            t = yy if name == "N" else n - yy                       # distance from the edge
            inside = (yy <= c) if name == "N" else (yy >= c)
        else:
            across = np.abs(yy - c)
            t = n - xx if name == "E" else xx
            inside = (xx >= c) if name == "E" else (xx <= c)
        e = np.abs(np.mod(t + period / 2, period) - period / 2)    # dashes centred on t = 0, 22, 44 ...
        arm = np.maximum(across - half_w, e - dash / 2)
        d = np.minimum(d, np.where(inside, arm, 1e9))
    node = np.hypot(xx - c, yy - c) - (9.0 if mask else 8.0)
    d = np.minimum(d, node)
    if not mask:
        d = np.minimum(d, np.abs(np.hypot(xx - c, yy - c) - 17.0) - 2.5)
    img = np.zeros((n, n, 4), np.float32)
    _paint(img, _cov(d - 2.5), HC_EDGE, 0.75)      # dark rim: contrast on any ground
    _paint(img, _cov(d), HC_INK)
    name = "hidden_conduit_%02d" % mask
    path = S.out_path(FAMILY, name)
    S.write_png(img, path)
    meta.write(FAMILY, name, {"image": "generated/world/%s/%s.png" % (FAMILY, name), "scale": C.SCALE,
                              "canvas": [n, n], "footprint": [1, 1], "origin_px": [0, 0],
                              "shadow": {"kind": "none", "w": 0.0, "h": 0.0, "d": 0.0},
                              "autotile": "hidden_conduit", "mask": mask, "overlay": "power",
                              "net_colour": True})
    return path


# ---------------------------------------------------------------- switch
def _switch(on):
    def build():
        S.box("plate", 0.24, -0.80, 0.0, 0.76, -0.30, 0.04, "steel_dark", bevel=0.012)
        S.box("body", 0.32, -0.68, 0.04, 0.68, -0.38, 0.40, "metal_dark", bevel=0.03)
        S.box("cap", 0.30, -0.70, 0.40, 0.70, -0.36, 0.45, "steel", bevel=0.015)
        _hazard(0.36, 0.64, -0.69, -0.675, 0.14, 0.32, n=3)
        for x in (0.42, 0.58):                                     # pivot cheeks on the cap
            S.box("cheek", x - 0.025, -0.60, 0.45, x + 0.025, -0.50, 0.52, "steel_dark", bevel=0.008)
        n0 = len(S.objects())
        S.box("lever", 0.475, -0.57, 0.47, 0.525, -0.53, 0.74, "steel", bevel=0.012)
        S.ellipsoid("knob", 0.5, -0.55, 0.76, 0.055, 0.055, 0.05, "pod_paint", segs=14, rings=8)
        _spin(_since(n0), (0.5, -0.55, 0.49), "X", -32 if on else 58)   # + leans south
        S.ellipsoid("lens", 0.62, -0.42, 0.47, 0.035, 0.035, 0.03, "circuit" if on else "med_red", segs=12, rings=6)
        return (1, 1), 0.78
    return build


# ------------------------------------------------------------ generators
WOOD_STACK = (0.40, -0.36, 1.66)


def _wood_generator():
    """2 x 2 wood-fired generator. Firebox (front-west) with a fire window
    and a log stack, upright boiler and smokestack behind it, an engine
    cylinder driving the alternator (east) behind a flywheel."""
    S.box("skid", 0.06, -1.94, 0.0, 1.94, -0.06, 0.08, "steel_dark", bevel=0.02)
    for k, (x, y) in enumerate(((0.12, -0.12), (1.88, -0.12), (0.12, -1.88), (1.88, -1.88))):
        S.cyl("skid_bolt", x, y, 0.025, 0.08, 0.10, "metal_dark", segs=6, bevel=0.0)
    # boiler and stack
    S.cyl("boiler", 0.62, -0.60, 0.42, 0.08, 1.00, "steel", segs=28, bevel=0.03)
    for z in (0.30, 0.62, 0.90):
        S.cyl("band", 0.62, -0.60, 0.432, z, z + 0.05, "brass", segs=28, bevel=0.0)
    S.ellipsoid("dome", 0.62, -0.60, 1.00, 0.42, 0.42, 0.14, "steel", segs=28, rings=10)
    S.cyl("valve", 0.82, -0.52, 0.035, 1.04, 1.16, "brass", segs=8, bevel=0.005)
    S.cyl("stack", WOOD_STACK[0], WOOD_STACK[1], 0.085, 0.95, 1.60, "metal_dark", segs=14, bevel=0.01)
    S.cyl("stack_cap", WOOD_STACK[0], WOOD_STACK[1], 0.12, 1.56, WOOD_STACK[2], "metal_dark", segs=14,
          bevel=0.01)
    # firebox: the window is the working glow
    S.box("firebox", 0.12, -1.66, 0.08, 1.02, -0.98, 0.72, "metal_dark", bevel=0.04)
    S.box("firebox_top", 0.10, -1.68, 0.72, 1.04, -0.96, 0.78, "steel_dark", bevel=0.02)
    S.box("door", 0.30, -1.70, 0.16, 0.88, -1.65, 0.66, "steel_dark", bevel=0.015)
    glow(S.box("window", 0.36, -1.715, 0.24, 0.82, -1.69, 0.58, "ash_char", bevel=0.005), "hive_glow")
    glow(S.box("ash_slot", 0.38, -1.70, 0.10, 0.80, -1.66, 0.14, "ash_char", bevel=0.004), "gland_ember")
    for x in (0.48, 0.59, 0.70):
        S.box("bar", x - 0.013, -1.725, 0.24, x + 0.013, -1.71, 0.58, "metal_dark", bevel=0.0)
    S.box("latch", 0.84, -1.735, 0.38, 0.94, -1.70, 0.43, "steel", bevel=0.008)
    # logs ready beside the door (west of it), end grain to the viewer
    for k, (x, z) in enumerate(((0.11, 0.13), (0.23, 0.13), (0.17, 0.24))):
        _rod("log", (x, -1.90, z), (x, -1.62, z), 0.058, "bark", segs=10)
        _rod("log_end", (x, -1.91, z), (x, -1.90, z), 0.047, "wood", segs=10, bevel=0.0)
    # engine cylinder to the alternator
    _rod("piston", (1.02, -1.30, 0.46), (1.22, -1.30, 0.46), 0.12, "steel", segs=16)
    # alternator: axis north-south, cooling rings, copper windings band
    _rod("alternator", (1.48, -1.62, 0.44), (1.48, -0.36, 0.44), 0.30, "steel_dark", segs=24, bevel=0.03)
    for y in (-0.55, -0.75, -1.35, -1.52):
        _rod("fin", (1.48, y - 0.025, 0.44), (1.48, y + 0.025, 0.44), 0.325, "metal_dark", segs=24, bevel=0.0)
    _rod("winding", (1.48, -1.18, 0.44), (1.48, -0.92, 0.44), 0.318, "copper", segs=24, bevel=0.0)
    # flywheel in front: rim, recess, hub and spokes
    _rod("flywheel", (1.48, -1.80, 0.44), (1.48, -1.70, 0.44), 0.37, "metal_dark", segs=32, bevel=0.02)
    _rod("recess", (1.48, -1.81, 0.44), (1.48, -1.79, 0.44), 0.28, "steel_dark", segs=32, bevel=0.0)
    n0 = len(S.objects())
    for k in range(4):
        S.box("spoke", 1.48 - 0.026, -1.82, 0.44, 1.48 + 0.026, -1.795, 0.44 + 0.27, "steel", bevel=0.006)
        _spin(_since(n0 + k), (1.48, -1.8, 0.44), "Y", 45 + 90 * k)
    _rod("hub", (1.48, -1.86, 0.44), (1.48, -1.78, 0.44), 0.07, "steel", segs=14)
    # control box on the alternator: gauge and the run lamp
    S.box("control", 1.26, -0.56, 0.68, 1.72, -0.30, 0.94, "steel", bevel=0.025)
    S.cyl("gauge", 1.40, -0.43, 0.085, 0.94, 0.955, "ceramic", segs=18, bevel=0.0)
    S.box("needle", 1.395, -0.44, 0.955, 1.47, -0.42, 0.962, "med_red", bevel=0.0)
    glow(S.cyl("run_lamp", 1.62, -0.43, 0.045, 0.94, 0.975, "backdrop", segs=12, bevel=0.0), "indicator")
    return (2, 2), WOOD_STACK[2] + 0.02, {"smoke": WOOD_STACK}


def _solar_panel():
    """3 x 3 solar array: nine modules on a rack tilted 16 degrees to the
    south, an inverter on the front-east leg."""
    tilt, z_front, y_front, y_back = 16.0, 0.42, -2.88, -0.12
    rise = (y_back - y_front) * math.tan(math.radians(tilt))
    for x in (0.16, 1.5, 2.84):
        S.box("leg_f", x - 0.04, -2.78, 0.0, x + 0.04, -2.70, z_front + 0.04, "steel", bevel=0.01)
        S.box("leg_b", x - 0.04, -0.30, 0.0, x + 0.04, -0.22, z_front + rise - 0.05, "steel", bevel=0.01)
        S.box("foot", x - 0.08, -2.82, 0.0, x + 0.08, -2.66, 0.04, "steel_dark", bevel=0.01)
    S.box("rail", 0.10, -2.78, 0.20, 2.90, -2.72, 0.25, "steel", bevel=0.01)
    n0 = len(S.objects())
    S.box("backing", 0.06, y_front, z_front, 2.94, y_back, z_front + 0.05, "steel", bevel=0.015)
    mw, mh, gap = (2.88 - 4 * 0.04) / 3, (2.76 - 4 * 0.04) / 3, 0.04
    for i in range(3):
        for j in range(3):
            x0 = 0.06 + gap + i * (mw + gap)
            y0 = y_front + gap + j * (mh + gap)
            S.box("module", x0, y0, z_front + 0.05, x0 + mw, y0 + mh, z_front + 0.08, "jacket_navy", bevel=0.006)
            for k in (1, 2):                                       # cell seams
                xs = x0 + mw * k / 3
                S.box("seam", xs - 0.011, y0 + 0.03, z_front + 0.08, xs + 0.011, y0 + mh - 0.03,
                      z_front + 0.083, "mug_blue", bevel=0.0)
            ys = y0 + mh / 2
            S.box("seam", x0 + 0.03, ys - 0.011, z_front + 0.08, x0 + mw - 0.03, ys + 0.011, z_front + 0.083,
                  "mug_blue", bevel=0.0)
    _spin(_since(n0), (0.0, y_front, z_front), "X", tilt)
    # inverter under the front edge, run lamp on its face
    S.box("inverter", 2.40, -2.96, 0.04, 2.80, -2.80, 0.34, "steel_dark", bevel=0.02)
    S.box("inverter_fins", 2.44, -2.965, 0.08, 2.62, -2.955, 0.30, "metal_dark", bevel=0.0)
    glow(_rod("inv_lamp", (2.71, -2.975, 0.24), (2.71, -2.955, 0.24), 0.03, "backdrop", segs=10, bevel=0.0),
         "indicator")
    S.box("cable", 2.58, -2.80, 0.0, 2.64, -2.70, 0.05, "fur_black", bevel=0.01)
    return (3, 3), z_front + rise + 0.12


GEO_STACK = (1.56, -0.42, 1.60)


def _geothermal():
    """2 x 2 geothermal generator over a steam vent: a bolted collar, the
    turbine drum with a copper heat-exchange band, an exhaust stack, a
    valve wheel and a control cabinet."""
    S.cyl("collar", 1.0, -1.0, 0.88, 0.0, 0.14, "steel_dark", segs=36, bevel=0.03)
    _bolts(1.0, -1.0, 0.80, 0.14, 0.165, 16)
    S.cyl("drum", 0.94, -0.94, 0.50, 0.14, 0.92, "steel", segs=32, bevel=0.04)
    for z in (0.30, 0.70):
        S.cyl("drum_band", 0.94, -0.94, 0.512, z, z + 0.05, "metal_dark", segs=32, bevel=0.0)
    S.cyl("exchanger", 0.94, -0.94, 0.52, 0.44, 0.58, "copper", segs=32, bevel=0.01)
    S.ellipsoid("dome", 0.94, -0.94, 0.92, 0.50, 0.50, 0.18, "steel", segs=32, rings=12)
    S.cyl("dome_cap", 0.94, -0.94, 0.10, 1.04, 1.12, "steel_dark", segs=14)
    # exhaust stack (north-east) with a warning band, fed from the drum
    S.cyl("stack", GEO_STACK[0], GEO_STACK[1], 0.13, 0.14, 1.52, "steel_dark", segs=18)
    S.cyl("stack_band", GEO_STACK[0], GEO_STACK[1], 0.138, 0.30, 0.40, "corn_yellow", segs=18, bevel=0.0)
    S.cyl("stack_lip", GEO_STACK[0], GEO_STACK[1], 0.17, 1.50, GEO_STACK[2], "metal_dark", segs=18)
    _rod("feed", (1.28, -0.70, 0.66), (1.50, -0.46, 0.66), 0.07, "copper", segs=12)
    # valve wheel on a pipe to the east
    _rod("pipe_e", (1.42, -1.20, 0.30), (1.86, -1.20, 0.30), 0.06, "copper", segs=12)
    _rod("valve_stem", (1.70, -1.20, 0.30), (1.70, -1.20, 0.52), 0.02, "steel", segs=8)
    S.cyl("valve_wheel", 1.70, -1.20, 0.10, 0.52, 0.56, "med_red", segs=18, bevel=0.01)
    S.cyl("valve_hub", 1.70, -1.20, 0.035, 0.52, 0.58, "steel", segs=8, bevel=0.0)
    # control cabinet (south-west): gauge, two lamps and a light strip
    S.box("cabinet", 0.10, -1.92, 0.0, 0.66, -1.48, 0.70, "steel_dark", bevel=0.03)
    S.box("cabinet_top", 0.08, -1.94, 0.70, 0.68, -1.46, 0.74, "steel", bevel=0.015)
    glow(S.box("strip", 0.16, -1.935, 0.58, 0.60, -1.915, 0.63, "backdrop", bevel=0.004), "indicator")
    _rod("dial", (0.28, -1.94, 0.38), (0.28, -1.92, 0.38), 0.09, "ceramic", segs=20, bevel=0.0)
    S.box("dial_needle", 0.275, -1.945, 0.38, 0.34, -1.94, 0.395, "med_red", bevel=0.0)
    for x in (0.48, 0.56):
        glow(_rod("lamp", (x, -1.94, 0.40), (x, -1.92, 0.40), 0.03, "backdrop", segs=10, bevel=0.0), "indicator")
    _hazard(0.16, 0.60, -1.935, -1.92, 0.10, 0.20, n=4)
    return (2, 2), GEO_STACK[2] + 0.02, {"steam": GEO_STACK}


# ----------------------------------------------------------- the turbine
# A three-blade rotor on a nacelle. The nacelle yaws into the wind, so it is
# always drawn facing the viewer (south) whatever the base's facing; only
# the footing, mast and cabinet turn. The rotor is a separate layer.
HUB_Z = 1.62               # hub height
HUB_DY = 0.30              # hub ahead (south) of the mast axis
ROTOR_R = 0.70
ROTOR_FRAMES = 8
ROTOR_STEP = 15            # degrees per frame; 8 x 15 = 120, one blade pitch
BASE_ZMAX = HUB_Z + 0.14
ROTOR_ZMAX = HUB_Z + ROTOR_R + 0.04
ROTOR_MARGINS = (0.35, 0.35, C.DEFAULT_MARGIN, C.DEFAULT_MARGIN)   # blade tips overhang the tile


def _turbine_base():
    """1 x 2 wind turbine base: a footing and mast on the north tile, the
    generator cabinet on the south tile, a conduit between them."""
    S.box("footing", 0.12, -0.88, 0.0, 0.88, -0.12, 0.10, "stone", bevel=0.02)
    _bolts(0.5, -0.5, 0.26, 0.10, 0.125, 6)
    S.cyl("flange", 0.5, -0.5, 0.17, 0.10, 0.17, "metal_dark", segs=18, bevel=0.01)
    S.cyl("mast", 0.5, -0.5, 0.085, 0.17, HUB_Z - 0.06, "steel", r_top=0.055, segs=16, bevel=0.0)
    for z in (0.40, 0.62, 0.84, 1.06):                               # climbing rungs on the south side
        S.box("rung", 0.46, -0.60, z, 0.54, -0.57, z + 0.025, "metal_dark", bevel=0.0)
    S.box("conduit", 0.455, -1.20, 0.0, 0.545, -0.80, CONDUIT_H, "steel_dark", bevel=0.03)
    S.box("cabinet", 0.16, -1.86, 0.0, 0.84, -1.18, 0.54, "steel_dark", bevel=0.04)
    S.box("roof", 0.13, -1.89, 0.54, 0.87, -1.15, 0.60, "steel", bevel=0.02)
    for z in (0.14, 0.24, 0.34):
        S.box("louvre", 0.24, -1.875, z, 0.62, -1.855, z + 0.05, "metal_dark", bevel=0.004)
    glow(S.cyl("lamp", 0.70, -1.34, 0.045, 0.60, 0.635, "backdrop", segs=12, bevel=0.0), "indicator")
    _hazard(0.20, 0.80, -1.885, -1.87, 0.46, 0.52, n=4)
    return (1, 2), BASE_ZMAX


def _nacelle(facing):
    """The nacelle on top of the (turned) mast, nose to the south; returns
    the rotor attachment point."""
    mx, my, _ = turn((0.5, -0.5, 0.0), facing, (1, 2))
    S.box("nacelle", mx - 0.11, my - HUB_DY + 0.07, HUB_Z - 0.11, mx + 0.11, my + 0.24, HUB_Z + 0.10, "steel",
          bevel=0.05)
    S.box("nacelle_vent", mx - 0.07, my - 0.02, HUB_Z + 0.09, mx + 0.07, my + 0.16, HUB_Z + 0.125, "steel_dark",
          bevel=0.015)
    S.cyl("yaw_ring", mx, my, 0.075, HUB_Z - 0.14, HUB_Z - 0.09, "steel_dark", segs=16, bevel=0.0)
    return {"rotor": (mx, my - HUB_DY, HUB_Z)}


def _blade(name, angle, hub):
    """One tapered blade from the hub, in the rotor plane (x-z), at angle."""
    n0 = len(S.objects())
    r = ROTOR_R
    S.prism(name, [(0.07, -0.065), (r, -0.025), (r, 0.03), (0.22, 0.09), (0.07, 0.08)], 0.0, 0.035,
            "med_white", bevel=0.01)
    S.prism(name + "_tip", [(r - 0.12, -0.031), (r, -0.025), (r, 0.03), (r - 0.12, 0.039)], -0.004, 0.039,
            "pod_paint", bevel=0.004)
    m = Matrix.Translation(hub) @ Matrix.Rotation(math.radians(angle), 4, "Y") @ Matrix.Rotation(
        math.radians(90), 4, "X")
    S.transform(_since(n0), m)


def _rotor(frame):
    """The rotor alone (spinner and three blades) in front of its mast
    tile's axis, turned frame x 15 degrees."""
    def build():
        hub = (0.5, -0.5 - HUB_DY, HUB_Z)
        for k in range(3):
            _blade("blade%d" % k, frame * ROTOR_STEP + k * 120 + 90, hub)
        S.ellipsoid("spinner", hub[0], hub[1] - 0.06, hub[2], 0.085, 0.12, 0.085, "steel", segs=16, rings=10)
        _rod("hub", (hub[0], hub[1] + 0.06, hub[2]), (hub[0], hub[1] - 0.02, hub[2]), 0.10, "steel_dark", segs=18)
        return (1, 1), ROTOR_ZMAX, {"hub": hub}
    return build


# ------------------------------------------------------- store and climate
def _battery():
    """1 x 2 battery: finned cabinet, terminals at the north end, the charge
    slot along the lid and a status lamp at the south end."""
    S.box("plinth", 0.08, -1.92, 0.0, 0.92, -0.08, 0.06, "metal_dark", bevel=0.015)
    S.box("body", 0.13, -1.87, 0.06, 0.87, -0.13, 0.60, "steel_dark", bevel=0.04)
    for k in range(7):
        y = -0.30 - k * 0.22
        for x0, x1 in ((0.10, 0.135), (0.865, 0.90)):
            S.box("fin", x0, y - 0.03, 0.10, x1, y + 0.03, 0.54, "metal_dark", bevel=0.006)
    S.box("lid", 0.14, -1.86, 0.60, 0.86, -0.14, 0.65, "steel", bevel=0.02)
    for x, cap in ((0.32, "med_red"), (0.68, "fur_black")):
        S.cyl("terminal", x, -0.32, 0.07, 0.65, 0.71, "metal_dark", segs=14, bevel=0.01)
        S.cyl("terminal_cap", x, -0.32, 0.05, 0.71, 0.77, cap, segs=14, bevel=0.012)
    S.box("gauge", 0.44, -1.64, 0.65, 0.56, -0.52, 0.662, "backdrop", bevel=0.004)
    glow(S.cyl("status", 0.5, -1.76, 0.04, 0.65, 0.68, "backdrop", segs=12, bevel=0.0), "indicator")
    _hazard(0.20, 0.80, -1.885, -1.87, 0.32, 0.46, n=4)
    return (1, 2), 0.80, {"gauge0": (0.5, -1.62, 0.662), "gauge1": (0.5, -0.54, 0.662)}


def _heater():
    """1 x 1 electric heater: cabinet on feet, three heating elements behind
    a guard on the front, a thermostat dial and a lamp on top."""
    for x, y in ((0.24, -0.38), (0.76, -0.38), (0.24, -0.70), (0.76, -0.70)):
        S.box("foot", x - 0.04, y - 0.04, 0.0, x + 0.04, y + 0.04, 0.06, "metal_dark", bevel=0.01)
    S.box("body", 0.18, -0.76, 0.06, 0.82, -0.32, 0.72, "steel", bevel=0.05)
    S.box("top", 0.16, -0.78, 0.72, 0.84, -0.30, 0.76, "steel_dark", bevel=0.02)
    for k in range(6):                                               # radiator fins across the top
        x = 0.23 + k * 0.095
        S.box("fin", x, -0.70, 0.76, x + 0.04, -0.36, 0.84, "steel_dark", bevel=0.012)
    S.box("recess", 0.24, -0.775, 0.14, 0.76, -0.755, 0.62, "ash_char", bevel=0.008)
    for z in (0.22, 0.34, 0.46):
        glow(S.box("element", 0.28, -0.785, z, 0.72, -0.765, z + 0.06, "steel_dark", bevel=0.012), "gland_ember")
    for x in (0.35, 0.45, 0.55, 0.65):
        S.box("guard", x - 0.012, -0.80, 0.16, x + 0.012, -0.785, 0.60, "metal_dark", bevel=0.0)
    S.box("panel", 0.30, -0.785, 0.64, 0.70, -0.755, 0.71, "steel_dark", bevel=0.006)
    _rod("dial", (0.40, -0.80, 0.675), (0.40, -0.78, 0.675), 0.045, "ceramic", segs=16, bevel=0.0)
    S.box("dial_mark", 0.395, -0.805, 0.675, 0.43, -0.80, 0.70, "med_red", bevel=0.0)
    glow(_rod("lamp", (0.60, -0.80, 0.675), (0.60, -0.78, 0.675), 0.025, "backdrop", segs=10, bevel=0.0),
         "indicator")
    S.box("cable", 0.46, -0.32, 0.0, 0.54, -0.20, 0.05, "fur_black", bevel=0.01)
    return (1, 1), 0.86


COOLER_H = 0.92


def _fan(cx, cy, z, r, axis):
    """A round fan: rim, dark well, four blades and a hub. axis 'Z' lies on
    a top face, 'Y' stands on a north face (opening north)."""
    if axis == "Z":
        S.cyl("fan_rim", cx, cy, r, z - 0.04, z + 0.03, "pod_paint", segs=24, bevel=0.008)
        S.cyl("fan_well", cx, cy, r - 0.035, z - 0.03, z + 0.034, "metal_dark", segs=24, bevel=0.0)
        n0 = len(S.objects())
        for k in range(4):
            S.box("fan_blade", cx, cy - 0.022, z + 0.03, cx + r - 0.05, cy + 0.022, z + 0.045, "steel_dark",
                  bevel=0.005)
            _spin(_since(n0 + k), (cx, cy, 0), "Z", 20 + 90 * k)
        S.cyl("fan_hub", cx, cy, 0.035, z + 0.03, z + 0.055, "steel", segs=10, bevel=0.0)
    else:
        _rod("fan_rim", (cx, cy - 0.02, z), (cx, cy + 0.04, z), r, "pod_paint", segs=24, bevel=0.008)
        _rod("fan_well", (cx, cy - 0.01, z), (cx, cy + 0.032, z), r - 0.035, "metal_dark", segs=24, bevel=0.0)
        n0 = len(S.objects())
        for k in range(4):
            S.box("fan_blade", cx - 0.022, cy + 0.025, z, cx + 0.022, cy + 0.04, z + r - 0.05, "steel_dark",
                  bevel=0.005)
            _spin(_since(n0 + k), (cx, cy, z), "Y", 20 + 90 * k)
        _rod("fan_hub", (cx, cy + 0.02, z), (cx, cy + 0.05, z), 0.035, "steel", segs=10, bevel=0.0)


def _cooler():
    """1 x 1 cooler in a wall gap, authored facing south: the cold side
    (south) has blue louvres on its face and on the south half of the top;
    the hot side (north) a round exhaust fan on its face and on the top.
    Steel posts bridge to the walls west and east."""
    S.box("post_w", -EXT, -0.96, 0.0, 0.12, -0.04, 1.0, "metal_dark", bevel=0.02)
    S.box("post_e", 0.88, -0.96, 0.0, 1 + EXT, -0.04, 1.0, "metal_dark", bevel=0.02)
    S.box("housing", 0.10, -0.92, 0.0, 0.90, -0.08, COOLER_H, "steel", bevel=0.04)
    # cold face: a louvred panel with a blue frame
    S.box("cold_frame", 0.16, -0.935, 0.12, 0.84, -0.915, 0.80, "med_blue", bevel=0.01)
    S.box("cold_well", 0.20, -0.94, 0.16, 0.80, -0.92, 0.76, "metal_dark", bevel=0.0)
    for z in (0.20, 0.32, 0.44, 0.56, 0.68):
        S.box("cold_slat", 0.20, -0.955, z, 0.80, -0.925, z + 0.05, "ice", bevel=0.008)
    # top: cold half (south) slats, hot half (north) the exhaust fan
    for y in (-0.84, -0.74, -0.64):
        S.box("top_slat", 0.20, y - 0.03, COOLER_H - 0.01, 0.80, y + 0.03, COOLER_H + 0.025, "med_blue", bevel=0.008)
    _fan(0.5, -0.30, COOLER_H + 0.01, 0.17, "Z")
    # hot face (north): the exhaust fan, seen when the cooler faces north
    _fan(0.5, -0.08, 0.46, 0.30, "Y")
    glow(S.cyl("lamp", 0.80, -0.52, 0.035, COOLER_H, COOLER_H + 0.03, "backdrop", segs=12, bevel=0.0), "indicator")
    return (1, 1), 1.0


# ------------------------------------------------- PLAY-024 radio and beacon
RADIO_MAST = (1.80, -0.26, 1.62)   # antenna tip (lamp)
RADIO_PANEL = (-0.56, 0.64)        # the sloped panel's front-bottom edge (y, z)
RADIO_SLOPE = 52                   # degrees the panel leans back from upright


def _radio_console():
    """2 x 1 radio console, authored facing south (the operator stands on
    the tile south of it): a steel desk on two drawer pedestals, an enamel
    radio set on its back half with a screen tracing a green waveform
    (glow), a lit frequency strip and dials, a speaker, a handset on its
    cradle, and an antenna mast at the north-east corner whose tip lamp
    (glow) shows in every facing."""
    for x0, x1 in ((0.10, 0.52), (1.48, 1.90)):                     # drawer pedestals
        S.box("pedestal", x0, -0.86, 0.0, x1, -0.16, 0.46, "steel_dark", bevel=0.025)
        for z in (0.12, 0.30):
            S.box("drawer", x0 + 0.05, -0.875, z, x1 - 0.05, -0.855, z + 0.12, "steel", bevel=0.008)
            S.box("pull", (x0 + x1) / 2 - 0.05, -0.89, z + 0.05, (x0 + x1) / 2 + 0.05, -0.87, z + 0.07,
                  "metal_dark", bevel=0.0)
    S.box("modesty", 0.52, -0.30, 0.10, 1.48, -0.18, 0.46, "metal_dark", bevel=0.01)
    S.box("desk", 0.06, -0.92, 0.46, 1.94, -0.10, 0.52, "steel", bevel=0.02)
    # the set: an enamel cabinet whose front panel slopes back, so the
    # screen, frequency strip and dials face the operator and the sky
    S.box("set_lip", 0.24, -0.58, 0.52, 1.34, -0.30, RADIO_PANEL[1], "radio_enamel", bevel=0.02)
    S.box("set_back", 0.24, -0.34, 0.52, 1.34, -0.14, 0.98, "radio_enamel", bevel=0.03)
    S.box("set_lid", 0.22, -0.36, 0.98, 1.36, -0.12, 1.02, "steel_dark", bevel=0.015)
    for x in (0.40, 0.52, 0.64, 0.76):
        S.box("vent", x, -0.32, 1.02, x + 0.06, -0.16, 1.03, "metal_dark", bevel=0.0)
    yp, zp = RADIO_PANEL
    n0 = len(S.objects())
    S.box("panel", 0.24, yp, zp, 1.34, yp + 0.26, zp + 0.30, "radio_enamel", bevel=0.02)
    S.box("bezel", 0.30, yp - 0.012, zp + 0.03, 0.86, yp + 0.01, zp + 0.27, "metal_dark", bevel=0.01)
    S.box("screen", 0.34, yp - 0.02, zp + 0.05, 0.82, yp, zp + 0.25, "backdrop", bevel=0.004)
    wave = [(0.36, 0.15), (0.42, 0.15), (0.46, 0.22), (0.51, 0.08), (0.56, 0.23), (0.61, 0.09), (0.66, 0.20),
            (0.70, 0.15), (0.80, 0.15)]
    for k in range(len(wave) - 1):
        (x0, z0), (x1, z1) = wave[k], wave[k + 1]
        glow(_rod("wave%d" % k, (x0, yp - 0.026, zp + z0), (x1, yp - 0.026, zp + z1), 0.02, "circuit", segs=6,
                  bevel=0.0), "radio_screen_glow")
    glow(S.box("freq", 0.92, yp - 0.018, zp + 0.19, 1.28, yp, zp + 0.26, "backdrop", bevel=0.004), "lamp_warm")
    for k, x in enumerate((0.98, 1.10, 1.22)):
        _rod("dial%d" % k, (x, yp - 0.04, zp + 0.11), (x, yp, zp + 0.11), 0.045, "ceramic", segs=14, bevel=0.006)
        S.box("dial_mark%d" % k, x - 0.006, yp - 0.045, zp + 0.11, x + 0.006, yp - 0.04, zp + 0.155, "fur_black",
              bevel=0.0)
    for k, x in enumerate((0.98, 1.10)):
        glow(_rod("led%d" % k, (x, yp - 0.03, zp + 0.035), (x, yp, zp + 0.035), 0.02, "backdrop", segs=8,
                  bevel=0.0), "indicator")
    _spin(_since(n0), (0.0, yp, zp), "X", -RADIO_SLOPE)
    # speaker on the desk, east of the set
    S.box("speaker", 1.42, -0.50, 0.52, 1.72, -0.20, 0.80, "steel_dark", bevel=0.025)
    for z in (0.58, 0.63, 0.68, 0.73):
        S.box("grille", 1.47, -0.51, z, 1.67, -0.495, z + 0.025, "metal_dark", bevel=0.0)
    # handset on its cradle at the front west of the desk, cord to the set
    S.box("cradle", 0.12, -0.84, 0.52, 0.42, -0.62, 0.56, "metal_dark", bevel=0.01)
    S.box("handset", 0.14, -0.78, 0.56, 0.40, -0.70, 0.60, "fur_black", bevel=0.025)
    for x in (0.15, 0.39):
        S.ellipsoid("cup", x, -0.74, 0.60, 0.045, 0.055, 0.03, "fur_black", segs=12, rings=6)
    _rod("cord", (0.27, -0.70, 0.56), (0.30, -0.52, 0.60), 0.012, "fur_black", segs=6, bevel=0.0)
    # a notepad by the operator
    S.box("pad", 0.64, -0.84, 0.52, 0.82, -0.68, 0.53, "numbroot_label", bevel=0.004)
    # antenna mast at the north-east corner: guy collar, two crossbars, lamp
    mx, my, top = RADIO_MAST
    S.box("mast_foot", mx - 0.07, my - 0.07, 0.52, mx + 0.07, my + 0.07, 0.58, "metal_dark", bevel=0.01)
    S.cyl("mast", mx, my, 0.025, 0.58, top - 0.04, "steel", r_top=0.016, segs=10, bevel=0.0)
    for z, half in ((1.20, 0.16), (1.40, 0.11)):
        _rod("crossbar", (mx - half, my, z), (mx + half, my, z), 0.012, "steel", segs=6, bevel=0.0)
    _rod("feedline", (mx, my - 0.02, 0.60), (1.30, -0.30, 0.90), 0.010, "fur_black", segs=6, bevel=0.0)
    glow(S.ellipsoid("tip", mx, my, top, 0.04, 0.04, 0.04, "med_red", segs=12, rings=8), "sentry_lamp_glow")
    return (2, 1), top + 0.06


BEACON_TOP = 1.24


def _trade_beacon():
    """1 x 1 trade beacon: a low concrete pad, a slim steel mast with a
    small dish tilted at the sky, a junction box, and a caged lamp on top
    (glow; the renderer blinks it). Slim so the tile stays walkable."""
    cx, cy = 0.5, -0.5
    S.box("pad", 0.30, -0.70, 0.0, 0.70, -0.30, 0.06, "stone", bevel=0.02)
    _bolts(cx, cy, 0.15, 0.06, 0.08, 4, phase=math.pi / 4)
    S.cyl("collar", cx, cy, 0.06, 0.06, 0.14, "metal_dark", segs=12, bevel=0.01)
    S.cyl("mast", cx, cy, 0.042, 0.14, BEACON_TOP - 0.10, "steel", r_top=0.032, segs=12, bevel=0.0)
    for z in (0.40, 0.70):                                            # hazard bands
        S.cyl("band", cx, cy, 0.038, z, z + 0.06, "corn_yellow", segs=12, bevel=0.0)
    # junction box on the mast's south side
    S.box("junction", cx - 0.08, cy - 0.10, 0.20, cx + 0.08, cy - 0.03, 0.36, "steel_dark", bevel=0.015)
    S.box("junction_lid", cx - 0.06, cy - 0.105, 0.30, cx + 0.06, cy - 0.10, 0.33, "metal_dark", bevel=0.0)
    # dish on an arm to the west, facing up and south-west
    n0 = len(S.objects())
    S.cyl("dish", 0.0, 0.0, 0.15, -0.02, 0.02, "med_white", segs=24, r_top=0.13, bevel=0.01)
    S.cyl("dish_well", 0.0, 0.0, 0.11, 0.015, 0.03, "steel", segs=24, bevel=0.0)
    S.cyl("feed_horn", 0.0, 0.0, 0.016, 0.02, 0.12, "metal_dark", segs=8, bevel=0.0)
    S.cyl("horn_tip", 0.0, 0.0, 0.03, 0.12, 0.15, "steel_dark", segs=10, bevel=0.0)
    tilt = (Matrix.Translation((0.28, -0.50, 0.86)) @ Matrix.Rotation(math.radians(-35), 4, "Z")
            @ Matrix.Rotation(math.radians(-30), 4, "X"))
    S.transform(_since(n0), tilt)
    _rod("dish_arm", (cx, cy, 0.84), (0.30, -0.50, 0.84), 0.016, "steel_dark", segs=8, bevel=0.0)
    # the lamp: a dome lens open to the sky inside a cage of four bars
    S.cyl("lamp_base", cx, cy, 0.085, BEACON_TOP - 0.12, BEACON_TOP - 0.08, "metal_dark", segs=14, bevel=0.01)
    glow(S.ellipsoid("lens", cx, cy, BEACON_TOP - 0.08, 0.075, 0.075, 0.10, "pod_paint", segs=16, rings=10),
         "beacon_glow")
    for k in range(4):
        a = math.pi / 4 + k * math.pi / 2
        S.cyl("cage", cx + 0.08 * math.cos(a), cy + 0.08 * math.sin(a), 0.009, BEACON_TOP - 0.08,
              BEACON_TOP + 0.03, "metal_dark", segs=6, bevel=0.0)
    S.cyl("lamp_knob", cx, cy, 0.018, BEACON_TOP + 0.015, BEACON_TOP + 0.04, "metal_dark", segs=8, bevel=0.0)
    return (1, 1), BEACON_TOP + 0.08


def build_play024():
    """The radio console (four facings) and the trade beacon."""
    out = []
    for f in FACINGS:
        out += render("radio_console_" + f, _radio_console, facing=f, extra={"building": "radio console"})
    out += render("trade_beacon", _trade_beacon, extra={"building": "trade beacon"})
    return out


# ---------------------------------------------------------------- build
def _clip_for(facing):
    return ("W", "E") if facing in ("south", "north") else ("N", "S")


def _cooler_shadow(facing):
    w, d = (1.0, 0.84) if facing in ("south", "north") else (0.84, 1.0)
    return {"kind": "box", "w": w, "h": 1.0, "d": d}


def build():
    out = []
    for mask in range(16):
        con = _conduit(mask)
        out.append(S.render_asset(FAMILY, "power_conduit_%02d" % mask, zmax=0.16, clip=tuple(sorted(con)),
                                  shadow_kind="none",
                                  extra={"autotile": "conduit", "mask": mask, "building": "power conduit"}))
        out.append(_hidden_conduit(mask))
    for on in (True, False):
        out += render("power_switch_" + ("on" if on else "off"), _switch(on),
                      extra={"building": "power switch", "state": "on" if on else "off"})
    out += render("wood_fired_generator", _wood_generator, extra={"building": "wood-fired generator"})
    out += render("solar_panel", _solar_panel, extra={"building": "solar panel"})
    out += render("geothermal_generator", _geothermal, extra={"building": "geothermal generator",
                                                              "over": "steam vent"})
    for f in FACINGS:
        out += render("wind_turbine_" + f, _turbine_base, facing=f, after=_nacelle,
                      extra={"building": "wind turbine", "rotor_layer": "power/wind_turbine_blades",
                             "frames": ROTOR_FRAMES})
        out += render("battery_" + f, _battery, facing=f, extra={"building": "battery"})
        out += render("cooler_" + f, _cooler, facing=f, clip=_clip_for(f),
                      extra={"building": "cooler", "cold_side": f, "in_wall": True, "shadow": _cooler_shadow(f)})
    for i in range(ROTOR_FRAMES):
        out += render("wind_turbine_blades_%02d" % (i + 1), _rotor(i), shadow_kind="none", margins=ROTOR_MARGINS,
                      extra={"layer": "rotor", "of": "power/wind_turbine", "frame": i + 1,
                             "frames": ROTOR_FRAMES, "step_deg": ROTOR_STEP})
    out += render("heater", _heater, extra={"building": "heater"})
    out += build_play024()
    return out


# ------------------------------------------------------------ previews
def turbine_composite(facing="south", frame=1):
    """The turbine base with one rotor frame on it (a temp PNG for the
    Architect icon); None until both are rendered."""
    base, blades = S.out_path(FAMILY, "wind_turbine_" + facing), S.out_path(FAMILY, "wind_turbine_blades_%02d" % frame)
    if not (os.path.exists(base) and os.path.exists(blades)):
        return None
    b, r = S._read_png(base), S._read_png(blades)
    mx, my, _ = turn((0.5, -0.5, 0.0), facing, (1, 2))
    rotor = project((mx, my - HUB_DY, HUB_Z), BASE_ZMAX)
    hub = project((0.5, -0.5 - HUB_DY, HUB_Z), ROTOR_ZMAX, ROTOR_MARGINS)
    dx, dy = int(round(rotor[0] - hub[0])), int(round(rotor[1] - hub[1]))
    x0, y0 = min(0, dx), min(0, dy)
    w = max(b.shape[1], dx + r.shape[1]) - x0
    h = max(b.shape[0], dy + r.shape[0]) - y0
    out = np.zeros((h, w, 4), np.float32)
    for img, ox, oy in ((b, -x0, -y0), (r, dx - x0, dy - y0)):
        dst = out[oy:oy + img.shape[0], ox:ox + img.shape[1]]
        a = img[..., 3:4]
        oa = a + dst[..., 3:4] * (1 - a)
        dst[..., :3] = (img[..., :3] * a + dst[..., :3] * dst[..., 3:4] * (1 - a)) / np.maximum(oa, 1e-6)
        dst[..., 3:4] = oa
    path = os.path.join(tempfile.gettempdir(), "rwg_wind_turbine_preview.png")
    S.write_png(out, path)
    return path
