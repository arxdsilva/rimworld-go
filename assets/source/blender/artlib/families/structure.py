"""Structures (ART spec §12-§13): autotiled walls, doors, floors, frames.

Wall connectivity mask bits: N=1, E=2, S=4, W=8 (neighbour is a wall or
door). 16 sprites per material; connected edges are clipped so adjacent
tiles join without seams. Draw order must be north -> south.
"""
from .. import stage as S
from .linked import _variants, relevant

WALL_H = 1.0
EXT = 0.2          # geometry overshoot past connected edges (clipped away)
INSET = 0.04       # gap to the tile edge on free sides
BITS = (("N", 1), ("E", 2), ("S", 4), ("W", 8))


RIM = 0.13
CORNER_XY = {  # corner -> (sx, sy): which tile corner (+1 east/north, -1 west/south)
    "NE": (1, 1), "SE": (1, -1), "SW": (-1, -1), "NW": (-1, 1)}


def _corner_range(sx, sy):
    """x / y range of the corner INSET square, extended past the tile edge by EXT (clipped)."""
    # reaches 0.1 into the arms so their bevels hide under the (marginally lower) corner top
    xr = (1 - 0.1, 1 + EXT) if sx > 0 else (-EXT, 0.1)
    yr = (-0.1, EXT) if sy > 0 else (-1 - EXT, -1 + 0.1)
    return xr, yr


def _wall(mask, filled, body, cap, trim, kind):
    """Body is a plus of two boxes (centre + arms to every linked side); a
    relevant corner is either filled (corner box to the tile corner, flat top,
    no rim) or notched (INSET square left empty, rim L wrapped around it that
    continues the neighbours' free-edge rims)."""
    con = {d for d, b in BITS if mask & b}
    x0 = -EXT if "W" in con else INSET
    x1 = 1 + EXT if "E" in con else 1 - INSET
    y1 = EXT if "N" in con else -INSET
    y0 = -1 - EXT if "S" in con else -1 + INSET
    top = WALL_H - 0.05
    if con & {"E", "W"} or not con & {"N", "S"}:
        S.box("body", x0, -1 + INSET, 0.0, x1, -INSET, top, body, bevel=0.02)
    if con & {"N", "S"}:
        S.box("body_v", INSET, y0, 0.0, 1 - INSET, y1, top, body, bevel=0.02)
    rel = relevant(mask)
    for name in rel:
        if name in filled:
            (cx0, cx1), (cy0, cy1) = _corner_range(*CORNER_XY[name])
            S.box("corner_" + name, cx0, cy0, 0.0, cx1, cy1, top - 0.001, body, bevel=0.02)
    # raised rim on free edges only; connected edges stay open so runs read continuous
    if "N" not in con:
        S.box("rim_n", x0, y1 - RIM, top - 0.02, x1, y1, WALL_H, cap, bevel=0.02)
    if "S" not in con:
        S.box("rim_s", x0, y0, top - 0.02, x1, y0 + RIM, WALL_H, cap, bevel=0.02)
    if "W" not in con:
        S.box("rim_w", x0, y0, top - 0.02, x0 + RIM, y1, WALL_H, cap, bevel=0.02)
    if "E" not in con:
        S.box("rim_e", x1 - RIM, y0, top - 0.02, x1, y1, WALL_H, cap, bevel=0.02)
    for name in rel:
        if name in filled:
            continue
        sx, sy = CORNER_XY[name]
        # vertical leg: along the N/S arm's side edge, reaching past the tile edge
        xs = (1 - INSET - RIM, 1 - INSET) if sx > 0 else (INSET, INSET + RIM)
        ys = (-1 - EXT, -1 + INSET + RIM) if sy < 0 else (-INSET - RIM, EXT)
        S.box("rimv_" + name, xs[0], ys[0], top - 0.02, xs[1], ys[1], WALL_H, cap, bevel=0.02)
        # horizontal leg: along the E/W arm's side edge
        xh = (-EXT, INSET + RIM) if sx < 0 else (1 - INSET - RIM, 1 + EXT)
        yh = (-1 + INSET, -1 + INSET + RIM) if sy < 0 else (-INSET - RIM, -INSET)
        S.box("rimh_" + name, xh[0], yh[0], top - 0.02, xh[1], yh[1], WALL_H, cap, bevel=0.02)
    if "S" not in con:  # visible front face: horizontal seams (+ block joints / panel rivets)
        for z in (0.30, 0.60):
            S.box("seam", x0 + 0.02, y0 - 0.012, z, x1 - 0.02, y0 + 0.01, z + 0.03, trim, bevel=0.0)
        if kind == "stone":
            for i, (zl, zh) in enumerate(((0.0, 0.30), (0.33, 0.60), (0.63, WALL_H - 0.06))):
                for xs in ((0.35, 0.72) if i % 2 == 0 else (0.18, 0.55, 0.9)):
                    if x0 < xs < x1:
                        S.box("joint", xs - 0.012, y0 - 0.012, zl, xs + 0.012, y0 + 0.01, zh, trim, bevel=0.0)
        elif kind == "steel":
            if x0 < 0.5 < x1:   # one panel joint per tile, riveted seams
                S.box("joint", 0.5 - 0.014, y0 - 0.012, 0.0, 0.5 + 0.014, y0 + 0.01, WALL_H - 0.06, trim, bevel=0.0)
            for z in (0.15, 0.45, 0.78):
                for xs in (0.1, 0.3, 0.7, 0.9):
                    if x0 + 0.04 < xs < x1 - 0.04:
                        S.box("rivet", xs - 0.018, y0 - 0.02, z - 0.018, xs + 0.018, y0 + 0.01, z + 0.018,
                              "steel", bevel=0.006)
    return con


