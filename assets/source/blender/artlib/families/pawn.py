"""Compositional pawns (ART spec §10) and the first weapon (§15).

Layers share one canvas/origin per facing and are drawn in this order:
    body (skin) -> pants -> shirt -> vest -> hair -> held equipment
Every non-body layer is rendered with the body as a holdout, so hidden
surfaces are already removed and any subset of layers composites
correctly. Neutral greys + tint masks give skin/hair/cloth variety.

Original proportions: compact geometric torso, short legs, large head.
"""
import bmesh

from .. import contracts as C
from .. import stage as S

FACINGS = ("south", "east", "north", "west")
CX, CY = 0.5, -0.52
ZMAX = 1.10
MARGINS = (0.125, 0.125, 0.10, 0.125)
HEAD_Z = 0.84


def _body():
    objs = []
    for sx in (-1, 1):
        objs.append(S.cyl("leg", CX + sx * 0.075, CY, 0.062, 0.0, 0.32, "skin", segs=10))
        objs.append(S.cyl("arm", CX + sx * 0.19, CY, 0.048, 0.33, 0.60, "skin", segs=10))
        objs.append(S.ellipsoid("hand", CX + sx * 0.195, CY - 0.01, 0.31, 0.055, 0.055, 0.05, "skin", segs=10, rings=6))
    objs.append(S.box("torso", CX - 0.15, CY - 0.10, 0.28, CX + 0.15, CY + 0.10, 0.63, "skin", bevel=0.06, segments=2))
    objs.append(S.cyl("neck", CX, CY, 0.06, 0.60, 0.70, "skin", segs=10))
    objs.append(S.ellipsoid("head", CX, CY, HEAD_Z, 0.19, 0.175, 0.19, "skin", segs=24, rings=14))
    for sx in (-1, 1):
        objs.append(S.ellipsoid("eye", CX + sx * 0.068, CY - 0.158, HEAD_Z - 0.01, 0.026, 0.02, 0.034,
                                "metal_dark", segs=10, rings=6))
    objs.append(S.ellipsoid("nose", CX, CY - 0.175, HEAD_Z - 0.06, 0.03, 0.02, 0.03, "skin", segs=8, rings=6))
    return objs


def _pants():
    for sx in (-1, 1):
        S.cyl("pleg", CX + sx * 0.075, CY, 0.074, 0.05, 0.34, "cloth", segs=12)
    S.box("hips", CX - 0.16, CY - 0.11, 0.26, CX + 0.16, CY + 0.11, 0.40, "cloth", bevel=0.05, segments=2)
    S.box("belt", CX - 0.165, CY - 0.115, 0.38, CX + 0.165, CY + 0.115, 0.42, "leather", bevel=0.02)


def _shirt():
    S.box("shirt", CX - 0.162, CY - 0.112, 0.36, CX + 0.162, CY + 0.112, 0.645, "cloth", bevel=0.07, segments=2)
    for sx in (-1, 1):
        S.cyl("sleeve", CX + sx * 0.19, CY, 0.06, 0.45, 0.62, "cloth", segs=12)
    S.cyl("collar", CX, CY, 0.075, 0.62, 0.67, "cloth", segs=12)


def _vest():
    S.box("vest", CX - 0.172, CY - 0.122, 0.40, CX + 0.172, CY + 0.122, 0.635, "cloth", bevel=0.06, segments=2)
    for sx in (-1, 1):
        S.box("pocket", CX + sx * 0.085 - 0.05, CY - 0.135, 0.43, CX + sx * 0.085 + 0.05, CY - 0.10, 0.52,
              "leather", bevel=0.012)
        S.box("strap", CX + sx * 0.09 - 0.025, CY - 0.127, 0.40, CX + sx * 0.09 + 0.025, CY + 0.127, 0.645,
              "leather", bevel=0.008)


