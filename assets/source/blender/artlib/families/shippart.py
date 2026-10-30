"""Crashed ship part (DEF-4; ART spec §12-§14): the 6x3 wreck the
CrashedShipPart threat drops, with its dormant mechanoid guards beside it.
An ORIGINAL design: a long hexagonal hull segment of gunmetal plates
banded in soot-black armour rings, nose buried in a scorched crater at the
east end, a torn west end showing dark ribs and trailing cables, three
swept dorsal fins and cyan-lit slots along its back (the mechs' lens
colour, so it reads as theirs), plate shards thrown around it.

One sprite, not rotatable: defense/ship_part, `building: ship part`. The
lit slots are emissive palette colours baked into the sprite (no separate
glow layer).
"""
import math

from mathutils import Vector

from .. import stage as S
from .animals.mech import hull, obox, ocyl

FAMILY = "defense"
CY = -1.5           # hull axis (y), mid footprint
X0, X1 = 0.55, 4.75  # hull west (torn) and east (nose root) sections


def _hex(x, cz, ry, rz, k=1.0):
    """Six points of a flat-sided hexagonal section at x."""
    return [(x, CY + ry * k * math.cos(a), cz + rz * k * math.sin(a))
            for a in (math.radians(d) for d in (0, 60, 120, 180, 240, 300))]


def _z(x):
    """Hull axis height: the hull tips down towards the buried nose."""
    return 0.62 - 0.06 * (x - X0)


def _ship_part():
    # scorched crater floor and thrown-up earth round the buried nose
    S.ellipsoid("crater", 3.1, CY, 0.0, 2.85, 1.38, 0.025, "pod_scorch", segs=40, rings=8)
    for k, (x, y, r) in enumerate(((5.55, -0.45, 0.24), (5.75, -1.1, 0.28), (5.8, -1.85, 0.27), (5.5, -2.55, 0.22),
                                   (4.9, -0.3, 0.18), (4.95, -2.7, 0.18))):
        S.blob("berm%d" % k, x, y, 0.0, r, "mud", seed=700 + k, lumps=0.25, squash=0.55)
    ry, rz = 0.66, 0.56
    # main hull, its nose wedge driven into the ground
    hull("hull", _hex(X0, _z(X0), ry, rz) + _hex(X1, _z(X1), ry, rz)
         + [(5.55, CY + s * 0.3, z) for s in (-1, 1) for z in (0.05, 0.42)], "mech_gunmetal", bevel=0.03)
    for k, x in enumerate((1.25, 2.15, 3.05, 3.95)):      # armour rings
        hull("band%d" % k, _hex(x - 0.09, _z(x), ry, rz, 1.07) + _hex(x + 0.09, _z(x), ry, rz, 1.07), "mech_soot",
             bevel=0.02)
    hull("nose_band", _hex(X1 - 0.05, _z(X1), ry, rz, 1.06) + _hex(X1 + 0.12, _z(X1) - 0.02, ry * 0.9, rz * 0.9, 1.06),
         "mech_soot", bevel=0.02)
    # torn west end: a dark interior face, broken ribs and hanging cables
    hull("torn", _hex(X0 - 0.04, _z(X0), ry * 0.86, rz * 0.86) + _hex(X0 + 0.02, _z(X0), ry * 0.86, rz * 0.86),
         "backdrop", bevel=0.0)
    for k, (dy, dz, ln) in enumerate(((0.45, 0.25, 0.3), (-0.4, 0.32, 0.22), (0.1, 0.5, 0.35), (-0.15, -0.2, 0.18))):
        a = Vector((X0, CY + dy, _z(X0) + dz))
        obox("rib%d" % k, a, a + Vector((-ln, dy * 0.15, -0.05)), 0.025, 0.02, "steel_dark")
    for k, (dy, mat) in enumerate(((0.2, "copper"), (-0.25, "mech_joint"), (0.0, "mech_stripe"))):
        from .animals import parts as AP
        pts = AP.bezier((X0, CY + dy, _z(X0) + 0.1), (X0 - 0.35, CY + dy * 1.5, 0.2), (X0 - 0.45, CY + dy * 2.2, 0.03), 7)
        AP.tube("cable%d" % k, pts, [0.03] * 7, mat, segs=8, caps=(0.5, 0.5))
    # dorsal fins, swept back (west)
    for k, x in enumerate((1.7, 2.6, 3.5)):
        top = _z(x) + rz
        hull("fin%d" % k, [(x + 0.32, CY + s * 0.05, top - 0.02) for s in (-1, 1)]
             + [(x - 0.28, CY + s * 0.05, top - 0.02) for s in (-1, 1)]
             + [(x - 0.38, CY + s * 0.025, top + 0.5 - 0.06 * k) for s in (-1, 1)]
             + [(x - 0.12, CY + s * 0.025, top + 0.42 - 0.06 * k) for s in (-1, 1)], "mech_soot", bevel=0.012)
    # lit slots along both upper flanks, between the rings; a core vent amidships
    for x in (1.7, 2.6, 3.5):
        for s in (-1, 1):
            zc = _z(x) + rz * 0.87
            obox("slot", (x - 0.25, CY + s * ry * 0.48, zc), (x + 0.25, CY + s * ry * 0.48, zc), 0.035, 0.012,
                 "mech_lens_cyan", bevel=0.004)
    ocyl("vent", (4.35, CY, _z(4.35) + rz - 0.05), (4.35, CY, _z(4.35) + rz + 0.06), 0.2, "mech_soot", segs=20)
    ocyl("vent_core", (4.35, CY, _z(4.35) + rz + 0.05), (4.35, CY, _z(4.35) + rz + 0.075), 0.12, "mech_lens_cyan",
         segs=18)
    # plate shards thrown clear
    for k, (x, y, rot) in enumerate(((0.35, -0.35, 20), (1.5, -2.75, -35), (3.2, -0.25, 60), (0.25, -2.6, -10))):
        c, d = math.cos(math.radians(rot)), math.sin(math.radians(rot))
        hull("shard%d" % k, [(x + c * 0.2, y + d * 0.2, 0.0), (x - c * 0.18, y - d * 0.18, 0.0),
                             (x - d * 0.12, y + c * 0.12, 0.0), (x + d * 0.08, y - c * 0.08, 0.07)],
             "mech_gunmetal", bevel=0.008)
    return (6, 3), _z(X0) + rz + 0.5


def build(only=None):
    """The ship part sprite (defense/ship_part)."""
    fp, zmax = _ship_part()
    out = [S.render_asset(FAMILY, "ship_part", footprint=fp, zmax=zmax, shadow_kind="box",
                          extra={"building": "ship part", "rotatable": False, "facing": "south"})]
    return out
