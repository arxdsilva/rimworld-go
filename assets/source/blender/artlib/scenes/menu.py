"""Main-menu background: a small colony at dusk (MAIN UI spec §2).

One 3D scene built from the family builders and rendered through the
canonical stage (camera, sun, shear, outline), over numpy terrain:

    blender -b --factory-startup -P build.py -- menu      (make art-menu)
    live: build_scene() to inspect, preview() for a 1/4-size render

Writes generated/ui/menu/menu_diorama.png (graded, opaque), two
half-size additive glow layers for the game to flicker, and
metadata/ui_menu/menu_diorama.yaml with the fire and lamp points.
Layout is in game tiles: x east, y south, footprint top-left at (x, y).
The bust covers the bottom-left and the buttons the right, so the room
and the campfire sit centre-top; a rock ridge keeps the rail calm.
"""
import os
import tempfile

import numpy as np

from .. import contracts as C
from .. import meta, palette, post
from .. import stage as S
from ..families import animal, furniture, item, pawn, plant, rock, structure, terrain

FRAME_PX = (3200, 1800)
TILES_W, TILES_H = 25, 15          # ground tiles covering the frame
X0, X1, Y0, Y1 = -2, 27, -2, 16    # built area (bleed for shear and edges)
OUT_DIR = os.path.join(C.ASSETS_DIR, "generated", "ui", "menu")

# ---- layout ----------------------------------------------------------------
ROOM = (11, 3, 17, 8)              # walls on the border, inclusive
DOOR = (14, 8)
WALLS = ({(x, y) for x in range(ROOM[0], ROOM[2] + 1) for y in (ROOM[1], ROOM[3])}
         | {(x, y) for y in range(ROOM[1], ROOM[3] + 1) for x in (ROOM[0], ROOM[2])}) - {DOOR}
FLOOR = {(x, y) for x in range(ROOM[0] + 1, ROOM[2]) for y in range(ROOM[1] + 1, ROOM[3])}
FIRE = (14, 10)
PLAZA = {(x, y) for x in range(13, 16) for y in range(9, 12)}
PATH = {(16, 10), (17, 10)}          # gravel to the stockpile
CROPS = {(x, y): ("wheat" if y < 13 else "corn") for x in range(1, 9) for y in range(10, 16)}
RIDGE_EDGE = (21, 21, 20, 21, 22, 21, 21, 20, 20, 21, 22, 22, 21, 21, 20, 21, 21, 22, 22)  # by y from Y0
RIDGE = {(x, y) for y in range(Y0, Y1 + 1) for x in range(RIDGE_EDGE[y - Y0], X1 + 1)}

# (builder name, args, x, y, facing)
FURNITURE = [
    ("bed", 12, 4, "south", "navy"), ("bed", 13, 4, "south", "rust"),
    ("lamp", 15, 4, "south", None), ("table", 15, 6, "south", None),
    ("chair", 14, 6, "east", None), ("chair", 16, 6, "west", None),
    ("workbench", 9, 5, "south", None), ("lamp", 9, 4, "south", None),
    ("campfire", FIRE[0], FIRE[1], "south", None),
]
LAMPS = [(15, 4), (9, 4)]
ITEMS = [("_wood", 18, 9), ("_food", 19, 9), ("_stone", 18, 10)]
# x, y, facing, frame, skin, hair style, hair tint, shirt tint, vest tint
COLONISTS = [
    (13, 10, "east", "kneel", "tan", "hair_short", "black", "sand", "olive"),
    (15, 10, "west", "bench1", "pale", "hair_bun", "auburn", "slate", None),
    (14, 11, "north", None, "dark", "hair_crop", "grey", "cream", "rust"),
    (15, 9, "south", None, "light", "hair_bun", "black", "rust", "slate"),
]
TREES = [("oak", 0, "autumn", 0, 4), ("birch", 1, "autumn", 3, 7), ("oak", 2, "autumn", -1, 9),
         ("birch", 0, "autumn", 6, 12), ("birch", 2, "summer", 2, 0), ("oak", 1, "summer", 8, -1),
         ("birch", 1, "summer", 16, 0), ("pine", 0, None, 19, 3), ("pine", 1, None, 19, 11),
         ("pine", 2, None, 20, 13)]
DUSTBUCK = (7, 1, "east")
FILL_SEED = 7