def _cut(ob, keep):
    """Delete vertices of a mesh for which keep(x, y, z) is False."""
    bm = bmesh.new()
    bm.from_mesh(ob.data)
    kill = [v for v in bm.verts if not keep(v.co.x, v.co.y, v.co.z)]
    bmesh.ops.delete(bm, geom=kill, context="VERTS")
    bm.to_mesh(ob.data)
    bm.free()
    ob.data.update()


def _hair_shell(name, grow, keep):
    ob = S.ellipsoid(name, CX, CY, HEAD_Z, 0.19 + grow, 0.175 + grow, 0.19 + grow, "hair", segs=28, rings=16)
    _cut(ob, keep)
    # give the shell thickness so the outline has a body to follow
    sol = ob.modifiers.new("solid", "SOLIDIFY")
    sol.thickness = 0.02
    sol.offset = -1
    return ob


def _hair(style):
    front = CY - 0.06  # y below this is "face side"
    if style == "hair_short":
        _hair_shell("hair", 0.025, lambda x, y, z: z > HEAD_Z + 0.05 or (y > front and z > HEAD_Z - 0.12))
        S.box("fringe", CX - 0.12, CY - 0.19, HEAD_Z + 0.07, CX + 0.06, CY - 0.13, HEAD_Z + 0.15, "hair",
              bevel=0.025)
    elif style == "hair_crop":
        _hair_shell("hair", 0.012, lambda x, y, z: z > HEAD_Z + 0.09 or (y > CY + 0.02 and z > HEAD_Z - 0.04))
    elif style == "hair_bun":
        _hair_shell("hair", 0.022, lambda x, y, z: z > HEAD_Z + 0.03 or (y > front and z > HEAD_Z - 0.16))
        S.ellipsoid("bun", CX, CY + 0.17, HEAD_Z + 0.12, 0.085, 0.08, 0.085, "hair", segs=14, rings=8)
        S.cyl("tie", CX, CY + 0.13, 0.05, HEAD_Z + 0.08, HEAD_Z + 0.10, "food_red", segs=10, ry=0.04)
    else:
        raise KeyError(style)


def _apply_modifiers():
    import bpy
    for ob in S.objects():
        if ob.modifiers:
            dg = bpy.context.evaluated_depsgraph_get()
            me = bpy.data.meshes.new_from_object(ob.evaluated_get(dg))
            old = ob.data
            ob.modifiers.clear()
            ob.data = me
            bpy.data.meshes.remove(old)


def _lean_matrix():
    import math
    from mathutils import Matrix
    t = math.radians(C.FIGURE_LEAN_DEG)
    feet = Matrix.Translation((0.5, CY, 0))
    return (feet @ Matrix.Rotation(-t, 4, "X") @ Matrix.Scale(C.FIGURE_SCALE, 4)
            @ Matrix.Translation((-0.5, -CY, 0)))


def _figure(facing):
    """Facing rotation, then the figure lean (always towards the viewer)."""
    S.face(S.objects(), facing, (1, 1))
    S.transform(S.objects(), _lean_matrix())


def _zmax_screen():
    """Equivalent sheared height that covers the leaned figure."""
    import math
    t = math.radians(C.FIGURE_LEAN_DEG)
    hgt = 1.05 * C.FIGURE_SCALE
    return hgt * (math.sin(t) / C.SHEAR_K + math.cos(t)) + 0.05


def _project(x, y, z, origin):
    """Blender point -> image px on the pawn canvas (sheared)."""
    return [round(origin[0] + x * C.PPT, 1), round(origin[1] + (-y - C.SHEAR_K * z) * C.PPT, 1)]


HAND = {  # where held equipment's grip goes, per facing (Blender coords before facing rotation)
    "south": (CX + 0.195, CY - 0.06, 0.34),
}


def _hand(facing):
    import math
    from mathutils import Matrix, Vector
    deg = S.FACINGS[facing]
    m = (Matrix.Translation((0.5, -0.5, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-0.5, 0.5, 0)))
    return _lean_matrix() @ m @ Vector(HAND["south"])


