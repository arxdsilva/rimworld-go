"""The one canonical stage (ART spec §21): camera, light, toon materials,
geometry helpers, render + export. Family modules must not create their
own camera, light, or render settings.
"""
import math
import os
import tempfile
from contextlib import contextmanager

import bmesh
import bpy
import numpy as np
from mathutils import Matrix, Vector

from . import contracts as C
from . import meta, palette, post

STAGE = "rwg_stage"
_PAL = None


# ---------------------------------------------------------------- scene
def stage():
    sc = bpy.data.scenes.get(STAGE) or bpy.data.scenes.new(STAGE)
    r = sc.render
    r.engine = "BLENDER_EEVEE"
    r.film_transparent = True
    r.filter_size = 0.75
    r.resolution_percentage = 100
    r.image_settings.file_format = "PNG"
    r.image_settings.color_mode = "RGBA"
    r.image_settings.color_depth = "8"
    sc.view_settings.view_transform = "Standard"
    sc.view_settings.look = "None"
    sc.view_settings.exposure = 0.0
    sc.view_settings.gamma = 1.0
    sc.eevee.taa_render_samples = 16
    sc.eevee.use_shadows = True
    sc.eevee.use_raytracing = False
    if sc.world is None:
        sc.world = bpy.data.worlds.get("rwg_world") or bpy.data.worlds.new("rwg_world")
    sc.world.use_nodes = True
    bg = sc.world.node_tree.nodes.get("Background")
    if bg:
        bg.inputs[0].default_value = (0, 0, 0, 1)
        bg.inputs[1].default_value = 0.0
    win = getattr(bpy.context, "window", None)
    if win is not None and win.scene != sc:
        win.scene = sc
    return sc


def clear():
    sc = stage()
    sc.pop("rwg_sheared", None)
    for ob in list(sc.collection.all_objects):
        data = ob.data
        bpy.data.objects.remove(ob, do_unlink=True)
        if data is not None and data.users == 0:
            if isinstance(data, bpy.types.Mesh):
                bpy.data.meshes.remove(data)
            elif isinstance(data, bpy.types.Camera):
                bpy.data.cameras.remove(data)
            elif isinstance(data, bpy.types.Light):
                bpy.data.lights.remove(data)
    return sc


def _link(ob):
    stage().collection.objects.link(ob)
    return ob


# ------------------------------------------------------------ materials
def pal():
    global _PAL
    if _PAL is None:
        _PAL = palette.load()
    return _PAL


def reset_palette():
    global _PAL
    _PAL = None
    for m in list(bpy.data.materials):
        if m.name.startswith("rwg_"):
            bpy.data.materials.remove(m)


def mat(name):
    """Toon material bound to a palette entry (three exact colour bands)."""
    key = "rwg_" + name
    m = bpy.data.materials.get(key)
    if m:
        return m
    mats, emis = pal()
    m = bpy.data.materials.new(key)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    em = nt.nodes.new("ShaderNodeEmission")
    em.inputs["Strength"].default_value = 1.0
    nt.links.new(em.outputs[0], out.inputs["Surface"])
    if name in emis:
        em.inputs["Color"].default_value = (*palette.hex_to_linear(emis[name]), 1)
        return m
    if name not in mats:
        raise KeyError("unknown palette material %r" % name)
    p = mats[name]
    dif = nt.nodes.new("ShaderNodeBsdfDiffuse")
    dif.inputs["Color"].default_value = (1, 1, 1, 1)
    s2r = nt.nodes.new("ShaderNodeShaderToRGB")
    ramp = nt.nodes.new("ShaderNodeValToRGB")
    cr = ramp.color_ramp
    stops = ramp_stops()
    cr.interpolation = C.RAMP_INTERP
    cr.elements[0].position = stops[0][0]
    cr.elements[1].position = stops[1][0]
    for pos, _ in stops[2:]:
        cr.elements.new(pos)
    _color_ramp(cr, p)
    nt.links.new(dif.outputs[0], s2r.inputs[0])
    nt.links.new(s2r.outputs["Color"], ramp.inputs["Fac"])
    nt.links.new(ramp.outputs["Color"], em.inputs["Color"])
    return m


RAMP_INTERPS = ("CONSTANT", "EASE", "LINEAR")


