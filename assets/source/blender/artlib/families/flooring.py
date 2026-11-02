"""Flooring family (AX, A3 rows; ART spec §8, §13): the vanilla floors the
Architect gained, plus their Architect icons. Our own designs: nothing
traced or reconstructed.

Floors follow structure.py's floor rules: structure/floor_<name>, one
128 x 128 tile, tileable with a period of one tile (geometry overshoots
the tile and is cropped), no outline, no shadow, no normal map (floors are
lit flat), drawn hard-edged under zones and furniture. Every pattern is
low contrast so pawns and furniture stay on top (§13).

- floor_straw_matting: a basket weave of straw strips, 2x2 blocks a tile.
- floor_concrete: one poured slab a tile, thin joints, a few pores.
- floor_paved_tile: small pavers in a running bond, two clay tones.
- floor_steel_tile: one tread plate a tile, raised bars, corner rivets.
- floor_silver_tile: four silver plates a tile, each with a set diamond.
- floor_gold_tile: one gold plate a tile, an inset border and a boss.
- floor_sterile_tile: a 4x4 grid of small white clinic tiles.
- floor_stone_tile_<granite|limestone|marble|sandstone|slate>: cut stone,
  one look per stone (granite: speckled slabs; limestone: 2x2 squares;
  marble: veined slabs in a bond; sandstone: layered ashlar; slate: long
  split slabs at uneven heights). Drawn in the stone neutrals: the
  renderer multiplies the stone's tint (stuff colour), as for floor_stone.
- floor_flagstone_<stone>: irregular laid stones in mortar, one layout per
  stone (a periodic jittered grid, so it tiles), stone neutrals as above.
- floor_carpet: one wall-to-wall loop pile in near-white neutrals; the
  renderer multiplies the carpet's colour over the whole tile (one texture
  for every carpet colour, no mask).

Icons ui_icon/<name> are cut from the floors: stone tile and flagstone
from the granite floor with the granite tint baked in, the carpet with a
rust preset (the HUD draws icons untinted).

    make art FAMILIES=flooring
"""
import math

from .. import stage as S
from . import decor as D
from . import overlay as O
from . import power as P

STONES = ("granite", "limestone", "marble", "sandstone", "slate")
NEUTRAL, DARK = "stone_neutral", "stone_neutral_dark"


def _h(*k):
    """Deterministic hash in [0, 1) of integers k (visual only)."""
    v = 0.0
    for i, x in enumerate(k):
        v += x * (12.9898, 78.233, 37.719, 4.581)[i % 4] + i * 1.618
    v = math.sin(v) * 43758.5453
    return v - math.floor(v)


def _copies():
    """Tile offsets the pattern is repeated at; the render crops to (0, 0)."""
    return [(ox, oy) for ox in (-1, 0, 1) for oy in (-1, 0, 1)]


def _under(material, depth=0.08):
    S.box("under", -1, -2, -depth, 2, 1, -depth + 0.02, material, bevel=0.0)


def _grid(n, gap, material, z0=-0.04, bevel=0.02, alt=None, segments=1):
    """An n x n grid of square tiles per tile (grout gap between)."""
    s = 1.0 / n
    for ox, oy in _copies():
        for i in range(n):
            for j in range(n):
                m = alt if alt and (i + j) % 2 else material
                x0, y1 = ox + i * s, oy - j * s
                S.box("t", x0 + gap / 2, y1 - s + gap / 2, z0, x0 + s - gap / 2, y1 - gap / 2, 0.0, m,
                      bevel=bevel, segments=segments)


