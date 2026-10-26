#!/usr/bin/env python3
"""ART §34 style-prototype scene: composes generated sprites into one
representative colony shot using the same placement rules the renderer
uses (origin_px anchors the footprint's top-left at tile (x, y); draw
order terrain -> floors -> objects north-to-south -> overlays).

    python3 assets/source/tools/compose_scene.py [-o out.png] [--zoom 1|2]
"""
import argparse
import os
import re

from PIL import Image

import contact_sheet as cs

ROOT = cs.ROOT
GEN = cs.GEN
PPT = 128
OFF = 0

W, H = 16, 11

# ---- layout ---------------------------------------------------------------
TERRAIN = [[None] * W for _ in range(H)]
for y in range(H):
    for x in range(W):
        t = "soil"
        if x >= 12 and y <= 3:
            t = "stone"
        if x >= 13 and y <= 5:
            t = "stone"
        if 1 <= x <= 5 and 7 <= y <= 9:
            t = "soil_rich"
        if 12 <= x <= 14 and 9 <= y <= 10 or (x == 15 and y == 9):
            t = "gravel"
        TERRAIN[y][x] = t

ROOM = (6, 1, 11, 6)        # x0, y0, x1, y1 inclusive (walls on border)
DOOR = (8, 6)
WALLS = {(x, y) for x in range(ROOM[0], ROOM[2] + 1) for y in (ROOM[1], ROOM[3])} | \
        {(x, y) for y in range(ROOM[1], ROOM[3] + 1) for x in (ROOM[0], ROOM[2])}
WALLS.discard(DOOR)
FLOORS = {(x, y) for x in range(ROOM[0] + 1, ROOM[2]) for y in range(ROOM[1] + 1, ROOM[3])}

OBJECTS = [
    # (family, name, x, y, tint)
    ("furniture", "bed_basic_south", 7, 2, {"cloth": "navy"}),
    ("furniture", "bed_basic_south", 10, 2, {"cloth": "rust"}),
    ("furniture", "lamp_standing", 9, 2, None),
    ("furniture", "table_small", 8, 4, None),
    ("furniture", "chair_basic_east", 7, 4, None),
    ("furniture", "chair_basic_west", 9, 4, None),
    ("furniture", "workbench_basic_south", 2, 3, None),
    ("structure", "frame_1x1", 12, 7, None),
    ("terrain", "rock_chunk_00", 14, 2, None),
    ("terrain", "rock_chunk_01", 13, 4, None),
    ("terrain", "rock_chunk_02", 15, 1, None),
    ("plant", "tree_oak_00", 0, 2, None),
    ("plant", "tree_oak_01", 3, 1, None),
    ("plant", "tree_oak_02", 14, 7, None),
    ("plant", "tree_oak_01", 0, 5, None),
    ("plant", "berry_bush_harvestable_v0", 13, 9, None),
    ("plant", "bush_mature_v1", 15, 10, None),
    ("plant", "grass_short_v0", 5, 5, None),
    ("plant", "grass_short_v1", 11, 8, None),
    ("item", "wood_pile", 4, 5, None),
    ("item", "food_crate", 10, 8, None),
    ("item", "steel_bars", 3, 5, None),
    ("item", "stone_blocks", 11, 9, None),
]
for i, x in enumerate(range(1, 6)):
    for j, y in enumerate(range(7, 10)):
        stage = ["crop_sown", "crop_young", "crop_mature", "crop_harvestable"][min(3, (x + j) % 4)]
        OBJECTS.append(("plant", stage, x, y, None))

PAWNS = [
    # x, y, facing, skin, hair style, hair tint, shirt tint, vest tint|None, rifle[, build]
    (8, 7, "south", "tan", "hair_short", "black", "sand", "olive", True),
    (4, 6, "east", "pale", "hair_bun", "auburn", "slate", None, False, "f_avg"),
    (10, 4, "west", "dark", "hair_crop", "grey", "cream", "rust", False),
    (14, 3, "south", "light", "hair_bun", "black", "rust", "slate", True, "f_avg"),
    (2, 8, "north", "brown", "hair_short", "blond", "olive", None, False),
]
SELECTED = (8, 7)
DESIGNATED = [(3, 1), (0, 2)]
BLUEPRINTS = [(12, 6), (13, 6), (14, 6)]
STOCKPILE = [(10, 8), (11, 8), (10, 9), (11, 9)]


# ---- drawing ----------------------------------------------------------------
def meta(family, name):
    return cs.meta_of(family, name)


