"""Fences and fence gates (structure family, own module so a fence render
never rewrites the wall/door sprites).

Fence connectivity mask bits match walls: N=1, E=2, S=4, W=8 (neighbour is a
fence, fence gate, wall or door). Low post-and-rail fences: ~0.55 tall,
two open rails so you can see through. Every tile has a centre post; rails
run from the centre to each connected edge (clipped so tiles join).

Gates: `fence_gate_<facing>` (closed) and `fence_gate_open_<facing>`.
south/north gates span east-west in a fence line, east/west span north-south.
Hinge side: south=W, north=E, east=N, west=N (leaf swings west / east).
"""
from .. import stage as S

FH = 0.68          # post height
EXT = 0.2
BITS = (("N", 1), ("E", 2), ("S", 4), ("W", 8))
FACINGS = ("south", "east", "north", "west")


def _fence(mask, mat):
    con = {d for d, b in BITS if mask & b}
    if mat == "wood":
        post, rail, cap = "wood", "wood_plank", "wood"
        pw, rw = 0.058, 0.026
    else:
        post, rail, cap = "stone_neutral", "stone_neutral", "stone_neutral_dark"
        pw, rw = 0.075, 0.04
    cx, cy = 0.5, -0.5
    S.box("post", cx - pw, cy - pw, 0.0, cx + pw, cy + pw, FH, post, bevel=0.015)
    S.box("cap", cx - pw - 0.012, cy - pw - 0.012, FH - 0.03, cx + pw + 0.012, cy + pw + 0.012, FH + 0.02,
          cap, bevel=0.01)
    for z in (0.12, 0.44):
        h = 0.06 if mat == "wood" else 0.09
        if "E" in con:
            S.box("rail_e", cx, cy - rw, z, 1 + EXT, cy + rw, z + h, rail, bevel=0.008)
        if "W" in con:
            S.box("rail_w", -EXT, cy - rw, z, cx, cy + rw, z + h, rail, bevel=0.008)
        if "N" in con:
            S.box("rail_n", cx - rw, cy, z, cx + rw, EXT, z + h, rail, bevel=0.008)
        if "S" in con:
            S.box("rail_s", cx - rw, -1 - EXT, z, cx + rw, cy, z + h, rail, bevel=0.008)
    return con


def _gate(facing, open_):
    ew = facing in ("south", "north")
    flip = facing == "north"   # hinge on the far end

    def b(name, x0, y0, z0, x1, y1, z1, m, bevel=0.01):
        if flip:                          # mirror along the gate axis
            x0, x1 = 1 - x1, 1 - x0
        if ew:
            S.box(name, x0, y0, z0, x1, y1, z1, m, bevel=bevel)
        else:
            if facing == "west":      # same hinge (N end), leaf swings east
                S.box(name, -y1, -x1, z0, -y0, -x0, z1, m, bevel=bevel)
            else:
                S.box(name, y0 + 1, -x1, z0, y1 + 1, -x0, z1, m, bevel=bevel)

    pw = 0.065
    cy = -0.5
    # fence-end posts, taller than the fence, bridging to neighbours
    b("post_h", 0.02, cy - pw, 0.0, 0.02 + 2 * pw, cy + pw, FH + 0.06, "wood", bevel=0.015)
    b("post_l", 0.98 - 2 * pw, cy - pw, 0.0, 0.98, cy + pw, FH + 0.06, "wood", bevel=0.015)
    b("stub_h", -EXT, cy - 0.03, 0.16, 0.02, cy + 0.03, 0.23, "wood_plank", bevel=0.008)
    b("stub_l", 0.98, cy - 0.03, 0.16, 1 + EXT, cy + 0.03, 0.23, "wood_plank", bevel=0.008)
    b("stub_h2", -EXT, cy - 0.03, 0.36, 0.02, cy + 0.03, 0.43, "wood_plank", bevel=0.008)
    b("stub_l2", 0.98, cy - 0.03, 0.36, 1 + EXT, cy + 0.03, 0.43, "wood_plank", bevel=0.008)
    x0, x1 = 0.02 + 2 * pw, 0.98 - 2 * pw
    if not open_:
        for z in (0.12, 0.44):
            b("rail", x0, cy - 0.03, z, x1, cy + 0.03, z + 0.06, "wood_plank", bevel=0.008)
        for x in (x0, x1 - 0.06):
            b("stile", x, cy - 0.035, 0.08, x + 0.06, cy + 0.035, 0.57, "wood_plank", bevel=0.008)
        b("brace", x0 + 0.06, cy - 0.03, 0.18, x1 - 0.06, cy + 0.03, 0.24, "bark", bevel=0.004)
        b("latch", x1 - 0.02, cy - 0.05, 0.30, x1 + 0.03, cy + 0.05, 0.38, "metal_dark", bevel=0.004)
    else:
        # leaf swung 90 degrees about the hinge post, pointing toward the viewer
        hx = 0.02 + pw
        for z in (0.12, 0.44):
            b("rail", hx - 0.03, cy + pw, z, hx + 0.03, -0.96, z + 0.06, "wood_plank", bevel=0.008)
        for y in (cy + pw + 0.02, -0.90):
            b("stile", hx - 0.035, y, 0.08, hx + 0.035, y + 0.06, 0.57, "wood_plank", bevel=0.008)
    return ("W", "E") if ew else ("N", "S")


def build():
    out = []
    for mat in ("wood", "stone"):
        for mask in range(16):
            con = _fence(mask, mat)
            out.append(S.render_asset("structure", "fence_%s_%02d" % (mat, mask), zmax=FH + 0.03,
                                      clip=tuple(con), shadow_kind="none",
                                      extra={"autotile": "fence", "mask": mask, "material": mat}))
    for open_ in (False, True):
        for facing in FACINGS:
            con = _gate(facing, open_)
            name = "fence_gate_%s%s" % ("open_" if open_ else "", facing)
            out.append(S.render_asset("structure", name, zmax=FH + 0.08, clip=con, shadow_kind="none",
                                      extra={"facing": facing, "state": "open" if open_ else "closed",
                                             "material": "wood"}))
    return out