# ----------------------------------------------------------- the floors
def _straw_matting():
    _under("carpet_straw_dark")
    for ox, oy in _copies():
        for bi in range(2):
            for bj in range(2):
                x0, y1 = ox + bi * 0.5, oy - bj * 0.5
                for k in range(3):
                    a, b = k / 6 + 0.008, (k + 1) / 6 - 0.008
                    if (bi + bj) % 2:      # strips run east-west
                        S.box("s", x0 + 0.01, y1 - b, -0.03, x0 + 0.49, y1 - a, 0.0, "straw", bevel=0.02,
                              segments=2)
                    else:
                        S.box("s", x0 + a, y1 - 0.49, -0.03, x0 + b, y1 - 0.01, 0.0, "straw", bevel=0.02,
                              segments=2)


def _concrete():
    _under("concrete_dark")
    for ox, oy in _copies():
        S.box("slab", ox + 0.006, oy - 0.994, -0.04, ox + 0.994, oy - 0.006, 0.0, "concrete", bevel=0.006)
        for k in range(7):
            x, y, r = 0.08 + 0.84 * _h(1, k), -0.08 - 0.84 * _h(2, k), 0.008 + 0.01 * _h(3, k)
            S.box("pore", ox + x - r, oy + y - r, -0.003, ox + x + r, oy + y + r, 0.001, "concrete_dark",
                  bevel=0.0)


def _paved_tile():
    _under("grout")
    rows, length, gap = 4, 0.5, 0.025
    for ox, oy in _copies():
        for r in range(rows):
            off = 0.25 if r % 2 else 0.0
            y1 = oy - r / rows
            for k in range(-1, 3):
                x0 = ox + off + k * length
                m = "paver_alt" if _h(r, k % 2) > 0.5 else "paver"
                S.box("p", x0 + gap / 2, y1 - 1 / rows + gap / 2, -0.04, x0 + length - gap / 2, y1 - gap / 2, 0.0,
                      m, bevel=0.025)


def _steel_tile():
    _under("metal_dark")
    for ox, oy in _copies():
        S.box("plate", ox + 0.01, oy - 0.99, -0.05, ox + 0.99, oy - 0.01, -0.012, "steel", bevel=0.02)
        for i in range(4):
            for j in range(4):
                cx, cy = ox + (i + 0.5) / 4, oy - (j + 0.5) / 4
                if (i + j) % 2:
                    S.box("bar", cx - 0.07, cy - 0.018, -0.02, cx + 0.07, cy + 0.018, 0.0, "steel", bevel=0.012)
                else:
                    S.box("bar", cx - 0.018, cy - 0.07, -0.02, cx + 0.018, cy + 0.07, 0.0, "steel", bevel=0.012)
        for x, y in ((0.05, -0.05), (0.95, -0.05), (0.05, -0.95), (0.95, -0.95)):
            S.cyl("rivet", ox + x, oy + y, 0.022, -0.02, 0.0, "metal_dark", segs=10, bevel=0.006)


def _silver_tile():
    _under("silver_trim")
    _grid(2, 0.02, "silver", bevel=0.025)
    for ox, oy in _copies():
        for i in range(2):
            for j in range(2):
                S.cyl("diamond", ox + 0.25 + i * 0.5, oy - 0.25 - j * 0.5, 0.11, -0.006, 0.003, "silver_trim",
                      segs=4, bevel=0.0)
                S.cyl("inset", ox + 0.25 + i * 0.5, oy - 0.25 - j * 0.5, 0.075, -0.004, 0.006, "silver",
                      segs=4, bevel=0.0)


def _gold_tile():
    _under("gold_trim")
    for ox, oy in _copies():
        S.box("plate", ox + 0.012, oy - 0.988, -0.05, ox + 0.988, oy - 0.012, 0.0, "gold", bevel=0.025)
        for k, (x0, y0, x1, y1) in enumerate(((0.10, -0.13, 0.90, -0.10), (0.10, -0.90, 0.90, -0.87),
                                              (0.10, -0.90, 0.13, -0.10), (0.87, -0.90, 0.90, -0.10))):
            S.box("border", ox + x0, oy + y0, -0.006, ox + x1, oy + y1, 0.003, "gold_trim", bevel=0.0)
        S.cyl("boss", ox + 0.5, oy - 0.5, 0.13, -0.01, 0.006, "gold_trim", segs=24, bevel=0.004)
        S.cyl("boss_in", ox + 0.5, oy - 0.5, 0.09, -0.005, 0.01, "gold", segs=24, bevel=0.004)