# ---- dusk grade (position-independent; the game adds the vignette) --------
EXPOSURE = 0.60
COOL = 0.25            # mix toward ui.space
DESATURATE = 0.20
SHADOW_MUL = (0.62, 0.64, 0.72)   # ground under cast shadow
LAMP_RGB = palette.hex_to_srgb("#ffd98a")
FIRE_RGB = palette.hex_to_srgb("#ff9a4a")


def _ui_space():
    path = os.path.join(C.PALETTE_DIR, "ui.yaml")
    for line in open(path, encoding="utf-8"):
        if line.startswith("ui.space:"):
            return palette.hex_to_srgb(line.split('"')[1][:7])
    return (0.04, 0.06, 0.09)


# ---- terrain -----------------------------------------------------------------
def terrain_at(x, y):
    if (x, y) in CROPS:
        return "soil_rich"
    if (x, y) in PLAZA or (x, y) in PATH:
        return "gravel"
    if x >= RIDGE_EDGE[min(max(y, Y0), Y1) - Y0] - 2:
        return "stone"
    return "soil"


def ground():
    """Terrain tiles plus soft transitions, as the renderer draws them."""
    N = C.PPT
    img = np.zeros((TILES_H * N, TILES_W * N, 4), np.float32)
    cache = {}

    def tile(name, v):
        k = (name, v)
        if k not in cache:
            cache[k] = terrain._tile_img(name, v)
        return cache[k]

    edges = {}
    for y in range(TILES_H):
        for x in range(TILES_W):
            t = terrain_at(x, y)
            v = (x * 7 + y * 13 + x * y) % terrain.VARIANTS
            img[y * N:(y + 1) * N, x * N:(x + 1) * N] = tile(t, v)
            for hi in terrain.PRIORITY[terrain.PRIORITY.index(t) + 1:]:
                mask = sum(b for (dx, dy), b in (((0, -1), 1), ((1, 0), 2), ((0, 1), 4), ((-1, 0), 8))
                           if terrain_at(x + dx, y + dy) == hi)
                if not mask:
                    continue
                if (hi, mask) not in edges:
                    edges[(hi, mask)] = terrain._edge_img(hi, mask)
                e = edges[(hi, mask)]
                dst = img[y * N:(y + 1) * N, x * N:(x + 1) * N]
                a = e[..., 3:4]
                dst[..., :3] = e[..., :3] * a + dst[..., :3] * (1 - a)
    img[..., 3] = 1.0
    return img[:FRAME_PX[1], :FRAME_PX[0]]


def floor_tile():
    """One 128 px tile of the wood floor, rendered alone."""
    S.clear()
    structure._floor_wood()
    t = S.render_scene(0, 0, C.PPT, C.PPT, outline=False)
    S.clear()
    return t


# ---- scene -------------------------------------------------------------------
def _wall_mask(x, y):
    solid = WALLS | {DOOR}
    return sum(b for (dx, dy), b in (((0, -1), 1), ((1, 0), 2), ((0, 1), 4), ((-1, 0), 8))
               if (x + dx, y + dy) in solid)


def _rock_mask(x, y):
    return sum(b for (dx, dy), b in (((0, -1), 1), ((1, 0), 2), ((0, 1), 4), ((-1, 0), 8))
               if (x + dx, y + dy) in RIDGE)


def _tint(ob, tints):
    slot = ob.material_slots[0] if ob.material_slots else None
    if slot is None or slot.material is None or not slot.material.name.startswith("rwg_"):
        return
    name = slot.material.name[4:].split("@")[0]
    hexes = S.pal()[0]
    if name in tints and name in hexes:
        part = ob.name.split(".")[0]
        key = tints[name] if not isinstance(tints[name], dict) else tints[name].get(part, tints[name]["*"])
        slot.material = S.mat_tinted(name, key)


def _colonist(facing, frame, skin, style, hair_t, shirt_t, vest_t):
    tints_hex = _tint_table()

    def build():
        pawn._body()
        pawn._pants()
        pawn._shirt()
        if vest_t:
            pawn._vest()
        pawn._apply_modifiers()
        if frame:
            pawn._pose(frame, facing)
        before = {o.name for o in S.objects()}
        pawn._hair(style)
        pawn._apply_modifiers()
        if frame:
            hair = [o for o in S.objects() if o.name not in before]
            S.transform(hair, pawn._head_matrix(pawn._params(frame, facing)))
        pawn._figure(facing)
        pants = "slate" if shirt_t != "slate" else "navy"
        cloth = {"*": tints_hex["cloth"][shirt_t], "pleg": tints_hex["cloth"][pants],
                 "hips": tints_hex["cloth"][pants]}
        if vest_t:
            cloth["vest"] = tints_hex["cloth"][vest_t]
        for ob in S.objects():
            _tint(ob, {"skin": tints_hex["skin"][skin], "hair": tints_hex["hair"][hair_t], "cloth": cloth})
            ob.visible_shadow = False   # figures get a blob shadow (ART §6.1)
            ob["rwg_figure"] = True
    return build


