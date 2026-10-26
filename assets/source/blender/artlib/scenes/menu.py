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
from contextlib import contextmanager

import numpy as np

from .. import contracts as C
from .. import meta, palette, post
from .. import stage as S
from ..families import animal_roster, furniture, item, linked, pawn, plant, rock, structure, terrain
from ..families.animals import quadruped, rig

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
    # in front of the doorway, not a wall: figures lean back, so a head
    # against the wall sinks into it and loses its hair
    (14, 9, "south", None, "light", "hair_bun", "black", "rust", "slate"),
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

# ---- lamps (PLAY-020): the room is roofed, so no sun reaches inside it -------
# Indoors a point light at the bulb shades the furniture toward it and casts
# soft shadows away from it; outdoors the sun stays and the workbench lamp
# adds its own light, which walls block. stage.point_light / point_toon.
INSIDE = (ROOM[0] + 1, ROOM[1] + 1, ROOM[2], ROOM[3])   # roofed interior [x0, x1) x [y0, y1), tiles
LAMP_IN = next(p for p in LAMPS if p in FLOOR)
LAMP_OUT = next(p for p in LAMPS if p not in FLOOR)
LAMP_Z = 1.5            # light height, just over the bulb disc (furniture._lamp, 1.38-1.40)
LAMP_SIZE = 0.2         # light radius: soft shadows
LAMP_REACH = 6.0        # tiles: falloff (1 - (d/reach)^2)^2, as the game's lamps
OUT_REACH = 3.5         # the workbench lamp: a small pool, so the yard stays at dusk
LAMP_WRAP = (0.30, 0.75)  # toon band input = a + b * lambert: unlit faces sit in the shadow band
ROOM_AMBIENT = 0.24     # share of an indoor colour that no lamp light reaches
INDOOR_WARM = 0.40      # indoor colours pulled this far toward LAMP_RGB
OUTDOOR_LAMP = 0.55     # outdoor lamp light: surface colour x LAMP_RGB x this x falloff
SHADOW_SOFT = 4         # px at full size: extra blur on lamp shadows on the ground
GLOW_POOL = 0.16        # glow layer: share of the lamp light the game breathes
INDOOR_NUDGE = 0.08     # tiles: a surface is indoors when a step along its normal lands in INSIDE


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


def _wall_filled(x, y, mask, solid=None):
    """The linked corners whose diagonal neighbour is solid too (linked.py)."""
    if solid is None:
        solid = WALLS | {DOOR}
    diag = {"NE": (1, -1), "SE": (1, 1), "SW": (-1, 1), "NW": (-1, -1)}
    return {n for n in linked.relevant(mask) if (x + diag[n][0], y + diag[n][1]) in solid}


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
    # a grazing deer from the animal rig (ART-011); the old dustbuck model
    # is gone from the roster
    row = next(r for r in animal_roster.ROSTER if r.key == "deer")
    spec = quadruped.spec(row)
    rig.build(spec, rig.graze_poses(spec)[0])
    S.face(S.objects(), DUSTBUCK[2], (1, 1))
    S.transform(S.objects(), rig.lean_matrix())
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
            f = _wall_filled(x, y, m)
            S.place(lambda m=m, f=f: structure._wall(m, f, "wood_plank", "wood", "bark", "wood"), x, y, tag="wall")
        S.place(lambda: structure._door("ew", True, "wood", "wood_plank"), DOOR[0], DOOR[1], tag="door")
    finally:
        structure.EXT = ext
    for (x, y) in sorted(RIDGE):
        m = _rock_mask(x, y)
        S.place(lambda m=m, f=_wall_filled(x, y, m, RIDGE): rock._rock(m, f), x, y, tag="rock")
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
def _new_mat(name):
    import bpy
    m = bpy.data.materials.get(name)
    if m is not None:
        bpy.data.materials.remove(m)
    m = bpy.data.materials.new(name)
    m.use_nodes = True
    nt = m.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    em = nt.nodes.new("ShaderNodeEmission")
    nt.links.new(em.outputs[0], out.inputs["Surface"])
    return m, nt, em


def _sun_catcher_mat():
    """Ground: white in sunlight, black in sun shadow."""
    m, nt, em = _new_mat("rwg__catcher")
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
    return m


