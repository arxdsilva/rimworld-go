"""Library family (AX, A7 rows; ART spec §12-§13, §15-§16): the three book
items, the bookcase and the small bookcase, and their icons. Our own
designs: nothing traced or reconstructed.

- item/novel: a slim hardback lying a little askew: wine-red boards, a
  cream page block, gold bands across the spine end and a yellow ribbon
  marker trailing out of the pages.
- item/textbook: a heavy, thick volume: blue boards with brass corner
  guards, an ochre title label and coloured index tabs sticking out of the
  fore-edge.
- item/schematic: a flat drafting folio, drafting blue with pale grid
  lines and a cog drawn on it, a rolled sheet tied with a band lying
  across it.
  The three read apart by silhouette (slim / thick with tabs / flat folio
  with a roll) before colour. One sprite per kind: quality is a label in
  the inspector, not a tint (21 near-duplicates for no gameplay read, §35).
- furniture/bookcase[_half|_full]_<facing> (2x1): a low open case, two
  cells (five books each, wiki) side by side, two tiers each; the top tier
  is open above, so the books' cream page tops and coloured boards read
  from above in every facing, and through the open front when it faces
  south. `bookcase_<facing>` is empty, `_half` holds about half a load
  (the left cell's upper tier, a few more), `_full` both cells.
- furniture/small_bookcase[_half|_full] (1x1, not rotatable): one cell of
  the same case.

Wood parts are `wood`/`wood_plank` and take the renderer's whole-sprite
stuff colour shift like the shelves. No tint masks.

    make art FAMILIES=library
"""
import math

from mathutils import Matrix

from .. import stage as S
from . import overlay as O
from . import power as P
from .item import _spin

FACINGS = ("south", "east", "north", "west")


# ------------------------------------------------------------------ items
def _boards(x0, y0, x1, y1, z0, t, cover, board=0.012, inset=0.018, spine="w"):
    """A closed book lying flat: two boards of `cover`, the page block
    between them inset on the three open sides, a rounded spine on `spine`
    (w: the west edge). Returns the top board's height."""
    S.box("board_lo", x0, y0, z0, x1, y1, z0 + board, cover, bevel=0.004)
    px0 = x0 + (0.0 if spine == "w" else inset)
    S.box("pages", px0 + (0.01 if spine == "w" else 0.0), y0 + inset, z0 + board, x1 - inset, y1 - inset,
          z0 + t - board, "page_cream", bevel=0.003)
    S.box("board_hi", x0, y0, z0 + t - board, x1, y1, z0 + t, cover, bevel=0.004)
    P._rod("spine", (x0 + 0.006, y0 + 0.004, z0 + t / 2), (x0 + 0.006, y1 - 0.004, z0 + t / 2), t / 2, cover,
           segs=12, bevel=0.0)
    return z0 + t


def _novel():
    n0 = len(S.objects())
    x0, y0, x1, y1 = 0.31, -0.76, 0.69, -0.26
    top = _boards(x0, y0, x1, y1, 0.0, 0.075, "book_red")
    for k, y in enumerate((y0 + 0.06, y1 - 0.06)):   # gold bands across the spine end
        S.box("band%d" % k, x0 - 0.01, y - 0.012, top - 0.002, x0 + 0.09, y + 0.012, top + 0.003, "gold_trim",
              bevel=0.0)
    S.box("title", x0 + 0.13, -0.55, top - 0.002, x1 - 0.06, -0.47, top + 0.003, "gold_trim", bevel=0.002)
    S.box("title_in", x0 + 0.15, -0.535, top + 0.002, x1 - 0.08, -0.485, top + 0.005, "book_red", bevel=0.0)
    S.box("ribbon", 0.52, y0 - 0.07, 0.035, 0.55, y0 + 0.04, 0.042, "corn_yellow", bevel=0.0)
    _spin(n0, 0.5, -0.51, -16)
    return S.render_asset("item", "novel", zmax=0.1, extra={"item": "novel"})