_TINTS = None


def _tint_table():
    """base.yaml tints: {group: {choice: hex}}."""
    global _TINTS
    if _TINTS is None:
        import re
        _TINTS = {}
        for line in open(os.path.join(C.PALETTE_DIR, "base.yaml"), encoding="utf-8"):
            m = re.match(r"^\s+(skin|hair|cloth):\s*\{(.*)\}", line)
            if m:
                _TINTS[m.group(1)] = dict(re.findall(r'(\w+):\s*"(#[0-9a-fA-F]{6})"', m.group(2)))
    return _TINTS


def _dustbuck():
    animal._dustbuck()
    animal._pose_action("dustbuck", animal.POSES["dustbuck"]["graze1"])
    S.face(S.objects(), DUSTBUCK[2], (1, 1))
    S.transform(S.objects(), animal._lean_matrix())
    for ob in S.objects():
        ob.visible_shadow = False
        ob["rwg_figure"] = True


FURN = {"bed": furniture._bed, "lamp": furniture._lamp, "table": furniture._table, "chair": furniture._chair,
        "workbench": furniture._workbench, "campfire": furniture._campfire}


def _fill_spots():
    """Seeded grass and bushes on free soil."""
    taken = set(WALLS) | FLOOR | {DOOR} | PLAZA | set(CROPS) | RIDGE
    taken |= {(x, y) for _, x, y, _, _ in FURNITURE} | {(x, y) for _, x, y in ITEMS}
    taken |= {(t[3], t[4]) for t in TREES} | {(c[0], c[1]) for c in COLONISTS} | {DUSTBUCK[:2]}
    taken |= PATH
    rng = np.random.RandomState(FILL_SEED)
    out = []
    for y in range(Y0, Y1 + 1):
        for x in range(X0, X1 + 1):
            if (x, y) in taken or terrain_at(x, y) not in ("soil", "stone"):
                continue
            r = rng.rand()
            if r < 0.20:
                out.append(("grass", x, y, rng.randint(8)))
            elif r < 0.25:
                out.append(("bush", x, y, rng.randint(4)))
    return out


def build_scene():
    """Every 3D piece of the diorama on the stage (not yet sheared)."""
    S.clear()
    ext = structure.EXT
    structure.EXT = 0.03   # one render: connected wall bodies overlap instead of being clipped
    try:
        for (x, y) in sorted(WALLS):
            m = _wall_mask(x, y)
            S.place(lambda m=m: structure._wall(m, "wood_plank", "wood", "bark"), x, y, tag="wall")
        S.place(lambda: structure._door("ew", True, "wood", "wood_plank"), DOOR[0], DOOR[1], tag="door")
    finally:
        structure.EXT = ext
    for (x, y) in sorted(RIDGE):
        S.place(lambda m=_rock_mask(x, y): rock._rock(m), x, y, tag="rock")
    for name, x, y, facing, cloth in FURNITURE:
        objs = S.place(FURN[name], x, y, facing=facing, tag=name)
        if cloth:
            for ob in objs:
                _tint(ob, {"cloth": _tint_table()["cloth"][cloth]})
    for fn, x, y in ITEMS:
        S.place(getattr(item, fn), x, y, tag="item")
    for (x, y), species in sorted(CROPS.items()):
        v = (x + y) % 3
        S.place(lambda s=species, v=v: plant._crop(s, "mature", v, "x"), x, y, tag="crop")
    for kind, i, season, x, y in TREES:
        if kind == "pine":
            S.place(lambda i=i: plant._tree("pine", i, "summer", "x"), x, y, tag="tree")
        else:
            S.place(lambda k=kind, i=i, s=season: plant._tree(k, i, s, "x"), x, y, tag="tree")
    for kind, x, y, v in _fill_spots():
        fn = plant._grass if kind == "grass" else plant._bush
        S.place(lambda fn=fn, v=v: fn(v), x, y, tag=kind)
    for x, y, facing, frame, *look in COLONISTS:
        S.place(_colonist(facing, frame, *look), x, y, tag="colonist")
    S.place(_dustbuck, DUSTBUCK[0], DUSTBUCK[1], tag="animal")
    return S.objects()