def origin(md):
    m = re.search(r"origin_px: \[(\d+), (\d+)\]", md)
    return (int(m.group(1)), int(m.group(2))) if m else (0, 0)


def sprite(family, name, tint=None):
    p = os.path.join(GEN, family, name + ".png")
    if not os.path.exists(p):
        return None, None
    return cs.tinted(p, family, name, tint), origin(meta(family, name))


def put(canvas, family, name, x, y, tint=None):
    img, o = sprite(family, name, tint)
    if img is None:
        return False
    canvas.alpha_composite(img, (OFF + x * PPT - o[0], OFF + y * PPT - o[1]))
    return True


def wall_mask(x, y):
    m = 0
    for (dx, dy), bit in (((0, -1), 1), ((1, 0), 2), ((0, 1), 4), ((-1, 0), 8)):
        if (x + dx, y + dy) in WALLS or (x + dx, y + dy) == DOOR:
            m |= bit
    return m


def compose(zoom):
    pad = PPT
    canvas = Image.new("RGBA", (W * PPT + 2 * pad, H * PPT + 2 * pad), (20, 18, 16, 255))
    world = Image.new("RGBA", (W * PPT + 2 * pad, H * PPT + 2 * pad), (0, 0, 0, 0))
    global OFF
    OFF = pad
    for y in range(H):
        for x in range(W):
            v = (x * 7 + y * 13 + (x * y) % 5) % 4
            put(world, "terrain", "%s_%02d" % (TERRAIN[y][x], v), x, y)
    for (x, y) in sorted(FLOORS):
        put(world, "structure", "floor_wood", x, y)
    tint = cs.tints()
    for (x, y) in STOCKPILE:
        put(world, "overlay", "stockpile", x, y)
    draws = []
    for (x, y) in WALLS:
        draws.append((y, x, "structure", "wall_wood_%02d" % wall_mask(x, y), None))
    draws.append((DOOR[1], DOOR[0], "structure", "door_wood_ew_open", None))
    for fam, name, x, y, t in OBJECTS:
        md = meta(fam, name)
        fp = re.search(r"footprint: \[(\d+), (\d+)\]", md)
        hgt = int(fp.group(2)) if fp else 1
        draws.append((y + hgt - 1, x, fam, name, (x, y, t)))
    for p in PAWNS:
        draws.append((p[1] + 0.5, p[0], "pawn", None, p))
    for row, col, fam, name, extra in sorted(draws, key=lambda d: (d[0], d[1])):
        if fam == "pawn":
            draw_pawn(world, extra)
        elif extra is None:
            put(world, fam, name, col, row)
        else:
            put(world, fam, name, extra[0], extra[1], extra[2])
    for (x, y) in DESIGNATED:
        put(world, "overlay", "designation_cut", x, y)
    for (x, y) in BLUEPRINTS:
        put(world, "overlay", "blueprint", x, y)
    put(world, "overlay", "selection", *SELECTED)
    canvas.alpha_composite(world, (0, 0))
    if zoom == 1:
        canvas = canvas.resize((canvas.width // 2, canvas.height // 2), Image.LANCZOS)
    return canvas


def attach(md, key):
    m = re.search(key + r": \[([\d.]+), ([\d.]+)\]", md)
    return (float(m.group(1)), float(m.group(2))) if m else None


# Pawns (ART §10, §27.1): one standing sprite per layer for south, east and
# north; west is east mirrored about the canvas. Layers share a canvas and
# origin per facing. Held items put their `grip` on the body's `hold`
# attachment, behind the body facing north, with a tinted `pawn/hand` dot on
# the grip (hidden facing north).
PAWN_SRC = {"south": "south", "east": "east", "north": "north", "west": "east"}
BUILD_LAYERS = ("body", "pants_basic", "shirt_basic", "vest_utility", "parka_leather", "duster_leather")
LEGACY_BUILD = "m_avg"
ROT = {"east": 0, "south": -90, "west": 0, "north": 90}  # PIL rotate() is counter-clockwise


def pawn_key(layer, facing, build=LEGACY_BUILD):
    tag = layer if build == LEGACY_BUILD or layer not in BUILD_LAYERS else "%s_%s" % (layer, build)
    return "%s_%s" % (tag, PAWN_SRC[facing])


def mirror_x(img, o, footprint_w=1):
    """Flip a sprite left-right about its footprint: the origin moves so the
    footprint stays on the same tile."""
    return img.transpose(Image.FLIP_LEFT_RIGHT), (img.width - o[0] - footprint_w * PPT, o[1])


def pawn_sprite(name, facing, tint=None):
    img, o = sprite("pawn", name, tint)
    if img is not None and facing == "west":
        img, o = mirror_x(img, o)
    return img, o


def put_pawn(world, name, facing, x, y, tint=None):
    img, o = pawn_sprite(name, facing, tint)
    if img is None:
        return False
    world.alpha_composite(img, (OFF + x * PPT - o[0], OFF + y * PPT - o[1]))
    return True


def hold_point(x, y, facing, build=LEGACY_BUILD):
    """World px of the body's `hold` attachment (mirrored facing west)."""
    key = pawn_key("body", facing, build)
    md = meta("pawn", key)
    hp = attach(md, "hold")
    img, po = sprite("pawn", key)
    if hp is None or img is None:
        return None
    if facing == "west":
        hp = (img.width - hp[0], hp[1])
        po = (img.width - po[0] - PPT, po[1])
    return (OFF + x * PPT - po[0] + hp[0], OFF + y * PPT - po[1] + hp[1])


def draw_rifle(world, x, y, facing, skin, build=LEGACY_BUILD):
    img, o = sprite("weapon", "rifle_basic")
    hp = hold_point(x, y, facing, build)
    if img is None or hp is None:
        return
    gx, gy = attach(meta("weapon", "rifle_basic"), "grip")
    # rotate about the grip so the grip stays fixed
    big = Image.new("RGBA", (img.width * 3, img.height * 3))
    big.alpha_composite(img, (img.width, img.height))
    cx, cy = gx + img.width, gy + img.height
    rot = big.rotate(ROT[facing], center=(cx, cy), resample=Image.BICUBIC)
    if facing == "west":
        rot = rot.transpose(Image.FLIP_LEFT_RIGHT)
        cx = big.width - cx
    world.alpha_composite(rot, (int(hp[0] - cx), int(hp[1] - cy)))
    if facing != "north":
        hand, _ = sprite("pawn", "hand", {"skin": skin})
        c = attach(meta("pawn", "hand"), "center")
        if hand is not None and c is not None:
            world.alpha_composite(hand, (int(hp[0] - c[0]), int(hp[1] - c[1])))


def draw_pawn(world, p):
    x, y, f, skin, hair, hair_t, shirt_t, vest_t, rifle = p[:9]
    build = p[9] if len(p) > 9 else LEGACY_BUILD
    layers = [("body", {"skin": skin}), ("pants_basic", {"cloth": "slate" if shirt_t != "slate" else "navy"}),
              ("shirt_basic", {"cloth": shirt_t})]
    if vest_t:
        layers.append(("vest_utility", {"cloth": vest_t}))
    layers.append((hair, {"hair": hair_t}))
    if rifle and f == "north":
        draw_rifle(world, x, y, f, skin, build)
    for name, t in layers:
        put_pawn(world, pawn_key(name, f, build), f, x, y, t)
    if rifle and f != "north":
        draw_rifle(world, x, y, f, skin, build)


def pawn_sheet(path, builds=(LEGACY_BUILD,)):
    outfits = [
        ("tan", "hair_short", "black", "sand", "olive", True),
        ("pale", "hair_bun", "auburn", "slate", None, False),
        ("dark", "hair_crop", "grey", "cream", "rust", False),
        ("brown", "hair_short", "blond", "olive", None, False),
    ]
    facings = ["south", "east", "north", "west"]
    rows = [(o, b) for b in builds for o in outfits]
    img = Image.new("RGBA", (len(facings) * PPT * 2, len(rows) * PPT * 2), (134, 112, 79, 255))
    for r, (o, b) in enumerate(rows):
        for c, f in enumerate(facings):
            draw_pawn(img, (c * 2, r * 2 + 1, f) + o + (b,))
    img.convert("RGB").save(path)
    print(path)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-o", "--out", default=os.path.join(cs.OUT_ROOT, "generated", "prototype_scene.png"))
    ap.add_argument("--zoom", type=int, default=1)
    ap.add_argument("--pawns", help="write a pawn layer sheet to this path instead")
    ap.add_argument("--builds", default=LEGACY_BUILD, help="pawn sheet builds, comma list (e.g. m_avg,f_avg)")
    a = ap.parse_args()
    if a.pawns:
        return pawn_sheet(a.pawns, [b for b in a.builds.split(",") if b])
    img = compose(a.zoom)
    img.convert("RGB").save(a.out)
    print(a.out, img.size)


if __name__ == "__main__":
    main()