def ramp_stops():
    """(position, band) colour-ramp stops over Lambert intensity. CONSTANT:
    three exact bands switching at SHADOW_T and LIGHT_T. EASE / LINEAR: the
    same bands, blended over RAMP_SOFT either side of each threshold."""
    if C.RAMP_INTERP not in RAMP_INTERPS:
        raise ValueError("contracts.RAMP_INTERP must be one of %s, got %r" % (RAMP_INTERPS, C.RAMP_INTERP))
    if C.RAMP_INTERP == "CONSTANT":
        return [(0.0, "shadow"), (C.SHADOW_T, "base"), (C.LIGHT_T, "light")]
    s = C.RAMP_SOFT
    return [(0.0, "shadow"), (max(0.0, C.SHADOW_T - s), "shadow"), (C.SHADOW_T + s, "base"),
            (C.LIGHT_T - s, "base"), (min(1.0, C.LIGHT_T + s), "light")]


def _color_ramp(cr, bands):
    """Colour a toon ramp built from ramp_stops(); bands maps band -> hex."""
    els = sorted(cr.elements, key=lambda e: e.position)
    for el, (_, band) in zip(els, ramp_stops()):
        el.color = (*palette.hex_to_linear(bands[band]), 1)


# ------------------------------------------------------------- geometry
def _obj(name, bm, material, smooth=False):
    me = bpy.data.meshes.new(name)
    bm.to_mesh(me)
    bm.free()
    if material:
        me.materials.append(mat(material))
    for p in me.polygons:
        p.use_smooth = smooth
    return _link(bpy.data.objects.new(name, me))


def _bevel(bm, amount, segments=1):
    if amount > 0:
        bmesh.ops.bevel(bm, geom=list(bm.edges) + list(bm.verts), offset=amount,
                        segments=segments, affect="EDGES", profile=0.5, clamp_overlap=True)


def box(name, x0, y0, z0, x1, y1, z1, material, bevel=0.025, segments=1):
    bm = bmesh.new()
    bmesh.ops.create_cube(bm, size=1.0)
    for v in bm.verts:
        v.co.x = x0 + (v.co.x + 0.5) * (x1 - x0)
        v.co.y = y0 + (v.co.y + 0.5) * (y1 - y0)
        v.co.z = z0 + (v.co.z + 0.5) * (z1 - z0)
    _bevel(bm, min(bevel, (x1 - x0) / 2.2, (y1 - y0) / 2.2, (z1 - z0) / 2.2), segments)
    return _obj(name, bm, material)


def cyl(name, cx, cy, r, z0, z1, material, segs=16, r_top=None, bevel=0.02, ry=None):
    bm = bmesh.new()
    rt = r if r_top is None else r_top
    bmesh.ops.create_cone(bm, cap_ends=True, cap_tris=False, segments=segs,
                          radius1=r, radius2=rt, depth=1.0)
    for v in bm.verts:
        v.co.z = z0 + (v.co.z + 0.5) * (z1 - z0)
        if ry is not None:
            v.co.y *= ry / r
        v.co.x += cx
        v.co.y += cy
    _bevel(bm, min(bevel, (z1 - z0) / 2.2, r / 2.2), 1)
    return _obj(name, bm, material, smooth=True)


def ellipsoid(name, cx, cy, cz, rx, ry, rz, material, segs=20, rings=12):
    bm = bmesh.new()
    bmesh.ops.create_uvsphere(bm, u_segments=segs, v_segments=rings, radius=1.0)
    for v in bm.verts:
        v.co.x = cx + v.co.x * rx
        v.co.y = cy + v.co.y * ry
        v.co.z = cz + v.co.z * rz
    return _obj(name, bm, material, smooth=True)


def blob(name, cx, cy, cz, r, material, seed, lumps=0.18, segs=18, rings=10, squash=0.75):
    """Irregular rounded rock/bush: deterministic per seed."""
    rng = np.random.RandomState(seed)
    bm = bmesh.new()
    bmesh.ops.create_icosphere(bm, subdivisions=2, radius=1.0)
    dirs = rng.normal(size=(5, 3))
    dirs /= np.linalg.norm(dirs, axis=1, keepdims=True)
    amp = rng.uniform(-lumps, lumps, size=5)
    for v in bm.verts:
        n = np.array(v.co[:]) / max(v.co.length, 1e-6)
        k = 1.0 + float(np.sum(amp * np.maximum(dirs @ n, 0) ** 2))
        v.co.x = cx + n[0] * r * k
        v.co.y = cy + n[1] * r * k
        v.co.z = max(0.0, cz + n[2] * r * k * squash)
    return _obj(name, bm, material, smooth=False)