def _door(axis, open_, frame, leaf):
    """axis 'ew': door spans east-west (walls left/right, passage N-S)."""
    def b(name, x0, y0, z0, x1, y1, z1, m, bevel=0.02):
        if axis == "ew":
            S.box(name, x0, y0, z0, x1, y1, z1, m, bevel=bevel)
        else:  # swap x and y
            # rotate 90 deg about the tile centre: (x, y) -> (-y - 0.5 + 0.5, x - 1) mapping
            # u = x in [0,1] along the door  -> y' = -u ; v = y in [-1,0] across -> x' = v + 1
            S.box(name, y0 + 1, -x1, z0, y1 + 1, -x0, z1, m, bevel=bevel)
    # posts/frame connect to the neighbouring walls
    b("post_w", -EXT, -0.96, 0.0, 0.12, -0.04, WALL_H, frame)
    b("post_e", 0.88, -0.96, 0.0, 1 + EXT, -0.04, WALL_H, frame)
    b("sill", 0.10, -0.62, 0.0, 0.90, -0.38, 0.03, "metal_dark", bevel=0.005)
    if open_:
        b("leaf_w", 0.10, -0.57, 0.02, 0.24, -0.43, WALL_H - 0.16, leaf)
        b("leaf_e", 0.76, -0.57, 0.02, 0.90, -0.43, WALL_H - 0.16, leaf)
    else:
        b("leaf_w", 0.10, -0.57, 0.02, 0.50, -0.43, WALL_H - 0.16, leaf)
        b("leaf_e", 0.50, -0.57, 0.02, 0.90, -0.43, WALL_H - 0.16, leaf)
        b("handle_w", 0.42, -0.60, 0.42, 0.47, -0.40, 0.56, "metal_dark", bevel=0.005)
        b("handle_e", 0.53, -0.60, 0.42, 0.58, -0.40, 0.56, "metal_dark", bevel=0.005)
    return ("W", "E") if axis == "ew" else ("N", "S")


