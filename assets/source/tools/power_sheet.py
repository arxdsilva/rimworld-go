#!/usr/bin/env python3
"""ART §29 `power-buildings` review sheet (PLAY-021): every power sprite at
game scale (64 px per tile), placed with the renderer's rules (origin_px on
the footprint's top-left tile, objects north -> south then west -> east).

Top: daylight, unpowered (no glow layers), on soil. Middle: the same scene
at night, powered: the world darkened as the light grid would, the `_glow`
layers drawn on top unlit. Then the conduits as power-overlay markings (one
colour per net) and the new icons at their HUD size on the HUD charcoal.

    python3 assets/source/tools/power_sheet.py [-o assets/generated/power_buildings.png]

Dev aid only; the game never reads it.
"""
import argparse
import os
import re

from PIL import Image, ImageDraw

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
GEN = os.path.join(ROOT, "generated", "world")
META = os.path.join(ROOT, "metadata")
ICONS = os.path.join(ROOT, "generated", "ui", "icons")
PPT = 128                                      # @2x pixels per tile
SOIL, SOIL_DARK, HUD = (0x86, 0x70, 0x4f), (0x5c, 0x4a, 0x37), (0x1c, 0x21, 0x28)
NIGHT = (0.30, 0.33, 0.45)                     # multiply for the night strip
NETS = ((0xf2, 0xc0, 0x4a), (0x5a, 0xd0, 0xe6))

W, H = 30, 13


def meta(key):
    fam, name = key.split("/")
    txt = open(os.path.join(META, fam, name + ".yaml"), encoding="utf-8").read()
    ox, oy = re.search(r"^origin_px: \[([\d.]+), ([\d.]+)\]", txt, re.M).groups()
    att = dict((k, (float(x), float(y))) for k, x, y in
               re.findall(r"^  (\w+): \[([\d.-]+), ([\d.-]+)\]", txt.split("attachments:")[1], re.M)) \
        if "attachments:" in txt else {}
    glow = re.search(r"^glow: (\S+)", txt, re.M)
    return (float(ox), float(oy)), att, glow.group(1) if glow else None


def sprite(key):
    fam, name = key.split("/")
    return Image.open(os.path.join(GEN, fam, name + ".png")).convert("RGBA")


def mul(img, rgb):
    r, g, b, a = img.split()
    return Image.merge("RGBA", [c.point(lambda v, k=k: int(v * k)) for c, k in zip((r, g, b), rgb)] + [a])


# ---------------------------------------------------------------- layout
CONDUIT = {(x, 2) for x in range(1, 9)} | {(1, y) for y in range(2, 6)} | {(8, y) for y in range(2, 6)} | \
          {(x, 5) for x in range(1, 9)} | {(4, y) for y in range(5, 8)} | {(11, 4)}
SWITCHES = {(6, 2): "on", (6, 5): "off"}
for p in SWITCHES:
    CONDUIT.discard(p)
LINKS = CONDUIT | set(SWITCHES)


def mask(x, y):
    m = 0
    for bit, (dx, dy) in ((1, (0, -1)), (2, (1, 0)), (4, (0, 1)), (8, (-1, 0))):
        if (x + dx, y + dy) in LINKS:
            m |= bit
    return m


def wall_run(x, y, axis):
    """Steel walls either side of an in-wall cooler at (x, y)."""
    out = []
    if axis == "ew":
        for dx, m in ((-2, 2), (-1, 10), (1, 10), (2, 8)):
            out.append(("structure/wall_steel_%02d" % m, x + dx, y, 1))
    else:
        for dy, m in ((-2, 4), (-1, 5), (1, 5), (2, 1)):
            out.append(("structure/wall_steel_%02d" % m, x, y + dy, 1))
    return out