def prism(name, pts, z0, z1, material, bevel=0.02):
    """Extruded polygon (pts in XY, counter-clockwise)."""
    bm = bmesh.new()
    vs = [bm.verts.new((x, y, z0)) for x, y in pts]
    f = bm.faces.new(vs)
    r = bmesh.ops.extrude_face_region(bm, geom=[f])
    for v in [e for e in r["geom"] if isinstance(e, bmesh.types.BMVert)]:
        v.co.z = z1
    bmesh.ops.recalc_face_normals(bm, faces=bm.faces[:])
    _bevel(bm, bevel, 1)
    return _obj(name, bm, material)


FACINGS = {"south": 0, "east": 90, "north": 180, "west": -90}


def face(objs, facing, footprint=(1, 1)):
    """Rotate geometry authored facing south to `facing`, keeping the
    footprint's top-left at the origin. Returns the new footprint."""
    w, h = footprint
    deg = FACINGS[facing]
    nw, nh = (h, w) if deg in (90, -90) else (w, h)
    m = (Matrix.Translation((nw / 2, -nh / 2, 0)) @ Matrix.Rotation(math.radians(deg), 4, "Z")
         @ Matrix.Translation((-w / 2, h / 2, 0)))
    transform(objs, m)
    return (nw, nh)


def objects():
    return [o for o in stage().collection.all_objects if o.type == "MESH"]


def transform(objs, matrix):
    for ob in objs:
        ob.matrix_world = matrix @ ob.matrix_world


def rotate_z(objs, degrees, pivot=(0.5, -0.5)):
    m = (Matrix.Translation((pivot[0], pivot[1], 0)) @ Matrix.Rotation(math.radians(degrees), 4, "Z")
         @ Matrix.Translation((-pivot[0], -pivot[1], 0)))
    transform(objs, m)


# ---------------------------------------------------------------- render
def _shear_all(sc):
    S = Matrix(((1, 0, 0, 0), (0, 1, C.SHEAR_K, 0), (0, 0, 1, 0), (0, 0, 0, 1)))
    for ob in sc.collection.all_objects:
        if ob.type == "MESH":
            ob.data.transform(S @ ob.matrix_world)
            ob.matrix_world = Matrix.Identity(4)
            ob.data.update()


def _rig(sc, x_min, y_max, w_px, h_px):
    """The canonical camera and sun; reused when the stage already has them
    (multi-pass scene renders)."""
    cam = sc.camera if sc.camera is not None and sc.camera.name.startswith("rwg_cam") else None
    if cam is None:
        cam_data = bpy.data.cameras.new("rwg_cam")
        cam_data.type = "ORTHO"
        cam_data.clip_start = 0.1
        cam_data.clip_end = 200
        cam = _link(bpy.data.objects.new("rwg_cam", cam_data))
        sc.camera = cam
        sun_data = bpy.data.lights.new("rwg_sun", "SUN")
        sun_data.energy = math.pi
        sun_data.angle = math.radians(1.0)
        sun_data.use_shadow = True
        sun = _link(bpy.data.objects.new("rwg_sun", sun_data))
        L = Vector(C.light_vector())
        sun.rotation_euler = Vector((0, 0, -1)).rotation_difference(-L).to_euler()
    cam.data.ortho_scale = max(w_px, h_px) / C.PPT
    cam.location = (x_min + w_px / C.PPT / 2, y_max - h_px / C.PPT / 2, 100)
    sc.render.resolution_x = w_px
    sc.render.resolution_y = h_px


def _read_png(path):
    im = bpy.data.images.load(path, check_existing=False)
    w, h = im.size
    px = np.empty(w * h * 4, np.float32)
    im.pixels.foreach_get(px)
    bpy.data.images.remove(im)
    return px.reshape(h, w, 4)[::-1].copy()


def write_png(arr, path):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    h, w = arr.shape[:2]
    im = bpy.data.images.new("rwg_out", w, h, alpha=True, float_buffer=False)
    im.colorspace_settings.name = "sRGB"
    im.alpha_mode = "STRAIGHT"
    q = np.round(np.clip(arr[::-1], 0, 1) * 255.0) / 255.0
    im.pixels.foreach_set(q.astype(np.float32).ravel())
    im.filepath_raw = path
    im.file_format = "PNG"
    im.save()
    bpy.data.images.remove(im)


