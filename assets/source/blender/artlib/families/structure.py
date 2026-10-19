"""Structures (ART spec §12-§13): autotiled walls, doors, floors, frames.

Wall connectivity mask bits: N=1, E=2, S=4, W=8 (neighbour is a wall or
door). 16 sprites per material; connected edges are clipped so adjacent
tiles join without seams. Draw order must be north -> south.
"""
from .. import stage as S

WALL_H = 1.0
EXT = 0.2          # geometry overshoot past connected edges (clipped away)
INSET = 0.04       # gap to the tile edge on free sides
BITS = (("N", 1), ("E", 2), ("S", 4), ("W", 8))


def _wall(mask, body, cap, trim):
    con = {d for d, b in BITS if mask & b}
    x0 = -EXT if "W" in con else INSET
    x1 = 1 + EXT if "E" in con else 1 - INSET
    y1 = EXT if "N" in con else -INSET
    y0 = -1 - EXT if "S" in con else -1 + INSET
    top = WALL_H - 0.05
    S.box("body", x0, y0, 0.0, x1, y1, top, body, bevel=0.02)
    rim = 0.13
    # raised rim on free edges only; connected edges stay open so runs read continuous
    if "N" not in con:
        S.box("rim_n", x0, y1 - rim, top - 0.02, x1, y1, WALL_H, cap, bevel=0.02)
    if "S" not in con:
        S.box("rim_s", x0, y0, top - 0.02, x1, y0 + rim, WALL_H, cap, bevel=0.02)
    if "W" not in con:
        S.box("rim_w", x0, y0, top - 0.02, x0 + rim, y1, WALL_H, cap, bevel=0.02)
    if "E" not in con:
        S.box("rim_e", x1 - rim, y0, top - 0.02, x1, y1, WALL_H, cap, bevel=0.02)
    if not con:
        pass
    if "S" not in con:  # visible front face: horizontal seams (+ block joints for stone)
        for z in (0.30, 0.60):
            S.box("seam", x0 + 0.02, y0 - 0.012, z, x1 - 0.02, y0 + 0.01, z + 0.03, trim, bevel=0.0)
        if body == "stone":
            for i, (zl, zh) in enumerate(((0.0, 0.30), (0.33, 0.60), (0.63, WALL_H - 0.06))):
                for xs in ((0.35, 0.72) if i % 2 == 0 else (0.18, 0.55, 0.9)):
                    if x0 < xs < x1:
                        S.box("joint", xs - 0.012, y0 - 0.012, zl, xs + 0.012, y0 + 0.01, zh, trim, bevel=0.0)
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
                  "stone" if (r + k) % 3 else "gravel", bevel=0.02)
    S.box("under", -1, -2, -0.08, 2, 1, -0.035, "stone_dark", bevel=0.0)


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


def build():
    out = []
    for mat, body, cap, trim in (("wood", "wood_plank", "wood", "bark"), ("stone", "stone", "stone", "stone_dark")):
        for mask in range(16):
            con = _wall(mask, body, cap, trim)
            out.append(S.render_asset("structure", "wall_%s_%02d" % (mat, mask), zmax=WALL_H,
                                      clip=tuple(con), extra={"autotile": "wall", "mask": mask,
                                                              "material": mat}))
    for axis in ("ew", "ns"):
        for open_ in (False, True):
            con = _door(axis, open_, "wood", "wood_plank")
            out.append(S.render_asset("structure", "door_wood_%s_%s" % (axis, "open" if open_ else "closed"),
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
    return out