def _floor_stone():
    # Flagstones on a 2x2 grid per tile with a staggered second row.
    for r in range(-1, 3):
        y1 = -r / 2
        y0 = y1 - 0.5
        off = 0.25 if r % 2 else 0.0
        for k in range(-1, 3):
            x0 = k / 2 + off
            S.box("flag", x0 + 0.018, y0 + 0.018, -0.035, x0 + 0.5 - 0.018, y1 - 0.018, 0.0,
                  "stone_neutral" if (r + k) % 3 else "gravel", bevel=0.02)
    S.box("under", -1, -2, -0.08, 2, 1, -0.035, "stone_neutral_dark", bevel=0.0)


def _floor_wood():
    # Planks run east-west, period 1 tile in both axes; overshoot is cropped.
    rows = 5
    offs = (0.0, 0.4, 0.8, 0.2, 0.6)
    for r in range(-1, rows + 1):
        y1 = -r / rows
        y0 = y1 - 1 / rows
        off = offs[r % rows]
        for k in range(-1, 2):
            x0 = k + off
            S.box("plank", x0 + 0.006, y0 + 0.006, -0.03, x0 + 1 - 0.006, y1 - 0.006, 0.0,
                  "wood_plank", bevel=0.012)
    S.box("under", -1, -2, -0.08, 2, 1, -0.03, "bark", bevel=0.0)


def _frame():
    # Construction scaffold (blueprint -> frame -> object, ART §12).
    for i, (x, y) in enumerate([(0.12, -0.12), (0.88, -0.12), (0.12, -0.88), (0.88, -0.88)]):
        S.box("post%d" % i, x - 0.05, y - 0.05, 0.0, x + 0.05, y + 0.05, 0.75, "wood", bevel=0.01)
    for z in (0.08, 0.70):
        S.box("bn", 0.07, -0.17, z, 0.93, -0.07, z + 0.06, "wood_plank", bevel=0.01)
        S.box("bs", 0.07, -0.93, z, 0.93, -0.83, z + 0.06, "wood_plank", bevel=0.01)
        S.box("bw", 0.07, -0.93, z, 0.17, -0.07, z + 0.06, "wood_plank", bevel=0.01)
        S.box("be", 0.83, -0.93, z, 0.93, -0.07, z + 0.06, "wood_plank", bevel=0.01)
    S.box("brace", 0.14, -0.90, 0.10, 0.86, -0.82, 0.16, "wood", bevel=0.01)
    S.box("stack", 0.30, -0.60, 0.0, 0.70, -0.36, 0.14, "wood_plank", bevel=0.02)


WALLS = (("wood", "wood_plank", "wood", "bark", "wood"),
         ("stone", "stone_neutral", "stone_neutral", "stone_neutral_dark", "stone"),
         ("steel", "steel", "steel", "metal_dark", "steel"))


def _tilt(n0, pivot, rx=0.0, ry=0.0, rz=0.0):
    """Rotate the objects made since index n0 (degrees, about a pivot point)."""
    import math
    from mathutils import Matrix
    m = (Matrix.Translation(pivot) @ Matrix.Rotation(math.radians(rz), 4, "Z")
         @ Matrix.Rotation(math.radians(ry), 4, "Y") @ Matrix.Rotation(math.radians(rx), 4, "X")
         @ Matrix.Translation((-pivot[0], -pivot[1], -pivot[2])))
    for ob in S.objects()[n0:]:
        ob.matrix_world = m @ ob.matrix_world