def out_path(family, name):
    return os.path.join(C.GENERATED_DIR, family, name + ".png")


def _flat(name, rgb=None, holdout=False):
    key = "rwg__" + name
    m = bpy.data.materials.get(key)
    if m:
        return m
    m = bpy.data.materials.new(key)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    if holdout:
        sh = nt.nodes.new("ShaderNodeHoldout")
    else:
        sh = nt.nodes.new("ShaderNodeEmission")
        sh.inputs["Color"].default_value = (*rgb, 1)
        sh.inputs["Strength"].default_value = 1.0
    nt.links.new(sh.outputs[0], out.inputs["Surface"])
    return m


def _tint_group(ob):
    mats, _ = pal()
    for slot in ob.material_slots:
        if slot.material and slot.material.name.startswith("rwg_"):
            name = slot.material.name[4:]
            if name in mats and mats[name]["tint"]:
                return name
    return None


def _render_raw(sc):
    # per process: parallel headless builds must not share one file
    tmp = os.path.join(tempfile.gettempdir(), "rwg_render_%d.png" % os.getpid())
    sc.render.filepath = tmp
    bpy.ops.render.render(write_still=True, scene=sc.name)
    return _read_png(tmp)


def _swap(sc, pick):
    saved = []
    for ob in sc.collection.all_objects:
        if ob.type != "MESH":
            continue
        saved.append((ob, [s.material for s in ob.material_slots]))
        m = pick(ob)
        for s in ob.material_slots:
            s.material = m
    return saved


def _restore(saved):
    for ob, ms in saved:
        for s, m in zip(ob.material_slots, ms):
            s.material = m


def holdout(objs):
    """Mark objects as occluders that never appear in the sprite."""
    for ob in objs:
        ob["rwg_holdout"] = True
        for s in ob.material_slots:
            s.material = _flat("holdout", holdout=True)


def _clip(img, origin, footprint, clip, zclip):
    """Remove pixels past connected footprint edges (seamless autotiles)."""
    left, top = origin
    w, h = footprint
    zpx = int(round(C.SHEAR_K * zclip * C.PPT))
    H, W = img.shape[:2]
    if "W" in clip:
        img[:, :left] = 0
    if "E" in clip:
        img[:, left + w * C.PPT:] = 0
    if "N" in clip:
        img[:max(0, top - zpx), :] = 0
    if "S" in clip:
        img[top + h * C.PPT - zpx:, :] = 0
    return img


SHADOW_KINDS = {"pawn": "blob", "animal": "blob", "plant": "blob", "item": "blob", "gear": "blob",
                "structure": "box", "furniture": "box", "rock": "box",
                "terrain": "none", "overlay": "none", "weather": "none"}
FLAT_H = 0.05   # tiles: casters this low cast no shadow


def figure_lean(family):
    """Degrees an upright figure family is leaned back towards the viewer
    (contracts), or None for families drawn with the plain shear."""
    return {"pawn": C.FIGURE_LEAN_DEG, "animal": C.ANIMAL_LEAN_DEG}.get(family)


def _shadow_meta(family, name, kind=None, footprint=None):
    """Caster bounds (tiles) from the unsheared, non-holdout meshes.

    Figures are modelled leaned back towards the camera; their bounds are
    measured after undoing the lean so h is the standing height and w/d
    the ground footprint. Structures are clamped to their footprint
    (autotile posts overhang to join neighbours)."""
    unlean = None
    if figure_lean(family) is not None:
        unlean = Matrix.Rotation(math.radians(figure_lean(family)), 4, "X")
    lo = np.full(3, 1e9)
    hi = np.full(3, -1e9)
    for ob in stage().collection.all_objects:
        if ob.type != "MESH" or ob.get("rwg_holdout"):
            continue
        n = len(ob.data.vertices)
        if not n:
            continue
        co = np.empty(n * 3, np.float32)
        ob.data.vertices.foreach_get("co", co)
        m = np.array(ob.matrix_world if unlean is None else unlean @ ob.matrix_world)
        v = co.reshape(n, 3) @ m[:3, :3].T + m[:3, 3]
        lo, hi = np.minimum(lo, v.min(axis=0)), np.maximum(hi, v.max(axis=0))
    if hi[0] < lo[0]:
        lo = hi = np.zeros(3)
    w, d = float(hi[0] - lo[0]), float(hi[1] - lo[1])
    h = float(hi[2] - lo[2]) if unlean is not None else float(max(hi[2], 0.0))
    if footprint is not None and family in ("structure", "furniture", "rock"):
        w, d = min(w, footprint[0]), min(d, footprint[1])
    k = kind or SHADOW_KINDS.get(family, "none")
    if kind is None and family == "structure" and name.startswith("floor_"):
        k = "none"
    if h <= FLAT_H:
        k = "none"
    return {"kind": k, "w": round(w, 3), "h": round(h, 3), "d": round(d, 3)}


