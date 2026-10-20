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

# Trot (ART spec §27.1): 8 poses per facing, walk01..walk08. Diagonal leg
# pairs (front-left + back-right, front-right + back-left) swing together
# by TROT_DEG; the swinging leg lifts, the body bobs twice per cycle (highest
# at mid-stance) and the head nods +-NOD_DEG.
TROT_DEG = 30.0
LIFT = 0.025
BOB = 0.014
NOD_DEG = 3.0
TROT_N = 8
WALK_FRAMES = {"walk%02d" % i: i for i in range(1, TROT_N + 1)}
HEAD_PARTS = ("neck", "head", "muzzle", "nose", "eye", "ear", "prong", "tine", "snout")
NECK_PIVOT = {"dustbuck": (CY - 0.12, 0.40), "ridgewolf": (CY - 0.15, 0.42)}


def _mean(ob, axis):
    vs = ob.data.vertices
    return sum((ob.matrix_world @ v.co)[axis] for v in vs) / max(len(vs), 1)


def _pose(species, frame):
    from mathutils import Matrix
    ph = (WALK_FRAMES[frame] - 1) / TROT_N
    pz = HIP_Z[species]
    bob = BOB * (1.0 - math.cos(4 * math.pi * ph)) / 2.0
    nod = math.radians(NOD_DEG * math.sin(4 * math.pi * ph))
    ny, nz = NECK_PIVOT[species]
    for ob in S.objects():
        n = ob.name.split(".")[0]
        if n in ("leg", "hoof"):
            side = -1 if _mean(ob, 0) < CX else 1
            front = 1 if _mean(ob, 1) < CY else -1
            sg = side * front
            a = math.radians(TROT_DEG * math.cos(2 * math.pi * ph) * sg)
            py = _mean(ob, 1)
            lift = LIFT * max(0.0, sg * math.sin(2 * math.pi * ph))
            m = (Matrix.Translation((0, 0, lift)) @ Matrix.Translation((0, py, pz))
                 @ Matrix.Rotation(a, 4, "X") @ Matrix.Translation((0, -py, -pz)))
            ob.matrix_world = m @ ob.matrix_world
            continue
        m = Matrix.Translation((0, 0, bob))
        if n in HEAD_PARTS:
            m = m @ (Matrix.Translation((0, ny, nz)) @ Matrix.Rotation(nod, 4, "X")
                     @ Matrix.Translation((0, -ny, -nz)))
        ob.matrix_world = m @ ob.matrix_world


# --- action poses (ART §27.2) -----------------------------------------------
# Hand-authored keyframes, built on the same species model and canvas as the
# standing frame. Angles in degrees. Rotation about X: a leg's positive angle
# swings the foot back (toward the tail); a head's positive angle pitches the
# nose down; a body's positive pitch drops the front.
#   ty: body shift along Y (negative = forward)   bz: body drop (legs shorten)
#   pitch: torso pitch   fl/hl: front/hind leg swing   hlift: hind leg lift
#   head: neck pitch   yaw: head turn   jaw/up: lower/upper jaw opening (wolf)
POSES = {
    "dustbuck": {
        "kick1": dict(ty=-0.02, bz=0.02, pitch=7, fl=-10, hl=6, head=8),
        "kick2": dict(ty=-0.02, bz=0.0, pitch=12, fl=-6, hl=38, hlift=0.03, head=14),
        "kick3": dict(ty=-0.01, bz=-0.01, pitch=17, fl=-4, hl=78, hlift=0.08, head=-6),
        "kick4": dict(ty=0.0, bz=0.015, pitch=4, fl=2, hl=14, head=4),
        "graze1": dict(ty=-0.02, pitch=4, fl=-8, hl=4, head=40),
        "graze2": dict(ty=-0.03, pitch=6, fl=-10, hl=4, head=62),
        "flinch1": dict(ty=0.05, bz=0.02, pitch=-7, fl=-14, hl=10, head=-20, yaw=18),
        "flinch2": dict(ty=0.025, bz=0.01, pitch=-3, fl=-6, hl=5, head=-9, yaw=8),
    },
    "ridgewolf": {
        "bite1": dict(ty=0.02, bz=0.07, pitch=11, fl=-14, hl=14, head=6),
        "bite2": dict(ty=-0.05, bz=0.03, pitch=7, fl=-34, hl=36, hlift=0.01, head=-6, jaw=58, up=22),
        "bite3": dict(ty=-0.06, bz=0.05, pitch=12, fl=-42, hl=44, hlift=0.02, head=8, jaw=8, up=2),
        "bite4": dict(ty=-0.03, bz=0.03, pitch=5, fl=-12, hl=14, head=2),
        "graze1": dict(ty=-0.02, pitch=4, fl=-8, hl=4, head=42),
        "graze2": dict(ty=-0.03, pitch=6, fl=-10, hl=4, head=64),
        "flinch1": dict(ty=0.05, bz=0.03, pitch=-7, fl=-14, hl=10, head=-20, yaw=18),
        "flinch2": dict(ty=0.025, bz=0.015, pitch=-3, fl=-6, hl=5, head=-9, yaw=8),
    },
}
ACTION_FRAMES = {sp: list(p) for sp, p in POSES.items()}
JAW_HINGE = {"ridgewolf": (CY - 0.33, 0.5)}
UPPER_JAW = ("snout", "nose", "fang_u")
LOWER_JAW = ("jaw", "fang_l")