def _layer(name, facing, builder, body_holdout=True):
    if body_holdout:
        objs = _body()
        S.holdout(objs)
    builder()
    _apply_modifiers()
    _figure(facing)
    zm = _zmax_screen()
    top = int(-(-((C.SHEAR_K * zm + MARGINS[2]) * C.PPT) // 1))
    origin = (int(round(MARGINS[0] * C.PPT)), top)
    h = _hand(facing)
    return S.render_asset("pawn", "%s_%s" % (name, facing), zmax=zm, margins=MARGINS,
                          shadow=(name == "body"),
                          extra={"layer": name, "facing": facing,
                                 "attachments": {"hand": _project(h.x, h.y, h.z, origin)}})


LAYERS = {
    "pants_basic": _pants,
    "shirt_basic": _shirt,
    "vest_utility": _vest,
    "hair_short": lambda: _hair("hair_short"),
    "hair_crop": lambda: _hair("hair_crop"),
    "hair_bun": lambda: _hair("hair_bun"),
}


def _rifle():
    """Lying flat, muzzle pointing east (+X). Grip/muzzle in image px."""
    y = -0.5
    S.box("stock", 0.06, y - 0.045, 0.0, 0.30, y + 0.045, 0.06, "wood", bevel=0.02)
    S.box("receiver", 0.28, y - 0.04, 0.0, 0.58, y + 0.04, 0.07, "metal_dark", bevel=0.012)
    S.box("grip", 0.30, y - 0.10, 0.0, 0.36, y - 0.03, 0.05, "wood", bevel=0.012)
    S.box("mag", 0.44, y - 0.13, 0.0, 0.51, y - 0.03, 0.045, "metal_dark", bevel=0.01)
    S.box("guard", 0.56, y - 0.035, 0.0, 0.74, y + 0.035, 0.055, "wood", bevel=0.012)
    S.cyl("barrel", 0, 0, 0.018, 0.0, 0.24, "steel", segs=8, bevel=0.0)
    import math
    from mathutils import Matrix
    S.objects()[-1].matrix_world = Matrix.Translation((0.72, y, 0.03)) @ Matrix.Rotation(math.radians(90), 4, "Y")
    S.box("sight", 0.40, y - 0.015, 0.07, 0.46, y + 0.015, 0.10, "metal_dark", bevel=0.005)
    S.transform(S.objects(), Matrix.Translation((0.5, y, 0)) @ Matrix.Scale(0.8, 4) @ Matrix.Translation((-0.5, -y, 0)))
    m = (int(round(0.125 * C.PPT)), int(-(-((C.SHEAR_K * 0.11 + 0.125) * C.PPT) // 1)))
    return S.render_asset("weapon", "rifle_basic", zmax=0.11,
                          extra={"weapon": "rifle", "rotatable": True, "points": "east",
                                 "attachments": {"grip": _project(0.5 + (0.33 - 0.5) * 0.8, y - 0.056, 0.016, m),
                                                 "muzzle": _project(0.5 + (0.96 - 0.5) * 0.8, y, 0.024, m)}})


def build():
    out = []
    for f in FACINGS:
        _body()
        _apply_modifiers()
        _figure(f)
        zm = _zmax_screen()
        top = int(-(-((C.SHEAR_K * zm + MARGINS[2]) * C.PPT) // 1))
        origin = (int(round(MARGINS[0] * C.PPT)), top)
        h = _hand(f)
        out.append(S.render_asset("pawn", "body_%s" % f, zmax=zm, margins=MARGINS,
                                  extra={"layer": "body", "facing": f,
                                         "attachments": {"hand": _project(h.x, h.y, h.z, origin)}}))
        for name, fn in LAYERS.items():
            out.append(_layer(name, f, fn))
    out.append(_rifle())
    return out