def _figure_cover(sc, family):
    """Layered figure sprites (contracts.OUTLINE_INNER_FAMILIES) are rendered
    over a holdout of the figure beneath them (pawn apparel and hair over the
    body). Returns where the composited figure covers the canvas (this layer
    or the holdout), so post.outline caps the contour that would land on the
    figure and keeps the full width where it meets transparency. None for
    every other sprite (holdouts there are scenery occluders, not figure)."""
    if family not in C.OUTLINE_INNER_FAMILIES:
        return None
    if not any(o.get("rwg_holdout") for o in sc.collection.all_objects if o.type == "MESH"):
        return None
    white = _flat("mask_on", (1, 1, 1))
    saved = _swap(sc, lambda o: white)
    try:
        cover = _render_raw(sc)
    finally:
        _restore(saved)
    return cover[..., 3] > 0.5


def render_asset(family, name, footprint=(1, 1), zmax=1.0, margins=None,
                 outline=True, shadow=False, extra=None, keep=False,
                 clip=(), zclip=None, tint=True, shadow_kind=None, outline_width=None):
    """Shear, frame, render, post-process, write PNG + metadata.

    footprint: (w, h) tiles. zmax: tallest point (m). margins: tiles
    (left, right, top, bottom) beyond the footprint/overhang.
    shadow: bake a UI-art drop shadow (world sprites never do; the game
    casts sun shadows from the `shadow:` metadata). shadow_kind overrides
    the per-family caster kind (blob|box|none).
    """
    sc = stage()
    shadow_meta = _shadow_meta(family, name, shadow_kind, footprint)
    w, h = footprint
    ml, mr, mt, mb = margins or (C.DEFAULT_MARGIN,) * 4
    left = int(round(ml * C.PPT))
    right = int(round(mr * C.PPT))
    top = int(math.ceil((C.SHEAR_K * zmax + mt) * C.PPT))
    bottom = int(round(mb * C.PPT))
    W = left + w * C.PPT + right
    H = top + h * C.PPT + bottom
    _shear_all(sc)
    _rig(sc, -left / C.PPT, top / C.PPT, W, H)
    groups = {g for g in (_tint_group(o) for o in sc.collection.all_objects if o.type == "MESH") if g}
    if len(groups) > 1:
        raise ValueError("%s: one tint group per sprite, got %s" % (name, sorted(groups)))
    raw = _render_raw(sc)
    zc = zmax if zclip is None else zclip
    if clip:
        raw = _clip(raw, (left, top), (w, h), clip, zc)
    inner = _figure_cover(sc, family) if outline else None
    if inner is not None and clip:
        inner = _clip(inner[..., None].astype(np.float32), (left, top), (w, h), clip, zc)[..., 0] > 0.5
    img = post.process(raw, do_outline=outline, do_shadow=shadow, width=outline_width, inner=inner)
    if clip:
        img = _clip(img, (left, top), (w, h), clip, zc)
    path = out_path(family, name)
    write_png(img, path)
    data = {
        "image": os.path.relpath(path, C.OUT_ROOT).replace(os.sep, "/"),
        "scale": C.SCALE,
        "canvas": [W, H],
        "footprint": [w, h],
        "origin_px": [left, top],
        "projection": {"shear_k": C.SHEAR_K},
        "shadow": shadow_meta,
    }
    if groups and tint:
        white, black = _flat("mask_on", (1, 1, 1)), _flat("mask_off", (0, 0, 0))
        hold = _flat("holdout", holdout=True)
        saved = _swap(sc, lambda o: hold if o.get("rwg_holdout") else (white if _tint_group(o) else black))
        m = _render_raw(sc)
        _restore(saved)
        m[..., 3] = raw[..., 3]
        if clip:
            m = _clip(m, (left, top), (w, h), clip, zc)
        if outline:
            m = post.outline(m, width=outline_width, darken=1.0, saturate=1.0, inner=inner)
        mask = np.zeros_like(m)
        mask[..., :3] = m[..., :1]
        mask[..., 3] = 1.0
        mpath = out_path(family, name + "_mask")
        write_png(mask, mpath)
        data["tint"] = {"group": groups.pop(),
                        "mask": os.path.relpath(mpath, C.OUT_ROOT).replace(os.sep, "/")}
    if extra:
        data.update(extra)
    meta.write(family, name, data)
    if not keep:
        clear()
    return path