# ---- render passes -----------------------------------------------------------
def _catcher():
    import bpy
    me = bpy.data.meshes.new("rwg_catcher")
    me.from_pydata([(X0, -Y0 + 1, 0), (X1 + 1, -Y0 + 1, 0), (X1 + 1, -(Y1 + 1), 0), (X0, -(Y1 + 1), 0)],
                   [], [(0, 1, 2, 3)])
    m = bpy.data.materials.get("rwg__catcher") or bpy.data.materials.new("rwg__catcher")
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    em = nt.nodes.new("ShaderNodeEmission")
    dif = nt.nodes.new("ShaderNodeBsdfDiffuse")
    s2r = nt.nodes.new("ShaderNodeShaderToRGB")
    ramp = nt.nodes.new("ShaderNodeValToRGB")
    ramp.color_ramp.interpolation = "CONSTANT"
    ramp.color_ramp.elements[0].color = (0, 0, 0, 1)
    ramp.color_ramp.elements[1].position = C.SHADOW_T
    ramp.color_ramp.elements[1].color = (1, 1, 1, 1)
    nt.links.new(dif.outputs[0], s2r.inputs[0])
    nt.links.new(s2r.outputs["Color"], ramp.inputs["Fac"])
    nt.links.new(ramp.outputs["Color"], em.inputs["Color"])
    nt.links.new(em.outputs[0], out.inputs["Surface"])
    me.materials.append(m)
    ob = bpy.data.objects.new("rwg_catcher", me)
    S.stage().collection.objects.link(ob)
    return ob


def _passes(w, h, outline_px, scale):
    """objects RGBA, lit-ground mask (1 lit, 0 shadow), emissive mask."""
    raw = S.render_scene(0, 0, w, h, outline=False, scale=scale)
    objs = post.outline(raw, width=outline_px) if outline_px else raw
    meshes = [o for o in S.stage().collection.all_objects if o.type == "MESH"]
    catcher = _catcher()
    for ob in meshes:
        ob.visible_camera = False
    try:
        lit = S.render_scene(0, 0, w, h, outline=False, scale=scale)[..., 0]
    finally:
        for ob in meshes:
            ob.visible_camera = True
        import bpy
        bpy.data.objects.remove(catcher, do_unlink=True)
    black = S.flat("black", (0, 0, 0))
    lamp = S.mat("lamp_warm")
    with S.swap_materials(lambda o: lamp if any(s.material == lamp for s in o.material_slots) else black):
        em = S.render_scene(0, 0, w, h, outline=False, scale=scale)
    emissive = (em[..., :3].max(axis=2) > 0.2) & (em[..., 3] > 0.5)
    return objs, lit, emissive


def _blur(a, k):
    """Separable box blur, radius k px, repeated for a soft falloff."""
    for _ in range(3):
        c = np.cumsum(np.pad(a, ((0, 0), (k + 1, k)), mode="edge"), axis=1)
        a = (c[:, 2 * k + 1:] - c[:, :-2 * k - 1]) / (2 * k + 1)
        c = np.cumsum(np.pad(a, ((k + 1, k), (0, 0)), mode="edge"), axis=0)
        a = (c[2 * k + 1:] - c[:-2 * k - 1]) / (2 * k + 1)
    return a


def _pool(h, w, cx, cy, r, clip=None):
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32)
    d = np.hypot(xx - cx, yy - cy) / r
    p = np.clip(1 - d, 0, 1) ** 2
    if clip:
        x0, y0, x1, y1 = clip
        p *= ((xx >= x0) & (xx < x1) & (yy >= y0) & (yy < y1)).astype(np.float32)
        p = _blur(p, 6)
    return p


def _glow(h, w, emissive, scale):
    """Additive glow layers: rgb = light colour, alpha = intensity."""
    P = C.PPT * scale
    halo = _blur(emissive.astype(np.float32), max(2, int(10 * scale)))
    lamp = np.clip(halo * 2.5, 0, 0.9)
    room = tuple(v * P for v in (ROOM[0] + 1, ROOM[1] + 1, ROOM[2], ROOM[3]))
    for (x, y) in LAMPS:
        inside = (x, y) in FLOOR
        lamp = np.maximum(lamp, 0.32 * _pool(h, w, (x + 0.5) * P, (y + 0.5) * P, (2.2 if inside else 2.6) * P,
                                             room if inside else None))
    fire = 0.42 * _pool(h, w, (FIRE[0] + 0.5) * P, (FIRE[1] + 0.5) * P, 3.4 * P)

    def layer(a, rgb):
        out = np.zeros((h, w, 4), np.float32)
        out[..., :3] = rgb
        out[..., 3] = a
        return out
    return layer(lamp, LAMP_RGB), layer(fire, FIRE_RGB)