def _pod_shell():
    """A crashed drop pod, broken open: charred ground, a torn hull plate with an
    orange stripe, the cracked porthole ring, a bent strut and loose scrap.
    Low and walkable: everything stays under knee height."""
    S.cyl("scorch", 0.5, -0.5, 0.47, 0.0, 0.012, "pod_scorch", segs=22, bevel=0.0, ry=0.40)
    n0 = len(S.objects())
    S.box("plate", 0.12, -0.78, 0.04, 0.60, -0.46, 0.09, "pod_hull", bevel=0.02)
    S.box("plate_stripe", 0.12, -0.64, 0.085, 0.60, -0.56, 0.10, "pod_paint", bevel=0.0)
    S.box("plate_rib", 0.30, -0.78, 0.09, 0.34, -0.46, 0.14, "pod_hull", bevel=0.01)
    _tilt(n0, (0.36, -0.62, 0.05), rx=-9, ry=7, rz=-14)
    n0 = len(S.objects())
    S.cyl("port_ring", 0.70, -0.40, 0.17, 0.04, 0.12, "metal_dark", segs=14, bevel=0.015)
    S.cyl("port_glass", 0.70, -0.40, 0.125, 0.12, 0.135, "glass", segs=14, bevel=0.0)
    S.cyl("port_core", 0.70, -0.40, 0.045, 0.135, 0.145, "pod_scorch", segs=8, bevel=0.0)
    _tilt(n0, (0.70, -0.40, 0.06), rx=12, ry=-10)
    n0 = len(S.objects())
    S.box("shard", 0.58, -0.30, 0.03, 0.86, -0.22, 0.07, "pod_hull", bevel=0.01)
    S.box("shard_tip", 0.80, -0.31, 0.03, 0.92, -0.21, 0.065, "pod_scorch", bevel=0.005)
    _tilt(n0, (0.72, -0.26, 0.04), rz=28)
    n0 = len(S.objects())
    S.box("strut", 0.10, -0.26, 0.04, 0.50, -0.20, 0.09, "metal_dark", bevel=0.008)
    S.box("strut_bend", 0.46, -0.26, 0.04, 0.62, -0.20, 0.09, "metal_dark", bevel=0.008)
    _tilt(n0, (0.46, -0.23, 0.05), rz=-16, ry=-8)
    for k, (x, y, r, sd) in enumerate(((0.28, -0.86, 0.07, 1), (0.78, -0.72, 0.06, 2), (0.20, -0.44, 0.05, 3),
                                       (0.84, -0.56, 0.05, 4), (0.52, -0.90, 0.045, 5))):
        S.blob("scrap%d" % k, x, y, 0.0, r, "pod_hull" if k % 2 == 0 else "pod_scorch", seed=sd, lumps=0.3,
               squash=0.6)
    return S.render_asset("structure", "pod_shell", zmax=0.22,
                          extra={"building": "pod shell", "walkable": True, "salvage": "steel"})


def build():
    out = []
    for mat, body, cap, trim, kind in WALLS:
        for mask in range(16):
            for filled, suffix in _variants(mask):
                con = _wall(mask, filled, body, cap, trim, kind)
                bits = int(suffix[2:], 16) if suffix else 0
                out.append(S.render_asset("structure", "wall_%s_%02d%s" % (mat, mask, suffix), zmax=WALL_H,
                                          clip=tuple(con), extra={"autotile": "wall", "mask": mask,
                                                                  "corners": bits, "material": mat}))
    for mat, frame, leaf in (("wood", "wood", "wood_plank"), ("steel", "metal_dark", "steel")):
        for axis in ("ew", "ns"):
            for open_ in (False, True):
                con = _door(axis, open_, frame, leaf)
                out.append(S.render_asset("structure", "door_%s_%s_%s" % (mat, axis, "open" if open_ else "closed"),
                                          zmax=WALL_H, clip=con,
                                          extra={"axis": axis, "state": "open" if open_ else "closed"}))
    _floor_wood()
    out.append(S.render_asset("structure", "floor_wood", zmax=0.0, margins=(0, 0, 0, 0),
                              outline=False, shadow=False, extra={"tileable": True}))
    _floor_stone()
    out.append(S.render_asset("structure", "floor_stone", zmax=0.0, margins=(0, 0, 0, 0),
                              outline=False, shadow=False, extra={"tileable": True}))
    _frame()
    out.append(S.render_asset("structure", "frame_1x1", zmax=0.76, extra={"stage": "frame"}))
    out.append(_pod_shell())
    return out
