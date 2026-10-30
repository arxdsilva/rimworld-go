"""Storage furniture (ART spec §12-§13, PRD-4): the shelf, plus its
Architect icon.

The shelf is 2x1, rotatable, authored facing SOUTH (its open front points
there) and turned with stage.face() like the dresser. It is built of its
stuff: the frame is drawn in `wood` and takes no mask; stone and steel
stuff colour it through the renderer's default stuff colour shift, as for
the other wooden furniture (bedroom.py). Low and open so stored items
drawn on its boards read clearly: a top of three planks with open gaps over a
dark lower board, end frames, a centre post and a back rail along the north
edge.
"""
from .. import stage as S
from . import overlay as O

FACINGS = ("south", "east", "north", "west")


def _shelf():
    """2x1 shelf: end frames with corner posts, a centre post, a dark low
    board, an open top of three planks, a back rail and brace along the north."""
    x0, x1, y0, y1 = 0.06, 1.94, -0.9, -0.12
    for k, x in enumerate((x0, 0.97, x1 - 0.07)):          # end frames and the centre post pair
        for j, y in enumerate((y0, y1 - 0.07)):
            S.box("post%d%d" % (k, j), x, y, 0.0, x + 0.07, y + 0.07, 0.6, "wood", bevel=0.012)
        S.box("side%d" % k, x + 0.01, y0 + 0.07, 0.28, x + 0.06, y1 - 0.07, 0.33, "wood_plank", bevel=0.006)
    # the low board, dark under the open top so its gaps read
    S.box("board_low", x0 + 0.02, y0 + 0.02, 0.1, x1 - 0.02, y1 - 0.02, 0.15, "bark", bevel=0.01)
    # the top: three planks along the shelf with open gaps between them, on cross bearers
    for k, xb in enumerate((x0 + 0.03, 0.97, x1 - 0.1)):
        S.box("bearer%d" % k, xb, y0 + 0.02, 0.47, xb + 0.07, y1 - 0.02, 0.52, "wood", bevel=0.008)
    depth = (y1 - y0) - 0.04
    for k in range(3):
        py0 = y0 + 0.02 + depth * k / 3 + 0.025
        S.box("plank%d" % k, x0 + 0.02, py0, 0.52, x1 - 0.02, py0 + depth / 3 - 0.05, 0.57, "wood_plank", bevel=0.01)
    S.box("rail", x0, y1 - 0.06, 0.6, x1, y1, 0.66, "wood", bevel=0.01)
    S.box("brace", x0 + 0.07, y1 - 0.04, 0.2, x1 - 0.07, y1 - 0.01, 0.26, "wood_plank", bevel=0.006)
    S.box("kick", x0 + 0.04, y0 - 0.005, 0.02, x1 - 0.04, y0 + 0.03, 0.09, "bark", bevel=0.006)
    return (2, 1), 0.66


def build(only=None):
    """The shelf in four facings and its icon (ui_icon/shelf)."""
    out = []
    for f in FACINGS:
        fp0, zmax = _shelf()
        fp = S.face(S.objects(), f, fp0)
        out.append(S.render_asset("furniture", "shelf_" + f, footprint=fp, zmax=zmax,
                                  extra={"building": "shelf", "rotatable": True, "facing": f}))
    p = O._icon("shelf", "furniture", "shelf_south")
    if p:
        out.append(p)
    return out