def _lamp_catcher_mat(lin, lout):
    """Ground under the two lamps, coloured apart (in red, out green):
    half of each lamp's Lambert x shadow term (headroom for the gain check)."""
    m, nt, em = _new_mat("rwg__lampcatch")
    rgb = nt.nodes.new("ShaderNodeCombineColor")
    for i, light in enumerate((lin, lout)):
        lam, _ = S.point_nodes(nt, light, LAMP_REACH, channel=i)
        nt.links.new(S.math_node(nt, "MULTIPLY", lam, 0.5), rgb.inputs[i])
    nt.links.new(rgb.outputs[0], em.inputs["Color"])
    return m


def _data_mat(bulb, lout, gain):
    """Per-object data: R bulb, G roofed (a step along the normal lands
    inside the room), B the outdoor lamp's light (Lambert x shadow x falloff)."""
    m, nt, em = _new_mat("rwg__menu_data_%d" % bulb)
    rgb = nt.nodes.new("ShaderNodeCombineColor")
    rgb.inputs[0].default_value = 1.0 if bulb else 0.0
    geo = nt.nodes.new("ShaderNodeNewGeometry")
    q = S.vector_node(nt, "MULTIPLY_ADD", geo.outputs["Normal"], (INDOOR_NUDGE,) * 3, geo.outputs["Position"])
    p = nt.nodes.new("ShaderNodeSeparateXYZ")
    nt.links.new(q, p.inputs[0])
    yb = S.math_node(nt, "MULTIPLY_ADD", p.outputs["Z"], -C.SHEAR_K, p.outputs["Y"])   # unsheared, Blender +y north
    x0, y0, x1, y1 = INSIDE
    inside = S.math_node(nt, "MULTIPLY", S.math_node(nt, "GREATER_THAN", p.outputs["X"], x0),
                         S.math_node(nt, "LESS_THAN", p.outputs["X"], x1))
    inside = S.math_node(nt, "MULTIPLY", inside, S.math_node(nt, "GREATER_THAN", yb, -y1))
    inside = S.math_node(nt, "MULTIPLY", inside, S.math_node(nt, "LESS_THAN", yb, -y0))
    nt.links.new(inside, rgb.inputs[1])
    lam, f = S.point_nodes(nt, lout, OUT_REACH, gain)
    nt.links.new(S.math_node(nt, "MULTIPLY", lam, f, clamp=True), rgb.inputs[2])
    nt.links.new(rgb.outputs[0], em.inputs["Color"])
    return m


@contextmanager
def _ground_only(material):
    """Render the ground plane alone: every mesh hidden from the camera but
    still casting shadows on it."""
    import bpy
    meshes = [o for o in S.stage().collection.all_objects if o.type == "MESH"]
    me = bpy.data.meshes.new("rwg_catcher")
    me.from_pydata([(X0, -Y0 + 1, 0), (X1 + 1, -Y0 + 1, 0), (X1 + 1, -(Y1 + 1), 0), (X0, -(Y1 + 1), 0)],
                   [], [(0, 1, 2, 3)])
    me.materials.append(material)
    plane = bpy.data.objects.new("rwg_catcher", me)
    S.stage().collection.objects.link(plane)
    for ob in meshes:
        ob.visible_camera = False
    try:
        yield
    finally:
        for ob in meshes:
            ob.visible_camera = True
        bpy.data.objects.remove(plane, do_unlink=True)


@contextmanager
def _lamps_cast_no_lamp_shadow():
    """A lamp's own shade and pole never shadow its light."""
    lamps = [o for o in S.objects() if o.get("rwg_piece") == "lamp"]
    saved = [(o, o.visible_shadow) for o in lamps]
    for o in lamps:
        o.visible_shadow = False
    try:
        yield
    finally:
        for o, v in saved:
            o.visible_shadow = v


def _is_bulb(ob):
    return any(s.material is not None and s.material.name == "rwg_lamp_warm" for s in ob.material_slots)


def _lin(v):
    """sRGB-encoded render values back to linear (data passes)."""
    return np.where(v <= 0.04045, v / 12.92, ((v + 0.055) / 1.055) ** 2.4)


def _ground_xy(h, w, scale):
    """Game tile coordinates of each ground pixel's centre."""
    P = C.PPT * scale
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32)
    return (xx + 0.5) / P, (yy + 0.5) / P


def _falloff(gx, gy, lamp, reach=LAMP_REACH):
    d2 = (gx - lamp[0] - 0.5) ** 2 + (gy - lamp[1] - 0.5) ** 2 + LAMP_Z ** 2
    return np.clip(1 - d2 / reach ** 2, 0, 1) ** 2