# ------------------------------------------------------- whole scenes
# Scenes (artlib/scenes) compose many family builders into one render
# through the same camera, light, shear and post-pass as sprites.

@contextmanager
def isolate():
    """Park the stage's objects while one builder runs, so builders that
    act on S.objects() (posing, clear()) only see their own piece."""
    sc = stage()
    park = bpy.data.collections.get("rwg_parked") or bpy.data.collections.new("rwg_parked")
    park.use_fake_user = True
    moved = list(sc.collection.objects)
    for ob in moved:
        park.objects.link(ob)
        sc.collection.objects.unlink(ob)
    try:
        yield
    finally:
        for ob in list(park.objects):
            sc.collection.objects.link(ob)
            park.objects.unlink(ob)


@contextmanager
def no_render():
    """Builders that render themselves (render_asset) only build."""
    global render_asset
    real = render_asset
    render_asset = lambda *a, **k: None  # noqa: E731
    try:
        yield
    finally:
        render_asset = real


def place(builder, x, y, facing=None, tag=None):
    """Build one piece alone and move it to game tile (x, y) (top-left of
    its footprint). builder may return (footprint, zmax); facing rotates
    south-authored geometry. Returns the piece's objects."""
    with isolate(), no_render():
        ret = builder()
        objs = objects()
        fp = ret[0] if isinstance(ret, tuple) and ret and isinstance(ret[0], tuple) else (1, 1)
        if facing and facing != "south":
            face(objs, facing, fp)
        transform(objs, Matrix.Translation((x, -y, 0)))
        for ob in objs:
            ob["rwg_piece"] = tag or builder.__name__
    return objs


def mat_tinted(name, hex_):
    """Palette material with the runtime multiply tint baked in (sRGB)."""
    key = "rwg_%s@%s" % (name, hex_.lstrip("#"))
    m = bpy.data.materials.get(key)
    if m:
        return m
    p = dict(pal()[0][name])
    t = palette.hex_to_srgb(hex_)
    for band in ("light", "base", "shadow"):
        c = palette.hex_to_srgb(p[band])
        p[band] = "#" + "".join("%02x" % int(round(min(1, ci * ti) * 255)) for ci, ti in zip(c, t))
    m = mat(name).copy()
    m.name = key
    ramp = next(n for n in m.node_tree.nodes if n.type == "VALTORGB")
    _color_ramp(ramp.color_ramp, p)
    return m


@contextmanager
def swap_materials(pick):
    """Temporarily give every mesh the material pick(object) returns."""
    saved = _swap(stage(), pick)
    try:
        yield
    finally:
        _restore(saved)


def flat(name, rgb=None, holdout=False):
    return _flat(name, rgb, holdout)


def render_scene(x_min, y_max, w_px, h_px, outline=True, scale=1.0):
    """Render the whole stage through the canonical rig: frame top-left at
    Blender (x_min, y_max), w_px x h_px (times scale, for previews). Shears
    once; never writes or clears. Returns the post-processed RGBA image."""
    sc = stage()
    if not sc.get("rwg_sheared"):
        _shear_all(sc)
        sc["rwg_sheared"] = True
    _rig(sc, x_min, y_max, w_px, h_px)
    sc.render.resolution_percentage = int(round(scale * 100))
    return post.process(_render_raw(sc), do_outline=outline, do_shadow=False)


TEMPLATE = os.path.join(C.SOURCE_DIR, "template.blend")


