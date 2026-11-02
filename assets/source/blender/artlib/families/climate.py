"""Climate buildings (TEMP-1; ART spec §12-§14): the passive cooler and the
vent. Our own designs: nothing traced or reconstructed.

- furniture/passive_cooler (1x1, not rotatable, fuelled): a slatted
  plank cabinet on a plinth with a clay basin on top, brimming with
  water, a soaked straw pad and floating ice; a damp cloth hangs on the
  south face. furniture/passive_cooler_empty is the same cabinet unfuelled:
  a dry basin, a slack dry pad and no ice (the feeding trough's
  empty/full idea, one sprite per state).
- furniture/vent_<open|closed>_<facing> (1x1): stands in a wall gap and
  joins the walls either side like the cooler (clip). A steel frame holds
  a louvred damper through the wall; the louvres show on the top and both
  faces. Open: the slats are turned edge-on with dark gaps between them;
  closed: they lie flat into one shut panel. A red damper handle slides in
  a slot along the top's facing edge (east end open, west end closed), so
  the four facings differ.

Ui icons: ui_icon/passive_cooler and ui_icon/vent, cut from the south
sprites like the Architect icons in overlay.py.

    make art FAMILIES=climate
"""
from .. import stage as S
from . import power as P

FAMILY = "furniture"
FACINGS = P.FACINGS
COOLER_H = 0.62
VENT_H = 0.94


# ------------------------------------------------------- passive cooler
def _passive_cooler(full):
    def build():
        S.box("plinth", 0.12, -0.88, 0.0, 0.88, -0.12, 0.06, "bark", bevel=0.02)
        S.box("cabinet", 0.16, -0.84, 0.06, 0.84, -0.16, COOLER_H - 0.10, "wood_plank", bevel=0.03)
        for z in (0.16, 0.28, 0.40):                                  # dark gaps between the side slats
            S.box("gap_s", 0.18, -0.848, z, 0.82, -0.836, z + 0.022, "bark", bevel=0.0)
            S.box("gap_e", 0.836, -0.82, z, 0.848, -0.18, z + 0.022, "bark", bevel=0.0)
            S.box("gap_w", 0.152, -0.82, z, 0.164, -0.18, z + 0.022, "bark", bevel=0.0)
        for x, y in ((0.16, -0.84), (0.84, -0.84), (0.16, -0.16), (0.84, -0.16)):
            S.box("post", x - 0.035, y - 0.035, 0.06, x + 0.035, y + 0.035, COOLER_H - 0.06, "bark", bevel=0.012)
        S.box("rim", 0.12, -0.88, COOLER_H - 0.10, 0.88, -0.12, COOLER_H - 0.06, "bark", bevel=0.015)
        # the clay basin: a raised lip round a sunken bed
        S.box("basin", 0.18, -0.82, COOLER_H - 0.07, 0.82, -0.18, COOLER_H, "ceramic", bevel=0.025)
        if full:
            S.box("water", 0.23, -0.77, COOLER_H - 0.06, 0.77, -0.23, COOLER_H + 0.004, "water_shallow",
                  bevel=0.01)
            S.box("pad", 0.30, -0.70, COOLER_H - 0.02, 0.70, -0.42, COOLER_H + 0.018, "straw", bevel=0.012)
            for k, (x, y, s) in enumerate(((0.62, -0.32, 0.055), (0.36, -0.30, 0.045), (0.70, -0.62, 0.04))):
                S.box("ice%d" % k, x - s, y - s, COOLER_H - 0.01, x + s, y + s, COOLER_H + 0.03, "ice", bevel=0.012)
            S.box("cloth", 0.30, -0.865, 0.20, 0.70, -0.845, COOLER_H - 0.04, "water_shallow", bevel=0.006)
        else:
            S.box("bed", 0.23, -0.77, COOLER_H - 0.06, 0.77, -0.23, COOLER_H + 0.002, "soil", bevel=0.01)
            for k, (x0, y0, x1, y1) in enumerate(((0.30, -0.66, 0.52, -0.64), (0.50, -0.66, 0.52, -0.40),
                                                  (0.52, -0.52, 0.70, -0.50))):   # dry cracks
                S.box("crack%d" % k, x0, y0, COOLER_H, x1, y1, COOLER_H + 0.006, "bark", bevel=0.0)
            S.box("pad", 0.30, -0.42, COOLER_H - 0.01, 0.62, -0.28, COOLER_H + 0.012, "straw", bevel=0.01)
            S.box("cloth", 0.36, -0.865, 0.30, 0.64, -0.845, COOLER_H - 0.04, "burlap", bevel=0.006)
        return (1, 1), COOLER_H + 0.04
    return build