def _sterile_tile():
    _under("sterile_grout")
    _grid(4, 0.014, "sterile", bevel=0.012)


def _stone_tile(stone):
    _under(DARK)
    if stone == "granite":          # one polished slab a tile, speckled
        for ox, oy in _copies():
            S.box("slab", ox + 0.012, oy - 0.988, -0.04, ox + 0.988, oy - 0.012, 0.0, NEUTRAL, bevel=0.03)
            for k in range(16):
                x, y, r = 0.08 + 0.84 * _h(11, k), -0.08 - 0.84 * _h(12, k), 0.007 + 0.009 * _h(13, k)
                S.box("fleck", ox + x - r, oy + y - r, -0.004, ox + x + r, oy + y + r, 0.001, DARK, bevel=0.0)
    elif stone == "limestone":      # 2x2 soft squares
        _grid(2, 0.02, NEUTRAL, bevel=0.04, segments=2)
    elif stone == "marble":         # veined slabs in a half bond
        for ox, oy in _copies():
            for r in range(2):
                off = 0.5 if r else 0.0
                for k in range(-1, 2):
                    x0, y1 = ox + off + k, oy - r * 0.5
                    S.box("slab", x0 + 0.01, y1 - 0.49, -0.04, x0 + 0.99, y1 - 0.01, 0.0, NEUTRAL, bevel=0.02)
            for k in range(3):      # thin veins wandering across the slabs
                n0 = len(S.objects())
                x, y = ox + 0.15 + 0.3 * k, oy - 0.2 - 0.25 * _h(21, k)
                S.box("vein", x - 0.004, y - 0.22, -0.003, x + 0.004, y + 0.22, 0.002, DARK, bevel=0.0)
                P._spin(P._since(n0), (x, y, 0.0), "Z", 25 + 40 * _h(22, k))
    elif stone == "sandstone":      # layered ashlar
        for ox, oy in _copies():
            for r in range(3):
                off = (0.0, 0.25, 0.5)[r]
                y1 = oy - r / 3
                for k in range(-1, 3):
                    x0 = ox + off + k * 0.5
                    S.box("block", x0 + 0.012, y1 - 1 / 3 + 0.012, -0.04, x0 + 0.488, y1 - 0.012, 0.0, NEUTRAL,
                          bevel=0.025)
                    S.box("layer", x0 + 0.04, y1 - 1 / 6 - 0.004, -0.003, x0 + 0.46, y1 - 1 / 6 + 0.004, 0.002,
                          DARK, bevel=0.0)
    elif stone == "slate":          # long split slabs at uneven heights
        offs = (0.0, 0.5, 0.25, 0.75)
        for ox, oy in _copies():
            for r in range(4):
                y1 = oy - r / 4
                for k in range(-1, 2):
                    x0 = ox + offs[r] + k
                    top = -0.02 * _h(31, r)
                    S.box("slab", x0 + 0.008, y1 - 0.25 + 0.008, -0.05, x0 + 0.992, y1 - 0.008, top, NEUTRAL,
                          bevel=0.012)


# flagstone layouts per stone: (grid n, jitter, mortar gap, bevel, segments)
FLAGS = {"granite": (3, 0.10, 0.028, 0.025, 1), "limestone": (3, 0.07, 0.034, 0.045, 2),
         "marble": (2, 0.11, 0.026, 0.03, 1), "sandstone": (3, 0.05, 0.03, 0.03, 1),
         "slate": (4, 0.07, 0.022, 0.012, 1)}