def save_template():
    """Write the canonical stage (camera, key light, reference tile, sample
    materials) to template.blend for humans opening the source in Blender."""
    clear()
    sc = stage()
    box("reference_tile", 0.0, -1.0, -0.02, 1.0, 0.0, 0.0, "soil", bevel=0.0)
    box("reference_block", 0.25, -0.75, 0.0, 0.75, -0.25, 0.5, "wood", bevel=0.03)
    for name in pal()[0]:
        mat(name)
    _shear_all(sc)
    left = top = int(round(C.DEFAULT_MARGIN * C.PPT))
    top = int(math.ceil((C.SHEAR_K * 0.5 + C.DEFAULT_MARGIN) * C.PPT))
    _rig(sc, -left / C.PPT, top / C.PPT, 2 * left + C.PPT, top + C.PPT + left)
    sc["rwg_contracts"] = "see artlib/contracts.py; render via build.py, never by hand"
    blocks = {sc} | set(sc.collection.all_objects) | {m for m in bpy.data.materials if m.name.startswith("rwg_")}
    bpy.data.libraries.write(TEMPLATE, blocks, fake_user=True, compress=True)
    clear()
    return TEMPLATE


# ------------------------------------------------------- point lights
# Additive (PLAY-020): scenes may light a roofed room with lamps. Sprites
# never do: render_asset and _rig only ever see the sun, and a point light
# is created hidden, so it shows only inside lighting(points=...).
# Positions are game tiles (x east, y south, z up); a light sits where the
# sheared render draws that point. The shear keeps the ground plane, so
# shadows on the ground land where the unsheared lamp would cast them.
#
# POINT_POWER makes Shader-to-RGB of a white diffuse read cos / d^2 (d in
# sheared tiles), as the sun (energy pi) reads cos: point_nodes multiplies
# d^2 back out, so a toon band follows only the direction to the lamp and
# the falloff is ours, (1 - (d/reach)^2)^2 over the real distance.
POINT_POWER = 4 * math.pi ** 2


def sheared(x, y, z):
    """Blender position of game point (x, y, z) once the stage is sheared."""
    return (x, -y + C.SHEAR_K * z, z)


def point_light(name, x, y, z, rgb=(1.0, 1.0, 1.0), radius=0.2, cutoff=12.0):
    """A hidden point light at game point (x, y, z); radius softens its
    shadows. Turn it on with lighting(points=...)."""
    data = bpy.data.lights.new("rwg_pt_" + name, "POINT")
    data.energy = POINT_POWER
    data.color = rgb
    data.shadow_soft_size = radius
    data.use_shadow = True
    for k, v in (("use_soft_falloff", False), ("use_custom_distance", True), ("cutoff_distance", cutoff)):
        if hasattr(data, k):
            setattr(data, k, v)
    ob = _link(bpy.data.objects.new("rwg_pt_" + name, data))
    ob.location = sheared(x, y, z)
    ob.hide_render = True
    return ob


@contextmanager
def lighting(sun=True, points=()):
    """Render with the canonical sun on or off and, of the stage's point
    lights, only `points` on."""
    sc = stage()
    if sc.camera is None or not sc.camera.name.startswith("rwg_cam"):
        _rig(sc, 0.0, 0.0, C.PPT, C.PPT)   # make the sun now; render_scene re-frames
    keep = {p.name for p in points}
    saved = [(ob, ob.hide_render) for ob in sc.collection.all_objects if ob.type == "LIGHT"]
    for ob, _ in saved:
        ob.hide_render = (not sun) if ob.data.type == "SUN" else ob.name not in keep
    try:
        yield
    finally:
        for ob, hidden in saved:
            ob.hide_render = hidden


@contextmanager
def map_materials(fn):
    """Temporarily give every mesh slot the material fn(object, material)."""
    saved = []
    for ob in stage().collection.all_objects:
        if ob.type != "MESH":
            continue
        saved.append((ob, [s.material for s in ob.material_slots]))
        for s in ob.material_slots:
            s.material = fn(ob, s.material)
    try:
        yield
    finally:
        _restore(saved)


def math_node(nt, op, a, b=None, c=None, clamp=False):
    """A Math node; inputs are sockets or numbers. Returns its output."""
    n = nt.nodes.new("ShaderNodeMath")
    n.operation = op
    n.use_clamp = clamp
    for i, v in enumerate((a, b, c)):
        if v is None:
            continue
        if isinstance(v, (int, float)):
            n.inputs[i].default_value = v
        else:
            nt.links.new(v, n.inputs[i])
    return n.outputs[0]


