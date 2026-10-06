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
HIP_Z = {"dustbuck": 0.27, "ridgewolf": 0.29}

# Trot (ART spec §27): diagonal leg pairs swing together. Same 8-frame
# cycle as pawns: neutral, w1, w2, w1, neutral, w3, w4, w3.
TROT_DEG = 30.0
WALK_FRAMES = {"walk1": 0.5, "walk2": 1.0, "walk3": -0.5, "walk4": -1.0}


def _mean(ob, axis):
    vs = ob.data.vertices
    return sum((ob.matrix_world @ v.co)[axis] for v in vs) / max(len(vs), 1)


def _pose(species, swing):
    from mathutils import Matrix
    if swing == 0:
        return
    pz = HIP_Z[species]
    for ob in S.objects():
        if ob.name.split(".")[0] not in ("leg", "hoof"):
            continue
        side = -1 if _mean(ob, 0) < CX else 1
        front = 1 if _mean(ob, 1) < CY else -1
        a = math.radians(TROT_DEG * swing * side * front)
        py = _mean(ob, 1)
        m = (Matrix.Translation((0, py, pz)) @ Matrix.Rotation(a, 4, "X")
             @ Matrix.Translation((0, -py, -pz)))
        ob.matrix_world = m @ ob.matrix_world


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


def _corpse(builder):
    """The species model rolled onto its side, lying across the tile,
    over a small dark pool."""
    from mathutils import Matrix, Vector
    builder()
    objs = S.objects()
    S.face(objs, "east", (1, 1))
    pivot = Vector((0.5, -0.5, 0.0))
    S.transform(objs, Matrix.Translation(pivot) @ Matrix.Rotation(math.radians(100), 4, "X")
                @ Matrix.Scale(0.82, 4) @ Matrix.Translation(-pivot))
    zmin = min((ob.matrix_world @ v.co).z for ob in objs for v in ob.data.vertices)
    S.transform(objs, Matrix.Translation((0, 0, -zmin)))
    zmax = max((ob.matrix_world @ v.co).z for ob in objs for v in ob.data.vertices)
    S.ellipsoid("pool", 0.5, -0.66, 0.0, 0.26, 0.1, 0.01, "food_red", segs=20, rings=6)
    return zmax


def build():
    out = []
    for species, (builder, _h) in SPECIES.items():
        zmax = _corpse(builder)
        out.append(S.render_asset("item", "corpse_%s" % species, zmax=zmax + 0.05,
                                  extra={"item": "%s corpse" % species, "species": species}))
    for species, (builder, height) in SPECIES.items():
        for f in FACINGS:
            for fr in [None] + list(WALK_FRAMES):
                builder()
                _pose(species, WALK_FRAMES.get(fr, 0.0))
                S.face(S.objects(), f, (1, 1))
                S.transform(S.objects(), _lean_matrix())
                extra = {"species": species, "facing": f}
                if fr:
                    extra["frame"] = fr
                name = "%s_%s" % (species, f) + ("_" + fr if fr else "")
                out.append(S.render_asset("animal", name, zmax=_zmax_screen(height), margins=MARGINS, extra=extra))
    return out
