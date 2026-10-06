"""The one canonical stage (ART spec §21): camera, light, toon materials,
geometry helpers, render + export. Family modules must not create their
own camera, light, or render settings.
"""
import math
import os
import tempfile

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
    cr.interpolation = "CONSTANT"
    cr.elements[0].position = 0.0
    cr.elements[0].color = (*palette.hex_to_linear(p["shadow"]), 1)
    cr.elements[1].position = C.SHADOW_T
    cr.elements[1].color = (*palette.hex_to_linear(p["base"]), 1)
    e = cr.elements.new(C.LIGHT_T)
    e.color = (*palette.hex_to_linear(p["light"]), 1)
    nt.links.new(dif.outputs[0], s2r.inputs[0])
    nt.links.new(s2r.outputs["Color"], ramp.inputs["Fac"])
    nt.links.new(ramp.outputs["Color"], em.inputs["Color"])
    return m


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
    cam_data = bpy.data.cameras.new("rwg_cam")
    cam_data.type = "ORTHO"
    cam_data.ortho_scale = max(w_px, h_px) / C.PPT
    cam_data.clip_start = 0.1
    cam_data.clip_end = 200
    cam = _link(bpy.data.objects.new("rwg_cam", cam_data))
    cam.location = (x_min + w_px / C.PPT / 2, y_max - h_px / C.PPT / 2, 100)
    sc.camera = cam
    sun_data = bpy.data.lights.new("rwg_sun", "SUN")
    sun_data.energy = math.pi
    sun_data.angle = math.radians(1.0)
    sun_data.use_shadow = True
    sun = _link(bpy.data.objects.new("rwg_sun", sun_data))
    L = Vector(C.light_vector())
    sun.rotation_euler = Vector((0, 0, -1)).rotation_difference(-L).to_euler()
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
    tmp = os.path.join(tempfile.gettempdir(), "rwg_render.png")
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


def render_asset(family, name, footprint=(1, 1), zmax=1.0, margins=None,
                 outline=True, shadow=True, extra=None, keep=False,
                 clip=(), zclip=None, tint=True):
    """Shear, frame, render, post-process, write PNG + metadata.

    footprint: (w, h) tiles. zmax: tallest point (m). margins: tiles
    (left, right, top, bottom) beyond the footprint/overhang.
    """
    sc = stage()
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
    img = post.process(raw, do_outline=outline, do_shadow=shadow)
    if clip:
        img = _clip(img, (left, top), (w, h), clip, zc)
    path = out_path(family, name)
    write_png(img, path)
    data = {
        "image": os.path.relpath(path, C.ASSETS_DIR).replace(os.sep, "/"),
        "scale": C.SCALE,
        "canvas": [W, H],
        "footprint": [w, h],
        "origin_px": [left, top],
        "projection": {"shear_k": C.SHEAR_K},
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
            m = post.outline(m, darken=1.0, saturate=1.0)
        mask = np.zeros_like(m)
        mask[..., :3] = m[..., :1]
        mask[..., 3] = 1.0
        mpath = out_path(family, name + "_mask")
        write_png(mask, mpath)
        data["tint"] = {"group": groups.pop(),
                        "mask": os.path.relpath(mpath, C.ASSETS_DIR).replace(os.sep, "/")}
    if extra:
        data.update(extra)
    meta.write(family, name, data)
    if not keep:
        clear()
    return path


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