def vector_node(nt, op, a, b=None, c=None, out="Vector"):
    """A Vector Math node; inputs are sockets or 3-tuples. Returns output `out`."""
    n = nt.nodes.new("ShaderNodeVectorMath")
    n.operation = op
    for i, v in enumerate((a, b, c)):
        if v is None:
            continue
        if isinstance(v, (tuple, list)):
            n.inputs[i].default_value = v
        else:
            nt.links.new(v, n.inputs[i])
    return n.outputs[out]


def point_nodes(nt, light, reach, gain=1.0, channel=None):
    """Shader nodes in tree nt for one point light. Returns sockets
    (lambert, falloff): lambert is cos x shadow toward the light with the
    distance taken out (Shader-to-RGB x d'^2 / gain; channel 0-2 reads one
    colour channel when lights are coloured apart); falloff is
    (1 - (d/reach)^2)^2 over the unsheared distance d in tiles."""
    geo = nt.nodes.new("ShaderNodeNewGeometry")
    dif = nt.nodes.new("ShaderNodeBsdfDiffuse")
    dif.inputs["Color"].default_value = (1, 1, 1, 1)
    s2r = nt.nodes.new("ShaderNodeShaderToRGB")
    nt.links.new(dif.outputs[0], s2r.inputs[0])
    if channel is None:
        bw = nt.nodes.new("ShaderNodeRGBToBW")
        nt.links.new(s2r.outputs["Color"], bw.inputs[0])
        e = bw.outputs[0]
    else:
        sep = nt.nodes.new("ShaderNodeSeparateColor")
        nt.links.new(s2r.outputs["Color"], sep.inputs[0])
        e = sep.outputs[channel]
    lx, ly, lz = light.location
    rel = vector_node(nt, "SUBTRACT", geo.outputs["Position"], (lx, ly, lz))
    d2 = vector_node(nt, "DOT_PRODUCT", rel, rel, out="Value")
    lam = math_node(nt, "MULTIPLY", math_node(nt, "MULTIPLY", e, d2), 1.0 / gain)
    p = nt.nodes.new("ShaderNodeSeparateXYZ")
    nt.links.new(geo.outputs["Position"], p.inputs[0])
    un = nt.nodes.new("ShaderNodeCombineXYZ")
    nt.links.new(p.outputs["X"], un.inputs["X"])
    nt.links.new(math_node(nt, "MULTIPLY_ADD", p.outputs["Z"], -C.SHEAR_K, p.outputs["Y"]), un.inputs["Y"])
    nt.links.new(p.outputs["Z"], un.inputs["Z"])
    d = vector_node(nt, "DISTANCE", un.outputs[0], (lx, ly - C.SHEAR_K * lz, lz), out="Value")
    q = math_node(nt, "POWER", math_node(nt, "DIVIDE", d, reach), 2.0)
    f = math_node(nt, "POWER", math_node(nt, "SUBTRACT", 1.0, q, clamp=True), 2.0)
    return lam, f


def point_toon(m, light, reach, ambient, gain=1.0, wrap=(0.0, 1.0)):
    """A copy of toon material m lit by one point light instead of the sun:
    the band follows wrap[0] + wrap[1] * lambert toward the light, and the
    band colour is scaled by ambient + (1 - ambient) * falloff. Materials
    that are not toon (emission) come back unchanged."""
    if m is None or m.node_tree is None:
        return m
    kinds = {n.type for n in m.node_tree.nodes}
    if not {"SHADER_TO_RGB", "VALTORGB", "EMISSION"} <= kinds:
        return m
    key = "%s@%s" % (m.name, light.name)
    c = bpy.data.materials.get(key)
    if c is not None:
        return c
    c = m.copy()
    c.name = key
    nt = c.node_tree
    ramp = next(n for n in nt.nodes if n.type == "VALTORGB")
    em = next(n for n in nt.nodes if n.type == "EMISSION")
    lam, f = point_nodes(nt, light, reach, gain)
    nt.links.new(math_node(nt, "MULTIPLY_ADD", lam, wrap[1], wrap[0]), ramp.inputs["Fac"])
    b = math_node(nt, "MULTIPLY_ADD", f, 1.0 - ambient, ambient)
    sc = nt.nodes.new("ShaderNodeVectorMath")
    sc.operation = "SCALE"
    nt.links.new(ramp.outputs["Color"], sc.inputs[0])
    nt.links.new(b, sc.inputs["Scale"])
    nt.links.new(sc.outputs["Vector"], em.inputs["Color"])
    return c