def _ground_cos(gx, gy, lamp):
    """cos of the lamp's light on flat ground, in the sheared space it is lit in."""
    dy = gy - (lamp[1] + 0.5 - C.SHEAR_K * LAMP_Z)
    return LAMP_Z / np.sqrt((gx - lamp[0] - 0.5) ** 2 + dy ** 2 + LAMP_Z ** 2)


def _inside(gx, gy, margin=0.06):
    """1 on the roofed room's ground (with the strip under its inner wall faces)."""
    x0, y0, x1, y1 = INSIDE
    return ((gx >= x0 - margin) & (gx < x1 + margin) & (gy >= y0 - margin) & (gy < y1 + margin)).astype(np.float32)


def _gain(lam, gx, gy):
    """Measured Shader-to-RGB scale of a point light: lit ground near the
    indoor lamp reads lambert = cos there (1.0 when Blender's units are as
    stage.POINT_POWER assumes)."""
    cos = _ground_cos(gx, gy, LAMP_IN)
    near = (_inside(gx, gy, 0) > 0) & (cos > 0.5)
    if not near.any():
        return 1.0
    return float(np.percentile(lam[near] / cos[near], 99))


def _passes(w, h, outline_px, scale):
    """Every render the grade needs, as float arrays (rows top-down):
    sun, lamp: outlined RGBA lit by the sun or by the room's lamp; alpha: the
    unoutlined coverage; lit: ground in sun (1) or sun shadow (0); emissive,
    indoor: bulb and roofed-surface masks; out: the outdoor lamp's light on
    objects; g_in, g_out: each lamp's Lambert x shadow on the ground; gain."""
    import bpy
    render = lambda: S.render_scene(0, 0, w, h, outline=False, scale=scale)  # noqa: E731
    edge = (lambda im: post.outline(im, width=outline_px)) if outline_px else (lambda im: im)
    raw = render()
    p = {"sun": edge(raw), "alpha": raw[..., 3]}
    with _ground_only(_sun_catcher_mat()):
        p["lit"] = render()[..., 0]
    lin = S.point_light("lamp_in", LAMP_IN[0] + 0.5, LAMP_IN[1] + 0.5, LAMP_Z, (1, 0, 0), LAMP_SIZE)
    lout = S.point_light("lamp_out", LAMP_OUT[0] + 0.5, LAMP_OUT[1] + 0.5, LAMP_Z, (0, 1, 0), LAMP_SIZE)
    with _lamps_cast_no_lamp_shadow():
        with S.lighting(sun=False, points=(lin, lout)), _ground_only(_lamp_catcher_mat(lin, lout)):
            g = render()
        p["g_in"], p["g_out"] = 2 * _lin(g[..., 0]), 2 * _lin(g[..., 1])
        gain = p["gain"] = _gain(p["g_in"], *_ground_xy(*g.shape[:2], scale))
        print("menu: point light gain %.3f (1.0 = stage.POINT_POWER as assumed)" % gain)
        lin.data.color = lout.data.color = (1, 1, 1)
        toon = lambda ob, m: S.point_toon(m, lin, LAMP_REACH, ROOM_AMBIENT, gain, LAMP_WRAP)  # noqa: E731
        with S.lighting(sun=False, points=(lin,)), S.map_materials(toon):
            p["lamp"] = edge(render())
        for m in [m for m in bpy.data.materials if m.name.endswith("@" + lin.name)]:
            bpy.data.materials.remove(m)   # gain-specific variants
        bulb, other = _data_mat(True, lout, gain), _data_mat(False, lout, gain)
        with S.lighting(sun=False, points=(lout,)), S.map_materials(lambda ob, m: bulb if _is_bulb(ob) else other):
            d = render()
    p["emissive"] = (_lin(d[..., 0]) > 0.5) & (d[..., 3] > 0.5)
    p["indoor"] = _lin(d[..., 1])
    p["out"] = _lin(d[..., 2])
    return p


def _blur(a, k):
    """Separable box blur, radius k px, repeated for a soft falloff."""
    for _ in range(3):
        c = np.cumsum(np.pad(a, ((0, 0), (k + 1, k)), mode="edge"), axis=1)
        a = (c[:, 2 * k + 1:] - c[:, :-2 * k - 1]) / (2 * k + 1)
        c = np.cumsum(np.pad(a, ((k + 1, k), (0, 0)), mode="edge"), axis=0)
        a = (c[2 * k + 1:] - c[:-2 * k - 1]) / (2 * k + 1)
    return a