def objects():
    """(key, x, y, footprint height) for the scene, plus label anchors."""
    objs, labels = [], []

    def add(key, x, y, h=1, label=None):
        objs.append((key, x, y, h))
        if label:
            labels.append((label, x, y + h))

    for (x, y) in sorted(CONDUIT):
        add("power/power_conduit_%02d" % mask(x, y), x, y)
    for (x, y), s in SWITCHES.items():
        add("power/power_switch_" + s, x, y, 1, "switch " + s)
    labels.append(("conduit (linked)", 1, 8))
    add("power/wood_fired_generator", 13, 1, 2, "wood-fired")
    add("power/solar_panel", 16, 1, 3, "solar panel")
    add("structure/steam_vent", 20, 1, 2)
    add("power/geothermal_generator", 20, 1, 2, "geothermal")
    for k, (f, x, y, h) in enumerate((("south", 23, 1, 2), ("north", 25, 1, 2), ("east", 27, 1, 1),
                                      ("west", 27, 3, 1))):
        add("power/wind_turbine_" + f, x, y, h, "turbine " + f[0])
    for k, (f, x, y, h) in enumerate((("south", 13, 6, 2), ("north", 14, 6, 2), ("east", 15, 6, 1),
                                      ("west", 15, 8, 1))):
        add("power/battery_" + f, x, y, h, "battery " + f[0] if k < 3 else None)
    add("power/heater", 18, 6, 1, "heater")
    add("furniture/torch_lamp", 20, 6, 1, "torch")
    add("furniture/lamp_standing", 21, 6, 1, "(lamp)")
    for f, x, y, axis in (("south", 3, 10, "ew"), ("north", 9, 10, "ew"), ("east", 23, 9, "ns"),
                          ("west", 27, 9, "ns")):
        for o in wall_run(x, y, axis):
            add(*o)
        add("power/cooler_" + f, x, y, 1, "cooler " + f[0])
    for k, f in enumerate(("south", "north")):
        add("furniture/electric_stove_" + f, 13 + 3 * k, 10, 1, "e-stove " + f[0])
    add("furniture/electric_tailoring_bench_south", 13, 12, 1, "e-tailor")
    add("furniture/electric_research_bench_south", 16, 12, 1, "e-research")
    add("furniture/electric_research_bench_east", 19, 10, 2, "e-research e")
    add("furniture/electric_tailoring_bench_west", 20, 10, 2, "e-tailor w")
    return objs, labels


def scene(night):
    img = Image.new("RGBA", (W * PPT, H * PPT), SOIL + (255,))
    dr = ImageDraw.Draw(img)
    dr.rectangle([0, 9 * PPT, 12 * PPT, H * PPT], fill=SOIL_DARK + (255,))     # dark ground under the coolers
    objs, labels = objects()
    glows = []
    order = sorted(objs, key=lambda o: (o[2] + o[3], o[1]))     # stable: a vent stays under its generator
    conduits = [o for o in order if "conduit" in o[0]]
    rest = [o for o in order if "conduit" not in o[0]]
    for key, x, y, h in conduits + rest:
        (ox, oy), att, glow = meta(key)
        px, py = int(x * PPT - ox), int(y * PPT - oy)
        img.alpha_composite(sprite(key), (px, py))
        if glow:
            glows.append((glow, px, py))
        if "rotor" in att:                       # the turbine's rotor layer, a different frame each
            n = 1 + (x * 3 + y) % 8
            bkey = "power/wind_turbine_blades_%02d" % n
            (_, _), batt, _ = meta(bkey)
            hx, hy = batt["hub"]
            img.alpha_composite(sprite(bkey), (int(px + att["rotor"][0] - hx), int(py + att["rotor"][1] - hy)))
        if "fire" in att and night:
            fx, fy = px + att["fire"][0], py + att["fire"][1]
            glows.append(("flame", fx, fy))
    if night:
        img = mul(img, NIGHT)
        for g, px, py in glows:
            if g == "flame":                     # stand-in for drawFlame: a warm dot
                d = ImageDraw.Draw(img)
                d.ellipse([px - 14, py - 26, px + 14, py + 6], fill=(255, 190, 90, 255))
                continue
            img.alpha_composite(sprite(g), (px, py))
    return img, labels