def _blob_shadows(img, scale):
    P = C.PPT * scale
    h, w = img.shape[:2]
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32)
    figures = [(c[0], c[1]) for c in COLONISTS] + [DUSTBUCK[:2]]
    for x, y in figures:
        d = ((xx - (x + 0.5) * P) / (0.30 * P)) ** 2 + ((yy - (y + 0.58) * P) / (0.13 * P)) ** 2
        a = np.clip(1 - d, 0, 1) ** 0.6 * 0.35
        img[..., :3] *= (1 - a)[..., None]


def _dusk(img, keep):
    rgb = img[..., :3]
    lum = rgb.mean(axis=2, keepdims=True)
    g = (lum + (rgb - lum) * (1 - DESATURATE)) * EXPOSURE
    g = g * (1 - COOL) + np.array(_ui_space(), np.float32) * COOL
    img[..., :3] = np.where(keep[..., None], rgb, g)
    return img


def _half(a):
    h, w = a.shape[0] // 2 * 2, a.shape[1] // 2 * 2
    a = a[:h, :w]
    return (a[0::2, 0::2] + a[1::2, 0::2] + a[0::2, 1::2] + a[1::2, 1::2]) / 4


def compose(scale=1.0):
    """Render and grade the diorama at scale (1 = full size)."""
    import bpy
    w, h = FRAME_PX
    floor = floor_tile()
    base = ground()
    N = C.PPT
    for (x, y) in FLOOR:
        a = floor[..., 3:4]
        dst = base[y * N:(y + 1) * N, x * N:(x + 1) * N]
        dst[..., :3] = floor[..., :3] * a + dst[..., :3] * (1 - a)
    if scale != 1.0:
        k = int(round(1 / scale))
        base = base[::k, ::k].copy()
    build_scene()
    objs, lit, emissive = _passes(w, h, max(1, int(round(C.OUTLINE_PX * scale))), scale)
    hh, ww = objs.shape[:2]
    base = base[:hh, :ww]
    shade = np.where(lit[..., None] > 0.5, 1.0, np.array(SHADOW_MUL, np.float32))
    base[..., :3] *= shade
    _blob_shadows(base, scale)
    a = objs[..., 3:4]
    base[..., :3] = objs[..., :3] * a + base[..., :3] * (1 - a)
    base[..., 3] = 1.0
    base = _dusk(base, emissive)
    lamp, fire = _glow(hh, ww, emissive, scale)
    S.clear()
    bpy.context.view_layer.update()
    return base, lamp, fire


def preview():
    """Quarter-size render for review; returns the PNG path."""
    base, lamp, fire = compose(0.25)
    out = base.copy()
    for g in (lamp, fire):
        out[..., :3] = np.clip(out[..., :3] + g[..., :3] * g[..., 3:4], 0, 1)
    path = os.path.join(tempfile.gettempdir(), "rwg_menu_preview.png")
    S.write_png(out, path)
    return path


def _project(x, y, z):
    return [round((x + 0.5) * C.PPT, 1), round((y + 0.5) * C.PPT - C.SHEAR_K * z * C.PPT, 1)]


def build():
    base, lamp, fire = compose(1.0)
    paths = {k: os.path.join(OUT_DIR, "menu_%s.png" % k) for k in ("diorama", "glow_lamp", "glow_fire")}
    S.write_png(base, paths["diorama"])
    S.write_png(_half(lamp), paths["glow_lamp"])
    S.write_png(_half(fire), paths["glow_fire"])
    rel = {k: os.path.relpath(p, C.ASSETS_DIR).replace(os.sep, "/") for k, p in paths.items()}
    att = {"fire0": _project(FIRE[0], FIRE[1], 0.22)}
    for i, (x, y) in enumerate(LAMPS):
        att["lamp%d" % i] = _project(x, y, 1.39)
    meta.write("ui_menu", "menu_diorama", {
        "image": rel["diorama"], "canvas": list(FRAME_PX), "tile_px": C.PPT, "scale": C.SCALE,
        "glow_lamp": rel["glow_lamp"], "glow_fire": rel["glow_fire"], "attachments": att,
    })
    return list(paths.values())
