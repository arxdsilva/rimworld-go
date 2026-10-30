"""Sarcophagus family (STO-2/3; ART spec §12-§13): a 1x2 stone coffin,
plus its Architect icon. Our own design: nothing traced or reconstructed.

- sarcophagus_<facing> (1x2, furniture): authored facing SOUTH with the
  head end north, like the grave (the Def row is rotatable; four facings,
  turned with stage.face()). A dark plinth carries a chest with corner
  pilasters under an overhanging lid; the lid has a raised ridge, a raised
  head slab and an inlaid disc-and-bar sigil with two bands. The chest,
  pilasters and lid are the `stone_tint` tint group (a neutral body plus a
  _mask.png), as for the sculptures, so the renderer multiplies them by
  the stuff colour; the plinth and inlays stay a dark neutral.

    make art FAMILIES=sarcophagus
"""
from .. import stage as S
from . import overlay as O

FACINGS = ("south", "east", "north", "west")


def _sarcophagus():
    """1x2, head north: plinth, chest, pilasters, lid, ridge, head slab, sigil."""
    S.box("plinth", 0.06, -1.94, 0.0, 0.94, -0.06, 0.08, "stone_neutral_dark", bevel=0.02)
    S.box("chest", 0.13, -1.87, 0.08, 0.87, -0.13, 0.46, "stone_tint", bevel=0.025)
    for i, x in enumerate((0.11, 0.81)):
        for j, y in enumerate((-1.89, -0.19)):
            S.box("pilaster%d%d" % (i, j), x, y, 0.08, x + 0.08, y + 0.08, 0.46, "stone_tint", bevel=0.012)
    S.box("panel_s", 0.24, -1.885, 0.16, 0.76, -1.87, 0.38, "stone_neutral_dark", bevel=0.0)
    S.box("lid", 0.08, -1.92, 0.46, 0.92, -0.08, 0.54, "stone_tint", bevel=0.02)
    S.box("ridge", 0.20, -1.78, 0.54, 0.80, -0.46, 0.60, "stone_tint", bevel=0.035, segments=2)
    S.box("head", 0.16, -0.46, 0.54, 0.84, -0.16, 0.68, "stone_tint", bevel=0.04, segments=2)
    # inlays: two bands across the ridge and a disc-and-bar sigil
    for k, y in enumerate((-1.62, -0.64)):
        S.box("band%d" % k, 0.20, y - 0.02, 0.60, 0.80, y + 0.02, 0.606, "stone_neutral_dark", bevel=0.0)
    S.cyl("sigil_ring", 0.5, -1.10, 0.13, 0.60, 0.606, "stone_neutral_dark", segs=28, bevel=0.0)
    S.cyl("sigil_disc", 0.5, -1.10, 0.09, 0.60, 0.608, "stone_tint", segs=28, bevel=0.0)
    S.box("sigil_bar", 0.485, -1.40, 0.60, 0.515, -0.80, 0.611, "stone_neutral_dark", bevel=0.0)
    return (1, 2), 0.68


def build(only=None):
    """The sarcophagus in four facings and its icon (ui_icon/sarcophagus)."""
    out = []
    for f in FACINGS:
        fp0, zmax = _sarcophagus()
        fp = S.face(S.objects(), f, fp0)
        out.append(S.render_asset("furniture", "sarcophagus_" + f, footprint=fp, zmax=zmax,
                                  extra={"building": "sarcophagus", "stuff_tint": "stone_tint",
                                         "rotatable": True, "facing": f}))
    p = O._icon("sarcophagus", "furniture", "sarcophagus_south")
    if p:
        out.append(p)
    return out