def _add_jaws():
    """Lower jaw and fangs for the bite set (ridgewolf only)."""
    S.ellipsoid("jaw", CX, CY - 0.39, 0.447, 0.038, 0.08, 0.024, "fur_cream", segs=12, rings=8)
    for sx in (-1, 1):
        S.cyl("fang_u", CX + sx * 0.025, CY - 0.45, 0.002, 0.435, 0.475, "ceramic", segs=6, r_top=0.009, bevel=0.0)
        S.cyl("fang_l", CX + sx * 0.022, CY - 0.43, 0.008, 0.45, 0.49, "ceramic", segs=6, r_top=0.002, bevel=0.0)


def _pose_action(species, p):
    from mathutils import Matrix
    pz = HIP_Z[species]
    ny, nz = NECK_PIVOT[species]
    ty, bz = p.get("ty", 0.0), p.get("bz", 0.0)
    shrink = (pz - bz) / pz
    body = (Matrix.Translation((0, ty, -bz)) @ Matrix.Translation((0, CY + 0.2, pz))
            @ Matrix.Rotation(math.radians(p.get("pitch", 0.0)), 4, "X") @ Matrix.Translation((0, -(CY + 0.2), -pz)))
    head = (Matrix.Translation((0, ny, nz)) @ Matrix.Rotation(math.radians(p.get("yaw", 0.0)), 4, "Z")
            @ Matrix.Rotation(math.radians(p.get("head", 0.0)), 4, "X") @ Matrix.Translation((0, -ny, -nz)))
    jy, jz = JAW_HINGE.get(species, (ny, nz))

    def hinge(deg):
        return Matrix.Translation((0, jy, jz)) @ Matrix.Rotation(math.radians(deg), 4, "X") @ Matrix.Translation((0, -jy, -jz))

    for ob in S.objects():
        n = ob.name.split(".")[0]
        if n in ("leg", "hoof"):
            front = _mean(ob, 1) < CY
            py = _mean(ob, 1)
            a = p.get("fl", 0.0) if front else p.get("hl", 0.0)
            lift = 0.0 if front else p.get("hlift", 0.0)
            hip = shrink * pz
            m = (Matrix.Translation((0, ty, lift)) @ Matrix.Translation((0, py, hip))
                 @ Matrix.Rotation(math.radians(a), 4, "X") @ Matrix.Translation((0, -py, -hip))
                 @ Matrix.Scale(shrink, 4, (0, 0, 1)))
            ob.matrix_world = m @ ob.matrix_world
            continue
        m = body
        if n in HEAD_PARTS or n in UPPER_JAW or n in LOWER_JAW:
            m = m @ head
            if n in UPPER_JAW:
                m = m @ hinge(-p.get("up", 0.0))
            elif n in LOWER_JAW:
                m = m @ hinge(p.get("jaw", 0.0))
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
            for fr in [None] + list(WALK_FRAMES) + ACTION_FRAMES[species]:
                builder()
                if fr in WALK_FRAMES:
                    _pose(species, fr)
                elif fr:
                    if species == "ridgewolf" and fr.startswith("bite"):
                        _add_jaws()
                    _pose_action(species, POSES[species][fr])
                S.face(S.objects(), f, (1, 1))
                S.transform(S.objects(), _lean_matrix())
                extra = {"species": species, "facing": f}
                if fr:
                    extra["frame"] = fr
                name = "%s_%s" % (species, f) + ("_" + fr if fr else "")
                out.append(S.render_asset("animal", name, zmax=_zmax_screen(height), margins=MARGINS, extra=extra))
    return out