def _textbook():
    n0 = len(S.objects())
    x0, y0, x1, y1 = 0.24, -0.82, 0.74, -0.22
    top = _boards(x0, y0, x1, y1, 0.0, 0.14, "book_blue", board=0.016)
    for k, (cx, cy) in enumerate(((x1, y0), (x1, y1))):  # brass guards on the outer corners
        sx, sy = -1, (1 if cy == y0 else -1)
        S.prism("guard%d" % k, [(cx, cy), (cx + sx * 0.09, cy), (cx, cy + sy * 0.09)], top - 0.006, top + 0.004,
                "brass", bevel=0.0)
    S.box("label", x0 + 0.13, -0.60, top - 0.002, x1 - 0.09, -0.40, top + 0.004, "book_ochre", bevel=0.004)
    for k, y in enumerate((-0.45, -0.50, -0.55)):
        S.box("line%d" % k, x0 + 0.16, y - 0.008, top + 0.003, x1 - (0.13 if k else 0.12), y + 0.008, top + 0.006,
              "bark", bevel=0.0)
    for k, y in enumerate((-0.32, -0.46, -0.62)):     # index tabs out of the fore-edge
        S.box("tab%d" % k, x1 - 0.03, y - 0.03, 0.05, x1 + 0.045, y + 0.03, 0.08,
              ("food_red", "corn_yellow", "foam_paint")[k], bevel=0.004)
    _spin(n0, 0.5, -0.52, 10)
    return S.render_asset("item", "textbook", zmax=0.16, extra={"item": "textbook"})


def _schematic():
    n0 = len(S.objects())
    x0, y0, x1, y1 = 0.16, -0.76, 0.84, -0.30
    S.box("back", x0, y0, 0.0, x1, y1, 0.02, "schematic_blue", bevel=0.005)
    S.box("sheets", x0 + 0.02, y0 + 0.02, 0.02, x1 - 0.02, y1 - 0.02, 0.035, "page_cream", bevel=0.002)
    S.box("cover", x0, y0, 0.035, x1, y1, 0.05, "schematic_blue", bevel=0.005)
    for k in range(1, 6):               # drafting grid
        x = x0 + (x1 - x0) * k / 6
        S.box("gx%d" % k, x - 0.004, y0 + 0.04, 0.049, x + 0.004, y1 - 0.04, 0.052, "schematic_line", bevel=0.0)
    for k in range(1, 4):
        y = y0 + (y1 - y0) * k / 4
        S.box("gy%d" % k, x0 + 0.04, y - 0.004, 0.049, x1 - 0.04, y + 0.004, 0.052, "schematic_line", bevel=0.0)
    cx, cy = 0.62, -0.52                # a cog drawn on the cover
    S.cyl("cog", cx, cy, 0.10, 0.050, 0.056, "schematic_line", segs=24, bevel=0.0)
    S.cyl("cog_in", cx, cy, 0.065, 0.054, 0.058, "schematic_blue", segs=24, bevel=0.0)
    S.cyl("hub", cx, cy, 0.025, 0.056, 0.060, "schematic_line", segs=12, bevel=0.0)
    for k in range(8):
        a = 2 * math.pi * k / 8
        tx, ty = cx + 0.115 * math.cos(a), cy + 0.115 * math.sin(a)
        S.box("tooth%d" % k, tx - 0.018, ty - 0.018, 0.050, tx + 0.018, ty + 0.018, 0.056, "schematic_line", bevel=0.0)
    p0, p1 = (0.20, -0.66, 0.095), (0.66, -0.26, 0.095)
    P._rod("roll", p0, p1, 0.042, "schematic_blue", segs=16, bevel=0.0)
    for k, (a, b) in enumerate(((p0, 0.015), (p1, -0.015))):
        d = [(p1[i] - p0[i]) for i in range(3)]
        n = math.sqrt(sum(v * v for v in d))
        e = tuple(a[i] - d[i] / n * b for i in range(3))
        P._rod("roll_end%d" % k, a, e, 0.036, "schematic_line", segs=16, bevel=0.0)
    mid = tuple((p0[i] + p1[i]) / 2 for i in range(3))
    d = [(p1[i] - p0[i]) * 0.03 for i in range(3)]
    P._rod("tie", tuple(mid[i] - d[i] for i in range(3)), tuple(mid[i] + d[i] for i in range(3)), 0.046, "bark",
           segs=16, bevel=0.0)
    _spin(n0, 0.5, -0.52, -8)
    return S.render_asset("item", "schematic", zmax=0.15, extra={"item": "schematic"})


# -------------------------------------------------------------- bookcases
COVERS = ("book_red", "book_blue", "book_green", "book_ochre", "schematic_blue", "felt_wine", "book_blue",
          "book_green", "wool_cream", "book_red")
WIDTHS = (0.075, 0.06, 0.085, 0.065, 0.07, 0.09, 0.06, 0.08)
HEIGHTS = (0.27, 0.23, 0.30, 0.25, 0.28, 0.22, 0.29, 0.24)