def overlay_strip():
    """The conduit network as power-overlay markings: two nets, two colours."""
    img = Image.new("RGBA", (W * PPT, 9 * PPT), tuple(int(c * 0.45) for c in SOIL) + (255,))
    left = {p for p in LINKS if p[0] <= 4 and p != (4, 5)}
    for (x, y) in sorted(LINKS):
        if (x, y) in SWITCHES:
            continue
        key = "power/hidden_conduit_%02d" % mask(x, y)
        net = NETS[0] if (x, y) in left else NETS[1]
        img.alpha_composite(mul(sprite(key), tuple(c / 255 for c in net)), (x * PPT, y * PPT))
    for m in range(16):                          # every mask, in the first net colour
        x, y = 13 + (m % 8) * 2, 1 + (m // 8) * 2
        img.alpha_composite(mul(sprite("power/hidden_conduit_%02d" % m), tuple(c / 255 for c in NETS[0])),
                            (x * PPT, y * PPT))
        (ox, oy), _, _ = meta("power/power_conduit_%02d" % m)
        img.alpha_composite(sprite("power/power_conduit_%02d" % m), (int(x * PPT - ox), int((y + 4) * PPT - oy)))
    return img


ICON_NAMES = ("power_conduit", "hidden_conduit", "power_switch", "wood_fired_generator", "solar_panel",
              "wind_turbine", "geothermal_generator", "battery", "torch_lamp", "heater", "cooler",
              "electric_stove", "electric_tailoring_bench", "electric_research_bench",
              "overlay_power", "power_toggle", "thermostat_up", "thermostat_down", "no_power")


def icon_strip(width):
    img = Image.new("RGBA", (width, 110), HUD + (255,))
    dr = ImageDraw.Draw(img)
    x = 8
    for n in ICON_NAMES:
        p = os.path.join(ICONS, n + ".png")
        if os.path.exists(p):
            ic = Image.open(p).convert("RGBA").resize((64, 64), Image.LANCZOS)
            img.alpha_composite(ic, (x, 8))
            sm = ic.resize((32, 32), Image.LANCZOS)        # the smallest HUD size
            img.alpha_composite(sm, (x + 16, 74))
        dr.text((x, 60), n[:12], fill=(220, 220, 210, 255))
        x += 76
    return img


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("-o", "--out", default=os.path.join(ROOT, "generated", "power_buildings.png"))
    a = ap.parse_args()
    day, labels = scene(False)
    night, _ = scene(True)
    ov = overlay_strip()
    parts = [day, night, ov]
    half = [p.resize((p.width // 2, p.height // 2), Image.LANCZOS) for p in parts]   # game scale
    icons = icon_strip(half[0].width)
    sheet = Image.new("RGBA", (half[0].width, sum(p.height for p in half) + icons.height + 12 * 4), HUD + (255,))
    y = 0
    dr = ImageDraw.Draw(sheet)
    for k, (p, title) in enumerate(zip(half, ("day, unpowered", "night, powered (glow layers)",
                                              "power overlay: hidden-conduit markings per net; all masks"))):
        dr.text((6, y), title, fill=(240, 236, 220, 255))
        y += 12
        sheet.alpha_composite(p, (0, y))
        if k < 2:
            for text, tx, ty in labels:
                ly = ty * 64 + 1 if ty < H else ty * 64 - 12      # the last row labels inside
                dr.text((tx * 64 + 2, y + ly), text, fill=(255, 250, 235, 255))
        y += p.height
    dr.text((6, y), "icons: 64 px and 32 px", fill=(240, 236, 220, 255))
    sheet.alpha_composite(icons, (0, y + 12))
    os.makedirs(os.path.dirname(a.out), exist_ok=True)
    sheet.convert("RGB").save(a.out)
    print(a.out, sheet.size)


if __name__ == "__main__":
    main()