# ----------------------------------------------------------------- vent
def _vent(is_open):
    """Authored facing south: the handle on the south edge."""
    def build():
        S.box("post_w", -P.EXT, -0.96, 0.0, 0.12, -0.04, 1.0, "metal_dark", bevel=0.02)
        S.box("post_e", 0.88, -0.96, 0.0, 1 + P.EXT, -0.04, 1.0, "metal_dark", bevel=0.02)
        S.box("frame", 0.10, -0.92, 0.0, 0.90, -0.08, VENT_H, "steel", bevel=0.035)
        S.box("well_top", 0.18, -0.80, VENT_H - 0.02, 0.82, -0.20, VENT_H + 0.004, "metal_dark", bevel=0.0)
        for face, y in (("s", -0.925), ("n", -0.075)):
            S.box("well_" + face, 0.18, y - 0.006, 0.18, 0.82, y + 0.006, 0.80, "metal_dark", bevel=0.0)
        # louvres: on the top they run east-west; on the faces, across them
        n = 5
        for k in range(n):
            y = -0.76 + k * 0.13
            if is_open:
                S.box("slat_top", 0.20, y - 0.012, VENT_H - 0.06, 0.80, y + 0.012, VENT_H + 0.016, "steel",
                      bevel=0.004)
            else:
                S.box("slat_top", 0.19, y - 0.062, VENT_H - 0.012, 0.81, y + 0.062, VENT_H + 0.014, "steel",
                      bevel=0.008)
        for face, y0, y1 in (("s", -0.945, -0.915), ("n", -0.085, -0.055)):
            for k in range(n):
                z = 0.24 + k * 0.12
                if is_open:
                    S.box("slat_" + face, 0.20, y0, z - 0.012, 0.80, y1, z + 0.012, "steel", bevel=0.004)
                else:
                    S.box("slat_" + face, 0.19, y0, z - 0.058, 0.81, y1, z + 0.058, "steel", bevel=0.006)
        # damper handle in a slot along the top's facing edge: slid east
        # (open) or west (closed)
        S.box("slot", 0.26, -0.885, VENT_H - 0.01, 0.74, -0.835, VENT_H + 0.006, "fur_black", bevel=0.0)
        x = 0.68 if is_open else 0.32
        S.box("stem", x - 0.02, -0.875, VENT_H, x + 0.02, -0.845, VENT_H + 0.05, "metal_dark", bevel=0.006)
        S.ellipsoid("knob", x, -0.86, VENT_H + 0.06, 0.045, 0.04, 0.03, "med_red", segs=12, rings=8)
        return (1, 1), 1.0
    return build


# ---------------------------------------------------------------- build
def build():
    out = []
    for full in (True, False):
        name = "passive_cooler" if full else "passive_cooler_empty"
        out += P.render(name, _passive_cooler(full), family=FAMILY,
                        extra={"building": "passive cooler", "state": "full" if full else "empty"})
    for f in FACINGS:
        for is_open in (True, False):
            state = "open" if is_open else "closed"
            out += P.render("vent_%s_%s" % (state, f), _vent(is_open), facing=f, family=FAMILY,
                            clip=P._clip_for(f),
                            extra={"building": "vent", "state": state, "in_wall": True})
    from . import overlay as O
    for name, sprite in (("passive_cooler", "passive_cooler"), ("vent", "vent_open_south")):
        p = O._icon(name, FAMILY, sprite)
        if p:
            out.append(p)
    return out