def _pool(h, w, cx, cy, r):
    yy, xx = np.mgrid[0:h, 0:w].astype(np.float32)
    d = np.hypot(xx - cx, yy - cy) / r
    return np.clip(1 - d, 0, 1) ** 2


def _glow(h, w, emissive, scale, pool):
    """Additive glow layers: rgb = light colour, alpha = intensity. The lamp
    layer is the bulbs' halo over a share of the baked lamp light (pool), so
    the game's breathing follows the room's falloff and shadows."""
    P = C.PPT * scale
    halo = _blur(emissive.astype(np.float32), max(2, int(10 * scale)))
    lamp = np.clip(halo * 2.5, 0, 0.9)
    lamp = np.maximum(lamp, GLOW_POOL * _blur(pool.astype(np.float32), max(1, int(8 * scale))))
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


def _dusk(rgb):
    """Outdoor grade: the sky is going dark."""
    lum = rgb.mean(axis=2, keepdims=True)
    g = (lum + (rgb - lum) * (1 - DESATURATE)) * EXPOSURE
    return g * (1 - COOL) + np.array(_ui_space(), np.float32) * COOL


def _lamplit(rgb):
    """Indoor grade: the roofed room under its lamp keeps its exposure and warms."""
    return rgb * (1 + INDOOR_WARM * (np.array(LAMP_RGB, np.float32) - 1))


def _grow(m, k):
    """Max-filter m k times over 8 neighbours (masks into the outline ring)."""
    for _ in range(k):
        g = m.copy()
        for dy in (-1, 0, 1):
            for dx in (-1, 0, 1):
                if dy or dx:
                    g = np.maximum(g, post.shift(m, dy, dx))
        m = g
    return m


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
    ring = max(1, int(round(C.OUTLINE_PX * scale)))
    p = _passes(w, h, ring, scale)
    hh, ww = p["sun"].shape[:2]
    albedo = base[:hh, :ww, :3]
    gx, gy = _ground_xy(hh, ww, scale)
    warm = np.array(LAMP_RGB, np.float32)
    soft = max(1, int(round(SHADOW_SOFT * scale)))
    # ground: outdoors the sun and its shadows, indoors only the lamp
    v_in = _blur(np.clip(p["g_in"] / _ground_cos(gx, gy, LAMP_IN) / p["gain"], 0, 1), soft)
    v_out = _blur(np.clip(p["g_out"] / _ground_cos(gx, gy, LAMP_OUT) / p["gain"], 0, 1), soft)
    e_in = _falloff(gx, gy, LAMP_IN) * v_in
    e_out = _falloff(gx, gy, LAMP_OUT, OUT_REACH) * v_out
    sun_g = albedo * np.where(p["lit"][..., None] > 0.5, 1.0, np.array(SHADOW_MUL, np.float32))
    _blob_shadows(sun_g, scale)
    out_g = _dusk(sun_g) + albedo * (OUTDOOR_LAMP * e_out)[..., None] * warm
    in_g = _lamplit(albedo * (ROOM_AMBIENT + (1 - ROOM_AMBIENT) * e_in)[..., None])
    fi = _blur(_inside(gx, gy), soft)[..., None]
    ground_rgb = fi * in_g + (1 - fi) * out_g
    # objects: roofed surfaces from the lamp pass, the rest from the sun pass
    a = p["sun"][..., 3:4]
    mi = np.where(p["alpha"] < 0.5, _grow(p["indoor"], ring + 1), p["indoor"])[..., None]
    sun_o = p["sun"][..., :3]
    out_o = _dusk(sun_o) + sun_o * (OUTDOOR_LAMP * p["out"])[..., None] * warm
    obj_rgb = mi * _lamplit(p["lamp"][..., :3]) + (1 - mi) * out_o
    img = np.ones((hh, ww, 4), np.float32)
    img[..., :3] = obj_rgb * a + ground_rgb * (1 - a)
    img[p["emissive"], :3] = sun_o[p["emissive"]]   # bulbs: emission, no light, no grade
    # glow pool: the lamp light the room and the yard receive
    a2, m2, f2 = a[..., 0], mi[..., 0], fi[..., 0]
    w_in = m2 * a2 + f2 * (1 - a2)
    pool = (w_in * _falloff(gx, gy, LAMP_IN) * ((1 - a2) * v_in + a2)
            + (1 - w_in) * ((1 - a2) * e_out + a2 * p["out"]))
    lamp, fire = _glow(hh, ww, p["emissive"], scale, pool)
    S.clear()
    bpy.context.view_layer.update()
    return img, lamp, fire


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