def _book(tag, x0, w, y0, y1, z0, h, cover, lean=0.0):
    """One book standing on a shelf, spine to the south: coloured boards,
    a cream page top, a gold band near the spine top; lean tips it
    (degrees about the y axis, through its foot's west or east edge)."""
    n0 = len(S.objects())
    S.box(tag, x0, y0, z0, x0 + w, y1, z0 + h, cover, bevel=0.006)
    S.box(tag + "_top", x0 + 0.017, y0 + 0.03, z0 + h - 0.004, x0 + w - 0.017, y1 - 0.012, z0 + h + 0.003,
          "page_cream", bevel=0.0)
    S.box(tag + "_band", x0 + 0.004, y0 - 0.004, z0 + h - 0.06, x0 + w - 0.004, y0 + 0.004, z0 + h - 0.045,
          "gold_trim", bevel=0.0)
    if lean:
        px = x0 + w if lean > 0 else x0
        m = Matrix.Translation((px, 0, z0)) @ Matrix.Rotation(math.radians(lean), 4, "Y") @ Matrix.Translation(
            (-px, 0, -z0))
        S.transform(P._since(n0), m)


def _row(tag, x0, x1, y0, y1, z0, count, seed, lean_last=False):
    """count books standing from x0 eastward (pattern by seed), the last
    one leaning on its neighbour when lean_last and there is room."""
    x = x0
    for k in range(count):
        i = seed + k
        w, h = WIDTHS[i % len(WIDTHS)], HEIGHTS[(i * 3) % len(HEIGHTS)]
        if x + w > x1:
            break
        lean = -14.0 if (lean_last and k == count - 1 and x + w + 0.08 < x1) else 0.0
        _book("%s%d" % (tag, k), x, w, y0, y1, z0, h, COVERS[(i * 7) % len(COVERS)], lean)
        x += w + 0.004


def _case(cells, fill):
    """A low open case of `cells` cells (1 tile each) side by side, front
    to the south. fill: 0 empty, 1 half, 2 full."""
    W = float(cells)
    yb, yf = -0.18, -0.66                       # back and front faces
    S.box("plinth", 0.03, yf - 0.01, 0.0, W - 0.03, yb + 0.02, 0.07, "bark", bevel=0.012)
    S.box("back", 0.06, yb - 0.06, 0.07, W - 0.06, yb, 0.86, "wood_plank", bevel=0.01)
    S.box("crown", 0.03, yb - 0.08, 0.86, W - 0.03, yb + 0.02, 0.92, "wood", bevel=0.012)
    for k in range(cells + 1):                  # sides and dividers
        x = 0.03 if k == 0 else (W - 0.11 if k == cells else k - 0.04)
        S.box("side%d" % k, x, yf, 0.0, x + 0.08, yb, 0.88, "wood", bevel=0.012)
    for k, z in enumerate((0.07, 0.46)):
        S.box("shelf%d" % k, 0.08, yf + 0.01, z, W - 0.08, yb - 0.04, z + 0.04, "wood_plank", bevel=0.006)
        S.box("lip%d" % k, 0.08, yf, z + 0.01, W - 0.08, yf + 0.03, z + 0.05, "wood", bevel=0.006)
    by0, by1 = yf + 0.05, yb - 0.08
    for c in range(cells):
        x0, x1 = c + (0.12 if c == 0 else 0.05), c + 1 - (0.12 if c == cells - 1 else 0.05)
        if fill == 2 or (fill == 1 and c == 0):
            _row("up%d_" % c, x0, x1, by0, by1, 0.50, 9, 3 * c, lean_last=fill == 1)
        if fill == 2:
            _row("lo%d_" % c, x0, x1, by0, by1, 0.11, 9, 5 + 2 * c)
        elif fill == 1:
            _row("lo%d_" % c, x0, x1, by0, by1, 0.11, 3 if c == 0 else 2, 5 + 2 * c, lean_last=True)
    return (cells, 1), 0.92


FILLS = (("", 0), ("_half", 1), ("_full", 2))


def _bookcases():
    out = []
    for suffix, fill in FILLS:
        for f in FACINGS:
            out += P.render("bookcase%s_%s" % (suffix, f), lambda fl=fill: _case(2, fl), facing=f,
                            extra={"building": "bookcase", "capacity": 10, "fill": fill}, family="furniture")
        out += P.render("small_bookcase%s" % suffix, lambda fl=fill: _case(1, fl),
                        extra={"building": "small bookcase", "capacity": 5, "fill": fill}, family="furniture")
    return out


ICONS = {
    "novel": ("item", "novel"),
    "textbook": ("item", "textbook"),
    "schematic": ("item", "schematic"),
    "bookcase": ("furniture", "bookcase_full_south"),
    "small_bookcase": ("furniture", "small_bookcase_full"),
}


def build(only=None):
    """Every book item, bookcase and icon."""
    out = [_novel(), _textbook(), _schematic()] + _bookcases()
    for name, (fam, spr) in ICONS.items():
        p = O._icon(name, fam, spr)
        if p:
            out.append(p)
    return out