def _flagstone(stone):
    n, jit, gap, bevel, segs = FLAGS[stone]
    salt = STONES.index(stone) * 97

    def vert(i, j):
        a, b = i % n, j % n
        jx = (_h(salt, a, b, 1) - 0.5) * 2 * jit
        jy = (_h(salt, a, b, 2) - 0.5) * 2 * jit
        if stone == "sandstone":    # squarer blocks: rows stay level
            jy *= 0.3
        return i / n + jx, -j / n + jy

    _under(DARK)
    for i in range(-n, 2 * n):
        for j in range(-n, 2 * n):
            pts = [vert(i, j), vert(i, j + 1), vert(i + 1, j + 1), vert(i + 1, j)]   # counter-clockwise
            cx = sum(p[0] for p in pts) / 4
            cy = sum(p[1] for p in pts) / 4
            inset = []
            for x, y in pts:
                dx, dy = cx - x, cy - y
                d = math.hypot(dx, dy) or 1.0
                inset.append((x + dx / d * gap, y + dy / d * gap))
            top = -0.006 * int(3 * _h(salt, i % n, j % n, 3))
            S.prism("flag", inset, -0.05, top, NEUTRAL, bevel=min(bevel, 0.02))


def _carpet():
    _under("carpet_neutral_dark")
    n, gap = 8, 0.014
    s = 1.0 / n
    for ox, oy in _copies():
        for r in range(n):
            off = s / 2 if r % 2 else 0.0
            for k in range(-1, n + 1):
                x0, y1 = ox + off + k * s, oy - r * s
                S.box("loop", x0 + gap / 2, y1 - s + gap / 2, -0.03, x0 + s - gap / 2, y1 - gap / 2, 0.0,
                      "carpet_neutral", bevel=0.035, segments=2)


# ---------------------------------------------------------------- build
FLOORS = {"straw_matting": _straw_matting, "concrete": _concrete, "paved_tile": _paved_tile,
          "steel_tile": _steel_tile, "silver_tile": _silver_tile, "gold_tile": _gold_tile,
          "sterile_tile": _sterile_tile, "carpet": _carpet}
FLOORS.update({"stone_tile_" + s: (lambda s=s: _stone_tile(s)) for s in STONES})
FLOORS.update({"flagstone_" + s: (lambda s=s: _flagstone(s)) for s in STONES})

# ui_icon/<name>: (floor, baked tint or None)
GRANITE = "#695f61"   # base.yaml tints.stone_tint.granite
ICONS = {"straw_matting": ("straw_matting", None), "concrete": ("concrete", None),
         "paved_tile": ("paved_tile", None), "steel_tile": ("steel_tile", None),
         "silver_tile": ("silver_tile", None), "gold_tile": ("gold_tile", None),
         "sterile_tile": ("sterile_tile", None), "stone_tile": ("stone_tile_granite", GRANITE),
         "flagstone": ("flagstone_granite", GRANITE), "carpet_floor": ("carpet", "#a2593a")}


def build(only=None):
    """Every floor and its icon; `only` (FLOORS names) renders just those floors."""
    out = []
    for name, fn in FLOORS.items():
        if only and name not in only:
            continue
        fn()
        extra = {"tileable": True, "floor": name}
        if name.startswith(("stone_tile_", "flagstone_")):
            extra["multiply"] = "stone"      # the stone's tint over the whole tile, no mask
        elif name == "carpet":
            extra["multiply"] = "colour"     # the carpet colour over the whole tile
        out.append(S.render_asset("structure", "floor_" + name, zmax=0.0, margins=(0, 0, 0, 0),
                                  outline=False, shadow=False, extra=extra))
    if only:
        return out
    for icon, (floor, tint) in ICONS.items():
        p = D.tinted_icon(icon, "structure", "floor_" + floor, tint, masked=False) if tint else \
            O._icon(icon, "structure", "floor_" + floor)
        if p:
            out.append(p)
    return out
