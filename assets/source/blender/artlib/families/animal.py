"""Wild animals (ART spec §11): quadrupeds in four facings.

Animals follow the figure rule (contracts.FIGURE_LEAN_DEG): modelled
upright with the head towards -Y (south), rotated to the facing, then
leaned back towards the viewer about the feet. Fixed palette materials,
no tint masks: species read by silhouette and colour.

- dustbuck: small sandy grazer, cream belly and tail, short horn prongs.
- ridgewolf: slate predator with a dark ridge down its back, pale ruff,
  pointed ears and a bushy tail.
"""
import math

from .. import contracts as C
from .. import stage as S

FACINGS = ("south", "east", "north", "west")
CX, CY = 0.5, -0.5
MARGINS = (0.125, 0.125, 0.10, 0.125)
LEAN_DEG = 40.0  # quadrupeds lean less than people: the body stays long


def _legs(dx, front, back, r, z1, fur, hoof):
    for sx in (-1, 1):
        for y in (front, back):
            S.cyl("leg", CX + sx * dx, CY + y, r, 0.03, z1, fur, segs=10)
            S.cyl("hoof", CX + sx * dx, CY + y, r * 1.15, 0.0, 0.05, hoof, segs=10)


def _dustbuck():
    _legs(0.065, -0.15, 0.15, 0.032, 0.27, "fur_tan", "fur_dark")
    S.ellipsoid("body", CX, CY, 0.34, 0.12, 0.24, 0.11, "fur_tan", segs=22, rings=12)
    S.ellipsoid("belly", CX, CY + 0.01, 0.29, 0.1, 0.19, 0.075, "fur_cream", segs=18, rings=10)
    S.ellipsoid("neck", CX, CY - 0.2, 0.44, 0.055, 0.075, 0.11, "fur_tan", segs=14, rings=8)
    S.ellipsoid("head", CX, CY - 0.27, 0.54, 0.065, 0.095, 0.065, "fur_tan", segs=18, rings=10)
    S.ellipsoid("muzzle", CX, CY - 0.35, 0.52, 0.042, 0.045, 0.038, "fur_cream", segs=12, rings=8)
    S.ellipsoid("nose", CX, CY - 0.39, 0.53, 0.02, 0.015, 0.016, "fur_dark", segs=8, rings=6)
    for sx in (-1, 1):
        S.ellipsoid("ear", CX + sx * 0.075, CY - 0.24, 0.6, 0.05, 0.02, 0.028, "fur_tan", segs=10, rings=6)
        S.ellipsoid("eye", CX + sx * 0.05, CY - 0.33, 0.565, 0.014, 0.012, 0.016, "metal_dark", segs=8, rings=6)
        S.cyl("prong", CX + sx * 0.03, CY - 0.25, 0.012, 0.58, 0.72, "horn", segs=8, r_top=0.006, bevel=0.0)
        S.cyl("tine", CX + sx * 0.05, CY - 0.27, 0.009, 0.63, 0.69, "horn", segs=6, r_top=0.004, bevel=0.0)
    S.ellipsoid("tail", CX, CY + 0.25, 0.39, 0.035, 0.035, 0.045, "fur_cream", segs=10, rings=6)


def _ridgewolf():
    _legs(0.075, -0.18, 0.19, 0.038, 0.29, "fur_grey", "fur_dark")
    S.ellipsoid("body", CX, CY + 0.02, 0.37, 0.13, 0.28, 0.12, "fur_grey", segs=24, rings=12)
    S.ellipsoid("ridge", CX, CY + 0.03, 0.45, 0.05, 0.25, 0.05, "fur_dark", segs=16, rings=8)
    S.ellipsoid("ruff", CX, CY - 0.2, 0.42, 0.12, 0.1, 0.13, "fur_cream", segs=18, rings=10)
    S.ellipsoid("head", CX, CY - 0.29, 0.53, 0.08, 0.09, 0.075, "fur_grey", segs=18, rings=10)
    S.ellipsoid("snout", CX, CY - 0.4, 0.49, 0.04, 0.075, 0.036, "fur_grey", segs=14, rings=8)
    S.ellipsoid("nose", CX, CY - 0.47, 0.5, 0.02, 0.016, 0.016, "fur_dark", segs=8, rings=6)
    for sx in (-1, 1):
        S.cyl("ear", CX + sx * 0.05, CY - 0.27, 0.032, 0.58, 0.68, "fur_dark", segs=8, r_top=0.004, bevel=0.0)
        S.ellipsoid("eye", CX + sx * 0.045, CY - 0.36, 0.555, 0.014, 0.012, 0.014, "crop_fruit", segs=8, rings=6)
    S.ellipsoid("tail", CX, CY + 0.33, 0.37, 0.045, 0.1, 0.045, "fur_grey", segs=12, rings=8)
    S.ellipsoid("tail_tip", CX, CY + 0.42, 0.31, 0.04, 0.06, 0.04, "fur_dark", segs=10, rings=6)


SPECIES = {"dustbuck": (_dustbuck, 0.74), "ridgewolf": (_ridgewolf, 0.7)}


def _lean_matrix():
    from mathutils import Matrix
    t = math.radians(LEAN_DEG)
    feet = Matrix.Translation((0.5, CY, 0))
    return (feet @ Matrix.Rotation(-t, 4, "X") @ Matrix.Scale(C.FIGURE_SCALE, 4)
            @ Matrix.Translation((-0.5, -CY, 0)))


def _zmax_screen(height):
    t = math.radians(LEAN_DEG)
    hgt = height * C.FIGURE_SCALE
    return hgt * (math.sin(t) / C.SHEAR_K + math.cos(t)) + 0.05


def build():
    out = []
    for species, (builder, height) in SPECIES.items():
        for f in FACINGS:
            builder()
            S.face(S.objects(), f, (1, 1))
            S.transform(S.objects(), _lean_matrix())
            out.append(S.render_asset("animal", "%s_%s" % (species, f), zmax=_zmax_screen(height),
                                      margins=MARGINS, extra={"species": species, "facing": f}))
    return out
