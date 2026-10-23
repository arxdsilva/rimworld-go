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
    # x, y, facing, skin, hair style, hair tint, shirt tint, vest tint|None, rifle
    (8, 7, "south", "tan", "hair_short", "black", "sand", "olive", True),
    (4, 6, "east", "pale", "hair_bun", "auburn", "slate", None, False),
    (10, 4, "west", "dark", "hair_crop", "grey", "cream", "rust", False),
    (14, 3, "south", "light", "hair_bun", "black", "rust", "slate", True),
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


ROT = {"east": 0, "south": -90, "west": 180, "north": 90}  # PIL rotate() is counter-clockwise


def draw_rifle(world, x, y, facing):
    img, o = sprite("weapon", "rifle_basic")
    if img is None:
        return
    md = meta("weapon", "rifle_basic")
    gx, gy = attach(md, "grip")
    hand = attach(meta("pawn", "body_" + facing), "hand")
    _, po = sprite("pawn", "body_" + facing)
    # rotate about the grip so the grip stays fixed
    big = Image.new("RGBA", (img.width * 3, img.height * 3))
    big.alpha_composite(img, (img.width, img.height))
    cx, cy = gx + img.width, gy + img.height
    rot = big.rotate(ROT[facing], center=(cx, cy), resample=Image.BICUBIC)
    if facing == "west":
        rot = big.transpose(Image.FLIP_LEFT_RIGHT)
        cx = big.width - cx
    px = OFF + x * PPT - po[0] + hand[0]
    py = OFF + y * PPT - po[1] + hand[1]
    world.alpha_composite(rot, (int(px - cx), int(py - cy)))


def draw_pawn(world, p):
    x, y, f, skin, hair, hair_t, shirt_t, vest_t, rifle = p
    layers = [("body", {"skin": skin}), ("pants_basic", {"cloth": "slate" if shirt_t != "slate" else "navy"}),
              ("shirt_basic", {"cloth": shirt_t})]
    if vest_t:
        layers.append(("vest_utility", {"cloth": vest_t}))
    layers.append((hair, {"hair": hair_t}))
    if rifle and f == "north":
        draw_rifle(world, x, y, f)
    for name, t in layers:
        put(world, "pawn", "%s_%s" % (name, f), x, y, t)
    if rifle and f != "north":
        draw_rifle(world, x, y, f)


def pawn_sheet(path):
    outfits = [
        ("tan", "hair_short", "black", "sand", "olive", True),
        ("pale", "hair_bun", "auburn", "slate", None, False),
        ("dark", "hair_crop", "grey", "cream", "rust", False),
        ("brown", "hair_short", "blond", "olive", None, False),
    ]
    facings = ["south", "east", "north", "west"]
    img = Image.new("RGBA", (len(facings) * PPT * 2, len(outfits) * PPT * 2), (134, 112, 79, 255))
    for r, o in enumerate(outfits):
        for c, f in enumerate(facings):
            draw_pawn(img, (c * 2, r * 2 + 1, f) + o)
    img.convert("RGB").save(path)
    print(path)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-o", "--out", default=os.path.join(ROOT, "generated", "prototype_scene.png"))
    ap.add_argument("--zoom", type=int, default=1)
    ap.add_argument("--pawns", help="write a pawn layer sheet to this path instead")
    a = ap.parse_args()
    if a.pawns:
        return pawn_sheet(a.pawns)
    img = compose(a.zoom)
    img.convert("RGB").save(a.out)
    print(a.out, img.size)


if __name__ == "__main__":
    main()
