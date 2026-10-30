"""Insect hive (CBT-4; ART spec §12): the hive mound an infestation raises
in a dark cave. Our own design: nothing traced or reconstructed.

- hive_mound (1x1, structure, never built by the colony): a lumpy ochre
  mound of chewed earth in three tiers, darker clods round its foot, a
  black tunnel mouth on the crown and two on the south face, each in a
  raised rim, and a few amber resin beads. Natural, so it
  neither rotates nor tints.

    make art FAMILIES=hive
"""
from .. import stage as S


def _hive_mound():
    # three stacked tiers of chewed earth narrowing to a crown
    S.blob("base", 0.5, -0.54, 0.0, 0.40, "soil", seed=9101, lumps=0.16, squash=0.55)
    S.blob("mid", 0.49, -0.52, 0.16, 0.27, "fur_ochre", seed=9102, lumps=0.2, squash=0.85)
    S.blob("crown", 0.48, -0.50, 0.36, 0.16, "fur_sand", seed=9103, lumps=0.22, squash=0.9)
    for k, (x, y, r) in enumerate(((0.18, -0.78, 0.10), (0.80, -0.80, 0.09), (0.86, -0.36, 0.08),
                                   (0.16, -0.30, 0.08))):
        S.blob("lump%d" % k, x, y, 0.0, r, "bark", seed=9110 + k, lumps=0.25, squash=0.8)
    # tunnel mouths: a dark hole in a raised rim, one on the crown, two on the south face
    S.cyl("crown_rim", 0.48, -0.52, 0.08, 0.46, 0.51, "soil_rich", segs=9, ry=0.065, bevel=0.012)
    S.cyl("crown_hole", 0.48, -0.525, 0.062, 0.49, 0.515, "eye_black", segs=9, ry=0.05, bevel=0.0)
    for k, (x, y, z, r) in enumerate(((0.36, -0.90, 0.08, 0.06), (0.62, -0.76, 0.26, 0.05))):
        S.ellipsoid("rim%d" % k, x, y, z, r * 1.45, r * 0.8, r * 1.25, "soil_rich", segs=14, rings=8)
        S.ellipsoid("hole%d" % k, x, y - r * 0.55, z, r * 1.15, r * 0.4, r * 0.95, "eye_black", segs=12, rings=6)
    for k, (x, y, z) in enumerate(((0.28, -0.62, 0.22), (0.70, -0.58, 0.28), (0.40, -0.44, 0.44))):
        S.ellipsoid("resin%d" % k, x, y, z, 0.022, 0.022, 0.026, "tooth_orange", segs=10, rings=6)
    return (1, 1), 0.62


def build():
    fp, zmax = _hive_mound()
    return [S.render_asset("structure", "hive_mound", footprint=fp, zmax=zmax,
                           extra={"building": "hive mound", "rotatable": False, "facing": "south"})]
